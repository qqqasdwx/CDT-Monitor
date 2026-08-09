package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"cdt-monitor/backend/internal/service"
)

type envelope struct {
	Data any `json:"data,omitempty"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: payload})
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "服务器内部错误"

	var validationErr service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		status = http.StatusBadRequest
		code = "validation_failed"
		message = validationErr.Message
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "资源不存在"
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
		code = "conflict"
		message = "资源状态冲突"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: apiError{Code: code, Message: message},
	})
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: apiError{Code: "unauthorized", Message: "请先登录"},
	})
}

func writeRateLimitError(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: apiError{Code: "login_locked", Message: "登录失败次数过多，请稍后再试"},
	})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return service.ValidationError{Message: "请求 JSON 不合法"}
	}
	return nil
}
