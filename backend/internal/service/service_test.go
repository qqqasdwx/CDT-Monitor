package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cdt-monitor/backend/internal/aliyun"
	"cdt-monitor/backend/internal/logger"
	"cdt-monitor/backend/internal/secrets"
	"cdt-monitor/backend/internal/store"
)

type fakeCloud struct {
	trafficBytes int64
	status       string
	stopped      bool
	started      bool
	stopMode     aliyun.StopMode
}

func (f *fakeCloud) QueryTraffic(context.Context, aliyun.AccountCredentials) (aliyun.TrafficUsage, error) {
	now := time.Now().UTC()
	return aliyun.TrafficUsage{
		TotalBytes:  f.trafficBytes,
		PeriodStart: now.Add(-time.Hour),
		PeriodEnd:   now,
	}, nil
}

func (f *fakeCloud) QueryInstanceStatus(context.Context, aliyun.AccountCredentials, string) (aliyun.InstanceStatus, error) {
	return aliyun.InstanceStatus{InstanceID: "i-test", Status: f.status}, nil
}

func (f *fakeCloud) QueryAccountBalance(context.Context, aliyun.AccountCredentials) (aliyun.CostSnapshot, error) {
	return aliyun.CostSnapshot{
		AvailableAmount: 12.5,
		CreditAmount:    0,
		Currency:        "CNY",
		CollectedAt:     time.Now().UTC(),
	}, nil
}

func (f *fakeCloud) StartInstance(context.Context, aliyun.AccountCredentials, string) error {
	f.started = true
	return nil
}

func (f *fakeCloud) StopInstance(_ context.Context, _ aliyun.AccountCredentials, _ string, mode aliyun.StopMode) error {
	f.stopped = true
	f.stopMode = mode
	return nil
}

func TestSyncNowStopsInstanceWhenThresholdExceeded(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	cloud := &fakeCloud{trafficBytes: 200, status: "Running"}
	svc := New(repo, cloud, testCodec(t), logger.New(nil, logger.LevelError))

	account, err := svc.CreateAccount(context.Background(), AccountInput{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Region:          "cn-hangzhou",
		Name:            "test",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if account.AccessKeySecretEncrypted != "" {
		t.Fatalf("secret leaked in API model")
	}

	instance, err := svc.CreateInstance(context.Background(), InstanceInput{
		AccountID:         account.ID,
		InstanceID:        "i-test",
		Name:              "vm",
		TrafficLimitBytes: 100,
		StopMode:          StopModeStopCharging,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	result, err := svc.SyncNow(context.Background())
	if err != nil {
		t.Fatalf("sync now: %v", err)
	}
	if !cloud.stopped {
		t.Fatalf("expected cloud stop to be called")
	}
	if cloud.stopMode != aliyun.StopModeStopCharging {
		t.Fatalf("unexpected stop mode: %s", cloud.stopMode)
	}
	if len(result.ProtectionLog) != 1 {
		t.Fatalf("expected one protection log, got %d", len(result.ProtectionLog))
	}
	updated, err := repo.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.StoppedByProtectionAt == nil {
		t.Fatalf("expected protection stop marker")
	}
}

func TestManualStopSetsManualMarker(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	cloud := &fakeCloud{trafficBytes: 0, status: "Running"}
	svc := New(repo, cloud, testCodec(t), logger.New(nil, logger.LevelError))

	account, err := svc.CreateAccount(context.Background(), AccountInput{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Region:          "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	instance, err := svc.CreateInstance(context.Background(), InstanceInput{
		AccountID:         account.ID,
		InstanceID:        "i-test",
		TrafficLimitBytes: 1024,
		StopMode:          StopModeKeepCharging,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	log, err := svc.StopInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("stop instance: %v", err)
	}
	if log.Result != "success" || log.TriggerSource != "manual" {
		t.Fatalf("unexpected log: %+v", log)
	}
	updated, err := repo.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.ManualStopAt == nil {
		t.Fatalf("expected manual stop marker")
	}
}

func TestWebhookStoppedEventTriggersKeepalive(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	cloud := &fakeCloud{trafficBytes: 0, status: "Stopped"}
	svc := New(repo, cloud, testCodec(t), logger.New(nil, logger.LevelError))

	account, err := svc.CreateAccount(context.Background(), AccountInput{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Region:          "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	keepalive := true
	instance, err := svc.CreateInstance(context.Background(), InstanceInput{
		AccountID:         account.ID,
		InstanceID:        "i-test",
		TrafficLimitBytes: 1024,
		StopMode:          StopModeKeepCharging,
		KeepaliveEnabled:  &keepalive,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if err := repo.UpdateInstanceSync(context.Background(), instance.ID, "Stopped", 0, time.Now().UTC()); err != nil {
		t.Fatalf("update sync: %v", err)
	}

	dashboard, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	token := dashboard.WebhookURL[strings.LastIndex(dashboard.WebhookURL, "/")+1:]
	result, err := svc.HandleWebhook(context.Background(), token, WebhookInput{
		EventID:    "evt-1",
		InstanceID: "i-test",
		EventTime:  time.Now().UTC(),
		Status:     "Stopped",
		Raw:        []byte(`{"eventId":"evt-1"}`),
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.Action == nil || !cloud.started {
		t.Fatalf("expected keepalive action and cloud start")
	}
	if result.Duplicate {
		t.Fatalf("first event should not be duplicate")
	}

	cloud.started = false
	duplicate, err := svc.HandleWebhook(context.Background(), token, WebhookInput{
		EventID:    "evt-1",
		InstanceID: "i-test",
		EventTime:  result.Event.EventTime,
		Status:     "Stopped",
		Raw:        []byte(`{"eventId":"evt-1"}`),
	})
	if err != nil {
		t.Fatalf("handle duplicate webhook: %v", err)
	}
	if !duplicate.Duplicate || cloud.started {
		t.Fatalf("duplicate event should not start instance")
	}
}

func TestKeepaliveSkipsProtectionStoppedInstance(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	cloud := &fakeCloud{trafficBytes: 2048, status: "Running"}
	svc := New(repo, cloud, testCodec(t), logger.New(nil, logger.LevelError))

	account, err := svc.CreateAccount(context.Background(), AccountInput{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Region:          "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	keepalive := true
	instance, err := svc.CreateInstance(context.Background(), InstanceInput{
		AccountID:         account.ID,
		InstanceID:        "i-test",
		TrafficLimitBytes: 1024,
		StopMode:          StopModeKeepCharging,
		KeepaliveEnabled:  &keepalive,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := svc.SyncNow(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := repo.UpdateInstanceSync(context.Background(), instance.ID, "Stopped", 2048, time.Now().UTC()); err != nil {
		t.Fatalf("update sync: %v", err)
	}
	cloud.status = "Stopped"
	cloud.started = false
	logs, err := svc.ReconcileKeepalive(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(logs) != 0 || cloud.started {
		t.Fatalf("protection stopped instance should not be restarted")
	}
}

func TestWebhookNotificationLogsResult(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	svc := New(repo, &fakeCloud{}, testCodec(t), logger.New(nil, logger.LevelError))

	received := make(chan NotificationEvent, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event NotificationEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode event: %v", err)
		}
		received <- event
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel, err := svc.CreateNotificationChannel(context.Background(), NotificationChannelInput{
		Name: "ops",
		Type: NotificationTypeWebhook,
		Config: map[string]string{
			"url": server.URL,
		},
	})
	if err != nil {
		t.Fatalf("create notification channel: %v", err)
	}
	log, err := svc.TestNotification(context.Background(), channel.ID)
	if err != nil {
		t.Fatalf("test notification: %v", err)
	}
	if log.Result != "success" {
		t.Fatalf("expected success log, got %+v", log)
	}
	select {
	case event := <-received:
		if event.Type != "test_notification" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatalf("webhook did not receive event")
	}
}

func TestNotificationFailureIsLogged(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	svc := New(repo, &fakeCloud{}, testCodec(t), logger.New(nil, logger.LevelError))

	channel, err := svc.CreateNotificationChannel(context.Background(), NotificationChannelInput{
		Name:   "mail",
		Type:   NotificationTypeEmail,
		Config: map[string]string{},
	})
	if err != nil {
		t.Fatalf("create notification channel: %v", err)
	}
	log, err := svc.TestNotification(context.Background(), channel.ID)
	if err == nil {
		t.Fatalf("expected email sender error")
	}
	if log.Result != "failed" || log.ErrorMessage == "" {
		t.Fatalf("expected failed log with error message, got %+v", log)
	}
}

func TestDueScheduledTaskRunsOnce(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	cloud := &fakeCloud{trafficBytes: 0, status: "Running"}
	svc := New(repo, cloud, testCodec(t), logger.New(nil, logger.LevelError))

	account, err := svc.CreateAccount(context.Background(), AccountInput{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Region:          "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	instance, err := svc.CreateInstance(context.Background(), InstanceInput{
		AccountID:         account.ID,
		InstanceID:        "i-test",
		TrafficLimitBytes: 1024,
		StopMode:          StopModeKeepCharging,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	task, err := svc.CreateScheduledTask(context.Background(), ScheduledTaskInput{
		InstanceConfigID: instance.ID,
		Action:           "stop",
		RunAt:            time.Now().UTC().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	logs, err := svc.RunDueScheduledTasks(context.Background())
	if err != nil {
		t.Fatalf("run due tasks: %v", err)
	}
	if len(logs) != 1 || !cloud.stopped {
		t.Fatalf("expected one stop action, logs=%d stopped=%v", len(logs), cloud.stopped)
	}
	updated, err := repo.GetScheduledTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updated.LastRunAt == nil {
		t.Fatalf("expected last run marker")
	}
	cloud.stopped = false
	logs, err = svc.RunDueScheduledTasks(context.Background())
	if err != nil {
		t.Fatalf("run due tasks second time: %v", err)
	}
	if len(logs) != 0 || cloud.stopped {
		t.Fatalf("task should not run twice")
	}
}

func TestSyncCostsDoesNotAffectTrafficProtection(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	svc := New(repo, &fakeCloud{}, testCodec(t), logger.New(nil, logger.LevelError))

	if _, err := svc.CreateAccount(context.Background(), AccountInput{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Region:          "cn-hangzhou",
	}); err != nil {
		t.Fatalf("create account: %v", err)
	}
	snapshots, err := svc.SyncCosts(context.Background())
	if err != nil {
		t.Fatalf("sync costs: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].AvailableAmount != 12.5 {
		t.Fatalf("unexpected snapshots: %+v", snapshots)
	}
	latest, err := svc.ListCostSnapshots(context.Background())
	if err != nil {
		t.Fatalf("list costs: %v", err)
	}
	if len(latest) != 1 {
		t.Fatalf("expected latest cost snapshot")
	}
}

func testCodec(t *testing.T) *secrets.Codec {
	t.Helper()
	codec, err := secrets.NewCodecFromKey([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	return codec
}

func newTestRepo(t *testing.T) *store.Repository {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return store.New(db)
}
