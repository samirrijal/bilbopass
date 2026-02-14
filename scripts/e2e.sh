#!/usr/bin/env bash
# scripts/e2e.sh — End-to-end smoke tests for the BilboPass API.
# Expects the API to be running on localhost:8080.
set -euo pipefail

BASE="http://localhost:8080"
PASS=0
FAIL=0
TOTAL=0

green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }

assert_status() {
    local label="$1" url="$2" expected="$3"
    TOTAL=$((TOTAL + 1))
    status=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    if [ "$status" = "$expected" ]; then
        green "  ✓ $label (HTTP $status)"
        PASS=$((PASS + 1))
    else
        red "  ✗ $label — expected $expected, got $status"
        FAIL=$((FAIL + 1))
    fi
}

assert_json_field() {
    local label="$1" url="$2" field="$3"
    TOTAL=$((TOTAL + 1))
    body=$(curl -s "$url")
    if echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); assert '$field' in d" 2>/dev/null; then
        green "  ✓ $label (has '$field')"
        PASS=$((PASS + 1))
    else
        red "  ✗ $label — missing field '$field'"
        FAIL=$((FAIL + 1))
    fi
}

assert_json_array_nonempty() {
    local label="$1" url="$2"
    TOTAL=$((TOTAL + 1))
    body=$(curl -s "$url")
    count=$(echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else len(d.get('data',[])))" 2>/dev/null || echo "0")
    if [ "$count" -gt 0 ] 2>/dev/null; then
        green "  ✓ $label ($count items)"
        PASS=$((PASS + 1))
    else
        red "  ✗ $label — expected non-empty array"
        FAIL=$((FAIL + 1))
    fi
}

assert_header() {
    local label="$1" url="$2" header="$3"
    TOTAL=$((TOTAL + 1))
    val=$(curl -s -I "$url" | grep -i "^$header:" | head -1)
    if [ -n "$val" ]; then
        green "  ✓ $label ($val)"
        PASS=$((PASS + 1))
    else
        red "  ✗ $label — header '$header' missing"
        FAIL=$((FAIL + 1))
    fi
}

# ──────────────────────────────────────────────
bold ""
bold "BilboPass E2E Smoke Tests"
bold "========================="
bold ""

# System
bold "[System]"
assert_status      "Health endpoint"          "$BASE/v1/health"  200
assert_json_field  "Health has status"        "$BASE/v1/health"  "status"
assert_status      "Ready endpoint"           "$BASE/v1/ready"   200
assert_status      "Metrics endpoint"         "$BASE/metrics"    200
assert_status      "Swagger UI"               "$BASE/docs"       200
assert_status      "OpenAPI spec"             "$BASE/docs/openapi.yaml" 200

# Security headers
bold ""
bold "[Security Headers]"
assert_header "X-Content-Type-Options" "$BASE/v1/health" "X-Content-Type-Options"
assert_header "X-Frame-Options"        "$BASE/v1/health" "X-Frame-Options"

# Agencies
bold ""
bold "[Agencies]"
assert_status              "List agencies"              "$BASE/v1/agencies"          200
assert_json_array_nonempty "Agencies non-empty"         "$BASE/v1/agencies"
assert_json_field          "Agencies paginated"         "$BASE/v1/agencies"          "pagination"
assert_status              "Agencies with pagination"   "$BASE/v1/agencies?offset=0&limit=3" 200

# Stops
bold ""
bold "[Stops]"
assert_status  "Nearby stops (Bilbao center)"  "$BASE/v1/stops/nearby?lat=43.263&lon=-2.935&radius=1000" 200
assert_json_array_nonempty "Nearby returns stops"       "$BASE/v1/stops/nearby?lat=43.263&lon=-2.935&radius=1000"
assert_status  "Search stops"                  "$BASE/v1/stops/search?q=abando"   200
assert_status  "Nearby missing params"         "$BASE/v1/stops/nearby"            400
assert_status  "Search missing query"          "$BASE/v1/stops/search"            400
assert_status  "Bad radius"                    "$BASE/v1/stops/nearby?lat=43.26&lon=-2.93&radius=50000"  400

# Routes
bold ""
bold "[Routes]"
assert_status  "Routes missing agency_id"  "$BASE/v1/routes"  400

# GraphQL
bold ""
bold "[GraphQL]"
TOTAL=$((TOTAL + 1))
gql_resp=$(curl -s -X POST "$BASE/graphql" \
    -H "Content-Type: application/json" \
    -d '{"query":"{ agencies { id slug name } }"}')
gql_count=$(echo "$gql_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('data',{}).get('agencies',[])))" 2>/dev/null || echo "0")
if [ "$gql_count" -gt 0 ] 2>/dev/null; then
    green "  ✓ GraphQL agencies query ($gql_count results)"
    PASS=$((PASS + 1))
else
    red "  ✗ GraphQL agencies query — no results"
    FAIL=$((FAIL + 1))
fi

# Error format
bold ""
bold "[Error Format]"
TOTAL=$((TOTAL + 1))
err_resp=$(curl -s "$BASE/v1/stops/nearby")
has_err=$(echo "$err_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('error')=='bad_request'; print('ok')" 2>/dev/null || echo "")
if [ "$has_err" = "ok" ]; then
    green "  ✓ Structured error response"
    PASS=$((PASS + 1))
else
    red "  ✗ Expected structured API error"
    FAIL=$((FAIL + 1))
fi

# ──────────────────────────────────────────────
bold ""
bold "Results: $PASS/$TOTAL passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
    red "SOME TESTS FAILED"
    exit 1
else
    green "ALL TESTS PASSED"
fi
