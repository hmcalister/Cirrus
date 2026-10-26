package api

import (
	"io/fs"
	"net/http"

	"github.com/hmcalister/cirrus/internal/template"
)

// Serves the embedded static assets (CSS and the vendored HTMX build).
func staticHandler() http.Handler {
	staticFS, err := fs.Sub(template.FS, template.WebStaticDir)
	if err != nil {
		// The directory is embedded at compile time, so this cannot happen.
		panic("api: static assets missing from embedded templates: " + err.Error())
	}

	return http.StripPrefix("/static/", http.FileServerFS(staticFS))
}

// Returns the raw session token from a Cookie header, or "" when the header is absent, malformed, or carries no session cookie.
func sessionTokenFromCookieHeader(cookieHeader, cookieName string) string {
	if cookieHeader == "" {
		return ""
	}

	cookies, _ := http.ParseCookie(cookieHeader)
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			return cookie.Value
		}
	}
	return ""
}
