# CI/CD 协作改造方案

## 协作方式

每轮先讲解一个关键决策，由用户选择并说明理由，再点评、实施和验证。未确认的设计保留为待决策项。

## 已确认决策

| 项目 | 选择 | 原因 |
| --- | --- | --- |
| 部署目标 | 一台现有远程云服务器 | 已有现网服务器，先围绕现有环境建设流水线 |
| 执行环境 | GitHub 托管 Runner | 在仓库侧执行测试与构建，构建资源与现网服务隔离 |
| 镜像仓库 | Docker Hub 公共仓库 | 暂无私有需求，以稳定可用为目标 |
| 镜像发布触发 | 合并到 main 后自动构建并推送 | 持续准备可选择的候选版本 |
| 生产部署触发 | 用户选择镜像版本后手动触发 | 用户控制上线版本和发布时间 |
| 发布门禁 | 检查、单元测试和集成测试全部成功后才推送镜像 | 集成失败属于发布阻断问题 |
| 交付方向 | 构建 Docker 镜像并推送镜像仓库，服务器拉取版本镜像 | 将代码验证、制品生成与应用运行连接起来 |

## 目标流程草案

代码变更 → Runner 检查和测试 → 构建镜像 → 推送镜像仓库 → 云服务器拉取镜像 → 部署 → 业务验证。

该流程尚未启用自动发布；下面的选择决定具体工作流实现。

## 待逐轮确认

1. 镜像仓库接入：确认 Docker Hub 命名空间、仓库名及认证配置；实际验证国内服务器拉取链路。
2. 仓库分支规则和手动部署输入校验待配置。
3. 将 DLQ 结构化日志接入现网告警，并逐条评估修复和重放。初版不自动重放。
4. 镜像版本及架构：可追溯标签、部署用 digest、服务器 CPU 架构。
5. 发布连接和凭据：Runner 到服务器的访问方式、权限及凭据保存位置。
6. 运行环境：确认现网部署方式、应用配置及持久化依赖，设计应用更新范围。
7. 发布验证与回滚：健康检查、业务检查、失败处理及数据库迁移兼容性。

## 当前实施记录

- 已检查仓库现有 Jekyll Pages workflow、Dockerfile 和 Compose 配置。
- 已记录上述架构决策。
- 已新增 `.github/workflows/publish-image.yml`，main 的 push 自动构建并推送候选镜像。合并产生 push，直接推送 main 也会触发；限制直接推送需要配置仓库分支规则。
- 工作流以 `sha-<完整 Git SHA>` 标识源代码版本，并在 Actions Summary 记录镜像 digest。标签可被重推，部署最终应使用 digest 固定镜像内容。
- 新增 `.github/workflows/verify.yml`：PR 执行 `go vet` 和 `go test -short -race`；main 发布通过 reusable workflow 执行同样检查，再运行现有集成套件。
- 镜像 publish job 使用 `needs: verify`，校验失败、取消或被跳过都会阻止推送。没有使用 `continue-on-error` 绕过测试。参见 [GitHub needs 语义](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-jobs)。
- `docker/ci/compose.yml` 专用于 Runner 的 MySQL/Kafka/ZooKeeper/MinIO，集成作业等待依赖就绪并预建 topic 后执行完整测试，保留测试 JSON 和依赖日志；`pipefail` 保证 tee 不吞掉测试退出码。收尾仅清理 `forum-ci` 项目。
- 依赖真实 Kafka/MinIO 的测试增加 `testing.Short()` 标记，快速测试跳过它们，集成作业不传 `-short`，依赖连接失败会使测试失败。
- 现有集成覆盖真实 Kafka/MinIO，新增评论恢复测试使用真实 MySQL；其他 MySQL 用例主要为 sqlmock，Redis 主要为 miniredis。Redis Cluster 专用测试仍依赖显式环境变量启用，当前 CI 尚未提供 Cluster，不代表已验收 Redis/MySQL 集群故障恢复。
- 缓存异步回填改为持有任务启动时的 client 和 breaker 引用，修复本轮全量 race 测试发现的跨测试全局变量竞争。

## 本轮验证与剩余缺口

- 本地 `go test -short -race -count=1 -timeout=5m ./...` 通过；因 miniredis 和 HTTP 测试需要监听本机临时端口，实际成功运行在沙箱外。
- `go vet ./...`、工作流 YAML/内嵌 Bash 语法、`git diff --check` 和 CI Compose 配置校验通过。
- 本轮已启动 Docker，完整集成套件、HTTP 受理测试和真实评论恢复测试均已执行通过。GitHub 云端测试和镜像推送尚未执行。
- 普通消费者现已使用 `FetchMessage` 和手动提交：失败保留当前消息并退避重试，写库成功后才提交 offset，提交失败也重试。新增真实 Kafka/MySQL 故障回归，见下节；旧版已经提前确认的丢失事件不能依靠本次改造自动找回。
- 远程手动部署工作流尚待接入。

## 评论异步受理：已确认并实施

用户选择：接口确认受理后，发评者立即看到“处理中”的评论，公共列表在落库后展示。参考 B 站公开体验设计，但以下是 Forumdemo 自身的实现。

### 接口契约

`POST /api/v1/comment` 请求示例：

```json
{"post_id":1,"parent_id":0,"content":"一条评论","client_comment_id":"客户端生成的UUID"}
```

- 身份取自 JWT，不接受客户端指定作者。入口验证正整数 post ID、非负 parent ID、1..2000 字符正文和请求 ID 格式。
- 客户端应生成并保留 `client_comment_id`，同一请求在响应超时后使用同一个 ID 重试。兼容未提供 ID 的旧客户端，由服务器补生成 UUID，但这类客户端无法可靠去重自身的网络重试。
- 生产端同步等待 Kafka `RequireAll` 确认后返回 HTTP 202：`{"state":"processing","comment":{...}}`。Kafka 不可用返回 503，参数非法返回 400。受理链路不访问 MySQL，也不使用投票的内存异步队列。
- `RequireAll` 是当前 ISR 的确认要求，不代表自动具备多副本容灾；CI 单 broker 仅验证同一持久存储上的重启恢复。
- `GET /api/v1/comments/submissions/:client_id` 只查询 JWT 用户自己的回执。未查到落库记录返回 `processing`；查到公开评论返回 `published`；已隐藏/删除返回 `unavailable`；数据库故障返回 503，不冒充“发布失败”。
- 查询接口用于追踪客户端已收到的 202 回执；单独查询一个不存在的 ID，不能证明它曾被 Kafka 受理。当前没有独立的持久化“已受理待消费”状态库；可关联的失败由 DLQ 路径记录。
- 公共评论列表仍仅从 MySQL 查询 `status=1` 的评论。

### 落盘、重放与迁移

新表 `comment_submissions` 通过 `(author_id, client_comment_id)` 唯一键协调重复投递。预留回执、插入评论、关联评论 ID 在同一个 MySQL 事务中提交。回放时读取已有评论，校验正文/帖子/父评论一致后复用原 ID，不重复插入。

历史 Kafka 评论事件没有客户端 ID 时，用 topic/partition/offset 的 SHA-256 生成稳定 ID，保证同一条历史事件重放不会重复插入；不同 offset 的历史重复业务事件无法自动识别。

**已有数据库须先执行 `migrations/004_comment_submissions.sql`，再启动新消费者。** 新建环境的 `migrations/init.sql` 已包含该表。本轮只在隔离 CI 数据库执行了建表，没有迁移现网。

消费者串行处理，外层退避从 1 秒增加至最多 32 秒。临时及未知错误原地退避并记录结构化错误日志；未知事件类型也保留，避免滚动发布时误隔离新版本事件。

明确坏消息（已知事件 JSON 无效、评论字段无效、评论请求 ID 冲突）写入 `<业务 topic>-dlq`。部署前预建该 topic，配置保留期、读写权限及与 broker 拓扑匹配的副本策略。原文、原 key、来源 topic/partition/offset 和原因保留在死信中；日志不写正文。DLQ 采用 RequireAll，且可关联的评论必须把失败状态写入 MySQL，才能提交原 offset。任一步失败都不继续。进程崩溃后 DLQ 可能重复，按来源坐标去重。

已有站点先应用 004（若尚未应用）和 `005_comment_submission_failures.sql`。新 202 回包含正文指纹，客户端携带指纹查询作者自己的失败状态，避免冲突事件覆盖原先已经发布的评论。无法解析或关联身份的坏消息只进 DLQ，无法提供个人状态。旧回执没有指纹时维持原查询行为。失败时卡片显示“发布失败，可重试”；重试使用新的请求 ID，同一次重试若未收到确认则继续沿用该新 ID。

生产告警应采集 `kafka event quarantined` 与 `kafka processing blocked; offset retained`，结合消费 lag 和最老积压年龄判断是否持续阻塞。应用目前提供日志信号，不负责外部通知，也不自动重放。

本轮隔离 Kafka/MySQL 实测：恢复回归通过（约 61 秒）；DLQ 回归通过（约 18 秒）。DLQ 测试注入首次写失败，确认 offset 未推进；恢复后实际读到死信，下一条正常评论落库，已发布的原评论未受冲突回执影响。两项均纳入 CI 发布门禁。

### 前端行为

- 收到 202 后立即展示正文及“处理中 · 仅自己可见”，不提供未落库评论的点赞和回复操作。
- sessionStorage 按用户和帖子保存已受理卡片；同一标签页刷新后恢复查询。关闭标签页后不保证保留这些本地卡片；它们不是服务端持久化依据。
- 轮询状态采用逐步延长间隔，最多 30 秒；查询失败仍保留卡片。成功确认落库后显示“已发布”，由用户点击刷新公共列表，避免后台刷新丢失正在输入的草稿。
- 公共列表查询失败时，待处理卡片和顶层评论表单仍保留。帖子详情页本身若无法加载，仍受原有页面依赖限制。
- 提交期间禁用按钮；同一正文在失败后原表单内重试会复用请求 ID。关闭页面或修改正文后属于新的提交，不能承诺端到端恰好一次。

### 本轮实际验证

| 检查 | 实测结果 |
| --- | --- |
| 全包快速测试与 race | 通过；覆盖失败不确认、恢复后重试、提交失败重试、取消不确认，以及事务插入/回放/冲突/回滚 |
| 全包完整测试，隔离 Kafka/MinIO | 通过；Kafka 包耗时约 70 秒 |
| 真实 HTTP 受理、无 MySQL 连接 | `TestCommentAcceptedWithoutDatabase` 通过，返回 202/processing，Kafka 中存在相应请求 ID；约 9 秒 |
| 真实故障恢复 | `TestCommentRecoveryIntegration` 通过；约 34 秒。两次投递同一逻辑评论，停 MySQL 验证无 offset 提交，恢复后在 DB 成功但 offset 未提交时模拟中断，再重启 broker 和 reader；最终两次投递确认、数据库仅一条评论 |
| 浏览器（模拟 API） | Playwright CLI 验证：受理后显示正文和处理中；状态查询 503 时保留卡片；恢复后显示已发布 |
| 静态检查 | go vet、JS 语法、工作流 YAML/Bash、Compose 配置及 diff 空白检查通过 |

复现故障测试（仅隔离 CI 环境）：

```bash
docker compose -p forum-ci -f docker/ci/compose.yml up -d --wait --wait-timeout 180
FORUM_CI_RECOVERY=1 go test ./internal/pkg/kafka \
  -run '^TestCommentRecoveryIntegration$' -v -count=1 -timeout=5m
```

CI 先运行普通集成套件，再单独运行该故障测试，防止服务重启干扰其他包；任一步失败均阻止镜像推送。该回归没有验证机器断电、多 broker 副本丢失、超过保留期或永久非法事件处理。

## 镜像工作流配置

在仓库 Settings → Secrets and variables → Actions 中配置：

| 类型 | 名称 | 值 |
| --- | --- | --- |
| Variable | DOCKERHUB_USERNAME | Docker Hub 登录用户名 |
| Variable | DOCKERHUB_REPOSITORY | 完整镜像名，例如 `yourname/forumdemo`，先在 Docker Hub 建立公共仓库 |
| Variable | IMAGE_PLATFORM | `linux/amd64` 或 `linux/arm64`，需与服务器 CPU 架构匹配 |
| Secret | DOCKERHUB_TOKEN | 有目标仓库推送权限的访问令牌，不要放入代码或聊天 |

服务器执行 `uname -m` 可确认架构：`x86_64` 对应 `linux/amd64`，`aarch64` 对应 `linux/arm64`。配置缺失时工作流会明确失败，避免猜测目标架构或镜像名。

代码提交到 GitHub 后，打开 Actions → Publish application image 查看构建日志和 Summary。每次成功运行应显示 SHA 标签、digest 和平台。本地静态校验不等于已在 GitHub 成功推送；真实推送需要配置上述变量和凭据后执行。

手动部署工作流尚未接入；其后续输入应选取上述 digest，并验证该镜像对应的检查结果。

## Docker Hub 接入约束

- GitHub 托管 Runner 构建并推送，国内云服务器拉取并部署。
- 国内服务器计划通过代理访问 Docker Hub；实际可用性尚待服务器侧验证。
- Docker Engine 拉取镜像使用 daemon 的网络配置，仅配置交互终端代理不能保证拉取生效。修改代理所需的 daemon 重启应结合现网发布安排执行。
- 公共镜像仍需认证才能推送。凭据通过流水线 Secrets 配置，运行配置和凭据不得打入镜像。
- 公共拉取仍受 Docker Hub 使用限制；正式接入时验证认证、完整拉取及失败重试，不能仅用网页可访问作为验收依据。
- 参考：[Docker daemon 代理](https://docs.docker.com/engine/daemon/proxy/)、[Docker Hub 拉取限制](https://docs.docker.com/docker-hub/usage/pulls/)。
