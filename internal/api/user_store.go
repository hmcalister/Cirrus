package api

import (
	"context"

	"github.com/hmcalister/cirrus/internal/database"
)

// Interface over the application-owned user profiles.
type userStore interface {
	GetOrCreateUser(ctx context.Context, email string) (database.UserAccount, error)
	SetActive(ctx context.Context, arg database.SetActiveParams) error
	DeleteUser(ctx context.Context, email string) error
	CountUsers(ctx context.Context) (int64, error)
	CountActiveUsers(ctx context.Context) (int64, error)
}

var _ userStore = (database.Querier)(nil)

const (
	insecureSessionCookieName = "cirrus_session"
	secureSessionCookieName   = "__Host-cirrus_session"
)

// Returns the session cookie name Stratus should use for the given security
// setting. The __Host- prefix is only valid on a Secure, host-only cookie at
// the root path.
func SessionCookieName(secure bool) string {
	if secure {
		return secureSessionCookieName
	}
	return insecureSessionCookieName
}
