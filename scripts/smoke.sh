#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE_URL:-http://localhost:3000/api/v1}"
EMAIL="${SUPER_ADMIN_EMAIL:-admin@app.local}"
PASSWORD="${SUPER_ADMIN_PASSWORD:-ChangeMe123!}"
COOKIE_JAR="/tmp/smoke-cookies-$$.txt"
trap "rm -f $COOKIE_JAR" EXIT

fail() { echo "❌ $1"; exit 1; }
ok() { echo "✅ $1"; }

http() {
  curl -sf -w "\n%{http_code}" -b "$COOKIE_JAR" -c "$COOKIE_JAR" "$@"
}

resp=$(http "$API_BASE/health") || fail "Health unreachable"
[[ $(echo "$resp" | tail -1) == "200" ]] || fail "Health status $(echo "$resp" | tail -1)"
ok "Health"

resp=$(http -H "content-type: application/json" -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" "$API_BASE/auth/login") || fail "Login failed"
[[ $(echo "$resp" | tail -1) == "201" ]] || fail "Login status $(echo "$resp" | tail -1)"
TOKEN=$(echo "$resp" | head -1 | python3 -c "import sys,json;print(json.load(sys.stdin)[\"data\"].get(\"accessToken\",\"\"))")
[[ -n "$TOKEN" ]] || fail "No access token"
ok "Login"

resp=$(http -H "authorization: Bearer $TOKEN" "$API_BASE/auth/me") || fail "/auth/me failed"
[[ $(echo "$resp" | tail -1) == "200" ]] || fail "/auth/me status $(echo "$resp" | tail -1)"
ok "/auth/me"

resp=$(http -X POST "$API_BASE/auth/refresh") || fail "Refresh failed"
[[ $(echo "$resp" | tail -1) == "200" ]] || fail "Refresh status $(echo "$resp" | tail -1)"
TOKEN2=$(echo "$resp" | head -1 | python3 -c "import sys,json;print(json.load(sys.stdin)[\"data\"].get(\"accessToken\",\"\"))")
[[ -n "$TOKEN2" && "$TOKEN2" != "undefined" ]] || fail "Refresh returned no token"
ok "Refresh"

resp=$(http -H "authorization: Bearer $TOKEN2" "$API_BASE/users?page=1&limit=5") || fail "List users failed"
[[ $(echo "$resp" | tail -1) == "200" ]] || fail "List users status $(echo "$resp" | tail -1)"
ok "List users"

resp=$(http -H "authorization: Bearer $TOKEN2" "$API_BASE/permissions") || fail "Permissions catalog failed"
[[ $(echo "$resp" | tail -1) == "200" ]] || fail "Permissions catalog status $(echo "$resp" | tail -1)"
ok "Permissions catalog"

resp=$(http -H "authorization: Bearer $TOKEN2" "$API_BASE/roles?page=1&limit=5") || fail "List roles failed"
[[ $(echo "$resp" | tail -1) == "200" ]] || fail "List roles status $(echo "$resp" | tail -1)"
ok "List roles"

echo "✅ All smoke checks passed"
