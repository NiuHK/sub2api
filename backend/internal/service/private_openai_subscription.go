package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
)

// ensurePrivateOpenAISubscription provisions only the supplied user. Keep the
// names/markers in sync with tools/provision_private_openai.sql. Existing rows
// are never renewed or taken over. The transaction prevents half-provisioning.
func ensurePrivateOpenAISubscription(ctx context.Context, client *dbent.Client, userID int64) error {
	if client == nil || userID <= 0 {
		return fmt.Errorf("private subscription: invalid user or client")
	}
	name := fmt.Sprintf("private-usr%d", userID)
	description := fmt.Sprintf("Managed private OpenAI group for user %d (private-subscription-v1)", userID)
	note := fmt.Sprintf("Managed private OpenAI subscription for user %d (private-subscription-v1)", userID)
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	g, err := tx.Group.Query().Where(group.NameEQ(name), group.DeletedAtIsNil()).Only(ctx)
	if dbent.IsNotFound(err) {
		g, err = tx.Group.Create().SetName(name).SetDescription(description).SetPlatform(PlatformOpenAI).
			SetSubscriptionType(SubscriptionTypeSubscription).SetIsExclusive(true).SetStatus(StatusActive).
			SetDefaultValidityDays(1000).Save(ctx)
	}
	if err != nil {
		return err
	}
	if g.Description == nil || *g.Description != description || g.Platform != PlatformOpenAI ||
		g.SubscriptionType != SubscriptionTypeSubscription || !g.IsExclusive || g.Status != StatusActive {
		return fmt.Errorf("private subscription: conflicting group %s", name)
	}
	foreign, err := tx.UserSubscription.Query().Where(usersubscription.GroupIDEQ(g.ID), usersubscription.UserIDNEQ(userID)).Exist(ctx)
	if err != nil {
		return err
	}
	if foreign {
		return fmt.Errorf("private subscription: group %s has another subscriber", name)
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
	return tx.Commit()
}
