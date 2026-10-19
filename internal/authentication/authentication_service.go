package authentication

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hmcalister/Cirrus/internal/database"
	"github.com/hmcalister/Cirrus/internal/email/sender"
	"github.com/hmcalister/Cirrus/internal/template"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Token purposes. Each token row records why it was issued so that a
// verification token can never be spent as a session, and so on.
const (
	purposeVerify  = "verify"
	purposeReset   = "reset"
	purposeSession = "session"
)

type AuthenticationConfig struct {
	VerificationExpiryDuration time.Duration
	ResetExpiryDuration        time.Duration
	SessionExpiryDuration      time.Duration
	Links                      TokenLinkBuilder
	Querier                    database.Querier
	EmailSender                sender.Sender
	EmailRenderer              template.EmailRenderer
}

// AuthenticationService manages account registration, verification, login, and
// the lifecycle of an account. All methods are safe for concurrent use.
type AuthenticationService struct {
	AuthenticationConfig

	// Enables testing of token expiry without sleeping.
	now func() time.Time
}

// Returns a service that validates cfg up front.
func NewAuthenticationService(cfg AuthenticationConfig) (*AuthenticationService, error) {
	if cfg.VerificationExpiryDuration <= 0 {
		return nil, errors.New("VerificationExpiryDuration must be positive")
	}
	if cfg.ResetExpiryDuration <= 0 {
		return nil, errors.New("ResetExpiryDuration must be positive")
	}
	if cfg.SessionExpiryDuration <= 0 {
		return nil, errors.New("SessionExpiryDuration must be positive")
	}
	if cfg.Links == nil {
		return nil, errors.New("links must be non-nil")
	}
	if cfg.Querier == nil {
		return nil, errors.New("querier must be non-nil")
	}
	if cfg.EmailSender == nil {
		return nil, errors.New("emailSender must be non-nil")
	}
	if cfg.EmailRenderer == nil {
		return nil, errors.New("emailRenderer must be non-nil")
	}

	return &AuthenticationService{
		AuthenticationConfig: cfg,
		now:                  time.Now,
	}, nil
}

// Creates an active, unverified account and emails a verification link.
//
// The email is normalised before storage.
// If the email is already registered, the returned error wraps ErrEmailTaken.
// A failure to send the verification email is returned to the caller,
// and the account remains in the database.
func (s *AuthenticationService) Register(ctx context.Context, userEmail, password string) error {
	normalized, err := normalizeEmail(userEmail)
	if err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("%w: password must not be empty", ErrInvalidPassword)
	}

	salt, err := generateSalt()
	if err != nil {
		return err
	}
	hash := computeHash([]byte(password), salt)

	user, err := s.Querier.CreateUser(ctx, database.CreateUserParams{
		Email:        normalized,
		PasswordHash: hash,
		Salt:         salt,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s", ErrEmailTaken, normalized)
		}
		return err
	}

	return s.sendVerificationEmail(ctx, user.ID, normalized)
}

// Issues a fresh verification token and re-sends the verification email. Any outstanding verification tokens are invalidated.
//
// If the account does not exist, nil is returned so that callers cannot use this method to discover registered emails.
// If the account is already verified, the returned error wraps ErrAlreadyVerified.
func (s *AuthenticationService) ResendVerification(ctx context.Context, userEmail string) error {
	normalized, err := normalizeEmail(userEmail)
	if err != nil {
		return err
	}

	user, err := s.Querier.GetUserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if user.Verified.Bool {
		return fmt.Errorf("%w: %s", ErrAlreadyVerified, normalized)
	}

	err = s.Querier.DeleteUserTokensByPurpose(ctx, database.DeleteUserTokensByPurposeParams{
		UserID:  user.ID,
		Purpose: purposeVerify,
	})
	if err != nil {
		return err
	}

	return s.sendVerificationEmail(ctx, user.ID, normalized)
}

// Burns a verification token and marks the matching account verified.
//
// Returns true when the token was valid and the account is now verified.
// A token that is unknown, expired, or already spent wraps ErrBadToken.
func (s *AuthenticationService) Verify(ctx context.Context, rawToken string) (bool, error) {
	userID, err := s.Querier.ConsumeUserToken(ctx, database.ConsumeUserTokenParams{
		TokenHash: hashToken(rawToken),
		Purpose:   purposeVerify,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrBadToken
		}
		return false, err
	}

	if err := s.Querier.MarkVerified(ctx, userID); err != nil {
		return false, err
	}

	return true, nil
}

// Login verifies credentials and issues a session token.
//
// The account must be verified. An inactive account may still be logged in.
// A wrong password or unknown email wraps ErrBadLogin. An unverified account wraps ErrUnverified.
func (s *AuthenticationService) Login(ctx context.Context, userEmail, password string) (string, error) {
	normalized, err := normalizeEmail(userEmail)
	if err != nil {
		return "", err
	}

	user, err := s.Querier.GetUserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Prevent timing attacks
			checkPassword([]byte(password), dummySalt, dummyHash)
			return "", ErrBadLogin
		}
		return "", err
	}

	if !checkPassword([]byte(password), user.Salt, user.PasswordHash) {
		return "", ErrBadLogin
	}

	raw, tokenHash := generateToken()
	err = s.Querier.CreateUserToken(ctx, database.CreateUserTokenParams{
		TokenHash: tokenHash,
		UserID:    user.ID,
		Purpose:   purposeSession,
		ExpiresAt: postgresExpiryTime(s.now(), s.SessionExpiryDuration),
	})
	if err != nil {
		return "", err
	}

	return raw, nil
}

// Emails a password reset link if the account exists.
//
// Always returns nil for an unknown email so that callers cannot use it to discover registered accounts.
// Any outstanding reset tokens are invalidated.
func (s *AuthenticationService) RequestPasswordReset(ctx context.Context, userEmail string) error {
	normalized, err := normalizeEmail(userEmail)
	if err != nil {
		return nil
	}

	user, err := s.Querier.GetUserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	err = s.Querier.DeleteUserTokensByPurpose(ctx, database.DeleteUserTokensByPurposeParams{
		UserID:  user.ID,
		Purpose: purposeReset,
	})
	if err != nil {
		return err
	}

	raw, tokenHash := generateToken()
	err = s.Querier.CreateUserToken(ctx, database.CreateUserTokenParams{
		TokenHash: tokenHash,
		UserID:    user.ID,
		Purpose:   purposeReset,
		ExpiresAt: postgresExpiryTime(s.now(), s.ResetExpiryDuration),
	})
	if err != nil {
		return err
	}

	body, err := s.EmailRenderer.RenderEmail(template.TemplatePasswordReset, struct {
		Link string
	}{Link: s.Links.PasswordResetLink(raw)})
	if err != nil {
		return err
	}

	return s.EmailSender.Send(ctx, sender.SendParams{
		Subject:      "Password Reset",
		Body:         body,
		ToRecipients: []string{normalized},
	})
}

// Burns a reset token and sets a new password.
//
// On success every existing session token for the account is invalidated, so
// any other logged-in devices are signed out.
// A token that is unknown, expired, or already spent wraps ErrBadToken.
func (s *AuthenticationService) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("%w: password must not be empty", ErrInvalidPassword)
	}

	userID, err := s.Querier.ConsumeUserToken(ctx, database.ConsumeUserTokenParams{
		TokenHash: hashToken(rawToken),
		Purpose:   purposeReset,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBadToken
		}
		return err
	}

	salt, err := generateSalt()
	if err != nil {
		return err
	}
	hash := computeHash([]byte(newPassword), salt)

	err = s.Querier.UpdatePassword(ctx, database.UpdatePasswordParams{
		ID:           userID,
		PasswordHash: hash,
		Salt:         salt,
	})
	if err != nil {
		return err
	}

	return s.Querier.DeleteUserTokensByPurpose(ctx, database.DeleteUserTokensByPurposeParams{
		UserID:  userID,
		Purpose: purposeSession,
	})
}

// Set the active flag of a user account. In this project, this controls whether the user receives daily emails.
func (s *AuthenticationService) SetAccountActive(ctx context.Context, userEmail string, active bool) error {
	user, err := s.lookupUser(ctx, userEmail)
	if err != nil {
		return err
	}

	err = s.Querier.SetActive(ctx, database.SetActiveParams{
		ID:     user.ID,
		Active: pgtype.Bool{Bool: active, Valid: true},
	})
	if err != nil {
		return err
	}

	return nil
}

// Reports whether the account is opted in to daily emails.
//
// An unknown account returns an error wrapping ErrInvalidEmail.
func (s *AuthenticationService) AccountActive(ctx context.Context, userEmail string) (bool, error) {
	user, err := s.lookupUser(ctx, userEmail)
	if err != nil {
		return false, err
	}

	return user.Active.Bool, nil
}

// Reports whether the account is verified.
//
// An unknown account returns an error wrapping ErrInvalidEmail.
func (s *AuthenticationService) AccountVerified(ctx context.Context, userEmail string) (bool, error) {
	user, err := s.lookupUser(ctx, userEmail)
	if err != nil {
		return false, err
	}

	return user.Verified.Bool, nil
}

// DeleteAccount permanently removes the account and all of its tokens. GDPR compliant.
func (s *AuthenticationService) DeleteAccount(ctx context.Context, userEmail string) error {
	user, err := s.lookupUser(ctx, userEmail)
	if err != nil {
		return err
	}

	return s.Querier.DeleteUser(ctx, user.ID)
}

// Resolves a raw session token to the email of the logged-in user.
//
// A token that is empty, unknown, or expired reports false.
func (s *AuthenticationService) SessionEmail(ctx context.Context, rawToken string) (string, bool) {
	if rawToken == "" {
		return "", false
	}

	userID, err := s.Querier.GetUserToken(ctx, database.GetUserTokenParams{
		TokenHash: hashToken(rawToken),
		Purpose:   purposeSession,
	})
	if err != nil {
		return "", false
	}

	user, err := s.Querier.GetUserByID(ctx, userID)
	if err != nil {
		return "", false
	}

	return user.Email, true
}

func (s *AuthenticationService) sendVerificationEmail(ctx context.Context, userID pgtype.UUID, userEmail string) error {
	raw, tokenHash := generateToken()
	err := s.Querier.CreateUserToken(ctx, database.CreateUserTokenParams{
		TokenHash: tokenHash,
		UserID:    userID,
		Purpose:   purposeVerify,
		ExpiresAt: postgresExpiryTime(s.now(), s.VerificationExpiryDuration),
	})
	if err != nil {
		return err
	}

	body, err := s.EmailRenderer.RenderEmail(template.TemplateVerification, struct {
		Link string
	}{Link: s.Links.VerificationLink(raw)})
	if err != nil {
		return err
	}

	return s.EmailSender.Send(ctx, sender.SendParams{
		Subject:      "Account Verification",
		Body:         body,
		ToRecipients: []string{userEmail},
	})
}

// Fetch an account by normalised email.
func (s *AuthenticationService) lookupUser(ctx context.Context, userEmail string) (database.UserAccount, error) {
	normalized, err := normalizeEmail(userEmail)
	if err != nil {
		return database.UserAccount{}, err
	}

	user, err := s.Querier.GetUserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.UserAccount{}, fmt.Errorf("%w: %s", ErrInvalidEmail, normalized)
		}
		return database.UserAccount{}, err
	}

	return user, nil
}
