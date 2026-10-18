package template

import "io"

// EmailRenderer renders a plain-text email body.
type EmailRenderer interface {
	RenderEmail(templateName string, data any) (string, error)
}

// PageRenderer renders an HTML page directly to w.
type PageRenderer interface {
	RenderPage(w io.Writer, templateName string, data any) error
}

type renderable interface {
	Execute(w io.Writer, data any) error
}
