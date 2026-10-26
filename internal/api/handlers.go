package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/template"
)

const (
	htmlContentType = "text/html; charset=utf-8"
	textContentType = "text/plain; charset=utf-8"
)

// Shared Huma output types.

type htmlOutput struct {
	Status      int    `doc:"HTTP status code."`
	ContentType string `header:"Content-Type" doc:"Response media type."`
	Body        []byte `doc:"Response body."`
}

type redirectOutput struct {
	Status     int      `doc:"HTTP status code."`
	Location   string   `header:"Location" doc:"Redirect target."`
	HXRedirect string   `header:"HX-Redirect" doc:"HTMX redirect target."`
	SetCookie  []string `header:"Set-Cookie" doc:"Cookies to set."`
}

type htmlRedirectOutput struct {
	Status      int      `doc:"HTTP status code."`
	HXRedirect  string   `header:"HX-Redirect" doc:"HTMX redirect target."`
	SetCookie   []string `header:"Set-Cookie" doc:"Cookies to set."`
	ContentType string   `header:"Content-Type" doc:"Response media type when returning an error fragment."`
	Body        []byte   `doc:"Error fragment when the redirect could not be produced."`
}

// Page data for rendering templates.

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

// Registers every application operation with Huma.
func (s *Server) registerOperations() {
	s.registerRootOperation()
	s.registerHealthOperation()
	s.registerSignInPageOperation()
	s.registerLogoutOperation()
	s.registerAccountPageOperation()
	s.registerSetActiveOperation()
	s.registerLogoutAllOperation()
	s.registerDeleteOperation()
}

// --------------------------------------------------------------------------------
// RESPONSE HELPERS

func htmlResponses(statuses ...int) map[string]*huma.Response {
	responses := make(map[string]*huma.Response, len(statuses))
	for _, status := range statuses {
		responses[strconv.Itoa(status)] = &huma.Response{
			Description: http.StatusText(status),
			Content: map[string]*huma.MediaType{
				"text/html": {Schema: &huma.Schema{Type: "string"}},
			},
		}
	}
	return responses
}

func textResponses(statuses ...int) map[string]*huma.Response {
	responses := make(map[string]*huma.Response, len(statuses))
	for _, status := range statuses {
		responses[strconv.Itoa(status)] = &huma.Response{
			Description: http.StatusText(status),
			Content: map[string]*huma.MediaType{
				"text/plain": {Schema: &huma.Schema{Type: "string"}},
			},
		}
	}
	return responses
}

func redirectResponses(status int) map[string]*huma.Response {
	return map[string]*huma.Response{
		strconv.Itoa(status): {Description: http.StatusText(status)},
	}
}

// --------------------------------------------------------------------------------
// ROOT

type rootInput struct{}

func (s *Server) registerRootOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "get-root",
		Method:        http.MethodGet,
		Path:          "/",
		Tags:          []string{"pages"},
		Summary:       "Redirect to the account page",
		DefaultStatus: http.StatusSeeOther,
		Responses:     redirectResponses(http.StatusSeeOther),
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleRoot)
}

func (s *Server) handleRoot(_ context.Context, _ *rootInput) (*redirectOutput, error) {
	return &redirectOutput{Status: http.StatusSeeOther, Location: "/account"}, nil
}

// --------------------------------------------------------------------------------
// HEALTH

type healthInput struct{}

func (s *Server) registerHealthOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "get-health",
		Method:        http.MethodGet,
		Path:          "/healthz",
		Tags:          []string{"system"},
		Summary:       "Health check",
		DefaultStatus: http.StatusOK,
		Responses:     textResponses(http.StatusOK),
	}, s.handleHealth)
}

func (s *Server) handleHealth(_ context.Context, _ *healthInput) (*htmlOutput, error) {
	return &htmlOutput{Status: http.StatusOK, ContentType: textContentType, Body: []byte("ok")}, nil
}

// --------------------------------------------------------------------------------
// SIGN IN PAGE

type signInPageInput struct{}

func (s *Server) registerSignInPageOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "get-sign-in-page",
		Method:        http.MethodGet,
		Path:          "/sign-in",
		Tags:          []string{"pages"},
		Summary:       "Sign-in page",
		DefaultStatus: http.StatusOK,
		Responses:     htmlResponses(http.StatusOK, http.StatusInternalServerError),
	}, s.handleSignInPage)
}

func (s *Server) handleSignInPage(_ context.Context, _ *signInPageInput) (*htmlOutput, error) {
	out := s.renderHTML(template.TemplatePageSignIn, nil)
	return &out, nil
}

// --------------------------------------------------------------------------------
// LOGOUT

type logoutInput struct {
	Cookie string `header:"Cookie" doc:"Session cookie to revoke."`
}

func (s *Server) registerLogoutOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "post-logout",
		Method:        http.MethodPost,
		Path:          "/logout",
		Tags:          []string{"auth"},
		Summary:       "Sign out",
		DefaultStatus: http.StatusSeeOther,
		Responses:     htmlResponses(http.StatusSeeOther, http.StatusInternalServerError),
	}, s.handleLogout)
}

func (s *Server) handleLogout(ctx context.Context, in *logoutInput) (*htmlRedirectOutput, error) {
	rawToken := sessionTokenFromCookieHeader(in.Cookie, s.SessionCookieName)

	if err := s.AuthService.Logout(ctx, rawToken); err != nil {
		slog.Warn("error when signing out", "err", err)
		out := s.renderAuthError("account_error", err)
		return &htmlRedirectOutput{Status: out.Status, ContentType: out.ContentType, Body: out.Body}, nil
	}

	return &htmlRedirectOutput{
		Status:     http.StatusSeeOther,
		HXRedirect: "/sign-in",
		SetCookie:  []string{s.clearSessionCookie().String()},
	}, nil
}

// --------------------------------------------------------------------------------
// ACCOUNT PAGE

type accountPageInput struct{}

func (s *Server) registerAccountPageOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "get-account",
		Method:        http.MethodGet,
		Path:          "/account",
		Tags:          []string{"account"},
		Summary:       "Account page",
		DefaultStatus: http.StatusOK,
		Responses:     htmlResponses(http.StatusOK, http.StatusInternalServerError),
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleAccountPage)
}

func (s *Server) handleAccountPage(ctx context.Context, _ *accountPageInput) (*htmlOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		// ensureUser guarantees a user in the context.
		return &htmlOutput{
			Status:      http.StatusInternalServerError,
			ContentType: htmlContentType,
			Body:        []byte(genericErrorMessage),
		}, nil
	}

	out := s.renderHTML(template.TemplatePageAccount, accountPageData{Email: user.Email, Active: user.Active})
	return &out, nil
}

// --------------------------------------------------------------------------------
// SET ACTIVE

type setActiveInput struct {
	Body struct {
		ActiveStatus string `json:"activeStatus" form:"activeStatus" enum:"true,false" doc:"Whether daily emails should be delivered."`
	} `contentType:"application/x-www-form-urlencoded"`
}

func (s *Server) registerSetActiveOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:      "post-account-set-active",
		Method:           http.MethodPost,
		Path:             "/account/setActive",
		Tags:             []string{"account"},
		Summary:          "Pause or resume daily emails",
		DefaultStatus:    http.StatusOK,
		Responses:        htmlResponses(http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError),
		Middlewares:      huma.Middlewares{s.requireUser},
		SkipValidateBody: true,
		MaxBodyBytes:     1 << 16,
		BodyReadTimeout:  -1,
	}, s.handleSetActive)
}

func (s *Server) handleSetActive(ctx context.Context, in *setActiveInput) (*htmlOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		return &htmlOutput{
			Status:      http.StatusInternalServerError,
			ContentType: htmlContentType,
			Body:        []byte(genericErrorMessage),
		}, nil
	}

	if in.Body.ActiveStatus != "true" && in.Body.ActiveStatus != "false" {
		slog.Warn("set active request sent with bad value", "activeStatus", in.Body.ActiveStatus)
		out := s.renderFragment(http.StatusBadRequest, "account_error", errorFragment("invalid active status."))
		return &out, nil
	}

	activeStatus := in.Body.ActiveStatus == "true"
	if err := s.Users.SetActive(ctx, database.SetActiveParams{Email: user.Email, Active: activeStatus}); err != nil {
		slog.Warn("error when setting account active", "err", err)
		out := s.renderAuthError("account_error", err)
		return &out, nil
	}

	slog.Info("account active status changed",
		"request_id", requestIDFromContext(ctx),
		"email_hash", hashEmail(user.Email),
		"active", activeStatus,
	)

	message := "Daily emails paused."
	if activeStatus {
		message = "Daily emails resumed."
	}
	out := s.renderFragment(http.StatusOK, "account_active_status", accountStatusFragment{
		Kind:    "status",
		Message: message,
		Active:  activeStatus,
	})
	return &out, nil
}

// --------------------------------------------------------------------------------
// LOGOUT ALL

type logoutAllInput struct{}

func (s *Server) registerLogoutAllOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "post-account-logout-all",
		Method:        http.MethodPost,
		Path:          "/account/logout-all",
		Tags:          []string{"account"},
		Summary:       "Sign out of every device",
		DefaultStatus: http.StatusSeeOther,
		Responses:     htmlResponses(http.StatusSeeOther, http.StatusInternalServerError),
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleLogoutAll)
}

func (s *Server) handleLogoutAll(ctx context.Context, _ *logoutAllInput) (*htmlRedirectOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		return &htmlRedirectOutput{
			Status:      http.StatusInternalServerError,
			ContentType: htmlContentType,
			Body:        []byte(genericErrorMessage),
		}, nil
	}

	if err := s.AuthService.LogoutAll(ctx, user.Email); err != nil {
		slog.Warn("error when signing out all sessions", "err", err)
		out := s.renderAuthError("account_error", err)
		return &htmlRedirectOutput{Status: out.Status, ContentType: out.ContentType, Body: out.Body}, nil
	}

	slog.Info("all sessions signed out", "request_id", requestIDFromContext(ctx), "email_hash", hashEmail(user.Email))
	return &htmlRedirectOutput{
		Status:     http.StatusSeeOther,
		HXRedirect: "/sign-in",
		SetCookie:  []string{s.clearSessionCookie().String()},
	}, nil
}

// --------------------------------------------------------------------------------
// DELETE ACCOUNT

type deleteAccountInput struct{}

func (s *Server) registerDeleteOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "post-account-delete",
		Method:        http.MethodPost,
		Path:          "/account/delete",
		Tags:          []string{"account"},
		Summary:       "Delete the account permanently",
		DefaultStatus: http.StatusSeeOther,
		Responses:     htmlResponses(http.StatusSeeOther, http.StatusInternalServerError),
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleDelete)
}

func (s *Server) handleDelete(ctx context.Context, _ *deleteAccountInput) (*htmlRedirectOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		return &htmlRedirectOutput{
			Status:      http.StatusInternalServerError,
			ContentType: htmlContentType,
			Body:        []byte(genericErrorMessage),
		}, nil
	}

	// Revoke sessions first. If the profile delete then fails, the user is
	// signed out rather than left with a session that would recreate a profile.
	if err := s.AuthService.LogoutAll(ctx, user.Email); err != nil {
		slog.Warn("error when signing out before account deletion", "err", err)
		out := s.renderAuthError("account_error", err)
		return &htmlRedirectOutput{Status: out.Status, ContentType: out.ContentType, Body: out.Body}, nil
	}

	if err := s.Users.DeleteUser(ctx, user.Email); err != nil {
		slog.Warn("error when deleting account", "err", err)
		out := s.renderAuthError("account_error", err)
		return &htmlRedirectOutput{Status: out.Status, ContentType: out.ContentType, Body: out.Body}, nil
	}

	slog.Info("account deleted", "request_id", requestIDFromContext(ctx), "email_hash", hashEmail(user.Email))
	return &htmlRedirectOutput{
		Status:     http.StatusSeeOther,
		HXRedirect: "/sign-in",
		SetCookie:  []string{s.clearSessionCookie().String()},
	}, nil
}
