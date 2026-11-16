// Package pagerduty provides a notification service for PagerDuty incident management.
//
// The PagerDuty service sends alert events to PagerDuty through the Events API v2.
// A trigger event opens an alert. An acknowledge or resolve event changes the alert
// that its dedup key names.
//
// # URL Format
//
// The service URL follows the format:
//
//	pagerduty://[host[:port]]/integration-key[?query]
//
// Where:
//   - host: optional PagerDuty API host (default: events.pagerduty.com)
//   - port: optional port number (default: 443)
//   - integration-key: the 32-character hexadecimal Events API v2 integration key
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - severity: alert severity (critical, error, warning, info; default: error)
//   - source: unique location of affected system (default: default)
//   - action: event type (trigger, acknowledge, resolve; default: trigger)
//   - dedup_key: identifies the alert, at most 255 characters
//   - title: the summary of a trigger event
//   - details: a JSON object sent as the custom details
//   - contexts: links and images, as a JSON array or as link:URL and image:URL entries
//   - client and client_url: the monitoring client that sends the event
//
// # Title and Message
//
// A trigger event's summary is the title when one is set, and the message
// otherwise. PagerDuty uses the summary as the alert's title and limits it to
// 1024 characters. With a title, the message goes to the custom details as
// "message". A message cut to fit the summary goes there in full.
// A trigger event without a title or message fails before any request.
//
// # Event Actions
//
// A trigger event opens an alert, or adds to the open alert with the same dedup
// key. PagerDuty generates a dedup key for a trigger event without one.
//
//	url := "pagerduty:///INTEGRATION_KEY?dedup_key=disk-db1"
//	err := shoutrrr.Send(url, "Disk space critically low")
//
// An acknowledge or resolve event needs the dedup key of the alert it changes.
// A URL or send without one fails before any request. These events carry only
// the routing key, the action, and the dedup key, so the message, the title,
// and the event fields are not sent.
//
//	url := "pagerduty:///INTEGRATION_KEY?action=resolve&dedup_key=disk-db1"
//	err := shoutrrr.Send(url, "")
//
// # Using Parameters in Code
//
//	err := service.Send("db1 is at 95%", &types.Params{
//		"title":     "Disk full",
//		"severity":  "critical",
//		"source":    "db1",
//		"dedup_key": "disk-db1",
//	})
//
// # Command Line Usage
//
//	shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?severity=critical&source=app' -m 'Application crashed'
//
// # Limits and Errors
//
// PagerDuty limits an event to 512 KB, and an encoded event larger than
// 524,288 bytes fails before any request. A status outside 2xx fails the send
// with PagerDuty's error message.
//
// # API Reference
//
// This service uses the PagerDuty Events API v2. For detailed API documentation,
// see: https://developer.pagerduty.com/docs/events-api-v2-overview
package pagerduty
