package template

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	texttemplate "text/template"
)

// Renders a plain-text email body.
type EmailRenderer interface {
	RenderEmail(templateName string, data any) (string, error)
}

// Renders an HTML page.
type PageRenderer interface {
	RenderPage(templateName string, data any) (string, error)
}

var (
	_ EmailRenderer = (*Loader)(nil)
	_ PageRenderer  = (*Loader)(nil)
)

type Loader struct {
	email_templates *texttemplate.Template
	web_templates   *template.Template
}

// Builds a template loader from the embedded FS.
// Required templates, as defined in `internal/template/embed.go` are checked at init time.
func New() (*Loader, error) {
	return newLoaderInternal(FS, requiredEmailTemplates, requiredWebTemplates)
}

// Internal constructor that allows setting required files. For testing.
func newLoaderInternal(fsys fs.FS, requiredEmail, requiredWeb []string) (*Loader, error) {
	if fsys == nil {
		return nil, errors.New("template: fsys must be non-nil")
	}
	if err := requireTemplates(fsys, emailDir, requiredEmail); err != nil {
		return nil, err
	}
	if err := requireTemplates(fsys, webDir, requiredWeb); err != nil {
		return nil, err
	}

	email, err := parseTextTemplates(fsys, emailDir)
	if err != nil {
		return nil, err
	}
	web, err := parseHTMLTemplates(fsys, webDir)
	if err != nil {
		return nil, err
	}

	return &Loader{email_templates: email, web_templates: web}, nil
}

// Execute the named text template and return its body.
func (l *Loader) RenderEmail(templateName string, data any) (string, error) {
	if l.email_templates == nil {
		return "", fmt.Errorf("template: email template %q not found", templateName)
	}
	tmpl := l.email_templates.Lookup(templateName)
	if tmpl == nil {
		return "", fmt.Errorf("template: email template %q not found", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: rendering email %q: %w", templateName, err)
	}
	return buf.String(), nil
}

// Execute the named HTML template and return the escaped page.
func (l *Loader) RenderPage(templateName string, data any) (string, error) {
	if l.web_templates == nil {
		return "", fmt.Errorf("template: web template %q not found", templateName)
	}
	tmpl := l.web_templates.Lookup(templateName)
	if tmpl == nil {
		return "", fmt.Errorf("template: web template %q not found", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: rendering page %q: %w", templateName, err)
	}
	return buf.String(), nil
}
