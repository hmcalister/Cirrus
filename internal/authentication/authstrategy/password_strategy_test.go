package authstrategy_test

import (
	"testing"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/authstrategy"
)

// Compile-time check that PasswordAuthenticationStrategy implements AuthenticationStrategy
var _ authstrategy.AuthenticationStrategy = authstrategy.PasswordAuthenticationStrategy{}

func TestPasswordAuthenticationStrategy_GetAuthType(t *testing.T) {
	strategy := authstrategy.NewPasswordStrategy()

	authType := strategy.GetAuthType()
	expected := "password"

	if authType != expected {
		t.Errorf("Expected auth type %q, got %q", expected, authType)
	}
}

func TestPasswordAuthenticationStrategy_GetRouter(t *testing.T) {
	strategy := authstrategy.NewPasswordStrategy()

	router := strategy.GetRouter()
	if router == nil {
		t.Error("Expected non-nil router")
	}
}

func TestPasswordAuthenticationStrategy_SetDatabase(t *testing.T) {
	strategy := authstrategy.NewPasswordStrategy()

	// Should not panic with nil arguments (will panic when used, but not on set)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetDatabase panicked: %v", r)
		}
	}()

	strategy.SetDatabase(nil, nil)
}

func TestPasswordAuthenticationStrategy_SetAuthTokenManager(t *testing.T) {
	strategy := authstrategy.NewPasswordStrategy()

	// Should not panic with nil argument (will panic when used, but not on set)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetAuthTokenManager panicked: %v", r)
		}
	}()

	strategy.SetAuthTokenManager(nil)
}
