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
}

type Server struct {
	ServerConfig
	cookieSecure bool
}

func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.TokenLinks == nil {
		return nil, errors.New("TokenLinks must be non-nil")
	}
	if cfg.PagesRenderer == nil {
		return nil, errors.New("PagesRender must be non-nil")
	}

	return &Server{
		ServerConfig: cfg,
		cookieSecure: true,
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.Handle("GET /me", s.RequireAuth(http.HandlerFunc(s.handleMe)))
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"email": emailFromContext(r.Context())})
}

// Middleware for authentication.
// Wraps next so that only requests carrying a valid session token reach it.
// The email of the authenticated user is placed in the request context, where handlers read it with emailFromContext.
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := sessionToken(r, authenticationCookieName)
		userEmail, ok := s.AuthService.SessionEmail(r.Context(), raw)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r.WithContext(withEmail(r.Context(), userEmail)))
	})
}
