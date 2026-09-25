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

func (r *privateCreateGroupRepo) ListActiveByPlatform(_ context.Context, platform string) ([]Group, error) {
	var groups []Group
	for _, group := range r.groups {
		if group.Platform == platform {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (r *privateCreateGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	for i := range r.groups {
		if r.groups[i].ID == id {
			return &r.groups[i], nil
		}
	}
	return nil, ErrGroupNotFound
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

func TestRegularUserBulkGroupChangeDenied(t *testing.T) {
	svc := &adminServiceImpl{}
	groups := []int64{77}
	_, err := svc.BulkUpdateAccounts(WithAccountCreatorUserID(context.Background(), 12), &BulkUpdateAccountsInput{GroupIDs: &groups})
	require.Error(t, err)
}

func TestRegularUserOpenAICreateForcesOwnPrivateGroup(t *testing.T) {
	groups := &privateCreateGroupRepo{groups: []Group{{ID: 11, Name: "private-usr12", Platform: PlatformOpenAI}, {ID: 22, Name: "private-usr123", Platform: PlatformOpenAI}, {ID: 33, Name: "private-usr12-anthropic", Platform: PlatformAnthropic}, {ID: 44, Name: "private-usr123-anthropic", Platform: PlatformAnthropic}}}
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

	// Other platforms also bind exclusively to their own private group.
	anthropicRepo := &privateCreateAccountRepo{}
	anthropicSvc := &adminServiceImpl{groupRepo: groups, accountRepo: anthropicRepo}
	_, err = anthropicSvc.CreateAccount(WithAccountCreatorUserID(context.Background(), 12), &CreateAccountInput{
		Name: "anthropic", Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test"}, GroupIDs: []int64{44}, SkipMixedChannelCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{33}, anthropicRepo.bound)

	// A missing private group must fail closed, even with a public or foreign group.
	foreignRepo := &privateCreateAccountRepo{}
	foreignSvc := &adminServiceImpl{groupRepo: groups, accountRepo: foreignRepo}
	_, err = foreignSvc.CreateAccount(WithAccountCreatorUserID(context.Background(), 12), &CreateAccountInput{
		Name: "foreign", Platform: PlatformGemini, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test"}, GroupIDs: []int64{22},
	})
	require.Error(t, err)
	require.Nil(t, foreignRepo.created)
}
