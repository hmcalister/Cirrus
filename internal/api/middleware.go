package api

import (
	"log/slog"
	"net/http"
	"slices"

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

// Returns a Huma middleware that rejects the request unless the authenticated user holds the given role.
//
// It must run after requireUser, which attaches the application profile to the request context. A missing profile is an internal error rather than a permission failure.
func (s *Server) makeRequiredRoleMiddleware(role string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		user, ok := userFromContext(ctx.Context())
		if !ok {
			_ = huma.WriteErr(s.api, ctx, http.StatusInternalServerError, "account profile unavailable")
			return
		}

		if !slices.Contains(user.Roles, role) {
			_ = huma.WriteErr(s.api, ctx, http.StatusForbidden, "insufficient permissions")
			return
		}

		next(ctx)
	}
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
