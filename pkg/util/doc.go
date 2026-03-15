// Package util provides common utility functions for Shoutrrr.
//
// This package includes general-purpose utilities organized by domain:
//
//   - Math: Min, Max for basic numeric operations
//
//   - Reflection: IsUnsignedInt, IsSignedInt, IsCollection, IsNumeric
//     for type checking via reflect.Kind
//
//   - Number parsing: StripNumberPrefix for parsing number strings with
//     various prefix formats (#, 0x, 0X)
//
//   - URL handling: URLUserPassword for creating url.Userinfo with
//     empty string handling
//
//   - Message partitioning: PartitionMessage, MessageItemsFromLines,
//     and Ellipsis for splitting large messages into service-compatible chunks
//
//   - Logging: DiscardLogger for discarding log output
//
// Subpackages provide additional utilities:
//   - jsonclient: HTTP client for JSON APIs
//   - generator: Service configuration generation utilities
package util
