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
//   - RichSender: Interface for services that support structured message items
//     (attachments, fields, timestamps). The router dispatches SendItems to
//     services implementing this interface; services that don't implement it
//     fall back to plain text via ItemsToPlain.
//   - ContextSender: Opt-in interface for services that accept a context.Context
//     for cancellation and deadline propagation via SendContext.
//   - ContextAttachmentSender: Opt-in interface for services that accept a
//     context.Context in SendItemsContext for cancellation and deadline
//     propagation on rich sends.
//   - Templater: Provides template management for message formatting.
//   - ServiceConfig: Common interface for service configuration types.
//   - Generator: Interface for tools that generate service configurations.
//   - HTTPClient: Interface for outbound HTTP operations (satisfied by *http.Client).
//     Used for SSRF protection and custom egress control.
//   - HTTPClientSetter: Implemented by services to accept a custom HTTPClient
//     (injected by router.NewWithOptions / NewSenderWithOptions).
//   - SenderOptions: Options for creating senders/routers, including HTTPClient
//     and Timeout overrides.
//
// # Message Types
//
// Message handling is supported through several types:
//
//   - MessageItem: Represents an individual notification entry with text,
//     timestamp, level, and optional fields or file attachments.
//   - MessageLevel: Denotes the urgency/severity of a message (Unknown, Debug,
//     Info, Warning, Error). Params.SetLevel and Params.Level provide a
//     convention for passing severity through the send pipeline.
//   - Field: Key/value pairs for extra data in log messages.
//   - File: Represents file attachments for messages.
//   - TargetError: Wraps a send failure with the service URL/ID that produced it,
//     enabling callers to identify which target failed in a multi-target send.
//     Implements Unwrap so errors.Is and errors.As work against the underlying error.
//
// # Configuration Types
//
// Configuration management types include:
//
//   - Params: A string map for providing additional variables to service
//     templates, with helper methods for setting and retrieving common
//     parameters like title, message, and level.
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
//   - CustomURLConfig: Interface for configurations that support custom URL
//     resolution.
//   - MessageLimit: Defines limits for message content.
//
// The types in this package are designed to be used by both service
// implementers and consumers of the shoutrrr library, providing a consistent
// and extensible foundation for notification functionality.
package types
