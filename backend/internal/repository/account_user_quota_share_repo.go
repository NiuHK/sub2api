package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountquotashareusage"
	"github.com/Wei-Shaw/sub2api/ent/accountuserquotashare"
	"github.com/Wei-Shaw/sub2api/ent/accountuserquotashareusage"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountUserQuotaShareRepository struct{ client *ent.Client }

func NewAccountUserQuotaShareRepository(client *ent.Client) service.AccountUserQuotaShareService {
	return &accountUserQuotaShareRepository{client: client}
}

func (r *accountUserQuotaShareRepository) List(ctx context.Context, userID int64, accountIDs []int64) ([]service.AccountUserQuotaShare, error) {
	q := r.client.AccountUserQuotaShare.Query().Where(accountuserquotashare.UserIDEQ(userID))
	if len(accountIDs) > 0 {
		q = q.Where(accountuserquotashare.AccountIDIn(accountIDs...))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.AccountUserQuotaShare, len(rows))
	for i, v := range rows {
		out[i] = shareRecord(v)
	}
	return out, nil
}
func (r *accountUserQuotaShareRepository) GetForAccounts(ctx context.Context, accountIDs []int64) (map[int64][]service.AccountUserQuotaShare, error) {
	out := make(map[int64][]service.AccountUserQuotaShare)
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.client.AccountUserQuotaShare.Query().Where(accountuserquotashare.AccountIDIn(accountIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, v := range rows {
		out[v.AccountID] = append(out[v.AccountID], shareRecord(v))
	}
	return out, nil
}

func (r *accountUserQuotaShareRepository) GetUsageSnapshot(ctx context.Context, accountID int64, windowKind string, resetAt *time.Time) (service.AccountUserQuotaShareUsageSnapshot, error) {
	snapshot := service.AccountUserQuotaShareUsageSnapshot{UserCosts: make(map[int64]float64)}
	if accountID <= 0 || (windowKind != "five_hour" && windowKind != "seven_day") {
		return snapshot, fmt.Errorf("invalid quota share usage lookup")
	}
	windowDuration := 5 * time.Hour
	if windowKind == "seven_day" {
		windowDuration = 7 * 24 * time.Hour
	}
	accountQuery := r.client.AccountQuotaShareUsage.Query().Where(
		accountquotashareusage.AccountIDEQ(accountID),
		accountquotashareusage.WindowKindEQ(windowKind),
	)
	userQuery := r.client.AccountUserQuotaShareUsage.Query().Where(
		accountuserquotashareusage.AccountIDEQ(accountID),
		accountuserquotashareusage.WindowKindEQ(windowKind),
	)
	anchor := resetAt
	if anchor == nil {
		latest, err := accountQuery.Order(accountquotashareusage.ByResetAt(entsql.OrderDesc())).First(ctx)
		if ent.IsNotFound(err) {
			return snapshot, nil
		}
		if err != nil {
			return snapshot, err
		}
		anchor = &latest.ResetAt
	}
	from := anchor.Add(-windowDuration / 2)
	to := anchor.Add(windowDuration / 2)
	accountRows, err := accountQuery.
		Where(accountquotashareusage.ResetAtGTE(from), accountquotashareusage.ResetAtLTE(to)).
		All(ctx)
	if err != nil {
		return snapshot, err
	}
	for _, row := range accountRows {
		snapshot.AccountCost += row.Cost
	}
	userRows, err := userQuery.
		Where(accountuserquotashareusage.ResetAtGTE(from), accountuserquotashareusage.ResetAtLTE(to)).
		All(ctx)
	if err != nil {
		return snapshot, err
	}
	for _, row := range userRows {
		// A window can contain several billing events for the same user. Keep
		// the same additive semantics as the account aggregate above; assigning
		// here would leave only whichever row Ent returned last.
		snapshot.UserCosts[row.UserID] += row.Cost
	}
	return snapshot, nil
}

func (r *accountUserQuotaShareRepository) Replace(ctx context.Context, accountID int64, shares []service.AccountUserQuotaShare) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.AccountUserQuotaShare.Update().Where(accountuserquotashare.AccountIDEQ(accountID)).SetDeletedAt(time.Now()).Save(ctx); err != nil {
		return err
	}
	for _, s := range shares {
		if s.AccountID != 0 && s.AccountID != accountID {
			return fmt.Errorf("share account mismatch: %d", s.AccountID)
		}
		if err = validateShare(s); err != nil {
			return err
		}
		_, err = tx.AccountUserQuotaShare.Create().SetAccountID(accountID).SetUserID(s.UserID).SetFiveHourPercent(s.FiveHourPercent).SetSevenDayPercent(s.SevenDayPercent).Save(ctx)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}
func (r *accountUserQuotaShareRepository) CheckEligible(ctx context.Context, accountID, userID int64, windowKind string) (bool, error) {
	if windowKind != "five_hour" && windowKind != "seven_day" {
		return false, fmt.Errorf("invalid window kind %q", windowKind)
	}
	v, err := r.client.AccountUserQuotaShare.Query().Where(accountuserquotashare.AccountIDEQ(accountID), accountuserquotashare.UserIDEQ(userID)).Only(ctx)
	if ent.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if windowKind == "five_hour" {
		return v.FiveHourPercent != 0, nil
	}
	return v.SevenDayPercent != 0, nil
}

func (r *accountUserQuotaShareRepository) Evaluate(ctx context.Context, account *service.Account, userID int64) (service.AccountQuotaShareDecision, error) {
	if account == nil || !account.IsOpenAIOAuth() || account.ID <= 0 || userID <= 0 {
		return service.AccountQuotaShareDecision{Eligible: true, Known: true}, nil
	}
	shares, err := r.client.AccountUserQuotaShare.Query().Where(accountuserquotashare.AccountIDEQ(account.ID)).All(ctx)
	if err != nil {
		return service.AccountQuotaShareDecision{}, err
	}
	if len(shares) == 0 {
		return service.AccountQuotaShareDecision{Eligible: true, Known: true}, nil
	}
	var share *ent.AccountUserQuotaShare
	for _, candidate := range shares {
		if candidate.UserID == userID {
			share = candidate
			break
		}
	}
	if share == nil {
		return service.AccountQuotaShareDecision{Eligible: false, Known: true, Reason: "account_user_quota_share_binding"}, nil
	}

	decision := service.AccountQuotaShareDecision{Eligible: true, Known: true}
	for _, window := range []struct {
		kind     string
		percent  float64
		usedKey  string
		resetKey string
	}{
		{kind: "five_hour", percent: share.FiveHourPercent, usedKey: "codex_5h_used_percent", resetKey: "codex_5h_reset_at"},
		{kind: "seven_day", percent: share.SevenDayPercent, usedKey: "codex_7d_used_percent", resetKey: "codex_7d_reset_at"},
	} {
		if window.percent < 0 {
			continue
		}
		resetAt, ok := quotaShareTime(account.Extra[window.resetKey])
		if !ok || resetAt.Before(time.Now()) {
			continue
		}
		updatedAt, ok := quotaShareTime(account.Extra["codex_usage_updated_at"])
		if !ok || time.Since(updatedAt) > 8*time.Hour {
			continue
		}
		usedPercent, ok := quotaShareNumber(account.Extra[window.usedKey])
		if !ok || usedPercent < 0 {
			continue
		}
		accountUsage, err := r.client.AccountQuotaShareUsage.Query().Where(
			accountquotashareusage.AccountIDEQ(account.ID),
			accountquotashareusage.WindowKindEQ(window.kind),
			accountquotashareusage.ResetAtEQ(resetAt),
		).Only(ctx)
		if ent.IsNotFound(err) {
			continue
		}
		if err != nil {
			return service.AccountQuotaShareDecision{}, err
		}
		userUsage, err := r.client.AccountUserQuotaShareUsage.Query().Where(
			accountuserquotashareusage.AccountIDEQ(account.ID),
			accountuserquotashareusage.UserIDEQ(userID),
			accountuserquotashareusage.WindowKindEQ(window.kind),
			accountuserquotashareusage.ResetAtEQ(resetAt),
		).Only(ctx)
		userCost := float64(0)
		if ent.IsNotFound(err) {
			if window.percent > 0 {
				continue
			}
		} else if err == nil {
			userCost = userUsage.Cost
		}
		if err != nil {
			if !ent.IsNotFound(err) {
				return service.AccountQuotaShareDecision{}, err
			}
		}
		if accountUsage.Cost <= 0 {
			continue
		}
		estimated := usedPercent * userCost / accountUsage.Cost
		if estimated >= window.percent {
			decision.Eligible = false
			decision.Known = true
			decision.ResetAt = &resetAt
			decision.Reason = "account_user_quota_share_" + window.kind
			return decision, nil
		}
	}
	return decision, nil
}

func quotaShareNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		parsed, err := v.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func quotaShareTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case string:
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}
func (r *accountUserQuotaShareRepository) RecordUsage(ctx context.Context, accountID, userID int64, windowKind string, resetAt time.Time, cost float64) error {
	if windowKind != "five_hour" && windowKind != "seven_day" {
		return fmt.Errorf("invalid window kind %q", windowKind)
	}
	if cost < 0 {
		return fmt.Errorf("cost must be non-negative")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	// A new reset_at creates a new window; retries in the same window atomically add cost.
	for _, table := range []string{"account_quota_share_usages", "account_user_quota_share_usages"} {
		var query string
		var args []any
		if table == "account_quota_share_usages" {
			query = "INSERT INTO account_quota_share_usages (account_id,window_kind,reset_at,cost) VALUES ($1,$2,$3,$4) ON CONFLICT (account_id,window_kind,reset_at) DO UPDATE SET cost = account_quota_share_usages.cost + EXCLUDED.cost"
			args = []any{accountID, windowKind, resetAt, cost}
		} else {
			query = "INSERT INTO account_user_quota_share_usages (account_id,user_id,window_kind,reset_at,cost) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (account_id,user_id,window_kind,reset_at) DO UPDATE SET cost = account_user_quota_share_usages.cost + EXCLUDED.cost"
			args = []any{accountID, userID, windowKind, resetAt, cost}
		}
		if _, err = tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}
func validateShare(s service.AccountUserQuotaShare) error {
	for _, p := range []float64{s.FiveHourPercent, s.SevenDayPercent} {
		if p < -1 || (p > -1 && (p < 0 || p > 100)) {
			return fmt.Errorf("quota share percent must be -1 or between 0 and 100")
		}
	}
	return nil
}
func shareRecord(v *ent.AccountUserQuotaShare) service.AccountUserQuotaShare {
	return service.AccountUserQuotaShare{AccountID: v.AccountID, UserID: v.UserID, FiveHourPercent: v.FiveHourPercent, SevenDayPercent: v.SevenDayPercent}
}
