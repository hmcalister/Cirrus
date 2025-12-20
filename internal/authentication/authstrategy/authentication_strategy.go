package authstrategy

import (
	"fmt"
	"net/http"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/claims"
	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"
	"github.com/hmcalister/LiteralCloudService/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthenticationStrategy interface {
	// Return the auth_type string for this strategy (e.g., "password", "oauth2_google")
	GetAuthType() string

	// Defines all routes to be used for this strategy.
	// Will be publicly available on `/auth/{authType}/...` but those prefixes will be stripped.
	//
	// All strategies should have some method that give the user a token with their claims.
	// This should be done by calling the `authManager.returnAuthToken` method
	GetRouter() *http.ServeMux

	// Set the database connection for the strategy
	//
	// This is intended to used by `AuthenticationManager.RegisterService`.
	// This does not have to be called before registering the service.
	// This method should NOT be called manually!
	SetDatabase(db *database.Queries, connPool *pgxpool.Pool)

	// Set the AuthTokenManager for the strategy.
	//
	// This is intended to used by `AuthenticationManager.RegisterService`.
	// This does not have to be called before registering the service.
	// This method should NOT be called manually!
	SetAuthTokenManager(authTokenManager token.AuthTokenManager)
}

type baseAuthenticationStrategy struct {
	db               *database.Queries
	connPool         *pgxpool.Pool
	authTokenManager token.AuthTokenManager
	router           *http.ServeMux
}

func newBaseAuthenticationStrategy() *baseAuthenticationStrategy {
	return &baseAuthenticationStrategy{
		router: http.NewServeMux(),
	}
}

func (base *baseAuthenticationStrategy) GetRouter() *http.ServeMux {
	return base.router
}

func (base *baseAuthenticationStrategy) SetDatabase(db *database.Queries, connPool *pgxpool.Pool) {
	base.db = db
	base.connPool = connPool
}

func (base *baseAuthenticationStrategy) SetAuthTokenManager(authTokenManager token.AuthTokenManager) {
	base.authTokenManager = authTokenManager
}

// Respond to the authentication request with the corresponding auth token
// Only to be called once a user has been authenticated
func (base *baseAuthenticationStrategy) respondWithAuthToken(w http.ResponseWriter, user database.User) {
	claims := claims.UserToTokenClaims(user)
	authToken := base.authTokenManager.CreateToken(claims)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"authToken": "%s"}`, authToken)))
}
