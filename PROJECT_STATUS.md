# CDT-Monitor 项目状态回顾

更新时间：2026-08-09

## 当前定位

CDT-Monitor 是一个轻量级自托管控制台，用于监控阿里云 CDT 流量、管理 ECS 生命周期，并在达到配置阈值时执行保护动作。项目技术栈已经固定为 Go 后端、SQLite 数据库、React + Vite + TypeScript 前端。

当前代码已经搭起完整的前后端骨架和业务闭环，并接入真实阿里云 CDT/ECS OpenAPI。系统仍需要使用专门的测试账号完成真实环境验收，未经验收前不应直接切换到 `live` 模式投入无人值守运行。

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

### 真实阿里云接入

- 已使用阿里云官方 OpenAPI SDK 接入 `ListCdtInternetTraffic` 和 `DescribeInstanceStatus`。
- CDT 流量按目标实例所属的国内或海外区域归类汇总，异常响应会明确失败，不会继续触发保护动作。
- 已接入真实 `StartInstance` 和 `StopInstance`，支持 `KeepCharging`、`StopCharging`。
- 已提供 `dry-run`、`read-only`、`live` 三档模式，默认 `dry-run`；`read-only` 会在后端阻断所有云资源变更。
- Dashboard 会显示当前运行模式，只读模式禁用启停按钮，真实模式手动操作需要再次确认。

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

### 生产部署

- 已提供多阶段 `Dockerfile`，构建 React 静态资源和 Go 二进制。
- Go 服务可通过 `CDTM_FRONTEND_DIR` 同时托管多页面前端和 API。
- 已提供 `docker-compose.yml` 和 `.env.example`，SQLite、加密密钥、会话密钥使用命名卷持久化。
- Compose 默认只绑定 `127.0.0.1`，并启用只读根文件系统、删除 Linux capabilities 和 `no-new-privileges`。

### 增强功能原型

- 系统设置已建模，包括默认同步频率、默认阈值、默认停机模式、默认保活开关、保活冷却时间和日志保留天数。
- 定时开机、定时停机、月初恢复已有数据模型、API 和调度入口。
- 通知通道已有 webhook、email、Telegram 三类配置模型和通知日志。
- Cloudflare DDNS 已有凭据、DNS 记录和本地更新日志流程。
- 高风险 ECS 操作已有评估入口和审计日志。
- Telegram 远程控制已有配置校验和命令处理入口。

## 已规划但尚未真正落地

### 真实费用查询

- BssOpenApi 余额模型和本地快照流程已经存在。
- 真实模式尚未调用 `QueryAccountBalance`，因为账号模型还没有中国站/国际站属性，不能可靠选择费用中心 endpoint。
- 真实模式会明确返回不支持错误，不会将余额伪造成 0。

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

### 测试和验收

- 后端测试覆盖核心业务、认证限速、CDT 响应聚合、ECS 状态解析、只读模式阻断和真实停机参数。
- 前端 lint 和生产构建已经通过。
- Docker 镜像和 Compose 配置已经在本机完成构建验证；临时容器的健康检查、静态多页面、登录 Cookie、受保护 API 和数据卷重建路径均已验证。
- 真实云 API、Webhook 回调和浏览器关键流程仍需要使用专门测试凭据完成端到端验证。

## 需要讨论或确认的问题

### MVP 边界

第一版代码已经收敛到以下最小闭环：

1. 真实查询 CDT 流量。
2. 真实查询 ECS 状态。
3. 超阈值后真实停止 ECS。
4. 动作可审计，失败可排查。

上述能力已经完成代码接入，但尚未完成真实账号验收。费用、DDNS、Telegram、高风险 ECS 操作继续保留为后续增强，不阻塞第一版。

### 阿里云权限边界

真实环境验收前需要确认 RAM 权限范围：

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

当前部署形态已确定为 Go + SQLite 单体自托管，由 Go 同时提供前端静态文件和 API，并通过 Docker Compose 管理。Cloudflare Worker + D1 不进入当前版本。

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

## 当前主要风险

- 真实阿里云调用尚未使用专门测试账号验收，错误码、权限、限流和 CDT 实际响应仍可能与参考数据不同。
- `live` 模式会允许调度器、保活和阈值保护操作真实 ECS，切换前必须先在 `read-only` 模式核对数据。
- 外部集成仍有缺口，包括 BssOpenApi、Cloudflare DNS、SMTP、Telegram Bot。
- 公网暴露时，内置登录需要配合强密码、HTTPS、限速、可信反代配置和 webhook token 管理。
- Webhook 真实回调和生产反向代理路径还没有端到端验收。

## 推荐下一步

1. 使用最小 RAM 权限的专门测试账号，在 `read-only` 模式验证 CDT 流量和 ECS 状态。
2. 使用非关键测试实例切换到 `live`，分别验证手动停止、保护停止和保活启动。
3. 验证阿里云云监控 Webhook 的真实 payload 和反向代理转发路径。
4. 明确账号中国站/国际站属性后接入 BssOpenApi；DDNS、SMTP、Telegram 继续后置。
