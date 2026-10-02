package admin

import (
	"encoding/json"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type quotaShareResponse struct {
	UserID              int64    `json:"user_id"`
	Username            string   `json:"username,omitempty"`
	Email               string   `json:"email,omitempty"`
	FiveHourPercent     float64  `json:"five_hour_percent"`
	SevenDayPercent     float64  `json:"seven_day_percent"`
	FiveHourUsedPercent *float64 `json:"five_hour_used_percent,omitempty"`
	SevenDayUsedPercent *float64 `json:"seven_day_used_percent,omitempty"`
}

func (h *AccountHandler) quotaShareAccount(c *gin.Context) (*service.Account, bool) {
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		response.ErrorFrom(c, infraerrors.Forbidden("ADMIN_REQUIRED", "administrator access required"))
		return nil, false
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id"))
		return nil, false
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	if account == nil || account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeOAuth {
		response.ErrorFrom(c, infraerrors.BadRequest("QUOTA_SHARE_UNSUPPORTED", "quota shares require an OpenAI OAuth account"))
		return nil, false
	}
	return account, true
}

// GetAccountQuotaShares GET /admin/accounts/:id/quota-shares
func (h *AccountHandler) GetAccountQuotaShares(c *gin.Context) {
	account, ok := h.quotaShareAccount(c)
	if !ok {
		return
	}
	if h.accountQuotaShare == nil {
		response.Error(c, 503, "quota share service unavailable")
		return
	}
	shares, err := h.accountQuotaShare.GetForAccounts(c.Request.Context(), []int64{account.ID})
	if err != nil {
		response.Error(c, 500, "failed to load quota shares")
		return
	}
	response.Success(c, gin.H{"shares": h.quotaShareResponses(c, account, shares[account.ID])})
}

// UpdateAccountQuotaShares PUT /admin/accounts/:id/quota-shares
func (h *AccountHandler) UpdateAccountQuotaShares(c *gin.Context) {
	account, ok := h.quotaShareAccount(c)
	if !ok {
		return
	}
	var req updateAccountQuotaSharesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_QUOTA_SHARES", "invalid quota shares payload"))
		return
	}
	seen := make(map[int64]bool, len(req.Shares))
	for _, share := range req.Shares {
		if share.UserID <= 0 || seen[share.UserID] {
			response.ErrorFrom(c, infraerrors.BadRequest("INVALID_QUOTA_SHARES", "user ids must be positive and unique"))
			return
		}
		seen[share.UserID] = true
		for _, value := range []float64{share.FiveHourPercent, share.SevenDayPercent} {
			if value != -1 && (value < 0 || value > 100) {
				response.ErrorFrom(c, infraerrors.BadRequest("INVALID_QUOTA_SHARES", "quota percentages must be -1 or between 0 and 100"))
				return
			}
		}
	}
	if h.accountQuotaShare == nil {
		response.Error(c, 503, "quota share service unavailable")
		return
	}
	shares := make([]service.AccountUserQuotaShare, len(req.Shares))
	for i, share := range req.Shares {
		shares[i] = service.AccountUserQuotaShare{AccountID: account.ID, UserID: share.UserID, FiveHourPercent: share.FiveHourPercent, SevenDayPercent: share.SevenDayPercent}
	}
	if err := h.accountQuotaShare.Replace(c.Request.Context(), account.ID, shares); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"shares": req.Shares})
}

func (h *AccountHandler) quotaShareResponses(c *gin.Context, account *service.Account, shares []service.AccountUserQuotaShare) []quotaShareResponse {
	fiveHourUsage := h.quotaShareUsageSnapshot(c, account, "five_hour")
	sevenDayUsage := h.quotaShareUsageSnapshot(c, account, "seven_day")
	out := make([]quotaShareResponse, 0, len(shares))
	for _, share := range shares {
		item := quotaShareResponse{UserID: share.UserID, FiveHourPercent: share.FiveHourPercent, SevenDayPercent: share.SevenDayPercent}
		if user, err := h.adminService.GetUser(c.Request.Context(), share.UserID); err == nil && user != nil {
			item.Username = user.Username
			item.Email = user.Email
		}
		item.FiveHourUsedPercent = quotaShareEstimatedUsage(account, fiveHourUsage, "codex_5h_used_percent", share.UserID)
		item.SevenDayUsedPercent = quotaShareEstimatedUsage(account, sevenDayUsage, "codex_7d_used_percent", share.UserID)
		out = append(out, item)
	}
	return out
}

func (h *AccountHandler) quotaShareUsageSnapshot(c *gin.Context, account *service.Account, windowKind string) service.AccountUserQuotaShareUsageSnapshot {
	if account == nil || h.accountQuotaShare == nil {
		return service.AccountUserQuotaShareUsageSnapshot{}
	}
	var resetAt *time.Time
	if raw, ok := account.Extra["codex_"+map[string]string{"five_hour": "5h", "seven_day": "7d"}[windowKind]+"_reset_at"]; ok {
		if parsed, ok := quotaShareTimeValue(raw); ok {
			resetAt = &parsed
		}
	}
	snapshot, err := h.accountQuotaShare.GetUsageSnapshot(c.Request.Context(), account.ID, windowKind, resetAt)
	if err != nil {
		return service.AccountUserQuotaShareUsageSnapshot{}
	}
	return snapshot
}

func quotaShareEstimatedUsage(account *service.Account, snapshot service.AccountUserQuotaShareUsageSnapshot, usedKey string, userID int64) *float64 {
	if account == nil {
		return nil
	}
	used, ok := quotaShareNumberValue(account.Extra[usedKey])
	if !ok {
		return nil
	}
	if snapshot.AccountCost <= 0 {
		if used == 0 {
			zero := float64(0)
			return &zero
		}
		return nil
	}
	userCost := snapshot.UserCosts[userID]
	value := used * userCost / snapshot.AccountCost
	return &value
}

func quotaShareNumberValue(value any) (float64, bool) {
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
		parsed, err := strconv.ParseFloat(v, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func quotaShareTimeValue(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, v)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}
