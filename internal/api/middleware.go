package api

import (
	"log/slog"
	"net/http"
)

// Middleware for authentication.
//
// Wraps next so that only requests carrying a valid Stratus session reach it.
// On the first authenticated request for an identity, the matching user profile
// is created.
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := sessionToken(r, s.SessionCookieName)

		identity, err := s.AuthService.ValidateSession(r.Context(), rawToken)
		if err != nil {
			if rawToken != "" {
				s.clearSessionCookie(w)
			}
			slog.Warn("attempt to reach authentication route without a valid session", "err", err)
			http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
			return
		}

		user, err := s.Users.GetOrCreateUser(r.Context(), identity.Email)
		if err != nil {
			slog.Error("error ensuring user profile", "email_hash", hashEmail(identity.Email), "err", err)
			writePage(w, http.StatusInternalServerError, genericErrorMessage)
			return
		}

		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}
