# Prompt: 为 Go 论坛后端构建 Nothing-Style Web 前端

## 任务

为现有的 Go 论坛后端构建完整 Web 前端。后端已实现 10 个 API 端点，前端采用 nothing-style 设计。

## Nothing-Style 设计规范

- 纯黑 (#000) 文字 / 纯白 (#fff) 背景
- 系统字体栈：-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif
- 蓝色链接 (#00e)，保持浏览器默认下划线
- 1px solid #000 边框用于表格、输入框、按钮
- 按钮：background: #eee，无圆角，无阴影
- <table> 用于结构化数据，border-collapse: collapse
- <label> 在上，<input> 在下，无 placeholder 替代 label
- @media (prefers-color-scheme: dark) 反转颜色
- CSS 总量严格控制在 200 行以内
- 零 class 名，仅通过 #header、#main 后代选择器限定样式
- 零 JS/CSS 框架，零构建步骤，纯 vanilla JS ES modules

## 目录结构（新增文件）

```
Forum/
├── web/
│   ├── index.html
│   ├── css/
│   │   └── style.css          (<200 行)
│   └── js/
│       ├── app.js              # Router + Header + Auth Guard
│       ├── api.js              # fetch() 封装 + JWT 注入
│       ├── auth.js             # 登录/注册/登出
│       ├── posts.js            # 帖子列表/详情/创建
│       ├── comments.js         # 评论列表/创建
│       ├── vote.js             # 投票按钮
│       ├── video.js            # 视频上传
│       └── utils.js            # escapeHtml, formatDate, el()
```

## 后端改动

1. 新建 internal/middleware/cors.go — CORS 中间件
2. 修改 internal/router/router.go — 注册 CORS、r.Static("/static", "./web")、SPA fallback NoRoute

## 页面路由

| Hash | 页面 | 需要认证 |
|------|------|----------|
| #/ | 帖子列表 | 是 |
| #/login | 登录 | 否 |
| #/signup | 注册 | 否 |
| #/post/:id | 帖子详情+评论+投票 | 是 |
| #/new-post | 创建帖子 | 是 |
| #/upload | 视频上传 | 是 |

## API 端点

Base: /api/v1
Auth: Authorization: Bearer <token> (JWT, 24h, key: "forum-secret")

1. POST /api/v1/signup — {username, password, email}
2. POST /api/v1/login — {username, password} → {msg, token}
3. POST /api/v1/post — {title, content, community_id} (auth)
4. GET /api/v1/post/:id — 返回 Post 对象 (auth)
5. GET /api/v1/posts?page=1&size=10 — 返回 Post[] (auth)
6. POST /api/v1/vote — {post_id:"string", direction:"string"} direction∈{1,0,-1} 必须是 JSON string (auth)
7. POST /api/v1/comment — {content, post_id, parent_id} (auth)
8. GET /api/v1/post/:post_id/comments — 返回 Comment[] (auth)
9. POST /api/v1/video/upload — multipart: title + video file (auth)
10. DELETE /api/v1/video/:id (auth)

## JS 模块设计

### api.js
- token() 从 localStorage 读取 forum_token
- request(method, path, body) 自动注入 Authorization header
- FormData 不设 Content-Type
- 401 → 清除 token → 跳转 #/login
- 429 → 抛出 "请求过于频繁，请稍候"
- !res.ok → 抛出 res.msg

### app.js
- hashchange 事件驱动路由
- 解析 hash 提取路由和参数
- 认证页面白名单：#/login, #/signup
- 未认证跳转 #/login
- 渲染 Header（根据登录态显示不同导航链接）

### utils.js
- escapeHtml(str): 防止 XSS
- formatDate(iso): 格式化日期
- el(tag, attrs, ...children): DOM 创建辅助

## 实施顺序

1. cors.go + router.go 修改
2. index.html + style.css
3. api.js + utils.js
4. app.js + auth.js
5. posts.js
6. comments.js + vote.js
7. video.js
8. 暗色模式 + 边界状态处理

## 验证

- go run ./cmd/main.go 启动服务
- 浏览器访问 http://localhost:端口
- 测试完整流程：注册→登录→创建帖子→投票→评论→上传视频
- 测试 401 拦截、429 限流提示、XSS 防御、暗色模式
