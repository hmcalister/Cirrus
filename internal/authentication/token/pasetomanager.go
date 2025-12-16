package token

import (
	"errors"
	"os"
	"time"

	"aidanwoods.dev/go-paseto"
)

const (
	PASETO_AUDIENCE            = "CloudApp"
	PASETO_ISSUED_BY           = "CloudAppPasetoMediator"
	PASETO_SUBJECT             = "CloudAppAuth"
	PASETO_EXPIRATION_DURATION = 1 * time.Hour
)

var (
	ErrInvalidSigningSecret = errors.New("invalid signing secret found")
)

type PasetoManager struct {
	secretKey paseto.V4AsymmetricSecretKey
	publicKey paseto.V4AsymmetricPublicKey
	parser    paseto.Parser
}

func NewPasetoMediator() (p *PasetoManager, err error) {
	signingSecret := os.Getenv("PASETO_SIGNING_SECRET")
	if signingSecret == "" {
		return nil, ErrInvalidSigningSecret
	}

	secretKey, err := paseto.NewV4AsymmetricSecretKeyFromHex(signingSecret)
	if err != nil {
		return nil, ErrInvalidSigningSecret
	}
	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromHex(signingSecret)
	if err != nil {
		return nil, ErrInvalidSigningSecret
	}

	// TODO: Validate for paseto.ValidAt (does time.Now() snapshot the time to parser creation?)
	parser := paseto.NewParser()
	parser.AddRule(paseto.NotExpired())
	parser.AddRule(paseto.ForAudience(PASETO_AUDIENCE))
	parser.AddRule(paseto.IssuedBy(PASETO_ISSUED_BY))
	parser.AddRule(paseto.Subject(PASETO_SUBJECT))

	p = &PasetoManager{
		secretKey: secretKey,
		publicKey: publicKey,
		parser:    parser,
	}

	return p, nil
}

// Creates and returns an encrypted paseto v4 token with the given claims
func (p PasetoManager) CreateToken(claims map[string]interface{}) (token string) {
	now := time.Now()
	rawToken := paseto.NewToken()

	rawToken.SetAudience(PASETO_AUDIENCE)
	rawToken.SetIssuer(PASETO_ISSUED_BY)
	rawToken.SetSubject(PASETO_SUBJECT)
	rawToken.SetIssuedAt(now)
	rawToken.SetNotBefore(now)
	rawToken.SetExpiration(now.Add(PASETO_EXPIRATION_DURATION))
	for k, v := range claims {
		rawToken.Set(k, v)
	}
	token = rawToken.V4Sign(p.secretKey, nil)

	return token
}

// Verifies the given token and returns the encrypted claims and true if ok, or
// an empty map and false if the token is invalid.
func (p PasetoManager) VerifyToken(token string) (claims map[string]interface{}, ok bool) {
	rawToken, err := p.parser.ParseV4Public(p.publicKey, token, nil)
	if err != nil {
		return claims, false
	}

	return rawToken.Claims(), true
}
