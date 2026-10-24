#!/usr/bin/env bash
# Browser e2e: builds the API and admin, seeds a throwaway user into the test
# database, starts the API, then runs Playwright against the admin preview.
# Needs: docker compose up -d postgres_test   (from the repo root)
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
port="${E2E_API_PORT:-3010}"
export SUPER_ADMIN_EMAIL="${SUPER_ADMIN_EMAIL:-e2e@app.local}"
export SUPER_ADMIN_PASSWORD="${SUPER_ADMIN_PASSWORD:-E2eSecret123!}"
export API_PROXY_TARGET="http://localhost:$port"

cd "$root/be"
export DATABASE_URL="${DATABASE_URL:-postgres://app:app@localhost:5436/app_test?sslmode=disable}"
export JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-$(printf 's%.0s' {1..32})}"
export REFRESH_TOKEN_PEPPER="${REFRESH_TOKEN_PEPPER:-$(printf 'p%.0s' {1..16})}"
export WEB_URL="http://localhost:4173" LOG_LEVEL=silent APP_PORT="$port"
export THROTTLE_LIMIT=100000 LOGIN_THROTTLE_LIMIT=100000
go build -o bin/api ./cmd/api
./bin/api migrate >/dev/null
./bin/api seed >/dev/null

./bin/api & api=$!
trap 'kill $api 2>/dev/null || true' EXIT
for _ in $(seq 30); do curl -sf "http://localhost:$port/api/v1/health" >/dev/null && break; sleep 1; done

cd "$root/fe"
pnpm --filter admin exec vite build >/dev/null
pnpm --filter e2e exec playwright test "$@"
