// Package crypto implements envy's authenticated encryption primitives.
//
// Keys are derived from the master password with Argon2id and data is sealed
// with AES-256-GCM. The KDF parameters travel with the ciphertext so they can
// be raised in future releases without breaking existing stores.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// KeySize is the AES-256 key length in bytes.
	KeySize = 32
	// SaltSize is the length of the random KDF salt in bytes.
	SaltSize = 16
)

// ErrDecrypt is returned when ciphertext cannot be authenticated. It covers
// both a wrong password and tampered data; GCM cannot tell the two apart.
var ErrDecrypt = errors.New("decryption failed: wrong password or corrupted data")

// Params are the Argon2id cost parameters.
type Params struct {
	Time    uint32 `json:"t"`
	Memory  uint32 `json:"m"` // KiB
	Threads uint8  `json:"p"`
}

// DefaultParams follow the second recommended option in RFC 9106 §4
// (64 MiB, 3 passes), which keeps unlocks well under a second on laptops.
var DefaultParams = Params{Time: 3, Memory: 64 * 1024, Threads: 4}

// Validate rejects parameters that are too weak to be safe or large enough
// to exhaust memory when read from an untrusted file.
func (p Params) Validate() error {
	switch {
	case p.Time < 1 || p.Time > 64:
		return fmt.Errorf("argon2id: time cost %d out of range", p.Time)
	case p.Memory < 8*1024 || p.Memory > 4*1024*1024:
		return fmt.Errorf("argon2id: memory cost %d KiB out of range", p.Memory)
	case p.Threads < 1:
		return fmt.Errorf("argon2id: parallelism must be at least 1")
	}
	return nil
}

// NewSalt returns SaltSize cryptographically random bytes.
func NewSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// DeriveKey stretches password into a KeySize key with Argon2id.
func DeriveKey(password, salt []byte, p Params) []byte {
	return argon2.IDKey(password, salt, p.Time, p.Memory, p.Threads, KeySize)
}

// Seal encrypts plaintext under key with a fresh random nonce. aad is
// authenticated but not encrypted; envy uses it to bind the file header.
func Seal(key, plaintext, aad []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, aad), nil
}

// Open authenticates and decrypts ciphertext. It returns ErrDecrypt if the
// key is wrong or any of nonce, ciphertext or aad were modified.
func Open(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrDecrypt
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key length %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Legacy (envy 0.x) format: base64(salt[32] || nonce[12] || ciphertext)
// with a PBKDF2-SHA256 key at 100k iterations. Only decryption is supported;
// stores are rewritten in the current format on the next save.
const (
	legacySaltSize   = 32
	legacyIterations = 100_000
)

// DecryptLegacy decrypts a store written by envy 0.x.
func DecryptLegacy(encoded string, password []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("legacy store: %w", err)
	}
	if len(data) < legacySaltSize {
		return nil, ErrDecrypt
	}
	salt, rest := data[:legacySaltSize], data[legacySaltSize:]
	key, err := pbkdf2.Key(sha256.New, string(password), salt, legacyIterations, KeySize)
	if err != nil {
		return nil, err
	}

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(rest) < gcm.NonceSize() {
		return nil, ErrDecrypt
	}
	return Open(key, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], nil)
}
