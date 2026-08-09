package aliyun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapimodels "github.com/alibabacloud-go/darabonba-openapi/v2/models"
	openapiutils "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
)

const (
	cdtEndpoint   = "cdt.aliyuncs.com"
	cdtRegionID   = "cn-hangzhou"
	cdtAPIVersion = "2021-08-13"
	ecsAPIVersion = "2014-05-26"
)

type OpenAPIOptions struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
}

type OpenAPIClient struct {
	mode     Mode
	executor rpcExecutor
	now      func() time.Time
}

type rpcRequest struct {
	Operation string
	Action    string
	Version   string
	Endpoint  string
	RegionID  string
	Query     map[string]string
}

type rpcExecutor interface {
	Execute(ctx context.Context, account AccountCredentials, request rpcRequest) (map[string]interface{}, error)
}

type sdkRPCExecutor struct {
	connectTimeout time.Duration
	readTimeout    time.Duration
}

func NewOpenAPIClient(mode Mode, options OpenAPIOptions) (*OpenAPIClient, error) {
	if mode != ModeReadOnly && mode != ModeLive {
		return nil, fmt.Errorf("openapi client requires read-only or live mode, got %q", mode)
	}
	if options.ConnectTimeout <= 0 {
		options.ConnectTimeout = 5 * time.Second
	}
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = 10 * time.Second
	}
	return newOpenAPIClient(mode, &sdkRPCExecutor{
		connectTimeout: options.ConnectTimeout,
		readTimeout:    options.ReadTimeout,
	}, time.Now), nil
}

func newOpenAPIClient(mode Mode, executor rpcExecutor, now func() time.Time) *OpenAPIClient {
	return &OpenAPIClient{mode: mode, executor: executor, now: now}
}

func (c *OpenAPIClient) Mode() Mode {
	return c.mode
}

func (c *OpenAPIClient) QueryTraffic(ctx context.Context, account AccountCredentials) (TrafficUsage, error) {
	if err := validateAccountCredentials(account); err != nil {
		return TrafficUsage{}, err
	}
	body, err := c.executor.Execute(ctx, account, rpcRequest{
		Operation: "query CDT internet traffic",
		Action:    "ListCdtInternetTraffic",
		Version:   cdtAPIVersion,
		Endpoint:  cdtEndpoint,
		RegionID:  cdtRegionID,
	})
	if err != nil {
		return TrafficUsage{}, err
	}

	var response cdtTrafficResponse
	if err := decodeBody(body, &response); err != nil {
		return TrafficUsage{}, invalidResponse("query CDT internet traffic", err)
	}
	if len(response.TrafficDetails) == 0 {
		return TrafficUsage{}, invalidResponse("query CDT internet traffic", errors.New("TrafficDetails is empty"))
	}

	targetOverseas := isOverseasRegion(account.Region)
	var total int64
	for _, detail := range response.TrafficDetails {
		region := strings.TrimSpace(detail.BusinessRegionID)
		if region == "" {
			return TrafficUsage{}, invalidResponse("query CDT internet traffic", errors.New("TrafficDetails contains an empty BusinessRegionId"))
		}
		if isOverseasRegion(region) != targetOverseas {
			continue
		}
		traffic := int64(detail.Traffic)
		if traffic > math.MaxInt64-total {
			return TrafficUsage{}, invalidResponse("query CDT internet traffic", errors.New("traffic total overflows int64"))
		}
		total += traffic
	}

	now := c.now().UTC()
	billingLocation := time.FixedZone("Asia/Shanghai", 8*60*60)
	billingNow := now.In(billingLocation)
	periodStart := time.Date(billingNow.Year(), billingNow.Month(), 1, 0, 0, 0, 0, billingLocation).UTC()
	return TrafficUsage{
		TotalBytes:  total,
		PeriodStart: periodStart,
		PeriodEnd:   now,
	}, nil
}

func (c *OpenAPIClient) QueryInstanceStatus(ctx context.Context, account AccountCredentials, instanceID string) (InstanceStatus, error) {
	if err := validateAccountCredentials(account); err != nil {
		return InstanceStatus{}, err
	}
	body, err := c.executor.Execute(ctx, account, rpcRequest{
		Operation: "query ECS instance status",
		Action:    "DescribeInstanceStatus",
		Version:   ecsAPIVersion,
		Endpoint:  ecsEndpoint(account.Region),
		RegionID:  account.Region,
		Query: map[string]string{
			"RegionId":     account.Region,
			"InstanceId.1": instanceID,
		},
	})
	if err != nil {
		return InstanceStatus{}, err
	}

	var response ecsStatusResponse
	if err := decodeBody(body, &response); err != nil {
		return InstanceStatus{}, invalidResponse("query ECS instance status", err)
	}
	for _, status := range response.InstanceStatuses.InstanceStatus {
		if status.InstanceID == instanceID {
			if strings.TrimSpace(status.Status) == "" {
				return InstanceStatus{}, invalidResponse("query ECS instance status", errors.New("instance status is empty"))
			}
			return InstanceStatus{InstanceID: instanceID, Status: status.Status}, nil
		}
	}
	return InstanceStatus{}, APIError{
		Operation: "query ECS instance status",
		Code:      "InstanceNotFound",
		Message:   "the configured instance was not returned by DescribeInstanceStatus",
	}
}

func (c *OpenAPIClient) QueryAccountBalance(context.Context, AccountCredentials) (CostSnapshot, error) {
	return CostSnapshot{}, APIError{
		Operation: "query account balance",
		Code:      "UnsupportedOperation",
		Message:   "BssOpenApi requires an explicit China or international account site setting",
	}
}

func (c *OpenAPIClient) StartInstance(ctx context.Context, account AccountCredentials, instanceID string) error {
	if err := validateAccountCredentials(account); err != nil {
		return err
	}
	if err := c.requireLive("start ECS instance"); err != nil {
		return err
	}
	_, err := c.executor.Execute(ctx, account, rpcRequest{
		Operation: "start ECS instance",
		Action:    "StartInstance",
		Version:   ecsAPIVersion,
		Endpoint:  ecsEndpoint(account.Region),
		RegionID:  account.Region,
		Query: map[string]string{
			"RegionId":   account.Region,
			"InstanceId": instanceID,
		},
	})
	return err
}

func (c *OpenAPIClient) StopInstance(ctx context.Context, account AccountCredentials, instanceID string, mode StopMode) error {
	if err := validateAccountCredentials(account); err != nil {
		return err
	}
	if err := c.requireLive("stop ECS instance"); err != nil {
		return err
	}
	_, err := c.executor.Execute(ctx, account, rpcRequest{
		Operation: "stop ECS instance",
		Action:    "StopInstance",
		Version:   ecsAPIVersion,
		Endpoint:  ecsEndpoint(account.Region),
		RegionID:  account.Region,
		Query: map[string]string{
			"RegionId":    account.Region,
			"InstanceId":  instanceID,
			"StoppedMode": string(mode),
		},
	})
	return err
}

func (c *OpenAPIClient) requireLive(operation string) error {
	if c.mode == ModeLive {
		return nil
	}
	return APIError{
		Operation: operation,
		Code:      "OperationDisabled",
		Message:   "cloud mutations require CDTM_ALIYUN_MODE=live",
	}
}

func (e *sdkRPCExecutor) Execute(ctx context.Context, account AccountCredentials, request rpcRequest) (map[string]interface{}, error) {
	config := (&openapimodels.Config{}).
		SetAccessKeyId(account.AccessKeyID).
		SetAccessKeySecret(account.AccessKeySecret).
		SetEndpoint(request.Endpoint).
		SetRegionId(request.RegionID).
		SetProtocol("HTTPS")
	client, err := openapi.NewClient(config)
	if err != nil {
		return nil, wrapAPIError(request.Operation, err)
	}

	params := (&openapiutils.Params{}).
		SetAction(request.Action).
		SetVersion(request.Version).
		SetProtocol("HTTPS").
		SetPathname("/").
		SetMethod("POST").
		SetAuthType("AK").
		SetStyle("RPC").
		SetReqBodyType("formData").
		SetBodyType("json")
	apiRequest := (&openapiutils.OpenApiRequest{}).SetQuery(pointerMap(request.Query))
	runtime := (&dara.RuntimeOptions{}).
		SetAutoretry(true).
		SetMaxAttempts(3).
		SetConnectTimeout(durationMilliseconds(e.connectTimeout)).
		SetReadTimeout(durationMilliseconds(e.readTimeout))
	result, err := client.CallApiWithCtx(ctx, params, apiRequest, runtime)
	if err != nil {
		return nil, wrapAPIError(request.Operation, err)
	}
	body, ok := result["body"].(map[string]interface{})
	if !ok {
		return nil, invalidResponse(request.Operation, errors.New("response body is not a JSON object"))
	}
	return body, nil
}

type cdtTrafficResponse struct {
	TrafficDetails []struct {
		BusinessRegionID string    `json:"BusinessRegionId"`
		Traffic          byteCount `json:"Traffic"`
	} `json:"TrafficDetails"`
}

type ecsStatusResponse struct {
	InstanceStatuses struct {
		InstanceStatus []struct {
			InstanceID string `json:"InstanceId"`
			Status     string `json:"Status"`
		} `json:"InstanceStatus"`
	} `json:"InstanceStatuses"`
}

type byteCount int64

func (b *byteCount) UnmarshalJSON(data []byte) error {
	value := strings.Trim(strings.TrimSpace(string(data)), "\"")
	if value == "" {
		return errors.New("traffic value is empty")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return fmt.Errorf("invalid traffic byte count %q", value)
	}
	*b = byteCount(parsed)
	return nil
}

func decodeBody(body map[string]interface{}, target interface{}) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func pointerMap(values map[string]string) map[string]*string {
	result := make(map[string]*string, len(values))
	for key, value := range values {
		value := value
		result[key] = &value
	}
	return result
}

func durationMilliseconds(value time.Duration) int {
	milliseconds := value.Milliseconds()
	maxInt := int64(^uint(0) >> 1)
	if milliseconds > maxInt {
		return int(maxInt)
	}
	return int(milliseconds)
}

func ecsEndpoint(region string) string {
	return "ecs." + strings.TrimSpace(region) + ".aliyuncs.com"
}

func isOverseasRegion(region string) bool {
	region = strings.ToLower(strings.TrimSpace(region))
	return !strings.HasPrefix(region, "cn-") || region == "cn-hongkong"
}

func validateAccountCredentials(account AccountCredentials) error {
	if strings.TrimSpace(account.AccessKeyID) == "" || strings.TrimSpace(account.AccessKeySecret) == "" {
		return APIError{
			Operation: "configure Alibaba Cloud client",
			Code:      "InvalidCredentials",
			Message:   "AccessKey ID and AccessKey Secret are required",
		}
	}
	if !IsValidRegionID(account.Region) {
		return APIError{
			Operation: "configure Alibaba Cloud client",
			Code:      "InvalidRegion",
			Message:   "region ID has an invalid format",
		}
	}
	return nil
}

func invalidResponse(operation string, err error) APIError {
	return APIError{
		Operation: operation,
		Code:      "InvalidResponse",
		Message:   err.Error(),
	}
}

func wrapAPIError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var cloudErr interface {
		error
		GetStatusCode() *int
		GetCode() *string
		GetMessage() *string
	}
	if errors.As(err, &cloudErr) {
		code := stringValue(cloudErr.GetCode())
		status := intValue(cloudErr.GetStatusCode())
		return APIError{
			Operation: operation,
			Code:      code,
			Message:   stringValue(cloudErr.GetMessage()),
			Retryable: status == 429 || status >= 500 || strings.Contains(strings.ToLower(code), "throttling"),
		}
	}
	return APIError{
		Operation: operation,
		Code:      "SDKError",
		Message:   err.Error(),
		Retryable: true,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
