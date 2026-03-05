// Package types provides the core type definitions and interfaces used throughout
// the shoutrrr notification library.
//
// This package defines the fundamental abstractions for notification services,
// including service interfaces, configuration types, message structures, and
// supporting utilities that enable consistent interaction with various
// notification channels.
//
// # Core Interfaces
//
// The package defines several key interfaces that form the foundation of
// shoutrrr's architecture:
//
//   - Service: The primary interface for all notification services, combining
//     sending, templating, and lifecycle management capabilities.
//   - Sender: Defines the basic contract for sending notifications.
//   - Templater: Provides template management for message formatting.
//   - ServiceConfig: Common interface for service configuration types.
//   - Generator: Interface for tools that generate service configurations.
//
// # Message Types
//
// Message handling is supported through several types:
//
//   - MessageItem: Represents an individual notification entry with text,
//     timestamp, level, and optional fields or file attachments.
//   - MessageLevel: Denotes the urgency/severity of a message (Unknown, Debug,
//     Info, Warning, Error).
//   - Field: Key/value pairs for extra data in log messages.
//   - File: Represents file attachments for messages.
//
// # Configuration Types
//
// Configuration management types include:
//
//   - Params: A string map for providing additional variables to service
//     templates, with helper methods for setting and retrieving common
//     parameters like title and message.
//   - ServiceOpts: Interface describing service options including verbosity,
//     logging, and properties.
//   - ConfigQueryResolver: Interface for getting, setting, and listing service
//     config query fields.
//
// # Supporting Types
//
// Additional utility types provided by the package:
//
//   - EnumFormatter: Handles formatting of enumerated configuration values.
//   - StdLogger: Standard logging interface used by services.
//   - QueuedSender: Interface for senders that support message queuing.
//   - RichSender: Interface for senders supporting rich message content.
//   - CustomURLConfig: Interface for configurations that support custom URL
//     resolution.
//   - MessageLimit: Defines limits for message content.
//
// The types in this package are designed to be used by both service
// implementers and consumers of the shoutrrr library, providing a consistent
// and extensible foundation for notification functionality.
package types
