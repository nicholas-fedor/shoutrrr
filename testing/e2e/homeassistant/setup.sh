#!/bin/bash
#
# Home Assistant E2E Test Environment Setup Script
#
# This script starts a local Home Assistant server, completes onboarding,
# and writes SHOUTRRR_HOMEASSISTANT_URL for end-to-end tests.
#
# Usage:
#   ./setup.sh [OPTIONS] [COMMAND]
#
# Commands:
#   start-server      Start Home Assistant using docker compose
#   stop-server       Stop Home Assistant
#   status            Check the status of Home Assistant
#   setup-all         Start the server, onboard, and write .env (default)
#
# Options:
#   --help, -h        Show this help message
#   --verbose, -v     Enable verbose output
#

set -euo pipefail

readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yaml"
CONFIG_DIR="${SCRIPT_DIR}/config"

readonly HA_HOST="localhost:8123"
readonly HA_BASE="http://${HA_HOST}"
readonly HA_CLIENT_ID="https://example.com/app"
readonly HA_USERNAME="shoutrrr"
readonly HA_PASSWORD="shoutrrr-e2e-password"
readonly HA_NAME="Shoutrrr"
readonly CURL_CONNECT_TIMEOUT=5
readonly CURL_MAX_TIME=15

VERBOSE=false
COMPOSE_CMD=""

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

debug() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

usage() {
    echo "Home Assistant E2E Test Environment Setup Script"
    echo ""
    echo "Usage:"
    echo "  ./setup.sh [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start-server      Start Home Assistant using docker compose"
    echo "  stop-server       Stop Home Assistant"
    echo "  status            Check the status of Home Assistant"
    echo "  setup-all         Start the server, onboard, and write .env (default)"
    echo ""
    echo "Options:"
    echo "  --help, -h        Show this help message"
    echo "  --verbose, -v     Enable verbose output"
}

check_requirements() {
    local missing_cmds=()

    if ! command -v docker &> /dev/null; then
        missing_cmds+=("docker")
    fi

    if docker compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        missing_cmds+=("docker-compose")
    fi

    if ! command -v curl &> /dev/null; then
        missing_cmds+=("curl")
    fi

    if ! command -v python3 &> /dev/null; then
        missing_cmds+=("python3")
    fi

    if [[ ${#missing_cmds[@]} -gt 0 ]]; then
        error "Missing required commands: ${missing_cmds[*]}"
    fi

    debug "Using compose command: ${COMPOSE_CMD}"
}

wait_for_onboarding() {
    local max_attempts="${1:-90}"
    local attempt=1

    info "Waiting for Home Assistant onboarding API at ${HA_BASE}..."

    while [[ $attempt -le $max_attempts ]]; do
        if curl -s --connect-timeout "${CURL_CONNECT_TIMEOUT}" --max-time "${CURL_MAX_TIME}" \
            -o /dev/null -w "%{http_code}" "${HA_BASE}/api/onboarding" 2>/dev/null | grep -q "200"; then
            info "Home Assistant onboarding API is ready"
            return 0
        fi

        debug "Attempt ${attempt}/${max_attempts}: onboarding API not ready yet"
        sleep 2
        ((attempt++))
    done

    error "Home Assistant did not become ready after ${max_attempts} attempts"
}

json_field() {
    local field="$1"
    python3 -c "import json,sys; print(json.load(sys.stdin)[${field@Q}])"
}

urlencode() {
    python3 -c "import sys,urllib.parse; print(urllib.parse.quote(sys.stdin.read().rstrip('\n'), safe=''))"
}

complete_onboarding() {
    info "Creating Home Assistant user via onboarding API"

    local user_json user_status
    user_json="$(curl -sS --connect-timeout "${CURL_CONNECT_TIMEOUT}" --max-time "${CURL_MAX_TIME}" \
        -w "\n%{http_code}" \
        -X POST "${HA_BASE}/api/onboarding/users" \
        -H "Content-Type: application/json" \
        -d "{\"client_id\":\"${HA_CLIENT_ID}\",\"name\":\"${HA_NAME}\",\"username\":\"${HA_USERNAME}\",\"password\":\"${HA_PASSWORD}\",\"language\":\"en\"}")"
    user_status="${user_json##*$'\n'}"
    user_json="${user_json%$'\n'*}"
    debug "onboarding/users HTTP ${user_status}"

    local auth_code
    auth_code="$(printf '%s' "${user_json}" | json_field "auth_code")" || error "Failed to parse onboarding auth_code"

    local token_json token_status
    token_json="$(curl -sS --connect-timeout "${CURL_CONNECT_TIMEOUT}" --max-time "${CURL_MAX_TIME}" \
        -w "\n%{http_code}" \
        -X POST "${HA_BASE}/auth/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        --data-urlencode "grant_type=authorization_code" \
        --data-urlencode "code=${auth_code}" \
        --data-urlencode "client_id=${HA_CLIENT_ID}")"
    token_status="${token_json##*$'\n'}"
    token_json="${token_json%$'\n'*}"
    debug "auth/token HTTP ${token_status}"

    local access_token
    access_token="$(printf '%s' "${token_json}" | json_field "access_token")" || error "Failed to parse access_token"

    curl -sS --connect-timeout "${CURL_CONNECT_TIMEOUT}" --max-time "${CURL_MAX_TIME}" \
        -o /dev/null -X POST "${HA_BASE}/api/onboarding/core_config" \
        -H "Authorization: Bearer ${access_token}" \
        -H "Content-Type: application/json" \
        -d '{}' || warn "core_config onboarding step returned an error"

    curl -sS --connect-timeout "${CURL_CONNECT_TIMEOUT}" --max-time "${CURL_MAX_TIME}" \
        -o /dev/null -X POST "${HA_BASE}/api/onboarding/analytics" \
        -H "Authorization: Bearer ${access_token}" \
        -H "Content-Type: application/json" \
        -d '{}' || warn "analytics onboarding step returned an error"

    local encoded_token
    encoded_token="$(printf '%s' "${access_token}" | urlencode)"

    cat > "${ENV_FILE}" <<EOF
SHOUTRRR_HOMEASSISTANT_URL=homeassistant://${encoded_token}@localhost:8123/?disabletls=yes
EOF

    info "Wrote ${ENV_FILE}"
}

start_server() {
    info "Starting Home Assistant"

    if [[ ! -f "$COMPOSE_FILE" ]]; then
        error "Docker compose file not found at ${COMPOSE_FILE}"
    fi

    if [[ -z "$COMPOSE_CMD" ]]; then
        check_requirements
    fi

    cd "$SCRIPT_DIR"

    mkdir -p "${CONFIG_DIR}"

    if $COMPOSE_CMD up -d; then
        info "Home Assistant started"
        info "Server is available at ${HA_BASE}"
    else
        error "Failed to start Home Assistant"
    fi
}

stop_server() {
    info "Stopping Home Assistant"

    if [[ -z "$COMPOSE_CMD" ]]; then
        check_requirements
    fi

    cd "$SCRIPT_DIR"

    if $COMPOSE_CMD down; then
        info "Home Assistant stopped"
    else
        error "Failed to stop Home Assistant"
    fi
}

check_status() {
    if [[ -z "$COMPOSE_CMD" ]]; then
        check_requirements
    fi

    cd "$SCRIPT_DIR"

    info "Checking Home Assistant status"
    $COMPOSE_CMD ps
}

setup_all() {
    info "Starting Home Assistant E2E test environment setup"
    echo ""

    check_requirements

    echo ""
    info "=== Step 1/3: Resetting config and starting Home Assistant ==="
    cd "$SCRIPT_DIR"
    $COMPOSE_CMD down -v >/dev/null 2>&1 || true
    rm -rf "${CONFIG_DIR}"
    mkdir -p "${CONFIG_DIR}"
    start_server

    echo ""
    info "=== Step 2/3: Waiting for onboarding API ==="
    wait_for_onboarding

    echo ""
    info "=== Step 3/3: Completing onboarding ==="
    complete_onboarding

    echo ""
    info "=========================================="
    info "Home Assistant E2E setup complete"
    info "=========================================="
    info ""
    info "Run the E2E tests with:"
    info "  go test -v ./testing/e2e/homeassistant/..."
    info ""
    info "To stop the server, run:"
    info "  ${SCRIPT_DIR}/setup.sh stop-server"
}

main() {
    local command="setup-all"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --help|-h)
                usage
                exit 0
                ;;
            --verbose|-v)
                VERBOSE=true
                shift
                ;;
            start-server|stop-server|status|setup-all)
                command="$1"
                shift
                ;;
            *)
                error "Unknown option or command: $1"
                ;;
        esac
    done

    case "$command" in
        start-server)
            check_requirements
            start_server
            ;;
        stop-server)
            check_requirements
            stop_server
            ;;
        status)
            check_requirements
            check_status
            ;;
        setup-all)
            setup_all
            ;;
    esac
}

main "$@"
