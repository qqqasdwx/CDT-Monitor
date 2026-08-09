CREATE TABLE aliyun_accounts (
  id TEXT PRIMARY KEY,
  access_key_id TEXT NOT NULL,
  access_key_secret_encrypted TEXT NOT NULL,
  region TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_aliyun_accounts_enabled ON aliyun_accounts(enabled);

CREATE TABLE ecs_instances (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES aliyun_accounts(id) ON DELETE CASCADE,
  instance_id TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  traffic_limit_bytes INTEGER NOT NULL,
  stop_mode TEXT NOT NULL DEFAULT 'KeepCharging',
  enabled INTEGER NOT NULL DEFAULT 1,
  protection_enabled INTEGER NOT NULL DEFAULT 1,
  keepalive_enabled INTEGER NOT NULL DEFAULT 0,
  keepalive_cooldown_seconds INTEGER NOT NULL DEFAULT 300,
  monthly_restore_enabled INTEGER NOT NULL DEFAULT 0,
  last_status TEXT NOT NULL DEFAULT 'Unknown',
  last_traffic_bytes INTEGER NOT NULL DEFAULT 0,
  last_synced_at TEXT,
  stopped_by_protection_at TEXT,
  manual_stop_at TEXT,
  last_keepalive_at TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(account_id, instance_id)
);

CREATE INDEX idx_ecs_instances_account_id ON ecs_instances(account_id);
CREATE INDEX idx_ecs_instances_enabled ON ecs_instances(enabled);
CREATE INDEX idx_ecs_instances_status ON ecs_instances(last_status);

CREATE TABLE scheduled_tasks (
  id TEXT PRIMARY KEY,
  instance_config_id TEXT NOT NULL REFERENCES ecs_instances(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  run_at TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  last_run_at TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_scheduled_tasks_due ON scheduled_tasks(enabled, run_at);

CREATE TABLE traffic_snapshots (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES aliyun_accounts(id) ON DELETE CASCADE,
  period_start TEXT NOT NULL,
  period_end TEXT NOT NULL,
  total_bytes INTEGER NOT NULL,
  collected_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_traffic_snapshots_account_collected ON traffic_snapshots(account_id, collected_at);

CREATE TABLE cost_snapshots (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES aliyun_accounts(id) ON DELETE CASCADE,
  available_amount REAL NOT NULL,
  credit_amount REAL NOT NULL,
  currency TEXT NOT NULL DEFAULT 'CNY',
  collected_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_cost_snapshots_account_collected ON cost_snapshots(account_id, collected_at);

CREATE TABLE instance_status_history (
  id TEXT PRIMARY KEY,
  instance_id TEXT NOT NULL,
  status TEXT NOT NULL,
  observed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_instance_status_history_instance_observed ON instance_status_history(instance_id, observed_at);

CREATE TABLE action_logs (
  id TEXT PRIMARY KEY,
  account_id TEXT REFERENCES aliyun_accounts(id) ON DELETE SET NULL,
  instance_id TEXT,
  action_type TEXT NOT NULL,
  trigger_source TEXT NOT NULL,
  result TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  traffic_bytes INTEGER,
  threshold_bytes INTEGER,
  stop_mode TEXT,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_action_logs_created_at ON action_logs(created_at);
CREATE INDEX idx_action_logs_instance_id ON action_logs(instance_id);
CREATE INDEX idx_action_logs_action_type ON action_logs(action_type);

CREATE TABLE cloud_events (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  event_time TEXT NOT NULL,
  status TEXT NOT NULL,
  raw_payload TEXT NOT NULL,
  process_status TEXT NOT NULL,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(event_id, instance_id, event_time)
);

CREATE INDEX idx_cloud_events_created_at ON cloud_events(created_at);
CREATE INDEX idx_cloud_events_instance_id ON cloud_events(instance_id);

CREATE TABLE system_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO system_settings (key, value) VALUES ('webhook_token', lower(hex(randomblob(24))));
INSERT INTO system_settings (key, value) VALUES ('default_sync_interval_seconds', '300');
INSERT INTO system_settings (key, value) VALUES ('default_traffic_limit_bytes', '107374182400');
INSERT INTO system_settings (key, value) VALUES ('default_stop_mode', 'StopCharging');
INSERT INTO system_settings (key, value) VALUES ('default_keepalive_enabled', 'false');
INSERT INTO system_settings (key, value) VALUES ('keepalive_cooldown_seconds', '300');
INSERT INTO system_settings (key, value) VALUES ('log_retention_days', '30');
INSERT INTO system_settings (key, value) VALUES ('last_monthly_restore_month', '');

CREATE TABLE notification_channels (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  config_json TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_notification_channels_enabled ON notification_channels(enabled);
CREATE INDEX idx_notification_channels_type ON notification_channels(type);

CREATE TABLE notification_logs (
  id TEXT PRIMARY KEY,
  channel_id TEXT REFERENCES notification_channels(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  target TEXT NOT NULL DEFAULT '',
  result TEXT NOT NULL,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_notification_logs_created_at ON notification_logs(created_at);
CREATE INDEX idx_notification_logs_event_type ON notification_logs(event_type);

CREATE TABLE cloudflare_credentials (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  api_token_encrypted TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE dns_records (
  id TEXT PRIMARY KEY,
  credential_id TEXT NOT NULL REFERENCES cloudflare_credentials(id) ON DELETE CASCADE,
  zone_id TEXT NOT NULL,
  record_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'A',
  current_value TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_dns_records_enabled ON dns_records(enabled);

CREATE TABLE telegram_bot_settings (
  id TEXT PRIMARY KEY,
  bot_token_encrypted TEXT NOT NULL,
  allowed_chat_id TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
