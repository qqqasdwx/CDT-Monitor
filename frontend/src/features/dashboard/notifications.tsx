import { zodResolver } from '@hookform/resolvers/zod'
import { Send } from 'lucide-react'
import { useForm, useWatch } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Input, Select } from '../../components/ui/input'
import { formatTime } from '../../lib/utils'
import type {
  NotificationChannel,
  NotificationChannelInput,
  NotificationLog,
} from '../../types/api'

const notificationSchema = z.object({
  name: z.string().min(1, '名称不能为空'),
  type: z.enum(['webhook', 'email', 'telegram']),
  url: z.string(),
  enabled: z.boolean(),
})

type NotificationValues = z.infer<typeof notificationSchema>

export function NotificationPanel({
  channels,
  logs,
  onCreate,
  onTest,
  isSubmitting,
}: {
  channels: NotificationChannel[]
  logs: NotificationLog[]
  onCreate: (input: NotificationChannelInput) => Promise<void>
  onTest: (id: string) => void
  isSubmitting: boolean
}) {
  const { register, handleSubmit, reset, formState, control } = useForm<NotificationValues>({
    resolver: zodResolver(notificationSchema),
    defaultValues: { name: '', type: 'webhook', url: '', enabled: true },
  })
  const selectedType = useWatch({ control, name: 'type' })

  async function submit(values: NotificationValues) {
    await onCreate({
      name: values.name,
      type: values.type,
      enabled: values.enabled,
      config: values.url ? { url: values.url } : {},
    })
    reset()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>通知通道</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-5">
        <form
          className="grid gap-3 md:grid-cols-[1fr_140px_1.4fr_auto]"
          onSubmit={handleSubmit(submit)}
        >
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            名称
            <Input {...register('name')} />
            {formState.errors.name ? (
              <span className="text-xs font-normal text-[#b42318]">
                {formState.errors.name.message}
              </span>
            ) : null}
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            类型
            <Select {...register('type')}>
              <option value="webhook">Webhook</option>
              <option value="email">Email</option>
              <option value="telegram">Telegram</option>
            </Select>
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            {selectedType === 'webhook' ? 'Webhook URL' : '配置'}
            <Input
              placeholder={
                selectedType === 'webhook'
                  ? 'https://example.com/webhook'
                  : '保存后测试会记录未配置发送器'
              }
              {...register('url')}
            />
          </label>
          <div className="flex items-end gap-3">
            <label className="mb-2 flex items-center gap-2 text-sm text-[#40555c]">
              <input type="checkbox" className="h-4 w-4 accent-[#0f766e]" {...register('enabled')} />
              启用
            </label>
            <Button type="submit" disabled={isSubmitting}>
              <Send className="h-4 w-4" />
              保存
            </Button>
          </div>
        </form>

        <div className="grid gap-3">
          {channels.length === 0 ? (
            <p className="text-sm text-[#687b82]">暂无通知通道</p>
          ) : (
            channels.map((channel) => (
              <div
                key={channel.id}
                className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm"
              >
                <div>
                  <div className="font-medium text-[#172026]">
                    {channel.name} / {channel.type}
                  </div>
                  <div className="mt-1 text-xs text-[#687b82]">
                    {channel.enabled ? '已启用' : '已禁用'} · {formatTime(channel.createdAt)}
                  </div>
                </div>
                <Button variant="secondary" onClick={() => onTest(channel.id)}>
                  测试
                </Button>
              </div>
            ))
          )}
        </div>

        <div className="grid gap-2">
          <h3 className="text-sm font-semibold text-[#172026]">通知日志</h3>
          {logs.length === 0 ? (
            <p className="text-sm text-[#687b82]">暂无通知日志</p>
          ) : (
            logs.slice(0, 5).map((log) => (
              <div key={log.id} className="rounded-md border border-[#e4eaed] p-3 text-sm">
                <div className="flex flex-wrap justify-between gap-2">
                  <span className="font-medium text-[#172026]">
                    {log.eventType} / {log.result}
                  </span>
                  <span className="text-xs text-[#687b82]">{formatTime(log.createdAt)}</span>
                </div>
                {log.errorMessage ? <p className="mt-1 text-[#b42318]">{log.errorMessage}</p> : null}
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  )
}
