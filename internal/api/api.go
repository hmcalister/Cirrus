package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hmcalister/cirrus/internal/authentication"
	"github.com/hmcalister/cirrus/internal/template"
)

// Interface over the authentication service.
type authService interface {
	Register(ctx context.Context, userEmail, password string) error
	Verify(ctx context.Context, rawToken string) (bool, error)
	SignIn(ctx context.Context, userEmail, password string) (string, error)
	ResendVerification(ctx context.Context, userEmail string) error
	RequestPasswordReset(ctx context.Context, userEmail string) error
	ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error
	SetAccountActive(ctx context.Context, userEmail string, active bool) error
	DeleteAccount(ctx context.Context, userEmail string) error
	SessionEmail(ctx context.Context, rawToken string) (string, bool)
	AccountActive(ctx context.Context, userEmail string) (bool, error)
	AccountVerified(ctx context.Context, userEmail string) (bool, error)
}

var _ authService = (*authentication.AuthenticationService)(nil)

type ServerConfig struct {
	AuthService   authService
	PagesRenderer template.PageRenderer
	CookieSecure  bool
}

type Server struct {
	ServerConfig

	// Enables deterministic request durations in tests. Never use time.Now directly.
	now func() time.Time
}

func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.AuthService == nil {
		return nil, errors.New("authService must be non-nil")
	}
	if cfg.PagesRenderer == nil {
		return nil, errors.New("pagesRenderer must be non-nil")
	}

	return &Server{ServerConfig: cfg, now: time.Now}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /{$}", s.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/account", http.StatusSeeOther)
	})))

	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /sign-in", s.handleSignInPage)
	mux.HandleFunc("GET /register", s.handleRegisterPage)
	mux.HandleFunc("GET /reset", s.handleResetPage)
	mux.HandleFunc("GET /verify", s.handleVerify)
	mux.HandleFunc("GET /password/reset", s.handlePasswordResetRequestPage)
	mux.HandleFunc("GET /verify/resend", s.handleVerifyResendPage)

	mux.HandleFunc("POST /sign-in", s.handleSignIn)
	mux.HandleFunc("POST /register", s.handleRegister)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("POST /password/reset", s.handlePasswordResetRequest)
	mux.HandleFunc("POST /reset", s.handleReset)
	mux.HandleFunc("POST /verify/resend", s.handleVerifyResend)

	mux.Handle("GET /static/", staticHandler())

	accountRoutes := http.NewServeMux()
	accountRoutes.HandleFunc("GET /account", s.handleAccountPage)
	accountRoutes.HandleFunc("POST /account/setActive", s.handleSetActive)
	accountRoutes.HandleFunc("POST /account/delete", s.handleDelete)
	mux.Handle("/account", s.RequireAuth(accountRoutes))
	mux.Handle("/account/", s.RequireAuth(accountRoutes))

	return s.logRequests(mux)
}

// Renders a full page with the given template and data.
func (s *Server) renderPage(w http.ResponseWriter, templateName string, data any) {
	body, err := s.PagesRenderer.RenderPage(templateName, data)
	if err != nil {
		slog.Error("error while rendering page", "template", templateName, "data", data, "err", err)
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return
	}

	writePage(w, http.StatusOK, body)
}

// Renders the fragment named by templateName with the given status.
//
// Reports whether the fragment rendered. A false return means the renderer
// failed and a generic 500 body was already written.
func (s *Server) renderFragment(w http.ResponseWriter, status int, templateName string, data any) bool {
	body, err := s.PagesRenderer.RenderPage(templateName, data)
	if err != nil {
		writePage(w, http.StatusInternalServerError, genericErrorMessage)
		return false
	}

	writePage(w, status, body)
	return true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     authenticationCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Clears the session cookie.
//
// Sets an empty value with an immediate expiry so the browser drops it.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	// Expires the session cookie immediately, used to log out.
	const sessionCookieMaxAge = -1

	http.SetCookie(w, &http.Cookie{
		Name:     authenticationCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Renders err as the named error fragment with its mapped status.
func (s *Server) renderAuthError(w http.ResponseWriter, templateName string, err error) {
	status, message := authErrorStatus(err)
	slog.Warn("authentication error encountered", "err", err, "message", message)
	s.renderFragment(w, status, templateName, errorFragment(message))
}
