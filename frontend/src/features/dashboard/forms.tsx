import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { KeyRound, Plus, Server } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Input, Select } from '../../components/ui/input'
import type { Account, AccountInput, InstanceInput } from '../../types/api'

const accountSchema = z.object({
  accessKeyId: z.string().min(1, 'AccessKey ID 不能为空'),
  accessKeySecret: z.string().min(1, 'AccessKey Secret 不能为空'),
  region: z.string().min(1, '区域不能为空'),
  name: z.string(),
  enabled: z.boolean(),
})

const instanceSchema = z.object({
  accountId: z.string().min(1, '请选择账号'),
  instanceId: z.string().min(1, '实例 ID 不能为空'),
  name: z.string(),
  trafficLimitGb: z.coerce.number().positive('流量阈值必须大于 0'),
  stopMode: z.enum(['KeepCharging', 'StopCharging']),
  enabled: z.boolean(),
  protectionEnabled: z.boolean(),
  keepaliveEnabled: z.boolean(),
  keepaliveCooldownSeconds: z.coerce.number().min(0, '冷却时间不能小于 0'),
  monthlyRestoreEnabled: z.boolean(),
})

type AccountFormValues = z.infer<typeof accountSchema>
type InstanceFormInput = z.input<typeof instanceSchema>
type InstanceFormValues = z.output<typeof instanceSchema>

type AccountFormProps = {
  onSubmit: (input: AccountInput) => Promise<void>
  isSubmitting: boolean
}

export function AccountForm({ onSubmit, isSubmitting }: AccountFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<AccountFormValues>({
    resolver: zodResolver(accountSchema),
    defaultValues: {
      accessKeyId: '',
      accessKeySecret: '',
      region: 'cn-hangzhou',
      name: '',
      enabled: true,
    },
  })

  async function submit(values: AccountFormValues) {
    await onSubmit(values)
    reset()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRound className="h-4 w-4" />
          阿里云账号
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form className="grid gap-3" onSubmit={handleSubmit(submit)}>
          <Field label="AccessKey ID" error={errors.accessKeyId?.message}>
            <Input autoComplete="off" {...register('accessKeyId')} />
          </Field>
          <Field label="AccessKey Secret" error={errors.accessKeySecret?.message}>
            <Input type="password" autoComplete="new-password" {...register('accessKeySecret')} />
          </Field>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="区域" error={errors.region?.message}>
              <Input {...register('region')} />
            </Field>
            <Field label="备注">
              <Input placeholder="生产账号" {...register('name')} />
            </Field>
          </div>
          <label className="flex items-center gap-2 text-sm text-[#40555c]">
            <input type="checkbox" className="h-4 w-4 accent-[#0f766e]" {...register('enabled')} />
            启用账号
          </label>
          <Button type="submit" disabled={isSubmitting}>
            <Plus className="h-4 w-4" />
            保存账号
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

type InstanceFormProps = {
  accounts: Account[]
  onSubmit: (input: InstanceInput) => Promise<void>
  isSubmitting: boolean
}

export function InstanceForm({ accounts, onSubmit, isSubmitting }: InstanceFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<InstanceFormInput, unknown, InstanceFormValues>({
    resolver: zodResolver(instanceSchema),
    defaultValues: {
      accountId: '',
      instanceId: '',
      name: '',
      trafficLimitGb: 100,
      stopMode: 'StopCharging',
      enabled: true,
      protectionEnabled: true,
      keepaliveEnabled: false,
      keepaliveCooldownSeconds: 300,
      monthlyRestoreEnabled: false,
    },
  })

  async function submit(values: InstanceFormValues) {
    await onSubmit({
      accountId: values.accountId,
      instanceId: values.instanceId,
      name: values.name,
      trafficLimitBytes: Math.round(values.trafficLimitGb * 1024 * 1024 * 1024),
      stopMode: values.stopMode,
      enabled: values.enabled,
      protectionEnabled: values.protectionEnabled,
      keepaliveEnabled: values.keepaliveEnabled,
      keepaliveCooldownSeconds: values.keepaliveCooldownSeconds,
      monthlyRestoreEnabled: values.monthlyRestoreEnabled,
    })
    reset()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Server className="h-4 w-4" />
          ECS 实例
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form className="grid gap-3" onSubmit={handleSubmit(submit)}>
          <Field label="账号" error={errors.accountId?.message}>
            <Select {...register('accountId')}>
              <option value="">选择账号</option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name || account.accessKeyId} / {account.region}
                </option>
              ))}
            </Select>
          </Field>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="实例 ID" error={errors.instanceId?.message}>
              <Input placeholder="i-..." {...register('instanceId')} />
            </Field>
            <Field label="名称">
              <Input placeholder="业务节点" {...register('name')} />
            </Field>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="流量阈值 GB" error={errors.trafficLimitGb?.message}>
              <Input type="number" step="0.1" min="0" {...register('trafficLimitGb')} />
            </Field>
            <Field label="停机模式" error={errors.stopMode?.message}>
              <Select {...register('stopMode')}>
                <option value="StopCharging">节省停机</option>
                <option value="KeepCharging">普通停机</option>
              </Select>
            </Field>
          </div>
          <div className="grid gap-2 text-sm text-[#40555c] sm:grid-cols-4">
            <label className="flex items-center gap-2">
              <input type="checkbox" className="h-4 w-4 accent-[#0f766e]" {...register('enabled')} />
              启用实例
            </label>
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                className="h-4 w-4 accent-[#0f766e]"
                {...register('protectionEnabled')}
              />
              启用保护
            </label>
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                className="h-4 w-4 accent-[#0f766e]"
                {...register('keepaliveEnabled')}
              />
              启用保活
            </label>
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                className="h-4 w-4 accent-[#0f766e]"
                {...register('monthlyRestoreEnabled')}
              />
              月初恢复
            </label>
          </div>
          <Field label="保活冷却时间 秒" error={errors.keepaliveCooldownSeconds?.message}>
            <Input type="number" min="0" {...register('keepaliveCooldownSeconds')} />
          </Field>
          <Button type="submit" disabled={isSubmitting || accounts.length === 0}>
            <Plus className="h-4 w-4" />
            保存实例
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

type FieldProps = {
  label: string
  error?: string
  children: React.ReactNode
}

function Field({ label, error, children }: FieldProps) {
  return (
    <label className="grid gap-1 text-sm font-medium text-[#263a40]">
      <span>{label}</span>
      {children}
      {error ? <span className="text-xs font-normal text-[#b42318]">{error}</span> : null}
    </label>
  )
}
