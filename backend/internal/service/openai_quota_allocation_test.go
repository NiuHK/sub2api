package service

import (
	"context"
	"testing"
	"time"
)

type quotaAllocationAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r *quotaAllocationAccountRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	return r.accounts, nil
}

type quotaAllocationUsageRepoStub struct {
	UsageLogRepository
	userCost  float64
	totalCost float64
}

func (r *quotaAllocationUsageRepoStub) GetGroupUserActualCostWindow(context.Context, int64, int64, time.Time) (float64, float64, error) {
	return r.userCost, r.totalCost, nil
}

type quotaAllocationRateRepoStub struct {
	UserGroupRateRepository
	allocation *OpenAIQuotaAllocationConfig
}

func (r *quotaAllocationRateRepoStub) GetQuotaAllocationByUserAndGroup(context.Context, int64, int64) (*OpenAIQuotaAllocationConfig, error) {
	return r.allocation, nil
}

type quotaAllocationQuotaStub struct {
	usage *OpenAIQuotaUsage
}

func (r *quotaAllocationQuotaStub) QueryUsage(context.Context, int64) (*OpenAIQuotaUsage, error) {
	return r.usage, nil
}

func quotaAllocationAccount(requireOAuthOnly bool) Account {
	return Account{
		ID:       7,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Groups:   []*Group{{ID: 22, RequireOAuthOnly: requireOAuthOnly}},
	}
}

func quotaAllocationService(allocation *OpenAIQuotaAllocationConfig, account Account, usage *OpenAIRateLimit) *OpenAIQuotaAllocationService {
	return NewOpenAIQuotaAllocationService(
		&quotaAllocationAccountRepoStub{accounts: []Account{account}},
		&quotaAllocationUsageRepoStub{userCost: 6, totalCost: 10},
		&quotaAllocationRateRepoStub{allocation: allocation},
		&quotaAllocationQuotaStub{usage: &OpenAIQuotaUsage{RateLimit: usage}},
	)
}

func TestOpenAIQuotaAllocationCheckUsesActualCostShare(t *testing.T) {
	limit := 40.0
	service := quotaAllocationService(
		&OpenAIQuotaAllocationConfig{Percentage: &limit, Enable5h: true},
		quotaAllocationAccount(true),
		&OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{UsedPercent: 50, LimitWindowSeconds: int64((5 * time.Hour).Seconds()), ResetAt: time.Now().Add(time.Hour).Unix()}},
	)

	if err := service.Check(context.Background(), 11, 22, &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}); err != nil {
		t.Fatalf("expected allocation to pass, got %v", err)
	}
}

func TestOpenAIQuotaAllocationCheckRejectsExceededAndUpstreamLimit(t *testing.T) {
	limit := 20.0
	quota := &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{UsedPercent: 50, LimitWindowSeconds: 60}}
	service := quotaAllocationService(&OpenAIQuotaAllocationConfig{Percentage: &limit, Enable5h: true}, quotaAllocationAccount(true), quota)

	selected := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	if err := service.Check(context.Background(), 11, 22, selected); !IsOpenAIQuotaAllocationExceeded(err) {
		t.Fatalf("expected exceeded allocation error, got %v", err)
	}

	quota.LimitReached = true
	if err := service.Check(context.Background(), 11, 22, selected); !IsOpenAIQuotaAllocationExceeded(err) {
		t.Fatalf("expected upstream limit error, got %v", err)
	}
}

func TestOpenAIQuotaAllocationCheckRejectsUpstreamLimitWithoutUsableWindow(t *testing.T) {
	limit := 20.0
	selected := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	for name, usage := range map[string]*OpenAIRateLimit{
		"nil windows": {LimitReached: true},
		"invalid primary window": {
			LimitReached:   true,
			PrimaryWindow:   &OpenAIRateLimitWindow{LimitWindowSeconds: 0},
			SecondaryWindow: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc := quotaAllocationService(&OpenAIQuotaAllocationConfig{Percentage: &limit, Enable5h: true}, quotaAllocationAccount(true), usage)
			if err := svc.Check(context.Background(), 11, 22, selected); !IsOpenAIQuotaAllocationExceeded(err) {
				t.Fatalf("expected upstream limit error despite unusable window, got %v", err)
			}
		})
	}
}

func TestOpenAIQuotaAllocationCheckAppliesOnlyEnabledWindows(t *testing.T) {
	limit := 20.0
	usage := &OpenAIRateLimit{
		PrimaryWindow:   &OpenAIRateLimitWindow{UsedPercent: 50, LimitWindowSeconds: 60},
		SecondaryWindow: &OpenAIRateLimitWindow{UsedPercent: 50, LimitWindowSeconds: 120},
	}
	selected := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	for name, allocation := range map[string]OpenAIQuotaAllocationConfig{
		"5h only": {Percentage: &limit, Enable5h: true},
		"7d only": {Percentage: &limit, Enable7d: true},
		"both":    {Percentage: &limit, Enable5h: true, Enable7d: true},
	} {
		t.Run(name, func(t *testing.T) {
			service := quotaAllocationService(&allocation, quotaAllocationAccount(true), usage)
			if err := service.Check(context.Background(), 11, 22, selected); !IsOpenAIQuotaAllocationExceeded(err) {
				t.Fatalf("expected enabled window to reject, got %v", err)
			}
		})
	}
}

func TestOpenAIQuotaAllocationCheckIgnoresDisabledWindowsAndUnsupportedGroups(t *testing.T) {
	limit := 20.0
	usage := &OpenAIRateLimit{
		LimitReached:    true,
		PrimaryWindow:   &OpenAIRateLimitWindow{UsedPercent: 100, LimitWindowSeconds: 60},
		SecondaryWindow: &OpenAIRateLimitWindow{UsedPercent: 100, LimitWindowSeconds: 120},
	}
	selected := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	allocation := &OpenAIQuotaAllocationConfig{Percentage: &limit}
	service := quotaAllocationService(allocation, quotaAllocationAccount(true), usage)
	if err := service.Check(context.Background(), 11, 22, selected); err != nil {
		t.Fatalf("expected both disabled to skip allocation, got %v", err)
	}

	service = quotaAllocationService(&OpenAIQuotaAllocationConfig{Percentage: &limit, Enable5h: true}, quotaAllocationAccount(false), usage)
	if err := service.Check(context.Background(), 11, 22, selected); err != nil {
		t.Fatalf("expected API-key-allowed group to skip allocation, got %v", err)
	}
}

func TestOpenAIQuotaAllocationCheckRequiresSingleOAuthOnlyAccount(t *testing.T) {
	limit := 20.0
	usage := &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{UsedPercent: 100, LimitWindowSeconds: 60}}
	selected := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	multiple := quotaAllocationAccount(true)
	service := NewOpenAIQuotaAllocationService(
		&quotaAllocationAccountRepoStub{accounts: []Account{multiple, {ID: 8, Platform: PlatformOpenAI, Type: AccountTypeOAuth}}},
		&quotaAllocationUsageRepoStub{userCost: 10, totalCost: 10},
		&quotaAllocationRateRepoStub{allocation: &OpenAIQuotaAllocationConfig{Percentage: &limit, Enable5h: true}},
		&quotaAllocationQuotaStub{usage: &OpenAIQuotaUsage{RateLimit: usage}},
	)
	if err := service.Check(context.Background(), 11, 22, selected); err != nil {
		t.Fatalf("expected multi-account group to skip allocation, got %v", err)
	}
}

func TestQuotaWindowStartAdvancesExpiredResetBoundary(t *testing.T) {
	now := time.Unix(2*3600+30*60, 0)
	window := &OpenAIRateLimitWindow{LimitWindowSeconds: 3600, ResetAt: 3600}

	got := quotaWindowStart(window, 0, now)
	want := time.Unix(2*3600, 0)
	if !got.Equal(want) {
		t.Fatalf("quota window start = %v, want %v", got, want)
	}
}

func TestQuotaWindowStartUsesFetchedAtWhenResetAtMissing(t *testing.T) {
	now := time.Unix(5400, 0)
	window := &OpenAIRateLimitWindow{
		LimitWindowSeconds: 3600,
		ResetAfterSeconds:  1800,
	}

	// The snapshot was fetched at 00:30 and said the next reset was at 01:00.
	// At 01:30 the active window therefore began at 01:00.
	got := quotaWindowStart(window, 1800, now)
	want := time.Unix(3600, 0)
	if !got.Equal(want) {
		t.Fatalf("quota window start = %v, want %v", got, want)
	}
}

func TestQuotaWindowStartFallsBackToCurrentWindowWithoutResetMetadata(t *testing.T) {
	now := time.Unix(5400, 0)
	window := &OpenAIRateLimitWindow{LimitWindowSeconds: 3600}

	got := quotaWindowStart(window, 0, now)
	want := time.Unix(1800, 0)
	if !got.Equal(want) {
		t.Fatalf("quota window start = %v, want %v", got, want)
	}
}
