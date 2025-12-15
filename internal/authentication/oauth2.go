package authentication

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
)

var (
	ErrOAuthBadProviderID     = errors.New("oauth providerID does not exist")
	ErrOAuthProviderBadConfig = errors.New("oauth providerID exists but secret of callback URL does not")
)

type gothProviderConstructor func(providerKey, secret, callbackURL string) goth.Provider

// Load a specific OAuth provider from environment variables and return the goth.Provider.
// providerConstructor should be the `new` function of the associated goth Provider, e.g. `google.New`.
// Order of parameters
//
// Returns an error if the providerID environment variable ( $(providerName)_PROVIDER_ID ) does not exist
// or if the provider callback or callback URL do not exist.
func loadOAuth2ConfigToProvider(providerName string, providerConstructor gothProviderConstructor) (goth.Provider, error) {
	providerID := os.Getenv(fmt.Sprintf("%s_PROVIDER_ID", providerName))
	if providerID == "" {
		return nil, ErrOAuthBadProviderID
	}

	providerSecret := os.Getenv(fmt.Sprintf("%s_PROVIDER_SECRET", providerName))
	if providerSecret == "" {
		return nil, ErrOAuthProviderBadConfig
	}

	callbackURL := os.Getenv(fmt.Sprintf("%s_CALLBACK_URL", providerName))
	if callbackURL == "" {
		return nil, ErrOAuthProviderBadConfig
	}

	return providerConstructor(providerID, providerSecret, callbackURL), nil
}

// Loads all available OAuth2 provider configurations from environment variables
func initializeGothWithAllProviders() error {
	var providers []goth.Provider

	// TODO: Fix ugly function inlines
	// TODO: Add scopes to providers
	for providerName, constructor := range map[string]gothProviderConstructor{
		"google": func(providerKey, secret, callbackURL string) goth.Provider {
			return google.New(providerKey, secret, callbackURL)
		},
		"github": func(providerKey, secret, callbackURL string) goth.Provider {
			return github.New(providerKey, secret, callbackURL)
		},
	} {
		gothProvider, err := loadOAuth2ConfigToProvider(providerName, constructor)
		if err != nil {
			slog.Error(
				"oAuth2 config failed to load",
				"providerName", providerName,
				"error", err,
			)
			continue
		}
		providers = append(providers, gothProvider)
	}

	if len(providers) == 0 {
		slog.Warn("no OAuth2 providers configured")
	}

	goth.UseProviders(providers...)

	return nil
}
