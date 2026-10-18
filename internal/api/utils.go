package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// Extract a session token from the request, preferring the session cookie and falling back to an Authorization Bearer header.
func sessionToken(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil {
		return cookie.Value
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("error writing response body", "error", err)
	}
}
