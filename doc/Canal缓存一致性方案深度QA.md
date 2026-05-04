# Canal 缓存一致性方案 深度 Q&A

> 项目: Go-Forum 微服务 | 日期: 2026-05-04 | 涉及组件: Canal / Kafka / Redis / MySQL / gobreaker

---

## Q1: Canal 如何连接 MySQL 并监听 binlog？

### 1.1 MySQL 端前置条件

Canal 依赖 MySQL 开启 **ROW 格式的 binlog**。在 `docker-compose.yml` 中配置：

```yaml
mysql:
  image: mysql:8.0
  command:
    - --server-id=1              # 服务器唯一 ID (主从复制必须)
    - --log-bin=mysql-bin        # 开启 binlog，日志文件前缀
    - --binlog-format=ROW        # Canal 必须用 ROW 格式
    - --binlog-row-image=FULL    # 记录完整行 (含修改前后全字段)
    - --expire-logs-days=7       # 日志保留 7 天
```

**为什么必须是 ROW 格式？**
- `STATEMENT` 格式只记录 SQL 语句，Canal 无法还原具体数据变更
- `MIXED` 格式在非确定性语句时切回 STATEMENT，有丢数据风险
- `ROW` 格式记录每行修改前后的完整数据，Canal 可精确提取变更字段

### 1.2 Canal 伪装成 MySQL Slave

Canal 的核心原理是**实现 MySQL Replication Protocol**，伪装成一个 Slave 节点：

```
                    MySQL Master (forum-mysql:3306)
                    server-id=1, binlog=ROW
                         │
                         │ ① TCP handshake
                         │ ② COM_REGISTER_SLAVE (server-id=1001)
                         │ ③ COM_BINLOG_DUMP (file, position)
                         │ ④ Stream binlog events
                         ▼
                    Canal (伪装 Slave)
                    server-id=1001
                         │
                         │ parse → filter → serialize
                         ▼
                    Kafka (canal-binlog)
```

**关键配置项解读**：

| 配置 | 值 | 含义 |
|:---|:---|:---|
| `canal.instance.master.address` | `mysql:3306` | MySQL 的容器内网络地址 |
| `canal.instance.dbUsername` | `root` | 连接 MySQL 的账号（**必须与 MYSQL_ROOT_PASSWORD 配套**） |
| `canal.instance.filter.regex` | `forum\\..*` | 只 dump `forum.` 开头的表 |
| `canal.instance.filter.black.regex` | `mysql\\.slave_.*` | 排除系统表 |
| `canal.serverMode` | `kafka` | 输出模式：直接写入 Kafka |
| `canal.mq.servers` | `kafka:9092` | Kafka broker 地址 |
| `canal.mq.topic` | `canal-binlog` | 写入的 Topic |

### 1.3 Canal 启动时的完整流程

从实际日志可追踪每一步：

```
Step 1: 加载表过滤器
  "init table filter : ^forum\\..*$"       ← 正则匹配
  "init table black filter : ^mysql\\.slave_.*$"

Step 2: 启动 Canal Instance
  "start CannalInstance for 1-example"
  "start successful...."

Step 3: 连接到 MySQL 并查找 binlog 起始位点
  "begin to find start position, it will be long time for reset or first position"
  "prepare to find start position just show master status"
  destination = example, address = mysql/172.24.0.5:3306

Step 4: 定位成功，准备 dump
  "find start position successfully"
  EntryPosition:
    journalName=mysql-bin.000003    ← binlog 文件名
    position=4                      ← 字节偏移量
    serverId=1
    cost: 189ms

Step 5: 开始持续监听
  "the next step is binlog dump"     ← 进入无限循环，持续接收 binlog 事件
```

### 1.4 数据流转示例

假设用户执行 `UPDATE posts SET title='new' WHERE id=1`：

```
① MySQL 执行 SQL，写入 binlog 事件:
   Table_map: forum.posts
   Update_rows: id=1, before={title:"old"}, after={title:"new"}

② Canal EventParser 从 TCP 流中解析出 ROW 事件

③ Canal EventSink 过滤 (regex=forum\\..*) → 序列化为 JSON:
   {
     "type": "UPDATE",
     "database": "forum",
     "table": "posts",
     "data": [{"id": 1, "title": "new"}]
   }

④ Canal MQ Producer 写入 Kafka:
   Topic: canal-binlog
   Key: posts
   Value: <JSON above>
   Partition: hash(key) % partition_count

⑤ forum-app Canal Consumer 消费:
   processCanalMessage() → redis.DeleteCacheWithCB("forum:post:1") → 缓存失效
```

---

## Q2: Kafka/Redis 宕机时，Canal 消息如何保障正确消费？

### 2.1 故障场景矩阵

| 故障组件 | 时机 | 后果 | 保护层级 |
|:---|:---|:---|:---|
| Kafka | Canal 写入时 | 消息丢失 (Canal 内部重连后从 binlog 续传) | Canal 层自动恢复 |
| Kafka | Consumer 读取时 | 消费中断，消息积压 | Consumer 重连机制 |
| Redis | 缓存删除时 | 缓存变 stale，与 DB 不一致 | L1→L2→L3→L4 逐级兜底 |
| Redis | 缓存查询时 | 读请求穿透到 MySQL | gobreaker + DB Fallback |
| Consumer | 处理逻辑异常 | 消息未处理但 offset 已提交 | 手动提交 + DLQ |

### 2.2 容灾架构全景

```
Canal → Kafka (canal-binlog)
              │
              ▼
    ┌─── ReadMessage() ───┐
    │   (consumer group)   │
    └──────────┬──────────┘
               │
               ▼
    ┌─────────────────────────┐
    │ processCanalMessage     │
    │ WithRetry()             │  ← L1: 指数退避重试
    │                         │
    │ attempt 1: 0ms  later   │
    │ attempt 2: 200ms later  │
    │ attempt 3: 400ms later  │
    │ attempt 4: 800ms later  │
    │                         │
    │ maxRetries = 3          │
    │ baseRetryBackoff = 200ms│
    └──────────┬──────────────┘
               │
          ┌────┴────┐
          │ 成功?     │
          ├─ Yes ────────────► CommitMessage() ──► offset 持久化
          │
          └─ No ────────────► sendToDLQ()
                                    │
                                    ▼
                         Topic: canal-binlog-dlq
                         Headers:
                           dlq-reason: "retry-exhausted"
                           dlq-original-offset: 12345
                                    │
                                    ▼
                         ProcessDLQMessages()
                         (独立 consumer group)
                                    │
                         ┌─ 成功 ──► 提交 DLQ offset
                         │
                         └─ 失败 ──► 保持积压，等待人工介入
```

### 2.3 四层保护详解

#### L1: 瞬时故障 — 指数退避重试

```go
const (
    maxRetries       = 3
    baseRetryBackoff = 200 * time.Millisecond
)

func processCanalMessageWithRetry(m kafka.Message) error {
    for attempt := 0; attempt < maxRetries; attempt++ {
        if attempt > 0 {
            backoff := baseRetryBackoff * time.Duration(1<<(attempt-1))
            time.Sleep(backoff)  // 200ms → 400ms → 800ms
        }
        if err := processCanalMessage(m); err == nil {
            return nil
        }
    }
    return fmt.Errorf("max retries exhausted")
}
```

**覆盖场景**: Redis 瞬时网络抖动、连接池耗尽、内存碎片导致的慢响应。

**代价**: 最多 200+400+800 = 1.4 秒额外延迟。

#### L2: 持续故障 — 熔断器降级 + TTL 兜底

```go
// gobreaker 熔断器配置
cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "redis-breaker",
    MaxRequests: 3,           // 半开状态最多 3 个探测请求
    Interval:    5 * time.Second,
    Timeout:     10 * time.Second,  // 熔断后 10s 进入半开
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures >= 5  // 连续 5 次失败 → 熔断
    },
})

// Canal consumer 中的熔断感知删除
func DeleteCacheWithCB(key string) error {
    _, err := cb.Execute(func() (interface{}, error) {
        return nil, rdb.Del(ctx, key).Err()
    })
    return err  // 返回 ErrCircuitOpen 但不阻塞主流程
}

// 调用方不因 Redis 失败而阻断
if err := redis.DeleteCacheWithCB(cacheKey); err != nil {
    zap.L().Warn("L2 cache invalidation failed, relying on TTL")
    // ← 关键: 不 return error，让 TTL 最终一致
}
```

**状态机**:

```
        正常 → Closed ── 连续5次失败 ──→ Open (拒绝所有请求)
                   ↑                          │
                   │                   10s 超时后
                   │                          ↓
                   └── 探测成功 ←── Half-Open (允许3个请求探测)
                         探测失败 → 回到 Open
```

**覆盖场景**: Redis 进程崩溃、网络分区、内存 OOM。熔断期间所有缓存删除被跳过，依赖 L1 本地缓存的短 TTL + Redis 自身 key TTL 实现最终一致。

#### L3: 处理失败 — 死信队列 (DLQ)

```go
func sendToDLQ(m kafka.Message) {
    dlqMsg := kafka.Message{
        Key:    m.Key,
        Value:  m.Value,
        Headers: []kafka.Header{
            {Key: "dlq-reason", Value: []byte("retry-exhausted")},
            {Key: "dlq-original-offset", Value: []byte(fmt.Sprintf("%d", m.Offset))},
        },
    }
    dlqWriter.WriteMessages(ctx, dlqMsg)
}
```

**为什么要 DLQ 而不是无限重试？**
1. **消费顺序**: Kafka 保证分区内有序。无限重试会阻塞后续消息
2. **资源占用**: 重试消耗 CPU + 网络，不如快速失败让运维介入
3. **问题定位**: DLQ 保留原始 offset + 失败原因 header，便于回溯

**DLQ 消息的生命周期**:

```
canal-binlog-dlq (积压)
    │
    │ ProcessDLQMessages()   ← 独立 consumer group，定时轮询
    │
    ├── 修复成功 → CommitMessage → 删除
    │
    └── 修复失败 → 保持积压 → 告警 → 人工:
         ├── 检查 Redis 状态
         ├── 判断是否需要手动失效缓存
         └── 或直接 ReconcileFromDB()
```

#### L4: 对账修复 — 全量重建

```go
func StartReconciliationLoop() {
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        for range ticker.C {
            ReconcileFromDB()
        }
    }()
}

func ReconcileFromDB() {
    // SELECT id, title, ... FROM posts WHERE status = 1
    // 逐条写入 Redis: SET forum:post:{id} <json> EX 3600
    // 逐条写入 L1: l1Cache.Set(key, json)
}
```

**触发条件**: DLQ 积压超过阈值 OR Redis 重启后首次启动 OR 手动触发。

**覆盖场景**: 长时间 Redis 宕机导致大量缓存 miss，批量预热避免冷启动雪崩。

### 2.4 为什么"手动提交 offset"是关键？

```go
// ❌ 默认: 自动提交 — 消息读取即提交
reader := kafka.NewReader(kafka.ReaderConfig{
    AutoCommit: true,  // 默认行为
})

// ✅ 容灾: 手动提交 — 仅在处理成功后提交
m, _ := reader.ReadMessage(ctx)
if err := processCanalMessageWithRetry(m); err != nil {
    sendToDLQ(m)  // 失败消息写入 DLQ
}
reader.CommitMessages(ctx, m)  // ← 不论成败都提交 (成功=已处理, 失败=已入DLQ)
```

**如果自动提交会发生什么？**

```
Time  Message   Action
──────────────────────────────────────────────
T1    M99       ReadMessage + CommitOffset=99 (自动)
T2              processMessage(M99) → Redis 宕机 → 失败
T3              M99 已丢失 (offset 已提交，不会重放)
T4              → 缓存永久不一致
```

**手动提交后**:

```
Time  Message   Action
──────────────────────────────────────────────
T1    M99       ReadMessage
T2              processMessage(M99) → 失败
T3              sendToDLQ(M99)      → canal-binlog-dlq
T4              CommitMessage(M99)  → offset=100 (下一轮从 100 开始)
T5              DLQ 中有 M99，等待对账修复
```

### 2.5 极限场景推演

**场景 A: Kafka 集群完全宕机**

```
Canal:  无法写入 → 内部重试 → binlog 位点保持不变 → Kafka 恢复后从断点续传
Consumer: ReadMessage 阻塞 → 超时后 continue → 等待 Kafka 恢复
结果:    无消息丢失 (Canal 的 binlog 位点持久化在 ZooKeeper)
```

**场景 B: Redis 集群完全宕机 + 用户此时查询帖子**

```
Read Path:
  ① L1 Cache (otter) → miss (冷数据或已过期)
  ② Singleflight → 合并并发请求
  ③ Circuit Breaker → Redis 熔断已 Open → 跳过
  ④ MySQL Fallback → 查 DB 返回数据
  ⑤ 异步回写 → go cb.Execute(rdb.Set) → Open → 跳过
  ⑥ L1 写入 → Set(key, data) ← 只有这一层成功

  → 请求被正常响应，只是没有 L2 缓存
  → Redis 恢复后 CB 进入 Half-Open → 探测成功 → Closed
  → 下次 ReconcileFromDB 预热回来
```

**场景 C: 应用进程 crash (Consumer + L1 缓存一起丢失)**

```
Consumer: Kafka group rebalance → 同组其他 consumer 接管 → 从上次 commit offset 继续
L1 Cache: 内存中数据全丢 → 启动后冷启动
恢复:
  T0: 进程重启
  T1: L1 Cache 为空，所有请求走 L2(Redis)
  T2: Redis 中的缓存大部分仍在 (独立进程)
  T3: Canal Consumer 恢复，从 last committed offset 继续
  → 短暂的 L1 miss 增加 Redis 负载，不影响正确性
```

### 2.6 容灾体系关键代码文件

| 文件 | 职责 |
|:---|:---|
| `internal/pkg/kafka/canal_consumer.go` | 重试 + DLQ + 对账循环 |
| `internal/dao/redis/redis.go` | `DeleteCacheWithCB()` 熔断保护删除 |
| `internal/dao/redis/cache.go` | L1(otter) + Singleflight + CB + L2(Redis) + MySQL 降级 |
| `docker-compose.yml` | Canal v1.1.7 + MySQL binlog ROW + Kafka 29092 端口 |
| `internal/pkg/kafka/canal_consumer_test.go` | 12 个单元/E2E 测试 |

### 2.7 验证结果

```bash
# 全部 12 个 Canal 相关测试通过
TestProcessCanalMessage_InsertPost        PASS ─── INSERT 不触发失效
TestProcessCanalMessage_UpdatePost        PASS ─── UPDATE 失效 L2+L1
TestProcessCanalMessage_DeletePost        PASS ─── DELETE 失效 L2+L1
TestProcessCanalMessage_NonPostTable      PASS ─── 非 posts 表不触发
TestProcessCanalMessage_InvalidJSON       PASS ─── 无效 JSON 返回 error
TestProcessCanalMessage_RedisDown         PASS ─── Redis 宕机→TTL 兜底
TestProcessCanalMessageWithRetry_Success  PASS ─── 重试包裹正常通过
TestProcessCanalMessageWithRetry_Exhausted PASS ── 重试耗尽返回 error
TestSendToDLQ                             PASS ─── DLQ 读写验证
TestCanalPipelineE2E                      PASS ─── 端到端 UPDATE 验证
TestCanalInsertDoesNotInvalidate          PASS ─── 端到端 INSERT 验证
TestProcessCanalMessage_UpdateComments    PASS ─── comments 表不触发
```
