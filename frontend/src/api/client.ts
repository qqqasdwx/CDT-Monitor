import type {
  Account,
  AccountInput,
  ActionLog,
  CloudEvent,
  Dashboard,
  Instance,
  InstanceInput,
  NotificationChannel,
  NotificationChannelInput,
  NotificationLog,
  SyncResult,
  TrafficPoint,
  InstanceStatusPoint,
  ScheduledTask,
  ScheduledTaskInput,
  SettingsInput,
  CostSnapshot,
  CloudflareCredential,
  DNSRecord,
  AuthSession,
} from '../types/api'

type Envelope<T> = {
  data?: T
  error?: {
    code: string
    message: string
  }
}

export class ApiError extends Error {
  code: string
  status: number

  constructor(message: string, code: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  const payload = (await response.json().catch(() => ({}))) as Envelope<T>
  if (!response.ok) {
    throw new ApiError(
      payload.error?.message ?? '请求失败',
      payload.error?.code ?? 'request_failed',
      response.status,
    )
  }
  return payload.data as T
}

export function redirectToLogin() {
  const current = `${window.location.pathname}${window.location.search}${window.location.hash}`
  const target = current && current !== '/login.html' ? `?next=${encodeURIComponent(current)}` : ''
  window.location.assign(`/login.html${target}`)
}

export function isUnauthorized(error: unknown) {
  return error instanceof ApiError && error.status === 401
}

export const api = {
  health: () => request<{ status: string; time: string }>('/healthz'),
  login: (password: string) =>
    request<AuthSession>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  logout: () => request<AuthSession>('/api/auth/logout', { method: 'POST' }),
  session: () => request<AuthSession>('/api/auth/session'),
  status: () => request<Dashboard>('/api/v1/status'),
  createAccount: (input: AccountInput) =>
    request<Account>('/api/v1/accounts', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listAccounts: () => request<Account[]>('/api/v1/accounts'),
  createInstance: (input: InstanceInput) =>
    request<Instance>('/api/v1/instances', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listInstances: () => request<Instance[]>('/api/v1/instances'),
  syncNow: () => request<SyncResult>('/api/v1/sync', { method: 'POST' }),
  startInstance: (id: string) =>
    request<ActionLog>(`/api/v1/instances/${id}/start`, { method: 'POST' }),
  stopInstance: (id: string) =>
    request<ActionLog>(`/api/v1/instances/${id}/stop`, { method: 'POST' }),
  listActionLogs: () => request<ActionLog[]>('/api/v1/action-logs'),
  listCloudEvents: () => request<CloudEvent[]>('/api/v1/cloud-events'),
  listSettings: () => request<Record<string, string>>('/api/v1/settings'),
  updateSettings: (input: SettingsInput) =>
    request<Record<string, string>>('/api/v1/settings', {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  resetWebhookToken: () =>
    request<{ webhookUrl: string }>('/api/v1/webhook-token/reset', { method: 'POST' }),
  createNotificationChannel: (input: NotificationChannelInput) =>
    request<NotificationChannel>('/api/v1/notification-channels', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listNotificationChannels: () =>
    request<NotificationChannel[]>('/api/v1/notification-channels'),
  testNotificationChannel: (id: string) =>
    request<NotificationLog>(`/api/v1/notification-channels/${id}/test`, { method: 'POST' }),
  listNotificationLogs: () => request<NotificationLog[]>('/api/v1/notification-logs'),
  trafficTrend: () => request<TrafficPoint[]>('/api/v1/traffic-trend?bucket=hour&limit=48'),
  instanceStatusHistory: () =>
    request<InstanceStatusPoint[]>('/api/v1/instance-status-history?limit=50'),
  cleanupLogs: () => request<ActionLog>('/api/v1/log-cleanup', { method: 'POST' }),
  listScheduledTasks: () => request<ScheduledTask[]>('/api/v1/scheduled-tasks'),
  createScheduledTask: (input: ScheduledTaskInput) =>
    request<ScheduledTask>('/api/v1/scheduled-tasks', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listCosts: () => request<CostSnapshot[]>('/api/v1/costs'),
  syncCosts: () => request<CostSnapshot[]>('/api/v1/costs/sync', { method: 'POST' }),
  listCloudflareCredentials: () =>
    request<CloudflareCredential[]>('/api/v1/cloudflare-credentials'),
  createCloudflareCredential: (input: {
    name: string
    apiToken: string
    enabled: boolean
  }) =>
    request<CloudflareCredential>('/api/v1/cloudflare-credentials', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listDNSRecords: () => request<DNSRecord[]>('/api/v1/dns-records'),
  createDNSRecord: (input: {
    credentialId: string
    zoneId: string
    recordId: string
    name: string
    type: string
    currentValue: string
    enabled: boolean
  }) =>
    request<DNSRecord>('/api/v1/dns-records', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateDDNS: (publicIp: string) =>
    request<ActionLog[]>('/api/v1/ddns/update', {
      method: 'POST',
      body: JSON.stringify({ publicIp }),
    }),
  evaluateRiskyOperation: (input: {
    instanceConfigId: string
    operation: string
    confirmText: string
  }) =>
    request<ActionLog>('/api/v1/risky-operations/evaluate', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  configureTelegram: (input: { botToken: string; allowedChatId: string; enabled: boolean }) =>
    request<ActionLog>('/api/v1/telegram/config', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  telegramCommand: (input: {
    chatId: string
    command: string
    instanceConfigId: string
    confirmText: string
  }) =>
    request<unknown>('/api/v1/telegram/command', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
}
