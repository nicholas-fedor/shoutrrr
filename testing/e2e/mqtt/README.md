# MQTT End-to-End Tests

This directory contains end-to-end (e2e) tests for the MQTT service in Shoutrrr.

## Overview

The end-to-end tests validate the complete MQTT notification functionality by connecting to a real MQTT broker and publishing messages to topics.

## Test Coverage

### Basic Functionality

- Plain text messages with various content types
- Basic connection and message publishing

### QoS (Quality of Service) Levels

- QoS 0 (at most once) - Fire and forget
- QoS 1 (at least once) - Acknowledged delivery
- QoS 2 (exactly once) - Four-part handshake
- QoS configuration via URL parameters and runtime params

### Retained Messages

- Non-retained messages (default)
- Retained message publishing
- Replacing retained messages

### Authentication

- Anonymous connections (no auth)
- Username/password authentication
- Failed authentication handling

### TLS Connections

- mqtts:// scheme for TLS connections
- Custom TLS port (8883)
- TLS failure scenarios

### Topic Formats

- Single-level topics (e.g., `test/topic`)
- Nested topics (e.g., `home/alerts/notifications`)
- Single-level wildcards (e.g., `sensors/+/temperature`)
- Multi-level wildcards (e.g., `home/#`)
- Topics with special characters (URL-encoded)

## Test Environment Setup

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Linux OS (for generating TLS certs)

### Quick Start

A pre-configured, local EMQX deployment is provided, which includes bootstrapped authentication.

Before running the tests, you will need to use the provided `generate.sh` script to generate self-signed TLS certificates:

```bash
cd testing/e2e/mqtt/certs
chmod +x generate.sh
./generate.sh
```

To start EMQX, run the following commands:

```bash
cd testing/e2e/mqtt
docker-compose up -d
```

This starts an EMQX broker with:

- Pre-configured MQTT test user
- TLS enabled on port 8883
- Anonymous access disabled

The EMQX Dashboard will be available at <http://localhost:18083>.

#### Dashboard Credentials

| Field     | Value     |
| --------- | --------- |
| Username  | `admin`   |
| Password  | `public`  |

### Pre-Configuring Authentication

The docker-compose setup includes a pre-configured test user that is automatically bootstrapped when the container starts.
This `shoutrrr` user is what is used as a target for sending MQTT messages, rather than the `admin` user that's used for webui access.

#### MQTT Test Credentials

| Field    | Value      |
|----------|------------|
| Username | `shoutrrr` |
| Password | `testing`  |

These credentials are used for MQTT connections (ports 1883 and 8883).

#### Bootstrap Configuration

The test user is defined in `auth-built-in-db-bootstrap.json`, which is mounted into the EMQX container and loaded on startup.

The bootstrap file contains:

- User ID (username)
- Password hash (SHA256 with salt prefix)
- Salt value
- Superuser status

## Setup Details

### EMQX Ports

| Port  | Description                 |
|-------|-----------------------------|
| 1883  | Standard MQTT (unencrypted) |
| 8883  | MQTT over TLS               |
| 8083  | WebSocket MQTT              |
| 8084  | WebSocket MQTT over TLS     |
| 18083 | Dashboard                   |

### EMQX Authentication Configuration

The docker-compose uses the latest (v6) EMQX authentication syntax:

```yaml
environment:
  # Enable authentication with built-in database
  - EMQX_AUTHENTICATION__1__ENABLE=true
  - EMQX_AUTHENTICATION__1__BACKEND=built_in_database
  - EMQX_AUTHENTICATION__1__MECHANISM=password_based
  - EMQX_AUTHENTICATION__1__USER_ID_TYPE=username
  - EMQX_AUTHENTICATION__1__PASSWORD_HASH_ALGORITHM={"name":"sha256","salt_position":"prefix"}
  - EMQX_AUTHENTICATION__1__BOOTSTRAP_FILE=/opt/emqx/etc/auth-built-in-db-bootstrap.json
  - EMQX_AUTHENTICATION__1__BOOTSTRAP_TYPE=hash
  # Disable anonymous access
  - EMQX_ALLOW_ANONYMOUS=false
```

### Generating New Password Hashes

A Python script [`shoutrrr-user-pw-hash.py`](shoutrrr-user-pw-hash.py) is provided for generating new password hashes:

```bash
python3 shoutrrr-user-pw-hash.py
```

Output example:

```bash
Salt: 28005eec628f27a17a5dd7fab3b659b4
Password hash: dff95fabfde15b11706761c4c890f01291b60d5a624de7c2dcdd2643a1ba61bd
```

To add a new user:

1. Edit the script to set your desired password
2. Run the script to generate salt and hash
3. Add a new entry to [`auth-built-in-db-bootstrap.json`](auth-built-in-db-bootstrap.json):

    ```json
    {
      "user_id": "new_username",
      "password_hash": "<generated_hash>",
      "salt": "<generated_salt>",
      "is_superuser": false
    }
    ```

4. Restart the EMQX container to load the new user

### TLS Setup

For TLS tests, generate certificates before starting the broker:

```bash
cd testing/e2e/mqtt/certs
chmod +x generate.sh
./generate.sh
```

This generates self-signed certificates valid for `localhost` and `127.0.0.1`.

**Note on TLS:** The TLS tests require proper certificate validation. For development with self-signed certificates, set `SHOUTRRR_MQTT_TLS_SKIP_VERIFY=true`.

### Environment Variables

A template `.env` file is provided with preconfigured defaults that should work if following the quick setup instructions.

**Default values (matching pre-configured setup):**
**Security Note**: Never commit real credentials to version control. The `.env` file is in `.gitignore`.

```bash
# Required: MQTT broker URL
SHOUTRRR_MQTT_URL=mqtt://localhost:1883/shoutrrr/test

# MQTT authentication (pre-configured user)
SHOUTRRR_MQTT_USERNAME=shoutrrr
SHOUTRRR_MQTT_PASSWORD=testing

# TLS URL
SHOUTRRR_MQTT_TLS_URL=mqtts://localhost:8883/shoutrrr/test

# Skip TLS certificate verification (for self-signed certs)
SHOUTRRR_MQTT_TLS_SKIP_VERIFY=true

# Authentication is required (anonymous access disabled)
SHOUTRRR_MQTT_AUTH_REQUIRED=true
```

### URL Format

The MQTT URL format is:

```uri
mqtt://[username:password@]host[:port]/topic[?options]
```

URL options include:

- `qos`: QoS level (0, 1, or 2)
- `retained`: Whether to retain the message (true/false)
- `clientid`: Client ID for the connection

## Running the Tests

### Prerequisites

- Go 1.25+
- Running MQTT broker
- `.env` file with required environment variables

### Execute E2E Tests

Run all e2e tests:

```bash
go test ./testing/e2e/mqtt/ -v
```

Run specific test files:

```bash
# Test basic functionality
go test ./testing/e2e/mqtt/ -v -args -ginkgo.focus="basic"

# Test QoS levels
go test ./testing/e2e/mqtt/ -v -args -ginkgo.focus="QoS"

# Test authentication
go test ./testing/e2e/mqtt/ -v -args -ginkgo.focus="Authentication"

# Test TLS connections
go test ./testing/e2e/mqtt/ -v -args -ginkgo.focus="TLS"

# Test topics
go test ./testing/e2e/mqtt/ -v -args -ginkgo.focus="Topic"
```

### Test Behavior

- Tests will skip if required environment variables are missing
- Tests include delays between executions to respect broker rate limits
- All tests send actual messages to your MQTT broker - monitor your broker during test runs
- Each test includes an "E2E Test:" prefix to identify test messages

### Expected Output

Successful test run will show messages being published to your MQTT broker. Use an MQTT client (like `mosquitto_sub`) to verify messages are being received:

```bash
# Subscribe to test topic (with authentication)
mosquitto_sub -t "test/#" -v -u shoutrrr -P testing

# Or for TLS
mosquitto_sub -t "test/#" -v --cafile certs/ca.pem -p 8883 -u shoutrrr -P testing
```

## Test Structure

```bash
testing/e2e/mqtt/
├── .env                                                      # Environment variables (not committed)
├── docker-compose.yaml                    # EMQX broker setup (Docker Compose)
├── auth-built-in-db-bootstrap.json   # Pre-configured MQTT user
├── shoutrrr-user-pw-hash.py             # Password hash generator script
├── suite_test.go                                      # Test suite setup and configuration
├── basic_test.go                                     # Basic message sending functionality
├── qos_test.go                                        # QoS level tests (0, 1, 2)
├── retained_test.go                               # Retained message tests
├── auth_test.go                                      # Authentication tests
├── tls_test.go                                          # TLS connection tests
├── topics_test.go                                   # Topic variations tests
├── certs/                                                  # TLS certificates directory
│   ├── generate.sh                                 # Certificate generation script
│   ├── ca.pem                                         # CA certificate
│   ├── cert.pem                                      # Server certificate
│   └── key.pem                                       # Server private key
└── README.md                                    # This file
```

### Test Organization

Each test file focuses on a specific feature category:

- **Basic Tests** (`basic_test.go`): Core message sending functionality
- **QoS Tests** (`qos_test.go`): Quality of Service level testing
- **Retained Tests** (`retained_test.go`): Retained message handling
- **Auth Tests** (`auth_test.go`): Username/password authentication
- **TLS Tests** (`tls_test.go`): Secure TLS connections
- **Topic Tests** (`topics_test.go`): Topic format variations and wildcards

## MQTT URL Examples

### Basic Connection

```bash
SHOUTRRR_MQTT_URL=mqtt://localhost:1883/test
```

### With Authentication

```bash
SHOUTRRR_MQTT_URL=mqtt://shoutrrr:testing@localhost:1883/test
```

### With QoS

```bash
SHOUTRRR_MQTT_URL=mqtt://localhost:1883/test?qos=1
```

### With TLS

```bash
SHOUTRRR_MQTT_TLS_URL=mqtts://localhost:8883/test
```

### Retained Message

```bash
SHOUTRRR_MQTT_URL=mqtt://localhost:1883/alerts?retained=true
```

### Using EMQX Dashboard

Access the EMQX dashboard at <http://localhost:18083> with the default credentials `admin:public` to monitor messages and connections during testing.

## Manual Docker Setup (Advanced)

If you prefer not to use docker-compose, you can run EMQX manually with the Docker CLI:

```bash
docker run -d --name emqx \
  -p 1883:1883 \
  -p 8883:8883 \
  -p 8083:8083 \
  -p 8084:8084 \
  -p 18083:18083 \
  -e EMQX_NAME=emqx \
  -e EMQX_HOST=127.0.0.1 \
  -e EMQX_DASHBOARD__DEFAULT_USERNAME=admin \
  -e EMQX_DASHBOARD__DEFAULT_PASSWORD=public \
  -e EMQX_ALLOW_ANONYMOUS=false \
  -v $(pwd)/auth-built-in-db-bootstrap.json:/opt/emqx/etc/auth-built-in-db-bootstrap.json:ro \
  -e EMQX_AUTHENTICATION__1__ENABLE=true \
  -e EMQX_AUTHENTICATION__1__BACKEND=built_in_database \
  -e EMQX_AUTHENTICATION__1__MECHANISM=password_based \
  -e EMQX_AUTHENTICATION__1__USER_ID_TYPE=username \
  -e EMQX_AUTHENTICATION__1__PASSWORD_HASH_ALGORITHM='{"name":"sha256","salt_position":"prefix"}' \
  -e EMQX_AUTHENTICATION__1__BOOTSTRAP_FILE=/opt/emqx/etc/auth-built-in-db-bootstrap.json \
  -e EMQX_AUTHENTICATION__1__BOOTSTRAP_TYPE=hash \
  emqx/emqx:latest
```

## Resources

- EMQX Docker Image: <https://hub.docker.com/r/emqx/emqx>
- EMQX Enterprise Documentation: <https://docs.emqx.com/en/emqx/latest/>
