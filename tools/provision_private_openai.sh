#!/usr/bin/env bash
# Preview by default; --apply requires a verified, externally stored database backup.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
apply=false
backup=''
case "${1:-}" in
  '') ;;
  --apply)
    apply=true
    backup="${2:-}"
    if [[ -z "$backup" || ! -f "$backup" || ! -s "$backup" ]]; then
      echo 'Usage: provision_private_openai.sh --apply /absolute/path/to/pre-change.dump' >&2
      exit 2
    fi
    if [[ "$(realpath "$backup")" == "$repo_root/"* ]]; then
      echo 'Backup must be outside the repository' >&2
      exit 2
    fi
    docker compose --env-file "$repo_root/deploy/.env" -f "$repo_root/deploy/docker-compose.yml" exec -T postgres \
      sh -c 'exec pg_restore -l' < "$backup" > /dev/null
    ;;
  *) echo 'Usage: provision_private_openai.sh [--apply /absolute/path/to/pre-change.dump]' >&2; exit 2 ;;
esac
if [[ $# -gt 0 && "$apply" == false || $# -gt 2 ]]; then
  echo 'Unexpected arguments' >&2
  exit 2
fi
docker compose --env-file "$repo_root/deploy/.env" -f "$repo_root/deploy/docker-compose.yml" exec -T postgres \
  sh -c 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -v "apply=$1"' sh "$apply" \
  < "$repo_root/tools/provision_private_openai.sql"
