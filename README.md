# Go Forum Microservice

这是一个基于 Go 语言开发的高性能论坛微服务项目。

## 技术栈

- **Web 框架**: Gin
- **配置管理**: Viper
- **日志**: Zap + Lumberjack
- **数据库**: MySQL (SQLx)
- **缓存**: Redis
- **消息队列**: Kafka (segmentio/kafka-go)
- **对象存储**: Minio
- **缓存一致性**: Canal (监听 binlog 异步失效缓存)
- **限流**: 令牌桶算法 (自实现)
- **并发控制**: Singleflight (请求合并)
- **链路追踪**: Trace ID (Context 传播)
- **容器化**: Docker + Docker Compose

## 核心功能

- 用户注册与登录 (JWT 认证)
- 帖子创建、查询与列表 (Redis 缓存 + Singleflight)
- 投票系统 (Redis ZSet 分数计算 + Kafka 异步落盘)
- 评论系统 (Kafka 异步持久化)
- 视频上传与存储 (Minio 支撑)
- 全链路日志追踪

## 目录结构

- `cmd/`: 应用入口
- `config/`: 配置文件与加载逻辑
- `internal/`: 内部核心逻辑
  - `handler/`: HTTP 请求处理层
  - `service/`: 业务逻辑层
  - `dao/`: 数据访问层 (MySQL, Redis)
  - `middleware/`: 中间件 (Auth, RateLimit, Tracing)
  - `model/`: 数据模型
  - `pkg/`: 公共组件 (Kafka, Minio, Logger, Ratelimit)
  - `router/`: 路由配置
- `migrations/`: 数据库初始化脚本
- `docker/`: Docker 相关配置 (Dockerfile, docker-compose)

## 快速启动

### 本地开发运行

1. 确保已安装 Go 1.25, MySQL, Redis, Kafka, Minio。
2. 修改 `config/config.yaml` 中的配置。
3. 运行项目：
   ```bash
   go run cmd/main.go
   ```

### Docker Compose 部署

一键启动所有基础设施及应用：

```bash
docker-compose up -d
```

## API 列表 (部分)

- `POST /api/v1/signup`: 用户注册
- `POST /api/v1/login`: 用户登录
- `POST /api/v1/post`: 发布帖子
- `GET /api/v1/post/:id`: 获取帖子详情
- `POST /api/v1/vote`: 投票
- `POST /api/v1/video/upload`: 上传视频

## 缓存一致性方案

本项目采用 **Canal** 监听 MySQL 的 binlog 变动，并将变更事件发送至 Kafka。应用中的 `canal_consumer` 消费这些事件并根据变动自动删除/更新对应的 Redis 缓存，从而保证 MySQL 与 Redis 的最终一致性。
