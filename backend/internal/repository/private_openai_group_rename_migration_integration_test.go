//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

const privateOpenAIGroupRenameMigration = "242_rename_private_openai_groups.sql"

func TestMigration242RenamesPrivateOpenAIGroups(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	for _, name := range []string{"private-usr712345", "private-usr812345"} {
		_, err := tx.ExecContext(ctx, `
INSERT INTO groups (name, description, platform, subscription_type, is_exclusive)
VALUES ($1, 'managed private group', 'openai', 'subscription', true)
`, name)
		require.NoError(t, err)
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO groups (name, description, platform, subscription_type, is_exclusive)
VALUES ('private-usr-not-a-user-id', 'unrelated', 'openai', 'subscription', true)
`)
	require.NoError(t, err)

	applyPrivateOpenAIGroupRename(ctx, t, tx)

	var renamed, unchanged int
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT count(*) FROM groups WHERE name IN ('Private-openai-USR712345', 'Private-openai-USR812345')").Scan(&renamed))
	require.Equal(t, 2, renamed)
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT count(*) FROM groups WHERE name = 'private-usr-not-a-user-id'").Scan(&unchanged))
	require.Equal(t, 1, unchanged)

	// Reapplying is safe and does not affect already-renamed groups.
	applyPrivateOpenAIGroupRename(ctx, t, tx)
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT count(*) FROM groups WHERE name IN ('Private-openai-USR712345', 'Private-openai-USR812345')").Scan(&renamed))
	require.Equal(t, 2, renamed)
}

func TestMigration242RejectsPrivateOpenAIGroupNameCollision(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	for _, name := range []string{"private-usr912345", "Private-openai-USR912345"} {
		_, err := tx.ExecContext(ctx, `
INSERT INTO groups (name, description, platform, subscription_type, is_exclusive)
VALUES ($1, 'group', 'openai', 'subscription', true)
`, name)
		require.NoError(t, err)
	}

	migrationSQL, err := dbmigrations.FS.ReadFile(privateOpenAIGroupRenameMigration)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.Error(t, err)
}

func applyPrivateOpenAIGroupRename(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()

	migrationSQL, err := dbmigrations.FS.ReadFile(privateOpenAIGroupRenameMigration)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
}
