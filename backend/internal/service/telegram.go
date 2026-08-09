package service

import (
	"context"
	"strings"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

type TelegramConfigInput struct {
	BotToken      string `json:"botToken"`
	AllowedChatID string `json:"allowedChatId"`
	Enabled       *bool  `json:"enabled"`
}

type TelegramCommandInput struct {
	ChatID           string `json:"chatId"`
	Command          string `json:"command"`
	InstanceConfigID string `json:"instanceConfigId"`
	ConfirmText      string `json:"confirmText"`
}

func (s *Service) ConfigureTelegram(ctx context.Context, input TelegramConfigInput) (store.ActionLog, error) {
	if strings.TrimSpace(input.BotToken) == "" || strings.TrimSpace(input.AllowedChatID) == "" {
		return store.ActionLog{}, validation("Telegram Bot Token 和允许的 Chat ID 不能为空")
	}
	if _, err := s.codec.Encrypt(input.BotToken); err != nil {
		return store.ActionLog{}, err
	}
	log := store.ActionLog{
		ID:            uuid.NewString(),
		ActionType:    "telegram_configure",
		TriggerSource: "manual",
		Result:        "success",
		Reason:        "Telegram Bot 配置已校验并记录审计",
		CreatedAt:     time.Now().UTC(),
	}
	return s.repo.CreateActionLog(ctx, log)
}

func (s *Service) HandleTelegramCommand(ctx context.Context, input TelegramCommandInput) (any, error) {
	if strings.TrimSpace(input.ChatID) == "" {
		return nil, validation("Telegram Chat ID 不能为空")
	}
	switch input.Command {
	case "status":
		status, err := s.Status(ctx)
		if err != nil {
			return nil, err
		}
		_, _ = s.repo.CreateActionLog(ctx, store.ActionLog{
			ID:            uuid.NewString(),
			ActionType:    "telegram_status",
			TriggerSource: "telegram",
			Result:        "success",
			Reason:        "Telegram 查询实例状态",
			CreatedAt:     time.Now().UTC(),
		})
		return status, nil
	case "start":
		return s.StartInstance(ctx, input.InstanceConfigID)
	case "stop":
		if input.ConfirmText != "CONFIRM" {
			return nil, validation("Telegram 停止实例需要 CONFIRM 确认")
		}
		return s.StopInstance(ctx, input.InstanceConfigID)
	default:
		return nil, validation("Telegram 命令不支持")
	}
}
