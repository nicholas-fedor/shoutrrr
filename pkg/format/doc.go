// Package format provides utilities for formatting, parsing, and rendering notification service
// configurations in the shoutrrr library.
//
// This package handles various aspects of service configuration management:
//
// # Query String Handling
//
// The package provides functions for building and parsing URL query strings that represent
// service configurations. It supports custom fields that can be escaped to avoid conflicts
// with built-in configuration properties using the "__" prefix.
//
// Key functions:
//   - BuildQuery: Converts a config object to a query string
//   - BuildQueryWithCustomFields: Builds a query with custom fields, escaping conflicting keys
//   - SetConfigPropsFromQuery: Sets config properties from query string values
//   - EscapeKey: Escapes a custom key that a config prop also uses
//
// # Configuration Formatting
//
// The package includes comprehensive support for parsing and formatting various field types
// found in service configurations:
//
//   - Basic types: strings, integers, booleans
//   - Duration types: [time.Duration], parsed and rendered with Go duration syntax
//     (e.g. "10s", "1h2m3s"). Bare, non-zero numbers are rejected; however, "0" is
//     accepted as the zero duration
//   - Complex types: maps, slices, arrays, structs
//   - Enum types: enumerated values with named options
//
// Key functions:
//   - ParseBool/PrintBool: Parse and print boolean values
//   - IsNumber: Check if a string represents a number
//   - SetConfigField: Set a config field from a string value
//   - GetConfigFieldString: Get string representation of a config field
//   - GetConfigFormat: Get type and field information from a service config
//
// # Deprecated Keys and Fields
//
// A renamed option keeps working through struct tags.
//
// A `deprecatedkeys` tag on a keyed field lists its old keys, separated by
// commas. They set the field like its other keys. They are never written to a
// URL. When a URL or params hold both an old key and a current key, the
// current key wins. [PropKeyResolver.DeprecatedKeys] reports the old keys a
// URL used, and [PropKeyResolver.DeprecatedParams] reports those in params, so
// the service can warn about them.
//
// A `deprecated` tag on an old field names the field that replaces it, which
// must be an exported field of the same type. The docs, the generators, and
// [MaskedURL] leave the old field out. [SyncDeprecatedFields] copies it into
// its replacement. A service calls it on the config it sends with, and on a
// copy of the config in GetURL.
//
//	SkipTLSVerify bool `default:"No" deprecatedkeys:"insecureskipverify" key:"skiptlsverify"`
//	InsecureSkipVerify bool `deprecated:"SkipTLSVerify"`
//
// A `replacedby` tag deprecates the keys of a string field in favor of the
// key it names, whose field may have another type. A `replacedvalues` tag maps
// each value of the old keys, in any case, onto a value of the new key, and a
// value it does not list is rejected. The old keys set the new key's field.
// Setting that field through any of its keys resets the old field to its
// default, so a value set on it in code before no longer overrides the new
// key. The old keys lose to the new key and are reported as deprecated keys.
// [SyncDeprecatedFields] moves a value set on the old field in code into the
// new key's field the same way.
//
//	Scheme string `default:"https" key:"scheme" replacedby:"disabletls" replacedvalues:"http=yes,https=no"`
//	DisableTLS bool `default:"No" key:"disabletls"`
//
// # Keys a URL Set
//
// A config that embeds [URLKeys] records the keys its URL set, so
// [PropKeyResolver.KeepsDefault] can tell a default that the URL named from
// one that it left out. A `levelmapped:"true"` tag marks a field that a
// service maps the level param onto. The URL its config writes keeps such a
// key whenever a URL set it, even at its default.
//
// # Format Conversion
//
// The package provides conversion between text, markdown, and HTML formats:
//
//   - markdown ↔ text
//   - html ↔ text
//   - text ↔ markdown
//   - text ↔ html
//   - markdown ↔ html
//
// Key functions:
//   - ConvertFormat: Convert between any supported formats
//
// # Property Serialization
//
// The package provides serialization/deserialization support for configuration properties
// that implement the ConfigProp interface:
//
// Key functions:
//   - GetConfigPropFromString: Deserialize a config property from a string
//   - GetConfigPropString: Serialize a config property to a string
//
// # Output Rendering
//
// The package supports multiple output formats for displaying configuration trees:
//
//   - Console output with color highlighting
//   - Markdown output for documentation
//   - Tree-based hierarchical display
//
// Key functions:
//   - ColorFormatTree: Generate color-highlighted tree output
//   - ConsoleTreeRenderer: Render configuration as console-friendly tree
//   - MarkdownTreeRenderer: Render configuration as markdown
//
// # Key Concepts
//
// KeyPrefix ("__"): Prefix used to escape custom URL query keys that would otherwise
// conflict with built-in service configuration property names.
//
// ConfigQueryResolver: Interface for objects that can provide and accept configuration
// through query string parameters.
//
// ContainerNode: Tree node structure representing configuration hierarchies.
//
// FieldInfo: Metadata about configuration fields including type, constraints, and
// formatting information.
//
// EnumFormatter: Formatter that reads and writes enumerated configuration
// values by their names.
package format
