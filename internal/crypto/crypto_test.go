package crypto

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// fastParams keeps the test suite quick; production code uses DefaultParams.
var fastParams = Params{Time: 1, Memory: 8 * 1024, Threads: 1}

func TestSealOpenRoundTrip(t *testing.T) {
	salt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	key := DeriveKey([]byte("correct horse"), salt, fastParams)

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"simple", []byte("hello world")},
		{"symbols", []byte("sk-abc123XYZ!@#$%")},
		{"empty", []byte{}},
		{"unicode", []byte("データベース接続文字列")},
		{"large", bytes.Repeat([]byte("a"), 1<<16)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aad := []byte("header")
			nonce, ct, err := Seal(key, tt.plaintext, aad)
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}
			got, err := Open(key, nonce, ct, aad)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if !bytes.Equal(got, tt.plaintext) {
				t.Fatalf("round trip mismatch: got %q want %q", got, tt.plaintext)
			}
		})
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	salt, _ := NewSalt()
	nonce, ct, err := Seal(DeriveKey([]byte("right"), salt, fastParams), []byte("secret"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Open(DeriveKey([]byte("wrong"), salt, fastParams), nonce, ct, nil)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
}

func TestOpenDetectsTampering(t *testing.T) {
	salt, _ := NewSalt()
	key := DeriveKey([]byte("pw"), salt, fastParams)
	nonce, ct, err := Seal(key, []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}

	flipped := bytes.Clone(ct)
	flipped[0] ^= 0x01
	if _, err := Open(key, nonce, flipped, []byte("aad")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("modified ciphertext: want ErrDecrypt, got %v", err)
	}
	if _, err := Open(key, nonce, ct, []byte("AAD")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("modified aad: want ErrDecrypt, got %v", err)
	}
	if _, err := Open(key, nonce[:4], ct, []byte("aad")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("short nonce: want ErrDecrypt, got %v", err)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	key := make([]byte, KeySize)
	n1, c1, _ := Seal(key, []byte("same"), nil)
	n2, c2, _ := Seal(key, []byte("same"), nil)
	if bytes.Equal(n1, n2) || bytes.Equal(c1, c2) {
		t.Fatal("two seals of the same plaintext must differ")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := []byte("0123456789abcdef")
	a := DeriveKey([]byte("pw"), salt, fastParams)
	b := DeriveKey([]byte("pw"), salt, fastParams)
	c := DeriveKey([]byte("pw"), []byte("fedcba9876543210"), fastParams)
	if !bytes.Equal(a, b) {
		t.Error("same inputs must derive the same key")
	}
	if bytes.Equal(a, c) {
		t.Error("different salts must derive different keys")
	}
	if len(a) != KeySize {
		t.Errorf("key length = %d, want %d", len(a), KeySize)
	}
}

func TestParamsValidate(t *testing.T) {
	if err := DefaultParams.Validate(); err != nil {
		t.Errorf("DefaultParams invalid: %v", err)
	}
	bad := []Params{
		{Time: 0, Memory: 64 * 1024, Threads: 1},
		{Time: 3, Memory: 1024, Threads: 1},
		{Time: 3, Memory: 1 << 30, Threads: 1},
		{Time: 3, Memory: 64 * 1024, Threads: 0},
	}
	for _, p := range bad {
		if err := p.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want error", p)
		}
	}
}

// legacyEncrypt reproduces the envy 0.x writer so we can prove old stores
// remain readable.
func legacyEncrypt(t *testing.T, plaintext, password string) string {
	t.Helper()
	salt := make([]byte, legacySaltSize)
	rand.Read(salt)
	key, _ := pbkdf2.Key(sha256.New, password, salt, legacyIterations, KeySize)
	gcm, err := newGCM(key)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(append(salt, ct...))
}

func TestDecryptLegacy(t *testing.T) {
	enc := legacyEncrypt(t, `{"version":"1.0.0"}`, "old-password")

	got, err := DecryptLegacy(enc, []byte("old-password"))
	if err != nil {
		t.Fatalf("DecryptLegacy: %v", err)
	}
	if string(got) != `{"version":"1.0.0"}` {
		t.Errorf("got %q", got)
	}

	if _, err := DecryptLegacy(enc, []byte("nope")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("wrong password: want ErrDecrypt, got %v", err)
	}
	if _, err := DecryptLegacy(base64.StdEncoding.EncodeToString([]byte("short")), []byte("x")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("short input: want ErrDecrypt, got %v", err)
	}
	if _, err := DecryptLegacy(strings.Repeat("!", 10), []byte("x")); err == nil {
		t.Error("invalid base64 should fail")
	}
}
