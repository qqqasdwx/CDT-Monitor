export type Account = {
  id: string
  accessKeyId: string
  region: string
  name: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export type Instance = {
  id: string
  accountId: string
  instanceId: string
  name: string
  trafficLimitBytes: number
  stopMode: 'KeepCharging' | 'StopCharging'
  enabled: boolean
  protectionEnabled: boolean
  keepaliveEnabled: boolean
  keepaliveCooldownSeconds: number
  monthlyRestoreEnabled: boolean
  lastStatus: string
  lastTrafficBytes: number
  lastSyncedAt?: string
  stoppedByProtectionAt?: string
  manualStopAt?: string
  createdAt: string
  updatedAt: string
  accountName?: string
  region?: string
}

export type ActionLog = {
  id: string
  accountId?: string
  instanceId?: string
  actionType: string
  triggerSource: string
  result: string
  reason: string
  trafficBytes?: number
  thresholdBytes?: number
  stopMode?: string
  errorMessage?: string
  createdAt: string
}

export type CloudEvent = {
  id: string
  eventId: string
  instanceId: string
  eventTime: string
  status: string
  rawPayload: string
  processStatus: string
  errorMessage?: string
  createdAt: string
}

export type NotificationChannel = {
  id: string
  name: string
  type: 'webhook' | 'email' | 'telegram'
  enabled: boolean
  configJson: string
  createdAt: string
  updatedAt: string
}

export type NotificationLog = {
  id: string
  channelId?: string
  eventType: string
  target: string
  result: string
  errorMessage?: string
  createdAt: string
}

export type NotificationChannelInput = {
  name: string
  type: 'webhook' | 'email' | 'telegram'
  enabled: boolean
  config: Record<string, string>
}

export type TrafficPoint = {
  bucketStart: string
  totalBytes: number
}

export type InstanceStatusPoint = {
  id: string
  instanceId: string
  status: string
  observedAt: string
}

export type ScheduledTask = {
  id: string
  instanceConfigId: string
  action: 'start' | 'stop'
  runAt: string
  enabled: boolean
  lastRunAt?: string
  createdAt: string
  updatedAt: string
  instanceId?: string
}

export type ScheduledTaskInput = {
  instanceConfigId: string
  action: 'start' | 'stop'
  runAt: string
  enabled: boolean
}

export type SettingsInput = {
  defaultSyncIntervalSeconds: string
  defaultTrafficLimitBytes: string
  defaultStopMode: 'KeepCharging' | 'StopCharging'
  defaultKeepaliveEnabled: string
  keepaliveCooldownSeconds: string
  logRetentionDays: string
}

export type CostSnapshot = {
  id: string
  accountId: string
  accountName?: string
  availableAmount: number
  creditAmount: number
  currency: string
  collectedAt: string
}

export type CloudflareCredential = {
  id: string
  name: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export type DNSRecord = {
  id: string
  credentialId: string
  zoneId: string
  recordId: string
  name: string
  type: string
  currentValue: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export type Dashboard = {
  accounts: Account[] | null
  instances: Instance[] | null
  actionLogs: ActionLog[] | null
  webhookUrl: string
  generatedAt: string
}

export type SyncResult = {
  accountId: string
  trafficBytes: number
  periodStart: string
  periodEnd: string
  instances: Instance[] | null
  protectionLog: ActionLog[] | null
  errors: string[] | null
  metadata?: Record<string, unknown>
}

export type AuthSession = {
  authenticated: boolean
  expiresAt?: string
}

export type AccountInput = {
  accessKeyId: string
  accessKeySecret: string
  region: string
  name: string
  enabled: boolean
}

export type InstanceInput = {
  accountId: string
  instanceId: string
  name: string
  trafficLimitBytes: number
  stopMode: 'KeepCharging' | 'StopCharging'
  enabled: boolean
  protectionEnabled: boolean
  keepaliveEnabled: boolean
  keepaliveCooldownSeconds: number
  monthlyRestoreEnabled: boolean
}
