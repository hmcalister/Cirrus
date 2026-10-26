package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/template"
	"github.com/hmcalister/stratus/pkg/auth"
)

// Interface over the Stratus authentication service.
type authService interface {
	ValidateSession(ctx context.Context, rawToken string) (auth.Identity, error)
	Logout(ctx context.Context, rawToken string) error
	LogoutAll(ctx context.Context, userEmail string) error
	RegisterHumaHandlers(api huma.API)
}

var _ authService = (*auth.AuthService)(nil)

// Interface over the application-owned user profiles.
type userStore interface {
	GetOrCreateUser(ctx context.Context, email string) (database.UserAccount, error)
	SetActive(ctx context.Context, arg database.SetActiveParams) error
	DeleteUser(ctx context.Context, email string) error
}

var _ userStore = (database.Querier)(nil)

const (
	insecureSessionCookieName = "cirrus_session"
	secureSessionCookieName   = "__Host-cirrus_session"
)

// Returns the session cookie name Stratus should use for the given security
// setting. The __Host- prefix is only valid on a Secure, host-only cookie at
// the root path, which is exactly how the server configures it.
func SessionCookieName(secure bool) string {
	if secure {
		return secureSessionCookieName
	}
	return insecureSessionCookieName
}

type ServerConfig struct {
	AuthService       authService
	Users             userStore
	PagesRenderer     template.PageRenderer
	CookieSecure      bool
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
	if cfg.PagesRenderer == nil {
		return nil, errors.New("pagesRenderer must be non-nil")
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

	mux.Handle("GET /static/", staticHandler())

	return server, nil
}

// Returns the Huma configuration used by the server, including the form codec.
func NewHumaConfig() huma.Config {
	config := huma.DefaultConfig("Cirrus API", "2.0.0")

	// Huma's default formats map is shared; clone it before adding the form codec.
	formats := make(map[string]huma.Format, len(huma.DefaultFormats)+1)
	for name, format := range huma.DefaultFormats {
		formats[name] = format
	}
	formats[formMediaType] = newFormFormat()
	config.Formats = formats

	return config
}

func (s *Server) Routes() http.Handler {
	return s.logRequests(s.mux)
}

// Returns the OpenAPI document built by Huma.
func (s *Server) OpenAPI() *huma.OpenAPI {
	return s.api.OpenAPI()
}

// Returns a cookie that removes the session cookie.
func (s *Server) clearSessionCookie() *http.Cookie {
	const sessionCookieMaxAge = -1

	return &http.Cookie{
		Name:     s.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// Renders a full page. A render failure becomes the generic HTML error page.
func (s *Server) renderHTML(templateName string, data any) htmlOutput {
	body, err := s.PagesRenderer.RenderPage(templateName, data)
	if err != nil {
		slog.Error("error while rendering page", "template", templateName, "data", data, "err", err)
		return htmlOutput{
			Status:      http.StatusInternalServerError,
			ContentType: htmlContentType,
			Body:        []byte(genericErrorMessage),
		}
	}

	return htmlOutput{Status: http.StatusOK, ContentType: htmlContentType, Body: []byte(body)}
}

// Renders a fragment with the given status.
func (s *Server) renderFragment(status int, templateName string, data any) htmlOutput {
	body, err := s.PagesRenderer.RenderPage(templateName, data)
	if err != nil {
		slog.Error("error while rendering fragment", "template", templateName, "data", data, "err", err)
		return htmlOutput{
			Status:      http.StatusInternalServerError,
			ContentType: htmlContentType,
			Body:        []byte(genericErrorMessage),
		}
	}

	return htmlOutput{Status: status, ContentType: htmlContentType, Body: []byte(body)}
}

// Renders err as the named error fragment with its mapped status.
func (s *Server) renderAuthError(templateName string, err error) htmlOutput {
	status, message := authErrorStatus(err)
	slog.Warn("authentication error encountered", "err", err, "message", message)
	return s.renderFragment(status, templateName, errorFragment(message))
}
