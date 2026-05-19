package sym

import "testing"

func TestAESEncryptDecrypt(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes

	tests := []struct {
		name      string
		plaintext string
	}{
		{"phone number", "13812345678"},
		{"empty string", ""},
		{"long string", "this is a longer string for testing AES encryption"},
		{"chinese chars", "你好世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := AESEncrypt(tt.plaintext, key)
			if err != nil {
				t.Fatalf("AESEncrypt failed: %v", err)
			}

			if encrypted == tt.plaintext {
				t.Fatal("encrypted should not equal plaintext")
			}

			decrypted, err := AESDecrypt(encrypted, key)
			if err != nil {
				t.Fatalf("AESDecrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Fatalf("decrypted mismatch, got: %s, want: %s", decrypted, tt.plaintext)
			}
		})
	}
}

func TestAESEncryptDifferentCiphertext(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	plaintext := "13812345678"

	// same plaintext should produce different ciphertext each time (due to random nonce)
	enc1, _ := AESEncrypt(plaintext, key)
	enc2, _ := AESEncrypt(plaintext, key)

	if enc1 == enc2 {
		t.Fatal("same plaintext should produce different ciphertext due to random nonce")
	}
}

func TestAESInvalidKeyLength(t *testing.T) {
	shortKey := []byte("too-short")

	_, err := AESEncrypt("test", shortKey)
	if err == nil {
		t.Fatal("should fail with short key")
	}

	_, err = AESDecrypt("aabbccdd", shortKey)
	if err == nil {
		t.Fatal("should fail with short key")
	}
}
