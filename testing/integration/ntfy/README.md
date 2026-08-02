# Ntfy Integration Tests

This directory contains integration tests for the Ntfy push notification service in Shoutrrr.
The tests validate ntfy functionality by mocking HTTP requests to the ntfy API, ensuring feature parity with the ntfy.sh service.

## Test Coverage

### Configuration Options

- URL parsing for `ntfy://` scheme
- Topic path handling (simple and nested paths)
- Username/password authentication from URL
- Query parameters (priority, tags, markdown, cache, firebase, title, click, attach, actions, delay, email, icon, filename)
- Default values (https scheme, ntfy.sh host, default priority)
- DisableTLS option (forces HTTP scheme)
- DisableTLSVerification option

### Message Sending

- Basic message sending
- Empty message handling
- Title via URL query parameter
- Title via params map
- Priority configuration
- Tags configuration
- Markdown formatting
- Click action links
- Attachment URLs
- Action buttons
- Delayed delivery
- Email notifications
- Icon URLs
- Filename for attachments
- Unicode message support
- Basic authentication

### Error Handling

- HTTP error responses (400, 401, 403, 404, 500)
- Network errors (connection refused, timeout)
- Malformed API error responses
- Empty error body handling

## Running the Tests

### Mocked Integration Tests (Default)

Run all integration tests with mocked ntfy API responses:

```bash
go test ./testing/integration/ntfy/ -v
```

Or run specific test categories:

```bash
# Run specific test categories
go test ./testing/integration/ntfy/ -run TestServiceInitialization -v
go test ./testing/integration/ntfy/ -run TestSend -v
go test ./testing/integration/ntfy/ -run 'TestSendWith.*Error|TestSendWith(400|401|403|404|500)|TestSendWithTimeout|TestSendWith(Nil|Empty)Params' -v
```

## Test Structure

The test suite is organized as a flat directory structure with individual test files for each feature category:

```bash
testing/integration/ntfy/
├── utils_test.go        # Helper functions and mock implementations
├── config_test.go       # Configuration and URL parsing tests
├── send_test.go         # Message sending tests
├── errors_test.go       # Error handling tests
└── README.md            # This documentation
```

### Test Organization

Each test file contains independent black-box tests for specific ntfy service behaviors.
Tests validate that the service correctly interacts with external APIs without inspecting internal data structures:

- **Config Tests** (`config_test.go`): Service configuration application in external API requests
- **Send Tests** (`send_test.go`): Service handling of various message types and external API responses
- **Error Tests** (`errors_test.go`): Service error handling when external APIs return failures
