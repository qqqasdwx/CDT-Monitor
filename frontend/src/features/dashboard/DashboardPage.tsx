import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  CloudLightning,
  LogOut,
  Pause,
  Play,
  RefreshCcw,
  Settings,
} from 'lucide-react'
import { api, ApiError, isUnauthorized, redirectToLogin } from '../../api/client'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Select } from '../../components/ui/input'
import { formatBytes, formatTime } from '../../lib/utils'
import type { ActionLog, CloudEvent, Instance } from '../../types/api'
import { CostPanel } from './costs'
import { HistoryPanel } from './history'

const emptyInstances: Instance[] = []
const emptyActionLogs: ActionLog[] = []
const emptyCloudEvents: CloudEvent[] = []

export function DashboardPage() {
  const queryClient = useQueryClient()
  const [accountFilter, setAccountFilter] = useState('all')
  const [regionFilter, setRegionFilter] = useState('all')
  const [statusFilter, setStatusFilter] = useState('all')

  const statusQuery = useQuery({
    queryKey: ['status'],
    queryFn: api.status,
    refetchInterval: 30_000,
  })
  const cloudEventsQuery = useQuery({
    queryKey: ['cloud-events'],
    queryFn: api.listCloudEvents,
    refetchInterval: 30_000,
  })
  const trafficTrendQuery = useQuery({
    queryKey: ['traffic-trend'],
    queryFn: api.trafficTrend,
  })
  const statusHistoryQuery = useQuery({
    queryKey: ['status-history'],
    queryFn: api.instanceStatusHistory,
  })
  const costsQuery = useQuery({
    queryKey: ['costs'],
    queryFn: api.listCosts,
  })

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['status'] }),
      queryClient.invalidateQueries({ queryKey: ['cloud-events'] }),
      queryClient.invalidateQueries({ queryKey: ['traffic-trend'] }),
      queryClient.invalidateQueries({ queryKey: ['status-history'] }),
      queryClient.invalidateQueries({ queryKey: ['costs'] }),
    ])
  }

  const syncNow = useMutation({
    mutationFn: api.syncNow,
    onSuccess: invalidate,
  })
  const startInstance = useMutation({
    mutationFn: (id: string) => api.startInstance(id),
    onSuccess: invalidate,
  })
  const stopInstance = useMutation({
    mutationFn: (id: string) => api.stopInstance(id),
    onSuccess: invalidate,
  })
  const cleanupLogs = useMutation({
    mutationFn: api.cleanupLogs,
    onSuccess: invalidate,
  })
  const syncCosts = useMutation({
    mutationFn: api.syncCosts,
    onSuccess: invalidate,
  })
  const logout = useMutation({
    mutationFn: api.logout,
    onSettled: () => {
      redirectToLogin()
    },
  })

  const accounts = statusQuery.data?.accounts ?? []
  const instances = statusQuery.data?.instances ?? emptyInstances
  const filteredInstances = useMemo(
    () =>
      instances.filter((instance) => {
        if (accountFilter !== 'all' && instance.accountId !== accountFilter) {
          return false
        }
        if (regionFilter !== 'all' && instance.region !== regionFilter) {
          return false
        }
        if (statusFilter !== 'all' && instance.lastStatus !== statusFilter) {
          return false
        }
        return true
      }),
    [accountFilter, instances, regionFilter, statusFilter],
  )
  const regions = useMemo(
    () =>
      Array.from(
        new Set(
          instances
            .map((instance) => instance.region)
            .filter((region): region is string => Boolean(region)),
        ),
      ),
    [instances],
  )
  const statuses = useMemo(
    () => Array.from(new Set(instances.map((instance) => instance.lastStatus).filter(Boolean))),
    [instances],
  )
  const logs = statusQuery.data?.actionLogs ?? emptyActionLogs
  const cloudEvents = cloudEventsQuery.data ?? emptyCloudEvents
  const trafficTrend = trafficTrendQuery.data ?? []
  const statusHistory = statusHistoryQuery.data ?? []
  const costs = costsQuery.data ?? []
  const minimumThreshold = instances.reduce((min, instance) => {
    if (!instance.protectionEnabled) {
      return min
    }
    return min === 0 ? instance.trafficLimitBytes : Math.min(min, instance.trafficLimitBytes)
  }, 0)
  const webhookUrl = statusQuery.data?.webhookUrl ?? ''
  const authError = firstUnauthorized(
    statusQuery.error,
    cloudEventsQuery.error,
    trafficTrendQuery.error,
    statusHistoryQuery.error,
    costsQuery.error,
    syncNow.error,
    startInstance.error,
    stopInstance.error,
    cleanupLogs.error,
    syncCosts.error,
  )
  if (authError) {
    redirectToLogin()
    return null
  }

  const error = firstError(
    statusQuery.error,
    cloudEventsQuery.error,
    trafficTrendQuery.error,
    statusHistoryQuery.error,
    costsQuery.error,
    syncNow.error,
    startInstance.error,
    stopInstance.error,
    cleanupLogs.error,
    syncCosts.error,
  )

  return (
    <main className="min-h-screen bg-[#f6f8f9]">
      <header className="border-b border-[#dbe3e6] bg-white">
        <div className="mx-auto flex max-w-7xl flex-col gap-4 px-4 py-5 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[#172026]">CDT Monitor</h1>
            <p className="mt-1 text-sm text-[#5a6d74]">流量保护、实例状态和动作审计</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge ok={!statusQuery.isError} loading={statusQuery.isFetching} />
            <Button onClick={() => syncNow.mutate()} disabled={syncNow.isPending} variant="secondary">
              <RefreshCcw className="h-4 w-4" />
              手动同步
            </Button>
            <a href="/settings.html">
              <Button type="button" variant="secondary">
                <Settings className="h-4 w-4" />
                配置
              </Button>
            </a>
            <Button onClick={() => logout.mutate()} disabled={logout.isPending} variant="ghost">
              <LogOut className="h-4 w-4" />
              退出
            </Button>
          </div>
        </div>
      </header>

      <section className="mx-auto grid max-w-7xl content-start gap-5 px-4 py-5 sm:px-6">
        {error ? <ErrorBanner message={error} /> : null}
        <Summary instances={filteredInstances} logs={logs} />
        <WebhookPanel webhookUrl={webhookUrl} />
        <InstanceFilters
          accounts={accounts}
          regions={regions}
          statuses={statuses}
          accountFilter={accountFilter}
          regionFilter={regionFilter}
          statusFilter={statusFilter}
          onAccountChange={setAccountFilter}
          onRegionChange={setRegionFilter}
          onStatusChange={setStatusFilter}
        />
        <InstanceList
          instances={filteredInstances}
          onStart={(id) => startInstance.mutate(id)}
          onStop={(id) => stopInstance.mutate(id)}
          actionPending={startInstance.isPending || stopInstance.isPending}
        />
        <HistoryPanel
          traffic={trafficTrend}
          thresholdBytes={minimumThreshold}
          statusHistory={statusHistory}
          onCleanup={() => cleanupLogs.mutate()}
          cleanupPending={cleanupLogs.isPending}
        />
        <CostPanel costs={costs} onSync={() => syncCosts.mutate()} isSyncing={syncCosts.isPending} />
        <div className="grid gap-5 xl:grid-cols-2">
          <ActionLogList logs={logs} />
          <CloudEventList events={cloudEvents} />
        </div>
      </section>
    </main>
  )
}

function CloudEventList({ events }: { events: CloudEvent[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>最近云事件</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        {events.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无云监控事件</p>
        ) : (
          events.map((event) => (
            <div
              key={event.id}
              className="grid gap-1 rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium text-[#172026]">
                  {event.instanceId} / {event.status}
                </span>
                <span className="text-xs text-[#687b82]">{formatTime(event.createdAt)}</span>
              </div>
              <p className="text-[#40555c]">
                {event.eventId} · {event.processStatus}
              </p>
              {event.errorMessage ? <p className="text-[#b42318]">{event.errorMessage}</p> : null}
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}

function WebhookPanel({ webhookUrl }: { webhookUrl: string }) {
  const origin = typeof window === 'undefined' ? '' : window.location.origin
  const absoluteUrl = webhookUrl ? `${origin}${webhookUrl}` : ''
  return (
    <Card>
      <CardHeader>
        <CardTitle>云监控事件 Webhook</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <code className="min-h-9 flex-1 overflow-x-auto rounded-md border border-[#dbe3e6] bg-[#f8fafb] px-3 py-2 text-xs text-[#31525b]">
            {absoluteUrl || '后端连接后生成'}
          </code>
          <span className="text-xs text-[#687b82]">订阅 ECS 实例状态改变通知</span>
        </div>
      </CardContent>
    </Card>
  )
}

function InstanceFilters({
  accounts,
  regions,
  statuses,
  accountFilter,
  regionFilter,
  statusFilter,
  onAccountChange,
  onRegionChange,
  onStatusChange,
}: {
  accounts: Array<{ id: string; name: string; accessKeyId: string }>
  regions: string[]
  statuses: string[]
  accountFilter: string
  regionFilter: string
  statusFilter: string
  onAccountChange: (value: string) => void
  onRegionChange: (value: string) => void
  onStatusChange: (value: string) => void
}) {
  return (
    <Card>
      <CardContent className="grid gap-3 p-4 md:grid-cols-3">
        <Select value={accountFilter} onChange={(event) => onAccountChange(event.target.value)}>
          <option value="all">全部账号</option>
          {accounts.map((account) => (
            <option key={account.id} value={account.id}>
              {account.name || account.accessKeyId}
            </option>
          ))}
        </Select>
        <Select value={regionFilter} onChange={(event) => onRegionChange(event.target.value)}>
          <option value="all">全部区域</option>
          {regions.map((region) => (
            <option key={region} value={region}>
              {region}
            </option>
          ))}
        </Select>
        <Select value={statusFilter} onChange={(event) => onStatusChange(event.target.value)}>
          <option value="all">全部状态</option>
          {statuses.map((status) => (
            <option key={status} value={status}>
              {status}
            </option>
          ))}
        </Select>
      </CardContent>
    </Card>
  )
}

function StatusBadge({ ok, loading }: { ok: boolean; loading: boolean }) {
  return (
    <div className="inline-flex h-9 items-center gap-2 rounded-md border border-[#cbd5d9] bg-white px-3 text-sm text-[#40555c]">
      {ok ? (
        <CheckCircle2 className="h-4 w-4 text-[#0f766e]" />
      ) : (
        <AlertTriangle className="h-4 w-4 text-[#b42318]" />
      )}
      {loading ? '刷新中' : ok ? '后端已连接' : '后端不可用'}
    </div>
  )
}

function Summary({ instances, logs }: { instances: Instance[]; logs: ActionLog[] }) {
  const traffic = instances.reduce((max, item) => Math.max(max, item.lastTrafficBytes), 0)
  const protectedStops = logs.filter((log) => log.actionType === 'auto_stop_instance').length
  const running = instances.filter((item) => item.lastStatus === 'Running').length
  const items = [
    { label: '账号实例', value: `${instances.length}`, icon: CloudLightning },
    { label: '运行中', value: `${running}`, icon: Activity },
    { label: '当前 CDT 流量', value: formatBytes(traffic), icon: RefreshCcw },
    { label: '保护停机', value: `${protectedStops}`, icon: AlertTriangle },
  ]
  return (
    <div className="grid gap-3 md:grid-cols-4">
      {items.map((item) => {
        const Icon = item.icon
        return (
          <Card key={item.label}>
            <CardContent className="flex items-center justify-between p-4">
              <div>
                <p className="text-xs text-[#687b82]">{item.label}</p>
                <p className="mt-1 text-xl font-semibold text-[#172026]">{item.value}</p>
              </div>
              <Icon className="h-5 w-5 text-[#0f766e]" />
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}

function InstanceList({
  instances,
  onStart,
  onStop,
  actionPending,
}: {
  instances: Instance[]
  onStart: (id: string) => void
  onStop: (id: string) => void
  actionPending: boolean
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>实例状态</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full min-w-[760px] border-collapse text-left text-sm">
          <thead>
            <tr className="border-b border-[#e4eaed] bg-[#f8fafb] text-xs text-[#5a6d74]">
              <th className="px-5 py-3 font-medium">实例</th>
              <th className="px-5 py-3 font-medium">账号/区域</th>
              <th className="px-5 py-3 font-medium">流量</th>
              <th className="px-5 py-3 font-medium">状态</th>
              <th className="px-5 py-3 font-medium">同步</th>
              <th className="px-5 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {instances.length === 0 ? (
              <tr>
                <td className="px-5 py-8 text-center text-[#687b82]" colSpan={6}>
                  暂无实例配置
                </td>
              </tr>
            ) : (
              instances.map((instance) => (
                <tr key={instance.id} className="border-b border-[#edf2f4] last:border-0">
                  <td className="px-5 py-4">
                    <div className="font-medium text-[#172026]">{instance.name || instance.instanceId}</div>
                    <div className="mt-1 text-xs text-[#687b82]">{instance.instanceId}</div>
                  </td>
                  <td className="px-5 py-4 text-[#40555c]">
                    {instance.accountName || instance.accountId}
                    <div className="mt-1 text-xs text-[#687b82]">{instance.region}</div>
                  </td>
                  <td className="px-5 py-4">
                    <div className="text-[#172026]">{formatBytes(instance.lastTrafficBytes)}</div>
                    <div className="mt-1 text-xs text-[#687b82]">
                      阈值 {formatBytes(instance.trafficLimitBytes)}
                    </div>
                  </td>
                  <td className="px-5 py-4">
                    <span className="rounded-md bg-[#e8f5f3] px-2 py-1 text-xs font-medium text-[#0f766e]">
                      {instance.lastStatus}
                    </span>
                  </td>
                  <td className="px-5 py-4 text-[#40555c]">{formatTime(instance.lastSyncedAt)}</td>
                  <td className="px-5 py-4">
                    <div className="flex gap-2">
                      <Button variant="secondary" onClick={() => onStart(instance.id)} disabled={actionPending}>
                        <Play className="h-4 w-4" />
                        启动
                      </Button>
                      <Button variant="danger" onClick={() => onStop(instance.id)} disabled={actionPending}>
                        <Pause className="h-4 w-4" />
                        停止
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </CardContent>
    </Card>
  )
}

function ActionLogList({ logs }: { logs: ActionLog[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>最近动作日志</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        {logs.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无动作日志</p>
        ) : (
          logs.map((log) => (
            <div
              key={log.id}
              className="grid gap-1 rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium text-[#172026]">
                  {log.actionType} / {log.triggerSource}
                </span>
                <span className="text-xs text-[#687b82]">{formatTime(log.createdAt)}</span>
              </div>
              <p className="text-[#40555c]">{log.reason}</p>
              {log.errorMessage ? <p className="text-[#b42318]">{log.errorMessage}</p> : null}
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-2 rounded-md border border-[#fecdca] bg-[#fffbfa] p-3 text-sm text-[#b42318]">
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  )
}

function firstError(...errors: Array<unknown>): string | null {
  const error = errors.find(Boolean)
  if (!error) {
    return null
  }
  if (error instanceof ApiError || error instanceof Error) {
    return error.message
  }
  return '请求失败'
}

function firstUnauthorized(...errors: Array<unknown>) {
  return errors.find(isUnauthorized)
}
