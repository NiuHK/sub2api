package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
)

// PrivateAccountPlatforms are platforms for which regular users can own upstream accounts.
// Composite is intentionally excluded because it is a routing layer, not an upstream account.
var PrivateAccountPlatforms = []string{
	PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformGrok, PlatformAntigravity,
	PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo,
}

func containsPlatform(platforms []string, platform string) bool {
	for _, candidate := range platforms {
		if candidate == platform {
			return true
		}
	}
	return false
}

// ensurePrivateOpenAISubscription remains as a compatibility wrapper for existing callers.
func ensurePrivateOpenAISubscription(ctx context.Context, client *dbent.Client, userID int64) error {
	return ensurePrivatePlatformSubscriptions(ctx, client, userID, []string{PlatformOpenAI})
}

func ensurePrivatePlatformSubscriptions(ctx context.Context, client *dbent.Client, userID int64, platforms []string) error {
	if client == nil || userID <= 0 {
		return fmt.Errorf("private subscription: invalid user or client")
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, platform := range platforms {
		if platform == "" || platform == PlatformComposite {
			continue
		}
		name := fmt.Sprintf("private-usr%d-%s", userID, platform)
		if platform == PlatformOpenAI {
			name = fmt.Sprintf("private-usr%d", userID)
		}
		marker := "private-subscription-v2"
		label := platform
		if platform == PlatformOpenAI {
			marker = "private-subscription-v1"
			label = "OpenAI" // preserve the original managed identity exactly
		}
		description := fmt.Sprintf("Managed private %s group for user %d (%s)", label, userID, marker)
		note := fmt.Sprintf("Managed private %s subscription for user %d (%s)", label, userID, marker)
		g, err := tx.Group.Query().Where(group.NameEQ(name), group.DeletedAtIsNil()).Only(ctx)
		if dbent.IsNotFound(err) {
			g, err = tx.Group.Create().SetName(name).SetDescription(description).SetPlatform(platform).
				SetSubscriptionType(SubscriptionTypeSubscription).SetIsExclusive(true).SetStatus(StatusActive).
				SetDefaultValidityDays(1000).Save(ctx)
		}
		if err != nil {
			return err
		}
		if g.Description == nil || *g.Description != description || g.Platform != platform || g.SubscriptionType != SubscriptionTypeSubscription || !g.IsExclusive || g.Status != StatusActive {
			return fmt.Errorf("private subscription: conflicting group %s", name)
		}
		foreign, err := tx.UserSubscription.Query().Where(usersubscription.GroupIDEQ(g.ID), usersubscription.UserIDNEQ(userID)).Exist(ctx)
		if err != nil || foreign {
			if foreign {
				return fmt.Errorf("private subscription: group %s has another subscriber", name)
			}
			return err
		}
		sub, err := tx.UserSubscription.Query().Where(usersubscription.GroupIDEQ(g.ID), usersubscription.UserIDEQ(userID)).Only(ctx)
		if dbent.IsNotFound(err) {
			now := time.Now().UTC()
			sub, err = tx.UserSubscription.Create().SetUserID(userID).SetGroupID(g.ID).
				SetStartsAt(now).SetExpiresAt(now.AddDate(0, 0, 1000)).SetStatus(SubscriptionStatusActive).
				SetNotes(note).Save(ctx)
		}
		if err != nil {
			return err
		}
		if sub.Notes == nil || *sub.Notes != note || sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(time.Now()) {
			return fmt.Errorf("private subscription: conflicting or expired subscription for user %d", userID)
		}
	}
	return tx.Commit()
}
