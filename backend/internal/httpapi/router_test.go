package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cdt-monitor/backend/internal/aliyun"
	"cdt-monitor/backend/internal/auth"
	"cdt-monitor/backend/internal/logger"
	"cdt-monitor/backend/internal/secrets"
	"cdt-monitor/backend/internal/service"
	"cdt-monitor/backend/internal/store"
)

type noopCloud struct{}

func (noopCloud) QueryTraffic(context.Context, aliyun.AccountCredentials) (aliyun.TrafficUsage, error) {
	return aliyun.TrafficUsage{}, nil
}

func (noopCloud) QueryInstanceStatus(context.Context, aliyun.AccountCredentials, string) (aliyun.InstanceStatus, error) {
	return aliyun.InstanceStatus{Status: "Running"}, nil
}

func (noopCloud) QueryAccountBalance(context.Context, aliyun.AccountCredentials) (aliyun.CostSnapshot, error) {
	return aliyun.CostSnapshot{}, nil
}

func (noopCloud) StartInstance(context.Context, aliyun.AccountCredentials, string) error { return nil }

func (noopCloud) StopInstance(context.Context, aliyun.AccountCredentials, string, aliyun.StopMode) error {
	return nil
}

func TestHealthAndValidationError(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("health status = %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(`{"accessKeyId":""}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", res.Code)
	}

	cookie := login(t, router, testAdminPassword)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(`{"accessKeyId":""}`))
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d", res.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != "validation_failed" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

func TestAuthSessionFlow(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", res.Code)
	}

	badLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"wrong"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, badLogin)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", res.Code)
	}

	cookie := login(t, router, testAdminPassword)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("logout status = %d", res.Code)
	}
}

func TestWebhookDoesNotRequireSessionCookie(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/aliyun/events/not-real", strings.NewReader(`{}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("webhook should reach token validation, status = %d", res.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	t.Parallel()

	router := newTestRouterWithLimiter(t, auth.NewLoginLimiter(auth.LoginLimiterConfig{
		MaxFailures: 2,
		Window:      time.Minute,
		Lockout:     time.Minute,
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"wrong"}`))
		req.RemoteAddr = "198.51.100.10:40000"
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if i == 0 && res.Code != http.StatusUnauthorized {
			t.Fatalf("first bad login status = %d", res.Code)
		}
		if i == 1 && res.Code != http.StatusTooManyRequests {
			t.Fatalf("lockout status = %d", res.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"`+testAdminPassword+`"}`))
	req.RemoteAddr = "198.51.100.10:40000"
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("locked valid login status = %d", res.Code)
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatalf("locked response missing Retry-After")
	}
}

func TestLoginRateLimitUsesForwardedForWhenTrusted(t *testing.T) {
	t.Parallel()

	router := newTestRouterWithLimiter(t, auth.NewLoginLimiter(auth.LoginLimiterConfig{
		MaxFailures: 2,
		Window:      time.Minute,
		Lockout:     time.Minute,
		TrustProxy:  true,
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"wrong"}`))
		req.RemoteAddr = "10.0.0.2:40000"
		req.Header.Set("X-Forwarded-For", "198.51.100.20")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"`+testAdminPassword+`"}`))
	req.RemoteAddr = "10.0.0.2:40000"
	req.Header.Set("X-Forwarded-For", "198.51.100.21")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("different forwarded ip login status = %d", res.Code)
	}
}

const testAdminPassword = "test-password"

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	return newTestRouterWithLimiter(t, auth.NewLoginLimiter(auth.LoginLimiterConfig{}))
}

func newTestRouterWithLimiter(t *testing.T, limiter *auth.LoginLimiter) http.Handler {
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
	codec, err := secrets.NewCodecFromKey([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	svc := service.New(store.New(db), noopCloud{}, codec, logger.New(nil, logger.LevelError))
	manager, err := auth.NewManager(auth.Config{
		AdminPassword: testAdminPassword,
		SessionKey:    []byte("abcdefghijklmnopqrstuvwx12345678"),
		SessionTTL:    12 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new auth manager: %v", err)
	}
	return NewRouter(svc, manager, limiter, logger.New(nil, logger.LevelError))
}

func login(t *testing.T, router http.Handler, password string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"`+password+`"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("login status = %d", res.Code)
	}
	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == auth.CookieName {
			return cookie
		}
	}
	t.Fatalf("login response did not set %s cookie", auth.CookieName)
	return nil
}
