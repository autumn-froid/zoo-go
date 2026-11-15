#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_DIR="$ROOT_DIR/deploy"
API_IMAGE="${1:-}"
WEB_IMAGE="${2:-}"

cd "$DEPLOY_DIR"

if [[ ! -f .env ]]; then
  echo "missing $DEPLOY_DIR/.env — copy from .env.example and fill secrets" >&2
  exit 1
fi

random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
}

ensure_secret() {
  local key="$1"
  if grep -Eq "^${key}=.+" .env; then
    return
  fi
  local value
  value="$(random_hex)"
  if grep -Eq "^${key}=" .env; then
    sed -i.bak "s|^${key}=.*|${key}=${value}|" .env
    rm -f .env.bak
  else
    printf '\n# Auto-generated once; back up before rotating.\n%s=%s\n' "$key" "$value" >> .env
  fi
}

ensure_secret PII_ENCRYPTION_KEY
ensure_secret PII_HASH_KEY

mkdir -p \
  "$ROOT_DIR/data/uploads" \
  "$ROOT_DIR/data/openobserve" \
  "$ROOT_DIR/data/caddy/data" \
  "$ROOT_DIR/data/caddy/config" \
  "$ROOT_DIR/secrets"
# Container runs as uid 10001; host-mounted SQLite needs write access.
if [[ "$(id -u)" -eq 0 ]]; then
  chown -R 10001:10001 "$ROOT_DIR/data" || true
fi

upsert_env() {
  local key="$1"
  local value="$2"
  if grep -q "^${key}=" .env; then
    sed -i.bak "s|^${key}=.*|${key}=${value}|" .env
    rm -f .env.bak
  else
    echo "${key}=${value}" >> .env
  fi
}

if [[ -n "$API_IMAGE" ]]; then
  upsert_env ZOO_IMAGE "$API_IMAGE"
fi
if [[ -n "$WEB_IMAGE" ]]; then
  upsert_env WEB_IMAGE "$WEB_IMAGE"
fi

# Registry images should be pulled; local tags like zoo-go:local can skip.
should_pull=false
zoo_image="$(grep -E '^ZOO_IMAGE=' .env | cut -d= -f2- || true)"
web_image="$(grep -E '^WEB_IMAGE=' .env | cut -d= -f2- || true)"
if [[ "$zoo_image" == */* || "$web_image" == */* ]]; then
  should_pull=true
fi
if [[ "$should_pull" == true ]]; then
  docker compose pull
fi
docker compose up -d --remove-orphans
docker compose ps
