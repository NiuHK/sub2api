-- Opt-in bindings are independent of the legacy group_id foreign key.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS group_bindings_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS group_bindings JSONB NOT NULL DEFAULT '[]'::jsonb;
CREATE INDEX IF NOT EXISTS idx_api_keys_enabled_group_bindings ON api_keys USING GIN (group_bindings jsonb_path_ops) WHERE group_bindings_enabled = TRUE AND deleted_at IS NULL;
