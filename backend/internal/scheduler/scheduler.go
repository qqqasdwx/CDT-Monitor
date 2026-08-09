package scheduler

import (
	"context"
	"sync"
	"time"

	"cdt-monitor/backend/internal/logger"
	"cdt-monitor/backend/internal/service"
)

type Scheduler struct {
	service           *service.Service
	syncInterval      time.Duration
	keepaliveInterval time.Duration
	logger            *logger.Logger
	cancel            context.CancelFunc
	wg                sync.WaitGroup
}

func New(service *service.Service, syncInterval, keepaliveInterval time.Duration, logger *logger.Logger) *Scheduler {
	return &Scheduler{
		service:           service,
		syncInterval:      syncInterval,
		keepaliveInterval: keepaliveInterval,
		logger:            logger,
	}
}

func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(2)
	go s.runSync(ctx)
	go s.runKeepalive(ctx)
	s.wg.Add(1)
	go s.runSchedules(ctx)
}

func (s *Scheduler) runSchedules(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.service.RunDueScheduledTasks(ctx); err != nil {
				s.logger.Error("scheduled task execution failed", "error", err.Error())
			}
			if _, err := s.service.RunMonthlyRestore(ctx); err != nil {
				s.logger.Error("monthly restore failed", "error", err.Error())
			}
		}
	}
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) runSync(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.service.SyncNow(ctx); err != nil {
				s.logger.Error("scheduled sync failed", "error", err.Error())
			}
		}
	}
}

func (s *Scheduler) runKeepalive(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.keepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.service.ReconcileKeepalive(ctx); err != nil {
				s.logger.Error("keepalive reconcile failed", "error", err.Error())
			}
		}
	}
}
