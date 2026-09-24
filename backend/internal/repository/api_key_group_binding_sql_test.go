package repository

import (
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/stretchr/testify/require"
)

// Group deletion calls this predicate even when no API key is bound. In Postgres,
// ExprP("?::jsonb") emits a literal ?, rather than a numbered placeholder.
func TestEnabledBindingGroupPredicatePostgresSQL(t *testing.T) {
	s := entsql.Dialect(dialect.Postgres).Select(apikey.FieldKey).From(entsql.Table(apikey.Table))
	enabledBindingGroupPredicate(42)(s)
	query, args := s.Query()
	require.Contains(t, query, `"group_bindings" @> $1::jsonb`)
	require.NotContains(t, query, "?::jsonb")
	require.Equal(t, []any{`[{"group_id":42}]`}, args)
	// The predicate is also embedded in the OR used by ListKeysByGroupID.
	s = entsql.Dialect(dialect.Postgres).Select(apikey.FieldKey).From(entsql.Table(apikey.Table))
	apikey.Or(apikey.GroupIDEQ(42), enabledBindingGroupPredicate(42))(s)
	query, args = s.Query()
	require.True(t, strings.Contains(query, "$2::jsonb"), query)
	require.Len(t, args, 2)
}
