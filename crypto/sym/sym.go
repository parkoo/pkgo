package sym

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// --- AES-256-GCM ---

// AESEncrypt encrypts plaintext using AES-256-GCM with the given key.
// Key must be 32 bytes (256 bits). Returns hex-encoded ciphertext.
func AESEncrypt(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("aes key must be 32 bytes, got: %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create aes cipher failed, err: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm failed, err: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce failed, err: %v", err)
	}

	// nonce is prepended to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// AESDecrypt decrypts hex-encoded ciphertext using AES-256-GCM with the given key.
// Key must be 32 bytes (256 bits).
func AESDecrypt(ciphertextHex string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("aes key must be 32 bytes, got: %d", len(key))
	}

	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decode hex failed, err: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create aes cipher failed, err: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm failed, err: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("aes decrypt failed, err: %v", err)
	}

	return string(plaintext), nil
}
