package api

import (
	"net/http"
	"strings"
)

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
