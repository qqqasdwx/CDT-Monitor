package service

import (
	"context"
	"strings"
	"time"

	"cdt-monitor/backend/internal/store"

	"github.com/google/uuid"
)

type CloudflareCredentialInput struct {
	Name     string `json:"name"`
	APIToken string `json:"apiToken"`
	Enabled  *bool  `json:"enabled"`
}

type DNSRecordInput struct {
	CredentialID string `json:"credentialId"`
	ZoneID       string `json:"zoneId"`
	RecordID     string `json:"recordId"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	CurrentValue string `json:"currentValue"`
	Enabled      *bool  `json:"enabled"`
}

func (s *Service) CreateCloudflareCredential(ctx context.Context, input CloudflareCredentialInput) (store.CloudflareCredential, error) {
	if strings.TrimSpace(input.Name) == "" {
		return store.CloudflareCredential{}, validation("Cloudflare 凭据名称不能为空")
	}
	if strings.TrimSpace(input.APIToken) == "" {
		return store.CloudflareCredential{}, validation("Cloudflare API Token 不能为空")
	}
	encrypted, err := s.codec.Encrypt(input.APIToken)
	if err != nil {
		return store.CloudflareCredential{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	credential, err := s.repo.CreateCloudflareCredential(ctx, store.CloudflareCredential{
		ID:                uuid.NewString(),
		Name:              strings.TrimSpace(input.Name),
		APITokenEncrypted: encrypted,
		Enabled:           enabled,
	})
	if err != nil {
		return store.CloudflareCredential{}, err
	}
	credential.APITokenEncrypted = ""
	return credential, nil
}

func (s *Service) ListCloudflareCredentials(ctx context.Context) ([]store.CloudflareCredential, error) {
	credentials, err := s.repo.ListCloudflareCredentials(ctx)
	if err != nil {
		return nil, err
	}
	for index := range credentials {
		credentials[index].APITokenEncrypted = ""
	}
	return credentials, nil
}

func (s *Service) CreateDNSRecord(ctx context.Context, input DNSRecordInput) (store.DNSRecord, error) {
	if strings.TrimSpace(input.CredentialID) == "" {
		return store.DNSRecord{}, validation("Cloudflare 凭据 ID 不能为空")
	}
	if strings.TrimSpace(input.ZoneID) == "" || strings.TrimSpace(input.RecordID) == "" {
		return store.DNSRecord{}, validation("Zone ID 和 Record ID 不能为空")
	}
	if strings.TrimSpace(input.Name) == "" {
		return store.DNSRecord{}, validation("域名记录名称不能为空")
	}
	if _, err := s.repo.GetCloudflareCredential(ctx, input.CredentialID); err != nil {
		return store.DNSRecord{}, mapStoreError(err)
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	recordType := strings.TrimSpace(input.Type)
	if recordType == "" {
		recordType = "A"
	}
	return s.repo.CreateDNSRecord(ctx, store.DNSRecord{
		ID:           uuid.NewString(),
		CredentialID: strings.TrimSpace(input.CredentialID),
		ZoneID:       strings.TrimSpace(input.ZoneID),
		RecordID:     strings.TrimSpace(input.RecordID),
		Name:         strings.TrimSpace(input.Name),
		Type:         recordType,
		CurrentValue: strings.TrimSpace(input.CurrentValue),
		Enabled:      enabled,
	})
}

func (s *Service) ListDNSRecords(ctx context.Context) ([]store.DNSRecord, error) {
	return s.repo.ListDNSRecords(ctx)
}

func (s *Service) UpdateDDNS(ctx context.Context, publicIP string) ([]store.ActionLog, error) {
	records, err := s.repo.ListDNSRecords(ctx)
	if err != nil {
		return nil, err
	}
	var logs []store.ActionLog
	for _, record := range records {
		if !record.Enabled || strings.TrimSpace(publicIP) == "" || record.CurrentValue == publicIP {
			continue
		}
		err := s.repo.UpdateDNSRecordValue(ctx, record.ID, publicIP)
		result := "success"
		errorMessage := ""
		if err != nil {
			result = "failed"
			errorMessage = err.Error()
		}
		log, logErr := s.repo.CreateActionLog(ctx, store.ActionLog{
			ID:            uuid.NewString(),
			ActionType:    "ddns_update",
			TriggerSource: "manual",
			Result:        result,
			Reason:        "公网 IP 变化后更新 DNS 记录",
			ErrorMessage:  errorMessage,
			CreatedAt:     time.Now().UTC(),
		})
		if logErr != nil {
			return logs, logErr
		}
		logs = append(logs, log)
		if err != nil {
			return logs, err
		}
	}
	return logs, nil
}
