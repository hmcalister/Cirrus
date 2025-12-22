package token_test

import (
	"testing"
	"time"

	"github.com/hmcalister/LiteralCloudService/internal/authentication/token"

	"aidanwoods.dev/go-paseto"
)

// generateTestKeyPair generates a valid PASETO v4 asymmetric key pair for testing
func generateTestKeyPair() (secretHex string, publicHex string) {
	secretKey := paseto.NewV4AsymmetricSecretKey()
	publicKey := secretKey.Public()
	return secretKey.ExportHex(), publicKey.ExportHex()
}

func TestNewPasetoManager_ValidKey(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if manager == nil {
		t.Fatal("expected manager to be non-nil")
	}
}

func TestNewPasetoManager_InvalidKey(t *testing.T) {
	tests := []struct {
		name      string
		secretKey string
	}{
		{
			name:      "empty string",
			secretKey: "",
		},
		{
			name:      "invalid hex",
			secretKey: "not-a-valid-hex-string",
		},
		{
			name:      "too short",
			secretKey: "abc123",
		},
		{
			name:      "random string",
			secretKey: "this is definitely not a valid key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := token.NewPasetoManager(tt.secretKey)

			if err != token.ErrInvalidSigningSecret {
				t.Errorf("expected error %v, got: %v", token.ErrInvalidSigningSecret, err)
			}
			if manager != nil {
				t.Error("expected manager to be nil")
			}
		})
	}
}

func TestCreateToken_BasicClaims(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	claims := map[string]interface{}{
		"user_id": "12345",
		"email":   "test@example.com",
	}

	tokenStr := manager.CreateToken(claims)

	if tokenStr == "" {
		t.Error("expected non-empty token")
	}
	if len(tokenStr) < 10 {
		t.Errorf("token seems too short: %s", tokenStr)
	}
}

func TestCreateToken_EmptyClaims(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	claims := map[string]interface{}{}

	tokenStr := manager.CreateToken(claims)

	if tokenStr == "" {
		t.Error("expected non-empty token")
	}
}

func TestCreateToken_VariousClaimTypes(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	claims := map[string]interface{}{
		"string_claim": "value",
		"int_claim":    42,
		"bool_claim":   true,
		"float_claim":  3.14,
		"array_claim":  []string{"a", "b", "c"},
	}

	tokenStr := manager.CreateToken(claims)

	if tokenStr == "" {
		t.Error("expected non-empty token")
	}
}

func TestVerifyToken_ValidToken(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	expectedClaims := map[string]interface{}{
		"user_id": "12345",
		"email":   "test@example.com",
		"role":    "admin",
	}

	tokenStr := manager.CreateToken(expectedClaims)
	claims, ok := manager.VerifyToken(tokenStr)

	if !ok {
		t.Error("expected token verification to succeed")
	}
	if claims == nil {
		t.Fatal("expected claims to be non-nil")
	}

	// Verify custom claims
	if claims["user_id"] != "12345" {
		t.Errorf("expected user_id to be '12345', got: %v", claims["user_id"])
	}
	if claims["email"] != "test@example.com" {
		t.Errorf("expected email to be 'test@example.com', got: %v", claims["email"])
	}
	if claims["role"] != "admin" {
		t.Errorf("expected role to be 'admin', got: %v", claims["role"])
	}

	// Verify standard claims are present
	if claims["aud"] != token.PASETO_AUDIENCE {
		t.Errorf("expected aud to be '%s', got: %v", token.PASETO_AUDIENCE, claims["aud"])
	}
	if claims["iss"] != token.PASETO_ISSUED_BY {
		t.Errorf("expected iss to be '%s', got: %v", token.PASETO_ISSUED_BY, claims["iss"])
	}
	if claims["sub"] != token.PASETO_SUBJECT {
		t.Errorf("expected sub to be '%s', got: %v", token.PASETO_SUBJECT, claims["sub"])
	}
	if claims["iat"] == nil {
		t.Error("expected iat to be present")
	}
	if claims["nbf"] == nil {
		t.Error("expected nbf to be present")
	}
	if claims["exp"] == nil {
		t.Error("expected exp to be present")
	}
}

func TestVerifyToken_InvalidToken(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty string",
			token: "",
		},
		{
			name:  "random string",
			token: "not-a-valid-token",
		},
		{
			name:  "malformed paseto",
			token: "v4.public.invalid",
		},
		{
			name:  "wrong version",
			token: "v3.public.something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, ok := manager.VerifyToken(tt.token)

			if ok {
				t.Error("expected token verification to fail")
			}
			if claims != nil {
				t.Error("expected claims to be nil")
			}
		})
	}
}

func TestVerifyToken_TamperedToken(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	originalToken := manager.CreateToken(map[string]interface{}{
		"user_id": "12345",
	})

	// Tamper with the token by modifying a character
	tamperedToken := originalToken[:len(originalToken)-5] + "XXXXX"

	claims, ok := manager.VerifyToken(tamperedToken)

	if ok {
		t.Error("expected tampered token verification to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestVerifyToken_DifferentKey(t *testing.T) {
	secretHex1, _ := generateTestKeyPair()
	manager1, err := token.NewPasetoManager(secretHex1)
	if err != nil {
		t.Fatalf("failed to create manager1: %v", err)
	}

	secretHex2, _ := generateTestKeyPair()
	manager2, err := token.NewPasetoManager(secretHex2)
	if err != nil {
		t.Fatalf("failed to create manager2: %v", err)
	}

	// Create token with manager1
	tokenStr := manager1.CreateToken(map[string]interface{}{
		"user_id": "12345",
	})

	// Try to verify with manager2 (different key)
	claims, ok := manager2.VerifyToken(tokenStr)

	if ok {
		t.Error("expected verification with different key to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestVerifyToken_ExpiredToken(t *testing.T) {
	// This test verifies that expired tokens are rejected
	// Since we can't easily create an expired token (the CreateToken method
	// sets expiration internally), we'll create a token with a custom expired time
	// by using the PASETO library directly

	secretKey := paseto.NewV4AsymmetricSecretKey()

	// Create a token that's already expired
	now := time.Now()
	rawToken := paseto.NewToken()
	rawToken.SetAudience(token.PASETO_AUDIENCE)
	rawToken.SetIssuer(token.PASETO_ISSUED_BY)
	rawToken.SetSubject(token.PASETO_SUBJECT)
	rawToken.SetIssuedAt(now.Add(-2 * time.Hour))
	rawToken.SetNotBefore(now.Add(-2 * time.Hour))
	rawToken.SetExpiration(now.Add(-1 * time.Hour)) // Expired 1 hour ago
	rawToken.Set("user_id", "12345")

	expiredToken := rawToken.V4Sign(secretKey, nil)

	// Create manager with the same key
	manager, err := token.NewPasetoManager(secretKey.ExportHex())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Verify the expired token should fail
	claims, ok := manager.VerifyToken(expiredToken)

	if ok {
		t.Error("expected expired token verification to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestVerifyToken_NotYetValidToken(t *testing.T) {
	// Test a token with nbf (not before) in the future
	secretKey := paseto.NewV4AsymmetricSecretKey()

	now := time.Now()
	rawToken := paseto.NewToken()
	rawToken.SetAudience(token.PASETO_AUDIENCE)
	rawToken.SetIssuer(token.PASETO_ISSUED_BY)
	rawToken.SetSubject(token.PASETO_SUBJECT)
	rawToken.SetIssuedAt(now)
	rawToken.SetNotBefore(now.Add(2 * time.Hour)) // Valid 2 hours from now
	rawToken.SetExpiration(now.Add(3 * time.Hour))
	rawToken.Set("user_id", "12345")

	futureToken := rawToken.V4Sign(secretKey, nil)

	// Create manager with the same key
	manager, err := token.NewPasetoManager(secretKey.ExportHex())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Verify the not-yet-valid token should fail
	claims, ok := manager.VerifyToken(futureToken)

	if ok {
		t.Error("expected not-yet-valid token verification to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestVerifyToken_WrongAudience(t *testing.T) {
	secretKey := paseto.NewV4AsymmetricSecretKey()

	now := time.Now()
	rawToken := paseto.NewToken()
	rawToken.SetAudience("WrongAudience")
	rawToken.SetIssuer(token.PASETO_ISSUED_BY)
	rawToken.SetSubject(token.PASETO_SUBJECT)
	rawToken.SetIssuedAt(now)
	rawToken.SetNotBefore(now)
	rawToken.SetExpiration(now.Add(1 * time.Hour))
	rawToken.Set("user_id", "12345")

	wrongAudienceToken := rawToken.V4Sign(secretKey, nil)

	manager, err := token.NewPasetoManager(secretKey.ExportHex())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	claims, ok := manager.VerifyToken(wrongAudienceToken)

	if ok {
		t.Error("expected verification with wrong audience to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestVerifyToken_WrongIssuer(t *testing.T) {
	secretKey := paseto.NewV4AsymmetricSecretKey()

	now := time.Now()
	rawToken := paseto.NewToken()
	rawToken.SetAudience(token.PASETO_AUDIENCE)
	rawToken.SetIssuer("WrongIssuer")
	rawToken.SetSubject(token.PASETO_SUBJECT)
	rawToken.SetIssuedAt(now)
	rawToken.SetNotBefore(now)
	rawToken.SetExpiration(now.Add(1 * time.Hour))
	rawToken.Set("user_id", "12345")

	wrongIssuerToken := rawToken.V4Sign(secretKey, nil)

	manager, err := token.NewPasetoManager(secretKey.ExportHex())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	claims, ok := manager.VerifyToken(wrongIssuerToken)

	if ok {
		t.Error("expected verification with wrong issuer to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestVerifyToken_WrongSubject(t *testing.T) {
	secretKey := paseto.NewV4AsymmetricSecretKey()

	now := time.Now()
	rawToken := paseto.NewToken()
	rawToken.SetAudience(token.PASETO_AUDIENCE)
	rawToken.SetIssuer(token.PASETO_ISSUED_BY)
	rawToken.SetSubject("WrongSubject")
	rawToken.SetIssuedAt(now)
	rawToken.SetNotBefore(now)
	rawToken.SetExpiration(now.Add(1 * time.Hour))
	rawToken.Set("user_id", "12345")

	wrongSubjectToken := rawToken.V4Sign(secretKey, nil)

	manager, err := token.NewPasetoManager(secretKey.ExportHex())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	claims, ok := manager.VerifyToken(wrongSubjectToken)

	if ok {
		t.Error("expected verification with wrong subject to fail")
	}
	if claims != nil {
		t.Error("expected claims to be nil")
	}
}

func TestTokenRoundTrip_MultipleClaims(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	originalClaims := map[string]interface{}{
		"user_id":  "abc-123",
		"username": "testuser",
		"email":    "test@example.com",
		"role":     "admin",
		"active":   true,
		"age":      30,
	}

	tokenStr := manager.CreateToken(originalClaims)
	retrievedClaims, ok := manager.VerifyToken(tokenStr)

	if !ok {
		t.Fatal("expected token verification to succeed")
	}

	if retrievedClaims["user_id"] != originalClaims["user_id"] {
		t.Errorf("user_id mismatch: expected %v, got %v", originalClaims["user_id"], retrievedClaims["user_id"])
	}
	if retrievedClaims["username"] != originalClaims["username"] {
		t.Errorf("username mismatch: expected %v, got %v", originalClaims["username"], retrievedClaims["username"])
	}
	if retrievedClaims["email"] != originalClaims["email"] {
		t.Errorf("email mismatch: expected %v, got %v", originalClaims["email"], retrievedClaims["email"])
	}
	if retrievedClaims["role"] != originalClaims["role"] {
		t.Errorf("role mismatch: expected %v, got %v", originalClaims["role"], retrievedClaims["role"])
	}
	if retrievedClaims["active"] != originalClaims["active"] {
		t.Errorf("active mismatch: expected %v, got %v", originalClaims["active"], retrievedClaims["active"])
	}

	// Note: JSON numbers are decoded as float64
	if retrievedClaims["age"] != float64(30) {
		t.Errorf("age mismatch: expected %v, got %v", float64(30), retrievedClaims["age"])
	}
}

func TestTokenRoundTrip_EmptyClaims(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	originalClaims := map[string]interface{}{}

	tokenStr := manager.CreateToken(originalClaims)
	retrievedClaims, ok := manager.VerifyToken(tokenStr)

	if !ok {
		t.Fatal("expected token verification to succeed")
	}

	// Should still have standard claims
	if retrievedClaims["aud"] == nil {
		t.Error("expected aud claim to be present")
	}
	if retrievedClaims["iss"] == nil {
		t.Error("expected iss claim to be present")
	}
	if retrievedClaims["sub"] == nil {
		t.Error("expected sub claim to be present")
	}
}

func TestConcurrentTokenOperations(t *testing.T) {
	secretHex, _ := generateTestKeyPair()
	manager, err := token.NewPasetoManager(secretHex)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	const numGoroutines = 100
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	// Test concurrent token creation and verification
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			claims := map[string]interface{}{
				"user_id": id,
				"test":    "concurrent",
			}

			tokenStr := manager.CreateToken(claims)
			retrievedClaims, ok := manager.VerifyToken(tokenStr)

			if !ok {
				errors <- err
				done <- false
				return
			}

			if retrievedClaims["user_id"] != float64(id) {
				errors <- err
				done <- false
				return
			}

			if retrievedClaims["test"] != "concurrent" {
				errors <- err
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		result := <-done
		if !result {
			t.Error("concurrent operation failed")
		}
	}

	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent operation error: %v", err)
		}
	}
}
