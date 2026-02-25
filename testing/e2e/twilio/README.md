# Twilio End-to-End Tests

This directory contains end-to-end (e2e) tests for the Twilio SMS service in Shoutrrr.

## Overview

The end-to-end tests validate the complete Twilio SMS functionality by making actual API requests to Twilio's messaging API.

## Test Coverage

### Basic SMS Functionality

- **Basic Test** (`basic_test.go`): Sends a plain text SMS message through the Twilio API
- **Title Test** (`title_test.go`): Sends an SMS with a title prepended to the message body via the `title` query parameter

### Service Validation

- **Service ID Test** (`service_id_test.go`): Verifies that the service correctly identifies itself as `"twilio"` (does not send an SMS)

## Setup Requirements

### Twilio Account Configuration

1. Create or log in to a [Twilio account](https://www.twilio.com/)
2. Obtain the following from your Twilio console:
   - **Account SID** (starts with `AC`)
   - **Auth Token**
   - **Messaging Service SID** (starts with `MG`) or a Twilio phone number as the sender
   - **Recipient phone number** (E.164 format, e.g., `+1234567890`)

### Environment Variables

The tests require the `SHOUTRRR_TWILIO_URL` environment variable. You can provide it via a `.env` file placed in this directory, or by setting it directly in your shell.

#### `.env` file

Create a `.env` file in this directory (`testing/e2e/twilio/.env`):

```bash
SHOUTRRR_TWILIO_URL=twilio://<ACCOUNT_SID>:<AUTH_TOKEN>@<MESSAGING_SID>/<RECIPIENT_PHONE>
```

**Security Note**: Never commit real account credentials to version control. The `.env` file is included in `.gitignore`.

#### Inline environment variable

```bash
SHOUTRRR_TWILIO_URL='twilio://<ACCOUNT_SID>:<AUTH_TOKEN>@<MESSAGING_SID>/<RECIPIENT_PHONE>' \
  go test ./testing/e2e/twilio/ -v
```

### URL Format

```
twilio://<AccountSID>:<AuthToken>@<MessagingSID>/<RecipientPhone>
```

| Component        | Description                                     | Example                            |
| ---------------- | ----------------------------------------------- | ---------------------------------- |
| `AccountSID`     | Twilio Account SID                              | `ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx` |
| `AuthToken`      | Twilio Auth Token                               | `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`   |
| `MessagingSID`   | Messaging Service SID or sender phone number    | `MGxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx` |
| `RecipientPhone` | Destination phone number in E.164 format        | `+1234567890`                        |

## Running the Tests

### Prerequisites

- Go 1.25+
- Valid Twilio account credentials
- `SHOUTRRR_TWILIO_URL` environment variable set (via `.env` or shell)

### Execute E2E Tests

Run all Twilio e2e tests:

```bash
go test ./testing/e2e/twilio/ -v
```

Run a specific test:

```bash
# Basic SMS test
go test ./testing/e2e/twilio/ -v -args -ginkgo.focus="Basic"

# Title test
go test ./testing/e2e/twilio/ -v -args -ginkgo.focus="Title"

# Service ID test
go test ./testing/e2e/twilio/ -v -args -ginkgo.focus="Service ID"
```

### Test Behavior

- Tests will **skip** (not fail) if `SHOUTRRR_TWILIO_URL` is not set
- Two of the three tests send actual SMS messages — monitor your Twilio account and recipient device during test runs
- The service ID test only validates the service identifier and does not send any messages

### Expected Output

A successful run produces 3 passing specs. The basic and title tests each deliver one SMS to the configured recipient phone number (2 messages total).

## Test Structure

```
testing/e2e/twilio/
├── .env               # Environment variables (not committed)
├── README.md          # This file
├── suite_test.go      # Test suite setup and .env file loading
├── basic_test.go      # Basic SMS sending
├── title_test.go      # SMS with title parameter
└── service_id_test.go # Service identifier validation
```
