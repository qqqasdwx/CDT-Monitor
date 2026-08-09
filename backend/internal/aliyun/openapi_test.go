package aliyun

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRPCExecutor struct {
	responses map[string]map[string]interface{}
	requests  []rpcRequest
	err       error
}

func (f *fakeRPCExecutor) Execute(_ context.Context, _ AccountCredentials, request rpcRequest) (map[string]interface{}, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	return f.responses[request.Action], nil
}

func TestParseMode(t *testing.T) {
	t.Parallel()

	mode, err := ParseMode(" READ-ONLY ")
	if err != nil || mode != ModeReadOnly {
		t.Fatalf("ParseMode() = %q, %v", mode, err)
	}
	if _, err := ParseMode("enabled"); err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestIsValidRegionID(t *testing.T) {
	t.Parallel()

	for _, region := range []string{"cn-hangzhou", "ap-southeast-1", "cn-hongkong"} {
		if !IsValidRegionID(region) {
			t.Fatalf("expected valid region %q", region)
		}
	}
	for _, region := range []string{"", "CN-HANGZHOU", "ecs.example.com", "cn-hangzhou:443", "-cn-hangzhou"} {
		if IsValidRegionID(region) {
			t.Fatalf("expected invalid region %q", region)
		}
	}
}

func TestQueryTrafficAggregatesMatchingRegionClass(t *testing.T) {
	t.Parallel()

	executor := &fakeRPCExecutor{responses: map[string]map[string]interface{}{
		"ListCdtInternetTraffic": {
			"TrafficDetails": []interface{}{
				map[string]interface{}{"BusinessRegionId": "cn-hangzhou", "Traffic": float64(1024)},
				map[string]interface{}{"BusinessRegionId": "cn-shanghai", "Traffic": "2048"},
				map[string]interface{}{"BusinessRegionId": "ap-southeast-1", "Traffic": float64(4096)},
			},
		},
	}}
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	client := newOpenAPIClient(ModeReadOnly, executor, func() time.Time { return now })

	usage, err := client.QueryTraffic(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("QueryTraffic() error = %v", err)
	}
	if usage.TotalBytes != 3072 {
		t.Fatalf("TotalBytes = %d, want 3072", usage.TotalBytes)
	}
	wantStart := time.Date(2026, time.July, 31, 16, 0, 0, 0, time.UTC)
	if !usage.PeriodStart.Equal(wantStart) || !usage.PeriodEnd.Equal(now) {
		t.Fatalf("unexpected period: %s - %s", usage.PeriodStart, usage.PeriodEnd)
	}
	if len(executor.requests) != 1 || executor.requests[0].Endpoint != cdtEndpoint {
		t.Fatalf("unexpected request: %+v", executor.requests)
	}
}

func TestQueryTrafficRejectsMalformedResponse(t *testing.T) {
	t.Parallel()

	executor := &fakeRPCExecutor{responses: map[string]map[string]interface{}{
		"ListCdtInternetTraffic": {
			"TrafficDetails": []interface{}{
				map[string]interface{}{"BusinessRegionId": "", "Traffic": float64(1024)},
			},
		},
	}}
	client := newOpenAPIClient(ModeReadOnly, executor, time.Now)
	_, err := client.QueryTraffic(context.Background(), testCredentials())
	var apiErr APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "InvalidResponse" {
		t.Fatalf("error = %v, want InvalidResponse", err)
	}
}

func TestQueryInstanceStatus(t *testing.T) {
	t.Parallel()

	executor := &fakeRPCExecutor{responses: map[string]map[string]interface{}{
		"DescribeInstanceStatus": {
			"InstanceStatuses": map[string]interface{}{
				"InstanceStatus": []interface{}{
					map[string]interface{}{"InstanceId": "i-other", "Status": "Stopped"},
					map[string]interface{}{"InstanceId": "i-test", "Status": "Running"},
				},
			},
		},
	}}
	client := newOpenAPIClient(ModeReadOnly, executor, time.Now)
	status, err := client.QueryInstanceStatus(context.Background(), testCredentials(), "i-test")
	if err != nil {
		t.Fatalf("QueryInstanceStatus() error = %v", err)
	}
	if status.Status != "Running" || status.InstanceID != "i-test" {
		t.Fatalf("unexpected status: %+v", status)
	}
	request := executor.requests[0]
	if request.Endpoint != "ecs.cn-hangzhou.aliyuncs.com" || request.Query["InstanceId.1"] != "i-test" {
		t.Fatalf("unexpected request: %+v", request)
	}
}

func TestReadOnlyModeBlocksMutations(t *testing.T) {
	t.Parallel()

	executor := &fakeRPCExecutor{}
	client := newOpenAPIClient(ModeReadOnly, executor, time.Now)
	err := client.StopInstance(context.Background(), testCredentials(), "i-test", StopModeStopCharging)
	var apiErr APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "OperationDisabled" {
		t.Fatalf("error = %v, want OperationDisabled", err)
	}
	if len(executor.requests) != 0 {
		t.Fatalf("read-only mode executed requests: %+v", executor.requests)
	}
}

func TestLiveModeSendsStoppedMode(t *testing.T) {
	t.Parallel()

	executor := &fakeRPCExecutor{responses: map[string]map[string]interface{}{
		"StopInstance": {},
	}}
	client := newOpenAPIClient(ModeLive, executor, time.Now)
	if err := client.StopInstance(context.Background(), testCredentials(), "i-test", StopModeStopCharging); err != nil {
		t.Fatalf("StopInstance() error = %v", err)
	}
	if len(executor.requests) != 1 || executor.requests[0].Query["StoppedMode"] != "StopCharging" {
		t.Fatalf("unexpected request: %+v", executor.requests)
	}
}

func testCredentials() AccountCredentials {
	return AccountCredentials{
		AccessKeyID:     "test-access-key",
		AccessKeySecret: "test-access-secret",
		Region:          "cn-hangzhou",
	}
}
