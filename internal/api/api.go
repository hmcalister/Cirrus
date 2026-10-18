package api

import (
	"errors"
	"net/http"

	"github.com/hmcalister/Cirrus/internal/authentication"
	"github.com/hmcalister/Cirrus/internal/database"
	"github.com/hmcalister/Cirrus/internal/email/sender"
	"github.com/hmcalister/Cirrus/internal/template"
)

const (
	authenticationCookieName = "authenticationCookie"
)

type ServerConfig struct {
	Querier       database.Querier
	Sender        sender.Sender
	TokenLinks    authentication.TokenLinkBuilder
	AuthService   *authentication.AuthenticationService
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
