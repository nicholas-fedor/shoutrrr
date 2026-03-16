# Matrix Integration Tests

This directory contains black-box integration tests for the Matrix service in Shoutrrr.
The tests validate Matrix service functionality without making external calls, using the "dummy" URL pattern to bypass client initialization.

## Overview

These are **black-box integration tests** that verify the Matrix service's public API and behavior without connecting to an actual Matrix server. The tests use the special `dummy://` URL pattern to prevent client initialization while still testing configuration parsing and service behavior.

## Test Characteristics

- **Package**: `matrix_test`
- **Framework**: [Testify](https://github.com/stretchr/testify)
- **Location**: `testing/integration/matrix`
- **External Calls**: None (tests bypass client initialization)
- **Mocking**: Uses dummy URLs to avoid external connections

## Test Coverage

### Service Initialization

- Valid URL parsing with access token authentication
- Valid URL parsing with password authentication
- URL parsing with custom title parameter
- URL parsing with TLS disable option
- Error handling for missing host
- Error handling for missing credentials

### Send Method

- Behavior with uninitialized client (dummy URL)
- Error handling when client is not initialized
- Empty message handling
- Service ID retrieval

### Configuration

- URL parsing and configuration setting
- Special characters in passwords
- Port specification in URLs
- Query parameter handling (title, disableTLS)
- Room alias configuration

## Running the Tests

Run all integration tests:

```bash
go test -v ./testing/integration/matrix/...
```

Run specific test files:

```bash
go test ./testing/integration/matrix/ -run TestService -v
go test ./testing/integration/matrix/ -run TestConfig -v
```

Run with coverage:

```bash
go test -cover ./testing/integration/matrix/...
```

## Test Structure

```bash
testing/integration/matrix/
├── utils_test.go      # Helper functions and testLogger implementation
├── service_test.go   # Service initialization and Send method tests
├── config_test.go    # Configuration parsing and URL handling tests
└── README.md         # This documentation
```

## Helper Functions

### testLogger

A minimal logger implementation for testing that satisfies the `types.StdLogger` interface:

```go
type testLogger struct {
    messages []string
}
```

Methods:

- `Print(v ...any)` - Logs a message
- `Printf(format string, v ...any)` - Logs a formatted message
- `Println(v ...any)` - Logs a message with newline

### createTestService

Creates a Matrix service with the given URL string:

```go
func createTestService(t *testing.T, serviceURL string) *matrix.Service
```

Uses the "dummy" URL special case to avoid actual client initialization, allowing tests to verify configuration handling without network connections.

## Test Patterns

Tests use table-driven testing for comprehensive coverage:

```go
func TestServiceInitializeWithValidURL(t *testing.T) {
    tests := []struct {
        name       string
        serviceURL string
    }{
        // Test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test logic...
        })
    }
}
```

## Environment

No environment variables are required for integration tests since they do not connect to external services.

## Related Files

- Matrix Service Implementation: `pkg/services/chat/matrix/service.go`
- Matrix Configuration: `pkg/services/chat/matrix/config.go`
- Matrix Types: `pkg/services/chat/matrix/types.go`
