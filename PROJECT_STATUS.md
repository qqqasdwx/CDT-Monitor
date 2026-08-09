# CDT-Monitor 项目状态回顾

更新时间：2026-08-09

## 当前定位

CDT-Monitor 是一个轻量级自托管控制台，用于监控阿里云 CDT 流量、管理 ECS 生命周期，并在达到配置阈值时执行保护动作。项目技术栈已经固定为 Go 后端、SQLite 数据库、React + Vite + TypeScript 前端。

当前代码已经搭起完整的前后端骨架和业务闭环原型，但外部云服务仍以 dry-run 和本地数据流为主。也就是说，系统已经能验证页面、API、数据库、调度器、权限和日志链路，但还不能直接视为可操作真实阿里云资源的生产版本。

## 已开发内容

### 工程骨架

- 后端已建立 Go module，入口位于 `backend/cmd/server/main.go`。
- 后端已包含配置加载、结构化日志、HTTP 路由、统一 JSON 响应、SQLite 连接和 migration 机制。
- 前端已建立 React + Vite + TypeScript 项目，包含 Tailwind CSS、基础 UI 组件、统一 API client 和多页面入口。
- 前端不是单页 React 路由应用，而是 Vite 多 HTML 入口：`login.html`、`dashboard.html`、`settings.html`，页面之间通过普通链接跳转。

### 认证和访问控制

- 后端要求配置 `CDTM_ADMIN_PASSWORD`，未配置时拒绝启动。
- 已实现管理员密码登录、`HttpOnly` 会话 Cookie、会话有效期和退出登录。
- `/api/v1/*` 管理接口默认需要登录，未登录返回统一 401 JSON 错误。
- 登录失败限速已实现，支持通过环境变量调整失败次数、统计窗口和锁定时间。
- 支持在可信反向代理后启用 `CDTM_TRUST_PROXY_HEADERS`，按真实客户端 IP 做限速。
- 阿里云事件 webhook 独立于登录 Cookie，使用 URL token 校验，便于云监控服务端回调。

### 数据库和持久化

SQLite migration 已覆盖主要业务表：

- 阿里云账号：`aliyun_accounts`
- ECS 实例配置：`ecs_instances`
- 定时任务：`scheduled_tasks`
- 流量快照：`traffic_snapshots`
- 费用快照：`cost_snapshots`
- 实例状态历史：`instance_status_history`
- 动作日志：`action_logs`
- 云事件：`cloud_events`
- 系统设置：`system_settings`
- 通知通道和通知日志：`notification_channels`、`notification_logs`
- Cloudflare 凭据和 DNS 记录：`cloudflare_credentials`、`dns_records`
- Telegram Bot 设置表：`telegram_bot_settings`

账号密钥、Cloudflare token 等敏感字段在服务层按加密字段建模，列表接口不会明文返回密钥。

### 核心业务闭环原型

- 已实现账号和实例配置 API。
- 已实现周期同步任务框架，支持可配置同步间隔。
- 已实现手动同步 API。
- 已实现 CDT 流量同步、ECS 状态同步、阈值判断和保护停机的服务层流程。
- 已实现手动启动、手动停止 API，并记录动作日志。
- 已实现自动保护停机后的状态标记，避免保活逻辑误启动。
- 已实现低频保活巡检、webhook 事件保活和冷却时间判断。

### 事件监听和日志

- 已实现阿里云云监控事件 webhook：`POST /api/webhooks/aliyun/events/:token`。
- 已实现 webhook token 存储和重置。
- 已实现事件字段解析、原始 payload 保存、事件幂等去重和处理状态记录。
- 已实现动作日志、云事件日志、流量趋势、实例状态历史、费用快照和日志清理相关 API。

### 前端页面

- 登录页已实现。
- Dashboard 页面已实现运行状态展示、实例筛选、手动同步、手动启动/停止、webhook URL、流量趋势、费用、日志和云事件展示。
- Settings 页面已实现账号配置、实例配置、系统设置、定时任务、DDNS、通知通道等配置入口。
- 前端 API 调用集中在 `frontend/src/api/client.ts`，请求默认携带 Cookie。

### 增强功能原型

- 系统设置已建模，包括默认同步频率、默认阈值、默认停机模式、默认保活开关、保活冷却时间和日志保留天数。
- 定时开机、定时停机、月初恢复已有数据模型、API 和调度入口。
- 通知通道已有 webhook、email、Telegram 三类配置模型和通知日志。
- Cloudflare DDNS 已有凭据、DNS 记录和本地更新日志流程。
- 高风险 ECS 操作已有评估入口和审计日志。
- Telegram 远程控制已有配置校验和命令处理入口。

## 已规划但尚未真正落地

### 真实阿里云 API

当前 `backend/internal/aliyun/client.go` 使用 `DryRunClient`：

- `QueryTraffic` 固定返回 0 流量。
- `QueryInstanceStatus` 固定返回 `Running`。
- `QueryAccountBalance` 固定返回 0 余额。
- `StartInstance` 和 `StopInstance` 只写 dry-run 日志，不调用真实 ECS API。

因此，CDT 查询、ECS 状态查询、ECS 启停和费用查询都还是接口形状和本地业务流程，尚未接入真实阿里云 SDK 或 OpenAPI。

### 真实通知发送

- 通用 webhook 通知已经能发送 HTTP POST。
- 邮件通知只完成配置模型，尚未接入 SMTP。
- Telegram 通知只完成配置模型，尚未接入 Bot API 发送消息。
- 通知事件链路已有，但真实可用性取决于后续补齐具体发送器。

### 真实 Cloudflare DDNS

- Cloudflare API Token 和 DNS 记录已建模。
- `UpdateDDNS` 当前只更新本地数据库记录并写动作日志。
- 尚未调用 Cloudflare DNS API 修改真实 DNS 记录。

### Telegram Bot 接入

- 已有配置 API 和命令处理函数。
- 尚未接入 Telegram webhook 或 polling。
- 当前 Telegram 配置主要用于校验和审计，尚未形成真正的远程控制入口。

### 高风险 ECS 操作

以下操作只有评估和审计日志，当前不会执行真实云资源变更：

- 创建 ECS
- 释放 ECS
- 更换公网 IP

这些功能需要在真实阿里云 API、权限边界、二次确认和回滚策略明确后再实现。

### 生产部署

- README 已提供本地开发命令。
- 尚未形成完整生产部署方案，例如 Dockerfile、docker-compose、静态前端构建托管、后端静态文件服务、systemd 或反向代理示例。
- HTTPS 当前假定由外部反向代理提供，后端只负责 Cookie 的 `Secure` 配置开关。

### 测试和验收

- 后端已有部分测试文件。
- 前端构建、lint 命令已配置。
- 真实云 API 集成、Webhook 真实回调、浏览器端关键流程、生产部署启动路径仍需要补充端到端验证。

## 需要讨论或确认的问题

### MVP 边界

第一版应先收敛到最小可用闭环：

1. 真实查询 CDT 流量。
2. 真实查询 ECS 状态。
3. 超阈值后真实停止 ECS。
4. 动作可审计，失败可排查。

费用、DDNS、Telegram、高风险 ECS 操作可以继续保留为后续增强，不建议阻塞第一版。

### 阿里云权限边界

接入真实阿里云前需要确认 RAM 权限范围：

- CDT 流量查询需要哪些只读权限。
- ECS Describe、Start、Stop 需要哪些最小权限。
- BssOpenApi 余额查询是否进入第一版。
- 是否为开发、测试、生产使用不同 AccessKey。

### 安全策略

内置密码登录、HttpOnly Cookie 和登录失败限速已经实现，作为轻量自托管工具是合理的基础方案。公网部署前仍需要确认：

- 是否只使用内置登录。
- 是否叠加 Cloudflare Access。
- 是否后续增加 TOTP 二次验证。
- 反向代理是否会覆盖客户端伪造的 `X-Forwarded-For` / `X-Real-IP`。
- Webhook 是否需要额外签名校验、来源校验或更细的 token 轮换策略。

### 部署形态

需要确认最终部署方式：

- 继续 Go + SQLite 单体自托管。
- 后端是否同时托管前端静态文件。
- 是否提供 Docker Compose。
- 是否需要迁移到 Cloudflare Worker + D1。

当前项目包含定时任务、SQLite、本地密钥文件、后台调度和潜在真实云 API 调用，继续使用 Go 单体会更直接。Worker 方案需要重新评估定时任务、数据库、密钥管理、云 API SDK 兼容性和前后端部署边界。

### 通知优先级

需要决定第一版通知方式：

- 只做通用 webhook。
- 优先做 Telegram。
- 优先做邮件 SMTP。
- 同时支持多通道但只保证 webhook 生产可用。

从当前实现看，webhook 是最快可以生产化的通知通道。

### 高风险操作是否进入第一版

创建 ECS、释放 ECS、换公网 IP、改 DNS 都会改变真实云资源，风险明显高于查询流量和停止实例。需要确认：

- 是否完全延后。
- 是否只做只读预览和审计。
- 是否要求二次确认、冷却时间、权限分离或额外管理员确认。

### 任务清单状态修正

`TASKLIST.md` 中许多阶段已经标记为完成，但实际含义更接近“代码结构和原型链路已完成”。在接入真实阿里云、Cloudflare、SMTP、Telegram 之前，不应把这些项目理解为生产能力完成。

建议后续把任务状态拆成：

- 已完成：本地工程、API、数据库、前端页面和 dry-run 闭环。
- 待集成：真实外部服务调用。
- 待验收：真实环境端到端验证。

## 当前主要风险

- dry-run 客户端与真实云 API 行为可能存在差异，错误码、分页、限流、权限和状态流转都需要重新验证。
- 任务清单状态偏乐观，容易让接手者误以为真实云资源操作已经可用。
- 外部集成缺口较多，尤其是阿里云 SDK、Cloudflare DNS、SMTP、Telegram Bot。
- 公网暴露时，内置登录需要配合强密码、HTTPS、限速、可信反代配置和 webhook token 管理。
- 当前大量实现文件仍处于未跟踪状态，正式推进前需要整理 Git diff，避免混入无关改动。

## 推荐下一步

1. 修正 `TASKLIST.md`，把“原型完成”和“生产可用”区分开。
2. 先接入真实阿里云只读能力：CDT 流量查询、ECS 状态查询、费用查询可后置。
3. 在只读同步稳定后，接入真实 ECS StopInstance，并保留明确 dry-run 开关。
4. 做一个最小生产部署路径：后端托管 API，前端静态构建，SQLite 数据目录和密钥目录明确持久化。
5. 保留内置密码登录作为基础安全方案，公网部署时优先叠加强密码、HTTPS、登录限速和可信反代配置；TOTP 和 Cloudflare Access 作为后续增强决策。
6. 暂缓高风险 ECS 操作、Cloudflare DDNS 和 Telegram 远程控制，等核心保护闭环真实可用后再逐项落地。
