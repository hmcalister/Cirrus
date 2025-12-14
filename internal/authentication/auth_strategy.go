package authentication

import (
	"context"
	"errors"

	"github.com/hmcalister/LiteralCloudService/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrAuthMethodExists   = errors.New("authentication method already exists for this user")
)

// AuthenticationStrategy defines the interface for different authentication methods
// Each strategy implements how to authenticate users and register new auth methods
type AuthenticationStrategy interface {
	// GetAuthType returns the auth_type string for this strategy (e.g., "password", "oauth2_google")
	GetAuthType() string

	// Authenticate verifies credentials and returns the user if successful
	// For password auth: credentials contains email and password
	// For OAuth2: credentials contains provider tokens/code
	// Receives queries for database access (can be regular or transaction-scoped)
	Authenticate(ctx context.Context, queries *database.Queries, credentials map[string]string) (*database.User, error)

	// Register creates a new authentication method for a user
	// For password auth: registers email+password
	// For OAuth2: registers OAuth2 provider linkage
	// MUST receive transaction-scoped queries to ensure atomicity with user creation
	Register(ctx context.Context, queries *database.Queries, userID pgtype.UUID, credentials map[string]string) error

	// Link associates an existing authentication method with an existing user
	// Used to add additional auth methods to a user's account
	// Should receive transaction-scoped queries if linking needs to be atomic with other operations
	Link(ctx context.Context, queries *database.Queries, userID pgtype.UUID, credentials map[string]string) error
}

// AuthenticationManager manages multiple authentication strategies
type AuthenticationManager struct {
	strategies map[string]AuthenticationStrategy
	db         *database.Queries
	conn       *pgx.Conn
}

// NewAuthenticationManager creates a new authentication manager
func NewAuthenticationManager(db *database.Queries, conn *pgx.Conn) *AuthenticationManager {
	return &AuthenticationManager{
		strategies: make(map[string]AuthenticationStrategy),
		db:         db,
		conn:       conn,
	}
}

// RegisterStrategy adds a new authentication strategy to the manager
func (am *AuthenticationManager) RegisterStrategy(strategy AuthenticationStrategy) {
	am.strategies[strategy.GetAuthType()] = strategy
}

// GetStrategy retrieves a strategy by auth type
func (am *AuthenticationManager) GetStrategy(authType string) (AuthenticationStrategy, error) {
	strategy, exists := am.strategies[authType]
	if !exists {
		return nil, errors.New("authentication strategy not found")
	}
	return strategy, nil
}

// Authenticate attempts to authenticate using the specified auth type
// Uses regular (non-transactional) queries since authentication is read-only
func (am *AuthenticationManager) Authenticate(ctx context.Context, authType string, credentials map[string]string) (*database.User, error) {
	strategy, err := am.GetStrategy(authType)
	if err != nil {
		return nil, err
	}

	user, err := strategy.Authenticate(ctx, am.db, credentials)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Register creates a new user with the specified authentication method using a transaction
// This ensures atomicity: both user creation and auth method creation succeed or both fail
func (am *AuthenticationManager) Register(ctx context.Context, authType string, email string, credentials map[string]string) (*database.User, error) {
	strategy, err := am.GetStrategy(authType)
	if err != nil {
		return nil, err
	}

	// Begin transaction for atomic user + auth method creation
	tx, err := am.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create transaction-scoped queries
	txQueries := am.db.WithTx(tx)

	// Create user within transaction
	user, err := txQueries.CreateUser(ctx, email)
	if err != nil {
		return nil, err
	}

	// Register authentication method within same transaction
	// If this fails, user creation will be rolled back
	err = strategy.Register(ctx, txQueries, user.UserID, credentials)
	if err != nil {
		return nil, err
	}

	// Commit transaction - both user and auth method are now persisted
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &user, nil
}

// LinkAuthMethod adds an additional authentication method to an existing user
// Uses a transaction to ensure the link operation is atomic
func (am *AuthenticationManager) LinkAuthMethod(ctx context.Context, userID pgtype.UUID, authType string, credentials map[string]string) error {
	strategy, err := am.GetStrategy(authType)
	if err != nil {
		return err
	}

	// Begin transaction for atomic link operation
	tx, err := am.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Create transaction-scoped queries
	txQueries := am.db.WithTx(tx)

	// Link authentication method within transaction
	err = strategy.Link(ctx, txQueries, userID, credentials)
	if err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
