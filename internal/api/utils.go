package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

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

// Writes an escaped HTML body with the given status.
func writePage(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Warn("error writing response body", "error", err)
	}
}

// Extract a session token from the request.
func sessionToken(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil {
		return cookie.Value
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}

// Returns a trimmed form value.
func formValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.PostFormValue(name))
}

// Redirects an HTMX request with HTTP 303 and the HX-Redirect header.
func writeHXRedirect(w http.ResponseWriter, location string) {
	w.Header().Set("HX-Redirect", location)
	w.WriteHeader(http.StatusSeeOther)
}
