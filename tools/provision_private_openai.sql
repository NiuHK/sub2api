-- Run with psql -v ON_ERROR_STOP=1 -v apply=false|true -f this_file.
-- This provisions resources only. API Key authorization continues to use the real
-- user_subscriptions(user_id, group_id) row; the name is never an auth bypass.
SELECT count(*) AS eligible_users FROM users
WHERE role = 'user' AND deleted_at IS NULL;
SELECT count(*) AS groups_to_create FROM users u
WHERE u.role = 'user' AND u.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM groups g WHERE g.name = 'private-usr' || u.id AND g.deleted_at IS NULL);
SELECT count(*) AS subscriptions_to_create FROM users u
JOIN groups g ON g.name = 'private-usr' || u.id AND g.deleted_at IS NULL
WHERE u.role = 'user' AND u.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM user_subscriptions s WHERE s.user_id = u.id AND s.group_id = g.id);
SELECT count(*) AS conflicting_names FROM users u
JOIN groups g ON g.name = 'private-usr' || u.id AND g.deleted_at IS NULL
WHERE u.role = 'user' AND u.deleted_at IS NULL
  AND (g.description IS DISTINCT FROM 'Managed private OpenAI group for user ' || u.id || ' (private-subscription-v1)'
       OR g.platform <> 'openai' OR g.subscription_type <> 'subscription' OR NOT g.is_exclusive
       OR g.status <> 'active');
SELECT count(*) AS invalid_managed_subscriptions FROM users u
JOIN groups g ON g.name = 'private-usr' || u.id AND g.deleted_at IS NULL
JOIN user_subscriptions s ON s.user_id = u.id AND s.group_id = g.id
WHERE u.role = 'user' AND u.deleted_at IS NULL
  AND (s.notes IS DISTINCT FROM 'Managed private OpenAI subscription for user ' || u.id || ' (private-subscription-v1)'
       OR s.status <> 'active' OR s.expires_at <= now());
SELECT count(*) AS subscriptions_for_other_users FROM users u
JOIN groups g ON g.name = 'private-usr' || u.id AND g.deleted_at IS NULL
JOIN user_subscriptions s ON s.group_id = g.id AND s.user_id <> u.id
WHERE u.role = 'user' AND u.deleted_at IS NULL;

\if :apply
BEGIN;
DO $provision$
DECLARE
    u RECORD;
    g RECORD;
    s RECORD;
    group_name text;
    group_description text;
    sub_note text;
    started_at timestamptz := clock_timestamp();
BEGIN
    -- Serialize this script with itself; unique indexes handle outside writers.
    PERFORM pg_advisory_xact_lock(719023, 1000);
    FOR u IN SELECT id FROM users WHERE role = 'user' AND deleted_at IS NULL ORDER BY id LOOP
        group_name := 'private-usr' || u.id;
        group_description := 'Managed private OpenAI group for user ' || u.id || ' (private-subscription-v1)';
        sub_note := 'Managed private OpenAI subscription for user ' || u.id || ' (private-subscription-v1)';
        SELECT * INTO g FROM groups WHERE name = group_name AND deleted_at IS NULL;
        IF NOT FOUND THEN
            INSERT INTO groups (name, description, platform, subscription_type,
                                is_exclusive, status, default_validity_days)
            VALUES (group_name, group_description, 'openai', 'subscription', true, 'active', 1000)
            RETURNING * INTO g;
        ELSIF g.description IS DISTINCT FROM group_description OR g.platform <> 'openai'
           OR g.subscription_type <> 'subscription' OR NOT g.is_exclusive OR g.status <> 'active' THEN
            RAISE EXCEPTION 'Refusing to claim conflicting group % (id %)', group_name, g.id;
        END IF;

        IF EXISTS (SELECT 1 FROM user_subscriptions WHERE group_id = g.id AND user_id <> u.id) THEN
            RAISE EXCEPTION 'Private group % has subscriptions for another user', group_name;
        END IF;
        SELECT * INTO s FROM user_subscriptions WHERE user_id = u.id AND group_id = g.id;
        IF NOT FOUND THEN
            INSERT INTO user_subscriptions (user_id, group_id, starts_at, expires_at, status, notes)
            VALUES (u.id, g.id, started_at, started_at + interval '1000 days', 'active', sub_note);
        ELSIF s.notes IS DISTINCT FROM sub_note THEN
            RAISE EXCEPTION 'Refusing to claim existing subscription for user %, group %', u.id, g.id;
        ELSIF s.status <> 'active' OR s.expires_at <= started_at THEN
            RAISE EXCEPTION 'Managed subscription for user %, group % is inactive or expired', u.id, g.id;
        END IF;
        -- Existing managed subscriptions are deliberately NOT renewed or reactivated.
    END LOOP;
END
$provision$;
COMMIT;
\endif
