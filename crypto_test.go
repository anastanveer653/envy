package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		password  string
	}{
		{"simple string", "hello world", "password123"},
		{"complex secret", "sk-abc123XYZ!@#$%", "my-secure-password"},
		{"empty value", "", "password"},
		{"unicode", "データベース接続文字列", "password"},
		{"long value", strings.Repeat("a", 1000), "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.plaintext, tt.password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if encrypted == tt.plaintext {
				t.Error("Encrypted text should differ from plaintext")
			}

			decrypted, err := Decrypt(encrypted, tt.password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Expected %q, got %q", tt.plaintext, decrypted)
			}
		})
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	encrypted, err := Encrypt("secret value", "correct-password")
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(encrypted, "wrong-password")
	if err == nil {
		t.Error("Expected error with wrong password")
	}
}

func TestEncryptProducesUniqueOutput(t *testing.T) {
	// Same input should produce different ciphertext (due to random salt/nonce)
	enc1, _ := Encrypt("same value", "same password")
	enc2, _ := Encrypt("same value", "same password")

	if enc1 == enc2 {
		t.Error("Two encryptions of the same value should differ (random salt)")
	}
}

func TestHashPassword(t *testing.T) {
	h1 := HashPassword("password")
	h2 := HashPassword("password")
	h3 := HashPassword("different")

	if h1 != h2 {
		t.Error("Same password should produce same hash")
	}
	if h1 == h3 {
		t.Error("Different passwords should produce different hashes")
	}
}




