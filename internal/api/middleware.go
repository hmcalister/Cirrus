package api

import (
	"net/http"
)

const (
	authenticationCookieName = "session"
)

// Middleware for authentication.
//
// Wraps next so that only requests carrying a valid session token reach it.
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := sessionToken(r, authenticationCookieName)
		userEmail, ok := s.AuthService.SessionEmail(r.Context(), raw)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r.WithContext(withEmail(r.Context(), userEmail)))
	})
}
