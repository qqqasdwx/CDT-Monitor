package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cdt-monitor/backend/internal/logger"
)

type Config struct {
	Port              string
	DatabasePath      string
	FrontendDir       string
	SecretKeyPath     string
	SessionKeyPath    string
	AdminPassword     string
	SessionTTL        time.Duration
	SecureSession     bool
	LoginMaxFailures  int
	LoginWindow       time.Duration
	LoginLockout      time.Duration
	TrustProxyHeaders bool
	AliyunMode        string
	AliyunConnect     time.Duration
	AliyunRead        time.Duration
	SyncInterval      time.Duration
	KeepaliveInterval time.Duration
	LogLevel          logger.Level
}

func Load() Config {
	return Config{
		Port:              getEnv("CDTM_PORT", "8080"),
		DatabasePath:      getEnv("CDTM_DATABASE_PATH", filepath.Join("data", "cdt-monitor.sqlite")),
		FrontendDir:       strings.TrimSpace(os.Getenv("CDTM_FRONTEND_DIR")),
		SecretKeyPath:     getEnv("CDTM_SECRET_KEY_PATH", filepath.Join("data", "secret.key")),
		SessionKeyPath:    getEnv("CDTM_SESSION_KEY_PATH", filepath.Join("data", "session.key")),
		AdminPassword:     strings.TrimSpace(os.Getenv("CDTM_ADMIN_PASSWORD")),
		SessionTTL:        getDuration("CDTM_SESSION_TTL", 12*time.Hour),
		SecureSession:     getBool("CDTM_SECURE_SESSION_COOKIE", false),
		LoginMaxFailures:  getInt("CDTM_LOGIN_MAX_FAILURES", 5),
		LoginWindow:       getDuration("CDTM_LOGIN_WINDOW", 15*time.Minute),
		LoginLockout:      getDuration("CDTM_LOGIN_LOCKOUT", 15*time.Minute),
		TrustProxyHeaders: getBool("CDTM_TRUST_PROXY_HEADERS", false),
		AliyunMode:        getEnv("CDTM_ALIYUN_MODE", "dry-run"),
		AliyunConnect:     getDuration("CDTM_ALIYUN_CONNECT_TIMEOUT", 5*time.Second),
		AliyunRead:        getDuration("CDTM_ALIYUN_READ_TIMEOUT", 10*time.Second),
		SyncInterval:      getDuration("CDTM_SYNC_INTERVAL", 5*time.Minute),
		KeepaliveInterval: getDuration("CDTM_KEEPALIVE_INTERVAL", 10*time.Minute),
		LogLevel:          logger.ParseLevel(getEnv("CDTM_LOG_LEVEL", "info")),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
