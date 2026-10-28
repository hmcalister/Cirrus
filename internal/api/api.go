package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hmcalister/cirrus/internal/filestore"
)

type ServerConfig struct {
	AuthService       authService
	Users             userStore
	Files             filestore.FileStore
	SessionCookieName string
}

type Server struct {
	ServerConfig

	api huma.API
	mux *http.ServeMux

	// Enables deterministic request durations in tests. Never use time.Now directly.
	now func() time.Time
}

func NewServer(mux *http.ServeMux, humaAPI huma.API, cfg ServerConfig) (*Server, error) {
	if mux == nil {
		return nil, errors.New("mux must be non-nil")
	}
	if humaAPI == nil {
		return nil, errors.New("humaAPI must be non-nil")
	}
	if cfg.AuthService == nil {
		return nil, errors.New("authService must be non-nil")
	}
	if cfg.Users == nil {
		return nil, errors.New("users must be non-nil")
	}
	if cfg.Files == nil {
		return nil, errors.New("files must be non-nil")
	}
	if cfg.SessionCookieName == "" {
		return nil, errors.New("sessionCookieName must be non-empty")
	}

	server := &Server{
		ServerConfig: cfg,
		mux:          mux,
		api:          humaAPI,
		now:          time.Now,
	}
	server.registerOperations()

	// Everything not claimed by a Huma operation or Stratus is a Vue route.
	mux.Handle("GET /", spaHandler())

	return server, nil
}

// Returns the Huma configuration used by the server.
func NewHumaConfig() huma.Config {
	return huma.DefaultConfig("Cirrus API", "2.0.0")
}

func (s *Server) Routes() http.Handler {
	return s.logRequests(s.mux)
}

// Returns the OpenAPI document built by Huma.
func (s *Server) OpenAPI() *huma.OpenAPI {
	return s.api.OpenAPI()
}
