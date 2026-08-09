package service

import (
	"context"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

type RiskyOperationInput struct {
	InstanceConfigID string `json:"instanceConfigId"`
	Operation        string `json:"operation"`
	ConfirmText      string `json:"confirmText"`
}

func (s *Service) EvaluateRiskyOperation(ctx context.Context, input RiskyOperationInput) (store.ActionLog, error) {
	if input.Operation != "create_instance" && input.Operation != "release_instance" && input.Operation != "change_public_ip" {
		return store.ActionLog{}, validation("高风险操作类型不合法")
	}
	if input.Operation != "create_instance" && input.InstanceConfigID == "" {
		return store.ActionLog{}, validation("实例配置 ID 不能为空")
	}
	result := "blocked"
	errorMessage := "高风险 ECS 操作仅完成评估和审计，当前 dry-run 模式不会执行真实云资源变更"
	if input.ConfirmText == "CONFIRM" {
		result = "failed"
	}
	log := store.ActionLog{
		ID:            uuid.NewString(),
		InstanceID:    input.InstanceConfigID,
		ActionType:    input.Operation,
		TriggerSource: "manual",
		Result:        result,
		Reason:        "高风险 ECS 操作评估",
		ErrorMessage:  errorMessage,
		CreatedAt:     time.Now().UTC(),
	}
	created, err := s.repo.CreateActionLog(ctx, log)
	if err != nil {
		return store.ActionLog{}, err
	}
	return created, validation(errorMessage)
}
