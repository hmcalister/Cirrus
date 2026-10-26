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
	emailDir     = "email"
	webDir       = "web"
	WebStaticDir = webDir + "/static"
	// Template file pattern accepted in template directory.
	templatePattern = "*.tmpl"
)

// Template names.
// Define the filename here, and add to one of the required lists below.
const (
	TemplateMagicLink = "magic_link.tmpl"

	TemplatePageSignIn      = "sign_in.tmpl"
	TemplatePageAccount     = "account.tmpl"
	templateMessageFragment = "message.tmpl"
	templateLayout          = "layout.tmpl"
	templateAccountStatus   = "account_status.tmpl"
)

var (
	requiredEmailTemplates = []string{
		TemplateMagicLink,
	}
	requiredWebTemplates = []string{
		TemplatePageSignIn,
		TemplatePageAccount,
		templateMessageFragment,
		templateLayout,
		templateAccountStatus,
	}
)
