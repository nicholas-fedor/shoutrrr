// Package testutils provides testing utilities for Shoutrrr services and types.
//
// This package contains helper functions, mock implementations, and test utilities
// used throughout the Shoutrrr test suite. It is designed to simplify common testing
// patterns and provide reusable components for testing notification services.
//
// # Main Components
//
// Configuration Testing (config.go)
//
// Provides functions to test ServiceConfig implementations including validation
// of query values, default values, enum counts, and field counts.
//
// Helper Functions (helpers.go)
//
// Contains various test helper utilities:
//
//   - TestLogger: Creates test loggers that output to GinkgoWriter
//   - URLMust: Parses URLs and fails tests on error
//   - JSONRespondMust: Creates HTTP mock responders
//   - TestServiceSetInvalidParamValue: Tests service parameter validation
//
// Faking/Mocking Utilities (fakers.go)
//
// Provides utilities for mocking I/O operations and network connections:
//
//   - Eavesdropper: Interface for capturing connection conversations
//   - CreateFailWriter: Simulates write failures for testing error handling
//   - CreateTextConFaker: Creates fake textproto.Conn for protocol testing
//
// HTTP Mocking (mock_client_service.go)
//
// Defines the MockClientService interface for mocking HTTP client behavior
// in integration tests.
package testutils
