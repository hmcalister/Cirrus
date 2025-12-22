package authentication_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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

func extractAuthToken(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()

	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status OK, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	authToken, ok := result["authToken"]
	if !ok {
		t.Fatal("Response missing authToken field")
	}

	return authToken
}

func emailPasswordToRequest(t *testing.T, email string, password string, endpoint string) (req *http.Request) {
	t.Helper()
	bodyMap := map[string]string{
		"email":    email,
		"password": password,
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		t.Fatalf("error while marshalling user to JSON: %v", err)
	}
	req = httptest.NewRequest("POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
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

func TestPasswordStrategy_FullRegistrationAndAuthenticationFlow(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	testEmail := "test_password_flow@example.com"
	testPassword := "securePassword123"

	// Test 1: Register a new user
	t.Run("Register", func(t *testing.T) {
		req := emailPasswordToRequest(t, testEmail, testPassword, "/auth/password/register")
		resp := httptest.NewRecorder()
		mainRouter.ServeHTTP(resp, req)

		authToken := extractAuthToken(t, resp)
		if authToken == "" {
			t.Fatal("Expected non-empty auth token")
		}
		claims, ok := setup.authTokenManager.VerifyToken(authToken)
		if !ok {
			t.Fatal("Auth token verification failed")
		}
		if claims["email"] != testEmail {
			t.Errorf("Expected email %q in claims, got %q", testEmail, claims["email"])
		}
	})

	// Test 2: Authenticate with correct credentials
	t.Run("AuthenticateSuccess", func(t *testing.T) {
		req := emailPasswordToRequest(t, testEmail, testPassword, "/auth/password/authenticate")
		resp := httptest.NewRecorder()
		mainRouter.ServeHTTP(resp, req)

		authToken := extractAuthToken(t, resp)
		if authToken == "" {
			t.Fatal("Expected non-empty auth token")
		}
	})

	// Test 3: Authenticate with wrong password
	t.Run("AuthenticateWrongPassword", func(t *testing.T) {
		req := emailPasswordToRequest(t, testEmail, "wrongPassword123", "/auth/password/authenticate")
		resp := httptest.NewRecorder()
		mainRouter.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.Code)
		}
	})

	// Test 4: Try to register duplicate user
	t.Run("RegisterDuplicate", func(t *testing.T) {
		req := emailPasswordToRequest(t, testEmail, "anotherPassword123", "/auth/password/register")
		resp := httptest.NewRecorder()
		mainRouter.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.Code)
		}
	})

	// Test 5: Authenticate with non-existent user
	t.Run("AuthenticateNonExistent", func(t *testing.T) {
		req := emailPasswordToRequest(t, "nonexistent@example.com", "nonexistentPassword123", "/auth/password/register")
		resp := httptest.NewRecorder()
		mainRouter.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.Code)
		}
	})
}

// TestPasswordStrategy_WeakPasswordRejection tests password validation
func TestPasswordStrategy_WeakPasswordRejection(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	testEmail := "test_weak_password@example.com"
	weakPassword := "short"

	req := emailPasswordToRequest(t, testEmail, weakPassword, "/auth/password/register")
	resp := httptest.NewRecorder()
	mainRouter.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for weak password, got %d", http.StatusBadRequest, resp.Code)
	}
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

// TestAuthenticationManager_MultipleStrategies tests registering multiple strategies
func TestAuthenticationManager_MultipleStrategies(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	// Register password strategy
	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	oauth2Strategy, oauth2Cleanup := setupMockOAuth2Provider(t, "testprovider")
	defer oauth2Cleanup()

	setup.authManager.RegisterStrategy(oauth2Strategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	// Test that both strategies are accessible
	t.Run("PasswordStrategyAccessible", func(t *testing.T) {
		reqBody := map[string]string{
			"email":    "test_multi@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/password/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		mainRouter.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("Expected password strategy to be accessible, got status %d", resp.Code)
		}
	})

	// TODO: Implement OAuth2 providers to test this path!
	// t.Run("OAuth2StrategyAccessible", func(t *testing.T) {
	// 	req := httptest.NewRequest("GET", "/auth/oauth2_google/", nil)
	// 	resp := httptest.NewRecorder()

	// 	mainRouter.ServeHTTP(resp, req)

	// 	if resp.Code != http.StatusTemporaryRedirect {
	// 		t.Errorf("Expected OAuth2 strategy to be accessible, got status %d", resp.Code)
	// 	}
	// })
}

// TestAuthenticationManager_DuplicateStrategyRegistration tests that duplicate strategies are ignored
func TestAuthenticationManager_DuplicateStrategyRegistration(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy1 := authstrategy.NewPasswordStrategy()
	passwordStrategy2 := authstrategy.NewPasswordStrategy()

	setup.authManager.RegisterStrategy(passwordStrategy1)
	setup.authManager.RegisterStrategy(passwordStrategy2)

	// Should not panic and second registration should be ignored
	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	testEmail := "test_duplicate@example.com"
	testPassword := "password123"
	req := emailPasswordToRequest(t, testEmail, testPassword, "/auth/password/register")
	resp := httptest.NewRecorder()
	mainRouter.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", resp.Code)
	}
}

// TestPasswordStrategy_MissingFormFields tests validation of form data
func TestPasswordStrategy_MissingFormFields(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{"MissingEmail", "", "password123"},
		{"MissingPassword", "test@example.com", ""},
		{"MissingBoth", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := emailPasswordToRequest(t, tt.email, tt.password, "/auth/password/register")
			resp := httptest.NewRecorder()
			mainRouter.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d for missing fields, got %d", http.StatusBadRequest, resp.Code)
			}
		})
	}
}

func TestPasswordStrategy_InvalidFormData(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	// Send invalid form data
	req := httptest.NewRequest("POST", "/auth/password/register", strings.NewReader("invalid%form%data"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	mainRouter.ServeHTTP(resp, req)

	// Should either parse successfully (treating as empty fields) or fail parsing
	// Either way, should get BadRequest due to missing fields
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid form data, got %d", http.StatusBadRequest, resp.Code)
	}
}

// TestAuthToken_ValidityAndClaims tests that generated auth tokens contain correct claims
func TestAuthToken_ValidityAndClaims(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	testEmail := "test_token_claims@example.com"
	testPassword := "password123"

	reqBody := map[string]string{
		"email":    testEmail,
		"password": testPassword,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/auth/password/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	mainRouter.ServeHTTP(resp, req)

	authToken := extractAuthToken(t, resp)

	// Verify token
	claims, ok := setup.authTokenManager.VerifyToken(authToken)
	if !ok {
		t.Fatal("Token verification failed")
	}

	// Check claims
	if claims["email"] != testEmail {
		t.Errorf("Expected email claim %q, got %q", testEmail, claims["email"])
	}

	if claims["id"] == nil {
		t.Error("Expected id claim to be present")
	}
}

// TestPasswordStrategy_ConcurrentRegistrations tests thread safety
func TestPasswordStrategy_ConcurrentRegistrations(t *testing.T) {
	setup := setupIntegrationTest(t)
	defer setup.cleanup(t)

	passwordStrategy := authstrategy.NewPasswordStrategy()
	setup.authManager.RegisterStrategy(passwordStrategy)

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/auth/", http.StripPrefix("/auth", setup.authManager.GetAuthenticationSubrouter()))

	// Try to register the same user concurrently
	testEmail := "test_concurrent@example.com"
	testPassword := "password123"

	done := make(chan bool, 2)
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 2; i++ {
		go func() {
			reqBody := map[string]string{
				"email":    testEmail,
				"password": testPassword,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/auth/password/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			mainRouter.ServeHTTP(resp, req)

			if resp.Code == http.StatusOK {
				mu.Lock()
				successCount++
				mu.Unlock()
			}

			done <- true
		}()
	}

	// Wait for both goroutines
	<-done
	<-done

	// Exactly one should succeed
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful registration, got %d", successCount)
	}
}
