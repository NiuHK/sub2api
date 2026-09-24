package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type scopedAccountService struct {
	*stubAdminService
}

func (s *scopedAccountService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	groupIDs := []int64{77}
	if id == 2 {
		groupIDs = []int64{88}
	}
	return &service.Account{ID: id, GroupIDs: groupIDs}, nil
}

func TestRegularUserUpstreamBillingRatesUsePrivateGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	adminSvc.groups = []service.Group{{ID: 77, Name: "private-usr12"}}
	h := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 12})
		c.Next()
	})
	router.GET("/api/v1/admin/accounts/upstream-billing-rates", h.GetUpstreamBillingRates)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/upstream-billing-rates?group=999", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, int64(77), adminSvc.lastListAccounts.groupID)
}

func TestRegularUserAccountAPIGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := &scopedAccountService{newStubAdminService()}
	adminSvc.groups = []service.Group{{ID: 77, Name: "private-usr12"}, {ID: 88, Name: "private-usr123"}}
	h := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, tc := range []struct {
		method, path, body, role string
		want                     int
	}{
		{http.MethodGet, "/api/v1/admin/accounts/1", "", service.RoleUser, http.StatusOK},
		{http.MethodGet, "/api/v1/admin/accounts/2", "", service.RoleUser, http.StatusForbidden},
		{http.MethodPut, "/api/v1/admin/accounts/2", "", service.RoleUser, http.StatusForbidden},
		{http.MethodDelete, "/api/v1/admin/accounts/2", "", service.RoleUser, http.StatusForbidden},
		{http.MethodPost, "/api/v1/admin/openai/accounts/2/quota/refresh", "", service.RoleUser, http.StatusForbidden},
		{http.MethodGet, "/api/v1/admin/accounts/2", "", service.RoleAdmin, http.StatusOK},
		{http.MethodPost, "/api/v1/admin/accounts/usage/batch", `{"account_ids":[1]}`, service.RoleUser, http.StatusOK},
		{http.MethodPost, "/api/v1/admin/accounts/usage/batch", `{"account_ids":[1,2]}`, service.RoleUser, http.StatusForbidden},
		{http.MethodPost, "/api/v1/admin/accounts/bulk-update", `{"filters":{"platform":"openai"}}`, service.RoleUser, http.StatusForbidden},
		{http.MethodGet, "/api/v1/admin/accounts/data", "", service.RoleUser, http.StatusForbidden},
	} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUserRole), tc.role)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 12})
			c.Next()
		}, h.GuardUserAccountAccess)
		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/api/v1/admin/accounts/:id"},
			{http.MethodPut, "/api/v1/admin/accounts/:id"},
			{http.MethodDelete, "/api/v1/admin/accounts/:id"},
			{http.MethodPost, "/api/v1/admin/openai/accounts/:id/quota/refresh"},
			{http.MethodPost, "/api/v1/admin/accounts/usage/batch"},
			{http.MethodPost, "/api/v1/admin/accounts/bulk-update"},
			{http.MethodGet, "/api/v1/admin/accounts/data"},
		} {
			router.Handle(route.method, route.path, func(c *gin.Context) { c.Status(http.StatusOK) })
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body)))
		require.Equal(t, tc.want, rec.Code, "%s %s: %s", tc.method, tc.path, rec.Body.String())
	}
}
