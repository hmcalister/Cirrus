package api

import (
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Huma middleware that validates the Stratus session, creates the application profile for the authenticated identity, and attaches that profile to the request context.
func (s *Server) requireUser(ctx huma.Context, next func(huma.Context)) {
	rawToken := sessionTokenFromCookieHeader(ctx.Header("Cookie"), s.SessionCookieName)

	identity, err := s.AuthService.ValidateSession(ctx.Context(), rawToken)
	if err != nil {
		_ = huma.WriteErr(s.api, ctx, http.StatusUnauthorized, "a valid session is required")
		return
	}

	user, err := s.Users.GetOrCreateUser(ctx.Context(), identity.Email)
	if err != nil {
		slog.Error("error ensuring user profile", "email_hash", hashEmail(identity.Email), "err", err)
		_ = huma.WriteErr(s.api, ctx, http.StatusInternalServerError, "unable to load account")
		return
	}

	next(huma.WithValue(ctx, userKey, user))
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
