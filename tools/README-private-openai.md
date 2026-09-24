# Private OpenAI group/subscription provisioner

This is an external, idempotent provisioner for **existing non-deleted, role=user** rows.
For each user ID N, it creates the exclusive OpenAI subscription group `private-usrN`
and an active `user_subscriptions` row for that user and group, valid for 1000 days.
The group has no daily/weekly/monthly dollar cap (NULL); this does **not** add
upstream accounts. No new authorization mechanism is introduced: API Keys still
require a real active subscription for the selected group.

From the repository root, with the existing deploy stack available:

```sh
tools/provision_private_openai.sh                       # read-only preview
# Take a fresh off-repository database backup and verify it before writing.
# Example (restrict backup directory to owner; do not commit or publish it):
backup_dir="$(mktemp -d "$HOME/sub2api-private-backup-XXXXXXXX")"
chmod 700 "$backup_dir"
docker compose --env-file deploy/.env -f deploy/docker-compose.yml exec -T postgres \
  sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "$backup_dir/pre-private-subscription.dump"
chmod 600 "$backup_dir/pre-private-subscription.dump"
tools/provision_private_openai.sh --apply "$backup_dir/pre-private-subscription.dump"
```

`--apply` verifies the backup format with `pg_restore -l`, then runs one database
transaction; any conflict rolls the whole batch back. Existing unmanaged same-name
groups/subscriptions are refused, and existing managed subscriptions are not
extended or reactivated. Preview reports potential name collisions and invalid
subscriptions; resolve those before applying. The backup is for DBA-led recovery;
**do not** restore it over a live database without reviewing subsequent changes.

This tool is **not a registration hook**. Run it again (or schedule it with a
trusted operator) to provision newly registered users. Do not expose it as a
user-controlled endpoint. To verify the result, check that an eligible user's
`private-usrN` group appears under `/groups/available`, and that another user
without a subscription cannot bind an API Key to that group. No live request can
use the group until an upstream account is assigned separately.
