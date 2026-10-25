package types

import (
	"text/template"
)

// Templater is the interface for the service template API.
type Templater interface {
	// GetTemplate returns the template registered under an ID.
	//
	// Parameters:
	//   - id: the template ID.
	//
	// Returns:
	//   - template: the template, or nil when none is registered.
	//   - found: true when a template is registered under id.
	GetTemplate(id string) (template *template.Template, found bool)
	// SetTemplateString parses a template body and registers it under an ID.
	//
	// Parameters:
	//   - id: the template ID.
	//   - body: the template text, in [text/template] syntax.
	//
	// Returns:
	//   - error: the parse failure, or nil on success.
	SetTemplateString(id, body string) error
	// SetTemplateFile reads a template file and registers its parsed content
	// under an ID.
	//
	// Parameters:
	//   - id: the template ID.
	//   - file: the path of the template file.
	//
	// Returns:
	//   - error: the read or parse failure, or nil on success.
	SetTemplateFile(id, file string) error
}
