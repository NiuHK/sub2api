package service

import (
	"context"
	"time"
)

type AccountUserQuotaShare struct {
	AccountID       int64
	UserID          int64
	FiveHourPercent float64
	SevenDayPercent float64
}

type AccountQuotaShareDecision struct {
	Eligible bool
	Known    bool
	ResetAt  *time.Time
	Reason   string
}

type AccountQuotaShareUsage struct {
	AccountID  int64
	WindowKind string
	ResetAt    time.Time
	Cost       float64
}
type AccountUserQuotaShareUsage struct {
	AccountID, UserID int64
	WindowKind        string
	ResetAt           time.Time
	Cost              float64
}

// AccountUserQuotaShareUsageSnapshot contains the current-window aggregate and
// per-user costs used to estimate each user's share of the upstream quota.
type AccountUserQuotaShareUsageSnapshot struct {
	AccountCost float64
	UserCosts   map[int64]float64
}

type AccountUserQuotaShareService interface {
	List(ctx context.Context, userID int64, accountIDs []int64) ([]AccountUserQuotaShare, error)
	GetForAccounts(ctx context.Context, accountIDs []int64) (map[int64][]AccountUserQuotaShare, error)
	GetUsageSnapshot(ctx context.Context, accountID int64, windowKind string, resetAt *time.Time) (AccountUserQuotaShareUsageSnapshot, error)
	Replace(ctx context.Context, accountID int64, shares []AccountUserQuotaShare) error
	CheckEligible(ctx context.Context, accountID, userID int64, windowKind string) (bool, error)
	Evaluate(ctx context.Context, account *Account, userID int64) (AccountQuotaShareDecision, error)
	RecordUsage(ctx context.Context, accountID, userID int64, windowKind string, resetAt time.Time, cost float64) error
}
