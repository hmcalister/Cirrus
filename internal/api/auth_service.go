package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hmcalister/stratus/pkg/auth"
)

// Interface over the Stratus authentication service.
type authService interface {
	ValidateSession(ctx context.Context, rawToken string) (auth.Identity, error)
	LogoutAll(ctx context.Context, userEmail string) error
	RegisterHumaHandlers(api huma.API)
}

var _ authService = (*auth.AuthService)(nil)
