package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) CreateAccount(ctx context.Context, account Account) (Account, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO aliyun_accounts (
		id, access_key_id, access_key_secret_encrypted, region, name, enabled
	) VALUES (?, ?, ?, ?, ?, ?)`,
		account.ID, account.AccessKeyID, account.AccessKeySecretEncrypted, account.Region, account.Name, boolToInt(account.Enabled),
	)
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	return r.GetAccount(ctx, account.ID)
}

func (r *Repository) UpdateAccount(ctx context.Context, account Account, updateSecret bool) (Account, error) {
	if updateSecret {
		_, err := r.db.ExecContext(ctx, `UPDATE aliyun_accounts
			SET access_key_id = ?, access_key_secret_encrypted = ?, region = ?, name = ?, enabled = ?, updated_at = datetime('now')
			WHERE id = ?`,
			account.AccessKeyID, account.AccessKeySecretEncrypted, account.Region, account.Name, boolToInt(account.Enabled), account.ID,
		)
		if err != nil {
			return Account{}, fmt.Errorf("update account: %w", err)
		}
	} else {
		_, err := r.db.ExecContext(ctx, `UPDATE aliyun_accounts
			SET access_key_id = ?, region = ?, name = ?, enabled = ?, updated_at = datetime('now')
			WHERE id = ?`,
			account.AccessKeyID, account.Region, account.Name, boolToInt(account.Enabled), account.ID,
		)
		if err != nil {
			return Account{}, fmt.Errorf("update account: %w", err)
		}
	}
	return r.GetAccount(ctx, account.ID)
}

func (r *Repository) DeleteAccount(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM aliyun_accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return ensureAffected(result, "account")
}

func (r *Repository) GetAccount(ctx context.Context, id string) (Account, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, access_key_id, access_key_secret_encrypted, region, name, enabled, created_at, updated_at
		FROM aliyun_accounts WHERE id = ?`, id)
	return scanAccount(row)
}

func (r *Repository) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, access_key_id, access_key_secret_encrypted, region, name, enabled, created_at, updated_at
		FROM aliyun_accounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return accounts, nil
}

func (r *Repository) CreateInstance(ctx context.Context, instance Instance) (Instance, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO ecs_instances (
		id, account_id, instance_id, name, traffic_limit_bytes, stop_mode, enabled,
		protection_enabled, keepalive_enabled, keepalive_cooldown_seconds, monthly_restore_enabled
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		instance.ID, instance.AccountID, instance.InstanceID, instance.Name, instance.TrafficLimitBytes,
		instance.StopMode, boolToInt(instance.Enabled), boolToInt(instance.ProtectionEnabled),
		boolToInt(instance.KeepaliveEnabled), instance.KeepaliveCooldownSeconds, boolToInt(instance.MonthlyRestoreEnabled),
	)
	if err != nil {
		return Instance{}, fmt.Errorf("create instance: %w", err)
	}
	return r.GetInstance(ctx, instance.ID)
}

func (r *Repository) UpdateInstance(ctx context.Context, instance Instance) (Instance, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET account_id = ?, instance_id = ?, name = ?, traffic_limit_bytes = ?, stop_mode = ?, enabled = ?,
		    protection_enabled = ?, keepalive_enabled = ?, keepalive_cooldown_seconds = ?, monthly_restore_enabled = ?, updated_at = datetime('now')
		WHERE id = ?`,
		instance.AccountID, instance.InstanceID, instance.Name, instance.TrafficLimitBytes, instance.StopMode,
		boolToInt(instance.Enabled), boolToInt(instance.ProtectionEnabled), boolToInt(instance.KeepaliveEnabled),
		instance.KeepaliveCooldownSeconds, boolToInt(instance.MonthlyRestoreEnabled), instance.ID,
	)
	if err != nil {
		return Instance{}, fmt.Errorf("update instance: %w", err)
	}
	return r.GetInstance(ctx, instance.ID)
}

func (r *Repository) DeleteInstance(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM ecs_instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return ensureAffected(result, "instance")
}

func (r *Repository) GetInstance(ctx context.Context, id string) (Instance, error) {
	row := r.db.QueryRowContext(ctx, instanceSelectSQL()+` WHERE i.id = ?`, id)
	return scanInstance(row)
}

func (r *Repository) GetInstanceByCloudID(ctx context.Context, instanceID string) (Instance, error) {
	row := r.db.QueryRowContext(ctx, instanceSelectSQL()+` WHERE i.instance_id = ? ORDER BY i.created_at DESC LIMIT 1`, instanceID)
	return scanInstance(row)
}

func (r *Repository) ListInstances(ctx context.Context) ([]Instance, error) {
	rows, err := r.db.QueryContext(ctx, instanceSelectSQL()+` ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances: %w", err)
	}
	return instances, nil
}

func (r *Repository) UpdateInstanceSync(ctx context.Context, id string, status string, trafficBytes int64, syncedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET last_status = ?, last_traffic_bytes = ?, last_synced_at = ?, updated_at = datetime('now')
		WHERE id = ?`,
		status, trafficBytes, formatTime(syncedAt), id,
	)
	if err != nil {
		return fmt.Errorf("update instance sync: %w", err)
	}
	if err := ensureAffected(result, "instance"); err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO instance_status_history (id, instance_id, status, observed_at)
		VALUES (?, ?, ?, ?)`, uuid.NewString(), id, status, formatTime(syncedAt))
	if err != nil {
		return fmt.Errorf("create status history: %w", err)
	}
	return nil
}

func (r *Repository) MarkProtectionStop(ctx context.Context, id string, stoppedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET stopped_by_protection_at = ?, manual_stop_at = NULL, last_status = 'Stopping', updated_at = datetime('now')
		WHERE id = ?`, formatTime(stoppedAt), id)
	if err != nil {
		return fmt.Errorf("mark protection stop: %w", err)
	}
	return ensureAffected(result, "instance")
}

func (r *Repository) ClearProtectionStop(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET stopped_by_protection_at = NULL, updated_at = datetime('now')
		WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("clear protection stop: %w", err)
	}
	return ensureAffected(result, "instance")
}

func (r *Repository) MarkManualStop(ctx context.Context, id string, stoppedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET manual_stop_at = ?, last_status = 'Stopping', updated_at = datetime('now')
		WHERE id = ?`, formatTime(stoppedAt), id)
	if err != nil {
		return fmt.Errorf("mark manual stop: %w", err)
	}
	return ensureAffected(result, "instance")
}

func (r *Repository) MarkManualStart(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET manual_stop_at = NULL, stopped_by_protection_at = NULL, last_status = 'Starting', updated_at = datetime('now')
		WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mark manual start: %w", err)
	}
	return ensureAffected(result, "instance")
}

func (r *Repository) MarkKeepaliveStart(ctx context.Context, id string, startedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ecs_instances
		SET last_keepalive_at = ?, last_status = 'Starting', updated_at = datetime('now')
		WHERE id = ?`, formatTime(startedAt), id)
	if err != nil {
		return fmt.Errorf("mark keepalive start: %w", err)
	}
	return ensureAffected(result, "instance")
}

func (r *Repository) CreateTrafficSnapshot(ctx context.Context, snapshot TrafficSnapshot) (TrafficSnapshot, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO traffic_snapshots (
		id, account_id, period_start, period_end, total_bytes, collected_at
	) VALUES (?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.AccountID, formatTime(snapshot.PeriodStart), formatTime(snapshot.PeriodEnd),
		snapshot.TotalBytes, formatTime(snapshot.CollectedAt),
	)
	if err != nil {
		return TrafficSnapshot{}, fmt.Errorf("create traffic snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *Repository) CreateCostSnapshot(ctx context.Context, snapshot CostSnapshot) (CostSnapshot, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO cost_snapshots (
		id, account_id, available_amount, credit_amount, currency, collected_at
	) VALUES (?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.AccountID, snapshot.AvailableAmount, snapshot.CreditAmount, snapshot.Currency, formatTime(snapshot.CollectedAt),
	)
	if err != nil {
		return CostSnapshot{}, fmt.Errorf("create cost snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *Repository) ListLatestCostSnapshots(ctx context.Context) ([]CostSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.account_id, a.name, c.available_amount, c.credit_amount, c.currency, c.collected_at
		FROM cost_snapshots c
		JOIN aliyun_accounts a ON a.id = c.account_id
		WHERE c.collected_at = (
			SELECT max(inner_c.collected_at) FROM cost_snapshots inner_c WHERE inner_c.account_id = c.account_id
		)
		ORDER BY c.collected_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list latest cost snapshots: %w", err)
	}
	defer rows.Close()
	var snapshots []CostSnapshot
	for rows.Next() {
		snapshot, err := scanCostSnapshot(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cost snapshots: %w", err)
	}
	return snapshots, nil
}

func (r *Repository) TrafficTrend(ctx context.Context, bucket string, limit int) ([]TrafficPoint, error) {
	if bucket != "day" {
		bucket = "hour"
	}
	if limit <= 0 || limit > 720 {
		limit = 168
	}
	format := "%Y-%m-%dT%H:00:00Z"
	if bucket == "day" {
		format = "%Y-%m-%dT00:00:00Z"
	}
	rows, err := r.db.QueryContext(ctx, `SELECT strftime(?, collected_at) AS bucket_start, max(total_bytes)
		FROM traffic_snapshots
		GROUP BY bucket_start
		ORDER BY bucket_start DESC
		LIMIT ?`, format, limit)
	if err != nil {
		return nil, fmt.Errorf("query traffic trend: %w", err)
	}
	defer rows.Close()
	var points []TrafficPoint
	for rows.Next() {
		var point TrafficPoint
		if err := rows.Scan(&point.BucketStart, &point.TotalBytes); err != nil {
			return nil, fmt.Errorf("scan traffic trend: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate traffic trend: %w", err)
	}
	reverseTrafficPoints(points)
	return points, nil
}

func (r *Repository) ListInstanceStatusHistory(ctx context.Context, limit int) ([]InstanceStatusPoint, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, instance_id, status, observed_at
		FROM instance_status_history ORDER BY observed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list status history: %w", err)
	}
	defer rows.Close()
	var points []InstanceStatusPoint
	for rows.Next() {
		var point InstanceStatusPoint
		var observedAt string
		if err := rows.Scan(&point.ID, &point.InstanceID, &point.Status, &observedAt); err != nil {
			return nil, fmt.Errorf("scan status history: %w", err)
		}
		parsed, err := parseTime(observedAt)
		if err != nil {
			return nil, err
		}
		point.ObservedAt = parsed
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status history: %w", err)
	}
	return points, nil
}

func (r *Repository) CleanupLogs(ctx context.Context, before time.Time) (int64, error) {
	tables := []string{"action_logs", "cloud_events", "notification_logs", "instance_status_history"}
	var total int64
	for _, table := range tables {
		result, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE created_at < ?`, table), formatTime(before))
		if table == "instance_status_history" {
			result, err = r.db.ExecContext(ctx, `DELETE FROM instance_status_history WHERE observed_at < ?`, formatTime(before))
		}
		if err != nil {
			return total, fmt.Errorf("cleanup %s: %w", table, err)
		}
		affected, _ := result.RowsAffected()
		total += affected
	}
	return total, nil
}

func (r *Repository) CreateScheduledTask(ctx context.Context, task ScheduledTask) (ScheduledTask, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO scheduled_tasks (
		id, instance_config_id, action, run_at, enabled
	) VALUES (?, ?, ?, ?, ?)`, task.ID, task.InstanceConfigID, task.Action, formatTime(task.RunAt), boolToInt(task.Enabled))
	if err != nil {
		return ScheduledTask{}, fmt.Errorf("create scheduled task: %w", err)
	}
	return r.GetScheduledTask(ctx, task.ID)
}

func (r *Repository) GetScheduledTask(ctx context.Context, id string) (ScheduledTask, error) {
	row := r.db.QueryRowContext(ctx, scheduledTaskSelectSQL()+` WHERE t.id = ?`, id)
	return scanScheduledTask(row)
}

func (r *Repository) ListScheduledTasks(ctx context.Context) ([]ScheduledTask, error) {
	rows, err := r.db.QueryContext(ctx, scheduledTaskSelectSQL()+` ORDER BY t.run_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list scheduled tasks: %w", err)
	}
	defer rows.Close()
	var tasks []ScheduledTask
	for rows.Next() {
		task, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled tasks: %w", err)
	}
	return tasks, nil
}

func (r *Repository) DueScheduledTasks(ctx context.Context, now time.Time) ([]ScheduledTask, error) {
	rows, err := r.db.QueryContext(ctx, scheduledTaskSelectSQL()+` WHERE t.enabled = 1 AND t.run_at <= ? AND t.last_run_at IS NULL ORDER BY t.run_at ASC`, formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("list due scheduled tasks: %w", err)
	}
	defer rows.Close()
	var tasks []ScheduledTask
	for rows.Next() {
		task, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due scheduled tasks: %w", err)
	}
	return tasks, nil
}

func (r *Repository) MarkScheduledTaskRun(ctx context.Context, id string, runAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE scheduled_tasks
		SET last_run_at = ?, updated_at = datetime('now')
		WHERE id = ?`, formatTime(runAt), id)
	if err != nil {
		return fmt.Errorf("mark scheduled task run: %w", err)
	}
	return ensureAffected(result, "scheduled task")
}

func (r *Repository) CreateActionLog(ctx context.Context, log ActionLog) (ActionLog, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO action_logs (
		id, account_id, instance_id, action_type, trigger_source, result, reason,
		traffic_bytes, threshold_bytes, stop_mode, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, nullEmpty(log.AccountID), nullEmpty(log.InstanceID), log.ActionType, log.TriggerSource, log.Result,
		log.Reason, log.TrafficBytes, log.ThresholdBytes, nullEmpty(log.StopMode), log.ErrorMessage, formatTime(log.CreatedAt),
	)
	if err != nil {
		return ActionLog{}, fmt.Errorf("create action log: %w", err)
	}
	return log, nil
}

type LogQuery struct {
	Limit int
	Since *time.Time
	Type  string
}

func (r *Repository) ListActionLogs(ctx context.Context, limit int) ([]ActionLog, error) {
	return r.QueryActionLogs(ctx, LogQuery{Limit: limit})
}

func (r *Repository) QueryActionLogs(ctx context.Context, query LogQuery) ([]ActionLog, error) {
	limit := query.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	sqlText := `SELECT id, account_id, instance_id, action_type, trigger_source, result, reason,
		traffic_bytes, threshold_bytes, stop_mode, error_message, created_at
		FROM action_logs WHERE 1 = 1`
	args := []any{}
	if query.Since != nil {
		sqlText += ` AND created_at >= ?`
		args = append(args, formatTime(*query.Since))
	}
	if query.Type != "" {
		sqlText += ` AND action_type = ?`
		args = append(args, query.Type)
	}
	sqlText += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("list action logs: %w", err)
	}
	defer rows.Close()

	var logs []ActionLog
	for rows.Next() {
		log, err := scanActionLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate action logs: %w", err)
	}
	return logs, nil
}

func (r *Repository) CreateCloudEvent(ctx context.Context, event CloudEvent) (CloudEvent, bool, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO cloud_events (
		id, event_id, instance_id, event_time, status, raw_payload, process_status, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.EventID, event.InstanceID, formatTime(event.EventTime), event.Status, event.RawPayload,
		event.ProcessStatus, event.ErrorMessage, formatTime(event.CreatedAt),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return CloudEvent{}, true, nil
		}
		return CloudEvent{}, false, fmt.Errorf("create cloud event: %w", err)
	}
	return event, false, nil
}

func (r *Repository) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key = ?`, key).Scan(&value)
	if err == nil {
		return value, nil
	}
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return "", fmt.Errorf("get setting: %w", err)
}

func (r *Repository) ListSettings(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()
	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return settings, nil
}

func (r *Repository) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO system_settings (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`, key, value)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func (r *Repository) CreateNotificationChannel(ctx context.Context, channel NotificationChannel) (NotificationChannel, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO notification_channels (
		id, name, type, enabled, config_json
	) VALUES (?, ?, ?, ?, ?)`, channel.ID, channel.Name, channel.Type, boolToInt(channel.Enabled), channel.ConfigJSON)
	if err != nil {
		return NotificationChannel{}, fmt.Errorf("create notification channel: %w", err)
	}
	return r.GetNotificationChannel(ctx, channel.ID)
}

func (r *Repository) GetNotificationChannel(ctx context.Context, id string) (NotificationChannel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, type, enabled, config_json, created_at, updated_at
		FROM notification_channels WHERE id = ?`, id)
	return scanNotificationChannel(row)
}

func (r *Repository) ListNotificationChannels(ctx context.Context) ([]NotificationChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, enabled, config_json, created_at, updated_at
		FROM notification_channels ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list notification channels: %w", err)
	}
	defer rows.Close()
	var channels []NotificationChannel
	for rows.Next() {
		channel, err := scanNotificationChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification channels: %w", err)
	}
	return channels, nil
}

func (r *Repository) CreateNotificationLog(ctx context.Context, log NotificationLog) (NotificationLog, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO notification_logs (
		id, channel_id, event_type, target, result, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		log.ID, nullEmpty(log.ChannelID), log.EventType, log.Target, log.Result, log.ErrorMessage, formatTime(log.CreatedAt),
	)
	if err != nil {
		return NotificationLog{}, fmt.Errorf("create notification log: %w", err)
	}
	return log, nil
}

func (r *Repository) ListNotificationLogs(ctx context.Context, limit int) ([]NotificationLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, channel_id, event_type, target, result, error_message, created_at
		FROM notification_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list notification logs: %w", err)
	}
	defer rows.Close()
	var logs []NotificationLog
	for rows.Next() {
		log, err := scanNotificationLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification logs: %w", err)
	}
	return logs, nil
}

func (r *Repository) CreateCloudflareCredential(ctx context.Context, credential CloudflareCredential) (CloudflareCredential, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO cloudflare_credentials (
		id, name, api_token_encrypted, enabled
	) VALUES (?, ?, ?, ?)`, credential.ID, credential.Name, credential.APITokenEncrypted, boolToInt(credential.Enabled))
	if err != nil {
		return CloudflareCredential{}, fmt.Errorf("create cloudflare credential: %w", err)
	}
	return r.GetCloudflareCredential(ctx, credential.ID)
}

func (r *Repository) GetCloudflareCredential(ctx context.Context, id string) (CloudflareCredential, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, api_token_encrypted, enabled, created_at, updated_at
		FROM cloudflare_credentials WHERE id = ?`, id)
	return scanCloudflareCredential(row)
}

func (r *Repository) ListCloudflareCredentials(ctx context.Context) ([]CloudflareCredential, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, api_token_encrypted, enabled, created_at, updated_at
		FROM cloudflare_credentials ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list cloudflare credentials: %w", err)
	}
	defer rows.Close()
	var credentials []CloudflareCredential
	for rows.Next() {
		credential, err := scanCloudflareCredential(rows)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloudflare credentials: %w", err)
	}
	return credentials, nil
}

func (r *Repository) CreateDNSRecord(ctx context.Context, record DNSRecord) (DNSRecord, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO dns_records (
		id, credential_id, zone_id, record_id, name, type, current_value, enabled
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.CredentialID, record.ZoneID, record.RecordID, record.Name, record.Type, record.CurrentValue, boolToInt(record.Enabled),
	)
	if err != nil {
		return DNSRecord{}, fmt.Errorf("create dns record: %w", err)
	}
	return r.GetDNSRecord(ctx, record.ID)
}

func (r *Repository) GetDNSRecord(ctx context.Context, id string) (DNSRecord, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, credential_id, zone_id, record_id, name, type, current_value, enabled, created_at, updated_at
		FROM dns_records WHERE id = ?`, id)
	return scanDNSRecord(row)
}

func (r *Repository) ListDNSRecords(ctx context.Context) ([]DNSRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, credential_id, zone_id, record_id, name, type, current_value, enabled, created_at, updated_at
		FROM dns_records ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list dns records: %w", err)
	}
	defer rows.Close()
	var records []DNSRecord
	for rows.Next() {
		record, err := scanDNSRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dns records: %w", err)
	}
	return records, nil
}

func (r *Repository) UpdateDNSRecordValue(ctx context.Context, id, value string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE dns_records
		SET current_value = ?, updated_at = datetime('now')
		WHERE id = ?`, value, id)
	if err != nil {
		return fmt.Errorf("update dns record value: %w", err)
	}
	return ensureAffected(result, "dns record")
}

func (r *Repository) ListCloudEvents(ctx context.Context, limit int) ([]CloudEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, event_id, instance_id, event_time, status, raw_payload, process_status, error_message, created_at
		FROM cloud_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list cloud events: %w", err)
	}
	defer rows.Close()

	var events []CloudEvent
	for rows.Next() {
		event, err := scanCloudEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud events: %w", err)
	}
	return events, nil
}

func ensureAffected(result sql.Result, name string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check %s rows affected: %w", name, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

var ErrNotFound = errors.New("not found")
