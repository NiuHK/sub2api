//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestEnsurePrivateOpenAISubscription(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:private_openai_test?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	first, err := client.User.Create().SetEmail("first-private@example.com").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	second, err := client.User.Create().SetEmail("second-private@example.com").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	require.NoError(t, ensurePrivateOpenAISubscription(ctx, client, first.ID))
	g, err := client.Group.Query().Where(group.NameEQ(fmt.Sprintf("private-usr%d", first.ID))).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, g.Platform)
	require.Equal(t, SubscriptionTypeSubscription, g.SubscriptionType)
	require.True(t, g.IsExclusive)
	sub, err := client.UserSubscription.Query().Where(usersubscription.UserIDEQ(first.ID), usersubscription.GroupIDEQ(g.ID)).Only(ctx)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().AddDate(0, 0, 1000), sub.ExpiresAt, time.Minute)
	require.NoError(t, ensurePrivateOpenAISubscription(ctx, client, first.ID))
	require.Equal(t, 1, client.UserSubscription.Query().CountX(ctx))
	require.Equal(t, 1, client.Group.Query().CountX(ctx))
	require.NoError(t, ensurePrivateOpenAISubscription(ctx, client, second.ID))
	require.Equal(t, 2, client.UserSubscription.Query().CountX(ctx))
	// A second user's entitlement to the first private group must block reuse.
	_, err = client.UserSubscription.Create().SetUserID(second.ID).SetGroupID(g.ID).
		SetStartsAt(time.Now()).SetExpiresAt(time.Now().AddDate(0, 0, 1)).Save(ctx)
	require.NoError(t, err)
	require.Error(t, ensurePrivateOpenAISubscription(ctx, client, first.ID))
}
