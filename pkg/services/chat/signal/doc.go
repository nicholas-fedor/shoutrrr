// Package signal provides functionality to send notifications via Signal Messenger
// through REST API servers that wrap the signal-cli command-line interface.
//
// This service supports sending text messages and base64-encoded attachments to
// individual phone numbers and Signal groups. Authentication supports both HTTP
// Basic Auth and Bearer tokens for compatibility with different API servers.
//
// It requires a Signal API server (such as signal-cli-rest-api or secured-signal-api)
// to be running and configured with a registered Signal account.
//
// URL format: signal://[user:pass@]host:port/source_phone/recipient1/recipient2
// URL format: signal://host:port/source_phone/recipient1/recipient2?token=apikey
//
// For setup instructions and API server options, see the service documentation.
package signal
