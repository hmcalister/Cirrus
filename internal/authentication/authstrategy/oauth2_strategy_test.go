package authstrategy_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/authstrategy"
	"golang.org/x/oauth2"
)

// Compile-time check that OAuth2AuthenticationStrategy implements AuthenticationStrategy
var _ authstrategy.AuthenticationStrategy = &authstrategy.OAuth2AuthenticationStrategy{}

const testProviderName = "testprovider"

// setupTestOAuth2Strategy creates a test OAuth2 strategy with environment variables set
// Returns the strategy and a cleanup function that should be deferred
func setupTestOAuth2Strategy(t *testing.T) (strategy *authstrategy.OAuth2AuthenticationStrategy, cleanup func()) {
	t.Helper()

	os.Setenv("OAUTH2_TESTPROVIDER_CLIENT_ID", "testprovider-client-id")
	os.Setenv("OAUTH2_TESTPROVIDER_CLIENT_SECRET", "testprovider-client-secret")

	cleanup = func() {
		os.Unsetenv("OAUTH2_TESTPROVIDER_CLIENT_ID")
		os.Unsetenv("OAUTH2_TESTPROVIDER_CLIENT_SECRET")
	}

	var err error
	strategy, err = authstrategy.NewOAuth2Strategy(
		testProviderName,
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
		t.Fatalf("Failed to create test OAuth2 strategy: %v", err)
	}

	return strategy, cleanup
}

func TestNewOAuth2Strategy_MissingClientID(t *testing.T) {
	os.Setenv("OAUTH2_TESTPROVIDER_CLIENT_SECRET", "testprovider-client-secret")
	os.Unsetenv("OAUTH2_TESTPROVIDER_CLIENT_ID")
	defer os.Unsetenv("OAUTH2_TESTPROVIDER_CLIENT_SECRET")

	_, err := authstrategy.NewOAuth2Strategy(
		testProviderName,
		oauth2.Endpoint{},
		func(token *oauth2.Token) (*http.Response, error) { return nil, nil },
		[]string{"email"},
	)

	if err == nil {
		t.Error("Expected error when CLIENT_ID is missing, got nil")
	}
}

func TestNewOAuth2Strategy_MissingClientSecret(t *testing.T) {
	os.Setenv("OAUTH2_TESTPROVIDER_CLIENT_ID", "testprovider-client-id")
	os.Unsetenv("OAUTH2_TESTPROVIDER_CLIENT_SECRET")
	defer os.Unsetenv("OAUTH2_TESTPROVIDER_CLIENT_ID")

	_, err := authstrategy.NewOAuth2Strategy(
		testProviderName,
		oauth2.Endpoint{},
		func(token *oauth2.Token) (*http.Response, error) { return nil, nil },
		[]string{"email"},
	)

	if err == nil {
		t.Error("Expected error when CLIENT_SECRET is missing, got nil")
	}
}

func TestNewOAuth2Strategy_Success(t *testing.T) {
	strategy, cleanup := setupTestOAuth2Strategy(t)
	defer cleanup()

	if strategy == nil {
		t.Fatal("Expected non-nil strategy")
	}
}

func TestOAuth2AuthenticationStrategy_GetAuthType(t *testing.T) {
	strategy, cleanup := setupTestOAuth2Strategy(t)
	defer cleanup()

	authType := strategy.GetAuthType()
	expected := "oauth2_testprovider"

	if authType != expected {
		t.Errorf("Expected auth type %q, got %q", expected, authType)
	}
}

func TestOAuth2AuthenticationStrategy_GetRouter(t *testing.T) {
	strategy, cleanup := setupTestOAuth2Strategy(t)
	defer cleanup()

	router := strategy.GetRouter()
	if router == nil {
		t.Error("Expected non-nil router")
	}
}

func TestOAuth2AuthenticationStrategy_SetDatabase(t *testing.T) {
	strategy, cleanup := setupTestOAuth2Strategy(t)
	defer cleanup()

	// Should not panic with nil arguments
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetDatabase panicked: %v", r)
		}
	}()

	strategy.SetDatabase(nil, nil)
}

func TestOAuth2AuthenticationStrategy_SetAuthTokenManager(t *testing.T) {
	strategy, cleanup := setupTestOAuth2Strategy(t)
	defer cleanup()

	// Should not panic with nil argument
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetAuthTokenManager panicked: %v", r)
		}
	}()

	strategy.SetAuthTokenManager(nil)
}
