//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// A multi-group request keeps the authenticated key's identity while the
// candidate group determines pricing and the recorded billing group.
func TestGroupBindingUsageLogUsesOriginalKeyAndSelectedGroup(t *testing.T) {
	primaryID, selectedID := int64(11), int64(22)
	original := &APIKey{
		ID: 5, UserID: 7, GroupID: &primaryID,
		Group: &Group{ID: primaryID}, GroupBindingsEnabled: true,
	}
	attempt := *original
	attempt.GroupID = &selectedID
	attempt.Group = &Group{ID: selectedID}
	attempt.GroupBindingsEnabled = false
	user := &User{ID: 7}
	account := &Account{ID: 9}
	subscription := &UserSubscription{ID: 31}
	result := &ForwardResult{Model: "gpt-test"}
	usage := (&GatewayService{}).buildRecordUsageLog(context.Background(), &recordUsageCoreInput{}, result, &attempt, user, account, subscription, "gpt-test", 1, 1, 1, 0, false, nil)
	require.Equal(t, original.ID, usage.APIKeyID)
	require.Equal(t, account.ID, usage.AccountID)
	require.Equal(t, &selectedID, usage.GroupID)
	require.Equal(t, &subscription.ID, usage.SubscriptionID)
	require.Equal(t, &primaryID, original.GroupID, "routing must not mutate the original key")
}
