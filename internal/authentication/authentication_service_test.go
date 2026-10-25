package authentication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockEmailRenderer struct {
	err error
}

func (m mockEmailRenderer) RenderEmail(_ string, data any) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return fmt.Sprintf("rendered: %+v", data), nil
}

type mockTokenLinkBuilder struct{}

func (mockTokenLinkBuilder) VerificationLink(rawToken string) string {
	return "https://cloud.example.com/verify?token=" + rawToken
}

func (mockTokenLinkBuilder) PasswordResetLink(rawToken string) string {
	return "https://cloud.example.com/reset?token=" + rawToken
}

type mockQuerier struct {
	now time.Time

	users     map[string]*database.UserAccount
	nextID    uint64
	tokens    map[string]mockToken
	createErr error
}

type mockToken struct {
	userID    pgtype.UUID
	purpose   string
	expiresAt time.Time
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{
		now:    time.Now(),
		users:  make(map[string]*database.UserAccount),
		tokens: make(map[string]mockToken),
	}
}

func (q *mockQuerier) nextUUID() pgtype.UUID {
	q.nextID++
	var id pgtype.UUID
	id.Valid = true
	binary := [16]byte{}
	binary[15] = byte(q.nextID)
	binary[0] = 0x01
	id.Bytes = binary
	return id
}

func (q *mockQuerier) CreateUser(_ context.Context, arg database.CreateUserParams) (database.UserAccount, error) {
	if q.createErr != nil {
		return database.UserAccount{}, q.createErr
	}
	if _, ok := q.users[arg.Email]; ok {
		return database.UserAccount{}, &pgconn.PgError{Code: "23505"}
	}
	user := &database.UserAccount{
		ID:           q.nextUUID(),
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		Salt:         arg.Salt,
		Verified:     pgtype.Bool{Bool: false, Valid: true},
		Active:       pgtype.Bool{Bool: true, Valid: true},
		CreatedAt:    pgtype.Timestamptz{Time: q.now, Valid: true},
	}
	q.users[arg.Email] = user
	return *user, nil
}

func (q *mockQuerier) GetUserByEmail(_ context.Context, userEmail string) (database.UserAccount, error) {
	user, ok := q.users[userEmail]
	if !ok {
		return database.UserAccount{}, pgx.ErrNoRows
	}
	return *user, nil
}

func (q *mockQuerier) GetUserByID(_ context.Context, id pgtype.UUID) (database.UserAccount, error) {
	for _, user := range q.users {
		if user.ID == id {
			return *user, nil
		}
	}
	return database.UserAccount{}, pgx.ErrNoRows
}

func (q *mockQuerier) MarkVerified(_ context.Context, id pgtype.UUID) error {
	for _, user := range q.users {
		if user.ID == id {
			user.Verified = pgtype.Bool{Bool: true, Valid: true}
			return nil
		}
	}
	return nil
}

func (q *mockQuerier) SetActive(_ context.Context, arg database.SetActiveParams) error {
	for _, user := range q.users {
		if user.ID == arg.ID {
			user.Active = arg.Active
			return nil
		}
	}
	return nil
}

func (q *mockQuerier) UpdatePassword(_ context.Context, arg database.UpdatePasswordParams) error {
	for _, user := range q.users {
		if user.ID == arg.ID {
			user.PasswordHash = arg.PasswordHash
			user.Salt = arg.Salt
			return nil
		}
	}
	return nil
}

func (q *mockQuerier) DeleteUser(_ context.Context, id pgtype.UUID) error {
	for addr, user := range q.users {
		if user.ID == id {
			delete(q.users, addr)
			return nil
		}
	}
	return nil
}

func (q *mockQuerier) CreateUserToken(_ context.Context, arg database.CreateUserTokenParams) error {
	q.tokens[string(arg.TokenHash)] = mockToken{
		userID:    arg.UserID,
		purpose:   arg.Purpose,
		expiresAt: arg.ExpiresAt.Time,
	}
	return nil
}

func (q *mockQuerier) GetUserToken(_ context.Context, arg database.GetUserTokenParams) (pgtype.UUID, error) {
	token, ok := q.tokens[string(arg.TokenHash)]
	if !ok || token.purpose != arg.Purpose || !token.expiresAt.After(q.now) {
		return pgtype.UUID{}, pgx.ErrNoRows
	}
	return token.userID, nil
}

func (q *mockQuerier) ConsumeUserToken(_ context.Context, arg database.ConsumeUserTokenParams) (pgtype.UUID, error) {
	token, ok := q.tokens[string(arg.TokenHash)]
	if !ok || token.purpose != arg.Purpose || !token.expiresAt.After(q.now) {
		return pgtype.UUID{}, pgx.ErrNoRows
	}
	delete(q.tokens, string(arg.TokenHash))
	return token.userID, nil
}

func (q *mockQuerier) DeleteUserTokensByPurpose(_ context.Context, arg database.DeleteUserTokensByPurposeParams) error {
	for hash, token := range q.tokens {
		if token.userID == arg.UserID && token.purpose == arg.Purpose {
			delete(q.tokens, hash)
		}
	}
	return nil
}

func (q *mockQuerier) DeleteUserTokens(_ context.Context, userID pgtype.UUID) error {
	for hash, token := range q.tokens {
		if token.userID == userID {
			delete(q.tokens, hash)
		}
	}
	return nil
}

type mockSender struct {
	sent []skysender.SendParams
	err  error
}

func (f *mockSender) Send(_ context.Context, params skysender.SendParams) error {
	f.sent = append(f.sent, params)
	return f.err
}

func newTestService(t *testing.T, q database.Querier, mail skysender.Sender) *AuthenticationService {
	t.Helper()
	service, err := NewAuthenticationService(AuthenticationConfig{
		VerificationExpiryDuration: time.Hour,
		ResetExpiryDuration:        time.Hour,
		SessionExpiryDuration:      24 * time.Hour,
		Links:                      mockTokenLinkBuilder{},
		Querier:                    q,
		EmailSender:                mail,
		EmailRenderer:              mockEmailRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fq, ok := q.(*mockQuerier); ok {
		service.now = func() time.Time { return fq.now }
	}
	return service
}

func rawTokenFromLink(t *testing.T, body string) string {
	t.Helper()
	token_prefix := "token="
	i := strings.Index(body, token_prefix)
	if i < 0 {
		t.Fatalf("body %q has no token", body)
	}
	raw := body[i+len(token_prefix):]
	// The rendered body wraps the link in other text.
	// The token is the first URL-encoded run of characters after token prefix.
	if end := strings.IndexAny(raw, " \n\r\t}\")"); end >= 0 {
		raw = raw[:end]
	}
	return raw
}

func TestNewAuthenticationServiceValidation(t *testing.T) {
	valid := AuthenticationConfig{
		VerificationExpiryDuration: time.Hour,
		ResetExpiryDuration:        time.Hour,
		SessionExpiryDuration:      time.Hour,
		Links:                      mockTokenLinkBuilder{},
		Querier:                    newMockQuerier(),
		EmailSender:                &mockSender{},
		EmailRenderer:              mockEmailRenderer{},
	}
	tests := []struct {
		name   string
		mutate func(*AuthenticationConfig)
	}{
		{"zero verification expiry", func(c *AuthenticationConfig) { c.VerificationExpiryDuration = 0 }},
		{"zero reset expiry", func(c *AuthenticationConfig) { c.ResetExpiryDuration = 0 }},
		{"zero session expiry", func(c *AuthenticationConfig) { c.SessionExpiryDuration = 0 }},
		{"nil links", func(c *AuthenticationConfig) { c.Links = nil }},
		{"nil querier", func(c *AuthenticationConfig) { c.Querier = nil }},
		{"nil sender", func(c *AuthenticationConfig) { c.EmailSender = nil }},
		{"nil renderer", func(c *AuthenticationConfig) { c.EmailRenderer = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if _, err := NewAuthenticationService(cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestRegister(t *testing.T) {
	q := newMockQuerier()
	mail := &mockSender{}
	service := newTestService(t, q, mail)

	if err := service.Register(context.Background(), "User@Example.com", "hunter2"); err != nil {
		t.Fatal(err)
	}

	// Email is normalised and the account starts unverified and active.
	user, err := q.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.Verified.Bool {
		t.Error("new account should be unverified")
	}
	if !user.Active.Bool {
		t.Error("new account should be active")
	}
	if !checkPassword([]byte("hunter2"), user.Salt, user.PasswordHash) {
		t.Error("stored hash does not match the password")
	}

	// A verification email with a token link was sent.
	if len(mail.sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(mail.sent))
	}
	link := mail.sent[0].Body
	raw := rawTokenFromLink(t, link)
	if _, ok := q.tokens[string(hashToken(raw))]; !ok {
		t.Error("verification token was not stored")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	q := newMockQuerier()
	service := newTestService(t, q, &mockSender{})

	if err := service.Register(context.Background(), "user@example.com", "hunter2"); err != nil {
		t.Fatal(err)
	}
	err := service.Register(context.Background(), "user@example.com", "hunter2")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRegisterInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		want     error
	}{
		{"empty email", "", "hunter2", ErrInvalidEmail},
		{"malformed email", "not-an-email", "hunter2", ErrInvalidEmail},
		{"empty password", "user@example.com", "", ErrInvalidPassword},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService(t, newMockQuerier(), &mockSender{})
			if err := service.Register(context.Background(), tc.email, tc.password); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestVerify(t *testing.T) {
	bg := context.Background()

	t.Run("valid token marks verified", func(t *testing.T) {
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		if err := service.Register(bg, "user@example.com", "hunter2"); err != nil {
			t.Fatal(err)
		}
		raw := rawTokenFromLink(t, mail.sent[0].Body)

		ok, err := service.Verify(bg, raw)
		if err != nil || !ok {
			t.Fatalf("Verify = %v, %v; want true, nil", ok, err)
		}
		user, _ := q.GetUserByEmail(bg, "user@example.com")
		if !user.Verified.Bool {
			t.Error("account should be verified")
		}
	})

	t.Run("token is burned", func(t *testing.T) {
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		_ = service.Register(bg, "user@example.com", "hunter2")
		raw := rawTokenFromLink(t, mail.sent[0].Body)

		_, _ = service.Verify(bg, raw)
		if _, err := service.Verify(bg, raw); !errors.Is(err, ErrBadToken) {
			t.Errorf("second Verify err = %v, want ErrBadToken", err)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		service := newTestService(t, newMockQuerier(), &mockSender{})
		if _, err := service.Verify(bg, "nonsense"); !errors.Is(err, ErrBadToken) {
			t.Errorf("err = %v, want ErrBadToken", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		_ = service.Register(bg, "user@example.com", "hunter2")
		raw := rawTokenFromLink(t, mail.sent[0].Body)

		// Advance the fake clock past the verification expiry.
		q.now = q.now.Add(2 * time.Hour)
		if _, err := service.Verify(bg, raw); !errors.Is(err, ErrBadToken) {
			t.Errorf("err = %v, want ErrBadToken", err)
		}
	})
}

func TestResendVerification(t *testing.T) {
	bg := context.Background()
	q := newMockQuerier()
	mail := &mockSender{}
	service := newTestService(t, q, mail)
	_ = service.Register(bg, "user@example.com", "hunter2")
	firstRaw := rawTokenFromLink(t, mail.sent[0].Body)

	if err := service.ResendVerification(bg, "user@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 2 {
		t.Fatalf("sent %d emails, want 2", len(mail.sent))
	}
	// The old token is invalidated, the new one works.
	if _, err := service.Verify(bg, firstRaw); !errors.Is(err, ErrBadToken) {
		t.Errorf("old token err = %v, want ErrBadToken", err)
	}
	secondRaw := rawTokenFromLink(t, mail.sent[1].Body)
	if ok, err := service.Verify(bg, secondRaw); err != nil || !ok {
		t.Errorf("new token Verify = %v, %v; want true, nil", ok, err)
	}
}

func TestResendVerificationAlreadyVerified(t *testing.T) {
	bg := context.Background()
	q := newMockQuerier()
	mail := &mockSender{}
	service := newTestService(t, q, mail)
	_ = service.Register(bg, "user@example.com", "hunter2")
	raw := rawTokenFromLink(t, mail.sent[0].Body)
	_, _ = service.Verify(bg, raw)

	if err := service.ResendVerification(bg, "user@example.com"); !errors.Is(err, ErrAlreadyVerified) {
		t.Errorf("err = %v, want ErrAlreadyVerified", err)
	}
}

func TestResendVerificationUnknownEmail(t *testing.T) {
	service := newTestService(t, newMockQuerier(), &mockSender{})
	if err := service.ResendVerification(context.Background(), "nobody@example.com"); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestSignIn(t *testing.T) {
	bg := context.Background()
	newVerifiedService := func(t *testing.T) (*AuthenticationService, *mockQuerier, *mockSender) {
		t.Helper()
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		if err := service.Register(bg, "user@example.com", "hunter2"); err != nil {
			t.Fatal(err)
		}
		verifyRaw := rawTokenFromLink(t, mail.sent[0].Body)
		if _, err := service.Verify(bg, verifyRaw); err != nil {
			t.Fatal(err)
		}
		return service, q, mail
	}

	t.Run("success issues session", func(t *testing.T) {
		service, q, _ := newVerifiedService(t)
		token, err := service.SignIn(bg, "user@example.com", "hunter2")
		if err != nil || token == "" {
			t.Fatalf("SignIn = %q, %v; want token, nil", token, err)
		}
		if _, ok := q.tokens[string(hashToken(token))]; !ok {
			t.Error("session token was not stored")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		service, _, _ := newVerifiedService(t)
		if _, err := service.SignIn(bg, "user@example.com", "wrong"); !errors.Is(err, ErrBadSignIn) {
			t.Errorf("err = %v, want ErrBadSignIn", err)
		}
	})

	t.Run("unknown email", func(t *testing.T) {
		service, _, _ := newVerifiedService(t)
		if _, err := service.SignIn(bg, "nobody@example.com", "hunter2"); !errors.Is(err, ErrBadSignIn) {
			t.Errorf("err = %v, want ErrBadSignIn", err)
		}
	})

	t.Run("inactive account can still log in", func(t *testing.T) {
		service, _, _ := newVerifiedService(t)
		if err := service.SetAccountActive(bg, "user@example.com", false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.SignIn(bg, "user@example.com", "hunter2"); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
}

func TestSessionEmail(t *testing.T) {
	bg := context.Background()
	q := newMockQuerier()
	mail := &mockSender{}
	service := newTestService(t, q, mail)
	_ = service.Register(bg, "user@example.com", "hunter2")
	verifyRaw := rawTokenFromLink(t, mail.sent[0].Body)
	_, _ = service.Verify(bg, verifyRaw)
	token, err := service.SignIn(bg, "user@example.com", "hunter2")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid token", func(t *testing.T) {
		got, ok := service.SessionEmail(bg, token)
		if !ok || got != "user@example.com" {
			t.Errorf("SessionEmail = %q, %v; want user@example.com, true", got, ok)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		if _, ok := service.SessionEmail(bg, ""); ok {
			t.Error("SessionEmail = true, want false")
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		if _, ok := service.SessionEmail(bg, "garbage"); ok {
			t.Error("SessionEmail = true, want false")
		}
	})

	t.Run("session valid across requests", func(t *testing.T) {
		if _, ok := service.SessionEmail(bg, token); !ok {
			t.Fatal("first SessionEmail = false, want true")
		}
		if _, ok := service.SessionEmail(bg, token); !ok {
			t.Error("second SessionEmail = false, want true")
		}
	})

	t.Run("expired session", func(t *testing.T) {
		q.now = q.now.Add(25 * time.Hour)
		if _, ok := service.SessionEmail(bg, token); ok {
			t.Error("expired SessionEmail = true, want false")
		}
	})
}

func TestDeactivateAndDeleteInvalidateSessions(t *testing.T) {
	bg := context.Background()
	setup := func(t *testing.T) (*AuthenticationService, string) {
		t.Helper()
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		_ = service.Register(bg, "user@example.com", "hunter2")
		raw := rawTokenFromLink(t, mail.sent[0].Body)
		_, _ = service.Verify(bg, raw)
		token, err := service.SignIn(bg, "user@example.com", "hunter2")
		if err != nil {
			t.Fatal(err)
		}
		return service, token
	}

	t.Run("SetAccountActive keeps sessions", func(t *testing.T) {
		service, token := setup(t)
		if err := service.SetAccountActive(bg, "user@example.com", false); err != nil {
			t.Fatal(err)
		}
		if _, ok := service.SessionEmail(bg, token); !ok {
			t.Error("session invalidated by deactivate")
		}
		user, _ := service.Querier.GetUserByEmail(bg, "user@example.com")
		if user.Active.Bool {
			t.Error("account should be inactive")
		}
	})

	t.Run("reactivate", func(t *testing.T) {
		service, _ := setup(t)
		_ = service.SetAccountActive(bg, "user@example.com", false)
		if err := service.SetAccountActive(bg, "user@example.com", true); err != nil {
			t.Fatal(err)
		}
		user, _ := service.Querier.GetUserByEmail(bg, "user@example.com")
		if !user.Active.Bool {
			t.Error("account should be active")
		}
	})

	t.Run("delete", func(t *testing.T) {
		service, token := setup(t)
		if err := service.DeleteAccount(bg, "user@example.com"); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Querier.GetUserByEmail(bg, "user@example.com"); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetUserByEmail err = %v, want ErrNoRows", err)
		}
		if _, ok := service.SessionEmail(bg, token); ok {
			t.Error("session still valid after delete")
		}
	})
}

func TestPasswordReset(t *testing.T) {
	bg := context.Background()
	newServiceWithUser := func(t *testing.T) (*AuthenticationService, *mockSender) {
		t.Helper()
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		if err := service.Register(bg, "user@example.com", "hunter2"); err != nil {
			t.Fatal(err)
		}
		return service, mail
	}

	t.Run("request sends a reset link", func(t *testing.T) {
		service, mail := newServiceWithUser(t)
		if err := service.RequestPasswordReset(bg, "user@example.com"); err != nil {
			t.Fatal(err)
		}
		// Registration sent the first mail; the reset is the second.
		if len(mail.sent) != 2 {
			t.Fatalf("sent %d emails, want 2", len(mail.sent))
		}
		if !strings.Contains(mail.sent[1].Body, "/reset?token=") {
			t.Errorf("reset body = %q", mail.sent[1].Body)
		}
	})

	t.Run("unknown email sends nothing", func(t *testing.T) {
		service, mail := newServiceWithUser(t)
		before := len(mail.sent)
		if err := service.RequestPasswordReset(bg, "nobody@example.com"); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		if len(mail.sent) != before {
			t.Error("an email was sent for an unknown address")
		}
	})

	t.Run("confirm sets new password and burns session", func(t *testing.T) {
		service, mail := newServiceWithUser(t)
		_ = service.RequestPasswordReset(bg, "user@example.com")
		raw := rawTokenFromLink(t, mail.sent[1].Body)

		// Establish a session before the reset.
		verifyRaw := rawTokenFromLink(t, mail.sent[0].Body)
		_, _ = service.Verify(bg, verifyRaw)
		sessionToken, err := service.SignIn(bg, "user@example.com", "hunter2")
		if err != nil {
			t.Fatal(err)
		}

		if err := service.ConfirmPasswordReset(bg, raw, "new-password"); err != nil {
			t.Fatal(err)
		}
		if _, err := service.SignIn(bg, "user@example.com", "new-password"); err != nil {
			t.Errorf("sign in with new password = %v, want nil", err)
		}
		if _, err := service.SignIn(bg, "user@example.com", "hunter2"); !errors.Is(err, ErrBadSignIn) {
			t.Errorf("sign in with old password err = %v, want ErrBadSignIn", err)
		}
		if _, ok := service.SessionEmail(bg, sessionToken); ok {
			t.Error("session survived password reset")
		}
	})

	t.Run("expired reset token", func(t *testing.T) {
		q := newMockQuerier()
		mail := &mockSender{}
		service := newTestService(t, q, mail)
		_ = service.Register(bg, "user@example.com", "hunter2")
		_ = service.RequestPasswordReset(bg, "user@example.com")
		raw := rawTokenFromLink(t, mail.sent[1].Body)

		q.now = q.now.Add(2 * time.Hour)
		if err := service.ConfirmPasswordReset(bg, raw, "new-password"); !errors.Is(err, ErrBadToken) {
			t.Errorf("err = %v, want ErrBadToken", err)
		}
	})

	t.Run("empty new password", func(t *testing.T) {
		service, _ := newServiceWithUser(t)
		if err := service.ConfirmPasswordReset(bg, "whatever", ""); !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("err = %v, want ErrInvalidPassword", err)
		}
	})
}

func TestRegisterSendsThroughSender(t *testing.T) {
	q := newMockQuerier()
	mail := &mockSender{}
	service := newTestService(t, q, mail)

	if err := service.Register(context.Background(), "user@example.com", "hunter2"); err != nil {
		t.Fatal(err)
	}
	if got := mail.sent[0].ToRecipients; len(got) != 1 || got[0] != "user@example.com" {
		t.Errorf("ToRecipients = %v", got)
	}
	if mail.sent[0].Subject != "Account Verification" {
		t.Errorf("Subject = %q", mail.sent[0].Subject)
	}
	if !strings.HasPrefix(mail.sent[0].Body, "rendered:") {
		t.Errorf("Body = %q, want the rendered template", mail.sent[0].Body)
	}
}
