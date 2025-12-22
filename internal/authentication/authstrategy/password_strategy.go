package authstrategy

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hmcalister/LiteralCloudService/internal/database"
	"golang.org/x/crypto/argon2"
)

const (
	passwordStrategy_SaltLen uint32 = 32
	argon2_TimeCost          uint32 = 1
	argon2_MemoryCost        uint32 = 8 * 1024
	argon2_Threads           uint8  = 1
	argon2_KeyLen            uint32 = 32
)

type PasswordAuthenticationStrategy struct {
	*baseAuthenticationStrategy
}

func NewPasswordStrategy() PasswordAuthenticationStrategy {
	passwordStrategy := PasswordAuthenticationStrategy{
		baseAuthenticationStrategy: newBaseAuthenticationStrategy(),
	}
	passwordStrategy.router.HandleFunc("POST /authenticate", passwordStrategy.authenticate)
	passwordStrategy.router.HandleFunc("POST /register", passwordStrategy.register)

	return passwordStrategy
}

func (passwordStrategy PasswordAuthenticationStrategy) GetAuthType() string {
	return "password"
}

func (passwordStrategy PasswordAuthenticationStrategy) authenticate(w http.ResponseWriter, r *http.Request) {

	email, requestPassword, err := passwordStrategy.extractEmailPasswordFromRequest(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if email == "" || requestPassword == "" {
		http.Error(w, "request is missing field in password authentication", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	passwordAuthenticationData, err := passwordStrategy.db.GetPasswordAuthentication(ctx, email)
	if passwordAuthenticationData.Email == "" {
		http.Error(w, "email does not exist with password authentication", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("error getting password data during password strategy authentication", "error", err)
		http.Error(w, "failed to retrieve password data from database in password authentication", http.StatusInternalServerError)
		return
	}

	requestHashedPassword := passwordStrategy.calculateHash([]byte(requestPassword), passwordAuthenticationData.Salt)
	if subtle.ConstantTimeCompare(requestHashedPassword, passwordAuthenticationData.HashedPassword) == 0 {
		http.Error(w, "incorrect email or password", http.StatusUnauthorized)
		return
	}

	// --------------------------------------------------------------------------------
	// The user is now authenticated, and we may return the auth token

	user, err := passwordStrategy.db.GetUserByEmail(ctx, email)
	if err != nil {
		http.Error(w, "failed to retrieve user data from database in password authentication", http.StatusInternalServerError)
		return
	}
	passwordStrategy.respondWithAuthToken(w, user)
}

func (passwordStrategy PasswordAuthenticationStrategy) register(w http.ResponseWriter, r *http.Request) {

	email, requestPassword, err := passwordStrategy.extractEmailPasswordFromRequest(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if email == "" || requestPassword == "" {
		http.Error(w, "request is missing field in password authentication", http.StatusBadRequest)
		return
	}

	salt := passwordStrategy.generateSalt()
	hashedPassword := passwordStrategy.calculateHash([]byte(requestPassword), salt)

	if !passwordStrategy.validatePassword(requestPassword) {
		http.Error(w, "password should be at least eight characters", http.StatusBadRequest)
		return
	}

	// Checking after hashing prevents timing attacks
	ctx := context.Background()
	existingUser, err := passwordStrategy.db.GetPasswordAuthentication(ctx, email)
	// If rows exist it means the user already exists in the database
	if existingUser.Email != "" {
		http.Error(w, "error occurred in password authentication", http.StatusBadRequest)
		return
	}

	// --------------------------------------------------------------------------------
	// Transaction Start
	tx, err := passwordStrategy.connPool.Begin(ctx)
	if err != nil {
		slog.Error("error starting transaction during password strategy registration", "error", err)
		http.Error(w, "error occurred in password authentication", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)
	txQueries := passwordStrategy.db.WithTx(tx)

	_, err = txQueries.UpsertUser(ctx, email)
	if err != nil {
		slog.Error("error upserting user during password strategy registration", "error", err)
		http.Error(w, "error occurred in password authentication", http.StatusInternalServerError)
		return
	}

	_, err = txQueries.CreatePasswordAuthentication(ctx, database.CreatePasswordAuthenticationParams{
		Email:          email,
		HashedPassword: hashedPassword,
		Salt:           salt,
	})
	if err != nil {
		slog.Error("error creating password authentication during password strategy registration", "error", err)
		http.Error(w, "error occurred in password authentication", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("error with transaction commit during password strategy registration", "error", err)
		http.Error(w, "error occurred in password authentication", http.StatusInternalServerError)
		return
	}
	// Transaction End
	// --------------------------------------------------------------------------------

	// --------------------------------------------------------------------------------
	// The user is now authenticated, and we may return the Paseto token

	user, err := passwordStrategy.db.GetUserByEmail(ctx, email)
	if err != nil {
		http.Error(w, "failed to retrieve user data from database in password authentication", http.StatusInternalServerError)
		return
	}
	passwordStrategy.respondWithAuthToken(w, user)
}

func (passwordStrategy PasswordAuthenticationStrategy) extractEmailPasswordFromRequest(r *http.Request) (email string, requestPassword string, err error) {

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", errors.New("could not parse JSON in password authentication")
		}
		email = req.Email
		requestPassword = req.Password
	} else {
		if err = r.ParseForm(); err != nil {
			return "", "", errors.New("could not parse form in password authentication")
		}
		email = r.PostFormValue("email")
		requestPassword = r.PostFormValue("password")
	}

	return email, requestPassword, nil
}

// Validate the given password.
// e.g. is longer than eight characters, contains a symbol, and so on.
// TODO: Implement reasonable checks
func (passwordStrategy PasswordAuthenticationStrategy) validatePassword(rawPassword string) bool {
	return len(rawPassword) >= 8
}

// Generate a new salt and return it.
//
// This function can error if a kernel function errors, although this should never happen.
func (passwordStrategy PasswordAuthenticationStrategy) generateSalt() ([]byte, error) {
	salt := make([]byte, passwordStrategy_SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	return salt, nil
}

// Perform the hash of a (plaintext) password with salt.
func (passwordStrategy PasswordAuthenticationStrategy) calculateHash(password []byte, salt []byte) []byte {
	hash := argon2.IDKey(password, salt, argon2_TimeCost, argon2_MemoryCost, argon2_Threads, argon2_KeyLen)

	return hash
}
