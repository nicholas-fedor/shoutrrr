// Package signal provides functionality to send notifications via Signal Messenger
// through REST API servers that wrap the signal-cli command-line interface.
//
// This service supports sending text messages and base64-encoded attachments to
// individual phone numbers, Signal usernames (u:name), and Signal groups.
// Optional text_mode=styled enables markup in the message body. A title is
// prepended to the body because Signal has no separate title field.
// Authentication supports both HTTP Basic Auth and Bearer tokens.
//
// It requires a Signal API server (such as signal-cli-rest-api or secured-signal-api)
// to be running and configured with a registered Signal account.
//
// URL format: signal://[user:pass@]host:port/source_phone/recipient1/recipient2
// URL format: signal://host:port/source_phone/recipient1?token=apikey&textmode=styled
//
// For setup instructions and API server options, see the service documentation.
package signal
