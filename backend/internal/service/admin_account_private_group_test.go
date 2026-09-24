package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type privateCreateGroupRepo struct {
	GroupRepository
	groups []Group
}

func (r *privateCreateGroupRepo) ListActiveByPlatform(_ context.Context, _ string) ([]Group, error) {
	return r.groups, nil
}

type privateCreateAccountRepo struct {
	AccountRepository
	bound   []int64
	created *Account
}

func (r *privateCreateAccountRepo) Create(_ context.Context, a *Account) error {
	a.ID = 100
	r.created = a
	return nil
}

func (r *privateCreateAccountRepo) BindGroups(_ context.Context, _ int64, ids []int64) error {
	r.bound = append([]int64(nil), ids...)
	return nil
}

func TestRegularUserOpenAICreateForcesOwnPrivateGroup(t *testing.T) {
	groups := &privateCreateGroupRepo{groups: []Group{{ID: 11, Name: "private-usr12"}, {ID: 22, Name: "private-usr123"}}}
	for _, tc := range []struct {
		userID int64
		want   []int64
	}{
		{12, []int64{11}},
		{123, []int64{22}},
	} {
		repo := &privateCreateAccountRepo{}
		svc := &adminServiceImpl{groupRepo: groups, accountRepo: repo}
		_, err := svc.CreateAccount(WithAccountCreatorUserID(context.Background(), tc.userID), &CreateAccountInput{
			Name: "my-openai", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "test"}, GroupIDs: []int64{999},
			SkipDefaultGroupBind: true, SkipMixedChannelCheck: true,
		})
		require.NoError(t, err)
		require.Equal(t, tc.want, repo.bound)
	}

	repo := &privateCreateAccountRepo{}
	svc := &adminServiceImpl{groupRepo: groups, accountRepo: repo}
	_, err := svc.CreateAccount(WithAccountCreatorUserID(context.Background(), 7), &CreateAccountInput{
		Name: "my-openai", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test"}, GroupIDs: []int64{11},
	})
	require.Error(t, err)
	require.Nil(t, repo.created)

	// Admin-created OpenAI accounts retain their explicitly selected groups.
	adminRepo := &privateCreateAccountRepo{}
	adminSvc := &adminServiceImpl{groupRepo: groups, accountRepo: adminRepo}
	_, err = adminSvc.CreateAccount(context.Background(), &CreateAccountInput{
		Name: "admin-openai", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test"}, GroupIDs: []int64{999},
		SkipMixedChannelCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{999}, adminRepo.bound)
}
