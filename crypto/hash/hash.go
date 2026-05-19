package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// --- SHA256 ---

// SHA256Hash returns the hex-encoded SHA256 hash of the input string.
func SHA256Hash(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

// --- bcrypt ---

// BcryptHash generates a bcrypt hash from the plaintext password.
// Uses bcrypt.DefaultCost (10).
func BcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash failed, err: %v", err)
	}
	return string(hash), nil
}

// BcryptCompare compares a plaintext password with a bcrypt hash.
// Returns true if the password matches the hash.
func BcryptCompare(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
