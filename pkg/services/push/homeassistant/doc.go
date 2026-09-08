// Package homeassistant sends notifications through the Home Assistant REST API.
//
// Upstream documentation: https://developers.home-assistant.io/docs/api/rest/
//
// # URL Format
//
//	homeassistant://TOKEN@host[:port][/basepath][?query]
//
// The long-lived access token is the URL username. The host identifies the
// Home Assistant instance. An optional path is a reverse-proxy prefix.
// Requests always use HTTPS with certificate verification.
//
// # Query Parameters
//
//   - title: optional notification title. Omitted from the API request when empty.
//   - service: Home Assistant action. Empty defaults to persistent_notification.create.
//     A value without a dot is sent as notify/<value>. A value with a dot is
//     sent as <domain>/<action>.
//   - targets: comma-separated notify destinations, sent as JSON field target.
//     Omitted for persistent notifications.
//   - nid: persistent notification ID. Omitted when empty. Replaces an existing
//     notification when reused.
//
// When the port is omitted, HTTPS uses 443.
//
// # Example
//
//	homeassistant://<token>@ha.example.com?title=Update
package homeassistant
