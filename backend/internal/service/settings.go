package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func (s *Service) Settings(ctx context.Context) (map[string]string, error) {
	return s.repo.ListSettings(ctx)
}

type SettingsInput struct {
	DefaultSyncIntervalSeconds string `json:"defaultSyncIntervalSeconds"`
	DefaultTrafficLimitBytes   string `json:"defaultTrafficLimitBytes"`
	DefaultStopMode            string `json:"defaultStopMode"`
	DefaultKeepaliveEnabled    string `json:"defaultKeepaliveEnabled"`
	KeepaliveCooldownSeconds   string `json:"keepaliveCooldownSeconds"`
	LogRetentionDays           string `json:"logRetentionDays"`
}

func (s *Service) UpdateSettings(ctx context.Context, input SettingsInput) (map[string]string, error) {
	updates := map[string]string{
		"default_sync_interval_seconds": strings.TrimSpace(input.DefaultSyncIntervalSeconds),
		"default_traffic_limit_bytes":   strings.TrimSpace(input.DefaultTrafficLimitBytes),
		"default_stop_mode":             strings.TrimSpace(input.DefaultStopMode),
		"default_keepalive_enabled":     strings.TrimSpace(input.DefaultKeepaliveEnabled),
		"keepalive_cooldown_seconds":    strings.TrimSpace(input.KeepaliveCooldownSeconds),
		"log_retention_days":            strings.TrimSpace(input.LogRetentionDays),
	}
	if err := validatePositiveInt(updates["default_sync_interval_seconds"], "默认同步频率必须大于 0"); err != nil {
		return nil, err
	}
	if err := validatePositiveInt(updates["default_traffic_limit_bytes"], "默认流量阈值必须大于 0"); err != nil {
		return nil, err
	}
	if updates["default_stop_mode"] != StopModeKeepCharging && updates["default_stop_mode"] != StopModeStopCharging {
		return nil, validation("默认停机模式不合法")
	}
	if updates["default_keepalive_enabled"] != "true" && updates["default_keepalive_enabled"] != "false" {
		return nil, validation("默认保活开关必须是 true 或 false")
	}
	if err := validatePositiveInt(updates["keepalive_cooldown_seconds"], "保活冷却时间必须大于 0"); err != nil {
		return nil, err
	}
	if err := validatePositiveInt(updates["log_retention_days"], "日志保留天数必须大于 0"); err != nil {
		return nil, err
	}
	for key, value := range updates {
		if err := s.repo.SetSetting(ctx, key, value); err != nil {
			return nil, err
		}
	}
	return s.repo.ListSettings(ctx)
}

func (s *Service) ResetWebhookToken(ctx context.Context) (string, error) {
	first := strings.ReplaceAll(uuid.NewString(), "-", "")
	second := strings.ReplaceAll(uuid.NewString(), "-", "")
	token := first + second[:16]
	if err := s.repo.SetSetting(ctx, "webhook_token", token); err != nil {
		return "", err
	}
	return "/api/webhooks/aliyun/events/" + token, nil
}

func validatePositiveInt(value, message string) error {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return validation(message)
	}
	return nil
}
