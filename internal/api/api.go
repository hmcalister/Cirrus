package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/template"
	"github.com/hmcalister/stratus/pkg/auth"
)

// Interface over the Stratus authentication service.
type authService interface {
	ValidateSession(ctx context.Context, rawToken string) (auth.Identity, error)
	Logout(ctx context.Context, rawToken string) error
	LogoutAll(ctx context.Context, userEmail string) error
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
	AuthHandlers      http.Handler
	PagesRenderer     template.PageRenderer
	CookieSecure      bool
	SessionCookieName string
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
	if cfg.Users == nil {
		return nil, errors.New("users must be non-nil")
	}
	if cfg.PagesRenderer == nil {
		return nil, errors.New("pagesRenderer must be non-nil")
	}
	if cfg.SessionCookieName == "" {
		return nil, errors.New("sessionCookieName must be non-empty")
	}
	if cfg.AuthHandlers == nil {
		cfg.AuthHandlers = http.NotFoundHandler()
	}

	return &Server{ServerConfig: cfg, now: time.Now}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Stratus serves the magic-link routes:
	//
	//	POST /auth/magic-link
	//	GET  /auth/magic-link/consume
	//	POST /auth/logout
	//	POST /auth/logout-all
	mux.Handle("/auth/", s.AuthHandlers)

	mux.Handle("GET /{$}", s.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/account", http.StatusSeeOther)
	})))

	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /sign-in", s.handleSignInPage)
	mux.HandleFunc("POST /logout", s.handleLogout)

	mux.Handle("GET /static/", staticHandler())

	accountRoutes := http.NewServeMux()
	accountRoutes.HandleFunc("GET /account", s.handleAccountPage)
	accountRoutes.HandleFunc("POST /account/setActive", s.handleSetActive)
	accountRoutes.HandleFunc("POST /account/logout-all", s.handleLogoutAll)
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

// Clears the session cookie.
//
// Sets an empty value with an immediate expiry so the browser drops it.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	// Expires the session cookie immediately, used to log out.
	const sessionCookieMaxAge = -1

	http.SetCookie(w, &http.Cookie{
		Name:     s.SessionCookieName,
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
