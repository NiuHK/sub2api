package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AccountHandler) privateGroupID(c *gin.Context, userID int64) (int64, error) {
	name := "private-usr" + strconv.FormatInt(userID, 10)
	groups, _, err := h.adminService.ListGroups(c.Request.Context(), 1, 10000, "", "", name, nil, "", "")
	if err != nil {
		return 0, err
	}
	for _, group := range groups {
		if group.Name == name {
			return group.ID, nil
		}
	}
	return 0, nil
}

// GuardUserAccountAccess applies to every admin account route shared with a
// regular user. Unrecognized account operations fail closed rather than
// inheriting unrestricted administrator access.
func (h *AccountHandler) GuardUserAccountAccess(c *gin.Context) {
	role, _ := middleware.GetUserRoleFromContext(c)
	if role == service.RoleAdmin {
		c.Next()
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		middleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
		return
	}
	path := c.FullPath()
	method := c.Request.Method
	// These reads supply the existing account form; none returns account details.
	if method == http.MethodGet && (path == "/api/v1/admin/groups/all" ||
		path == "/api/v1/admin/proxies/all" || path == "/api/v1/admin/settings/web-search-emulation") {
		c.Next()
		return
	}
	if path == "/api/v1/admin/accounts" && (method == http.MethodGet || method == http.MethodPost) {
		c.Next()
		return
	}
	if method == http.MethodGet && (path == "/api/v1/admin/accounts/upstream-billing-rates" ||
		path == "/api/v1/admin/accounts/antigravity/default-model-mapping" ||
		path == "/api/v1/admin/accounts/upstream-billing-probe/settings" ||
		path == "/api/v1/admin/accounts/ollama-cloud-usage/settings" ||
		path == "/api/v1/admin/accounts/opencode-go-usage/settings") {
		c.Next()
		return
	}
	// All bulk operations must name their targets; filter-based bulk writes,
	// exports and imports cannot be safely scoped to one user's accounts.
	if strings.HasPrefix(path, "/api/v1/admin/accounts/") &&
		(strings.HasSuffix(path, "/batch") || strings.Contains(path, "/batch-") ||
			path == "/api/v1/admin/accounts/bulk-update") {
		if path == "/api/v1/admin/accounts/batch" { // creation uses the server-side private binding
			c.Next()
			return
		}
		var body struct {
			AccountIDs []int64 `json:"account_ids"`
		}
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil || json.Unmarshal(raw, &body) != nil || len(body.AccountIDs) == 0 {
			middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Account access denied")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		groupID, err := h.privateGroupID(c, subject.UserID)
		if err != nil {
			middleware.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load private group")
			return
		}
		if groupID == 0 || !h.ownsAccounts(c, body.AccountIDs, groupID) {
			return
		}
		c.Next()
		return
	}
	// ID-bearing routes (including platform quota and refresh endpoints).
	if rawID := c.Param("id"); rawID != "" {
		if path == "/api/v1/admin/accounts/:id/shadow" || path == "/api/v1/admin/accounts/:id/duplicate" {
			middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Account access denied")
			return
		}
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Account access denied")
			return
		}
		groupID, err := h.privateGroupID(c, subject.UserID)
		if err != nil {
			middleware.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load private group")
			return
		}
		if groupID == 0 || !h.ownsAccounts(c, []int64{id}, groupID) {
			return
		}
		c.Next()
		return
	}
	// OAuth credential exchange and account creation do not read stored accounts.
	switch path {
	case "/api/v1/admin/openai/generate-auth-url", "/api/v1/admin/openai/exchange-code",
		"/api/v1/admin/openai/refresh-token", "/api/v1/admin/openai/create-from-oauth",
		"/api/v1/admin/openai/create-from-codex-pat",
		"/api/v1/admin/accounts/generate-auth-url", "/api/v1/admin/accounts/generate-setup-token-url",
		"/api/v1/admin/accounts/exchange-code", "/api/v1/admin/accounts/exchange-setup-token-code",
		"/api/v1/admin/accounts/cookie-auth", "/api/v1/admin/accounts/setup-token-cookie-auth",
		"/api/v1/admin/gemini/oauth/auth-url", "/api/v1/admin/gemini/oauth/exchange-code",
		"/api/v1/admin/gemini/oauth/capabilities", "/api/v1/admin/antigravity/oauth/auth-url",
		"/api/v1/admin/antigravity/oauth/exchange-code", "/api/v1/admin/antigravity/oauth/refresh-token",
		"/api/v1/admin/grok/oauth/capabilities", "/api/v1/admin/grok/oauth/auth-url",
		"/api/v1/admin/grok/oauth/exchange-code", "/api/v1/admin/grok/oauth/refresh-token",
		"/api/v1/admin/grok/oauth/sso-token", "/api/v1/admin/grok/oauth/password",
		"/api/v1/admin/grok/oauth/create-from-oauth":
		c.Next()
		return
	}
	middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Account access denied")
}

func (h *AccountHandler) ownsAccounts(c *gin.Context, ids []int64, groupID int64) bool {
	for _, id := range ids {
		account, err := h.adminService.GetAccount(c.Request.Context(), id)
		if err != nil {
			middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Account access denied")
			return false
		}
		found := false
		for _, boundID := range account.GroupIDs {
			if boundID == groupID {
				found = true
				break
			}
		}
		if !found {
			middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Account access denied")
			return false
		}
	}
	return true
}
