package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LoginLimiter struct {
	mu             sync.Mutex
	maxFailures    int
	window         time.Duration
	lockout        time.Duration
	trustProxy     bool
	now            func() time.Time
	failedAttempts map[string][]time.Time
	lockedUntil    map[string]time.Time
}

type LoginLimiterConfig struct {
	MaxFailures int
	Window      time.Duration
	Lockout     time.Duration
	TrustProxy  bool
}

func NewLoginLimiter(cfg LoginLimiterConfig) *LoginLimiter {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.Window <= 0 {
		cfg.Window = 15 * time.Minute
	}
	if cfg.Lockout <= 0 {
		cfg.Lockout = 15 * time.Minute
	}
	return &LoginLimiter{
		maxFailures:    cfg.MaxFailures,
		window:         cfg.Window,
		lockout:        cfg.Lockout,
		trustProxy:     cfg.TrustProxy,
		now:            time.Now,
		failedAttempts: make(map[string][]time.Time),
		lockedUntil:    make(map[string]time.Time),
	}
}

func (l *LoginLimiter) Locked(r *http.Request) (time.Duration, bool) {
	if l == nil {
		return 0, false
	}
	key := l.key(r)
	now := l.now().UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	until, ok := l.lockedUntil[key]
	if !ok {
		return 0, false
	}
	if now.Before(until) {
		return until.Sub(now), true
	}
	delete(l.lockedUntil, key)
	delete(l.failedAttempts, key)
	return 0, false
}

func (l *LoginLimiter) RecordFailure(r *http.Request) (time.Duration, bool) {
	if l == nil {
		return 0, false
	}
	key := l.key(r)
	now := l.now().UTC()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	attempts := l.failedAttempts[key]
	kept := attempts[:0]
	for _, attempt := range attempts {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}
	kept = append(kept, now)
	l.failedAttempts[key] = kept

	if len(kept) >= l.maxFailures {
		until := now.Add(l.lockout)
		l.lockedUntil[key] = until
		return l.lockout, true
	}
	return 0, false
}

func (l *LoginLimiter) RecordSuccess(r *http.Request) {
	if l == nil {
		return
	}
	key := l.key(r)
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failedAttempts, key)
	delete(l.lockedUntil, key)
}

func (l *LoginLimiter) key(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if l.trustProxy {
		if ip := firstForwardedIP(r.Header.Get("X-Forwarded-For")); ip != "" {
			return ip
		}
		if ip := parseIP(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := parseIP(host); ip != "" {
			return ip
		}
	}
	if ip := parseIP(r.RemoteAddr); ip != "" {
		return ip
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func firstForwardedIP(value string) string {
	for _, part := range strings.Split(value, ",") {
		if ip := parseIP(part); ip != "" {
			return ip
		}
	}
	return ""
}

func parseIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, ":") {
		if host, _, err := net.SplitHostPort(value); err == nil {
			value = host
		}
	}
	ip := net.ParseIP(strings.Trim(value, "[]"))
	if ip == nil {
		return ""
	}
	return ip.String()
}
