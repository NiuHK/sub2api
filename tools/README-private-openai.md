# Private per-platform group/subscription provisioner

This is an external, idempotent repair/backfill tool for **existing non-deleted, role=user** rows.
Use `--user-id N` to limit it to one user; omit it for a full backfill.
For each selected user ID N, it creates exclusive private groups and active
`user_subscriptions` rows for supported upstream account platforms, valid for
1000 days. OpenAI keeps the legacy group name `private-usrN`; other platforms use
`private-usrN-<platform>`. Groups have no daily/weekly/monthly dollar cap (NULL);
this does **not** add upstream accounts. No new authorization mechanism is introduced: API Keys still
require a real active subscription for the selected group.

From the repository root, with the existing deploy stack available:

```sh
tools/provision_private_openai.sh --user-id 2           # read-only preview for one user
tools/provision_private_openai.sh                       # read-only full preview
# Take a fresh off-repository database backup and verify it before writing.
# Example (restrict backup directory to owner; do not commit or publish it):
backup_dir="$(mktemp -d "$HOME/sub2api-private-backup-XXXXXXXX")"
chmod 700 "$backup_dir"
docker compose --env-file deploy/.env -f deploy/docker-compose.yml exec -T postgres \
  sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "$backup_dir/pre-private-subscription.dump"
chmod 600 "$backup_dir/pre-private-subscription.dump"
tools/provision_private_openai.sh --user-id 2 --apply "$backup_dir/pre-private-subscription.dump"
```

`--apply` verifies the backup format with `pg_restore -l`, then runs one database
transaction; any conflict rolls the whole batch back. Existing unmanaged same-name
groups/subscriptions are refused, and existing managed subscriptions are not
extended or reactivated. Preview reports potential name collisions and invalid
subscriptions; resolve those before applying. The backup is for DBA-led recovery;
**do not** restore it over a live database without reviewing subsequent changes.

New email/OAuth registrations and admin-created ordinary users call the private
per-platform provisioning routine after creation, with the same
identity/marker/1000-day semantics. Admin-created admins are excluded.
Provisioning failures are logged without invalidating successful creation;
use this tool with `--user-id` to repair the user after addressing the error.
Do not expose the tool as a user-controlled endpoint. To verify the result,
check that an eligible user's private groups (`private-usrN` and
`private-usrN-<platform>`) appear under
`/groups/available`, and that another user
without a subscription cannot bind an API Key to that group. No live request can
use the group until an upstream account is assigned separately.
