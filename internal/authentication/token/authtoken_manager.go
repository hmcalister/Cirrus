package token

type AuthTokenManager interface {
	// Create a token with the given claims and return it.
	// The returned token should be ready for transmission across the public internet,
	// i.e. it should be signed and/or encrypted.
	CreateToken(claims map[string]interface{}) (token string)

	// Verify the given token (which can be assumed to be from the CreateToken method)
	// Returns encrypted claims and true if token is valid,
	// or an empty map and false if the token is invalid.
	VerifyToken(token string) (claims map[string]interface{}, ok bool)
}
