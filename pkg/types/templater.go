package types

import (
	"text/template"
)

// Templater is the interface for the service template API.
type Templater interface {
	GetTemplate(id string) (template *template.Template, found bool)
	SetTemplateString(id, body string) error
	SetTemplateFile(id, file string) error
}
