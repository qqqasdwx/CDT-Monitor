# CDT Guard

这是 `cdt-guard` 分支专用的临时镜像说明。本分支不作为主线开发合并目标，只用于在完整 CDT-Monitor 开发完成前，先提供一个可运行的 CDT 流量保护守护进程。

镜像地址：

```bash
ghcr.io/qqqasdwx/cdt-monitor:guard
```

## 当前功能

这个镜像通过环境变量配置阿里云凭据、实例和流量阈值，并在容器中循环执行保护检查。

已支持：

- 定时查询阿里云 CDT 总流量。
- 查询指定 ECS 实例状态。
- 按流量阈值自动启动或停止 ECS。
- 支持单实例和多实例。
- 支持三种控制模式：`keep_running`、`protect_only`、`dry_run`。
- 支持普通停机和节省停机。
- 支持强制停机开关。
- 支持只执行一轮后退出，便于测试。
- 支持 Docker Compose 部署。
- 容器内置 heartbeat healthcheck。

默认行为：

```text
CDT 流量 < 阈值  -> 尝试启动 ECS
CDT 流量 >= 阈值 -> 尝试停止 ECS
```

## 不包含的功能

当前镜像只是临时保护版本，不包含完整 CDT-Monitor 功能。

- 没有 Web UI。
- 没有数据库。
- 没有账号管理页面。
- 没有历史图表。
- 没有邮件、Telegram、Webhook 通知。
- 没有云监控事件订阅 webhook 秒级保活。
- 没有定时开关机。
- 没有费用和余额查询。
- 没有 DDNS。
- 没有 ECS 创建、释放、换 IP。

## 快速运行

```bash
docker run -d \
  --name cdt-monitor \
  --restart unless-stopped \
  -e ALIYUN_ACCESS_KEY_ID="你的 AccessKey ID" \
  -e ALIYUN_ACCESS_KEY_SECRET="你的 AccessKey Secret" \
  -e ALIYUN_REGION_ID="cn-hongkong" \
  -e ECS_INSTANCE_ID="i-xxxxxxxxxxxxxxxxx" \
  -e CDT_TRAFFIC_THRESHOLD_GB="180" \
  ghcr.io/qqqasdwx/cdt-monitor:guard
```

查看日志：

```bash
docker logs -f cdt-monitor
```

停止：

```bash
docker rm -f cdt-monitor
```

## Docker Compose

本分支提供了 [compose.yaml](./compose.yaml) 示例。

建议在同目录创建 `.env`：

```env
ALIYUN_ACCESS_KEY_ID=你的 AccessKey ID
ALIYUN_ACCESS_KEY_SECRET=你的 AccessKey Secret
ALIYUN_REGION_ID=cn-hongkong
ECS_INSTANCE_ID=i-xxxxxxxxxxxxxxxxx
CDT_TRAFFIC_THRESHOLD_GB=180
CDT_CONTROL_MODE=protect_only
```

启动：

```bash
docker compose up -d
```

查看日志：

```bash
docker compose logs -f
```

更新镜像：

```bash
docker compose pull
docker compose up -d
```

## 环境变量

必填：

| 变量 | 说明 |
| --- | --- |
| `ALIYUN_ACCESS_KEY_ID` | 阿里云 AccessKey ID |
| `ALIYUN_ACCESS_KEY_SECRET` | 阿里云 AccessKey Secret |
| `ALIYUN_REGION_ID` | ECS 所在区域，例如 `cn-hongkong` |
| `ECS_INSTANCE_ID` | ECS 实例 ID |

可选：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ECS_INSTANCE_IDS` | 空 | 多实例 ID，英文逗号分隔；设置后优先于 `ECS_INSTANCE_ID` |
| `CDT_TRAFFIC_THRESHOLD_GB` | `180` | CDT 流量阈值，单位 GB |
| `CDT_CHECK_INTERVAL_SECONDS` | `60` | 检查间隔，最低 10 秒 |
| `CDT_CONTROL_MODE` | `keep_running` | 控制模式，见下文 |
| `CDT_DRY_RUN` | `false` | 设置为 `true` 时只打印动作，不真正启停 ECS |
| `ECS_STOPPED_MODE` | `StopCharging` | 停机模式：`StopCharging` 或 `KeepCharging` |
| `ECS_FORCE_STOP` | `false` | 是否强制停机 |
| `RUN_ONCE` | `false` | 只执行一轮后退出 |
| `LOG_LEVEL` | `INFO` | 日志级别 |

## 控制模式

### `keep_running`

默认模式。

```text
CDT 流量 < 阈值  -> 尝试启动 ECS
CDT 流量 >= 阈值 -> 尝试停止 ECS
```

适合希望实例在流量安全时自动保持运行的场景。

### `protect_only`

```text
CDT 流量 < 阈值  -> 不执行启动
CDT 流量 >= 阈值 -> 尝试停止 ECS
```

适合只想防超量、不想容器把手动停掉的实例重新拉起的场景。

### `dry_run`

```text
只打印计划动作，不真正启动或停止 ECS
```

适合第一次配置时验证 AK、区域、实例和阈值。

也可以使用：

```env
CDT_DRY_RUN=true
```

## 多实例

使用英文逗号分隔：

```env
ECS_INSTANCE_IDS=i-xxx1,i-xxx2,i-xxx3
```

如果同时设置了 `ECS_INSTANCE_IDS` 和 `ECS_INSTANCE_ID`，优先使用 `ECS_INSTANCE_IDS`。

## 停机模式

```env
ECS_STOPPED_MODE=StopCharging
```

默认模式。节省停机，释放计算资源并停止计算费用。注意：如果实例使用非 EIP 公网 IP，重启后公网 IP 可能变化。

```env
ECS_STOPPED_MODE=KeepCharging
```

普通停机。停止实例后保留计算资源，通常恢复更快，但仍可能产生相关费用。只有在明确需要保留计算资源或特定网络行为时再使用。

## RAM 权限

可以把本镜像需要的 CDT 和 ECS 权限写在一个自定义权限策略里。你截图里的页面就是正确位置：`RAM 访问控制 -> 权限管理 -> 权限策略 -> 创建权限策略 -> 脚本编辑`。

控制台入口：

- RAM 权限策略页面：https://ram.console.aliyun.com/policies
- 官方说明：https://help.aliyun.com/zh/ram/create-a-custom-policy

建议创建一个名为 `CDTGuardPolicy` 的自定义策略，把下面 JSON 粘贴到“脚本编辑”里：

### 完整策略

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cdt:List*",
        "cdt:Describe*",
        "cdt:Get*"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ecs:DescribeInstances",
        "ecs:StartInstances",
        "ecs:StopInstances"
      ],
      "Resource": [
        "acs:ecs:*:*:instance/*"
      ]
    }
  ]
}
```

这份策略适用于默认的 `keep_running` 模式，因为它需要在低于阈值时启动 ECS，在超过阈值时停止 ECS。

如果你使用 `protect_only` 模式，并且确认不需要自动启动实例，可以从策略里删除：

```json
"ecs:StartInstances"
```

如果想进一步收窄 ECS 资源范围，可以把：

```text
acs:ecs:*:*:instance/*
```

改成指定地域、账号和实例 ID：

```text
acs:ecs:cn-hongkong:<你的阿里云账号ID>:instance/i-xxxxxxxxxxxxxxxxx
```

创建完成后，到 RAM 用户或角色的“添加权限”页面，搜索并选择你刚创建的 `CDTGuardPolicy`。

如果只是临时快速跑通，也可以直接授权 `AliyunCDTFullAccess` + `AliyunECSFullAccess`，但权限明显更大，不建议长期使用。

## 测试配置

只执行一轮，并且不真正启停 ECS：

```bash
docker run --rm \
  -e RUN_ONCE=true \
  -e CDT_DRY_RUN=true \
  -e ALIYUN_ACCESS_KEY_ID="你的 AccessKey ID" \
  -e ALIYUN_ACCESS_KEY_SECRET="你的 AccessKey Secret" \
  -e ALIYUN_REGION_ID="cn-hongkong" \
  -e ECS_INSTANCE_ID="i-xxxxxxxxxxxxxxxxx" \
  -e CDT_TRAFFIC_THRESHOLD_GB="180" \
  ghcr.io/qqqasdwx/cdt-monitor:guard
```

## 注意事项

- CDT 流量是账号维度数据，不是单实例精确流量。多个实例共用同一账号额度时，要谨慎设置阈值。
- 当前守护进程依赖轮询，不是秒级事件监听。
- 阿里云 CDT 接口可能存在统计延迟，阈值不要贴着免费额度设置。
- 建议阈值留足缓冲，例如 200GB 免费额度可先设为 180GB。
- 不要把 AccessKey 写进源码、镜像或 Git。
- 这个分支只维护临时 guard 镜像；正式项目开发在 `dev` 分支继续。
