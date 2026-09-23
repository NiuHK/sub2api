//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type enabledBindingAdminRepo struct {
	APIKeyRepository
	keys []string
	err  error
}

func (r enabledBindingAdminRepo) ListEnabledBindingKeysByGroupID(context.Context, int64) ([]string, error) {
	return r.keys, r.err
}

func TestAdminRejectEnabledBindingGroupMutation(t *testing.T) {
	ctx := context.Background()
	svc := &adminServiceImpl{apiKeyRepo: enabledBindingAdminRepo{keys: []string{"credential"}}}
	require.ErrorContains(t, svc.rejectEnabledBindingGroupMutation(ctx, 1), "edit enabled API key bindings")
	svc.apiKeyRepo = enabledBindingAdminRepo{}
	require.NoError(t, svc.rejectEnabledBindingGroupMutation(ctx, 1))
	sentinel := errors.New("database unavailable")
	svc.apiKeyRepo = enabledBindingAdminRepo{err: sentinel}
	require.ErrorIs(t, svc.rejectEnabledBindingGroupMutation(ctx, 1), sentinel)
}
