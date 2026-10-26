package api

import (
	"log/slog"
	"net/http"

	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/template"
)

// Page data for rendering templates

type accountPageData struct {
	Email  string
	Active bool
}

// Data for the account_status fragment returned after a setActive POST. It
// carries the inline message and the new Active value so the OOB region swaps.
type accountStatusFragment struct {
	Kind    string
	Message string
	Active  bool
}

// Misc routes

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writePage(w, http.StatusOK, "ok")
}

// Page routes

func (s *Server) handleSignInPage(w http.ResponseWriter, _ *http.Request) {
	s.renderPage(w, template.TemplatePageSignIn, nil)
}

func (s *Server) handleAccountPage(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		// RequireAuth guarantees a user in the context.
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}

	s.renderPage(w, template.TemplatePageAccount, accountPageData{Email: user.Email, Active: user.Active})
}

// POST routes

// Revokes the current session and clears the cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	rawToken := sessionToken(r, s.SessionCookieName)

	if err := s.AuthService.Logout(r.Context(), rawToken); err != nil {
		slog.Warn("error when signing out", "request_context", r.Context(), "err", err)
		s.renderAuthError(w, "account_error", err)
		return
	}

	s.clearSessionCookie(w)
	writeHXRedirect(w, "/sign-in")
}

// Revokes every session belonging to the signed-in user.
func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}

	if err := s.AuthService.LogoutAll(r.Context(), user.Email); err != nil {
		slog.Warn("error when signing out all sessions", "request_context", r.Context(), "err", err)
		s.renderAuthError(w, "account_error", err)
		return
	}

	s.clearSessionCookie(w)
	slog.Info("all sessions signed out", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(user.Email))
	writeHXRedirect(w, "/sign-in")
}

func (s *Server) handleSetActive(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}

	activeStatusFormValue := r.PostFormValue("activeStatus")
	if activeStatusFormValue != "true" && activeStatusFormValue != "false" {
		slog.Warn("set active request sent with bad value", "request_context", r.Context(), "activeStatusFormValue", activeStatusFormValue)
		s.renderFragment(w, http.StatusBadRequest, "account_error", errorFragment("invalid active status."))
		return
	}

	activeStatus := activeStatusFormValue == "true"
	if err := s.Users.SetActive(r.Context(), database.SetActiveParams{Email: user.Email, Active: activeStatus}); err != nil {
		slog.Warn("error when setting account active", "request_context", r.Context(), "err", err)
		s.renderAuthError(w, "account_error", err)
		return
	}

	slog.Info("account active status changed",
		"request_id", requestIDFromContext(r.Context()),
		"email_hash", hashEmail(user.Email),
		"active", activeStatus,
	)

	message := "Daily emails paused."
	if activeStatus {
		message = "Daily emails resumed."
	}
	s.renderFragment(w, http.StatusOK, "account_active_status", accountStatusFragment{
		Kind:    "status",
		Message: message,
		Active:  activeStatus,
	})
}

// DeleteAccount permanently removes the profile and every session for the user.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}

	// Revoke sessions first. If the profile delete then fails, the user is
	// signed out rather than left with a session that would recreate a profile.
	if err := s.AuthService.LogoutAll(r.Context(), user.Email); err != nil {
		slog.Warn("error when signing out before account deletion", "request_context", r.Context(), "err", err)
		s.renderAuthError(w, "account_error", err)
		return
	}

	if err := s.Users.DeleteUser(r.Context(), user.Email); err != nil {
		slog.Warn("error when deleting account", "request_context", r.Context(), "err", err)
		s.renderAuthError(w, "account_error", err)
		return
	}

	s.clearSessionCookie(w)
	slog.Info("account deleted", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(user.Email))
	writeHXRedirect(w, "/sign-in")
}
