import { zodResolver } from '@hookform/resolvers/zod'
import { CalendarClock } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Input, Select } from '../../components/ui/input'
import { formatTime } from '../../lib/utils'
import type { Instance, ScheduledTask, ScheduledTaskInput } from '../../types/api'

const schema = z.object({
  instanceConfigId: z.string().min(1, '请选择实例'),
  action: z.enum(['start', 'stop']),
  runAt: z.string().min(1, '请选择执行时间'),
  enabled: z.boolean(),
})

type Values = z.infer<typeof schema>

export function SchedulePanel({
  instances,
  tasks,
  onCreate,
  isSubmitting,
}: {
  instances: Instance[]
  tasks: ScheduledTask[]
  onCreate: (input: ScheduledTaskInput) => Promise<void>
  isSubmitting: boolean
}) {
  const { register, handleSubmit, reset, formState } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { instanceConfigId: '', action: 'stop', runAt: '', enabled: true },
  })

  async function submit(values: Values) {
    await onCreate({
      ...values,
      runAt: new Date(values.runAt).toISOString(),
    })
    reset()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>定时开关机</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4">
        <form className="grid gap-3 md:grid-cols-[1fr_120px_220px_auto]" onSubmit={handleSubmit(submit)}>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            实例
            <Select {...register('instanceConfigId')}>
              <option value="">选择实例</option>
              {instances.map((instance) => (
                <option key={instance.id} value={instance.id}>
                  {instance.name || instance.instanceId}
                </option>
              ))}
            </Select>
            {formState.errors.instanceConfigId ? (
              <span className="text-xs font-normal text-[#b42318]">
                {formState.errors.instanceConfigId.message}
              </span>
            ) : null}
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            动作
            <Select {...register('action')}>
              <option value="stop">停止</option>
              <option value="start">启动</option>
            </Select>
          </label>
          <label className="grid gap-1 text-sm font-medium text-[#263a40]">
            执行时间
            <Input type="datetime-local" {...register('runAt')} />
          </label>
          <div className="flex items-end gap-3">
            <label className="mb-2 flex items-center gap-2 text-sm text-[#40555c]">
              <input type="checkbox" className="h-4 w-4 accent-[#0f766e]" {...register('enabled')} />
              启用
            </label>
            <Button type="submit" disabled={isSubmitting || instances.length === 0}>
              <CalendarClock className="h-4 w-4" />
              保存
            </Button>
          </div>
        </form>

        {tasks.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无定时任务</p>
        ) : (
          <div className="grid gap-2">
            {tasks.slice(0, 5).map((task) => (
              <div key={task.id} className="rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm">
                <div className="flex flex-wrap justify-between gap-2">
                  <span className="font-medium text-[#172026]">
                    {task.instanceId || task.instanceConfigId} / {task.action}
                  </span>
                  <span className="text-xs text-[#687b82]">{formatTime(task.runAt)}</span>
                </div>
                <p className="mt-1 text-[#40555c]">
                  {task.enabled ? '已启用' : '已禁用'}
                  {task.lastRunAt ? ` · 已执行 ${formatTime(task.lastRunAt)}` : ''}
                </p>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
