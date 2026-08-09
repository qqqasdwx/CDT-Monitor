package service

import (
	"context"
	"strconv"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

func (s *Service) TrafficTrend(ctx context.Context, bucket string, limit int) ([]store.TrafficPoint, error) {
	return s.repo.TrafficTrend(ctx, bucket, limit)
}

func (s *Service) InstanceStatusHistory(ctx context.Context, limit int) ([]store.InstanceStatusPoint, error) {
	return s.repo.ListInstanceStatusHistory(ctx, limit)
}

func (s *Service) QueryActionLogs(ctx context.Context, limit int, since *time.Time, actionType string) ([]store.ActionLog, error) {
	return s.repo.QueryActionLogs(ctx, store.LogQuery{Limit: limit, Since: since, Type: actionType})
}

func (s *Service) CleanupLogs(ctx context.Context) (store.ActionLog, error) {
	settings, err := s.repo.ListSettings(ctx)
	if err != nil {
		return store.ActionLog{}, err
	}
	retentionDays := 30
	if value, ok := settings["log_retention_days"]; ok {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			retentionDays = parsed
		}
	}
	before := time.Now().UTC().AddDate(0, 0, -retentionDays)
	deleted, err := s.repo.CleanupLogs(ctx, before)
	result := "success"
	errorMessage := ""
	if err != nil {
		result = "failed"
		errorMessage = err.Error()
	}
	reason := "清理超过保留天数的日志，删除记录数 " + strconv.FormatInt(deleted, 10)
	log := store.ActionLog{
		ID:            uuid.NewString(),
		ActionType:    "cleanup_logs",
		TriggerSource: "manual",
		Result:        result,
		Reason:        reason,
		ErrorMessage:  errorMessage,
		CreatedAt:     time.Now().UTC(),
	}
	created, logErr := s.repo.CreateActionLog(ctx, log)
	if logErr != nil {
		return store.ActionLog{}, logErr
	}
	if err != nil {
		return created, err
	}
	return created, nil
}
