package service

import (
	"context"
	"fmt"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

func (s *Service) SyncCosts(ctx context.Context) ([]store.CostSnapshot, error) {
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var snapshots []store.CostSnapshot
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		creds, err := s.credentials(account)
		if err != nil {
			return snapshots, err
		}
		balance, err := s.cloud.QueryAccountBalance(ctx, creds)
		if err != nil {
			_, _ = s.repo.CreateActionLog(ctx, store.ActionLog{
				ID:            uuid.NewString(),
				AccountID:     account.ID,
				ActionType:    "sync_cost",
				TriggerSource: "manual",
				Result:        "failed",
				Reason:        "同步费用余额失败",
				ErrorMessage:  err.Error(),
				CreatedAt:     time.Now().UTC(),
			})
			s.logger.Error("sync cost failed", "account_id", account.ID, "error", err.Error())
			continue
		}
		snapshot, err := s.repo.CreateCostSnapshot(ctx, store.CostSnapshot{
			ID:              uuid.NewString(),
			AccountID:       account.ID,
			AvailableAmount: balance.AvailableAmount,
			CreditAmount:    balance.CreditAmount,
			Currency:        balance.Currency,
			CollectedAt:     balance.CollectedAt,
		})
		if err != nil {
			return snapshots, fmt.Errorf("save cost snapshot: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func (s *Service) ListCostSnapshots(ctx context.Context) ([]store.CostSnapshot, error) {
	return s.repo.ListLatestCostSnapshots(ctx)
}
