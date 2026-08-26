// Package signalgrid sends push notifications through the Signalgrid Push API.
//
// Upstream documentation: https://docs.signalgrid.co/api/push-api/
//
// # URL Format
//
//	signalgrid://CLIENT_KEY@CHANNEL[?title=...&type=INFO&critical=No]
//
// The client key is the URL username and the channel token is the host.
//
// # Query Parameters
//
//   - title: optional notification title. Omitted from the API request when empty.
//   - type: severity (CRIT, WARN, INFO, SUCCESS). Default INFO. Lowercase aliases are accepted.
//   - critical: when true, deliver as a critical alert. Independent of type=CRIT.
//
// # Example
//
//	signalgrid://<client-key>@<channel>?title=Server%20Alert&type=CRIT&critical=Yes
package signalgrid
