# CDT-Monitor

轻量级阿里云 CDT 流量保护与 ECS 管理工具。

当前 `dev` 镜像先提供一版临时可用的 CDT 保护守护进程：它基于 `reference/cdt.sh` 的核心逻辑，将配置改为环境变量，并在容器中循环执行。

## 临时可用镜像

镜像：

```bash
ghcr.io/qqqasdwx/cdt-monitor:guard
```

最小运行示例：

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

默认行为：

- 每 60 秒检查一次 CDT 总流量。
- 当流量低于阈值时，尝试启动 ECS。
- 当流量达到或超过阈值时，尝试停止 ECS。
- 默认停机模式为 `KeepCharging`。

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
| `CDT_CONTROL_MODE` | `keep_running` | `keep_running`：低于阈值启动，高于阈值停机；`protect_only`：只在超阈值时停机；`dry_run`：只打印动作 |
| `CDT_DRY_RUN` | `false` | 设置为 `true` 时只打印将要执行的动作，不调用启动/停机 |
| `ECS_STOPPED_MODE` | `KeepCharging` | 停机模式：`KeepCharging` 或 `StopCharging` |
| `ECS_FORCE_STOP` | `false` | 是否强制停机 |
| `RUN_ONCE` | `false` | 只执行一轮后退出 |
| `LOG_LEVEL` | `INFO` | 日志级别 |

## 只保护不自动启动

如果你不希望容器把手动停掉的实例重新拉起，使用：

```bash
-e CDT_CONTROL_MODE="protect_only"
```

此模式下，低于阈值不会启动实例；达到阈值时仍会尝试停机。

## 最小 RAM 权限

建议给临时 AK/SK 只授予最小权限：

```text
cdt:ListCdtInternetTraffic
ecs:DescribeInstances
ecs:StartInstance
ecs:StopInstance
```

## 注意事项

- 当前镜像是正式开发前的临时保护版本，还没有 Web UI、SQLite、通知和 webhook 保活。
- CDT 流量是账号维度数据，多实例共用同一账号流量额度时要谨慎设置阈值。
- 使用 `StopCharging` 可能导致非 EIP 的固定公网 IP 变化。
- 不要把 AccessKey 写进源码或提交到 Git。
