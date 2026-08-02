#!/bin/bash
#
# ntfy E2E Test Environment Setup Script
#
# This script starts a local ntfy server for end-to-end testing
# of Shoutrrr's ntfy service.
#
# Usage:
#   ./setup.sh [OPTIONS] [COMMAND]
#
# Commands:
#   start-server      Start the ntfy server using docker compose
#   stop-server       Stop the ntfy server
#   status            Check the status of the ntfy server
#   setup-all         Start the server and wait for it to be ready (default)
#
# Options:
#   --help, -h        Show this help message
#   --verbose, -v    Enable verbose output
#
# Environment:
#   The script automatically loads .env file from its directory if present.
#   Required variables:
#     None (uses defaults: localhost:8080)
#

set -euo pipefail

# =============================================================================
# Configuration and Constants
# =============================================================================

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yaml"

# Default values
DEFAULT_NTFY_HOST="localhost:8080"

# Verbose mode
VERBOSE=false

# Docker compose command (detected in check_requirements)
COMPOSE_CMD=""

# =============================================================================
# Helper Functions
# =============================================================================

# Print an error message and exit
error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# Print an info message
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

# Print a warning message
warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Print a debug message (only in verbose mode)
debug() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

# Print usage information
usage() {
    echo "ntfy E2E Test Environment Setup Script"
    echo ""
    echo "This script starts a local ntfy server for end-to-end testing."
    echo ""
    echo "Usage:"
    echo "  ./setup.sh [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start-server      Start the ntfy server using docker compose"
    echo "  stop-server       Stop the ntfy server"
    echo "  status            Check the status of the ntfy server"
    echo "  setup-all         Start the server and wait for it to be ready (default)"
    echo ""
    echo "Options:"
    echo "  --help, -h       Show this help message"
    echo "  --verbose, -v    Enable verbose output"
    echo ""
    echo "Examples:"
    echo "  ./setup.sh                  # Start server and wait for readiness"
    echo "  ./setup.sh start-server     # Start server only"
    echo "  ./setup.sh stop-server      # Stop server"
    echo "  ./setup.sh --verbose setup-all  # Run with verbose output"
}

# Load environment variables from .env file
load_env_file() {
    if [[ -f "$ENV_FILE" ]]; then
        debug "Loading environment variables from ${ENV_FILE}"
        set -a
        source "$ENV_FILE"
        set +a
        info "Loaded environment variables from ${ENV_FILE}"
    else
        debug "No .env file found at ${ENV_FILE}"
    fi
}

# Check if required commands are available
check_requirements() {
    local missing_cmds=()

    # Check for docker
    if ! command -v docker &> /dev/null; then
        missing_cmds+=("docker")
    fi

    # Check for docker compose (plugin or standalone)
    if docker compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        missing_cmds+=("docker-compose")
    fi

    # Check for curl
    if ! command -v curl &> /dev/null; then
        missing_cmds+=("curl")
    fi

    if [[ ${#missing_cmds[@]} -gt 0 ]]; then
        error "Missing required commands: ${missing_cmds[*]}"
    fi

    debug "All required commands are available"
    debug "Using compose command: ${COMPOSE_CMD}"
}

# Wait for the ntfy server to be ready
wait_for_server() {
    local base="${1:-http://${DEFAULT_NTFY_HOST}}"
    local max_attempts="${2:-30}"
    local attempt=1

    info "Waiting for ntfy server at ${base} to be ready..."

    while [[ $attempt -le $max_attempts ]]; do
        if curl -s -o /dev/null -w "%{http_code}" "${base}/v1/health" 2>/dev/null | grep -q "200"; then
            info "ntfy server is ready!"
            return 0
        fi

        debug "Attempt ${attempt}/${max_attempts}: Server not ready yet..."
        sleep 2
        ((attempt++))
    done

    error "ntfy server did not become ready after ${max_attempts} attempts"
}

# =============================================================================
# Setup Functions
# =============================================================================

# Start the ntfy server using docker compose
start_server() {
    info "Starting ntfy server..."

    # Check if docker compose file exists
    if [[ ! -f "$COMPOSE_FILE" ]]; then
        error "Docker compose file not found at ${COMPOSE_FILE}"
    fi

    # Ensure COMPOSE_CMD is set
    if [[ -z "$COMPOSE_CMD" ]]; then
        check_requirements
    fi

    # Change to the script directory
    cd "$SCRIPT_DIR"

    # Stop any existing container
    if $COMPOSE_CMD ps --status running 2>/dev/null | grep -q ntfy; then
        info "Stopping existing ntfy container..."
        $COMPOSE_CMD down || true
    fi

    # Start the server
    if $COMPOSE_CMD up -d; then
        info "ntfy server started successfully"
        info "Server is available at http://${DEFAULT_NTFY_HOST}"
        info "Use 'docker logs ntfy -f' to watch the logs"
    else
        error "Failed to start ntfy server"
    fi
}

# Stop the ntfy server
stop_server() {
    info "Stopping ntfy server..."

    # Ensure COMPOSE_CMD is set
    if [[ -z "$COMPOSE_CMD" ]]; then
        check_requirements
    fi

    # Change to the script directory
    cd "$SCRIPT_DIR"

    if $COMPOSE_CMD down; then
        info "ntfy server stopped successfully"
    else
        error "Failed to stop ntfy server"
    fi
}

# Check the status of the ntfy server
check_status() {
    # Ensure COMPOSE_CMD is set
    if [[ -z "$COMPOSE_CMD" ]]; then
        check_requirements
    fi

    # Change to the script directory
    cd "$SCRIPT_DIR"

    info "Checking ntfy server status..."
    $COMPOSE_CMD ps
}

# Run all setup steps in order
setup_all() {
    info "Starting ntfy E2E test environment setup..."
    echo ""

    # Check requirements
    check_requirements

    # Step 1: Start server
    echo ""
    info "=== Step 1/2: Starting ntfy server ==="
    start_server

    # Step 2: Wait for server to be ready
    echo ""
    info "=== Step 2/2: Waiting for server to be ready ==="
    wait_for_server "http://${DEFAULT_NTFY_HOST}"

    echo ""
    info "=========================================="
    info "ntfy E2E test environment setup complete!"
    info "=========================================="
    info ""
    info "You can now run the E2E tests with:"
    info "  go test -v ./testing/e2e/ntfy/..."
    info ""
    info "To stop the server, run:"
    info "  ${SCRIPT_DIR}/setup.sh stop-server"
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    load_env_file

    # Parse command-line arguments
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

    # Execute the requested command
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

# Run main function with all arguments
main "$@"
