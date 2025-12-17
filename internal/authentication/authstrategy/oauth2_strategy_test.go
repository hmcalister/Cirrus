package authstrategy_test

import "github.com/hmcalister/LiteralCloudService/internal/authentication/authstrategy"

var (
	_ authstrategy.AuthenticationStrategy = authstrategy.OAuth2AuthenticationStrategy{}
)
