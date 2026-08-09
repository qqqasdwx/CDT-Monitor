import { zodResolver } from '@hookform/resolvers/zod'
import { RotateCcw, Save } from 'lucide-react'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Input, Select } from '../../components/ui/input'
import type { SettingsInput } from '../../types/api'

const schema = z.object({
  defaultSyncIntervalSeconds: z.string().min(1),
  defaultTrafficLimitBytes: z.string().min(1),
  defaultStopMode: z.enum(['KeepCharging', 'StopCharging']),
  defaultKeepaliveEnabled: z.enum(['true', 'false']),
  keepaliveCooldownSeconds: z.string().min(1),
  logRetentionDays: z.string().min(1),
})

type Values = z.infer<typeof schema>

export function SettingsPanel({
  settings,
  onSave,
  onResetWebhookToken,
  isSaving,
  isResetting,
}: {
  settings: Record<string, string>
  onSave: (input: SettingsInput) => Promise<void>
  onResetWebhookToken: () => void
  isSaving: boolean
  isResetting: boolean
}) {
  const { register, handleSubmit, reset } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: normalize(settings),
  })

  useEffect(() => {
    reset(normalize(settings))
  }, [settings, reset])

  async function submit(values: Values) {
    await onSave(values)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>系统设置</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4">
        <form className="grid gap-3 md:grid-cols-3" onSubmit={handleSubmit(submit)}>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            同步频率 秒
            <Input type="number" min="1" {...register('defaultSyncIntervalSeconds')} />
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            默认流量阈值 字节
            <Input type="number" min="1" {...register('defaultTrafficLimitBytes')} />
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            默认停机模式
            <Select {...register('defaultStopMode')}>
              <option value="StopCharging">节省停机</option>
              <option value="KeepCharging">普通停机</option>
            </Select>
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            默认保活
            <Select {...register('defaultKeepaliveEnabled')}>
              <option value="false">关闭</option>
              <option value="true">开启</option>
            </Select>
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            保活冷却 秒
            <Input type="number" min="1" {...register('keepaliveCooldownSeconds')} />
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            日志保留 天
            <Input type="number" min="1" {...register('logRetentionDays')} />
          </label>
          <div className="flex items-end gap-2 md:col-span-3">
            <Button type="submit" disabled={isSaving}>
              <Save className="h-4 w-4" />
              保存设置
            </Button>
            <Button type="button" variant="secondary" onClick={onResetWebhookToken} disabled={isResetting}>
              <RotateCcw className="h-4 w-4" />
              重置 Webhook Token
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function normalize(settings: Record<string, string>): Values {
  return {
    defaultSyncIntervalSeconds: settings.default_sync_interval_seconds ?? '300',
    defaultTrafficLimitBytes: settings.default_traffic_limit_bytes ?? '107374182400',
    defaultStopMode:
      settings.default_stop_mode === 'KeepCharging' ? 'KeepCharging' : 'StopCharging',
    defaultKeepaliveEnabled: settings.default_keepalive_enabled === 'true' ? 'true' : 'false',
    keepaliveCooldownSeconds: settings.keepalive_cooldown_seconds ?? '300',
    logRetentionDays: settings.log_retention_days ?? '30',
  }
}
