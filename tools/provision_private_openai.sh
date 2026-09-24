#!/usr/bin/env bash
# Preview by default; --apply requires a verified, externally stored database backup.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
apply=false
backup=''
user_id=all
while (( $# )); do
  case "$1" in
    --user-id)
      if (( $# < 2 )) || [[ ! "$2" =~ ^[1-9][0-9]*$ ]]; then
        echo 'Expected a positive numeric --user-id' >&2; exit 2
      fi
      user_id="$2"; shift 2 ;;
    --apply)
      if (( $# < 2 )); then echo 'Expected a backup path after --apply' >&2; exit 2; fi
      apply=true; backup="$2"; shift 2 ;;
    *) echo 'Usage: provision_private_openai.sh [--user-id ID] [--apply /absolute/path/to/pre-change.dump]' >&2; exit 2 ;;
  esac
done
if [[ "$apply" == true ]]; then
  if [[ ! -f "$backup" || ! -s "$backup" || "$(realpath "$backup")" == "$repo_root/"* ]]; then
    echo 'Apply requires a nonempty backup outside the repository' >&2; exit 2
  fi
  docker compose --env-file "$repo_root/deploy/.env" -f "$repo_root/deploy/docker-compose.yml" exec -T postgres \
    sh -c 'exec pg_restore -l' < "$backup" > /dev/null
fi
docker compose --env-file "$repo_root/deploy/.env" -f "$repo_root/deploy/docker-compose.yml" exec -T postgres \
  sh -c 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -v "apply=$1" -v "target_user_id=$2"' sh "$apply" "$user_id" \
  < "$repo_root/tools/provision_private_openai.sql"
