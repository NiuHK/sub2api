-- OpenAI OAuth account quota sharing. Percentages are allocation weights (0..100).
CREATE TABLE IF NOT EXISTS account_user_quota_shares (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    five_hour_percent DECIMAL(7,4) NOT NULL DEFAULT -1 CHECK (five_hour_percent = -1 OR (five_hour_percent >= 0 AND five_hour_percent <= 100)),
    seven_day_percent DECIMAL(7,4) NOT NULL DEFAULT -1 CHECK (seven_day_percent = -1 OR (seven_day_percent >= 0 AND seven_day_percent <= 100)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS account_user_quota_shares_active_uq ON account_user_quota_shares(account_id, user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS account_user_quota_shares_account_idx ON account_user_quota_shares(account_id);
CREATE INDEX IF NOT EXISTS account_user_quota_shares_user_idx ON account_user_quota_shares(user_id);

CREATE TABLE IF NOT EXISTS account_quota_share_usages (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_kind VARCHAR(20) NOT NULL CHECK (window_kind IN ('five_hour', 'seven_day')),
    reset_at TIMESTAMPTZ NOT NULL,
    cost DECIMAL(20,10) NOT NULL DEFAULT 0 CHECK (cost >= 0),
    UNIQUE(account_id, window_kind, reset_at)
);

CREATE TABLE IF NOT EXISTS account_user_quota_share_usages (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    window_kind VARCHAR(20) NOT NULL CHECK (window_kind IN ('five_hour', 'seven_day')),
    reset_at TIMESTAMPTZ NOT NULL,
    cost DECIMAL(20,10) NOT NULL DEFAULT 0 CHECK (cost >= 0),
    UNIQUE(account_id, user_id, window_kind, reset_at)
);
CREATE INDEX IF NOT EXISTS account_user_quota_share_usages_user_idx ON account_user_quota_share_usages(user_id);
