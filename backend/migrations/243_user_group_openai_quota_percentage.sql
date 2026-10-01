ALTER TABLE user_group_rate_multipliers
    ADD COLUMN IF NOT EXISTS quota_percentage DOUBLE PRECISION NULL;

ALTER TABLE user_group_rate_multipliers
    DROP CONSTRAINT IF EXISTS user_group_rate_multipliers_quota_percentage_check;

ALTER TABLE user_group_rate_multipliers
    ADD CONSTRAINT user_group_rate_multipliers_quota_percentage_check
    CHECK (quota_percentage IS NULL OR (quota_percentage >= 0 AND quota_percentage <= 100));

COMMENT ON COLUMN user_group_rate_multipliers.quota_percentage IS
    'OpenAI OAuth 分组窗口配额百分比（0-100）；NULL 表示不限制。';
