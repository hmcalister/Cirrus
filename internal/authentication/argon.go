package authentication

import (
	"crypto/rand"
	"crypto/subtle"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  uint32 = 64 * 1024
	argonTime    uint32 = 3
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	saltLen             = 32
)

var dummyHash, dummySalt, _ = hashPassword([]byte("only you can prevent timing attacks"))

func computeHash(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func hashPassword(password []byte) ([]byte, []byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	hash := computeHash(password, salt)

	return hash, salt, nil
}

func checkPassword(password, salt, hash []byte) bool {
	return subtle.ConstantTimeCompare(computeHash(password, salt), hash) == 1
}
