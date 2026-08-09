import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, ArrowLeft, LogOut, RefreshCcw } from 'lucide-react'
import { api, ApiError, isUnauthorized, redirectToLogin } from '../../api/client'
import { Button } from '../../components/ui/button'
import { AccountForm, InstanceForm } from '../../features/dashboard/forms'
import { DDNSPanel } from '../../features/dashboard/ddns'
import { NotificationPanel } from '../../features/dashboard/notifications'
import { SchedulePanel } from '../../features/dashboard/schedules'
import { SettingsPanel } from '../../features/dashboard/settings'

export function SettingsPage() {
  const queryClient = useQueryClient()
  const statusQuery = useQuery({
    queryKey: ['status'],
    queryFn: api.status,
  })
  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: api.listSettings,
  })
  const scheduledTasksQuery = useQuery({
    queryKey: ['scheduled-tasks'],
    queryFn: api.listScheduledTasks,
  })
  const notificationChannelsQuery = useQuery({
    queryKey: ['notification-channels'],
    queryFn: api.listNotificationChannels,
  })
  const notificationLogsQuery = useQuery({
    queryKey: ['notification-logs'],
    queryFn: api.listNotificationLogs,
  })
  const cloudflareCredentialsQuery = useQuery({
    queryKey: ['cloudflare-credentials'],
    queryFn: api.listCloudflareCredentials,
  })
  const dnsRecordsQuery = useQuery({
    queryKey: ['dns-records'],
    queryFn: api.listDNSRecords,
  })

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['status'] }),
      queryClient.invalidateQueries({ queryKey: ['settings'] }),
      queryClient.invalidateQueries({ queryKey: ['scheduled-tasks'] }),
      queryClient.invalidateQueries({ queryKey: ['notification-channels'] }),
      queryClient.invalidateQueries({ queryKey: ['notification-logs'] }),
      queryClient.invalidateQueries({ queryKey: ['cloudflare-credentials'] }),
      queryClient.invalidateQueries({ queryKey: ['dns-records'] }),
    ])
  }

  const createAccount = useMutation({
    mutationFn: api.createAccount,
    onSuccess: invalidate,
  })
  const createInstance = useMutation({
    mutationFn: api.createInstance,
    onSuccess: invalidate,
  })
  const createScheduledTask = useMutation({
    mutationFn: api.createScheduledTask,
    onSuccess: invalidate,
  })
  const updateSettings = useMutation({
    mutationFn: api.updateSettings,
    onSuccess: invalidate,
  })
  const resetWebhookToken = useMutation({
    mutationFn: api.resetWebhookToken,
    onSuccess: invalidate,
  })
  const createNotificationChannel = useMutation({
    mutationFn: api.createNotificationChannel,
    onSuccess: invalidate,
  })
  const testNotificationChannel = useMutation({
    mutationFn: api.testNotificationChannel,
    onSuccess: invalidate,
  })
  const createCloudflareCredential = useMutation({
    mutationFn: api.createCloudflareCredential,
    onSuccess: invalidate,
  })
  const createDNSRecord = useMutation({
    mutationFn: api.createDNSRecord,
    onSuccess: invalidate,
  })
  const updateDDNS = useMutation({
    mutationFn: api.updateDDNS,
    onSuccess: invalidate,
  })
  const logout = useMutation({
    mutationFn: api.logout,
    onSettled: () => {
      redirectToLogin()
    },
  })

  const authError = [
    statusQuery.error,
    settingsQuery.error,
    scheduledTasksQuery.error,
    notificationChannelsQuery.error,
    notificationLogsQuery.error,
    cloudflareCredentialsQuery.error,
    dnsRecordsQuery.error,
    createAccount.error,
    createInstance.error,
    createScheduledTask.error,
    updateSettings.error,
    resetWebhookToken.error,
    createNotificationChannel.error,
    testNotificationChannel.error,
    createCloudflareCredential.error,
    createDNSRecord.error,
    updateDDNS.error,
  ].find(isUnauthorized)
  if (authError) {
    redirectToLogin()
    return null
  }

  const accounts = statusQuery.data?.accounts ?? []
  const instances = statusQuery.data?.instances ?? []
  const settings = settingsQuery.data ?? {}
  const scheduledTasks = scheduledTasksQuery.data ?? []
  const notificationChannels = notificationChannelsQuery.data ?? []
  const notificationLogs = notificationLogsQuery.data ?? []
  const cloudflareCredentials = cloudflareCredentialsQuery.data ?? []
  const dnsRecords = dnsRecordsQuery.data ?? []
  const error = firstError(
    statusQuery.error,
    settingsQuery.error,
    scheduledTasksQuery.error,
    notificationChannelsQuery.error,
    notificationLogsQuery.error,
    cloudflareCredentialsQuery.error,
    dnsRecordsQuery.error,
    createAccount.error,
    createInstance.error,
    createScheduledTask.error,
    updateSettings.error,
    resetWebhookToken.error,
    createNotificationChannel.error,
    testNotificationChannel.error,
    createCloudflareCredential.error,
    createDNSRecord.error,
    updateDDNS.error,
  )

  return (
    <main className="min-h-screen bg-[#f6f8f9]">
      <header className="border-b border-[#dbe3e6] bg-white">
        <div className="mx-auto flex max-w-7xl flex-col gap-4 px-4 py-5 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[#172026]">配置管理</h1>
            <p className="mt-1 text-sm text-[#5a6d74]">账号、实例、通知、DDNS 和系统参数</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="secondary" onClick={() => invalidate()}>
              <RefreshCcw className="h-4 w-4" />
              刷新
            </Button>
            <a href="/dashboard.html">
              <Button type="button" variant="secondary">
                <ArrowLeft className="h-4 w-4" />
                控制台
              </Button>
            </a>
            <Button onClick={() => logout.mutate()} disabled={logout.isPending} variant="ghost">
              <LogOut className="h-4 w-4" />
              退出
            </Button>
          </div>
        </div>
      </header>

      <section className="mx-auto grid max-w-7xl gap-5 px-4 py-5 sm:px-6">
        {error ? <ErrorBanner message={error} /> : null}
        <div className="grid gap-5 lg:grid-cols-2">
          <AccountForm
            onSubmit={async (input) => {
              await createAccount.mutateAsync(input)
            }}
            isSubmitting={createAccount.isPending}
          />
          <InstanceForm
            accounts={accounts}
            onSubmit={async (input) => {
              await createInstance.mutateAsync(input)
            }}
            isSubmitting={createInstance.isPending}
          />
        </div>
        <SettingsPanel
          settings={settings}
          onSave={async (input) => {
            await updateSettings.mutateAsync(input)
          }}
          onResetWebhookToken={() => resetWebhookToken.mutate()}
          isSaving={updateSettings.isPending}
          isResetting={resetWebhookToken.isPending}
        />
        <SchedulePanel
          instances={instances}
          tasks={scheduledTasks}
          onCreate={async (input) => {
            await createScheduledTask.mutateAsync(input)
          }}
          isSubmitting={createScheduledTask.isPending}
        />
        <DDNSPanel
          credentials={cloudflareCredentials}
          records={dnsRecords}
          onCreateCredential={async (input) => {
            await createCloudflareCredential.mutateAsync(input)
          }}
          onCreateRecord={async (input) => {
            await createDNSRecord.mutateAsync(input)
          }}
          onUpdate={(publicIp) => updateDDNS.mutate(publicIp)}
          isSubmitting={createCloudflareCredential.isPending || createDNSRecord.isPending}
        />
        <NotificationPanel
          channels={notificationChannels}
          logs={notificationLogs}
          onCreate={async (input) => {
            await createNotificationChannel.mutateAsync(input)
          }}
          onTest={(id) => testNotificationChannel.mutate(id)}
          isSubmitting={createNotificationChannel.isPending}
        />
      </section>
    </main>
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
