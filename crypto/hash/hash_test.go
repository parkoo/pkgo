package hash

import (
	"strings"
	"testing"
)

func TestSHA256Hash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"13812345678", "e3d2909bca0e1c7bf0a1cbfebfb0aa2cf1bdfbabdbe73e3b66d5b428e1b31a7b"},
	}

	for _, tt := range tests {
		result := SHA256Hash(tt.input)
		if len(result) != 64 {
			t.Fatalf("SHA256 hash should be 64 hex chars, got: %d", len(result))
		}
		// same input should always produce same hash
		result2 := SHA256Hash(tt.input)
		if result != result2 {
			t.Fatal("SHA256 should be deterministic")
		}
	}
}

func TestSHA256HashDifferentInputs(t *testing.T) {
	h1 := SHA256Hash("13812345678")
	h2 := SHA256Hash("13812345679")

	if h1 == h2 {
		t.Fatal("different inputs should produce different hashes")
	}
}

func TestBcryptHashAndCompare(t *testing.T) {
	password := "MyP@ssw0rd123"

	hash, err := BcryptHash(password)
	if err != nil {
		t.Fatalf("BcryptHash failed: %v", err)
	}

	// hash should start with $2a$ (bcrypt prefix)
	if !strings.HasPrefix(hash, "$2a$") {
		t.Fatalf("bcrypt hash should start with $2a$, got: %s", hash)
	}

	// correct password should match
	if !BcryptCompare(hash, password) {
		t.Fatal("correct password should match")
	}

	// wrong password should not match
	if BcryptCompare(hash, "wrongpassword") {
		t.Fatal("wrong password should not match")
	}
}

func TestBcryptDifferentHashes(t *testing.T) {
	password := "SamePassword"

	hash1, _ := BcryptHash(password)
	hash2, _ := BcryptHash(password)

	// same password should produce different hashes (different salt each time)
	if hash1 == hash2 {
		t.Fatal("bcrypt should produce different hashes for same password")
	}

	// but both should still match the original password
	if !BcryptCompare(hash1, password) || !BcryptCompare(hash2, password) {
		t.Fatal("both hashes should match the original password")
	}
}
