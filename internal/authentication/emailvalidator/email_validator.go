package emailvalidator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"
	"github.com/hmcalister/LiteralCloudService/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pendingCodeTimeout       time.Duration = 1 * time.Hour
	validationCodeByteLength int           = 32
)

var (
	ErrEmailAlreadyValidated = errors.New("email is already validated")
	ErrNoValidationCode      = errors.New("no validation code found for email")
	ErrInvalidValidationCode = errors.New("validation code does not match")
	ErrEmailClaimNotFound    = errors.New("email claim not found in token")
)

type EmailValidator struct {
	db               *database.Queries
	connPool         *pgxpool.Pool
	authTokenManager token.AuthTokenManager

	// Map from email to validation code.
	// Ensures only one code is available to an email at a time.
	// Codes are valid only for `pendingCodeTimeout` duration.
	pendingValidationCodeMap      map[string]string
	pendingValidationCodeMapMutex sync.RWMutex
	router                        *http.ServeMux
}

func NewEmailValidator(
	db *database.Queries,
	connPool *pgxpool.Pool,
	authTokenManager token.AuthTokenManager,
) *EmailValidator {
	ev := &EmailValidator{
		db:                       db,
		connPool:                 connPool,
		authTokenManager:         authTokenManager,
		pendingValidationCodeMap: make(map[string]string),
		router:                   http.NewServeMux(),
	}
	ev.router.HandleFunc("/{code}", ev.handleValidation)

	return ev
}

func (ev *EmailValidator) GetRouter() *http.ServeMux {
	return ev.router
}

func generateValidationCode() (string, error) {
	bytes := make([]byte, validationCodeByteLength)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	// Encode as URL-safe base64 to ensure it's ASCII-safe
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// CreateValidationCode checks if the email is already validated, and if not,
// creates a new validation code and stores it in the pending map.
// Returns the validation code to be sent to the user via email.
func (ev *EmailValidator) CreateValidationCode(ctx context.Context, email string) (string, error) {
	// Check if email is already validated in the database
	user, err := ev.db.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	if user.Validated {
		return "", ErrEmailAlreadyValidated
	}

	code, err := generateValidationCode()
	if err != nil {
		return "", err
	}

	ev.pendingValidationCodeMapMutex.Lock()
	ev.pendingValidationCodeMap[email] = code
	ev.pendingValidationCodeMapMutex.Unlock()

	go func() {
		time.Sleep(pendingCodeTimeout)
		ev.pendingValidationCodeMapMutex.Lock()
		delete(ev.pendingValidationCodeMap, email)
		ev.pendingValidationCodeMapMutex.Unlock()
	}()

	return code, nil
}

func (ev *EmailValidator) handleValidation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.PathValue("code")
	if code == "" {
		http.Error(w, "validation code is required", http.StatusBadRequest)
		return
	}

	authtoken, err := token.GetTokenFromRequestHeader(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	claims, ok := ev.authTokenManager.VerifyToken(authtoken)
	if !ok {
		http.Error(w, token.ErrInvalidBearerToken.Error(), http.StatusUnauthorized)
		return
	}
	emailClaim, exists := claims["email"]
	if !exists {
		http.Error(w, ErrEmailClaimNotFound.Error(), http.StatusBadRequest)
		return
	}
	email, ok := emailClaim.(string)
	if !ok {
		http.Error(w, ErrEmailClaimNotFound.Error(), http.StatusBadRequest)
		return
	}

	ev.pendingValidationCodeMapMutex.RLock()
	expectedCode, exists := ev.pendingValidationCodeMap[email]
	ev.pendingValidationCodeMapMutex.RUnlock()
	if !exists {
		http.Error(w, ErrNoValidationCode.Error(), http.StatusNotFound)
		return
	}
	if code != expectedCode {
		http.Error(w, ErrInvalidValidationCode.Error(), http.StatusForbidden)
		return
	}
	_, err = ev.db.SetUserValidatedStatus(ctx, database.SetUserValidatedStatusParams{
		Email:     email,
		Validated: true,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to set user validated status", "error", err, "email", email)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	ev.pendingValidationCodeMapMutex.Lock()
	delete(ev.pendingValidationCodeMap, email)
	ev.pendingValidationCodeMapMutex.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("email validated successfully"))
}
