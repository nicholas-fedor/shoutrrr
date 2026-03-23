package main

// fieldSchema represents a single configuration field exposed to the browser.
// It maps Shoutrrr's internal FieldInfo to a JSON-serializable structure
// that the frontend can use to render form inputs.
type fieldSchema struct {
	// Name is the Go struct field name (e.g., "WebhookID").
	Name string `json:"name"`
	// Type is the human-readable type string (e.g., "string", "bool", "enum").
	Type string `json:"type"`
	// Required indicates whether the field must be set for URL generation.
	Required bool `json:"required"`
	// Description provides context about the field's purpose.
	Description string `json:"description"`
	// DefaultValue is the field's default value from struct tags.
	DefaultValue string `json:"defaultValue"`
	// URLPart indicates where the field maps in the URL (e.g., "host", "user").
	URLPart string `json:"urlPart"`
	// Keys are the query parameter names that map to this field.
	Keys []string `json:"keys"`
	// EnumValues lists valid options for enum-type fields.
	EnumValues []string `json:"enumValues,omitempty"`
}

// configSchema represents the full configuration schema for a service.
// It includes the service identity and all configurable fields.
type configSchema struct {
	// Service is the service name (e.g., "discord").
	Service string `json:"service"`
	// Scheme is the URL scheme for this service.
	Scheme string `json:"scheme"`
	// Fields contains all configurable fields for the service.
	Fields []fieldSchema `json:"fields"`
}

// parseResult is the output of parsing a Shoutrrr URL.
type parseResult struct {
	// Service is the extracted service scheme.
	Service string `json:"service"`
	// Config contains field name-value pairs from the parsed URL.
	Config map[string]string `json:"config"`
}

// errorResult is returned when an operation fails.
type errorResult struct {
	// Error contains the human-readable error message.
	Error string `json:"error"`
}
