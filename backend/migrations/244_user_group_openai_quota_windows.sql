ALTER TABLE user_group_rate_multipliers
    ADD COLUMN IF NOT EXISTS quota_percentage_5h_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS quota_percentage_7d_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN user_group_rate_multipliers.quota_percentage_5h_enabled IS
    'Apply the per-user OpenAI quota percentage to the upstream 5-hour window.';
COMMENT ON COLUMN user_group_rate_multipliers.quota_percentage_7d_enabled IS
    'Apply the per-user OpenAI quota percentage to the upstream 7-day window.';
