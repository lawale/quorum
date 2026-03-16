#!/bin/sh
set -eu

# Quorum seed script — creates sample requests and approvals via app APIs.
# Assumes setup.sh has already run (tenants, policies, webhooks exist).
# Requests are created through the sample app APIs so they appear in both
# the app dashboards AND Quorum.
#
# Usage: QUORUM_API_URL=http://localhost:8080 sh scripts/seed.sh

API_URL="${QUORUM_API_URL:-http://localhost:8080}"
API="${API_URL}/api/v1"

BANKING_URL="${BANKING_URL:-http://banking:3001}"
EXPENSES_URL="${EXPENSES_URL:-http://expenses:3002}"
ACCESS_PORTAL_URL="${ACCESS_PORTAL_URL:-http://access-portal:3003}"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log()     { printf "${GREEN}[seed]${NC} %s\n" "$1"; }
section() { printf "\n${BLUE}=== %s ===${NC}\n" "$1"; }

# ---- Wait for sample apps ----
section "Waiting for sample apps"

wait_for_app() {
  local url="$1" name="$2" attempts=0
  until curl -sf "${url}/" > /dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 30 ]; then
      printf "  %s not ready after 60s, skipping.\n" "$name"
      return 1
    fi
    printf "  Waiting for %s...\n" "$name"
    sleep 2
  done
  log "$name is ready"
  return 0
}

wait_for_app "$BANKING_URL" "Banking"
wait_for_app "$EXPENSES_URL" "Expenses"
wait_for_app "$ACCESS_PORTAL_URL" "Access Portal"

# ---- Requests (via sample app APIs) ----
section "Creating Requests (via sample apps)"

log "Creating expense via Expenses app (alice)"
EXPENSE_RESP=$(curl -sf -X POST "${EXPENSES_URL}/api/expenses" \
  -H "Content-Type: application/json" \
  -d '{
    "employeeName": "Alice Johnson",
    "amount": 1250.00,
    "category": "Travel",
    "description": "Client visit to NYC - flights and hotel"
  }')
EXPENSE_QID=$(echo "$EXPENSE_RESP" | sed -n 's/.*"quorumRequestId": *"\([^"]*\)".*/\1/p')
log "  -> Quorum ID: ${EXPENSE_QID:-unknown}"

log "Creating wire transfer via Banking app (bob)"
WIRE_RESP=$(curl -sf -X POST "${BANKING_URL}/api/transfers" \
  -H "Content-Type: application/json" \
  -d '{
    "from_user": "bob",
    "source_account": "ACC-001",
    "amount": "50000",
    "destination": "IBAN-DE89370400440532013000"
  }')
WIRE_QID=$(echo "$WIRE_RESP" | sed -n 's/.*"quorum_request_id": *"\([^"]*\)".*/\1/p')
log "  -> Quorum ID: ${WIRE_QID:-unknown}"

log "Creating access request via Access Portal (charlie)"
ACCESS_RESP=$(curl -sf -X POST "${ACCESS_PORTAL_URL}/api/requests" \
  -H "Content-Type: application/json" \
  -d '{
    "requester": "charlie",
    "system_name": "Production Database",
    "access_level": "read-write",
    "justification": "Need to run quarterly data migration"
  }')
ACCESS_QID=$(echo "$ACCESS_RESP" | sed -n 's/.*"quorum_request_id": *"\([^"]*\)".*/\1/p')
log "  -> Quorum ID: ${ACCESS_QID:-unknown}"

log "Creating expense via Expenses app (dave)"
EXPENSE2_RESP=$(curl -sf -X POST "${EXPENSES_URL}/api/expenses" \
  -H "Content-Type: application/json" \
  -d '{
    "employeeName": "Dave Kim",
    "amount": 89.99,
    "category": "Software",
    "description": "Annual IDE license renewal"
  }')
EXPENSE2_QID=$(echo "$EXPENSE2_RESP" | sed -n 's/.*"quorumRequestId": *"\([^"]*\)".*/\1/p')
log "  -> Quorum ID: ${EXPENSE2_QID:-unknown}"

log "Creating account closure via Quorum API (eve, tenant: banking)"
CLOSURE_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: eve" \
  -H "X-Tenant-ID: banking" \
  -d '{
    "type": "account_closure",
    "payload": {
      "account_id": "ACC-4472",
      "customer_name": "Acme Corp",
      "reason": "Company dissolved"
    }
  }' | sed -n 's/.*"id": *"\([^"]*\)".*/\1/p')
log "  -> ${CLOSURE_ID}"

log "Creating wire transfer via Banking app (frank)"
WIRE2_RESP=$(curl -sf -X POST "${BANKING_URL}/api/transfers" \
  -H "Content-Type: application/json" \
  -d '{
    "from_user": "frank",
    "source_account": "ACC-007",
    "amount": "15000",
    "destination": "IBAN-GB29NWBK60161331926819"
  }')
WIRE2_QID=$(echo "$WIRE2_RESP" | sed -n 's/.*"quorum_request_id": *"\([^"]*\)".*/\1/p')
log "  -> Quorum ID: ${WIRE2_QID:-unknown}"

# ---- Approve some requests ----
section "Approving Some Requests"

if [ -n "${EXPENSE_QID:-}" ]; then
  log "Approving expense by alice (single-stage -> approved)"
  curl -sf -X POST "${API}/requests/${EXPENSE_QID}/approve" \
    -H "Content-Type: application/json" \
    -H "X-User-ID: manager-maria" \
    -H "X-User-Roles: manager" \
    -H "X-Tenant-ID: expenses" \
    -d '{"comment": "Approved - within travel budget"}' > /dev/null
fi

if [ -n "${WIRE_QID:-}" ]; then
  log "Approving wire transfer stage 1 (advances to compliance review)"
  curl -sf -X POST "${API}/requests/${WIRE_QID}/approve" \
    -H "Content-Type: application/json" \
    -H "X-User-ID: manager-maria" \
    -H "X-User-Roles: manager" \
    -H "X-Tenant-ID: banking" \
    -d '{"comment": "Manager approved"}' > /dev/null
fi

if [ -n "${EXPENSE2_QID:-}" ]; then
  log "Approving expense by dave (single-stage -> approved)"
  curl -sf -X POST "${API}/requests/${EXPENSE2_QID}/approve" \
    -H "Content-Type: application/json" \
    -H "X-User-ID: finance-fay" \
    -H "X-User-Roles: finance" \
    -H "X-Tenant-ID: expenses" \
    -d '{"comment": "Small amount, auto-approved"}' > /dev/null
fi

# ---- Summary ----
section "Seed Complete"
log "Requests:  6 created via sample app APIs (2 approved, 4 pending)"
log ""
log "Console:   ${API_URL}/console/  (admin / admin123)"
log "Banking:   ${BANKING_URL}"
log "Expenses:  ${EXPENSES_URL}"
log "Portal:    ${ACCESS_PORTAL_URL}"
