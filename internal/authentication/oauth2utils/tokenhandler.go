package oauth2utils

import (
	"net/http"

	"golang.org/x/oauth2"
)

// Handler for OAuth2 access tokens.
// Manages the provider-specific routes for OAuth2 flows.
// Use an existing function from the exposed OAUTH2_ACCESS_TOKEN_HANDLER_MAP or provide your own.
type AccessTokenHandler = func(token *oauth2.Token) (r *http.Response, err error)

var (
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
