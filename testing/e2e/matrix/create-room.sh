#!/bin/bash
#
# Creates a test room #test:localhost on the Synapse Matrix server.
# Reads credentials from environment variables.
#
# Required environment variables:
#   SHOUTRRR_MATRIX_HOST      - Matrix server host (e.g., localhost:8008)
#   SHOUTRRR_MATRIX_USER      - Matrix username
#   SHOUTRRR_MATRIX_PASSWORD  - Matrix password
#   SHOUTRRR_MATRIX_DISABLE_TLS (optional) - Set to "true" to disable TLS
#

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Error handling function
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

# Info output
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

# Warning output
warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Get the script's directory to locate .env file
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Load .env file from script's directory if it exists
if [[ -f "${SCRIPT_DIR}/.env" ]]; then
    info "Loading environment variables from ${SCRIPT_DIR}/.env"
    source "${SCRIPT_DIR}/.env"
else
    warn "No .env file found in script directory"
fi

# Check required environment variables
check_env_vars() {
    local missing_vars=()
    
    if [[ -z "${SHOUTRRR_MATRIX_HOST:-}" ]]; then
        missing_vars+=("SHOUTRRR_MATRIX_HOST")
    fi
    if [[ -z "${SHOUTRRR_MATRIX_USER:-}" ]]; then
        missing_vars+=("SHOUTRRR_MATRIX_USER")
    fi
    if [[ -z "${SHOUTRRR_MATRIX_PASSWORD:-}" ]]; then
        missing_vars+=("SHOUTRRR_MATRIX_PASSWORD")
    fi
    
    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        error "Missing required environment variables: ${missing_vars[*]}"
    fi
}

# Determine the base URL for Matrix API
# Default to HTTP (TLS disabled) for local Synapse development
get_base_url() {
    local host="$1"
    local disable_tls="${SHOUTRRR_MATRIX_DISABLE_TLS:-true}"
    
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
    
    info "Logging in as ${user}..."
    
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
    
    info "Login response received"
    
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
    
    info "Access token received successfully"
    
    if [[ -z "$access_token" || "$access_token" == "null" ]]; then
        error "No access token received from login"
    fi
    
    info "Login successful"
    echo "$access_token"
}

# Create a room with the specified alias
create_room() {
    local base_url="$1"
    local access_token="$2"
    local room_alias="$3"
    
    info "Creating room with alias ${room_alias}..."
    
    # Extract local part of the alias (e.g., "test" from "#test:localhost")
    local local_alias
    local_alias=$(echo "$room_alias" | sed 's/^#//' | cut -d':' -f1)
    
    local response
    response=$(
        curl -s -X POST "${base_url}/_matrix/client/v3/createRoom" \
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
    
    info "Room created successfully with ID: ${room_id}"
    echo "$room_id"
}

# Main function
main() {
    info "Starting Matrix test room creation..."
    
    # Check environment variables
    check_env_vars
    
    # Get configuration
    local host="${SHOUTRRR_MATRIX_HOST}"
    local user="${SHOUTRRR_MATRIX_USER}"
    local password="${SHOUTRRR_MATRIX_PASSWORD}"
    local room_alias="${SHOUTRRR_MATRIX_ROOM:-#test:localhost}"
    
    info "Matrix host: ${host}"
    info "Matrix user: ${user}"
    info "Room alias: ${room_alias}"
    
    # Get base URL
    local base_url
    base_url=$(get_base_url "$host")
    
    # Login to get access token
    local access_token
    access_token=$(login "$base_url" "$user" "$password")
    
    # Create room
    local room_id
    room_id=$(create_room "$base_url" "$access_token" "$room_alias")
    
    info "Successfully created test room ${room_alias} (${room_id})"
    info "Room creation complete!"
}

# Run main function
main "$@"
