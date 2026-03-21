#!/bin/bash
#
# Matrix E2E Test Environment Setup Script
#
# This script performs all the steps required to set up the Matrix Synapse server
# for end-to-end testing of Shoutrrr's Matrix service.
#
# Usage:
#   ./setup.sh [OPTIONS] [COMMAND]
#
# Commands:
#   generate-config   Generate the Synapse server configuration
#   start-server      Start the Matrix Synapse server using docker compose
#   create-user       Create the admin test user
#   create-room       Create the test room with alias #test:localhost
#   setup-all         Run all setup steps in order (default)
#
# Options:
#   --help, -h        Show this help message
#   --verbose, -v    Enable verbose output
#
# Environment:
#   The script automatically loads .env file from its directory if present.
#   Required variables for create-user:
#     None (uses defaults: admin/admin)
#   Required variables for create-room:
#     SHOUTRRR_MATRIX_HOST
#     SHOUTRRR_MATRIX_USER
#     SHOUTRRR_MATRIX_PASSWORD
#     SHOUTRRR_MATRIX_ROOM (optional, defaults to #test:localhost)
#     SHOUTRRR_MATRIX_DISABLE_TLS (optional)
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
DATA_DIR="${SCRIPT_DIR}/data"
ENV_FILE="${SCRIPT_DIR}/.env"
ELEMENT_ENV_FILE="${SCRIPT_DIR}/element.env"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yaml"

# Default values
DEFAULT_MATRIX_USER="admin"
DEFAULT_MATRIX_PASSWORD="admin"
DEFAULT_MATRIX_HOST="localhost:8008"
DEFAULT_MATRIX_ROOM="#test:localhost"

# Rate limiting configuration for E2E testing
RATE_LIMIT_CONFIG='# Rate limiting configuration - effectively disabled for E2E testing
# Using very high values to allow rapid test execution without rate limits.
# See: https://element-hq.github.io/synapse/latest/usage/configuration/config_documentation.html#ratelimiting
rc_message:
  per_second: 1000
  burst_count: 10000

rc_registration:
  per_second: 1000
  burst_count: 10000

rc_login:
  address:
    per_second: 1000
    burst_count: 10000
  account:
    per_second: 1000
    burst_count: 10000
  failed_attempts:
    per_second: 1000
    burst_count: 10000

rc_joins:
  local:
    per_second: 1000
    burst_count: 10000
  remote:
    per_second: 1000
    burst_count: 10000'

# Verbose mode
VERBOSE=false

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
    echo "Matrix E2E Test Environment Setup Script"
    echo ""
    echo "This script performs all the steps required to set up the Matrix Synapse server"
    echo "for end-to-end testing of Shoutrrr's Matrix service."
    echo ""
    echo "Usage:"
    echo "  ./setup.sh [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  generate-config   Generate the Synapse server configuration"
    echo "  start-server     Start the Matrix Synapse server using docker compose"
    echo "  create-user      Create the admin test user"
    echo "  create-room      Create the test room with alias #test:localhost"
    echo "  setup-all       Run all setup steps in order (default)"
    echo ""
    echo "Options:"
    echo "  --help, -h       Show this help message"
    echo "  --verbose, -v    Enable verbose output"
    echo ""
    echo "Environment:"
    echo "  The script automatically loads .env file from its directory if present."
    echo "  Required variables for create-user:"
    echo "    None (uses defaults: admin/admin)"
    echo "  Required variables for create-room:"
    echo "    SHOUTRRR_MATRIX_HOST"
    echo "    SHOUTRRR_MATRIX_USER"
    echo "    SHOUTRRR_MATRIX_PASSWORD"
    echo "    SHOUTRRR_MATRIX_ROOM (optional, defaults to #test:localhost)"
    echo "    SHOUTRRR_MATRIX_DISABLE_TLS (optional)"
    echo ""
    echo "Examples:"
    echo "  ./setup.sh                  # Run all setup steps"
    echo "  ./setup.sh generate-config # Generate configuration only"
    echo "  ./setup.sh start-server    # Start the server only"
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

    # Check for docker compose
    if ! docker compose version &> /dev/null 2>&1 && ! docker-compose --version &> /dev/null 2>&1; then
        missing_cmds+=("docker-compose")
    fi

    # Check for curl
    if ! command -v curl &> /dev/null; then
        missing_cmds+=("curl")
    fi

    # Check for jq
    if ! command -v jq &> /dev/null; then
        missing_cmds+=("jq")
    fi

    if [[ ${#missing_cmds[@]} -gt 0 ]]; then
        error "Missing required commands: ${missing_cmds[*]}"
    fi

    debug "All required commands are available"
}

# Wait for the Matrix server to be ready
wait_for_server() {
    local host="$1"
    local max_attempts="${2:-30}"
    local attempt=1

    info "Waiting for Matrix server at ${host} to be ready..."

    while [[ $attempt -le $max_attempts ]]; do
        if curl -s -o /dev/null -w "%{http_code}" "http://${host}/_matrix/client/versions" 2>/dev/null | grep -q "200"; then
            info "Matrix server is ready!"
            return 0
        fi

        debug "Attempt ${attempt}/${max_attempts}: Server not ready yet..."
        sleep 2
        ((attempt++))
    done

    error "Matrix server did not become ready after ${max_attempts} attempts"
}

# =============================================================================
# Setup Functions
# =============================================================================

# Generate the Synapse server configuration
# This creates the necessary configuration files in the data directory
generate_config() {
    info "Generating Matrix Synapse configuration..."

    # Check for element.env file
    if [[ ! -f "$ELEMENT_ENV_FILE" ]]; then
        warn "element.env not found at ${ELEMENT_ENV_FILE}"
        info "Creating default element.env file..."

        cat > "$ELEMENT_ENV_FILE" << 'EOF'
# Matrix Synapse configuration
# Server name
SYNAPSE_SERVER_NAME=localhost
# Enable registration
SYNAPSE_ALLOW_GUEST_ACCESS=true
# Enable registration without verification
SYNAPSE_ALLOW_INSECURE_REGISTRATION_TOKEN=true
EOF
    fi

    # Create data directory if it doesn't exist
    if [[ ! -d "$DATA_DIR" ]]; then
        mkdir -p "$DATA_DIR"
        debug "Created data directory at ${DATA_DIR}"
    fi

    # Run the Synapse generate command
    info "Running Synapse configuration generator..."
    docker run --rm \
        -u "$(id -u):$(id -g)" \
        -v "${DATA_DIR}:/data" \
        --env-file "$ELEMENT_ENV_FILE" \
        matrixdotorg/synapse:latest generate

    if [[ $? -eq 0 ]]; then
        info "Configuration generated successfully in ${DATA_DIR}"
    else
        error "Failed to generate configuration"
    fi

    # Add rate limiting configuration to homeserver.yaml
    # This must be done before starting the server to ensure the configuration takes effect
    local homeserver_yaml="${DATA_DIR}/homeserver.yaml"
    if [[ -f "$homeserver_yaml" ]]; then
        if ! grep -q "^rc_message:" "$homeserver_yaml" 2>/dev/null; then
            info "Adding rate limiting config to homeserver.yaml..."
            echo "$RATE_LIMIT_CONFIG" >> "$homeserver_yaml"
            info "Rate limiting configuration added"
        else
            debug "Rate limiting config already present in homeserver.yaml"
        fi
    fi
}

# Disable rate limiting in the Synapse configuration via Admin API
# This uses the Synapse Admin API to override rate limiting per user
disable_rate_limiting() {
    # Load environment variables
    load_env_file

    # Get configuration from environment variables
    local host="${SHOUTRRR_MATRIX_HOST:-${DEFAULT_MATRIX_HOST}}"
    local user="${SHOUTRRR_MATRIX_USER:-${DEFAULT_MATRIX_USER}}"
    local password="${SHOUTRRR_MATRIX_PASSWORD:-${DEFAULT_MATRIX_PASSWORD}}"

    # Validate required variables
    if [[ -z "$host" ]]; then
        error "SHOUTRRR_MATRIX_HOST is not set"
    fi
    if [[ -z "$user" ]]; then
        error "SHOUTRRR_MATRIX_USER is not set"
    fi
    if [[ -z "$password" ]]; then
        error "SHOUTRRR_MATRIX_PASSWORD is not set"
    fi

    local homeserver_yaml="${DATA_DIR}/homeserver.yaml"

    info "Disabling rate limiting for user '${user}' at ${host}..."

    # First, disable rate limiting in the global config (if not already done)
    if [[ -f "$homeserver_yaml" ]]; then
        if ! grep -q "^rc_message:" "$homeserver_yaml" 2>/dev/null; then
            info "Adding rate limiting config to homeserver.yaml..."
            echo "$RATE_LIMIT_CONFIG" >> "$homeserver_yaml"
        else
            debug "Rate limiting config already present in homeserver.yaml"
        fi
    fi

    # Wait for server to be ready
    wait_for_server "$host" 30

    # Get base URL using the helper function (handles TLS setting)
    local base_url
    base_url=$(get_base_url "$host")

    info "Using base URL: ${base_url}"

    # Login to get access token using the existing login function
    local admin_token
    admin_token=$(login "$base_url" "$user" "$password")

    # Now disable rate limiting for the user via Admin API
    info "Disabling rate limit for user via Admin API..."
    local response
    response=$(
        curl -s -X POST "${base_url}/_synapse/admin/v1/users/@${user}:localhost/override_ratelimit" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${admin_token}" \
            -d '{
                "messages_per_second": 0,
                "burst_count": 0
            }' 2>&1
    ) || error "Failed to disable rate limiting for user"

    # Check for errors in response
    local error_msg
    error_msg=$(echo "$response" | jq -r '.errcode // empty' 2>/dev/null || true)

    if [[ -n "$error_msg" && "$error_msg" != "null" ]]; then
        local error_desc
        error_desc=$(echo "$response" | jq -r '.error // "Unknown error"' 2>/dev/null || true)
        error "Failed to disable rate limiting: ${error_msg} - ${error_desc}"
    fi

    info "Rate limit override response for ${user}: $response"

    # Also disable for testuser if it exists
    info "Disabling rate limit for testuser via Admin API..."
    response=$(
        curl -s -X POST "${base_url}/_synapse/admin/v1/users/@testuser:localhost/override_ratelimit" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${admin_token}" \
            -d '{
                "messages_per_second": 0,
                "burst_count": 0
            }' 2>&1
    ) || warn "Failed to disable rate limiting for testuser (user may not exist)"

    # Check for errors - but don't fail if testuser doesn't exist
    error_msg=$(echo "$response" | jq -r '.errcode // empty' 2>/dev/null || true)

    if [[ -n "$error_msg" && "$error_msg" != "null" ]]; then
        # Only warn if it's not a "not found" error
        local error_desc
        error_desc=$(echo "$response" | jq -r '.error // "Unknown error"' 2>/dev/null || true)
        if [[ "$error_msg" == "M_NOT_FOUND" ]]; then
            info "testuser does not exist, skipping rate limit override"
        else
            warn "Failed to disable rate limiting for testuser: ${error_msg} - ${error_desc}"
        fi
    else
        info "Rate limit override response for testuser: $response"
    fi

    info "Rate limiting disabled successfully via Admin API"
}

# Start the Matrix Synapse server using docker compose
start_server() {
    info "Starting Matrix Synapse server..."

    # Check if docker compose file exists
    if [[ ! -f "$COMPOSE_FILE" ]]; then
        error "Docker compose file not found at ${COMPOSE_FILE}"
    fi

    # Change to the script directory
    cd "$SCRIPT_DIR"

    # Export UID and GID for docker-compose to use
    # This ensures the container runs as the current host user
    # Use _UID/_GID to avoid conflict with shell built-in readonly UID variable
    local _user_uid _user_gid
    _user_uid="${MATRIX_UID:-$(id -u)}"
    _user_gid="${MATRIX_GID:-$(id -g)}"
    export MATRIX_UID="$_user_uid"
    export MATRIX_GID="$_user_gid"

    debug "Using MATRIX_UID=${MATRIX_UID} MATRIX_GID=${MATRIX_GID} for container"

    # Stop any existing container
    if docker compose ps --status running 2>/dev/null | grep -q synapse; then
        info "Stopping existing Synapse container..."
        docker compose down || true
    fi

    # Start the server
    docker compose up -d

    if [[ $? -eq 0 ]]; then
        info "Matrix Synapse server started successfully"
        info "Server is available at http://localhost:8008"
        info "Use 'docker logs synapse -f' to watch the logs"
    else
        error "Failed to start Matrix Synapse server"
    fi
}

# Create the admin test user
create_user() {
    info "Creating admin test user..."

    # Default credentials
    local user="${SHOUTRRR_MATRIX_USER:-${DEFAULT_MATRIX_USER}}"
    local password="${SHOUTRRR_MATRIX_PASSWORD:-${DEFAULT_MATRIX_PASSWORD}}"
    local server_url="${SHOUTRRR_MATRIX_HOST:-${DEFAULT_MATRIX_HOST}}"

    # Wait for server to be ready
    wait_for_server "$server_url" 60

    # Register the user using docker exec
    info "Registering user '${user}'..."

    docker exec synapse \
        register_new_matrix_user \
        -u "$user" \
        -p "$password" \
        -a \
        -c /data/homeserver.yaml \
        "http://${server_url}"

    if [[ $? -eq 0 ]]; then
        info "User '${user}' created successfully with password '${password}'"
    else
        # Check if user already exists
        if docker exec synapse \
            register_new_matrix_user \
            -u "$user" \
            -p "$password" \
            -a \
            -c /data/homeserver.yaml \
            "http://${server_url}" 2>&1 | grep -q "already exists"; then
            warn "User '${user}' already exists, continuing..."
        else
            error "Failed to create user '${user}'"
        fi
    fi
}

# Get the base URL for Matrix API
get_base_url() {
    local host="${1:-${DEFAULT_MATRIX_HOST}}"
    local disable_tls="${SHOUTRRR_MATRIX_DISABLE_TLS:-false}"

    if [[ "$disable_tls" == "true" ]]; then
        echo "http://${host}"
    else
        echo "https://${host}"
    fi
}

# Login to Matrix and get access token
login() {
    local base_url="$1"
    local user="$2"
    local password="$3"

    debug "Logging in as ${user} to ${base_url}..."

    local response
    response=$(
        curl -s -X POST "${base_url}/_matrix/client/v3/login" \
            -H "Content-Type: application/json" \
            -d "{
                \"type\": \"m.login.password\",
                \"identifier\": {
                    \"type\": \"m.id.user\",
                    \"user\": \"${user}\"
                },
                \"password\": \"${password}\"
            }" 2>&1
    ) || error "Failed to connect to Matrix server at ${base_url}"

    # Check for errors in response
    local error_msg
    error_msg=$(echo "$response" | jq -r '.errcode // empty' 2>/dev/null || true)

    if [[ -n "$error_msg" && "$error_msg" != "null" ]]; then
        local error_desc
        error_desc=$(echo "$response" | jq -r '.error // "Unknown error"' 2>/dev/null || true)
        error "Login failed: ${error_msg} - ${error_desc}"
    fi

    local access_token
    access_token=$(echo "$response" | jq -r '.access_token' 2>/dev/null) || error "Failed to parse login response"

    if [[ -z "$access_token" || "$access_token" == "null" ]]; then
        error "No access token received from login"
    fi

    debug "Login successful, token: ${access_token:0:20}..."
    echo "$access_token"
}

# Create the test room with alias #test:localhost
create_room() {
    info "Creating test room..."

    # Load environment variables
    load_env_file

    # Get configuration
    local host="${SHOUTRRR_MATRIX_HOST:-${DEFAULT_MATRIX_HOST}}"
    local user="${SHOUTRRR_MATRIX_USER:-${DEFAULT_MATRIX_USER}}"
    local password="${SHOUTRRR_MATRIX_PASSWORD:-${DEFAULT_MATRIX_PASSWORD}}"
    local room_alias="${SHOUTRRR_MATRIX_ROOM:-${DEFAULT_MATRIX_ROOM}}"

    # Validate required variables
    if [[ -z "$host" ]]; then
        error "SHOUTRRR_MATRIX_HOST is not set"
    fi
    if [[ -z "$user" ]]; then
        error "SHOUTRRR_MATRIX_USER is not set"
    fi
    if [[ -z "$password" ]]; then
        error "SHOUTRRR_MATRIX_PASSWORD is not set"
    fi

    info "Using host: ${host}"
    info "Using user: ${user}"
    info "Using room: ${room_alias}"

    # Get base URL
    local base_url
    base_url=$(get_base_url "$host")

    # Wait for server to be ready
    wait_for_server "$host" 30

    # Login to get access token
    local access_token
    access_token=$(login "$base_url" "$user" "$password")

    # Extract local part of the alias (e.g., "test" from "#test:localhost")
    local local_alias
    local_alias=$(echo "$room_alias" | sed 's/^#//' | cut -d':' -f1)

    debug "Creating room with alias: ${local_alias}"

    # Create the room
    local response
    response=$(
        curl -s -X POST "${base_url}/_matrix/client/r0/createRoom" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${access_token}" \
            -d "{
                \"room_alias_name\": \"${local_alias}\",
                \"name\": \"Test Room\",
                \"topic\": \"Test room for shoutrrr e2e tests\",
                \"visibility\": \"public\"
            }" 2>&1
    ) || error "Failed to create room"

    # Check for errors in response
    local error_msg
    error_msg=$(echo "$response" | jq -r '.errcode // empty' 2>/dev/null || true)

    if [[ -n "$error_msg" && "$error_msg" != "null" ]]; then
        local error_desc
        error_desc=$(echo "$response" | jq -r '.error // "Unknown error"' 2>/dev/null || true)

        # Room might already exist - that's okay
        if [[ "$error_msg" == "ROOM_ALIAS_EXISTS" ]]; then
            warn "Room alias already exists: ${room_alias}"
            return 0
        fi

        error "Failed to create room: ${error_msg} - ${error_desc}"
    fi

    local room_id
    room_id=$(echo "$response" | jq -r '.room_id' 2>/dev/null) || error "Failed to parse room creation response"

    if [[ -z "$room_id" || "$room_id" == "null" ]]; then
        error "No room_id received from room creation"
    fi

    info "Successfully created room: ${room_alias} (${room_id})"
}

# Run all setup steps in order
setup_all() {
    info "Starting complete Matrix E2E test environment setup..."
    echo ""

    # Check requirements
    check_requirements

    # Step 1: Generate config
    echo ""
    info "=== Step 1/5: Generating configuration ==="
    generate_config

    # Step 2: Start server
    echo ""
    info "=== Step 2/5: Starting server ==="
    start_server

    # Step 3: Create user
    echo ""
    info "=== Step 3/5: Creating test user ==="
    create_user

    # Step 4: Disable rate limiting (requires server and user to be ready)
    echo ""
    info "=== Step 4/5: Disabling rate limiting ==="
    disable_rate_limiting

    # Step 5: Create room
    echo ""
    info "=== Step 5/5: Creating test room ==="
    create_room

    echo ""
    info "=========================================="
    info "Matrix E2E test environment setup complete!"
    info "=========================================="
    info ""
    info "You can now run the E2E tests with:"
    info "  go test -v ./testing/e2e/matrix/..."
    info ""
    info "To stop the server, run:"
    info "  docker compose -f ${COMPOSE_FILE} down"
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
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
            generate-config|start-server|create-user|create-room|setup-all|disable_rate_limiting)
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
        generate-config)
            generate_config
            ;;
        start-server)
            start_server
            ;;
        create-user)
            create_user
            ;;
        create-room)
            create_room
            ;;
        setup-all)
            setup_all
            ;;
        disable_rate_limiting)
            disable_rate_limiting
            ;;
    esac
}

# Run main function with all arguments
main "$@"
