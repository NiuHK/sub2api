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

// AccountQuotaShareUsage stores the aggregate usage of an account quota pool.
type AccountQuotaShareUsage struct{ ent.Schema }

func (AccountQuotaShareUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "account_quota_share_usages"}}
}
func (AccountQuotaShareUsage) Fields() []ent.Field {
	return []ent.Field{field.Int64("account_id"), field.String("window_kind").MaxLen(20).Validate(validateQuotaShareWindow), field.Time("reset_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}), field.Float("cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0)}
}
func (AccountQuotaShareUsage) Edges() []ent.Edge {
	return []ent.Edge{edge.From("account", Account.Type).Ref("quota_share_usages").Field("account_id").Unique().Required()}
}
func (AccountQuotaShareUsage) Indexes() []ent.Index {
	return []ent.Index{index.Fields("account_id", "window_kind", "reset_at").Unique()}
}
