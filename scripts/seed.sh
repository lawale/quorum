#!/bin/sh
set -eu

# Quorum seed script — creates tenants, policies, requests, and approvals.
# Requests are created through the sample app APIs so they appear in both
# the app dashboards AND Quorum.
#
# Usage: QUORUM_API_URL=http://localhost:8080 sh scripts/seed.sh

API_URL="${QUORUM_API_URL:-http://localhost:8080}"
CONSOLE_API="${API_URL}/api/v1/console"
API="${API_URL}/api/v1"

BANKING_URL="${BANKING_URL:-http://banking:3001}"
EXPENSES_URL="${EXPENSES_URL:-http://expenses:3002}"
ACCESS_PORTAL_URL="${ACCESS_PORTAL_URL:-http://access-portal:3003}"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log()     { printf "${GREEN}[seed]${NC} %s\n" "$1"; }
section() { printf "\n${BLUE}=== %s ===${NC}\n" "$1"; }

# ---- Wait for server ----
section "Waiting for server"
attempts=0
until curl -sf "${API_URL}/health" > /dev/null 2>&1; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 30 ]; then
    printf "  Server not ready after 60s, giving up.\n"
    exit 1
  fi
  printf "  Waiting for %s/health ...\n" "$API_URL"
  sleep 2
done
log "Server is ready"

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

# ---- Console admin setup ----
section "Console Admin Setup"
NEEDS_SETUP=$(curl -sf "${CONSOLE_API}/auth/status" | sed -n 's/.*"needs_setup": *\([a-z]*\).*/\1/p')
if [ "$NEEDS_SETUP" = "true" ]; then
  SETUP_RESP=$(curl -sf -X POST "${CONSOLE_API}/auth/setup" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123","display_name":"Demo Admin"}')
  JWT=$(echo "$SETUP_RESP" | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p')
  log "Created admin operator (admin / admin123)"
else
  log "Admin already exists, logging in"
  LOGIN_RESP=$(curl -sf -X POST "${CONSOLE_API}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}')
  JWT=$(echo "$LOGIN_RESP" | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p')
fi

if [ -z "${JWT:-}" ]; then
  log "WARNING: Could not obtain JWT token, tenant creation may fail"
fi

# ---- Tenants ----
section "Creating Tenants"

log "Creating tenant: banking"
curl -sf -X POST "${CONSOLE_API}/tenants" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${JWT}" \
  -d '{"slug":"banking","name":"Banking App"}' > /dev/null 2>&1 || log "  (already exists or error)"

log "Creating tenant: expenses"
curl -sf -X POST "${CONSOLE_API}/tenants" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${JWT}" \
  -d '{"slug":"expenses","name":"Expense Tracker"}' > /dev/null 2>&1 || log "  (already exists or error)"

log "Creating tenant: access-portal"
curl -sf -X POST "${CONSOLE_API}/tenants" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${JWT}" \
  -d '{"slug":"access-portal","name":"Access Request Portal"}' > /dev/null 2>&1 || log "  (already exists or error)"

# ---- Policies ----
section "Creating Policies"

log "Creating policy: Expense Approval (tenant: expenses)"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: expenses" \
  -d '{
    "name": "Expense Approval",
    "request_type": "expense_approval",
    "stages": [{
      "index": 0,
      "name": "Manager Review",
      "required_approvals": 1,
      "allowed_checker_roles": ["manager", "finance"],
      "rejection_policy": "any"
    }],
    "auto_expire_duration": "48h",
    "display_template": {
      "title": "Expense - {{amount | currency}}",
      "fields": [
        {"label": "Employee", "path": "employee_name"},
        {"label": "Amount", "path": "amount", "format": "currency"},
        {"label": "Category", "path": "category"},
        {"label": "Description", "path": "description"}
      ]
    }
  }' > /dev/null 2>&1 || log "  (already exists)"

log "Creating policy: Wire Transfer (tenant: banking)"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: banking" \
  -d '{
    "name": "Wire Transfer Approval",
    "request_type": "wire_transfer",
    "stages": [
      {
        "index": 0,
        "name": "Manager Approval",
        "required_approvals": 1,
        "allowed_checker_roles": ["manager", "admin"],
        "rejection_policy": "any"
      },
      {
        "index": 1,
        "name": "Compliance Review",
        "required_approvals": 2,
        "allowed_checker_roles": ["compliance_officer", "admin"],
        "rejection_policy": "any"
      }
    ],
    "identity_fields": ["source_account_id"],
    "auto_expire_duration": "24h",
    "display_template": {
      "title": "Wire Transfer - {{amount | currency}}",
      "fields": [
        {"label": "From Account", "path": "source_account_id"},
        {"label": "Amount", "path": "amount", "format": "currency"},
        {"label": "Destination", "path": "destination"}
      ]
    }
  }' > /dev/null 2>&1 || log "  (already exists)"

log "Creating policy: System Access Request (tenant: access-portal)"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: access-portal" \
  -d '{
    "name": "System Access Request",
    "request_type": "access_request",
    "stages": [{
      "index": 0,
      "name": "Security Review",
      "required_approvals": 2,
      "allowed_checker_roles": ["security", "admin"],
      "rejection_policy": "threshold",
      "max_checkers": 3
    }],
    "auto_expire_duration": "72h",
    "display_template": {
      "title": "Access: {{system_name}}",
      "fields": [
        {"label": "System", "path": "system_name"},
        {"label": "Access Level", "path": "access_level"},
        {"label": "Justification", "path": "justification"}
      ]
    }
  }' > /dev/null 2>&1 || log "  (already exists)"

log "Creating policy: Account Closure (tenant: banking)"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: banking" \
  -d '{
    "name": "Account Closure",
    "request_type": "account_closure",
    "stages": [
      {
        "index": 0,
        "name": "Retention Review",
        "required_approvals": 1,
        "rejection_policy": "any"
      },
      {
        "index": 1,
        "name": "Final Approval",
        "required_approvals": 1,
        "allowed_checker_roles": ["manager", "admin"],
        "rejection_policy": "any"
      }
    ],
    "identity_fields": ["account_id"],
    "display_template": {
      "title": "Close Account {{account_id}}",
      "fields": [
        {"label": "Account ID", "path": "account_id"},
        {"label": "Customer", "path": "customer_name"},
        {"label": "Reason", "path": "reason"}
      ]
    }
  }' > /dev/null 2>&1 || log "  (already exists)"

# ---- Webhooks (before requests so approval callbacks work) ----
section "Creating Webhooks"

log "Registering webhook for banking tenant"
curl -sf -X POST "${API}/webhooks" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: banking" \
  -d '{
    "url": "http://banking:3001/webhooks/quorum",
    "events": ["approved", "rejected"],
    "secret": "banking-webhook-secret"
  }' > /dev/null 2>&1 || log "  (already exists or error)"

log "Registering webhook for expenses tenant"
curl -sf -X POST "${API}/webhooks" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: expenses" \
  -d '{
    "url": "http://expenses:3002/webhooks/quorum",
    "events": ["approved", "rejected"],
    "secret": "expenses-webhook-secret"
  }' > /dev/null 2>&1 || log "  (already exists or error)"

log "Registering webhook for access-portal tenant"
curl -sf -X POST "${API}/webhooks" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -H "X-Tenant-ID: access-portal" \
  -d '{
    "url": "http://access-portal:3003/webhooks/quorum",
    "events": ["approved", "rejected"],
    "secret": "access-webhook-secret"
  }' > /dev/null 2>&1 || log "  (already exists or error)"

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
log "Tenants:   3 created (banking, expenses, access-portal) + default"
log "Policies:  4 created across tenants"
log "Requests:  6 created via sample app APIs (2 approved, 4 pending)"
log "Webhooks:  3 registered (banking, expenses, access-portal)"
log ""
log "Console:   ${API_URL}/console/  (admin / admin123)"
log "Banking:   ${BANKING_URL}"
log "Expenses:  ${EXPENSES_URL}"
log "Portal:    ${ACCESS_PORTAL_URL}"
