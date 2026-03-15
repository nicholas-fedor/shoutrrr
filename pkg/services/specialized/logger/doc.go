// Package logger provides a notification service that outputs notifications to a logger.
//
// The Logger service is a specialized notification service that writes notifications
// to a logger instance rather than sending them to an external service. This is
// particularly useful for testing, debugging, or when you need to capture notification
// output within an application without external dependencies. The service writes
// messages to the standard logger configured by the consumer, making it integrate
// seamlessly with existing logging infrastructure.
//
// The logger service is part of the "specialized" category in Shoutrrr, which contains
// services that serve specific purposes beyond typical notification delivery. Unlike
// chat, push, or email services, the logger service doesn't communicate with external
// APIs but instead provides a local logging output mechanism.
//
// # URL Format
//
// The service URL follows the format:
//
//	logger://
//
// The logger service does not require any host, port, or authentication parameters.
// The URL scheme "logger" identifies this service type. No query parameters are
// supported as the service has no configuration options beyond the logger instance.
//
// # Configuration Options
//
// No configuration options are available for this service. The logger service
// inherits from the standard service infrastructure but does not expose any
// additional configuration through the URL. The logger instance itself must be
// provided when creating the service, typically through the shoutrrr API.
//
// The Config struct is intentionally minimal and implements the EnumlessConfig
// interface, indicating that no enumerable configuration fields are available.
// This design reflects the service's purpose as a simple passthrough to the
// configured logger.
//
// # Templates
//
// The logger service supports message templates through the standard Shoutrrr
// templating system. Templates allow you to customize the output format by
// processing notification parameters before writing to the logger.
//
// Template variables are populated from the notification params, allowing for
// dynamic message formatting. For example, you can include severity levels,
// timestamps, or custom fields in the logged output.
//
// ## Template Example
//
// To set a template that includes a severity level:
//
//	service.SetTemplateString("message", "{{.level}}: {{.message}}")
//
// Then send a notification with params:
//
//	params := types.Params{"level": "warning"}
//	service.Send("Disk space low", &params)
//
// Output: "warning: Disk space low"
//
// # Usage Examples
//
// ## Basic notification
//
//	url := "logger://"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Using with custom logger
//
//	url := "logger://"
//	logger := log.New(os.Stdout, "[shoutrrr] ", log.LstdFlags)
//	err := shoutrrr.Send(url, "Custom logger output", &types.Params{}, logger)
//
// ## With template
//
//	url := "logger://"
//	service, err := shoutrrr.CreateService(url)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	err = service.SetTemplateString("message", "[{{.level}}] {{.message}}")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	params := types.Params{"level": "INFO"}
//	err = service.Send("Application started", &params)
//
// # Common Use Cases
//
// ## Testing and Development
//
// The logger service is invaluable for testing notification workflows without
// external dependencies:
//
//	url := "logger://"
//	err := shoutrrr.Send(url, "Test notification")
//
// ## Debugging Notification Flows
//
// Capture and inspect notification messages during development:
//
//	url := "logger://"
//	err := shoutrrr.Send(url, "Debug: "+jsonString)
//
// ## Centralized Logging Integration
//
// Route notifications through your application's existing logging infrastructure:
//
//	url := "logger://"
//	customLogger := log.New(logFile, "[notifications] ", log.LstdFlags)
//	err := shoutrrr.Send(url, message, params, customLogger)
//
// ## Piping Notifications
//
// Combine with other services for complex notification routing:
//
//	// Log all outgoing notifications
//	loggerService, _ := shoutrrr.CreateService("logger://")
//	// Process and forward to other services...
//
// # Error Handling
//
// The logger service has minimal error conditions since it doesn't communicate
// with external services. However, errors can occur in the following scenarios:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to log notification: %v", err)
//	}
//
// Error scenarios:
//   - Template execution failure (invalid template syntax or missing fields)
//   - Logger write failures (if the underlying logger encounters an error)
//   - Nil logger (if no logger was provided during initialization)
//
// # Security Considerations
//
// The logger service has minimal security implications since it doesn't transmit
// data externally. However, consider the following:
//
// - Message content written to logs may contain sensitive information
// - Ensure your logging backend is properly secured
// - Be aware of log rotation policies to prevent disk space issues
// - Consider log levels - verbose notification logging may increase log volume
// - Template variables could potentially expose internal data if logged improperly
//
// # Limitations and Behaviors
//
// - No network communication is performed; all output goes to the configured logger
// - No authentication or authorization is required
// - No configuration parameters are available through the URL
// - The logger must be provided during service initialization
// - Template support is limited to the message field
// - The service always returns successfully unless template execution fails
// - Message content is written as-is without any transformation (except templates)
// - The service ID is "logger" for programmatic identification
package logger
