#!/bin/bash
#
# XMPP E2E test environment setup.
#
# Usage:
#   ./setup.sh [OPTIONS] [COMMAND]
#
# Commands:
#   start-server      Start ejabberd using docker compose
#   stop-server       Stop ejabberd
#   status            Check ejabberd status
#   setup-all         Start the server and wait for readiness (default)

set -euo pipefail

readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yaml"
VERBOSE=false
COMPOSE_CMD=""

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

debug() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

usage() {
    echo "XMPP E2E Test Environment Setup Script"
    echo ""
    echo "Usage:"
    echo "  ./setup.sh [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start-server      Start ejabberd using docker compose"
    echo "  stop-server       Stop ejabberd"
    echo "  status            Check ejabberd status"
    echo "  setup-all         Start the server and wait for readiness (default)"
}

check_requirements() {
    if ! command -v docker &> /dev/null; then
        error "Missing required command: docker"
    fi

    if docker compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        error "Missing required command: docker compose"
    fi
}

ensure_certs() {
    if [[ ! -f "${SCRIPT_DIR}/certs/server.pem" ]]; then
        info "Generating TLS certificates..."
        bash "${SCRIPT_DIR}/certs/generate.sh"
    fi
}

wait_for_server() {
    local max_attempts="${1:-30}"
    local attempt=1

    info "Waiting for ejabberd to be ready..."

    while [[ $attempt -le $max_attempts ]]; do
        if docker exec shoutrrr-xmpp-ejabberd ejabberdctl status &> /dev/null; then
            info "ejabberd is ready."
            return 0
        fi

        debug "Attempt ${attempt}/${max_attempts}: not ready yet..."
        sleep 2
        ((attempt++))
    done

    error "ejabberd did not become ready after ${max_attempts} attempts"
}

start_server() {
    ensure_certs

    if [[ ! -f "$COMPOSE_FILE" ]]; then
        error "Docker compose file not found at ${COMPOSE_FILE}"
    fi

    cd "$SCRIPT_DIR"

    if $COMPOSE_CMD ps --status running 2>/dev/null | grep -q shoutrrr-xmpp-ejabberd; then
        info "Stopping existing ejabberd container..."
        $COMPOSE_CMD down || true
    fi

    if $COMPOSE_CMD up -d; then
        info "ejabberd started."
    else
        error "Failed to start ejabberd"
    fi
}

stop_server() {
    cd "$SCRIPT_DIR"
    if $COMPOSE_CMD down; then
        info "ejabberd stopped."
    else
        error "Failed to stop ejabberd"
    fi
}

check_status() {
    cd "$SCRIPT_DIR"
    $COMPOSE_CMD ps
}

setup_all() {
    check_requirements
    start_server
    wait_for_server

    info "XMPP E2E environment is ready."
    info "Export these variables before running tests:"
    info "  export SHOUTRRR_XMPP_URL='xmpp://sender:senderpass@localhost:5222/?to=recipient@localhost&skiptlsverify=yes'"
    info "  export SHOUTRRR_XMPP_TLS_URL='xmpps://sender:senderpass@localhost:5223/?to=recipient@localhost&skiptlsverify=yes'"
    info "  export SHOUTRRR_XMPP_PLAIN_URL='xmpp://sender:senderpass@localhost:5224/?to=recipient@localhost&disabletls=yes'"
    info "  export SHOUTRRR_XMPP_MUC_URL='xmpp://sender:senderpass@localhost:5222/?rooms=alerts@conference.localhost&skiptlsverify=yes'"
    info "  export SHOUTRRR_XMPP_MUC_PASSWORD_URL='xmpp://sender:senderpass@localhost:5222/?rooms=secret@conference.localhost&roompassword=roompass&skiptlsverify=yes'"
    info "Then: go test -v ./testing/e2e/xmpp/"
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
