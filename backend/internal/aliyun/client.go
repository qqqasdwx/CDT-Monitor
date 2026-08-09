package aliyun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cdt-monitor/backend/internal/logger"
)

type Mode string

const (
	ModeDryRun   Mode = "dry-run"
	ModeReadOnly Mode = "read-only"
	ModeLive     Mode = "live"
)

func ParseMode(value string) (Mode, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(value)))
	switch mode {
	case ModeDryRun, ModeReadOnly, ModeLive:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid aliyun mode %q: expected dry-run, read-only, or live", value)
	}
}

func IsValidRegionID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 63 {
		return false
	}
	for index, char := range value {
		isLetter := char >= 'a' && char <= 'z'
		isDigit := char >= '0' && char <= '9'
		if !isLetter && !isDigit && char != '-' {
			return false
		}
		if char == '-' && (index == 0 || index == len(value)-1) {
			return false
		}
	}
	return true
}

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
	code := strings.ToLower(apiErr.Code)
	return strings.Contains(code, "invalidaccesskey") ||
		strings.Contains(code, "signaturedoesnotmatch") ||
		strings.Contains(code, "forbidden") ||
		strings.Contains(code, "nopermission") ||
		strings.Contains(code, "accessdenied")
}

type StopMode string

const (
	StopModeKeepCharging StopMode = "KeepCharging"
	StopModeStopCharging StopMode = "StopCharging"
)

type Client interface {
	Mode() Mode
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

func (c *DryRunClient) Mode() Mode {
	return ModeDryRun
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
