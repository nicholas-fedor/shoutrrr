# Matrix E2E Testing

This directory contains black-box end-to-end (E2E) tests for Shoutrrr's Matrix service.
These tests validate functionality by connecting to a real Matrix Synapse server and sending actual messages.

## Overview

These are **black-box E2E tests** that verify the Matrix service works correctly with a live Matrix server.
The tests make real HTTP connections to the Matrix server's Client-Server API.

## Test Characteristics

- **Package**: `e2e_test`
- **Framework**: [Ginkgo](https://github.com/onsi/ginkgo) + [Gomega](https://github.com/onsi/gomega)
- **Location**: `testing/e2e/matrix`
- **External Calls**: Yes (requires running Matrix server)
- **Requires**: Docker and Docker Compose for local testing

## Test Coverage

### Service Initialization

- Initialization with valid credentials from environment variables
- Error handling for invalid credentials
- Error handling for missing host
- Error handling for missing credentials

### Send Messages

- Sending basic text messages to Matrix rooms
- Sending messages with custom title parameter
- Error handling when client is not initialized

### Service ID

- Verifying correct service ID is returned

### Configuration

- Room alias parsing and handling

## Running the Test Environment

## Quick Setup

Use the provided setup script to automate all setup steps:

```bash
# Run full setup (generate config, start server, create user, create room)
./testing/e2e/matrix/setup.sh

# Or with explicit command
./testing/e2e/matrix/setup.sh setup-all
```

Individual steps can also be run separately:

```bash
./testing/e2e/matrix/setup.sh generate-config  # Generate Docker config
./testing/e2e/matrix/setup.sh start-server     # Start the server
./testing/e2e/matrix/setup.sh create-user      # Create admin user
./testing/e2e/matrix/setup.sh create-room      # Create test room
```

The script automatically finds the `.env` file in its directory.

### Prerequisites

1. **Generate the configuration**

    Run the following to generate a valid config file:

    ```bash
    docker run -it --rm \
    -v ./data:/data \
    --env-file element.env \
    matrixdotorg/synapse:latest \
    generate
    ```

2. **Start the Matrix Server**

   Use the provided `docker-compose.yaml` to start a local Matrix Synapse server:

   ```bash
   cd testing/e2e/matrix
   docker compose up -d
   ```

    > [!Note]
    > A fresh deployment will take a few minutes to startup.
    > Use `docker logs synapse -f`  to watch the logs.

3. **Create a Test User**

   After starting the server, register a test user:

   ```bash
   docker exec synapse \
   register_new_matrix_user \
   -u admin -p admin \
   -a http://localhost:8008  \
   -c /data/homeserver.yaml
   ```

4. **Create a Test Room**

   After creating the admin user, create a test room with the alias `#test:localhost`:

   > [!Tip]
   > An automated script is available to create the test room:
   >
   > ```bash
   > bash ./testing/e2e/matrix/create-room.sh
   > ```
   >
   > The script automatically finds the `.env` file in its directory.
   > Alternatively, you can run it directly if the file is executable:
   >
   > ```bash
   > ./testing/e2e/matrix/create-room.sh
   > ```
   >
   > This provides an easier alternative to the manual curl commands below.

   Alternatively, you can manually create the room using curl commands:

   First, login to get an access token:

   ```bash
   # Login to get access token
   TOKEN=$(curl -s -X POST \
     -H "Content-Type: application/json" \
     -d '{"type": "m.login.password", "identifier": {"type": "m.id.user", "user": "admin"}, "password": "admin"}' \
     http://localhost:8008/_matrix/client/v3/login | jq -r '.access_token')

   # Create a room with the alias #test:localhost
   ROOM_ID=$(curl -s -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name": "Test Room", "room_alias_name": "test"}' \
     http://localhost:8008/_matrix/client/v3/createRoom | jq -r '.room_id')

   echo "Created room: $ROOM_ID with alias #test:localhost"
   ```

   Or alternatively using a Matrix client like Element or any other Matrix client:

   1. Open Element (or any Matrix client)
   2. Connect to your local server at `http://localhost:8008`
   3. Login with the admin user (`admin` / `admin`)
   4. Create a new room with the alias `#test:localhost`

   > [!Important]
   > The test room `#test:localhost` must exist for the E2E tests to pass.
   > The tests expect this room to be available on the local Synapse server.

### Environment Variables

Configure the tests using environment variables:

| Variable                      | Description                    | Default           |
|-------------------------------|--------------------------------|-------------------|
| `SHOUTRRR_MATRIX_URL`         | Full Matrix service URL        | -                 |
| `SHOUTRRR_MATRIX_HOST`        | Matrix server host             | `localhost:8008`  |
| `SHOUTRRR_MATRIX_USER`        | Username for authentication    | -                 |
| `SHOUTRRR_MATRIX_PASSWORD`    | Password or access token       | -                 |
| `SHOUTRRR_MATRIX_ROOM`        | Room alias to send messages to | `#test:localhost` |
| `SHOUTRRR_MATRIX_DISABLE_TLS` | Disable TLS verification       | `false`           |

### Running the Tests

Run all E2E tests:

```bash
go test -v ./testing/e2e/matrix/...
```

Run specific test suites:

```bash
# Run service initialization tests
go test ./testing/e2e/matrix/ -v -run "Service Initialization"

# Run send message tests
go test ./testing/e2e/matrix/ -v -run "Send Messages"
```

### Using .env File

You can also create a `.env` file in the test directory:

```bash
# .env file
SHOUTRRR_MATRIX_HOST=localhost:8008
SHOUTRRR_MATRIX_USER=admin
SHOUTRRR_MATRIX_PASSWORD=admin
SHOUTRRR_MATRIX_ROOM=#test:localhost
SHOUTRRR_MATRIX_DISABLE_TLS=true
```

The test suite automatically loads environment variables from `.env` if present.

## Test Structure

```bash
testing/e2e/matrix/
├── matrix_suite_test.go      # Ginkgo test suite setup and helpers
├── service_e2e_test.go      # E2E test cases
├── docker-compose.yaml       # Local Matrix Synapse server
└── README.md                # This documentation
```

## Test Suite Helpers

### matrixServerURL

Returns the Matrix server URL from environment or builds it from components:

```go
func matrixServerURL() string
```

### matrixRoom

Returns the Matrix room from environment or default:

```go
func matrixRoom() string
```

### buildServiceURL

Builds a complete Matrix service URL with all parameters:

```go
func buildServiceURL() string
```

### loadEnvFile

Loads environment variables from a `.env` file:

```go
func loadEnvFile(filename string)
```

## Test Patterns

Tests use Ginkgo'sDescribe/It syntax:

```go
var _ = ginkgo.Describe("Matrix Service E2E Tests", func() {
    ginkgo.Describe("Service Initialization", func() {
        ginkgo.It("should initialize with valid credentials", func() {
            // Test logic
        })
    })
})
```

## Skipping Tests

Tests automatically skip when environment variables are not configured:

```go
if serviceURL == "" {
    ginkgo.Skip("Matrix server not configured, skipping test...")
}
```

## Docker Compose Configuration

The provided `docker-compose.yaml` starts a Matrix Synapse server with:

- **Image**: `matrixdotorg/synapse:latest`
- **Ports**:
  - `8008` - Client-Server API
  - `8448` - Federation API
- **Server Name**: `localhost`
- **Registration**: Enabled for testing

## Cleanup

To stop and remove the Docker container:

```bash
docker compose down
```

To remove the data volume:

```bash
sudo rm -rf  ./testing/e2e/matrix/data
```

## References

- [Element Synapse GitHub](https://github.com/element-hq/synapse)
- [Element Synapse Documentation](https://element-hq.github.io/synapse/latest/)
- [Matrix Client-Server API](https://matrix.org/docs/spec/client_server/latest)
- [Synapse Docker Image](https://hub.docker.com/r/matrixdotorg/synapse)
- [Synapse Docker Documentation](https://github.com/element-hq/synapse/blob/develop/docker/README.md)

## Related Files

- Matrix Service Implementation: `pkg/services/chat/matrix/service.go`
- Matrix Configuration: `pkg/services/chat/matrix/config.go`
- Integration Tests: `testing/integration/matrix/`
