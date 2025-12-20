package oauth2utils

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

var OAUTH2_ENDPOINT_MAP = map[string]oauth2.Endpoint{
	"google": google.Endpoint,
	"github": github.Endpoint,
}
