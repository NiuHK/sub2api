package middleware

import "testing"

func TestSharedAccountManagementPaths(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		allowed      bool
	}{
		{"GET", "/api/v1/admin/accounts", true},
		{"DELETE", "/api/v1/admin/accounts/12", true},
		{"POST", "/api/v1/admin/openai/create-from-oauth", true},
		{"GET", "/api/v1/admin/groups/all", true},
		{"GET", "/api/v1/admin/proxies/all", true},
		{"GET", "/api/v1/admin/settings/web-search-emulation", true},
		{"PUT", "/api/v1/admin/settings/web-search-emulation", false},
		{"POST", "/api/v1/admin/groups/all", false},
		{"GET", "/api/v1/admin/accounts-other", false},
		{"GET", "/api/v1/admin/users", false},
	} {
		if got := isSharedAccountManagementPath(tc.method, tc.path); got != tc.allowed {
			t.Errorf("%s %s: got %v, want %v", tc.method, tc.path, got, tc.allowed)
		}
	}
}
