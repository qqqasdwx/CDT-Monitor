package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
)

type scanner interface {
	Scan(dest ...any) error
}

func instanceSelectSQL() string {
	return `SELECT i.id, i.account_id, i.instance_id, i.name, i.traffic_limit_bytes, i.stop_mode,
		i.enabled, i.protection_enabled, i.keepalive_enabled, i.keepalive_cooldown_seconds,
		i.monthly_restore_enabled, i.last_status, i.last_traffic_bytes, i.last_synced_at, i.stopped_by_protection_at, i.manual_stop_at,
		i.last_keepalive_at, i.created_at, i.updated_at, a.name, a.region
		FROM ecs_instances i
		JOIN aliyun_accounts a ON a.id = i.account_id`
}

func scheduledTaskSelectSQL() string {
	return `SELECT t.id, t.instance_config_id, t.action, t.run_at, t.enabled, t.last_run_at,
		t.created_at, t.updated_at, i.instance_id
		FROM scheduled_tasks t
		JOIN ecs_instances i ON i.id = t.instance_config_id`
}

func scanAccount(s scanner) (Account, error) {
	var account Account
	var enabled int
	var createdAt, updatedAt string
	if err := s.Scan(
		&account.ID,
		&account.AccessKeyID,
		&account.AccessKeySecretEncrypted,
		&account.Region,
		&account.Name,
		&enabled,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Account{}, ErrNotFound
		}
		return Account{}, fmt.Errorf("scan account: %w", err)
	}

	var err error
	account.Enabled = enabled == 1
	account.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Account{}, err
	}
	account.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func scanInstance(s scanner) (Instance, error) {
	var instance Instance
	var enabled, protectionEnabled, keepaliveEnabled, monthlyRestoreEnabled int
	var createdAt, updatedAt string
	var lastSyncedAt, stoppedByProtectionAt, manualStopAt, lastKeepaliveAt sql.NullString
	if err := s.Scan(
		&instance.ID,
		&instance.AccountID,
		&instance.InstanceID,
		&instance.Name,
		&instance.TrafficLimitBytes,
		&instance.StopMode,
		&enabled,
		&protectionEnabled,
		&keepaliveEnabled,
		&instance.KeepaliveCooldownSeconds,
		&monthlyRestoreEnabled,
		&instance.LastStatus,
		&instance.LastTrafficBytes,
		&lastSyncedAt,
		&stoppedByProtectionAt,
		&manualStopAt,
		&lastKeepaliveAt,
		&createdAt,
		&updatedAt,
		&instance.AccountName,
		&instance.Region,
	); err != nil {
		if err == sql.ErrNoRows {
			return Instance{}, ErrNotFound
		}
		return Instance{}, fmt.Errorf("scan instance: %w", err)
	}

	var err error
	instance.Enabled = enabled == 1
	instance.ProtectionEnabled = protectionEnabled == 1
	instance.KeepaliveEnabled = keepaliveEnabled == 1
	instance.MonthlyRestoreEnabled = monthlyRestoreEnabled == 1
	instance.LastSyncedAt, err = parseNullTime(lastSyncedAt)
	if err != nil {
		return Instance{}, err
	}
	instance.StoppedByProtectionAt, err = parseNullTime(stoppedByProtectionAt)
	if err != nil {
		return Instance{}, err
	}
	instance.ManualStopAt, err = parseNullTime(manualStopAt)
	if err != nil {
		return Instance{}, err
	}
	instance.LastKeepaliveAt, err = parseNullTime(lastKeepaliveAt)
	if err != nil {
		return Instance{}, err
	}
	instance.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Instance{}, err
	}
	instance.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Instance{}, err
	}
	return instance, nil
}

func scanActionLog(s scanner) (ActionLog, error) {
	var log ActionLog
	var accountID, instanceID, stopMode sql.NullString
	var trafficBytes, thresholdBytes sql.NullInt64
	var createdAt string
	if err := s.Scan(
		&log.ID,
		&accountID,
		&instanceID,
		&log.ActionType,
		&log.TriggerSource,
		&log.Result,
		&log.Reason,
		&trafficBytes,
		&thresholdBytes,
		&stopMode,
		&log.ErrorMessage,
		&createdAt,
	); err != nil {
		return ActionLog{}, fmt.Errorf("scan action log: %w", err)
	}

	log.AccountID = accountID.String
	log.InstanceID = instanceID.String
	if trafficBytes.Valid {
		value := trafficBytes.Int64
		log.TrafficBytes = &value
	}
	if thresholdBytes.Valid {
		value := thresholdBytes.Int64
		log.ThresholdBytes = &value
	}
	log.StopMode = stopMode.String
	var err error
	log.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return ActionLog{}, err
	}
	return log, nil
}

func scanCloudEvent(s scanner) (CloudEvent, error) {
	var event CloudEvent
	var eventTime, createdAt string
	if err := s.Scan(
		&event.ID,
		&event.EventID,
		&event.InstanceID,
		&eventTime,
		&event.Status,
		&event.RawPayload,
		&event.ProcessStatus,
		&event.ErrorMessage,
		&createdAt,
	); err != nil {
		return CloudEvent{}, fmt.Errorf("scan cloud event: %w", err)
	}

	var err error
	event.EventTime, err = parseTime(eventTime)
	if err != nil {
		return CloudEvent{}, err
	}
	event.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return CloudEvent{}, err
	}
	return event, nil
}

func scanNotificationChannel(s scanner) (NotificationChannel, error) {
	var channel NotificationChannel
	var enabled int
	var createdAt, updatedAt string
	if err := s.Scan(
		&channel.ID,
		&channel.Name,
		&channel.Type,
		&enabled,
		&channel.ConfigJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return NotificationChannel{}, ErrNotFound
		}
		return NotificationChannel{}, fmt.Errorf("scan notification channel: %w", err)
	}
	channel.Enabled = enabled == 1
	var err error
	channel.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return NotificationChannel{}, err
	}
	channel.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return NotificationChannel{}, err
	}
	return channel, nil
}

func scanCostSnapshot(s scanner) (CostSnapshot, error) {
	var snapshot CostSnapshot
	var collectedAt string
	if err := s.Scan(
		&snapshot.ID,
		&snapshot.AccountID,
		&snapshot.AccountName,
		&snapshot.AvailableAmount,
		&snapshot.CreditAmount,
		&snapshot.Currency,
		&collectedAt,
	); err != nil {
		return CostSnapshot{}, fmt.Errorf("scan cost snapshot: %w", err)
	}
	var err error
	snapshot.CollectedAt, err = parseTime(collectedAt)
	if err != nil {
		return CostSnapshot{}, err
	}
	return snapshot, nil
}

func scanNotificationLog(s scanner) (NotificationLog, error) {
	var log NotificationLog
	var channelID sql.NullString
	var createdAt string
	if err := s.Scan(
		&log.ID,
		&channelID,
		&log.EventType,
		&log.Target,
		&log.Result,
		&log.ErrorMessage,
		&createdAt,
	); err != nil {
		return NotificationLog{}, fmt.Errorf("scan notification log: %w", err)
	}
	log.ChannelID = channelID.String
	var err error
	log.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return NotificationLog{}, err
	}
	return log, nil
}

func scanScheduledTask(s scanner) (ScheduledTask, error) {
	var task ScheduledTask
	var enabled int
	var runAt, createdAt, updatedAt string
	var lastRunAt sql.NullString
	if err := s.Scan(
		&task.ID,
		&task.InstanceConfigID,
		&task.Action,
		&runAt,
		&enabled,
		&lastRunAt,
		&createdAt,
		&updatedAt,
		&task.InstanceID,
	); err != nil {
		if err == sql.ErrNoRows {
			return ScheduledTask{}, ErrNotFound
		}
		return ScheduledTask{}, fmt.Errorf("scan scheduled task: %w", err)
	}
	var err error
	task.RunAt, err = parseTime(runAt)
	if err != nil {
		return ScheduledTask{}, err
	}
	task.LastRunAt, err = parseNullTime(lastRunAt)
	if err != nil {
		return ScheduledTask{}, err
	}
	task.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return ScheduledTask{}, err
	}
	task.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return ScheduledTask{}, err
	}
	task.Enabled = enabled == 1
	return task, nil
}

func scanCloudflareCredential(s scanner) (CloudflareCredential, error) {
	var credential CloudflareCredential
	var enabled int
	var createdAt, updatedAt string
	if err := s.Scan(&credential.ID, &credential.Name, &credential.APITokenEncrypted, &enabled, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return CloudflareCredential{}, ErrNotFound
		}
		return CloudflareCredential{}, fmt.Errorf("scan cloudflare credential: %w", err)
	}
	credential.Enabled = enabled == 1
	var err error
	credential.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return CloudflareCredential{}, err
	}
	credential.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return CloudflareCredential{}, err
	}
	return credential, nil
}

func scanDNSRecord(s scanner) (DNSRecord, error) {
	var record DNSRecord
	var enabled int
	var createdAt, updatedAt string
	if err := s.Scan(
		&record.ID,
		&record.CredentialID,
		&record.ZoneID,
		&record.RecordID,
		&record.Name,
		&record.Type,
		&record.CurrentValue,
		&enabled,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DNSRecord{}, ErrNotFound
		}
		return DNSRecord{}, fmt.Errorf("scan dns record: %w", err)
	}
	record.Enabled = enabled == 1
	var err error
	record.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return DNSRecord{}, err
	}
	record.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return DNSRecord{}, err
	}
	return record, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q", value)
}

func parseNullTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func nullEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint || sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

func reverseTrafficPoints(points []TrafficPoint) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}
