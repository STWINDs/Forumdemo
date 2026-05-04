# QA: 帖子编辑后缓存一致性问题

## 问题描述

编辑帖子保存后，首页帖子列表显示更新后的数据，但点进帖子详情页仍显示修改前的内容。

## 根因分析

帖子详情读取链路涉及三级存储，编辑时只更新了 MySQL，L1 (Otter) 和 L2 (Redis) 缓存仍然持有旧数据：

```
读取链路（编辑后）:
  GetPostByID → L1 hit → 返回旧数据 ✗

期望行为：
  GetPostByID → L1 miss → L2 miss → MySQL → 返回新数据 ✓
```

## 完整调用链路

### 1. 编辑请求链路（写入侧）

```
[前端] PUT /api/v1/post/:id  { title, content, post_type }
  ↓
[handler/post.go:112]  UpdatePostHandler(c)
  ├── c.Param("id") → id: int64            // 路由参数
  ├── c.Get(middleware.ContextUserIDKey) → userID: int64  // JWT 上下文
  ├── c.ShouldBindJSON(&body) → body{Title, Content, PostType}
  │
  ↓
[service/post.go:31]  UpdatePost(id, authorID, title, content, postType)
  ├── mysql.UpdatePost(id, authorID, title, content, postType)
  │     ↓
  │   [dao/mysql/post.go:28]  func UpdatePost(id, authorID int64, title, content string, postType int8)
  │     sqlStr := `update posts set title=?, content=?, post_type=? where id=? and author_id=?`
  │     db.Exec(sqlStr, title, content, postType, id, authorID)
  │     → MySQL 行已更新 ✓
  │
  ├── invalidatePostCache(id)               // ★ 新增：缓存失效
  │     ↓
  │   [service/post.go:47]  func invalidatePostCache(postID int64)
  │     cacheKey := "forum:post:" + strconv.FormatInt(postID, 10)
  │     // 例如 postID=14 → key = "forum:post:14"
  │     │
  │     ├── redis.DelL1(cacheKey)
  │     │     ↓
  │     │   [dao/redis/cache.go:128]  func DelL1(key string)
  │     │     l1Cache.Delete(key)           // Otter 本地缓存逐出
  │     │
  │     └── redis.DeleteCache(cacheKey)
  │           ↓
  │         [dao/redis/redis.go:48]  func DeleteCache(key string)
  │           if err := isReady(); err != nil { return err }  // nil guard
  │           rdb.Del(context.Background(), key)              // Redis L2 删除
  │
  └── return nil → handler 返回 {"msg": "updated"}
```

### 2. 编辑后读取链路（缓存穿透 → 回填）

```
[前端] window.location.reload() → hash router → renderPostDetail(14)
  ↓
[GET] /api/v1/post/14
  ↓
[handler/post.go:87]  GetPostByIDHandler(c)
  ├── c.Param("id") → 14
  ├── service.GetPostByID(ctx, 14)
  │     ↓
  │   [service/post.go:22]  GetPostByID(ctx, 14)
  │     ├── redis.GetPostDetailWithCache(ctx, 14)
  │     │     ↓
  │     │   [dao/redis/post.go:11]  func GetPostDetailWithCache(ctx, id)
  │     │     key := "forum:post:14"
  │     │     GetWithResilience(ctx, key, post, 1h, dbFetch)
  │     │       │
  │     │       ├── [L1] l1Cache.Get("forum:post:14") → miss (已被 DelL1 清除)
  │     │       │
  │     │       ├── [Singleflight] sf.Do(key, fn)
  │     │       │     │
  │     │       │     ├── [L1 双重检查] l1Cache.Get(key) → miss
  │     │       │     │
  │     │       │     ├── [L2 Circuit Breaker] cb.Execute → rdb.Get(ctx, key)
  │     │       │     │   → Redis nil (已被 DeleteCache 删除) → miss
  │     │       │     │
  │     │       │     ├── [DB Fallback] dbFetch()
  │     │       │     │     ↓
  │     │       │     │   mysql.GetPostByID(14)
  │     │       │     │     sql: `select ... from posts where id = 14`
  │     │       │     │     → Post{Title:"Cache FIXED", Content:"Brand new content..."}  ✓
  │     │       │     │
  │     │       │     ├── [回写 L2] goroutine: rdb.Set(key, json, 1h)
  │     │       │     │   → Redis 缓存新数据
  │     │       │     │
  │     │       │     └── [回写 L1] l1Cache.Set(key, jsonStr)
  │     │       │         → Otter 缓存新数据
  │     │       │
  │     │       └── return json.Unmarshal(v, post) → Post 新数据
  │     │
  │     └── enrichVideoURL(post)  // 仅 video 类型
  │
  └── c.JSON(200, post) → 返回最新数据
```

### 3. 后续读取（缓存命中）

```
任何用户再次 GET /api/v1/post/14:
  L1 hit → json.Unmarshal → 返回新数据  (0.001ms)
    或
  L1 miss → L2 hit → 返回新数据  (0.5ms)
```

## 为什么用"清除"而不是"刷新"

| 维度 | 清除 (Delete) | 刷新 (Write-through) |
|:--|:--|:--|
| 编辑写入操作 | UPDATE MySQL + DEL L1 + DEL L2 = 1 次 DB + 2 次 DEL | UPDATE MySQL + SET L1 + SET L2 = 1 次 DB + 2 次 SET + 序列化 |
| 写入原子性 | 部分失败无影响（DEL 幂等） | 部分失败导致三地不一致 |
| 读取惩罚 | 首次穿透到 MySQL（~2ms） | 0ms（命中） |
| 适用场景 | 低频编辑（论坛帖子） | 高频写入（计数器） |

编辑是论坛最低频操作。用户编辑后重新加载页面 → 穿透到 MySQL → 回填缓存 → 之后所有用户都命中。

Singleflight 合并保护：即使同一时刻有 N 个人点进刚编辑的帖子，也只产生 1 次 MySQL 查询。

## 关键文件

| 文件 | 行号 | 函数 | 角色 |
|------|------|------|------|
| `internal/handler/post.go` | 112 | `UpdatePostHandler` | 接收编辑请求，提取参数 |
| `internal/service/post.go` | 31 | `UpdatePost` | 更新 MySQL + 触发缓存失效 |
| `internal/service/post.go` | 47 | `invalidatePostCache` | 拼接 key，调用 L1/L2 删除 |
| `internal/dao/redis/cache.go` | 128 | `DelL1` | Otter 本地缓存逐出 |
| `internal/dao/redis/redis.go` | 48 | `DeleteCache` | Redis 键删除 (+ nil guard) |
| `internal/dao/redis/post.go` | 11 | `GetPostDetailWithCache` | 读取入口 |
| `internal/dao/redis/cache.go` | 50 | `GetWithResilience` | 多级缓存读取 + 回填 |
| `internal/dao/mysql/post.go` | 28 | `UpdatePost` | MySQL 行更新 |
| `web/js/posts.js` | 301 | `showEditForm` submit | 前端保存后 `window.location.reload()` |

## 验证方式

```bash
# 创建帖子
curl -X POST http://localhost:8080/api/v1/post \
  -H 'Authorization: Bearer <token>' \
  -d '{"title":"Old","content":"old","post_type":1,"community_id":1}'

# 编辑
curl -X PUT http://localhost:8080/api/v1/post/<id> \
  -H 'Authorization: Bearer <token>' \
  -d '{"title":"New","content":"new"}'

# 立即读取 — 缓存已清除，穿透到 MySQL 返回新数据
curl http://localhost:8080/api/v1/post/<id> \
  -H 'Authorization: Bearer <token>'
```
