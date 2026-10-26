package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/web"
)

// --------------------------------------------------------------------------------
// Standard (non-Huma) Handlers

// Serves the embedded Vue build. Unknown paths fall back to index.html so
// client-side routes survive a refresh.
func spaHandler() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic("api: embedded web build missing: " + err.Error())
	}

	fileServer := http.FileServerFS(dist)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, "auth/") {
			http.NotFound(w, r)
			return
		}

		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(dist, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}

// --------------------------------------------------------------------------------
// Huma Operations

func (s *Server) registerOperations() {
	s.registerHealthOperation()
	s.registerAccountOperation()
	s.registerSetActiveOperation()
	s.registerDeleteAccountOperation()
}

// --------------------------------------------------------------------------------
// HEALTH

type healthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

func (s *Server) registerHealthOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "get-health",
		Method:        http.MethodGet,
		Path:          "/healthz",
		Tags:          []string{"system"},
		Summary:       "Health check",
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

// --------------------------------------------------------------------------------
// ACCOUNT

type Account struct {
	Email  string `json:"email" doc:"Signed-in email address."`
	Active bool   `json:"active" doc:"Whether daily emails are delivered."`
}

type accountOutput struct {
	Body Account
}

func (s *Server) registerAccountOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "get-account",
		Method:        http.MethodGet,
		Path:          "/api/account",
		Tags:          []string{"account"},
		Summary:       "Get the signed-in account",
		DefaultStatus: http.StatusOK,
		Errors:        []int{http.StatusUnauthorized, http.StatusInternalServerError},
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleAccount)
}

func (s *Server) handleAccount(ctx context.Context, _ *struct{}) (*accountOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		return nil, huma.Error500InternalServerError("account profile unavailable")
	}

	out := &accountOutput{}
	out.Body.Email = user.Email
	out.Body.Active = user.Active
	return out, nil
}

// --------------------------------------------------------------------------------
// SET ACTIVE

type SetActiveInput struct {
	Active bool `json:"active" required:"true" doc:"Whether daily emails should be delivered."`
}

type setActiveInput struct {
	Body SetActiveInput
}

type SetActiveOutput struct {
	Active bool `json:"active" doc:"Whether daily emails are delivered."`
}

type setActiveOutput struct {
	Body SetActiveOutput
}

func (s *Server) registerSetActiveOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "post-account-active",
		Method:        http.MethodPost,
		Path:          "/api/account/active",
		Tags:          []string{"account"},
		Summary:       "Pause or resume daily emails",
		DefaultStatus: http.StatusOK,
		Errors:        []int{http.StatusUnauthorized, http.StatusInternalServerError},
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleSetActive)
}

func (s *Server) handleSetActive(ctx context.Context, in *setActiveInput) (*setActiveOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		return nil, huma.Error500InternalServerError("account profile unavailable")
	}

	if err := s.Users.SetActive(ctx, database.SetActiveParams{Email: user.Email, Active: in.Body.Active}); err != nil {
		slog.Warn("error when setting account active", "request_id", requestIDFromContext(ctx), "err", err)
		return nil, huma.Error500InternalServerError("unable to update account")
	}

	slog.Info("account active status changed",
		"request_id", requestIDFromContext(ctx),
		"email_hash", hashEmail(user.Email),
		"active", in.Body.Active,
	)

	out := &setActiveOutput{}
	out.Body.Active = in.Body.Active
	return out, nil
}

// --------------------------------------------------------------------------------
// DELETE ACCOUNT

type deleteAccountOutput struct{}

func (s *Server) registerDeleteAccountOperation() {
	huma.Register(s.api, huma.Operation{
		OperationID:   "delete-account",
		Method:        http.MethodDelete,
		Path:          "/api/account",
		Tags:          []string{"account"},
		Summary:       "Delete the account permanently",
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusInternalServerError},
		Middlewares:   huma.Middlewares{s.requireUser},
	}, s.handleDeleteAccount)
}

func (s *Server) handleDeleteAccount(ctx context.Context, _ *struct{}) (*deleteAccountOutput, error) {
	user, ok := userFromContext(ctx)
	if !ok {
		return nil, huma.Error500InternalServerError("account profile unavailable")
	}

	// Revoke sessions first. If the profile delete then fails, the user is
	// signed out rather than left with a session that would recreate a profile.
	if err := s.AuthService.LogoutAll(ctx, user.Email); err != nil {
		slog.Warn("error when signing out before account deletion", "request_id", requestIDFromContext(ctx), "err", err)
		return nil, huma.Error500InternalServerError("unable to delete account")
	}

	if err := s.Users.DeleteUser(ctx, user.Email); err != nil {
		slog.Warn("error when deleting account", "request_id", requestIDFromContext(ctx), "err", err)
		return nil, huma.Error500InternalServerError("unable to delete account")
	}

	slog.Info("account deleted", "request_id", requestIDFromContext(ctx), "email_hash", hashEmail(user.Email))
	return &deleteAccountOutput{}, nil
}
