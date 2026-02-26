// Package testutils provides testing utilities for shoutrrr services and types.
//
// This package contains helper functions, mock implementations, and test utilities
// used throughout the shoutrrr test suite. It is designed to simplify common testing
// patterns and provide reusable components for testing notification services.
//
// # Main Components
//
// Configuration Testing (config.go)
//
// Provides functions to test ServiceConfig implementations including validation
// of query values, default values, enum counts, and field counts.
//
// Service Testing (service.go)
//
// Offers utilities for testing Service implementations, including validation
// of parameter handling and error conditions.
//
// I/O Utilities
//
// Various utilities for mocking and testing I/O operations:
//
//   - failwriter.go: Simulates write failures for testing error handling
//   - iofaker.go: Provides fake ReadWriter implementations
//   - textconfaker.go: Creates fake textproto.Conn for protocol testing
//
// HTTP Mocking (mockclientservice.go)
//
// Defines the MockClientService interface for mocking HTTP client behavior
// in integration tests.
//
// Test Helpers
//
//   - must.go: Provides "must" style functions (URLMust, JSONRespondMust)
//     that fail tests on error rather than returning errors
//   - logging.go: Creates test loggers that output to GinkgoWriter
//   - eavesdropper.go: Interface for capturing connection conversations
package testutils
