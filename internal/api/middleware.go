package api

import (
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Huma middleware that validates the Stratus session, creates the application profile for the authenticated identity,
// and attaches that profile to the request context.
//
// Unauthenticated browser requests are redirected to the sign-in page.
func (s *Server) requireUser(ctx huma.Context, next func(huma.Context)) {
	rawToken := sessionTokenFromCookieHeader(ctx.Header("Cookie"), s.SessionCookieName)

	identity, err := s.AuthService.ValidateSession(ctx.Context(), rawToken)
	if err != nil {
		ctx.SetHeader("Location", "/sign-in")
		if rawToken != "" {
			ctx.AppendHeader("Set-Cookie", s.clearSessionCookie().String())
		}
		ctx.SetStatus(http.StatusSeeOther)
		slog.Warn("attempt to reach authentication route without a valid session", "err", err)
		return
	}

	user, err := s.Users.GetOrCreateUser(ctx.Context(), identity.Email)
	if err != nil {
		slog.Error("error ensuring user profile", "email_hash", hashEmail(identity.Email), "err", err)
		ctx.SetHeader("Content-Type", htmlContentType)
		ctx.SetStatus(http.StatusInternalServerError)
		_, _ = ctx.BodyWriter().Write([]byte(genericErrorMessage))
		return
	}

	next(huma.WithValue(ctx, userKey, user))
}
