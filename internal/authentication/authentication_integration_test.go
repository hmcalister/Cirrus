package authentication_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hmcalister/LiteralCloudService/internal/authentication"
	"github.com/hmcalister/LiteralCloudService/internal/authentication/authstrategy"
	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"
	"github.com/hmcalister/LiteralCloudService/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

// Integration tests for authentication
// Uses a real database which should be set up and configured for use by the variables in secrets/.env.test
//
// Inserted users will have emails like test_%@example.com
//
// OAuth2 strategies are added to the manager but the entire flow is not tested.
// Calling out to the providers is a TODO.
// Beware!

type testSetup struct {
	db               *database.Queries
	connPool         *pgxpool.Pool
	authTokenManager token.AuthTokenManager
	authManager      *authentication.AuthenticationManager
	ctx              context.Context
}

func setupIntegrationTest(t *testing.T) *testSetup {
	t.Helper()

	if os.Getenv("ENVIRONMENT") != "TESTING" {
		t.Fatalf("Cannot run integration test in non testing environment. ENVIRONMENT variable is \"%s\", not \"TESTING\". Are you sure you have set the test environment correctly? Use secrets/.env.test", os.Getenv("ENVIRONMENT"))
	}

	// defaultLoggerHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	// 	Level: slog.LevelDebug,
	// })
	// slog.SetDefault(slog.New(defaultLoggerHandler))

	ctx := context.Background()

	dbHost := os.Getenv("POSTGRES_HOST")
	dbPort := os.Getenv("POSTGRES_PORT")
	dbUser := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")
	connString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName,
	)
	connPool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	if err := connPool.Ping(ctx); err != nil {
		connPool.Close()
		t.Fatalf("Failed to ping test database: %v", err)
	}
	db := database.New(connPool)

	// Select a specific authTokenManager
	signingSecret := os.Getenv("PASETO_SIGNING_SECRET")
	authTokenManager, err := token.NewPasetoManager(signingSecret)
	if err != nil {
		connPool.Close()
		t.Fatalf("Failed to create auth token manager: %v", err)
	}

	authManager := authentication.NewAuthenticationManager(db, connPool, authTokenManager)
	return &testSetup{
		db:               db,
		connPool:         connPool,
		authTokenManager: authTokenManager,
		authManager:      authManager,
		ctx:              ctx,
	}
}

func (s *testSetup) cleanup(t *testing.T) {
	t.Helper()
	s.connPool.Exec(s.ctx, "DELETE FROM users WHERE email LIKE 'test_%@example.com'")
	s.connPool.Close()
}

func setupMockOAuth2Provider(t *testing.T, provider string) (strategy *authstrategy.OAuth2AuthenticationStrategy, cleanup func()) {
	providerUpper := strings.ToUpper(provider)
	os.Setenv(fmt.Sprintf("OAUTH2_%s_CLIENT_ID", providerUpper), "test-client-id")
	os.Setenv(fmt.Sprintf("OAUTH2_%s_CLIENT_SECRET", providerUpper), "test-client-secret")
	cleanup = func() {
		os.Unsetenv(fmt.Sprintf("OAUTH2_%s_CLIENT_ID", providerUpper))
		os.Unsetenv(fmt.Sprintf("OAUTH2_%s_CLIENT_SECRET", providerUpper))
	}

	oauth2Strategy, err := authstrategy.NewOAuth2Strategy(
		"testprovider",
		oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
		func(token *oauth2.Token) (*http.Response, error) {
			return nil, nil
		},
		[]string{"email", "profile"},
	)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 strategy: %v", err)
	}

	return oauth2Strategy, cleanup
}

// TestOAuth2Strategy_InitiateAuthRedirect tests OAuth2 initiation
func TestOAuth2Strategy_InitiateAuthRedirect(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	oauth2Strategy, oauth2Cleanup := setupMockOAuth2Provider(t, "testprovider")
	defer oauth2Cleanup()

	setup.authManager.RegisterStrategy(oauth2Strategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	req := httptest.NewRequest("GET", "/auth/oauth2_testprovider/", nil)
	resp := httptest.NewRecorder()
	mainRouter.ServeHTTP(resp, req)

	// Should redirect to provider
	if resp.Code != http.StatusTemporaryRedirect {
		t.Errorf("Expected status %d, got %d", http.StatusTemporaryRedirect, resp.Code)
	}

	// Check that state cookie was set
	cookies := resp.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth2_state_token" {
			stateCookie = c
			break
		}
	}

	if stateCookie == nil {
		t.Fatal("Expected oauth2_state_token cookie to be set")
	}

	if stateCookie.Value == "" {
		t.Fatal("Expected non-empty state token")
	}

	// Check redirect location contains auth URL
	location := resp.Header().Get("Location")
	if !strings.Contains(location, "https://example.com/auth") {
		t.Errorf("Expected redirect to provider auth URL, got %q", location)
	}
}

// TestOAuth2Strategy_CallbackInvalidState tests CSRF protection
func TestOAuth2Strategy_CallbackInvalidState(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	oauth2Strategy, oauth2Cleanup := setupMockOAuth2Provider(t, "testprovider")
	defer oauth2Cleanup()

	setup.authManager.RegisterStrategy(oauth2Strategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	// Try callback with mismatched state
	req := httptest.NewRequest("GET", "/auth/oauth2_testprovider/callback?state=invalid&code=testcode", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth2_state_token",
		Value: "different_state",
	})
	resp := httptest.NewRecorder()

	mainRouter.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for invalid state, got %d", http.StatusUnauthorized, resp.Code)
	}
}
