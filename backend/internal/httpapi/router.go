package httpapi

import (
	"net/http"
	"strings"
	"time"

	"cdt-monitor/backend/internal/auth"
	"cdt-monitor/backend/internal/logger"
	"cdt-monitor/backend/internal/service"
)

type API struct {
	service      *service.Service
	auth         *auth.Manager
	loginLimiter *auth.LoginLimiter
	logger       *logger.Logger
	mux          *http.ServeMux
}

func NewRouter(service *service.Service, authManager *auth.Manager, loginLimiter *auth.LoginLimiter, logger *logger.Logger) http.Handler {
	api := &API{
		service:      service,
		auth:         authManager,
		loginLimiter: loginLimiter,
		logger:       logger,
		mux:          http.NewServeMux(),
	}
	api.routes()
	return api.withLogging(api.withAuth(api.mux))
}

func (a *API) routes() {
	a.mux.HandleFunc("/healthz", a.health)
	a.mux.HandleFunc("/api/auth/login", a.login)
	a.mux.HandleFunc("/api/auth/logout", a.logout)
	a.mux.HandleFunc("/api/auth/session", a.session)
	a.mux.HandleFunc("/api/v1/status", a.status)
	a.mux.HandleFunc("/api/v1/sync", a.syncNow)
	a.mux.HandleFunc("/api/v1/accounts", a.accounts)
	a.mux.HandleFunc("/api/v1/accounts/", a.accountByID)
	a.mux.HandleFunc("/api/v1/instances", a.instances)
	a.mux.HandleFunc("/api/v1/instances/", a.instanceRoute)
	a.mux.HandleFunc("/api/v1/action-logs", a.actionLogs)
	a.mux.HandleFunc("/api/v1/cloud-events", a.cloudEvents)
	a.mux.HandleFunc("/api/v1/settings", a.settings)
	a.mux.HandleFunc("/api/v1/webhook-token/reset", a.resetWebhookToken)
	a.mux.HandleFunc("/api/v1/scheduled-tasks", a.scheduledTasks)
	a.mux.HandleFunc("/api/v1/traffic-trend", a.trafficTrend)
	a.mux.HandleFunc("/api/v1/instance-status-history", a.instanceStatusHistory)
	a.mux.HandleFunc("/api/v1/log-cleanup", a.logCleanup)
	a.mux.HandleFunc("/api/v1/costs", a.costs)
	a.mux.HandleFunc("/api/v1/costs/sync", a.syncCosts)
	a.mux.HandleFunc("/api/v1/cloudflare-credentials", a.cloudflareCredentials)
	a.mux.HandleFunc("/api/v1/dns-records", a.dnsRecords)
	a.mux.HandleFunc("/api/v1/ddns/update", a.updateDDNS)
	a.mux.HandleFunc("/api/v1/risky-operations/evaluate", a.evaluateRiskyOperation)
	a.mux.HandleFunc("/api/v1/telegram/config", a.telegramConfig)
	a.mux.HandleFunc("/api/v1/telegram/command", a.telegramCommand)
	a.mux.HandleFunc("/api/v1/notification-channels", a.notificationChannels)
	a.mux.HandleFunc("/api/v1/notification-channels/", a.notificationChannelAction)
	a.mux.HandleFunc("/api/v1/notification-logs", a.notificationLogs)
	a.mux.HandleFunc("/api/webhooks/aliyun/events/", a.webhook)
}

func (a *API) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := a.auth.FromRequest(r); !ok {
			writeAuthError(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func trimID(path, prefix string) string {
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}
