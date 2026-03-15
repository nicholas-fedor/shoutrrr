// Package standard provides standard implementations for core Shoutrrr service functionality.
//
// The standard package offers reusable components that notification services can embed
// to implement common functionality required by the Shoutrrr framework. Rather than
// implementing logging, templating, and error handling from scratch, services can embed
// the Standard type to get these capabilities out of the box.
//
// # Overview
//
// The package provides three main components:
//
//   - Standard: A composable type that embeds Logger and Templater
//   - Logger: Standard logging implementation for services
//   - Templater: Template management using Go's text/template package
//
// In addition, the package provides utility types and functions for error handling
// and configuration management that are commonly needed across different services.
//
// # Usage
//
// Services typically embed standard.Standard in their Service struct:
//
//	type Service struct {
//	    standard.Standard
//	    // service-specific fields
//	}
//
// This gives the Service access to all logging and templating methods through
// composition. The Standard type implements the types.StdLogger and types.Templater
// interfaces required by the shoutrrr.Service interface.
//
// # Key Types
//
// ## Standard
//
// The Standard type embeds both Logger and Templater, providing a convenient way
// to include all standard functionality in a single embed:
//
//	var service Service
//	service.Standard = standard.Standard{}
//	service.SetLogger(logger)
//	service.Log("Service initialized")
//
// ## Logger
//
// Logger provides methods for outputting non-fatal log information. It wraps the
// types.StdLogger interface and provides Log and Logf methods:
//
//	logger := &standard.Logger{}
//	logger.SetLogger(myLogger)
//	logger.Log("Message without formatting")
//	logger.Logf("Formatted message: %s", argument)
//
// When SetLogger is called with nil, a discard logger is used to prevent nil
// pointer dereferences.
//
// ## Templater
//
// Templater provides template management using Go's text/template package. Services
// can use it to store and retrieve templates by ID:
//
//	templater := &standard.Templater{}
//	templater.SetTemplateString("welcome", "Hello, {{.Name}}!")
//
//	tpl, found := templater.GetTemplate("welcome")
//	if found {
//	    // Use tpl.Execute() to render the template
//	}
//
// Templates can be loaded from strings or files:
//
//	err := templater.SetTemplateString("my-template", "Template content")
//	err := templater.SetTemplateFile("my-template", "/path/to/template.txt")
//
// # Error Handling
//
// The package provides failure types and helper functions for common error scenarios:
//
//	const (
//	    FailTestSetup  failures.FailureID = -1  // Test setup errors
//	    FailParseURL   failures.FailureID = -2  // URL parsing errors
//	    FailServiceInit failures.FailureID = -3 // Service initialization errors
//	    FailUnknown    failures.FailureID = 0   // Default/unknown errors
//	)
//
// The Failure function creates a Failure instance with a specific failure ID:
//
//	fail := standard.Failure(standard.FailParseURL, err, "invalid URL format")
//
// The IsTestSetupFailure helper checks if a failure is due to test setup issues:
//
//	if msg, isSetupFailure := standard.IsTestSetupFailure(fail); isSetupFailure {
//	    // Handle test setup failure
//	}
//
// # Configuration
//
// EnumlessConfig provides a minimal implementation of types.ServiceConfig for
// services that don't use enumerated configuration fields:
//
//	type Config struct {
//	    standard.EnumlessConfig
//	    // service-specific config fields
//	}
//
//	func (c *Config) Enums() map[string]types.EnumFormatter {
//	    return map[string]types.EnumFormatter{}  // Empty - no enum fields
//	}
//
// # Examples
//
// ## Basic Service Setup
//
//	func NewService() *Service {
//	    return &Service{
//	        Standard: standard.Standard{},
//	    }
//	}
//
//	func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
//	    s.SetLogger(logger)
//	    s.Log("Initializing service")
//	    // service-specific initialization
//	    return nil
//	}
//
// ## Using Templates
//
//	func (s *Service) Send(ctx context.Context, params *types.Params) error {
//	    // Load template if not already present
//	    if _, found := s.GetTemplate("message"); !found {
//	        s.SetTemplateString("message", "Title: {{.Title}}\n{{.Body}}")
//	    }
//
//	    // Render template
//	    tpl, _ := s.GetTemplate("message")
//	    var buf bytes.Buffer
//	    if err := tpl.Execute(&buf, params); err != nil {
//	        return standard.Failure(standard.FailUnknown, err)
//	    }
//
//	    return s.sendToProvider(buf.String())
//	}
//
// ## Error Handling
//
//	func (s *Service) parseURL(rawURL string) (*url.URL, error) {
//	    parsed, err := url.Parse(rawURL)
//	    if err != nil {
//	        return nil, standard.Failure(standard.FailParseURL, err)
//	    }
//
//	    if parsed.Host == "" {
//	        return nil, standard.Failure(standard.FailParseURL, errors.New("empty host"))
//	    }
//
//	    return parsed, nil
//	}
//
// # Integration with Services
//
// The standard package is designed to be embedded by notification service implementations.
// All built-in Shoutrrr services use this pattern:
//
//   - Chat services: Discord, Slack, Mattermost, Telegram, Teams, etc.
//   - Push services: Gotify, ntfy, Pushover, Pushbullet, etc.
//   - Email services: SMTP
//   - Incident services: PagerDuty, OpsGenie
//   - Specialized services: Generic, Logger, Notifiarr
//
// By embedding standard.Standard, services automatically get:
//
//   - Logging via Log() and Logf() methods
//   - Template management via GetTemplate(), SetTemplateString(), SetTemplateFile()
//   - Integration with shoutrrr's service discovery and URL parsing
//
// This approach promotes code reuse and consistency across different notification
// service implementations while allowing each service to focus on its specific
// provider integration.
package standard
