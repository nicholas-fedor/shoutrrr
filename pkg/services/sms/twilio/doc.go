// Package twilio provides a notification service for sending SMS messages via the Twilio API.
//
// The twilio service allows sending SMS messages to phone numbers using the Twilio
// messaging API. It supports standard notification features including message titles
// and configurable sender/recipient phone numbers.
//
// # URL Format
//
// The service URL follows the format:
//
//	twilio://accountSID:authToken@fromNumber/toNumber[?query]
//
// Where:
//   - accountSID: Twilio Account SID (required)
//   - authToken: Twilio Auth Token (required)
//   - fromNumber: sender phone number in E.164 format (required)
//   - toNumber: recipient phone number in E.164 format (required)
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - title: notification title, prepended to the message body
package twilio
