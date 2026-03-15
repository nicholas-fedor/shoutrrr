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
func (t *Templater) GetTemplate(id string) (*template.Template, bool) {
	tpl, found := t.templates[id]

	return tpl, found
}

// SetTemplateFile creates a new template from the file and assigns it the id.
func (t *Templater) SetTemplateFile(templateID, file string) error {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading template file %q for ID %q: %w", file, templateID, err)
	}

	return t.SetTemplateString(templateID, string(bytes))
}

// SetTemplateString creates a new template from the body and assigns it the id.
func (t *Templater) SetTemplateString(templateID, body string) error {
	tpl, err := template.New("").Parse(body)
	if err != nil {
		return fmt.Errorf("parsing template string for ID %q: %w", templateID, err)
	}

	if t.templates == nil {
		t.templates = make(map[string]*template.Template, 1)
	}

	t.templates[templateID] = tpl

	return nil
}
