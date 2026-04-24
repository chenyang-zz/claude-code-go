package prompts

import (
	"fmt"
	"strings"
)

// TemplateSet holds named templates that can be rendered with data.
type TemplateSet struct {
	templates map[string]string
}

// NewTemplateSet creates an empty template set.
func NewTemplateSet() *TemplateSet {
	return &TemplateSet{
		templates: make(map[string]string),
	}
}

// Register stores a named template.
func (t *TemplateSet) Register(name, template string) {
	if t.templates == nil {
		t.templates = make(map[string]string)
	}
	t.templates[name] = template
}

// Render replaces {key} placeholders in the named template with values from data.
// Returns an error if the named template is not registered.
func (t *TemplateSet) Render(name string, data map[string]string) (string, error) {
	tmpl, ok := t.templates[name]
	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}
	result := tmpl
	for key, value := range data {
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}
	return result, nil
}

// Has reports whether a template with the given name is registered.
func (t *TemplateSet) Has(name string) bool {
	_, ok := t.templates[name]
	return ok
}
