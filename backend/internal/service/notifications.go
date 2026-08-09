package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

const (
	NotificationTypeWebhook  = "webhook"
	NotificationTypeEmail    = "email"
	NotificationTypeTelegram = "telegram"
)

type NotificationChannelInput struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Enabled *bool             `json:"enabled"`
	Config  map[string]string `json:"config"`
}

type NotificationEvent struct {
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Message string         `json:"message"`
	Payload map[string]any `json:"payload,omitempty"`
}

func (s *Service) CreateNotificationChannel(ctx context.Context, input NotificationChannelInput) (store.NotificationChannel, error) {
	if strings.TrimSpace(input.Name) == "" {
		return store.NotificationChannel{}, validation("通知通道名称不能为空")
	}
	channelType := strings.TrimSpace(input.Type)
	if channelType != NotificationTypeWebhook && channelType != NotificationTypeEmail && channelType != NotificationTypeTelegram {
		return store.NotificationChannel{}, validation("通知通道类型不合法")
	}
	if input.Config == nil {
		input.Config = map[string]string{}
	}
	if channelType == NotificationTypeWebhook && strings.TrimSpace(input.Config["url"]) == "" {
		return store.NotificationChannel{}, validation("webhook URL 不能为空")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	configJSON, err := json.Marshal(input.Config)
	if err != nil {
		return store.NotificationChannel{}, err
	}
	channel := store.NotificationChannel{
		ID:         uuid.NewString(),
		Name:       strings.TrimSpace(input.Name),
		Type:       channelType,
		Enabled:    enabled,
		ConfigJSON: string(configJSON),
	}
	created, err := s.repo.CreateNotificationChannel(ctx, channel)
	if err != nil {
		return store.NotificationChannel{}, err
	}
	return sanitizeNotificationChannel(created), nil
}

func (s *Service) ListNotificationChannels(ctx context.Context) ([]store.NotificationChannel, error) {
	channels, err := s.repo.ListNotificationChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]store.NotificationChannel, 0, len(channels))
	for _, channel := range channels {
		out = append(out, sanitizeNotificationChannel(channel))
	}
	return out, nil
}

func (s *Service) ListNotificationLogs(ctx context.Context, limit int) ([]store.NotificationLog, error) {
	return s.repo.ListNotificationLogs(ctx, limit)
}

func (s *Service) TestNotification(ctx context.Context, channelID string) (store.NotificationLog, error) {
	channel, err := s.repo.GetNotificationChannel(ctx, channelID)
	if err != nil {
		return store.NotificationLog{}, mapStoreError(err)
	}
	event := NotificationEvent{
		Type:    "test_notification",
		Title:   "CDT Monitor 测试通知",
		Message: "通知通道配置已被后端测试。",
	}
	return s.sendToChannel(ctx, channel, event)
}

func (s *Service) SendNotification(ctx context.Context, event NotificationEvent) {
	channels, err := s.repo.ListNotificationChannels(ctx)
	if err != nil {
		s.logger.Error("list notification channels failed", "error", err.Error())
		return
	}
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		if _, err := s.sendToChannel(ctx, channel, event); err != nil {
			s.logger.Error("send notification failed", "channel_id", channel.ID, "event_type", event.Type, "error", err.Error())
		}
	}
}

func (s *Service) sendToChannel(ctx context.Context, channel store.NotificationChannel, event NotificationEvent) (store.NotificationLog, error) {
	target := channel.Name
	result := "success"
	errorMessage := ""
	err := s.deliverNotification(ctx, channel, event)
	if err != nil {
		result = "failed"
		errorMessage = err.Error()
	}
	log := store.NotificationLog{
		ID:           uuid.NewString(),
		ChannelID:    channel.ID,
		EventType:    event.Type,
		Target:       target,
		Result:       result,
		ErrorMessage: errorMessage,
		CreatedAt:    time.Now().UTC(),
	}
	created, logErr := s.repo.CreateNotificationLog(ctx, log)
	if logErr != nil {
		return store.NotificationLog{}, logErr
	}
	if err != nil {
		return created, err
	}
	return created, nil
}

func (s *Service) deliverNotification(ctx context.Context, channel store.NotificationChannel, event NotificationEvent) error {
	var config map[string]string
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &config); err != nil {
		return fmt.Errorf("解析通知通道配置失败: %w", err)
	}
	switch channel.Type {
	case NotificationTypeWebhook:
		return sendWebhookNotification(ctx, config["url"], event)
	case NotificationTypeEmail:
		return fmt.Errorf("邮件通知发送器尚未配置 SMTP")
	case NotificationTypeTelegram:
		return fmt.Errorf("Telegram 通知发送器尚未配置 Bot API")
	default:
		return fmt.Errorf("未知通知通道类型: %s", channel.Type)
	}
}

func sendWebhookNotification(ctx context.Context, url string, event NotificationEvent) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("webhook URL 为空")
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回状态码 %d", resp.StatusCode)
	}
	return nil
}

func sanitizeNotificationChannel(channel store.NotificationChannel) store.NotificationChannel {
	if channel.ConfigJSON == "" {
		return channel
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &config); err != nil {
		channel.ConfigJSON = "{}"
		return channel
	}
	for key := range config {
		normalized := strings.ToLower(key)
		if strings.Contains(normalized, "secret") || strings.Contains(normalized, "token") || strings.Contains(normalized, "password") {
			config[key] = "******"
		}
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		channel.ConfigJSON = "{}"
		return channel
	}
	channel.ConfigJSON = string(encoded)
	return channel
}
