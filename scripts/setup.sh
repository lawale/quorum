#!/bin/sh
set -eu

# Quorum setup script — creates admin operator, tenants, policies, and webhooks.
# Runs before the sample apps start so they have valid tenants on boot.
# Does NOT depend on sample apps — only on the Quorum server.
#
# Usage: QUORUM_API_URL=http://localhost:8080 sh scripts/setup.sh

API_URL="${QUORUM_API_URL:-http://localhost:8080}"
CONSOLE_API="${API_URL}/api/v1/console"
API="${API_URL}/api/v1"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log()     { printf "${GREEN}[setup]${NC} %s\n" "$1"; }
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
        "rejection_policy": "threshold",
        "max_checkers": 4
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

# ---- Webhooks ----
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

# ---- Summary ----
section "Setup Complete"
log "Tenants:   3 created (banking, expenses, access-portal)"
log "Policies:  4 created across tenants"
log "Webhooks:  3 registered (banking, expenses, access-portal)"
log "Console:   ${API_URL}/console/  (admin / admin123)"
