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
//	twilio://accountSID:authToken@fromNumber/toNumber1[/toNumber2/...][?query]
//
// Where:
//   - accountSID: Twilio Account SID (required)
//   - authToken: Twilio Auth Token, or the API key secret with apikey (required)
//   - fromNumber: sender phone number in E.164 format, alphanumeric sender ID, or Messaging Service SID (required)
//   - toNumber: recipient phone number(s) in E.164 format (required)
//   - query: configuration parameters
//
// # Basic Usage
//
//	import "github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
//
//	service := &twilio.Service{}
//	err := service.Initialize(url, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Hello via SMS!", nil)
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - title: notification title, on its own line before the message
//   - apikey: API key SID, SK followed by 32 hexadecimal characters
//
// # API Keys
//
// With apikey set, requests authenticate with the API key SID and the secret
// in the authToken position. The account SID still names the account.
//
// # Senders
//
// A fromNumber of MG followed by 32 hexadecimal characters is a Messaging
// Service SID, sent as the MessagingServiceSid parameter. Any other sender is
// sent as From. A sender that holds a letter, such as an alphanumeric sender
// ID, is sent as written.
//
// # Phone Number Normalization
//
// Common formatting characters (spaces, dashes, parentheses, dots) are
// stripped from recipient numbers and from a sender without letters.
//
// # Message Length
//
// Twilio accepts a body of up to 1600 characters, title included. A longer
// body, or an empty one, fails before any request.
//
// # Important Notes
//
//   - Multiple recipients can be specified by adding additional path segments
//   - The To and From numbers must not be the same (Twilio rejects such requests)
//   - Messaging Service SIDs bypass the To==From validation
//   - Authentication uses HTTP Basic Auth
//   - Requests are sent as application/x-www-form-urlencoded POST to the Twilio Messages API
package twilio
