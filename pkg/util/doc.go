// Package util provides helpers that several Shoutrrr packages share. They
// cover these areas.
//
//   - Checks of a reflect.Kind, such as whether it is a signed integer.
//   - Number parsing with prefixes such as 0x and #, and hex checks.
//   - URL hosts, ports, paths, queries, and user info.
//   - Message splitting, shortening, and titling.
//   - Lists without their empty elements.
//   - A logger that discards its output.
//
// Subpackages provide additional utilities:
//   - jsonclient: HTTP client for JSON APIs
//   - generator: Service configuration generation utilities
package util
