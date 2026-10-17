package authentication

// Converts a raw token into the URL a user should visit.
//
// This interface keeps HTTP logic out of the authentication service.
//
// Implementations must be safe for concurrent use.
type TokenLinkBuilder interface {
	VerificationLink(rawToken string) string
	PasswordResetLink(rawToken string) string
}
