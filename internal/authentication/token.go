package authentication

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func generateToken() (raw string, hash []byte) {
	b := make([]byte, 32)
	rand.Read(b)
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw)
}

func hashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}
