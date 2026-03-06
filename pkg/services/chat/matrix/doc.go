// Package matrix provides Matrix protocol support for Shoutrrr notifications.
//
// This package implements the Shoutrrr service interface for sending notifications
// via Matrix, an open network for secure, decentralized communication. It supports
// authentication via username/password or access tokens, multiple room targeting,
// and TLS configuration.
//
// # URL Format
//
//	matrix://user:password@host:port/[?rooms=!roomID1[,roomAlias2]][&disableTLS=yes]
//
// The URL scheme "matrix" identifies this service in Shoutrrr configurations.
// Authentication credentials and target rooms are specified through the URL.
//
// # Authentication
//
// The service supports two authentication methods:
//
//   - Password Login: When both user and password are provided, the service
//     attempts m.login.password authentication flow.
//   - Access Token: When no user is specified, the password field is treated
//     as an access token, allowing manual token authentication.
//
// # Room Configuration
//
// Rooms can be specified using room IDs (prefixed with '!') or room aliases.
// If no rooms are specified, the service sends messages to all joined rooms.
// Room aliases are resolved through directory lookup. Note that unescaped '#'
// characters should be URL-encoded as '%23' to avoid being treated as URL
// fragments.
//
// # TLS Configuration
//
// TLS is enabled by default. For servers without TLS, set disableTLS=yes
// to use HTTP instead of HTTPS for API calls.
//
// # Main Components
//
// Service (matrix.go)
//
// Implements the Shoutrrr service interface with Initialize and Send methods.
// Manages the Matrix client lifecycle and coordinates message delivery.
//
// Config (matrix_config.go)
//
// Defines configuration fields: User, Password, DisableTLS, Host, Rooms, Title.
// Handles URL parsing and query parameter resolution.
//
// Client (matrix_client.go)
//
// HTTP client for Matrix API interactions including authentication,
// room management, and message sending.
//
// API Types (matrix_api.go)
//
// Request and response types for Matrix protocol operations including
// login flows, room operations, and message sending.
package matrix
