package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cdt-monitor/backend/internal/aliyun"
	"cdt-monitor/backend/internal/auth"
	"cdt-monitor/backend/internal/config"
	httpapi "cdt-monitor/backend/internal/httpapi"
	"cdt-monitor/backend/internal/logger"
	"cdt-monitor/backend/internal/scheduler"
	"cdt-monitor/backend/internal/secrets"
	"cdt-monitor/backend/internal/service"
	"cdt-monitor/backend/internal/store"
)

func main() {
	cfg := config.Load()
	log := logger.New(os.Stdout, cfg.LogLevel)

	db, err := store.Open(cfg.DatabasePath)
	if err != nil {
		log.Error("open database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(context.Background(), db); err != nil {
		log.Error("run migrations", "error", err.Error())
		os.Exit(1)
	}

	repo := store.New(db)
	cloud := aliyun.NewDryRunClient(log)
	codec, err := secrets.LoadCodec(cfg.SecretKeyPath)
	if err != nil {
		log.Error("load secret codec", "error", err.Error())
		os.Exit(1)
	}
	sessionKey, err := auth.LoadOrCreateKey(cfg.SessionKeyPath)
	if err != nil {
		log.Error("load session key", "error", err.Error())
		os.Exit(1)
	}
	sessionManager, err := auth.NewManager(auth.Config{
		AdminPassword: cfg.AdminPassword,
		SessionKey:    sessionKey,
		SessionTTL:    cfg.SessionTTL,
		SecureCookie:  cfg.SecureSession,
	})
	if err != nil {
		log.Error("configure authentication", "error", err.Error(), "env", "CDTM_ADMIN_PASSWORD")
		os.Exit(1)
	}
	loginLimiter := auth.NewLoginLimiter(auth.LoginLimiterConfig{
		MaxFailures: cfg.LoginMaxFailures,
		Window:      cfg.LoginWindow,
		Lockout:     cfg.LoginLockout,
		TrustProxy:  cfg.TrustProxyHeaders,
	})
	app := service.New(repo, cloud, codec, log)

	worker := scheduler.New(app, cfg.SyncInterval, cfg.KeepaliveInterval, log)
	worker.Start()
	defer worker.Stop()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(app, sessionManager, loginLimiter, log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", server.Addr, "database", cfg.DatabasePath)
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "error", err.Error())
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("shutdown failed", "error", err.Error())
		os.Exit(1)
	}
}
