package api

import (
	"log/slog"
	"net/http"

	"github.com/hmcalister/Cirrus/internal/template"
)

// Page data for rendering templates

type loginPageData struct{}

type registerPageData struct{}

type accountPageData struct {
	Email    string
	Active   bool
	Verified bool
}
type accountStatusFragment struct {
	Kind    string
	Message string
	Active  bool
}
type resetPageData struct {
	Token string
}

type passwordResetRequestPageData struct{}

type verifyResendPageData struct{}

// Misc routes

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writePage(w, http.StatusOK, "ok")
}

// Page routes

func (s *Server) handleLoginPage(w http.ResponseWriter, _ *http.Request) {
	s.renderPage(w, template.TemplatePageLogin, loginPageData{})
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, _ *http.Request) {
	s.renderPage(w, template.TemplatePageRegister, registerPageData{})
}

func (s *Server) handleResetPage(w http.ResponseWriter, r *http.Request) {
	token := formValue(r, "token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	s.renderPage(w, template.TemplatePageReset, resetPageData{Token: token})
}

func (s *Server) handlePasswordResetRequestPage(w http.ResponseWriter, _ *http.Request) {
	s.renderPage(w, template.TemplatePagePasswordResetRequest, passwordResetRequestPageData{})
}

func (s *Server) handleVerifyResendPage(w http.ResponseWriter, _ *http.Request) {
	s.renderPage(w, template.TemplatePageVerifyResend, verifyResendPageData{})
}

func (s *Server) handleAccountPage(w http.ResponseWriter, r *http.Request) {
	userEmail := emailFromContext(r.Context())

	active, err := s.AuthService.AccountActive(r.Context(), userEmail)
	if err != nil {
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}
	verified, err := s.AuthService.AccountVerified(r.Context(), userEmail)
	if err != nil {
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}

	s.renderPage(w, template.TemplatePageAccount, accountPageData{Email: userEmail, Active: active, Verified: verified})
}

// POST routes

// Burns the token from the email link and redirects to the login page.
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")

	_, err := s.AuthService.Verify(r.Context(), rawToken)
	if err != nil {
		status, message := authErrorStatus(err)
		s.renderFragment(w, status, "login_error", errorFragment(message))
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	userEmail := formValue(r, "email")
	password := r.PostFormValue("password")

	rawToken, err := s.AuthService.Login(r.Context(), userEmail, password)
	if err != nil {
		slog.Info("login failed", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(userEmail))
		s.renderAuthError(w, "login_error", err)
		return
	}

	slog.Info("login succeeded", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(userEmail))
	s.setSessionCookie(w, rawToken)
	writeHXRedirect(w, "/account")
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	userEmail := formValue(r, "email")
	password := r.PostFormValue("password")

	if err := s.AuthService.Register(r.Context(), userEmail, password); err != nil {
		s.renderAuthError(w, "register_error", err)
		return
	}

	slog.Info("account registered", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(userEmail))
	writeHXRedirect(w, "/login")
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	s.clearSessionCookie(w)
	writeHXRedirect(w, "/login")
}

// Requests a reset email.
//
// The response never reveals whether the account exists, so it always renders the same status message with HTTP 200.
func (s *Server) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	userEmail := formValue(r, "email")

	if err := s.AuthService.RequestPasswordReset(r.Context(), userEmail); err != nil {
		s.renderAuthError(w, "password_reset_status", err)
		return
	}

	slog.Info("password reset requested", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(userEmail))
	s.renderFragment(w, http.StatusOK, "password_reset_status", statusFragment(passwordResetSentMessage))
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	rawToken := formValue(r, "token")
	password := r.PostFormValue("password")

	if err := s.AuthService.ConfirmPasswordReset(r.Context(), rawToken, password); err != nil {
		s.renderAuthError(w, "reset_error", err)
		return
	}

	slog.Info("password reset confirmed", "request_id", requestIDFromContext(r.Context()))
	writeHXRedirect(w, "/login")
}

// Resends the verification email.
//
// The response never reveals whether the account exists, so it always renders the same status message with HTTP 200.
func (s *Server) handleVerifyResend(w http.ResponseWriter, r *http.Request) {
	userEmail := formValue(r, "email")

	if err := s.AuthService.ResendVerification(r.Context(), userEmail); err != nil {
		s.renderAuthError(w, "verify_resend_status", err)
		return
	}

	slog.Info("verification resent", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(userEmail))
	s.renderFragment(w, http.StatusOK, "verify_resend_status", statusFragment(verificationResentMessage))
}

func (s *Server) handleSetActive(w http.ResponseWriter, r *http.Request) {
	userEmail := emailFromContext(r.Context())
	activeStatusFormValue := r.PostFormValue("activeStatus")
	if activeStatusFormValue != "true" && activeStatusFormValue != "false" {
		s.renderFragment(w, http.StatusBadRequest, "account_error", errorFragment("invalid active status."))
		return
	}

	activeStatus := activeStatusFormValue == "true"
	if err := s.AuthService.SetAccountActive(r.Context(), userEmail, activeStatus); err != nil {
		s.renderAuthError(w, "account_error", err)
		return
	}

	slog.Info("account active status changed",
		"request_id", requestIDFromContext(r.Context()),
		"email_hash", hashEmail(userEmail),
		"active", activeStatus,
	)

	message := "Daily emails paused."
	if activeStatus {
		message = "Daily emails resumed."
	}
	s.renderFragment(w, http.StatusOK, "account_status", accountStatusFragment{
		Kind:    "status",
		Message: message,
		Active:  activeStatus,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	userEmail := emailFromContext(r.Context())

	if err := s.AuthService.DeleteAccount(r.Context(), userEmail); err != nil {
		s.renderAuthError(w, "account_error", err)
		return
	}

	s.clearSessionCookie(w)
	slog.Info("account deleted", "request_id", requestIDFromContext(r.Context()), "email_hash", hashEmail(userEmail))
	writeHXRedirect(w, "/login")
}
