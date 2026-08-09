package aliyun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cdt-monitor/backend/internal/logger"
)

type AccountCredentials struct {
	AccessKeyID     string
	AccessKeySecret string
	Region          string
}

type InstanceStatus struct {
	InstanceID string
	Status     string
}

type TrafficUsage struct {
	TotalBytes  int64
	PeriodStart time.Time
	PeriodEnd   time.Time
}

type CostSnapshot struct {
	AvailableAmount float64
	CreditAmount    float64
	Currency        string
	CollectedAt     time.Time
}

type APIError struct {
	Operation string
	Code      string
	Message   string
	Retryable bool
}

func (e APIError) Error() string {
	return fmt.Sprintf("%s failed: %s %s", e.Operation, e.Code, e.Message)
}

func IsAuthError(err error) bool {
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == "InvalidAccessKeyId.NotFound" ||
		apiErr.Code == "InvalidAccessKeySecret" ||
		apiErr.Code == "Forbidden" ||
		apiErr.Code == "NoPermission"
}

type StopMode string

const (
	StopModeKeepCharging StopMode = "KeepCharging"
	StopModeStopCharging StopMode = "StopCharging"
)

type Client interface {
	QueryTraffic(ctx context.Context, account AccountCredentials) (TrafficUsage, error)
	QueryInstanceStatus(ctx context.Context, account AccountCredentials, instanceID string) (InstanceStatus, error)
	QueryAccountBalance(ctx context.Context, account AccountCredentials) (CostSnapshot, error)
	StartInstance(ctx context.Context, account AccountCredentials, instanceID string) error
	StopInstance(ctx context.Context, account AccountCredentials, instanceID string, mode StopMode) error
}

type DryRunClient struct {
	logger *logger.Logger
}

func NewDryRunClient(logger *logger.Logger) *DryRunClient {
	return &DryRunClient{logger: logger}
}

func (c *DryRunClient) QueryTraffic(_ context.Context, account AccountCredentials) (TrafficUsage, error) {
	now := time.Now().UTC()
	c.logger.Info("dry-run query traffic", "access_key_id", account.AccessKeyID, "region", account.Region)
	return TrafficUsage{
		TotalBytes:  0,
		PeriodStart: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   now,
	}, nil
}

func (c *DryRunClient) QueryInstanceStatus(_ context.Context, account AccountCredentials, instanceID string) (InstanceStatus, error) {
	c.logger.Info("dry-run query instance status", "access_key_id", account.AccessKeyID, "region", account.Region, "instance_id", instanceID)
	return InstanceStatus{InstanceID: instanceID, Status: "Running"}, nil
}

func (c *DryRunClient) QueryAccountBalance(_ context.Context, account AccountCredentials) (CostSnapshot, error) {
	c.logger.Info("dry-run query account balance", "access_key_id", account.AccessKeyID, "region", account.Region)
	return CostSnapshot{
		AvailableAmount: 0,
		CreditAmount:    0,
		Currency:        "CNY",
		CollectedAt:     time.Now().UTC(),
	}, nil
}

func (c *DryRunClient) StartInstance(_ context.Context, account AccountCredentials, instanceID string) error {
	c.logger.Info("dry-run start instance", "access_key_id", account.AccessKeyID, "region", account.Region, "instance_id", instanceID)
	return nil
}

func (c *DryRunClient) StopInstance(_ context.Context, account AccountCredentials, instanceID string, mode StopMode) error {
	c.logger.Info("dry-run stop instance", "access_key_id", account.AccessKeyID, "region", account.Region, "instance_id", instanceID, "mode", string(mode))
	return nil
}
