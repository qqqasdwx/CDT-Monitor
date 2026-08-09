package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"cdt-monitor/backend/internal/aliyun"
	"cdt-monitor/backend/internal/logger"
	"cdt-monitor/backend/internal/secrets"
	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

const (
	StopModeKeepCharging = "KeepCharging"
	StopModeStopCharging = "StopCharging"
)

type Service struct {
	repo   *store.Repository
	cloud  aliyun.Client
	codec  *secrets.Codec
	logger *logger.Logger
}

func New(repo *store.Repository, cloud aliyun.Client, codec *secrets.Codec, logger *logger.Logger) *Service {
	return &Service{repo: repo, cloud: cloud, codec: codec, logger: logger}
}

type AccountInput struct {
	AccessKeyID     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	Region          string `json:"region"`
	Name            string `json:"name"`
	Enabled         *bool  `json:"enabled"`
}

type InstanceInput struct {
	AccountID                string `json:"accountId"`
	InstanceID               string `json:"instanceId"`
	Name                     string `json:"name"`
	TrafficLimitBytes        int64  `json:"trafficLimitBytes"`
	StopMode                 string `json:"stopMode"`
	Enabled                  *bool  `json:"enabled"`
	ProtectionEnabled        *bool  `json:"protectionEnabled"`
	KeepaliveEnabled         *bool  `json:"keepaliveEnabled"`
	KeepaliveCooldownSeconds int64  `json:"keepaliveCooldownSeconds"`
	MonthlyRestoreEnabled    *bool  `json:"monthlyRestoreEnabled"`
}

type SyncResult struct {
	AccountID     string                 `json:"accountId"`
	TrafficBytes  int64                  `json:"trafficBytes"`
	PeriodStart   time.Time              `json:"periodStart"`
	PeriodEnd     time.Time              `json:"periodEnd"`
	Instances     []store.Instance       `json:"instances"`
	ProtectionLog []store.ActionLog      `json:"protectionLog"`
	Errors        []string               `json:"errors"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type WebhookInput struct {
	EventID    string          `json:"eventId"`
	InstanceID string          `json:"instanceId"`
	EventTime  time.Time       `json:"eventTime"`
	Status     string          `json:"status"`
	Raw        json.RawMessage `json:"raw"`
}

type WebhookResult struct {
	Event     store.CloudEvent `json:"event"`
	Duplicate bool             `json:"duplicate"`
	Action    *store.ActionLog `json:"action,omitempty"`
}

func (s *Service) Status(ctx context.Context) (store.Dashboard, error) {
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return store.Dashboard{}, err
	}
	instances, err := s.repo.ListInstances(ctx)
	if err != nil {
		return store.Dashboard{}, err
	}
	logs, err := s.repo.ListActionLogs(ctx, 20)
	if err != nil {
		return store.Dashboard{}, err
	}
	return store.Dashboard{
		Accounts:    sanitizeAccounts(accounts),
		Instances:   instances,
		ActionLogs:  logs,
		CloudMode:   string(s.cloud.Mode()),
		WebhookURL:  "/api/webhooks/aliyun/events/" + s.webhookToken(ctx),
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func (s *Service) CreateAccount(ctx context.Context, input AccountInput) (store.Account, error) {
	if err := validateAccountInput(input, true); err != nil {
		return store.Account{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	account := store.Account{
		ID:          uuid.NewString(),
		AccessKeyID: strings.TrimSpace(input.AccessKeyID),
		Region:      strings.TrimSpace(input.Region),
		Name:        strings.TrimSpace(input.Name),
		Enabled:     enabled,
	}
	encrypted, err := s.codec.Encrypt(input.AccessKeySecret)
	if err != nil {
		return store.Account{}, err
	}
	account.AccessKeySecretEncrypted = encrypted
	created, err := s.repo.CreateAccount(ctx, account)
	if err != nil {
		return store.Account{}, err
	}
	return sanitizeAccount(created), nil
}

func (s *Service) UpdateAccount(ctx context.Context, id string, input AccountInput) (store.Account, error) {
	if strings.TrimSpace(id) == "" {
		return store.Account{}, validation("账号 ID 不能为空")
	}
	if err := validateAccountInput(input, false); err != nil {
		return store.Account{}, err
	}
	current, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		return store.Account{}, mapStoreError(err)
	}
	current.AccessKeyID = strings.TrimSpace(input.AccessKeyID)
	current.Region = strings.TrimSpace(input.Region)
	current.Name = strings.TrimSpace(input.Name)
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	updateSecret := strings.TrimSpace(input.AccessKeySecret) != ""
	if updateSecret {
		encrypted, err := s.codec.Encrypt(input.AccessKeySecret)
		if err != nil {
			return store.Account{}, err
		}
		current.AccessKeySecretEncrypted = encrypted
	}
	updated, err := s.repo.UpdateAccount(ctx, current, updateSecret)
	if err != nil {
		return store.Account{}, err
	}
	return sanitizeAccount(updated), nil
}

func (s *Service) ListAccounts(ctx context.Context) ([]store.Account, error) {
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return sanitizeAccounts(accounts), nil
}

func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return validation("账号 ID 不能为空")
	}
	if err := s.repo.DeleteAccount(ctx, id); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func (s *Service) CreateInstance(ctx context.Context, input InstanceInput) (store.Instance, error) {
	instance, err := s.instanceFromInput(ctx, store.Instance{ID: uuid.NewString()}, input)
	if err != nil {
		return store.Instance{}, err
	}
	return s.repo.CreateInstance(ctx, instance)
}

func (s *Service) UpdateInstance(ctx context.Context, id string, input InstanceInput) (store.Instance, error) {
	if strings.TrimSpace(id) == "" {
		return store.Instance{}, validation("实例配置 ID 不能为空")
	}
	current, err := s.repo.GetInstance(ctx, id)
	if err != nil {
		return store.Instance{}, mapStoreError(err)
	}
	instance, err := s.instanceFromInput(ctx, current, input)
	if err != nil {
		return store.Instance{}, err
	}
	return s.repo.UpdateInstance(ctx, instance)
}

func (s *Service) DeleteInstance(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return validation("实例配置 ID 不能为空")
	}
	if err := s.repo.DeleteInstance(ctx, id); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func (s *Service) ListInstances(ctx context.Context) ([]store.Instance, error) {
	return s.repo.ListInstances(ctx)
}

func (s *Service) SyncNow(ctx context.Context) (SyncResult, error) {
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	instances, err := s.repo.ListInstances(ctx)
	if err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{Metadata: map[string]interface{}{
		"cloudMode": string(s.cloud.Mode()),
		"dryRun":    s.cloud.Mode() == aliyun.ModeDryRun,
	}}
	instancesByAccountRegion := make(map[string][]store.Instance)
	for _, instance := range instances {
		if instance.Enabled {
			key := instance.AccountID + "|" + instance.Region
			instancesByAccountRegion[key] = append(instancesByAccountRegion[key], instance)
		}
	}

	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		accountInstances := instancesByAccountRegion[account.ID+"|"+account.Region]
		if len(accountInstances) == 0 {
			continue
		}
		accountResult, err := s.syncAccount(ctx, account, accountInstances)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			s.logger.Error("sync account failed", "account_id", account.ID, "error", err.Error())
			continue
		}
		result.AccountID = accountResult.AccountID
		result.TrafficBytes = accountResult.TrafficBytes
		result.PeriodStart = accountResult.PeriodStart
		result.PeriodEnd = accountResult.PeriodEnd
		result.Instances = append(result.Instances, accountResult.Instances...)
		result.ProtectionLog = append(result.ProtectionLog, accountResult.ProtectionLog...)
	}

	if len(result.Instances) == 0 && len(result.Errors) == 0 {
		result.Errors = append(result.Errors, "没有可同步的启用账号和实例")
	}
	return result, nil
}

func (s *Service) StartInstance(ctx context.Context, id string) (store.ActionLog, error) {
	instance, account, err := s.instanceWithAccount(ctx, id)
	if err != nil {
		return store.ActionLog{}, err
	}
	creds, err := s.credentials(account)
	if err != nil {
		return store.ActionLog{}, err
	}
	err = s.cloud.StartInstance(ctx, creds, instance.InstanceID)
	now := time.Now().UTC()
	log := store.ActionLog{
		ID:            uuid.NewString(),
		AccountID:     account.ID,
		InstanceID:    instance.InstanceID,
		ActionType:    "start_instance",
		TriggerSource: "manual",
		Result:        "success",
		Reason:        "用户手动启动实例",
		CreatedAt:     now,
	}
	if err != nil {
		log.Result = "failed"
		log.ErrorMessage = err.Error()
		created, _ := s.repo.CreateActionLog(ctx, log)
		return created, fmt.Errorf("start instance: %w", err)
	}
	if err := s.repo.MarkManualStart(ctx, id); err != nil {
		return store.ActionLog{}, err
	}
	return s.repo.CreateActionLog(ctx, log)
}

func (s *Service) StopInstance(ctx context.Context, id string) (store.ActionLog, error) {
	instance, account, err := s.instanceWithAccount(ctx, id)
	if err != nil {
		return store.ActionLog{}, err
	}
	mode := aliyun.StopMode(instance.StopMode)
	creds, err := s.credentials(account)
	if err != nil {
		return store.ActionLog{}, err
	}
	err = s.cloud.StopInstance(ctx, creds, instance.InstanceID, mode)
	now := time.Now().UTC()
	log := store.ActionLog{
		ID:            uuid.NewString(),
		AccountID:     account.ID,
		InstanceID:    instance.InstanceID,
		ActionType:    "stop_instance",
		TriggerSource: "manual",
		Result:        "success",
		Reason:        "用户手动停止实例",
		StopMode:      instance.StopMode,
		CreatedAt:     now,
	}
	if err != nil {
		log.Result = "failed"
		log.ErrorMessage = err.Error()
		created, _ := s.repo.CreateActionLog(ctx, log)
		return created, fmt.Errorf("stop instance: %w", err)
	}
	if err := s.repo.MarkManualStop(ctx, id, now); err != nil {
		return store.ActionLog{}, err
	}
	return s.repo.CreateActionLog(ctx, log)
}

func (s *Service) ListActionLogs(ctx context.Context, limit int) ([]store.ActionLog, error) {
	return s.repo.ListActionLogs(ctx, limit)
}

func (s *Service) ListCloudEvents(ctx context.Context, limit int) ([]store.CloudEvent, error) {
	return s.repo.ListCloudEvents(ctx, limit)
}

func (s *Service) HandleWebhook(ctx context.Context, token string, input WebhookInput) (WebhookResult, error) {
	expected := s.webhookToken(ctx)
	if strings.TrimSpace(token) == "" || token != expected {
		return WebhookResult{}, validation("webhook token 不合法")
	}
	if strings.TrimSpace(input.EventID) == "" {
		return WebhookResult{}, validation("事件 ID 不能为空")
	}
	if strings.TrimSpace(input.InstanceID) == "" {
		return WebhookResult{}, validation("实例 ID 不能为空")
	}
	if input.EventTime.IsZero() {
		input.EventTime = time.Now().UTC()
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "Unknown"
	}
	raw := strings.TrimSpace(string(input.Raw))
	if raw == "" {
		raw = "{}"
	}
	event := store.CloudEvent{
		ID:            uuid.NewString(),
		EventID:       strings.TrimSpace(input.EventID),
		InstanceID:    strings.TrimSpace(input.InstanceID),
		EventTime:     input.EventTime.UTC(),
		Status:        status,
		RawPayload:    raw,
		ProcessStatus: "recorded",
		CreatedAt:     time.Now().UTC(),
	}
	created, duplicate, err := s.repo.CreateCloudEvent(ctx, event)
	if err != nil {
		return WebhookResult{}, err
	}
	result := WebhookResult{Event: created, Duplicate: duplicate}
	s.SendNotification(ctx, NotificationEvent{
		Type:    "aliyun_instance_stopped_event",
		Title:   "收到实例状态事件",
		Message: "收到阿里云云监控实例状态事件。",
		Payload: map[string]any{"instanceId": input.InstanceID, "status": status},
	})
	if duplicate || !isStoppedStatus(status) {
		return result, nil
	}
	action, ok, err := s.keepaliveInstance(ctx, input.InstanceID, "webhook")
	if err != nil {
		return result, err
	}
	if ok {
		result.Action = &action
	}
	return result, nil
}

func (s *Service) ReconcileKeepalive(ctx context.Context) ([]store.ActionLog, error) {
	instances, err := s.repo.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	var logs []store.ActionLog
	for _, instance := range instances {
		if !instance.KeepaliveEnabled || instance.LastStatus != "Stopped" {
			continue
		}
		log, ok, err := s.keepaliveInstance(ctx, instance.InstanceID, "reconcile")
		if err != nil {
			return logs, err
		}
		if ok {
			logs = append(logs, log)
		}
	}
	return logs, nil
}

func (s *Service) syncAccount(ctx context.Context, account store.Account, instances []store.Instance) (SyncResult, error) {
	creds, err := s.credentials(account)
	if err != nil {
		return SyncResult{}, err
	}
	usage, err := s.cloud.QueryTraffic(ctx, creds)
	if err != nil {
		log := store.ActionLog{
			ID:            uuid.NewString(),
			AccountID:     account.ID,
			ActionType:    "sync_traffic",
			TriggerSource: "scheduler",
			Result:        "failed",
			Reason:        "同步 CDT 流量失败",
			ErrorMessage:  err.Error(),
			CreatedAt:     time.Now().UTC(),
		}
		_, _ = s.repo.CreateActionLog(ctx, log)
		s.SendNotification(ctx, NotificationEvent{
			Type:    "aliyun_api_auth_failed",
			Title:   "阿里云 API 调用失败",
			Message: err.Error(),
			Payload: map[string]any{"accountId": account.ID, "operation": "query_traffic"},
		})
		return SyncResult{}, fmt.Errorf("query traffic: %w", err)
	}
	_, err = s.repo.CreateTrafficSnapshot(ctx, store.TrafficSnapshot{
		ID:          uuid.NewString(),
		AccountID:   account.ID,
		PeriodStart: usage.PeriodStart,
		PeriodEnd:   usage.PeriodEnd,
		TotalBytes:  usage.TotalBytes,
		CollectedAt: time.Now().UTC(),
	})
	if err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{
		AccountID:    account.ID,
		TrafficBytes: usage.TotalBytes,
		PeriodStart:  usage.PeriodStart,
		PeriodEnd:    usage.PeriodEnd,
	}
	for _, instance := range instances {
		status, err := s.cloud.QueryInstanceStatus(ctx, creds, instance.InstanceID)
		if err != nil {
			log := store.ActionLog{
				ID:            uuid.NewString(),
				AccountID:     account.ID,
				InstanceID:    instance.InstanceID,
				ActionType:    "sync_status",
				TriggerSource: "scheduler",
				Result:        "failed",
				Reason:        "同步 ECS 状态失败",
				ErrorMessage:  err.Error(),
				CreatedAt:     time.Now().UTC(),
			}
			_, _ = s.repo.CreateActionLog(ctx, log)
			s.SendNotification(ctx, NotificationEvent{
				Type:    "scheduled_sync_failed",
				Title:   "实例状态同步失败",
				Message: err.Error(),
				Payload: map[string]any{"instanceId": instance.InstanceID},
			})
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", instance.InstanceID, err.Error()))
			continue
		}
		if err := s.repo.UpdateInstanceSync(ctx, instance.ID, status.Status, usage.TotalBytes, time.Now().UTC()); err != nil {
			return SyncResult{}, err
		}
		updated, err := s.repo.GetInstance(ctx, instance.ID)
		if err != nil {
			return SyncResult{}, err
		}
		result.Instances = append(result.Instances, updated)

		if log, ok, err := s.evaluateProtection(ctx, account, updated, usage.TotalBytes); err != nil {
			result.Errors = append(result.Errors, err.Error())
		} else if ok {
			result.ProtectionLog = append(result.ProtectionLog, log)
		} else {
			s.notifyTrafficApproaching(ctx, updated, usage.TotalBytes)
		}
	}
	return result, nil
}

func (s *Service) evaluateProtection(ctx context.Context, account store.Account, instance store.Instance, trafficBytes int64) (store.ActionLog, bool, error) {
	if !instance.Enabled || !instance.ProtectionEnabled {
		return store.ActionLog{}, false, nil
	}
	if trafficBytes < instance.TrafficLimitBytes {
		return store.ActionLog{}, false, nil
	}
	if instance.LastStatus == "Stopped" || instance.LastStatus == "Stopping" {
		return store.ActionLog{}, false, nil
	}

	now := time.Now().UTC()
	creds, err := s.credentials(account)
	if err != nil {
		return store.ActionLog{}, false, err
	}
	err = s.cloud.StopInstance(ctx, creds, instance.InstanceID, aliyun.StopMode(instance.StopMode))
	traffic := trafficBytes
	threshold := instance.TrafficLimitBytes
	log := store.ActionLog{
		ID:             uuid.NewString(),
		AccountID:      account.ID,
		InstanceID:     instance.InstanceID,
		ActionType:     "auto_stop_instance",
		TriggerSource:  "threshold",
		Result:         "success",
		Reason:         "CDT 流量达到或超过实例阈值",
		TrafficBytes:   &traffic,
		ThresholdBytes: &threshold,
		StopMode:       instance.StopMode,
		CreatedAt:      now,
	}
	if err != nil {
		log.Result = "failed"
		log.ErrorMessage = err.Error()
		created, _ := s.repo.CreateActionLog(ctx, log)
		s.SendNotification(ctx, NotificationEvent{
			Type:    "traffic_limit_auto_stop_failed",
			Title:   "流量超限停机失败",
			Message: err.Error(),
			Payload: map[string]any{"instanceId": instance.InstanceID},
		})
		return created, true, fmt.Errorf("auto stop instance %s: %w", instance.InstanceID, err)
	}
	if err := s.repo.MarkProtectionStop(ctx, instance.ID, now); err != nil {
		return store.ActionLog{}, false, err
	}
	created, err := s.repo.CreateActionLog(ctx, log)
	s.SendNotification(ctx, NotificationEvent{
		Type:    "traffic_limit_auto_stop",
		Title:   "流量超限自动停机",
		Message: "CDT 流量达到或超过实例阈值，已执行停机保护。",
		Payload: map[string]any{"instanceId": instance.InstanceID, "trafficBytes": trafficBytes, "thresholdBytes": instance.TrafficLimitBytes},
	})
	return created, true, err
}

func (s *Service) notifyTrafficApproaching(ctx context.Context, instance store.Instance, trafficBytes int64) {
	if !instance.Enabled || !instance.ProtectionEnabled || instance.TrafficLimitBytes <= 0 {
		return
	}
	if trafficBytes < int64(float64(instance.TrafficLimitBytes)*0.8) || trafficBytes >= instance.TrafficLimitBytes {
		return
	}
	s.SendNotification(ctx, NotificationEvent{
		Type:    "traffic_approaching_limit",
		Title:   "流量接近阈值",
		Message: "CDT 流量已达到阈值的 80%。",
		Payload: map[string]any{
			"instanceId":     instance.InstanceID,
			"trafficBytes":   trafficBytes,
			"thresholdBytes": instance.TrafficLimitBytes,
		},
	})
}

func (s *Service) keepaliveInstance(ctx context.Context, cloudInstanceID, source string) (store.ActionLog, bool, error) {
	instance, err := s.repo.GetInstanceByCloudID(ctx, cloudInstanceID)
	if err != nil {
		if errors.Is(mapStoreError(err), ErrNotFound) {
			return store.ActionLog{}, false, nil
		}
		return store.ActionLog{}, false, err
	}
	if !instance.Enabled || !instance.KeepaliveEnabled {
		return store.ActionLog{}, false, nil
	}
	if instance.StoppedByProtectionAt != nil || instance.ManualStopAt != nil {
		return store.ActionLog{}, false, nil
	}
	if instance.LastKeepaliveAt != nil && time.Since(*instance.LastKeepaliveAt) < time.Duration(instance.KeepaliveCooldownSeconds)*time.Second {
		return store.ActionLog{}, false, nil
	}
	account, err := s.repo.GetAccount(ctx, instance.AccountID)
	if err != nil {
		return store.ActionLog{}, false, mapStoreError(err)
	}
	creds, err := s.credentials(account)
	if err != nil {
		return store.ActionLog{}, false, err
	}
	status, err := s.cloud.QueryInstanceStatus(ctx, creds, instance.InstanceID)
	if err != nil {
		log := store.ActionLog{
			ID:            uuid.NewString(),
			AccountID:     account.ID,
			InstanceID:    instance.InstanceID,
			ActionType:    "keepalive_start",
			TriggerSource: source,
			Result:        "failed",
			Reason:        "保活启动前确认实例状态失败",
			ErrorMessage:  err.Error(),
			CreatedAt:     time.Now().UTC(),
		}
		created, _ := s.repo.CreateActionLog(ctx, log)
		return created, true, err
	}
	if !isStoppedStatus(status.Status) {
		_ = s.repo.UpdateInstanceSync(ctx, instance.ID, status.Status, instance.LastTrafficBytes, time.Now().UTC())
		return store.ActionLog{}, false, nil
	}
	now := time.Now().UTC()
	err = s.cloud.StartInstance(ctx, creds, instance.InstanceID)
	log := store.ActionLog{
		ID:            uuid.NewString(),
		AccountID:     account.ID,
		InstanceID:    instance.InstanceID,
		ActionType:    "keepalive_start",
		TriggerSource: source,
		Result:        "success",
		Reason:        "实例停止事件触发保活启动",
		CreatedAt:     now,
	}
	if err != nil {
		log.Result = "failed"
		log.ErrorMessage = err.Error()
		created, _ := s.repo.CreateActionLog(ctx, log)
		s.SendNotification(ctx, NotificationEvent{
			Type:    "keepalive_start_failed",
			Title:   "保活启动失败",
			Message: err.Error(),
			Payload: map[string]any{"instanceId": instance.InstanceID},
		})
		return created, true, err
	}
	if err := s.repo.MarkKeepaliveStart(ctx, instance.ID, now); err != nil {
		return store.ActionLog{}, false, err
	}
	created, err := s.repo.CreateActionLog(ctx, log)
	s.SendNotification(ctx, NotificationEvent{
		Type:    "keepalive_start_success",
		Title:   "保活启动成功",
		Message: "实例停止后已按保活规则启动。",
		Payload: map[string]any{"instanceId": instance.InstanceID, "source": source},
	})
	return created, true, err
}

func (s *Service) instanceFromInput(ctx context.Context, current store.Instance, input InstanceInput) (store.Instance, error) {
	if err := validateInstanceInput(input); err != nil {
		return store.Instance{}, err
	}
	if _, err := s.repo.GetAccount(ctx, strings.TrimSpace(input.AccountID)); err != nil {
		return store.Instance{}, mapStoreError(err)
	}
	isUpdate := current.AccountID != ""
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	} else if isUpdate {
		enabled = current.Enabled
	}
	protectionEnabled := true
	if input.ProtectionEnabled != nil {
		protectionEnabled = *input.ProtectionEnabled
	} else if isUpdate {
		protectionEnabled = current.ProtectionEnabled
	}
	keepaliveEnabled := false
	if input.KeepaliveEnabled != nil {
		keepaliveEnabled = *input.KeepaliveEnabled
	} else if isUpdate {
		keepaliveEnabled = current.KeepaliveEnabled
	}
	cooldown := input.KeepaliveCooldownSeconds
	if cooldown == 0 {
		cooldown = 300
	}
	monthlyRestoreEnabled := false
	if input.MonthlyRestoreEnabled != nil {
		monthlyRestoreEnabled = *input.MonthlyRestoreEnabled
	} else if isUpdate {
		monthlyRestoreEnabled = current.MonthlyRestoreEnabled
	}
	current.AccountID = strings.TrimSpace(input.AccountID)
	current.InstanceID = strings.TrimSpace(input.InstanceID)
	current.Name = strings.TrimSpace(input.Name)
	current.TrafficLimitBytes = input.TrafficLimitBytes
	current.StopMode = strings.TrimSpace(input.StopMode)
	current.Enabled = enabled
	current.ProtectionEnabled = protectionEnabled
	current.KeepaliveEnabled = keepaliveEnabled
	current.KeepaliveCooldownSeconds = cooldown
	current.MonthlyRestoreEnabled = monthlyRestoreEnabled
	return current, nil
}

func (s *Service) instanceWithAccount(ctx context.Context, id string) (store.Instance, store.Account, error) {
	if strings.TrimSpace(id) == "" {
		return store.Instance{}, store.Account{}, validation("实例配置 ID 不能为空")
	}
	instance, err := s.repo.GetInstance(ctx, id)
	if err != nil {
		return store.Instance{}, store.Account{}, mapStoreError(err)
	}
	account, err := s.repo.GetAccount(ctx, instance.AccountID)
	if err != nil {
		return store.Instance{}, store.Account{}, mapStoreError(err)
	}
	return instance, account, nil
}

func validateAccountInput(input AccountInput, requireSecret bool) error {
	if strings.TrimSpace(input.AccessKeyID) == "" {
		return validation("AccessKey ID 不能为空")
	}
	if requireSecret && strings.TrimSpace(input.AccessKeySecret) == "" {
		return validation("AccessKey Secret 不能为空")
	}
	if strings.TrimSpace(input.Region) == "" {
		return validation("区域不能为空")
	}
	if !aliyun.IsValidRegionID(input.Region) {
		return validation("区域格式不合法")
	}
	return nil
}

func validateInstanceInput(input InstanceInput) error {
	if strings.TrimSpace(input.AccountID) == "" {
		return validation("账号 ID 不能为空")
	}
	if strings.TrimSpace(input.InstanceID) == "" {
		return validation("实例 ID 不能为空")
	}
	if input.TrafficLimitBytes <= 0 {
		return validation("流量阈值必须大于 0")
	}
	mode := strings.TrimSpace(input.StopMode)
	if mode == "" {
		return validation("停机模式不能为空")
	}
	if mode != StopModeKeepCharging && mode != StopModeStopCharging {
		return validation("停机模式必须是 KeepCharging 或 StopCharging")
	}
	if input.KeepaliveCooldownSeconds < 0 {
		return validation("保活冷却时间不能小于 0")
	}
	return nil
}

func (s *Service) credentials(account store.Account) (aliyun.AccountCredentials, error) {
	secret, err := s.codec.Decrypt(account.AccessKeySecretEncrypted)
	if err != nil {
		return aliyun.AccountCredentials{}, err
	}
	return aliyun.AccountCredentials{
		AccessKeyID:     account.AccessKeyID,
		AccessKeySecret: secret,
		Region:          account.Region,
	}, nil
}

func sanitizeAccount(account store.Account) store.Account {
	account.AccessKeySecretEncrypted = ""
	return account
}

func sanitizeAccounts(accounts []store.Account) []store.Account {
	out := make([]store.Account, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, sanitizeAccount(account))
	}
	return out
}

func mapStoreError(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Service) webhookToken(ctx context.Context) string {
	token, err := s.repo.GetSetting(ctx, "webhook_token")
	if err != nil || strings.TrimSpace(token) == "" {
		first := strings.ReplaceAll(uuid.NewString(), "-", "")
		second := strings.ReplaceAll(uuid.NewString(), "-", "")
		token = first + second[:16]
		_ = s.repo.SetSetting(ctx, "webhook_token", token)
	}
	return token
}

func isStoppedStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == "stopped" || normalized == "stop" || normalized == "instance_stopped"
}
