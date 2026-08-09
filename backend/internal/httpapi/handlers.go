package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cdt-monitor/backend/internal/service"
)

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if retryAfter, locked := a.loginLimiter.Locked(r); locked {
		writeRateLimitError(w, retryAfter)
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	if !a.auth.PasswordMatches(input.Password) {
		if retryAfter, locked := a.loginLimiter.RecordFailure(r); locked {
			writeRateLimitError(w, retryAfter)
			return
		}
		writeAuthError(w)
		return
	}
	a.loginLimiter.RecordSuccess(r)
	session, err := a.auth.Issue(w)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.auth.Clear(w)
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func (a *API) session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := a.auth.FromRequest(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status, err := a.service.Status(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *API) syncNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.service.SyncNow(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) accounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		accounts, err := a.service.ListAccounts(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, accounts)
	case http.MethodPost:
		var input service.AccountInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		account, err := a.service.CreateAccount(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, account)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) accountByID(w http.ResponseWriter, r *http.Request) {
	id := trimID(r.URL.Path, "/api/v1/accounts/")
	switch r.Method {
	case http.MethodPut:
		var input service.AccountInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		account, err := a.service.UpdateAccount(r.Context(), id, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	case http.MethodDelete:
		if err := a.service.DeleteAccount(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) instances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		instances, err := a.service.ListInstances(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, instances)
	case http.MethodPost:
		var input service.InstanceInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		instance, err := a.service.CreateInstance(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, instance)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) instanceRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/instances/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && r.Method == http.MethodPost {
		a.instanceAction(w, r)
		return
	}
	if len(parts) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	a.instanceByID(w, r)
}

func (a *API) instanceByID(w http.ResponseWriter, r *http.Request) {
	id := trimID(r.URL.Path, "/api/v1/instances/")
	if strings.Contains(id, "/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input service.InstanceInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		instance, err := a.service.UpdateInstance(r.Context(), id, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, instance)
	case http.MethodDelete:
		if err := a.service.DeleteInstance(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) instanceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/instances/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var (
		log any
		err error
	)
	switch parts[1] {
	case "start":
		log, err = a.service.StartInstance(r.Context(), parts[0])
	case "stop":
		log, err = a.service.StopInstance(r.Context(), parts[0])
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (a *API) actionLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logs, err := a.service.QueryActionLogs(r.Context(), parseLimit(r), parseSince(r), r.URL.Query().Get("type"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func parseSince(r *http.Request) *time.Time {
	value := strings.TrimSpace(r.URL.Query().Get("since"))
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func (a *API) cloudEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit := parseLimit(r)
	events, err := a.service.ListCloudEvents(r.Context(), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (a *API) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := a.service.Settings(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	case http.MethodPut:
		var input service.SettingsInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		settings, err := a.service.UpdateSettings(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) resetWebhookToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	url, err := a.service.ResetWebhookToken(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"webhookUrl": url})
}

func (a *API) scheduledTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks, err := a.service.ListScheduledTasks(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tasks)
	case http.MethodPost:
		var input service.ScheduledTaskInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		task, err := a.service.CreateScheduledTask(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, task)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) notificationChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channels, err := a.service.ListNotificationChannels(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, channels)
	case http.MethodPost:
		var input service.NotificationChannelInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		channel, err := a.service.CreateNotificationChannel(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, channel)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) trafficTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	points, err := a.service.TrafficTrend(r.Context(), r.URL.Query().Get("bucket"), parseLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (a *API) instanceStatusHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	points, err := a.service.InstanceStatusHistory(r.Context(), parseLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (a *API) logCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log, err := a.service.CleanupLogs(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (a *API) costs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	snapshots, err := a.service.ListCostSnapshots(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshots)
}

func (a *API) syncCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	snapshots, err := a.service.SyncCosts(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshots)
}

func (a *API) cloudflareCredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		credentials, err := a.service.ListCloudflareCredentials(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, credentials)
	case http.MethodPost:
		var input service.CloudflareCredentialInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		credential, err := a.service.CreateCloudflareCredential(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, credential)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) dnsRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := a.service.ListDNSRecords(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, records)
	case http.MethodPost:
		var input service.DNSRecordInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		record, err := a.service.CreateDNSRecord(r.Context(), input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, record)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) updateDDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		PublicIP string `json:"publicIp"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	logs, err := a.service.UpdateDDNS(r.Context(), input.PublicIP)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (a *API) evaluateRiskyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input service.RiskyOperationInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	log, err := a.service.EvaluateRiskyOperation(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (a *API) telegramConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input service.TelegramConfigInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	log, err := a.service.ConfigureTelegram(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (a *API) telegramCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input service.TelegramCommandInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.HandleTelegramCommand(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) notificationChannelAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/notification-channels/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "test" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	log, err := a.service.TestNotification(r.Context(), parts[0])
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (a *API) notificationLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logs, err := a.service.ListNotificationLogs(r.Context(), parseLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (a *API) webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token := trimID(r.URL.Path, "/api/webhooks/aliyun/events/")
	if token == "" {
		writeError(w, service.ValidationError{Message: "webhook token 不能为空"})
		return
	}

	var raw json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		writeError(w, err)
		return
	}
	input := parseWebhookPayload(raw)
	result, err := a.service.HandleWebhook(r.Context(), token, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func parseLimit(r *http.Request) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		return 50
	}
	return limit
}

func parseWebhookPayload(raw json.RawMessage) service.WebhookInput {
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	getString := func(keys ...string) string {
		for _, key := range keys {
			if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
		return ""
	}
	eventTime := time.Now().UTC()
	if value := getString("eventTime", "time", "timestamp"); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			eventTime = parsed.UTC()
		}
	}
	return service.WebhookInput{
		EventID:    getString("eventId", "id", "event_id"),
		InstanceID: getString("instanceId", "instance_id", "resourceId"),
		EventTime:  eventTime,
		Status:     getString("status", "state", "instanceStatus"),
		Raw:        raw,
	}
}
