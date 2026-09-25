-- Run with psql -v ON_ERROR_STOP=1 -v apply=false|true -f this_file.
-- Provisions private group/subscription resources only; it never adds accounts.
SELECT set_config('sub2api.private_target_user_id', :'target_user_id', false);
WITH platforms(platform) AS (VALUES ('openai'), ('anthropic'), ('gemini'), ('grok'), ('antigravity'), ('kimi'), ('zhipu'), ('deepseek'), ('minimax'), ('opencode_go'))
SELECT count(*) AS private_resources_to_create
FROM users u CROSS JOIN platforms p
WHERE u.role = 'user' AND u.deleted_at IS NULL
  AND u.id = COALESCE(NULLIF(current_setting('sub2api.private_target_user_id'), 'all')::bigint, u.id)
  AND NOT EXISTS (SELECT 1 FROM groups g WHERE g.name = CASE WHEN p.platform = 'openai' THEN 'private-usr' || u.id ELSE 'private-usr' || u.id || '-' || p.platform END AND g.deleted_at IS NULL);
WITH platforms(platform) AS (VALUES ('openai'), ('anthropic'), ('gemini'), ('grok'), ('antigravity'), ('kimi'), ('zhipu'), ('deepseek'), ('minimax'), ('opencode_go')),
targets AS (
 SELECT u.id AS user_id, p.platform,
 'Managed private ' || CASE WHEN p.platform = 'openai' THEN 'OpenAI' ELSE p.platform END || ' group for user ' || u.id || ' (' || CASE WHEN p.platform = 'openai' THEN 'private-subscription-v1' ELSE 'private-subscription-v2' END || ')' AS description,
 'Managed private ' || CASE WHEN p.platform = 'openai' THEN 'OpenAI' ELSE p.platform END || ' subscription for user ' || u.id || ' (' || CASE WHEN p.platform = 'openai' THEN 'private-subscription-v1' ELSE 'private-subscription-v2' END || ')' AS note,
 g.id AS group_id, g.description AS group_description, g.platform AS group_platform,
 g.subscription_type, g.is_exclusive, g.status AS group_status
 FROM users u CROSS JOIN platforms p LEFT JOIN groups g ON g.name = CASE WHEN p.platform = 'openai' THEN 'private-usr' || u.id ELSE 'private-usr' || u.id || '-' || p.platform END AND g.deleted_at IS NULL
 WHERE u.role = 'user' AND u.deleted_at IS NULL AND u.id = COALESCE(NULLIF(current_setting('sub2api.private_target_user_id'), 'all')::bigint, u.id)
)
SELECT count(*) FILTER (WHERE t.group_id IS NOT NULL AND (t.group_description IS DISTINCT FROM t.description OR t.group_platform <> t.platform OR t.subscription_type <> 'subscription' OR NOT t.is_exclusive OR t.group_status <> 'active')) AS conflicting_groups,
 count(*) FILTER (WHERE t.group_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM user_subscriptions s WHERE s.user_id = t.user_id AND s.group_id = t.group_id)) AS subscriptions_to_create,
 count(*) FILTER (WHERE t.group_id IS NOT NULL AND EXISTS (SELECT 1 FROM user_subscriptions s WHERE s.group_id = t.group_id AND s.user_id <> t.user_id)) AS foreign_subscribers,
 count(*) FILTER (WHERE t.group_id IS NOT NULL AND EXISTS (SELECT 1 FROM user_subscriptions s WHERE s.group_id = t.group_id AND s.user_id = t.user_id AND (s.notes IS DISTINCT FROM t.note OR s.status <> 'active' OR s.expires_at <= now()))) AS invalid_subscriptions
FROM targets t;

\if :apply
BEGIN;
DO $provision$
DECLARE
    u RECORD;
    p text;
    g RECORD;
    s RECORD;
    group_name text;
    group_description text;
    sub_note text;
    marker text;
    started_at timestamptz := clock_timestamp();
BEGIN
    PERFORM pg_advisory_xact_lock(719023, 1000);
    IF current_setting('sub2api.private_target_user_id') <> 'all' THEN
      IF NOT EXISTS (
          SELECT 1 FROM users WHERE id = current_setting('sub2api.private_target_user_id')::bigint
            AND role = 'user' AND deleted_at IS NULL
      ) THEN RAISE EXCEPTION 'Requested user does not exist or is not eligible'; END IF;
    END IF;
    FOR u IN SELECT id FROM users WHERE role = 'user' AND deleted_at IS NULL
             AND id = COALESCE(NULLIF(current_setting('sub2api.private_target_user_id'), 'all')::bigint, id) ORDER BY id LOOP
      FOREACH p IN ARRAY ARRAY['openai','anthropic','gemini','grok','antigravity','kimi','zhipu','deepseek','minimax','opencode_go'] LOOP
        group_name := CASE WHEN p = 'openai' THEN 'private-usr' || u.id ELSE 'private-usr' || u.id || '-' || p END;
        marker := CASE WHEN p = 'openai' THEN 'private-subscription-v1' ELSE 'private-subscription-v2' END;
        group_description := 'Managed private ' || CASE WHEN p = 'openai' THEN 'OpenAI' ELSE p END || ' group for user ' || u.id || ' (' || marker || ')';
        sub_note := 'Managed private ' || CASE WHEN p = 'openai' THEN 'OpenAI' ELSE p END || ' subscription for user ' || u.id || ' (' || marker || ')';
        SELECT * INTO g FROM groups WHERE name = group_name AND deleted_at IS NULL;
        IF NOT FOUND THEN
          INSERT INTO groups (name, description, platform, subscription_type, is_exclusive, status, default_validity_days)
          VALUES (group_name, group_description, p, 'subscription', true, 'active', 1000) RETURNING * INTO g;
        ELSIF g.description IS DISTINCT FROM group_description OR g.platform <> p OR g.subscription_type <> 'subscription' OR NOT g.is_exclusive OR g.status <> 'active' THEN
          RAISE EXCEPTION 'Refusing to claim conflicting group % (id %)', group_name, g.id;
        END IF;
        IF EXISTS (SELECT 1 FROM user_subscriptions WHERE group_id = g.id AND user_id <> u.id) THEN
          RAISE EXCEPTION 'Private group % has subscriptions for another user', group_name;
        END IF;
        SELECT * INTO s FROM user_subscriptions WHERE user_id = u.id AND group_id = g.id;
        IF NOT FOUND THEN
          INSERT INTO user_subscriptions (user_id, group_id, starts_at, expires_at, status, notes)
          VALUES (u.id, g.id, started_at, started_at + interval '1000 days', 'active', sub_note);
        ELSIF s.notes IS DISTINCT FROM sub_note OR s.status <> 'active' OR s.expires_at <= started_at THEN
          RAISE EXCEPTION 'Refusing to claim invalid managed subscription for user %, group %', u.id, g.id;
        END IF;
      END LOOP;
    END LOOP;
END
$provision$;
COMMIT;
\endif
