package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountUserQuotaShareUsage stores a user's accounted usage in a quota pool.
type AccountUserQuotaShareUsage struct{ ent.Schema }

func (AccountUserQuotaShareUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "account_user_quota_share_usages"}}
}
func (AccountUserQuotaShareUsage) Fields() []ent.Field {
	return []ent.Field{field.Int64("account_id"), field.Int64("user_id"), field.String("window_kind").MaxLen(20).Validate(validateQuotaShareWindow), field.Time("reset_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}), field.Float("cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0)}
}
func (AccountUserQuotaShareUsage) Edges() []ent.Edge {
	return []ent.Edge{edge.From("account", Account.Type).Ref("user_quota_share_usages").Field("account_id").Unique().Required(), edge.From("user", User.Type).Ref("quota_share_usages").Field("user_id").Unique().Required()}
}
func (AccountUserQuotaShareUsage) Indexes() []ent.Index {
	return []ent.Index{index.Fields("account_id", "user_id", "window_kind", "reset_at").Unique()}
}
