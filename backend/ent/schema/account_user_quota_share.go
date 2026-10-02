package schema

import (
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// AccountUserQuotaShare assigns a user's share of an OpenAI OAuth account quota.
type AccountUserQuotaShare struct{ ent.Schema }

func (AccountUserQuotaShare) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "account_user_quota_shares"}}
}
func (AccountUserQuotaShare) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}
func (AccountUserQuotaShare) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"), field.Int64("user_id"),
		field.Float("five_hour_percent").SchemaType(map[string]string{dialect.Postgres: "decimal(7,4)"}).Default(-1),
		field.Float("seven_day_percent").SchemaType(map[string]string{dialect.Postgres: "decimal(7,4)"}).Default(-1),
	}
}
func (AccountUserQuotaShare) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("account", Account.Type).Ref("user_quota_shares").Field("account_id").Unique().Required(),
		edge.From("user", User.Type).Ref("account_quota_shares").Field("user_id").Unique().Required(),
	}
}
func (AccountUserQuotaShare) Indexes() []ent.Index {
	return []ent.Index{index.Fields("account_id", "user_id").Unique().Annotations(entsql.IndexWhere("deleted_at IS NULL")), index.Fields("account_id"), index.Fields("user_id")}
}

func validateQuotaShareWindow(s string) error {
	if s != "five_hour" && s != "seven_day" {
		return fmt.Errorf("window kind %q is not allowed", s)
	}
	return nil
}
