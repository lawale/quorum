#!/usr/bin/env bash
set -euo pipefail

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

log()     { printf "${BLUE}==> ${NC}%s\n" "$*"; }
success() { printf "${GREEN}==> ${NC}%s\n" "$*"; }
warn()    { printf "${YELLOW}==> ${NC}%s\n" "$*"; }
error()   { printf "${RED}==> ${NC}%s\n" "$*" >&2; }

# --- Prerequisites -----------------------------------------------------------

if ! command -v docker &>/dev/null; then
  error "Docker is not installed. See https://docs.docker.com/get-docker/"
  exit 1
fi

if ! docker compose version &>/dev/null; then
  error "Docker Compose v2 plugin is required."
  error "Install it: https://docs.docker.com/compose/install/"
  exit 1
fi

# --- Network -----------------------------------------------------------------

if ! docker network inspect demo &>/dev/null; then
  log "Creating 'demo' network..."
  docker network create demo
fi

# --- Start services ----------------------------------------------------------

log "Building and starting services (this may take a few minutes on first run)..."
docker compose --profile demo up --build -d

# --- Wait helpers ------------------------------------------------------------

wait_for_url() {
  local url=$1 label=$2 timeout=${3:-120}
  local elapsed=0
  printf "    Waiting for %s..." "$label"
  while ! curl -sf "$url" >/dev/null 2>&1; do
    sleep 2
    elapsed=$((elapsed + 2))
    if [ "$elapsed" -ge "$timeout" ]; then
      printf " ${RED}timeout${NC}\n"
      error "$label did not become ready within ${timeout}s"
      error "Check logs: docker compose --profile demo logs $label"
      exit 1
    fi
    printf "."
  done
  printf " ${GREEN}ready${NC}\n"
}

wait_for_container_exit() {
  local service=$1 timeout=${2:-120}
  local elapsed=0
  printf "    Waiting for %s to finish..." "$service"
  while true; do
    local status
    status=$(docker compose --profile demo ps --format json "$service" 2>/dev/null | head -1)
    if [ -z "$status" ]; then
      printf " ${GREEN}done${NC}\n"
      return 0
    fi
    local state
    state=$(echo "$status" | python3 -c "import sys,json; print(json.load(sys.stdin).get('State',''))" 2>/dev/null || echo "")
    if [ "$state" = "exited" ]; then
      local exit_code
      exit_code=$(echo "$status" | python3 -c "import sys,json; print(json.load(sys.stdin).get('ExitCode',1))" 2>/dev/null || echo "1")
      if [ "$exit_code" = "0" ]; then
        printf " ${GREEN}done${NC}\n"
        return 0
      else
        printf " ${RED}failed (exit code $exit_code)${NC}\n"
        error "Seed script failed. Check logs: docker compose --profile demo logs seed"
        return 1
      fi
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    if [ "$elapsed" -ge "$timeout" ]; then
      printf " ${YELLOW}timeout (may still be running)${NC}\n"
      return 0
    fi
    printf "."
  done
}

# --- Health checks -----------------------------------------------------------

echo ""
log "Waiting for services to be ready..."
echo ""

wait_for_url "http://localhost:8080/health" "server" 120
wait_for_url "http://localhost:3001" "banking" 120
wait_for_url "http://localhost:3002" "expenses" 120
wait_for_url "http://localhost:3003" "access-portal" 120
wait_for_container_exit "seed" 60

# --- Banner ------------------------------------------------------------------

echo ""
printf "${GREEN}${BOLD}"
echo "  ================================================================"
echo "                    Quorum Demo is Ready!                         "
echo "  ================================================================"
printf "${NC}"
echo ""
printf "  ${BOLD}Quorum API${NC}        http://localhost:8080\n"
printf "  ${BOLD}Admin Console${NC}     http://localhost:8080/console/\n"
printf "  ${DIM}                  username: admin  password: admin123${NC}\n"
echo ""
printf "  ${BOLD}Banking Demo${NC}      http://localhost:3001\n"
printf "  ${BOLD}Expenses Demo${NC}     http://localhost:3002\n"
printf "  ${BOLD}Access Portal${NC}     http://localhost:3003\n"
echo ""
printf "  ${DIM}6 sample requests have been seeded across the demo apps.${NC}\n"
echo ""
printf "  To stop:   ${YELLOW}docker compose --profile demo down${NC}\n"
printf "  To reset:  ${YELLOW}docker compose --profile demo down -v${NC}\n"
echo ""
