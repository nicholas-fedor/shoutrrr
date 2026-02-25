# Twilio Integration Tests

This directory contains integration tests for the Twilio SMS service in Shoutrrr.
The tests validate Twilio SMS functionality by mocking HTTP requests to Twilio's API, ensuring feature parity with Twilio's Messages API.

## Test Coverage

### API Compliance

- API URL format validation (`https://api.twilio.com/2010-04-01/Accounts/{SID}/Messages.json`)
- Valid URL format parsing
- Payload structure (form-encoded: `Body`, `To`, `From` fields)
- Basic Auth header construction
- Content-Type header (`application/x-www-form-urlencoded`)
- HTTP method (POST)
- HTTPS requirement
- MessagingServiceSid support (replaces `From` field)

### Configuration

- Multiple recipient handling
- Phone number normalization (dashes, parentheses, dots)
- MessagingServiceSid configuration
- Title parameter (prepended to message body)
- Empty title handling

### Send Functionality

- Basic message sending
- Multiple recipient delivery
- Title via URL query parameter
- Title via params map
- Unicode message support
- Empty message handling
- Service initialization

### Error Handling

- HTTP error responses (400, 401, 403, 404, 500)
- Network errors
- Twilio API error parsing (code and message extraction)
- Malformed API error responses
- Partial failure (multi-recipient with mixed results)
- Empty error body handling

## Running the Tests

### Mocked Integration Tests (Default)

Run all integration tests with mocked Twilio API responses:

```bash
go test ./testing/integration/twilio/ -v
```

Or run specific test categories:

```bash
go test ./testing/integration/twilio/ -run TestAPICompliance -v
go test ./testing/integration/twilio/ -run TestConfig -v
go test ./testing/integration/twilio/ -run TestSend -v
go test ./testing/integration/twilio/ -run TestErrors -v
```

## Test Structure

The test suite is organized as a flat directory structure with individual test files for each feature category:

```bash
testing/integration/twilio/
├── api_compliance_test.go # API compliance and Twilio webhook specification validation
├── config_test.go         # Configuration options (phone normalization, recipients, title, MessagingServiceSid)
├── errors_test.go         # Error handling and edge cases (HTTP errors, API errors, partial failures)
├── send_test.go           # Send functionality (basic send, multi-recipient, title, unicode)
├── utils_test.go          # Shared test utilities (mock HTTP client, helpers)
└── README.md              # This documentation
```

### Test Organization

Each test file contains independent black-box tests for specific Twilio service behaviors.
Tests validate that the service correctly interacts with external APIs without inspecting internal data structures:

- **API Compliance Tests** (`api_compliance_test.go`): Service behavior with different Twilio API response scenarios
- **Config Tests** (`config_test.go`): Service configuration application in external API requests
- **Send Tests** (`send_test.go`): Service handling of various message types and external API responses
- **Error Tests** (`errors_test.go`): Service error handling when external APIs return failures
