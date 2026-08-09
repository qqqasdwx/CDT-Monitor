# CDT-Monitor

轻量级自托管控制台，用于监控阿里云 CDT 流量、管理 ECS 生命周期，并在达到流量保护阈值时执行停机保护动作。

## 技术栈

- 前端：React + Vite + TypeScript + Tailwind CSS
- UI：shadcn/ui 风格基础组件
- 后端：Go
- 数据库：SQLite

## 本地开发

后端：

```bash
cd backend
go test ./...
CDTM_ADMIN_PASSWORD='替换成自己的本地管理密码' \
CDTM_PORT=8080 \
CDTM_DATABASE_PATH=../data/cdt-monitor.sqlite \
CDTM_SECRET_KEY_PATH=../data/secret.key \
CDTM_SESSION_KEY_PATH=../data/session.key \
go run ./cmd/server
```

前端：

```bash
cd frontend
npm install
npm run dev
```

前端开发服务默认通过 Vite proxy 访问 `http://localhost:8080` 的后端 API。

访问入口：

- `http://localhost:5173/login.html`：登录页
- `http://localhost:5173/dashboard.html`：运行控制台
- `http://localhost:5173/settings.html`：配置管理

## 配置

后端支持以下环境变量：

- `CDTM_PORT`：HTTP 端口，默认 `8080`
- `CDTM_DATABASE_PATH`：SQLite 数据库路径，默认 `data/cdt-monitor.sqlite`
- `CDTM_SECRET_KEY_PATH`：本地加密密钥文件路径，默认 `data/secret.key`
- `CDTM_ADMIN_PASSWORD`：管理员登录密码，必须设置；未设置时后端拒绝启动
- `CDTM_SESSION_KEY_PATH`：会话签名密钥文件路径，默认 `data/session.key`
- `CDTM_SESSION_TTL`：登录会话有效期，默认 `12h`
- `CDTM_SECURE_SESSION_COOKIE`：是否给会话 Cookie 设置 `Secure`，HTTPS 部署时应设为 `true`
- `CDTM_LOGIN_MAX_FAILURES`：单个来源在窗口期内允许的登录失败次数，默认 `5`
- `CDTM_LOGIN_WINDOW`：登录失败统计窗口，默认 `15m`
- `CDTM_LOGIN_LOCKOUT`：触发限速后的锁定时间，默认 `15m`
- `CDTM_TRUST_PROXY_HEADERS`：是否信任 `X-Forwarded-For` / `X-Real-IP` 作为客户端 IP，默认 `false`
- `CDTM_SYNC_INTERVAL`：周期同步间隔，默认 `5m`
- `CDTM_KEEPALIVE_INTERVAL`：保活兜底巡检间隔，默认 `10m`
- `CDTM_LOG_LEVEL`：日志级别，支持 `debug`、`info`、`warn`、`error`

## 访问控制

管理接口默认不开放。所有 `/api/v1/*` 接口都需要先通过 `POST /api/auth/login` 登录，后端设置 `HttpOnly` 会话 Cookie 后才允许访问。前端不会把密码或会话令牌写入 `localStorage`。

登录接口带有失败限速：同一来源默认 15 分钟内失败 5 次会锁定 15 分钟，响应状态为 `429` 并包含 `Retry-After`。如果部署在可信反向代理后面，且反代会覆盖客户端传入的转发头，可以设置 `CDTM_TRUST_PROXY_HEADERS=true`，让限速按真实客户端 IP 统计。

阿里云云监控事件入口 `POST /api/webhooks/aliyun/events/:token` 不依赖登录 Cookie，用 URL 中的 webhook token 独立校验，便于云监控服务端回调。

前端不是单个 React 路由应用。Vite 构建包含多个 HTML 入口：`login.html`、`dashboard.html`、`settings.html`，页面之间通过普通链接跳转。

## 当前实现状态

已实现基础工程、SQLite migration、密码登录与会话保护、账号和实例配置 API、周期同步、手动同步、阈值保护判断、手动启动/停止 API、动作日志、云监控事件 webhook、Stopped 事件保活、低频保活巡检、通知通道、历史趋势、日志清理、定时开关机和月初恢复，以及对应的多页面前端入口。

当前阿里云客户端处于 dry-run 模式，不会调用真实云 API。它用于验证系统闭环、前端交互、数据库迁移和动作日志。接入真实阿里云 CDT/ECS SDK 前，开发环境默认不会操作真实 ECS 资源。

## 阿里云云监控事件订阅

在控制台复制页面展示的 Webhook URL，并在阿里云云监控中创建系统事件订阅：

- 产品：云服务器 ECS
- 事件类型：状态通知
- 事件名称：实例状态改变通知
- 通知方式：Webhook
- 事件内容：需要包含事件 ID、实例 ID、事件时间和实例状态字段

后端会校验 URL 中的 token，重复事件会被去重。收到 `Stopped` 状态后，系统会再次查询 ECS 实例状态，只有在实例启用保活、未被流量保护停机、未被手动停机且未命中冷却时间时才会启动实例。

## 费用接口边界

费用状态使用阿里云费用与成本 BssOpenApi 的账户余额方向建模，核心接口为 `QueryAccountBalance`；账单明细后续可扩展到 `QueryAccountBill` 或 `QueryInstanceBill`。当前实现保留 dry-run 客户端，不会调用真实费用 API。费用同步失败只写入动作日志，不影响 CDT 流量保护和 ECS 控制闭环。

## 验证命令

```bash
cd backend && go test ./...
cd frontend && npm run lint && npm run build
```
