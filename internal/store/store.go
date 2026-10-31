// Package store manages envy's encrypted, multi-environment secret store.
//
// On disk a store is a small JSON envelope: a cleartext header that records
// the format version and KDF parameters, and an AES-256-GCM sealed payload.
// The header is passed to GCM as associated data, so tampering with it (for
// example to downgrade the KDF cost) makes decryption fail.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/anastanveer653/envy/internal/crypto"
)

const (
	// Dir is the per-project directory that holds the store.
	Dir = ".envy"
	// FileName is the encrypted store file inside Dir.
	FileName = "store.enc"

	formatV2 = "envy/v2"
	kdfName  = "argon2id"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrExists         = errors.New("store already exists")
	ErrNotInitialized = errors.New("no envy store found (run 'envy init' first)")
	ErrWrongPassword  = errors.New("incorrect master password or corrupted store")
)

// KDFParams are the Argon2id parameters used for newly written stores.
// Tests lower them to keep the suite fast.
var KDFParams = crypto.DefaultParams

var (
	keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
)

// Secret is a single stored value with metadata.
type Secret struct {
	Value     string    `json:"value"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type header struct {
	Format string `json:"format"`
	KDF    kdf    `json:"kdf"`
}

type kdf struct {
	Alg string `json:"alg"`
	crypto.Params
	Salt []byte `json:"salt"`
}

type envelope struct {
	header
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

type payload struct {
	Environments map[string]map[string]Secret `json:"environments"`
}

// Store is an unlocked store held in memory. Mutations are not persisted
// until Save is called.
type Store struct {
	path     string
	key      []byte
	salt     []byte
	params   crypto.Params
	envs     map[string]map[string]Secret
	migrated bool
}

// PathIn returns the store path for a project root.
func PathIn(root string) string {
	return filepath.Join(root, Dir, FileName)
}

// FindRoot walks up from start until it finds a directory containing a store.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(PathIn(dir)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialized
		}
		dir = parent
	}
}

// ValidateKey reports whether key is a portable environment variable name.
func ValidateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid key %q: use letters, digits and underscores, not starting with a digit", key)
	}
	return nil
}

// ValidateEnv reports whether name is a safe environment name. Names end up
// in file names such as .env.<name>, so path separators are never allowed.
func ValidateEnv(name string) error {
	if !envPattern.MatchString(name) {
		return fmt.Errorf("invalid environment %q: use letters, digits, '.', '_' or '-' (max 64)", name)
	}
	return nil
}

// Create initialises a new store at path with the given environments and
// writes it to disk. It fails with ErrExists if a store is already there.
func Create(path string, password []byte, envs ...string) (*Store, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, ErrExists
	}
	if err := KDFParams.Validate(); err != nil {
		return nil, err
	}
	salt, err := crypto.NewSalt()
	if err != nil {
		return nil, err
	}
	s := &Store{
		path:   path,
		salt:   salt,
		params: KDFParams,
		key:    crypto.DeriveKey(password, salt, KDFParams),
		envs:   make(map[string]map[string]Secret),
	}
	for _, e := range envs {
		if err := s.AddEnvironment(e); err != nil {
			return nil, err
		}
	}
	return s, s.Save()
}

// Open reads and decrypts the store at path. Stores written by envy 0.x are
// transparently upgraded; Migrated reports whether that happened.
func Open(path string, password []byte) (*Store, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotInitialized
	}
	if err != nil {
		return nil, err
	}

	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] != '{' {
		return openLegacy(path, string(trimmed), password)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}
	if env.Format != formatV2 {
		return nil, fmt.Errorf("unsupported store format %q (upgrade envy)", env.Format)
	}
	if env.KDF.Alg != kdfName {
		return nil, fmt.Errorf("unsupported key derivation %q", env.KDF.Alg)
	}
	if err := env.KDF.Params.Validate(); err != nil {
		return nil, err
	}

	aad, err := json.Marshal(env.header)
	if err != nil {
		return nil, err
	}
	key := crypto.DeriveKey(password, env.KDF.Salt, env.KDF.Params)
	plaintext, err := crypto.Open(key, env.Nonce, env.Data, aad)
	if err != nil {
		return nil, ErrWrongPassword
	}

	var p payload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, fmt.Errorf("parse store payload: %w", err)
	}
	if p.Environments == nil {
		p.Environments = make(map[string]map[string]Secret)
	}
	return &Store{
		path:   path,
		key:    key,
		salt:   env.KDF.Salt,
		params: env.KDF.Params,
		envs:   p.Environments,
	}, nil
}

func openLegacy(path, encoded string, password []byte) (*Store, error) {
	plaintext, err := crypto.DecryptLegacy(encoded, password)
	if err != nil {
		if errors.Is(err, crypto.ErrDecrypt) {
			return nil, ErrWrongPassword
		}
		return nil, err
	}
	var p payload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, fmt.Errorf("parse legacy store: %w", err)
	}
	if p.Environments == nil {
		p.Environments = make(map[string]map[string]Secret)
	}
	salt, err := crypto.NewSalt()
	if err != nil {
		return nil, err
	}
	return &Store{
		path:     path,
		salt:     salt,
		params:   KDFParams,
		key:      crypto.DeriveKey(password, salt, KDFParams),
		envs:     p.Environments,
		migrated: true,
	}, nil
}

// Migrated reports whether the store was read from a legacy format and will
// be rewritten in the current format on the next Save.
func (s *Store) Migrated() bool { return s.migrated }

// Path returns the file the store is persisted to.
func (s *Store) Path() string { return s.path }

// Save encrypts the store and atomically replaces the file on disk, so a
// crash mid-write can never leave a truncated store behind.
func (s *Store) Save() error {
	plaintext, err := json.Marshal(payload{Environments: s.envs})
	if err != nil {
		return err
	}
	h := header{
		Format: formatV2,
		KDF:    kdf{Alg: kdfName, Params: s.params, Salt: s.salt},
	}
	aad, err := json.Marshal(h)
	if err != nil {
		return err
	}
	nonce, ct, err := crypto.Seal(s.key, plaintext, aad)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(envelope{header: h, Nonce: nonce, Data: ct}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.path, append(out, '\n')); err != nil {
		return err
	}
	s.migrated = false
	return nil
}

// ChangePassword re-keys the store with a new password and fresh salt, then
// saves it.
func (s *Store) ChangePassword(password []byte) error {
	salt, err := crypto.NewSalt()
	if err != nil {
		return err
	}
	s.salt = salt
	s.params = KDFParams
	s.key = crypto.DeriveKey(password, salt, KDFParams)
	return s.Save()
}

// AddEnvironment creates an empty environment if it does not exist yet.
func (s *Store) AddEnvironment(env string) error {
	if err := ValidateEnv(env); err != nil {
		return err
	}
	if s.envs[env] == nil {
		s.envs[env] = make(map[string]Secret)
	}
	return nil
}

// RemoveEnvironment deletes env and every secret in it.
func (s *Store) RemoveEnvironment(env string) error {
	if !s.HasEnvironment(env) {
		return fmt.Errorf("environment %q: %w", env, ErrNotFound)
	}
	delete(s.envs, env)
	return nil
}

// HasEnvironment reports whether env exists.
func (s *Store) HasEnvironment(env string) bool {
	_, ok := s.envs[env]
	return ok
}

// Environments returns all environment names, sorted.
func (s *Store) Environments() []string {
	names := make([]string, 0, len(s.envs))
	for name := range s.envs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Set creates or updates a secret, creating env if needed. An empty note
// keeps the existing note. It reports whether the key already existed.
func (s *Store) Set(env, key, value, note string) (updated bool, err error) {
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	if err := s.AddEnvironment(env); err != nil {
		return false, err
	}
	now := time.Now().UTC()
	sec, exists := s.envs[env][key]
	if !exists {
		sec.CreatedAt = now
	}
	sec.Value = value
	sec.UpdatedAt = now
	if note != "" {
		sec.Note = note
	}
	s.envs[env][key] = sec
	return exists, nil
}

// Get returns a single secret.
func (s *Store) Get(env, key string) (Secret, error) {
	secrets, ok := s.envs[env]
	if !ok {
		return Secret{}, fmt.Errorf("environment %q: %w", env, ErrNotFound)
	}
	sec, ok := secrets[key]
	if !ok {
		return Secret{}, fmt.Errorf("key %q in %q: %w", key, env, ErrNotFound)
	}
	return sec, nil
}

// Delete removes a secret.
func (s *Store) Delete(env, key string) error {
	if _, err := s.Get(env, key); err != nil {
		return err
	}
	delete(s.envs[env], key)
	return nil
}

// Keys returns the sorted key names in env.
func (s *Store) Keys(env string) ([]string, error) {
	secrets, ok := s.envs[env]
	if !ok {
		return nil, fmt.Errorf("environment %q: %w", env, ErrNotFound)
	}
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// Secrets returns a copy of every secret in env.
func (s *Store) Secrets(env string) (map[string]Secret, error) {
	secrets, ok := s.envs[env]
	if !ok {
		return nil, fmt.Errorf("environment %q: %w", env, ErrNotFound)
	}
	out := make(map[string]Secret, len(secrets))
	for k, v := range secrets {
		out[k] = v
	}
	return out, nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// CreateTemp opens the file with mode 0600.
	tmp, err := os.CreateTemp(dir, ".store-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
