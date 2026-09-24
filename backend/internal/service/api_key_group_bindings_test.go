//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type bindingUserSubRepo struct{ UserSubscriptionRepository }

func (bindingUserSubRepo) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	if userID != 7 || (groupID != 1 && groupID != 2) {
		return nil, ErrSubscriptionNotFound
	}
	return &UserSubscription{UserID: userID, GroupID: groupID}, nil
}

type bindingGroupRepo struct {
	GroupRepository
	groups map[int64]*Group
}

func (r bindingGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	group := r.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func TestAPIKeyServiceValidateGroupBindings(t *testing.T) {
	ctx := context.Background()
	svc := &APIKeyService{userSubRepo: bindingUserSubRepo{}, groupRepo: bindingGroupRepo{groups: map[int64]*Group{
		1: {ID: 1, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive},
		2: {ID: 2, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, IsExclusive: true},
		3: {ID: 3, Platform: PlatformAnthropic, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive},
		4: {ID: 4, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, Status: StatusDisabled},
		5: {ID: 5, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeStandard, Status: StatusActive},
		6: {ID: 6, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeStandard, Status: StatusActive, IsExclusive: true},
	}}}
	user := &User{ID: 7, AllowedGroups: []int64{2}}
	valid := []domain.APIKeyGroupBinding{{GroupID: 1, Priority: 0, CooldownSeconds: 0}, {GroupID: 2, Priority: 10, CooldownSeconds: 30}}
	require.NoError(t, svc.validateGroupBindings(ctx, user, valid))
	require.NoError(t, svc.validateGroupBindings(ctx, user, []domain.APIKeyGroupBinding{{GroupID: 1, Priority: 0}, {GroupID: 5, Priority: 1}}))
	for _, bindings := range [][]domain.APIKeyGroupBinding{
		nil, {{GroupID: 0}}, {{GroupID: 1, CooldownSeconds: -1}},
		{{GroupID: 1}, {GroupID: 1, Priority: 1}},
		{{GroupID: 1}, {GroupID: 2}},
		{{GroupID: 2, Priority: 2}, {GroupID: 1, Priority: 1}},
		{{GroupID: 1}, {GroupID: 3, Priority: 1}},
		{{GroupID: 4}},
		{{GroupID: 6}},
	} {
		require.Error(t, svc.validateGroupBindings(ctx, user, bindings), "%+v", bindings)
	}
	require.ErrorIs(t, svc.validateGroupBindings(ctx, &User{}, valid), ErrGroupNotAllowed)
}

type bindingUserRepo struct{ UserRepository }

func (bindingUserRepo) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 7, AllowedGroups: []int64{2}}, nil
}

type bindingAPIKeyRepo struct {
	APIKeyRepository
	key    *APIKey
	fields APIKeyUpdateFields
}

func (r *bindingAPIKeyRepo) Create(_ context.Context, key *APIKey) error {
	r.key = key
	key.ID = 11
	return nil
}
func (r *bindingAPIKeyRepo) GetByID(context.Context, int64) (*APIKey, error) {
	clone := *r.key
	return &clone, nil
}
func (r *bindingAPIKeyRepo) Update(_ context.Context, key *APIKey, fields APIKeyUpdateFields) error {
	r.key = key
	r.fields = fields
	return nil
}

func TestAPIKeyServiceGroupBindingsCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	repo := &bindingAPIKeyRepo{}
	svc := &APIKeyService{cfg: &config.Config{}, apiKeyRepo: repo, userRepo: bindingUserRepo{}, userSubRepo: bindingUserSubRepo{}, groupRepo: bindingGroupRepo{groups: map[int64]*Group{
		1: {ID: 1, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive},
		2: {ID: 2, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, IsExclusive: true},
	}}}
	bindings := []domain.APIKeyGroupBinding{{GroupID: 1, Priority: 0}, {GroupID: 2, Priority: 1, CooldownSeconds: 15}}
	_, err := svc.Create(ctx, 7, CreateAPIKeyRequest{Name: "test", GroupBindingsEnabled: true, GroupBindings: bindings})
	require.NoError(t, err)
	require.Equal(t, int64(1), *repo.key.GroupID)
	require.Equal(t, bindings, repo.key.GroupBindings)
	other := int64(2)
	_, err = svc.Update(ctx, repo.key.ID, 7, UpdateAPIKeyRequest{GroupID: &other})
	require.ErrorIs(t, err, ErrInvalidGroupBindings)
	revised := []domain.APIKeyGroupBinding{{GroupID: 2, Priority: 0, CooldownSeconds: 7}, {GroupID: 1, Priority: 1}}
	_, err = svc.Update(ctx, repo.key.ID, 7, UpdateAPIKeyRequest{GroupBindings: &revised})
	require.NoError(t, err)
	require.True(t, repo.fields.GroupBindings)
	require.True(t, repo.fields.GroupID)
	require.Equal(t, other, *repo.key.GroupID)
	disabled := false
	_, err = svc.Update(ctx, repo.key.ID, 7, UpdateAPIKeyRequest{GroupBindingsEnabled: &disabled})
	require.NoError(t, err)
	require.False(t, repo.key.GroupBindingsEnabled)
	require.Empty(t, repo.key.GroupBindings)
	require.Equal(t, other, *repo.key.GroupID, "disabled keys retain their primary legacy group")
}

func TestAPIKeyServiceUpdatePreservesExistingDisabledBinding(t *testing.T) {
	ctx := context.Background()
	first := int64(4)
	old := []domain.APIKeyGroupBinding{{GroupID: 4, Priority: 10, CooldownSeconds: 30}, {GroupID: 1, Priority: 100, CooldownSeconds: 30}, {GroupID: 5, Priority: 200}}
	repo := &bindingAPIKeyRepo{key: &APIKey{ID: 11, UserID: 7, GroupID: &first, GroupBindingsEnabled: true, GroupBindings: old}}
	svc := &APIKeyService{cfg: &config.Config{}, apiKeyRepo: repo, userRepo: bindingUserRepo{}, userSubRepo: bindingUserSubRepo{}, groupRepo: bindingGroupRepo{groups: map[int64]*Group{
		1: {ID: 1, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive},
		4: {ID: 4, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeStandard, Status: StatusDisabled},
		5: {ID: 5, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeStandard, Status: StatusActive},
	}}}
	active := int64(1)
	updated := []domain.APIKeyGroupBinding{{GroupID: 4, Priority: 10, CooldownSeconds: 30}, {GroupID: 1, Priority: 100, CooldownSeconds: 45}}
	_, err := svc.Update(ctx, 11, 7, UpdateAPIKeyRequest{GroupID: &active, GroupBindings: &updated})
	require.NoError(t, err)
	require.Equal(t, updated, repo.key.GroupBindings)
	require.Equal(t, active, *repo.key.GroupID, "primary must be the first active binding")

	removed := []domain.APIKeyGroupBinding{{GroupID: 1, Priority: 100, CooldownSeconds: 45}}
	_, err = svc.Update(ctx, 11, 7, UpdateAPIKeyRequest{GroupBindings: &removed})
	require.NoError(t, err)
	require.Equal(t, removed, repo.key.GroupBindings)
	_, err = svc.Update(ctx, 11, 7, UpdateAPIKeyRequest{GroupBindings: &updated})
	require.ErrorIs(t, err, ErrInvalidGroupBindings, "removed disabled bindings cannot be reintroduced")

	inactiveOnly := []domain.APIKeyGroupBinding{{GroupID: 4, Priority: 10}}
	_, err = svc.Update(ctx, 11, 7, UpdateAPIKeyRequest{GroupBindings: &inactiveOnly})
	require.ErrorIs(t, err, ErrInvalidGroupBindings)
}

func TestAPIKeyAuthSnapshotGroupBindingsRoundTrip(t *testing.T) {
	svc := &APIKeyService{}
	bindings := []domain.APIKeyGroupBinding{{GroupID: 1, Priority: 0, CooldownSeconds: 12}, {GroupID: 2, Priority: 1, CooldownSeconds: 0}}
	first := int64(1)
	source := &APIKey{ID: 5, UserID: 7, GroupID: &first, GroupBindingsEnabled: true, GroupBindings: bindings, User: &User{ID: 7}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), source)
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var decoded APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(data, &decoded))
	key, ok, err := svc.applyAuthCacheEntry("credential", &APIKeyAuthCacheEntry{Snapshot: &decoded})
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, key.GroupBindingsEnabled)
	require.Equal(t, bindings, key.GroupBindings)
	decoded.Version--
	key, ok, err = svc.applyAuthCacheEntry("credential", &APIKeyAuthCacheEntry{Snapshot: &decoded})
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, key)
}
