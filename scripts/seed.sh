#!/bin/sh
set -eu

# Quorum seed script — creates sample policies, requests, and approvals.
# Usage: QUORUM_API_URL=http://localhost:8080 sh scripts/seed.sh

API_URL="${QUORUM_API_URL:-http://localhost:8080}"
CONSOLE_API="${API_URL}/api/v1/console"
API="${API_URL}/api/v1"

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

# ---- Console admin setup ----
section "Console Admin Setup"
NEEDS_SETUP=$(curl -sf "${CONSOLE_API}/auth/status" | sed -n 's/.*"needs_setup":\([a-z]*\).*/\1/p')
if [ "$NEEDS_SETUP" = "true" ]; then
  curl -sf -X POST "${CONSOLE_API}/auth/setup" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123","display_name":"Demo Admin"}' > /dev/null
  log "Created admin operator (admin / admin123)"
else
  log "Admin already exists, skipping setup"
fi

# ---- Policies ----
section "Creating Policies"

log "Creating policy: Expense Approval"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
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
  }' > /dev/null

log "Creating policy: Wire Transfer"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
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
  }' > /dev/null

log "Creating policy: System Access Request"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
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
  }' > /dev/null

log "Creating policy: Account Closure"
curl -sf -X POST "${API}/policies" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
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
  }' > /dev/null

# ---- Requests ----
section "Creating Requests"

log "Creating request: Expense by alice"
EXPENSE_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: alice" \
  -d '{
    "type": "expense_approval",
    "payload": {
      "employee_name": "Alice Johnson",
      "amount": 1250.00,
      "category": "Travel",
      "description": "Client visit to NYC - flights and hotel"
    }
  }' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
log "  -> ${EXPENSE_ID}"

log "Creating request: Wire transfer by bob"
WIRE_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: bob" \
  -d '{
    "type": "wire_transfer",
    "payload": {
      "source_account_id": "ACC-001",
      "amount": 50000,
      "destination": "IBAN-DE89370400440532013000"
    }
  }' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
log "  -> ${WIRE_ID}"

log "Creating request: Access request by charlie"
ACCESS_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: charlie" \
  -d '{
    "type": "access_request",
    "payload": {
      "system_name": "Production Database",
      "access_level": "read-write",
      "justification": "Need to run quarterly data migration"
    }
  }' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
log "  -> ${ACCESS_ID}"

log "Creating request: Expense by dave"
EXPENSE2_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: dave" \
  -d '{
    "type": "expense_approval",
    "payload": {
      "employee_name": "Dave Kim",
      "amount": 89.99,
      "category": "Software",
      "description": "Annual IDE license renewal"
    }
  }' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
log "  -> ${EXPENSE2_ID}"

log "Creating request: Account closure by eve"
CLOSURE_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: eve" \
  -d '{
    "type": "account_closure",
    "payload": {
      "account_id": "ACC-4472",
      "customer_name": "Acme Corp",
      "reason": "Company dissolved"
    }
  }' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
log "  -> ${CLOSURE_ID}"

log "Creating request: Wire transfer by frank"
WIRE2_ID=$(curl -sf -X POST "${API}/requests" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: frank" \
  -d '{
    "type": "wire_transfer",
    "payload": {
      "source_account_id": "ACC-007",
      "amount": 15000,
      "destination": "IBAN-GB29NWBK60161331926819"
    }
  }' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
log "  -> ${WIRE2_ID}"

# ---- Approve some requests ----
section "Approving Some Requests"

log "Approving expense by alice (single-stage -> approved)"
curl -sf -X POST "${API}/requests/${EXPENSE_ID}/approve" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: manager-maria" \
  -H "X-User-Roles: manager" \
  -d '{"comment": "Approved - within travel budget"}' > /dev/null

log "Approving wire transfer stage 1 (advances to compliance review)"
curl -sf -X POST "${API}/requests/${WIRE_ID}/approve" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: manager-maria" \
  -H "X-User-Roles: manager" \
  -d '{"comment": "Manager approved"}' > /dev/null

log "Approving expense by dave (single-stage -> approved)"
curl -sf -X POST "${API}/requests/${EXPENSE2_ID}/approve" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: finance-fay" \
  -H "X-User-Roles: finance" \
  -d '{"comment": "Small amount, auto-approved"}' > /dev/null

# ---- Webhook ----
section "Creating Webhook"

log "Registering webhook"
curl -sf -X POST "${API}/webhooks" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: seed-admin" \
  -d '{
    "url": "https://httpbin.org/post",
    "events": ["approved", "rejected"],
    "secret": "demo-webhook-secret-change-me"
  }' > /dev/null

# ---- Summary ----
section "Seed Complete"
log "Policies:  4 created"
log "Requests:  6 created (2 approved, 4 pending at various stages)"
log "Webhook:   1 registered (httpbin.org)"
log ""
log "Console:   ${API_URL}/console/  (admin / admin123)"
log "API:       ${API_URL}/api/v1/"
log ""
log "Try: curl -H 'X-User-ID: alice' ${API_URL}/api/v1/requests"
