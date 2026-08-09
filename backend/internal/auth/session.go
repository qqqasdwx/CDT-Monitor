package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	CookieName     = "cdtm_session"
	keySize        = 32
	tokenVersion   = "v1"
	sessionKeyMode = 0o600
)

type Config struct {
	AdminPassword string
	SessionKey    []byte
	SessionTTL    time.Duration
	SecureCookie  bool
}

type Manager struct {
	adminPassword string
	sessionKey    []byte
	sessionTTL    time.Duration
	secureCookie  bool
}

type Session struct {
	Authenticated bool      `json:"authenticated"`
	ExpiresAt     time.Time `json:"expiresAt,omitempty"`
}

func NewManager(cfg Config) (*Manager, error) {
	if strings.TrimSpace(cfg.AdminPassword) == "" {
		return nil, fmt.Errorf("admin password is required")
	}
	if len(cfg.SessionKey) != keySize {
		return nil, fmt.Errorf("session key must be %d bytes", keySize)
	}
	if cfg.SessionTTL <= 0 {
		return nil, fmt.Errorf("session ttl must be positive")
	}
	return &Manager{
		adminPassword: cfg.AdminPassword,
		sessionKey:    append([]byte(nil), cfg.SessionKey...),
		sessionTTL:    cfg.SessionTTL,
		secureCookie:  cfg.SecureCookie,
	}, nil
}

func LoadOrCreateKey(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("session key path is empty")
	}
	if data, err := os.ReadFile(path); err == nil {
		key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("decode session key file: %w", err)
		}
		if len(key) != keySize {
			return nil, fmt.Errorf("session key file must contain %d bytes", keySize)
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read session key file: %w", err)
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create session key directory: %w", err)
		}
	}
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(encoded+"\n"), sessionKeyMode); err != nil {
		return nil, fmt.Errorf("write session key file: %w", err)
	}
	return key, nil
}

func (m *Manager) PasswordMatches(password string) bool {
	expected := sha256.Sum256([]byte(m.adminPassword))
	actual := sha256.Sum256([]byte(password))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}

func (m *Manager) Issue(w http.ResponseWriter) (Session, error) {
	expiresAt := time.Now().UTC().Add(m.sessionTTL)
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Session{}, fmt.Errorf("generate session nonce: %w", err)
	}
	nonceText := base64.RawURLEncoding.EncodeToString(nonce)
	payload := strings.Join([]string{tokenVersion, strconv.FormatInt(expiresAt.Unix(), 10), nonceText}, ".")
	token := payload + "." + m.signature(payload)

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.secureCookie,
		MaxAge:   int(m.sessionTTL.Seconds()),
		Expires:  expiresAt,
	})
	return Session{Authenticated: true, ExpiresAt: expiresAt}, nil
}

func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.secureCookie,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	})
}

func (m *Manager) FromRequest(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return Session{}, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 4 || parts[0] != tokenVersion {
		return Session{}, false
	}
	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Session{}, false
	}
	expiresAt := time.Unix(expiresUnix, 0).UTC()
	if !time.Now().UTC().Before(expiresAt) {
		return Session{}, false
	}
	payload := strings.Join(parts[:3], ".")
	if subtle.ConstantTimeCompare([]byte(parts[3]), []byte(m.signature(payload))) != 1 {
		return Session{}, false
	}
	return Session{Authenticated: true, ExpiresAt: expiresAt}, true
}

func (m *Manager) signature(payload string) string {
	mac := hmac.New(sha256.New, m.sessionKey)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
