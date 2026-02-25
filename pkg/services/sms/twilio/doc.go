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
//   - authToken: Twilio Auth Token (required)
//   - fromNumber: sender phone number in E.164 format, or Messaging Service SID (required)
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
//   - title: notification title, prepended to the message body
//
// # Messaging Service SID
//
// If the fromNumber starts with "MG", it is treated as a Twilio Messaging Service SID
// and sent as the MessagingServiceSid parameter instead of From.
//
// # Phone Number Normalization
//
// Phone numbers are automatically normalized by stripping common formatting characters
// (spaces, dashes, parentheses, dots), leaving only digits and a leading '+'.
//
// # Important Notes
//
//   - Multiple recipients can be specified by adding additional path segments
//   - The To and From numbers must not be the same (Twilio rejects such requests)
//   - Messaging Service SIDs bypass the To==From validation
//   - Authentication uses HTTP Basic Auth with AccountSID and AuthToken
//   - Requests are sent as application/x-www-form-urlencoded POST to the Twilio Messages API
package twilio
