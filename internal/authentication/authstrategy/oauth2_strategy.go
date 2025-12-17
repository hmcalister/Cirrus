package authstrategy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"
	"github.com/hmcalister/LiteralCloudService/internal/database"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// Handler for OAuth2 access tokens.
// Manages the provider-specific routes for OAuth2 flows.
// Use an existing function from the exposed OAUTH2_ACCESS_TOKEN_HANDLER_MAP or provide your own.
type AccessTokenHandler = func(token *oauth2.Token) (r *http.Response, err error)

var (
	OAUTH2_ENDPOINT_MAP = map[string]oauth2.Endpoint{
		"google": google.Endpoint,
		"github": github.Endpoint,
	}

	OAUTH2_ACCESS_TOKEN_HANDLER_MAP = map[string]AccessTokenHandler{
		"google": func(token *oauth2.Token) (r *http.Response, err error) {
			return http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
		},
		"github": func(token *oauth2.Token) (r *http.Response, err error) {
			client := &http.Client{}
			req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
			req.Header.Add("Authorization", "Bearer "+token.AccessToken)
			return client.Do(req)
		},
	}
)

// Define an OAuth2AuthenticationStrategy that authenticates a user using OAuth2.
// The given OAuth2 provider must supply an email.
type OAuth2AuthenticationStrategy struct {
	*baseAuthenticationStrategy

	providerName string

	// Config for the OAuth2 flow.
	// This struct also defines the provider name (e.g. google, github)
	config oauth2.Config

	// Handler for OAuth2 access tokens.
	// Manages the provider-specific routes for OAuth2 flows.
	accessTokenHandler AccessTokenHandler
}

// Creates a new OAuth2 strategy for a specific provider (e.g., "google", "github")
// providerName is used to construct the auth_type as "oauth2_{providerName}".
//
// Calling this function assumes the existence (and reasonable values of) the following environment variables:
// - OAUTH2_{PROVIDERNAME}_CLIENT_ID
// - OAUTH2_{PROVIDERNAME}_CLIENT_SECRET
// Where {PROVIDERNAME} is the uppercase (case sensitive) value of providerName.
func NewOAuth2Strategy(
	providerName string,
	providerEndpoint oauth2.Endpoint,
	providerAccessTokenHandler AccessTokenHandler,
	scopes []string,
) (*OAuth2AuthenticationStrategy, error) {
	oauth2Strategy := &OAuth2AuthenticationStrategy{
		baseAuthenticationStrategy: newBaseAuthenticationStrategy(),
		providerName:               providerName,
		accessTokenHandler:         providerAccessTokenHandler,
	}

	providerNameUpper := strings.ToUpper(providerName)
	clientID := os.Getenv(fmt.Sprintf("OAUTH2_%s_CLIENT_ID", providerNameUpper))
	if clientID == "" {
		return nil, fmt.Errorf("missing OAUTH2_%s_CLIENT_ID environment variable", providerNameUpper)
	}
	clientSecret := os.Getenv(fmt.Sprintf("OAUTH2_%s_CLIENT_SECRET", providerNameUpper))
	if clientSecret == "" {
		return nil, fmt.Errorf("missing OAUTH2_%s_CLIENT_SECRET environment variable", providerNameUpper)
	}

	oauth2Strategy.config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     providerEndpoint,
		RedirectURL:  fmt.Sprintf("/auth/%s/callback", oauth2Strategy.GetAuthType()),
		Scopes:       scopes,
	}

	oauth2Strategy.router.HandleFunc("GET /", oauth2Strategy.initiateAuth)
	oauth2Strategy.router.HandleFunc("GET /callback", oauth2Strategy.handleCallback)

	return oauth2Strategy, nil
}

func (oauth2Strategy OAuth2AuthenticationStrategy) GetAuthType() string {
	return fmt.Sprintf("oauth2_%s", oauth2Strategy.providerName)
}

// Initiates the OAuth2 authentication flow by redirecting to the provider
// Route: GET /auth/oauth2_{provider}/
func (oauth2Strategy *OAuth2AuthenticationStrategy) initiateAuth(w http.ResponseWriter, r *http.Request) {
	// Prevent CSRF attacks
	stateToken := oauth2Strategy.generateStateToken()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth2_state_token",
		Value:    stateToken,
		Path:     "/auth",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   os.Getenv("ENVIRONMENT") == "PRODUCTION",
		SameSite: http.SameSiteLaxMode,
	})

	url := oauth2Strategy.config.AuthCodeURL(stateToken)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Handles the OAuth2 callback from the provider
// Route: GET /auth/oauth2_{provider}/callback
func (oauth2Strategy *OAuth2AuthenticationStrategy) handleCallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("oauth2_state_token")
	if err != nil {
		http.Error(w, "No state cookie found", http.StatusBadRequest)
		return
	}
	cookieState := cookie.Value

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth2_state_token",
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1, // Delete cookie
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Verify state parameter to prevent CSRF
	if r.FormValue("state") != cookieState {
		http.Error(w, "invalid state parameter", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	authToken, err := oauth2Strategy.config.Exchange(ctx, r.FormValue("code"))
	if err != nil {
		slog.Error("oauth2 code exchange failed", "provider", oauth2Strategy.providerName, "error", err)
		http.Error(w, "oauth2 code exchange failed", http.StatusInternalServerError)
		return
	}

	response, err := oauth2Strategy.accessTokenHandler(authToken)
	if err != nil {
		slog.Error("failed getting user info from oauth2 provider", "provider", oauth2Strategy.providerName, "error", err)
		http.Error(w, "failed getting user info", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	var oauthUserInfo struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(response.Body).Decode(&oauthUserInfo); err != nil {
		slog.Error("could not decode oauth2 user info", "provider", oauth2Strategy.providerName, "error", err)
		http.Error(w, "could not decode user info", http.StatusInternalServerError)
		return
	}

	// --------------------------------------------------------------------------------
	// Transaction Start
	tx, err := oauth2Strategy.connPool.Begin(ctx)
	if err != nil {
		slog.Error("error starting transaction during oauth2 callback", "error", err)
		http.Error(w, "error occurred during oauth2 authentication", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)
	txQueries := oauth2Strategy.db.WithTx(tx)

	user, err := txQueries.UpsertUser(ctx, oauthUserInfo.Email)
	if err != nil {
		slog.Error("error upserting user during oauth2 callback", "error", err)
		http.Error(w, "error occurred during oauth2 authentication", http.StatusInternalServerError)
		return
	}

	// Create or verify OAuth2 authentication link
	// Try to get existing OAuth2 auth
	_, err = txQueries.GetOAuth2Authentication(ctx, database.GetOAuth2AuthenticationParams{
		UserID:         user.UserID,
		Oauth2Provider: oauth2Strategy.providerName,
	})

	if err != nil {
		// OAuth2 link doesn't exist, create it
		_, err = txQueries.CreateOAuth2Authentication(ctx, database.CreateOAuth2AuthenticationParams{
			UserID:         user.UserID,
			Oauth2Provider: oauth2Strategy.providerName,
			ProviderUserID: oauthUserInfo.ID,
		})
		if err != nil {
			slog.Error("error creating oauth2 authentication during callback", "error", err)
			http.Error(w, "error occurred during oauth2 authentication", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("error with transaction commit during oauth2 callback", "error", err)
		http.Error(w, "error occurred during oauth2 authentication", http.StatusInternalServerError)
		return
	}
	// Transaction End
	// --------------------------------------------------------------------------------

	claims := token.UserToTokenClaims(user)
	userAuthToken := oauth2Strategy.authTokenManager.CreateToken(claims)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"authToken": "%s"}`, userAuthToken)))
}

func (oauth2Strategy OAuth2AuthenticationStrategy) generateStateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
