// Package template loads and renders the operator-editable templates that live
// outside the binary. Templates are mounted into the container (or read from the
// working directory on the host) so that their content can change without a
// redeploy.
package template

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	texttemplate "text/template"
)

// Names of the templates required at startup.
const (
	TemplateVerification  = "verification.tmpl"
	TemplatePasswordReset = "password_reset.tmpl"

	// Template file pattern accepted in template directory.
	templatePattern = "*.tmpl"
)

var (
	_ EmailRenderer = (*Loader)(nil)
	_ PageRenderer  = (*Loader)(nil)
)

type Loader struct {
	email *texttemplate.Template
	web   *template.Template
}

func NewLoader(fsys fs.FS, emailDir, webDir string, requiredEmail, requiredWeb []string) (*Loader, error) {
	if fsys == nil {
		return nil, errors.New("template: fsys must be non-nil")
	}
	if emailDir == "" {
		return nil, errors.New("template: emailDir must not be empty")
	}
	if webDir == "" {
		return nil, errors.New("template: webDir must not be empty")
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

	return &Loader{email: email, web: web}, nil
}

// Execute the named text template and return its body.
func (l *Loader) RenderEmail(templateName string, data any) (string, error) {
	tmpl, err := l.getTemplate(kindEmail, templateName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: rendering email %q: %w", templateName, err)
	}
	return buf.String(), nil
}

// Execute the named HTML template, writing the escaped result to w.
func (l *Loader) RenderPage(w io.Writer, templateName string, data any) error {
	tmpl, err := l.getTemplate(kindWeb, templateName)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("template: rendering page %q: %w", templateName, err)
	}
	return nil
}

func (l *Loader) getTemplate(kind templateKind, name string) (renderable, error) {
	switch kind {
	case kindEmail:
		if l.email == nil {
			return nil, fmt.Errorf("template: email template %q not found", name)
		}
		tmpl := l.email.Lookup(name)
		if tmpl == nil {
			return nil, fmt.Errorf("template: email template %q not found", name)
		}
		return tmpl, nil
	case kindWeb:
		if l.web == nil {
			return nil, fmt.Errorf("template: web template %q not found", name)
		}
		tmpl := l.web.Lookup(name)
		if tmpl == nil {
			return nil, fmt.Errorf("template: web template %q not found", name)
		}
		return tmpl, nil
	default:
		return nil, fmt.Errorf("template: unknown template kind %d", kind)
	}
}
