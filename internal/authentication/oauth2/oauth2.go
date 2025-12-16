package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

var (
	ErrOAuthBadProviderID     = errors.New("oauth providerID does not exist")
	ErrOAuthProviderBadConfig = errors.New("oauth providerID exists but secret of callback URL does not")
)

type userInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type OAuth2Provider struct {
	oauth2.Config
	AccessTokenFunction func(token *oauth2.Token) (r *http.Response, err error)
}

// Load a specific OAuth provider from environment variables and return the goth.Provider.
// providerConstructor should be the `new` function of the associated goth Provider, e.g. `google.New`.
// Order of parameters
//
// Returns an error if the providerID environment variable ( $(providerName)_PROVIDER_ID ) does not exist
// or if the provider callback or callback URL do not exist.
func initializeOAuth2ConfigFromEnvironmentVariables(providerName string, config *OAuth2Provider) error {
	providerID := os.Getenv(fmt.Sprintf("%s_PROVIDER_ID", providerName))
	if providerID == "" {
		return ErrOAuthBadProviderID
	}

	providerSecret := os.Getenv(fmt.Sprintf("%s_PROVIDER_SECRET", providerName))
	if providerSecret == "" {
		return ErrOAuthProviderBadConfig
	}

	callbackURL := os.Getenv(fmt.Sprintf("%s_CALLBACK_URL", providerName))
	if callbackURL == "" {
		return ErrOAuthProviderBadConfig
	}

	config.ClientID = providerID
	config.ClientSecret = providerSecret
	config.RedirectURL = callbackURL

	return nil
}

// Returns a new subrouter that handles OAuth2 workflows based on the given providers.
// Provides routes `/{providerName}` and `/{providerName}/callback` for each provider given.
//
// This subrouter should be mounted to `/auth`
func createOAuth2Subrouter(providerConfigMap map[string]*OAuth2Provider, authTokenManager token.AuthTokenManager) *http.ServeMux {
	mux := http.NewServeMux()

	stateString := os.Getenv("OAUTH2_STATE_STRING")

	mux.HandleFunc("GET /{providerName}", func(w http.ResponseWriter, r *http.Request) {
		providerName := r.PathValue("providerName")
		if providerName == "" {
			http.Error(w, "No OAuth2 provider given", http.StatusBadRequest)
			return
		}

		providerConfig, ok := providerConfigMap[providerName]
		if !ok {
			slog.Warn("received bad OAuth2 request", "requested providerName", providerName)
			http.Error(w, "OAuth2 provider not recognized", http.StatusBadRequest)
			return
		}

		url := providerConfig.AuthCodeURL(stateString)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("GET /{providerName}/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("state") != stateString {
			http.Error(w, "Invalid state parameter", http.StatusUnauthorized)
			return
		}

		providerName := r.PathValue("providerName")
		if providerName == "" {
			http.Error(w, "No OAuth2 provider given", http.StatusBadRequest)
			return
		}

		providerConfig, ok := providerConfigMap[providerName]
		if !ok {
			slog.Warn("received bad OAuth2 request", "requested providerName", providerName)
			http.Error(w, "OAuth2 provider not recognized", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		token, err := providerConfig.Exchange(ctx, r.FormValue("code"))
		if err != nil {
			http.Error(w, "OAuth2 code exchange failed", http.StatusInternalServerError)
			return
		}

		response, err := providerConfig.AccessTokenFunction(token)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed getting user info: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		defer response.Body.Close()

		var oauthUserInfo userInfo
		if err := json.NewDecoder(response.Body).Decode(&oauthUserInfo); err != nil {
			http.Error(w, "Could not decode user info", http.StatusInternalServerError)
			return
		}

		authToken := authTokenManager.CreateToken(map[string]interface{}{
			"id":    oauthUserInfo.ID,
			"email": oauthUserInfo.Email,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"authToken": "%s"}`, authToken)))
	})

	return mux
}

// Loads all available OAuth2 provider configurations from environment variables and
// prepares a subrouter to be mounted on the `/auth` route.
//
// The passed tokenManager is used to create authentication tokens in the subrouter methods.
//
// Returns Map from the (case insensitive) string of the provider name to the oauth2 config.
// Note the environment variables for the providers must exist under the uppercase (case sensitive) names.
// For example, the provider "google" must have environment variables:
// `GOOGLE_PROVIDER_ID`, `GOOGLE_PROVIDER_SECRET`.
//
// All OAuth2 routes are available publicly on `/auth/{providerName}` and `/auth/{providerName}/callback`
// once the returned subrouter is mounted appropriately. It is important that the providerName is consistent.
// NOTE WELL: This means your OAuth2 setup should have the callback URL set correctly to `https://.../auth/{providerName}/callback`
func InitializeOAuth2(authTokenManager token.AuthTokenManager) (providerConfigMap map[string]*OAuth2Provider, authSubrouter *http.ServeMux, err error) {
	providerConfigMap = map[string]*OAuth2Provider{
		"google": {
			Config: oauth2.Config{
				Endpoint: google.Endpoint,
				Scopes:   []string{"email"},
			},
			AccessTokenFunction: func(token *oauth2.Token) (r *http.Response, err error) {
				return http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
			},
		},
		"github": {
			Config: oauth2.Config{
				Endpoint: github.Endpoint,
				Scopes:   []string{"email"},
			},
			AccessTokenFunction: func(token *oauth2.Token) (r *http.Response, err error) {
				client := &http.Client{}
				req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
				req.Header.Add("Authorization", "Bearer "+token.AccessToken)
				return client.Do(req)
			},
		},
	}

	for providerName, config := range providerConfigMap {
		err := initializeOAuth2ConfigFromEnvironmentVariables(providerName, config)
		if err != nil {
			slog.Error(
				"oAuth2 config failed to load",
				"providerName", providerName,
				"error", err,
			)
			delete(providerConfigMap, providerName)
			continue
		}
	}

	if len(providerConfigMap) == 0 {
		slog.Warn("no OAuth2 providers configured")
	}

	authSubrouter = createOAuth2Subrouter(providerConfigMap, authTokenManager)

	return providerConfigMap, authSubrouter, nil
}
