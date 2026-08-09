package service

import (
	"context"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

type ScheduledTaskInput struct {
	InstanceConfigID string    `json:"instanceConfigId"`
	Action           string    `json:"action"`
	RunAt            time.Time `json:"runAt"`
	Enabled          *bool     `json:"enabled"`
}

func (s *Service) CreateScheduledTask(ctx context.Context, input ScheduledTaskInput) (store.ScheduledTask, error) {
	if input.InstanceConfigID == "" {
		return store.ScheduledTask{}, validation("实例配置 ID 不能为空")
	}
	if input.Action != "start" && input.Action != "stop" {
		return store.ScheduledTask{}, validation("定时动作必须是 start 或 stop")
	}
	if input.RunAt.IsZero() {
		return store.ScheduledTask{}, validation("执行时间不能为空")
	}
	if _, err := s.repo.GetInstance(ctx, input.InstanceConfigID); err != nil {
		return store.ScheduledTask{}, mapStoreError(err)
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return s.repo.CreateScheduledTask(ctx, store.ScheduledTask{
		ID:               uuid.NewString(),
		InstanceConfigID: input.InstanceConfigID,
		Action:           input.Action,
		RunAt:            input.RunAt.UTC(),
		Enabled:          enabled,
	})
}

func (s *Service) ListScheduledTasks(ctx context.Context) ([]store.ScheduledTask, error) {
	return s.repo.ListScheduledTasks(ctx)
}

func (s *Service) RunDueScheduledTasks(ctx context.Context) ([]store.ActionLog, error) {
	tasks, err := s.repo.DueScheduledTasks(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	var logs []store.ActionLog
	for _, task := range tasks {
		var log store.ActionLog
		if task.Action == "start" {
			log, err = s.StartInstance(ctx, task.InstanceConfigID)
		} else {
			instance, getErr := s.repo.GetInstance(ctx, task.InstanceConfigID)
			if getErr == nil && instance.StoppedByProtectionAt != nil {
				continue
			}
			log, err = s.StopInstance(ctx, task.InstanceConfigID)
		}
		if err != nil {
			s.SendNotification(ctx, NotificationEvent{
				Type:    "scheduled_task_failed",
				Title:   "定时任务执行失败",
				Message: err.Error(),
				Payload: map[string]any{"taskId": task.ID, "action": task.Action},
			})
			return logs, err
		}
		logs = append(logs, log)
		if err := s.repo.MarkScheduledTaskRun(ctx, task.ID, time.Now().UTC()); err != nil {
			return logs, err
		}
	}
	return logs, nil
}

func (s *Service) RunMonthlyRestore(ctx context.Context) ([]store.ActionLog, error) {
	now := time.Now().UTC()
	month := now.Format("2006-01")
	lastMonth, _ := s.repo.GetSetting(ctx, "last_monthly_restore_month")
	if lastMonth == month || now.Day() != 1 {
		return nil, nil
	}
	instances, err := s.repo.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	var logs []store.ActionLog
	for _, instance := range instances {
		if !instance.Enabled || !instance.MonthlyRestoreEnabled || instance.StoppedByProtectionAt == nil {
			continue
		}
		if err := s.repo.ClearProtectionStop(ctx, instance.ID); err != nil {
			return logs, err
		}
		log, err := s.StartInstance(ctx, instance.ID)
		if err != nil {
			return logs, err
		}
		logs = append(logs, log)
	}
	if err := s.repo.SetSetting(ctx, "last_monthly_restore_month", month); err != nil {
		return logs, err
	}
	return logs, nil
}
