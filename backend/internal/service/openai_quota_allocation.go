package service

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type groupUserActualCostWindowReader interface {
	GetGroupUserActualCostWindow(ctx context.Context, groupID, userID int64, startTime time.Time) (userCost, totalCost float64, err error)
}

type openAIQuotaUsageReader interface {
	QueryUsage(ctx context.Context, accountID int64) (*OpenAIQuotaUsage, error)
}

// OpenAIQuotaAllocationService enforces the small, explicit first-stage rule:
// one OpenAI OAuth account per group, with the user's local ActualCost share
// multiplied by the account's live upstream window utilization.
type OpenAIQuotaAllocationService struct {
	accountRepo       AccountRepository
	usageLogRepo      UsageLogRepository
	userGroupRateRepo UserGroupRateRepository
	quotaService      openAIQuotaUsageReader
}

func NewOpenAIQuotaAllocationService(accountRepo AccountRepository, usageLogRepo UsageLogRepository, userGroupRateRepo UserGroupRateRepository, quotaService openAIQuotaUsageReader) *OpenAIQuotaAllocationService {
	return &OpenAIQuotaAllocationService{accountRepo: accountRepo, usageLogRepo: usageLogRepo, userGroupRateRepo: userGroupRateRepo, quotaService: quotaService}
}

// Check returns nil when the feature is not configured or the group is outside
// the supported first-stage shape. Upstream/local read failures fail open so a
// monitoring outage does not take down otherwise valid API traffic.
func (s *OpenAIQuotaAllocationService) Check(ctx context.Context, userID, groupID int64, selected *Account) error {
	if s == nil || selected == nil || groupID <= 0 || userID <= 0 || s.quotaService == nil {
		return nil
	}
	rateRepo, ok := s.userGroupRateRepo.(OpenAIQuotaAllocationRepository)
	if !ok || rateRepo == nil {
		return nil
	}
	allocation, err := rateRepo.GetQuotaAllocationByUserAndGroup(ctx, userID, groupID)
	if err != nil || allocation == nil || allocation.Percentage == nil || (!allocation.Enable5h && !allocation.Enable7d) {
		return nil
	}

	if selected.Platform != PlatformOpenAI || !selected.IsOpenAIOAuth() {
		return nil
	}
	accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
	if err != nil || len(accounts) != 1 || accounts[0].ID != selected.ID || !accounts[0].IsOpenAIOAuth() || !accountGroupRequiresOAuth(accounts[0], groupID) {
		return nil
	}
	usageReader, ok := s.usageLogRepo.(groupUserActualCostWindowReader)
	if !ok || usageReader == nil {
		return nil
	}
	usage, err := s.quotaService.QueryUsage(ctx, selected.ID)
	if err != nil || usage == nil || usage.RateLimit == nil {
		return nil
	}
	// LimitReached is an account-wide signal. Check it before inspecting the
	// configured windows so an upstream response with a missing/invalid window
	// cannot accidentally fall through and allow traffic.
	if usage.RateLimit.LimitReached {
		return newQuotaAllocationError("OpenAI upstream account quota is exhausted")
	}

	now := time.Now()
	windows := []struct {
		window  *OpenAIRateLimitWindow
		enabled bool
	}{
		{usage.RateLimit.PrimaryWindow, allocation.Enable5h},
		{usage.RateLimit.SecondaryWindow, allocation.Enable7d},
	}
	for _, item := range windows {
		window := item.window
		if !item.enabled {
			continue
		}
		if window == nil || window.LimitWindowSeconds <= 0 {
			continue
		}
		start := quotaWindowStart(window, usage.FetchedAt, now)
		userCost, totalCost, readErr := usageReader.GetGroupUserActualCostWindow(ctx, groupID, userID, start)
		if readErr != nil || totalCost <= 0 {
			continue
		}
		estimated := window.UsedPercent * userCost / totalCost
		if estimated > *allocation.Percentage {
			return newQuotaAllocationError(fmt.Sprintf("OpenAI quota allocation exceeded: estimated %.2f%%, configured %.2f%%", estimated, *allocation.Percentage))
		}
	}
	return nil
}

func accountGroupRequiresOAuth(account Account, groupID int64) bool {
	for _, group := range account.Groups {
		if group != nil && group.ID == groupID {
			return group.RequireOAuthOnly
		}
	}
	return false
}

// quotaWindowStart returns the beginning of the current fixed rate-limit
// window. ResetAt is the next boundary observed by the upstream. A stale
// snapshot can have a ResetAt in the past, so advance that boundary by whole
// window lengths until it is in the future instead of using now-window,
// which would include the tail of the previous window.
func quotaWindowStart(window *OpenAIRateLimitWindow, fetchedAtUnix int64, now time.Time) time.Time {
	if window == nil || window.LimitWindowSeconds <= 0 {
		return now
	}
	windowDuration := time.Duration(window.LimitWindowSeconds) * time.Second
	resetAtUnix := window.ResetAt
	if resetAtUnix <= 0 && fetchedAtUnix > 0 && window.ResetAfterSeconds > 0 {
		resetAtUnix = fetchedAtUnix + window.ResetAfterSeconds
	}
	if resetAtUnix <= 0 {
		return now.Add(-windowDuration)
	}

	resetAt := time.Unix(resetAtUnix, 0)
	if !resetAt.After(now) {
		elapsed := now.Sub(resetAt)
		// Move to the first boundary strictly after now. At an exact boundary,
		// that means advancing one more cycle because the boundary starts the
		// new window.
		resetAt = resetAt.Add((elapsed/windowDuration + 1) * windowDuration)
	}
	return resetAt.Add(-windowDuration)
}

type quotaAllocationError struct{ message string }

func (e *quotaAllocationError) Error() string { return e.message }

func newQuotaAllocationError(message string) error { return &quotaAllocationError{message: message} }

func IsOpenAIQuotaAllocationExceeded(err error) bool {
	_, ok := err.(*quotaAllocationError)
	return ok
}

const OpenAIQuotaAllocationHTTPStatus = http.StatusTooManyRequests

func ProvideOpenAIQuotaAllocationService(accountRepo AccountRepository, usageLogRepo UsageLogRepository, userGroupRateRepo UserGroupRateRepository, quotaService *OpenAIQuotaService) *OpenAIQuotaAllocationService {
	return NewOpenAIQuotaAllocationService(accountRepo, usageLogRepo, userGroupRateRepo, quotaService)
}
