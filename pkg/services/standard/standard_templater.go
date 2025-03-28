package standard

import (
	"fmt"
	"os"
	"text/template"
)

// Templater is the standard implementation of ApplyTemplate using the "text/template" library.
type Templater struct {
	templates map[string]*template.Template
}

// GetTemplate attempts to retrieve the template identified with id.
func (templater *Templater) GetTemplate(id string) (*template.Template, bool) {
	tpl, found := templater.templates[id]

	return tpl, found
}

// SetTemplateString creates a new template from the body and assigns it the id.
func (templater *Templater) SetTemplateString(templateID string, body string) error {
	tpl, err := template.New("").Parse(body)
	if err != nil {
		return fmt.Errorf("parsing template string for ID %q: %w", templateID, err)
	}

	if templater.templates == nil {
		templater.templates = make(map[string]*template.Template, 1)
	}

	templater.templates[templateID] = tpl

	return nil
}

// SetTemplateFile creates a new template from the file and assigns it the id.
func (templater *Templater) SetTemplateFile(templateID string, file string) error {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading template file %q for ID %q: %w", file, templateID, err)
	}

	return templater.SetTemplateString(templateID, string(bytes))
}
