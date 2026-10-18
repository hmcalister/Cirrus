package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/hmcalister/Cirrus/internal/authentication"
	"github.com/hmcalister/Cirrus/internal/template"
)

// Interface over the authentication service.
type authService interface {
	Register(ctx context.Context, userEmail, password string) error
	Verify(ctx context.Context, rawToken string) (bool, error)
	Login(ctx context.Context, userEmail, password string) (string, error)
	ResendVerification(ctx context.Context, userEmail string) error
	RequestPasswordReset(ctx context.Context, userEmail string) error
	ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error
	SetAccountActive(ctx context.Context, userEmail string, active bool) error
	DeleteAccount(ctx context.Context, userEmail string) error
	SessionEmail(ctx context.Context, rawToken string) (string, bool)
	AccountActive(ctx context.Context, userEmail string) (bool, error)
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

	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("GET /register", s.handleRegisterPage)
	mux.HandleFunc("GET /reset", s.handleResetPage)
	mux.HandleFunc("GET /verify", s.handleVerify)
	mux.HandleFunc("GET /password/reset", s.handlePasswordResetRequestPage)
	mux.HandleFunc("GET /verify/resend", s.handleVerifyResendPage)

	mux.HandleFunc("POST /login", s.handleLogin)
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
