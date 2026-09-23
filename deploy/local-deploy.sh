#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_DIR="$ROOT_DIR/deploy"
ENV_FILE="$DEPLOY_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  cp "$DEPLOY_DIR/.env.example" "$ENV_FILE"
  postgres_password="$(openssl rand -hex 32)"
  jwt_secret="$(openssl rand -hex 32)"
  totp_key="$(openssl rand -hex 32)"
  admin_password="$(openssl rand -hex 18)"
  export postgres_password jwt_secret totp_key admin_password
  python3 - "$ENV_FILE" <<'PY'
import os
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
values = {
    "POSTGRES_PASSWORD": os.environ["postgres_password"],
    "JWT_SECRET": os.environ["jwt_secret"],
    "TOTP_ENCRYPTION_KEY": os.environ["totp_key"],
    "ADMIN_PASSWORD": os.environ["admin_password"],
    "ADMIN_EMAIL": "admin@pinellia.uk",
}
lines = path.read_text().splitlines()
for i, line in enumerate(lines):
    key, sep, _ = line.partition("=")
    if sep and key in values:
        lines[i] = f"{key}={values[key]}"
path.write_text("\n".join(lines) + "\n")
PY
  chmod 600 "$ENV_FILE"
  printf '\nCreated %s with unique secrets. Initial admin credentials (save these now):\n  Email: admin@pinellia.uk\n  Password: %s\n\n' "$ENV_FILE" "$admin_password"
fi

cd "$ROOT_DIR"
docker compose --env-file "$ENV_FILE" \
  -f deploy/docker-compose.yml \
  -f deploy/docker-compose.source.yml \
  up -d --build
