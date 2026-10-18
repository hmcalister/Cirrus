package template

import "embed"

// Holds templates for email and HTML pages, compiled into the binary.
//
// Templates are added by writing a .tmpl file into the email/ or web/ directory,
// and adding its name to the matching required list in this file (see the
// Template* constants).
//
// On initialization of a Loader, every *.tmpl is parsed and all required files are checked.
//
// `embed all:` ensures the placeholder .gitignores are embedded. Stops errors when there are no templates.
//
//go:embed all:email all:web
var FS embed.FS

// Template directories within FS.
const (
	emailDir = "email"
	webDir   = "web"
	// Template file pattern accepted in template directory.
	templatePattern = "*.tmpl"
)

// Template names.
// Define the filename here, and add to one of the required lists below.
const (
	TemplateVerification  = "verification.tmpl"
	TemplatePasswordReset = "password_reset.tmpl"
)

var (
	requiredEmailTemplates = []string{
		TemplateVerification,
		TemplatePasswordReset,
	}
	requiredWebTemplates = []string{
)
