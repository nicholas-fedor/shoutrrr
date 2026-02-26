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
//   - EscapeKey/UnescapeKey: Escapes/unescapes keys that conflict with config props
//
// # Configuration Formatting
//
// The package includes comprehensive support for parsing and formatting various field types
// found in service configurations:
//
//   - Basic types: strings, integers, booleans
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
// EnumFormatter: Interface for handling enumerated configuration values with
// named options.
package format
