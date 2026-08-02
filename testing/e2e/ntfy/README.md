# ntfy End-to-End Tests

This directory contains end-to-end (e2e) tests for the ntfy service in Shoutrrr.

## Overview

The end-to-end tests validate the complete ntfy notification functionality by sending messages to a real ntfy server and verifying they are received.

## Test Coverage

### Basic Message Functionality

- Plain text messages
- Empty messages
- Message delivery verification via ntfy API

### Message Features

- Title
- Priority levels
- Tags/emojis
- Markdown formatting
- Click actions
- Attachments
- Action buttons
- Delayed delivery
- Email notifications
- Icon
- Filename
- Multiple tags
- Combined parameters

### Configuration

- Default configuration
- URL-based configuration
- Priority from URL
- Markdown from URL
- Tags from URL
- Title from URL
- DisableTLS from URL

### Authentication

- Basic authentication with username and password
- Username-only authentication

### TLS

- HTTPS connections (default)
- HTTP connections with DisableTLS

## Setup Requirements

### ntfy Server

A local ntfy server is required for e2e testing. The provided `docker-compose.yaml` sets up a local ntfy server.

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Linux OS

### Quick Start

1. Start the ntfy server and wait for it to be ready using the setup script:

    ```bash
    cd testing/e2e/ntfy
    ./setup.sh setup-all
    ```

2. Run the e2e tests:

    ```bash
    go test -v ./testing/e2e/ntfy/...
    ```

### ntfy Server Details

| Field   | Value                             |
|---------|-----------------------------------|
| Host    | localhost                         |
| Port    | 8080                              |
| API URL | <http://localhost:8080>           |
| Health  | <http://localhost:8080/v1/health> |

### Stopping the Server

To stop the ntfy server:

```bash
cd testing/e2e/ntfy
docker compose down
```

## Environment Variables

A `.env` file is provided with preconfigured defaults.

**Default values:**
**Security Note**: Never commit real credentials to version control. The `.env` file is in `.gitignore`.

```bash
# Required: ntfy server URL
SHOUTRRR_NTFY_URL=ntfy://localhost:8080/shoutrrr-e2e-test

# Optional: ntfy server authentication
# SHOUTRRR_NTFY_USERNAME=
# SHOUTRRR_NTFY_PASSWORD=

# Optional: TLS configuration
# SHOUTRRR_NTFY_DISABLE_TLS=false
```

## Running the Tests

### Test Execution Prerequisites

- Go 1.25+
- Running ntfy server
- `.env` file with required environment variables

### Execute E2E Tests

Run all e2e tests:

```bash
go test ./testing/e2e/ntfy/ -v
```

Run specific test files:

```bash
# Test basic functionality
go test ./testing/e2e/ntfy/ -v -args -ginkgo.focus="basic"

# Test configuration
go test ./testing/e2e/ntfy/ -v -args -ginkgo.focus="config"

# Test message features
go test ./testing/e2e/ntfy/ -v -args -ginkgo.focus="message"

# Test authentication
go test ./testing/e2e/ntfy/ -v -args -ginkgo.focus="auth"

# Test TLS
go test ./testing/e2e/ntfy/ -v -args -ginkgo.focus="TLS"
```

### Test Behavior

- Tests will skip if the ntfy server is not available
- Tests include delays between executions to respect ntfy rate limits
- All tests send actual messages to the ntfy server
- Messages are verified via the ntfy API polling endpoint
- Each test includes an "E2E Test:" prefix to identify test messages

### Message Verification

The e2e tests verify message delivery by polling the ntfy API:

1. A message is sent to a unique topic
2. The test polls `GET /<topic>/json?poll=1&message=<message>` to retrieve the message
3. The response is parsed as JSON and verified against expected values

## Test Structure

```bash
testing/e2e/ntfy/
├── .env                 # Environment variables (not committed)
├── .gitignore           # Git ignore rules
├── docker-compose.yaml  # ntfy server setup (Docker Compose)
├── setup.sh             # Setup script for starting/stopping the server
├── suite_test.go        # Test suite setup and configuration
├── basic_test.go        # Basic message sending functionality
├── config_test.go       # Configuration tests
├── message_features_test.go # Message features (title, priority, tags, etc.)
├── authentication_test.go   # Authentication tests
├── tls_test.go          # TLS connection tests
└── README.md            # This file
```

### Test Organization

Each test file focuses on a specific feature category:

- **Basic Tests** (`basic_test.go`): Core message sending functionality
- **Config Tests** (`config_test.go`): Configuration options and URL parsing
- **Message Features Tests** (`message_features_test.go`): Rich message features
- **Authentication Tests** (`authentication_test.go`): Basic auth handling
- **TLS Tests** (`tls_test.go`): TLS/HTTP connection handling

## ntfy URL Examples

### Basic Connection

```bash
SHOUTRRR_NTFY_URL=ntfy://localhost:8080/shoutrrr-e2e-test
```

### With Authentication

```bash
SHOUTRRR_NTFY_URL=ntfy://username:password@localhost:8080/shoutrrr-e2e-test
```

### With Priority

```bash
SHOUTRRR_NTFY_URL=ntfy://localhost:8080/shoutrrr-e2e-test?priority=5
```

### With Tags

```bash
SHOUTRRR_NTFY_URL=ntfy://localhost:8080/shoutrrr-e2e-test?tags=warning,skull
```

### With HTTP (DisableTLS)

```bash
SHOUTRRR_NTFY_URL=ntfy://localhost:8080/shoutrrr-e2e-test
SHOUTRRR_NTFY_DISABLE_TLS=true
```

## Resources

- ntfy Documentation: <https://docs.ntfy.sh>
- ntfy Docker Image: <https://hub.docker.com/r/binwiederhier/ntfy>
- ntfy GitHub: <https://github.com/binwiederhier/ntfy>
