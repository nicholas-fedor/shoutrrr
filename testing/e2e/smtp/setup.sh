#!/bin/bash
#
# SMTP E2E test environment setup.
#
# Usage:
#   ./setup.sh [OPTIONS] [COMMAND]
#
# Commands:
#   start-server      Start Mailpit using docker compose
#   stop-server       Stop Mailpit
#   status            Check Mailpit status
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
    echo "SMTP E2E Test Environment Setup Script"
    echo ""
    echo "Usage:"
    echo "  ./setup.sh [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start-server      Start Mailpit using docker compose"
    echo "  stop-server       Stop Mailpit"
    echo "  status            Check Mailpit status"
    echo "  setup-all         Start the server and wait for readiness (default)"
}

check_requirements() {
    if ! command -v openssl &> /dev/null; then
        error "Missing required command: openssl"
    fi

    if ! command -v docker &> /dev/null; then
        error "Missing required command: docker"
    fi

    if ! command -v curl &> /dev/null; then
        error "Missing required command: curl"
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
    if [[ ! -f "${SCRIPT_DIR}/certs/cert.pem" || ! -f "${SCRIPT_DIR}/certs/key.pem" ]]; then
        info "Generating TLS certificates..."
        bash "${SCRIPT_DIR}/certs/generate.sh"
    fi
}

wait_for_server() {
    local max_attempts="${1:-30}"
    local attempt=1

    info "Waiting for both Mailpit instances to be ready..."

    while [[ $attempt -le $max_attempts ]]; do
        if curl -sf http://localhost:8025/api/v1/info &> /dev/null &&
            curl -sf http://localhost:8026/api/v1/info &> /dev/null; then
            info "Mailpit is ready."
            return 0
        fi

        debug "Attempt ${attempt}/${max_attempts}: not ready yet..."
        sleep 2
        ((attempt++))
    done

    error "Mailpit did not become ready after ${max_attempts} attempts"
}

start_server() {
    ensure_certs

    if [[ ! -f "$COMPOSE_FILE" ]]; then
        error "Docker compose file not found at ${COMPOSE_FILE}"
    fi

    cd "$SCRIPT_DIR"

    if $COMPOSE_CMD ps --status running 2>/dev/null | grep -q shoutrrr-smtp-mailpit; then
        info "Stopping existing Mailpit containers..."
        $COMPOSE_CMD down || true
    fi

    if $COMPOSE_CMD up -d; then
        info "Mailpit started."
    else
        error "Failed to start Mailpit"
    fi
}

stop_server() {
    cd "$SCRIPT_DIR"
    if $COMPOSE_CMD down; then
        info "Mailpit stopped."
    else
        error "Failed to stop Mailpit"
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

    info "SMTP E2E environment is ready."
    info "Mailpit web UI: http://localhost:8025 (STARTTLS) and http://localhost:8026 (implicit TLS)"
    info "Run the tests with: go test -v ./testing/e2e/smtp/"
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
