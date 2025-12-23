package authentication

import (
	"fmt"
	"net/http"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/authstrategy"
	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"
	"github.com/hmcalister/LiteralCloudService/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Create an authentication manager to handle auth for the app.
// Flows look like creating a manager, add strategies (AddStrategy),
// and mounting the authSubrouter to the `/auth` route.
//
// Strategies will detail what routes will exist.
// Users will receive auth tokens which make (signed) claims about identity from
// strategies in the subroutes.
//
// Once created, mount the subrouter to the `/auth` route.
type AuthenticationManager struct {
	db                      *database.Queries
	connPool                *pgxpool.Pool
	authTokenManager        token.AuthTokenManager
	strategies              map[string]authstrategy.AuthenticationStrategy
	authenticationSubrouter *http.ServeMux
}

func NewAuthenticationManager(
	db *database.Queries,
	connPool *pgxpool.Pool,
	authTokenManager token.AuthTokenManager,
) *AuthenticationManager {
	am := &AuthenticationManager{
		strategies:              make(map[string]authstrategy.AuthenticationStrategy),
		db:                      db,
		connPool:                connPool,
		authTokenManager:        authTokenManager,
		authenticationSubrouter: http.NewServeMux(),
	}

	return am
}

// This subrouter must be mounted to `/auth`
// Requests should look like `/auth/{authType}/...`
//
// For example, password auth would make requests to `/auth/password`
// OAuth2 with Google would make requests to `/auth/oauth2_google` and callback `/auth/oauth2_google/callback`
// Email validation requests should be made to `/auth/validate_email/{code}`
func (am *AuthenticationManager) GetAuthenticationSubrouter() *http.ServeMux {
	return am.authenticationSubrouter
}

func (am *AuthenticationManager) RegisterStrategy(strategy authstrategy.AuthenticationStrategy) {
	authType := strategy.GetAuthType()
	if _, ok := am.strategies[authType]; ok {
		// Duplicate strategy to be registered, ignore
		return
	}
	am.strategies[authType] = strategy

	strategy.SetDatabase(am.db, am.connPool)
	strategy.SetAuthTokenManager(am.authTokenManager)

	am.authenticationSubrouter.Handle(
		fmt.Sprintf("/%s/", authType),
		http.StripPrefix(fmt.Sprintf("/%s", authType), strategy.GetRouter()),
	)
}
