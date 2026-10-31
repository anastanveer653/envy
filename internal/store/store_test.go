package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anastanveer653/envy/internal/crypto"
)

var pw = []byte("test-password-123")

func TestMain(m *testing.M) {
	KDFParams = crypto.Params{Time: 1, Memory: 8 * 1024, Threads: 1}
	os.Exit(m.Run())
}

func newStore(t *testing.T, envs ...string) (*Store, string) {
	t.Helper()
	path := PathIn(t.TempDir())
	s, err := Create(path, pw, envs...)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return s, path
}

func TestCreateAndReopen(t *testing.T) {
	s, path := newStore(t, "development", "production")
	if _, err := s.Set("development", "API_KEY", "sk-test123", "Test key"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path, pw)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sec, err := reopened.Get("development", "API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if sec.Value != "sk-test123" || sec.Note != "Test key" {
		t.Errorf("got %+v", sec)
	}
	if got := reopened.Environments(); strings.Join(got, ",") != "development,production" {
		t.Errorf("Environments() = %v", got)
	}
}

func TestCreateRefusesExisting(t *testing.T) {
	_, path := newStore(t)
	if _, err := Create(path, pw); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
}

func TestOpenWrongPassword(t *testing.T) {
	_, path := newStore(t, "development")
	if _, err := Open(path, []byte("wrong")); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("want ErrWrongPassword, got %v", err)
	}
}

func TestOpenMissing(t *testing.T) {
	if _, err := Open(PathIn(t.TempDir()), pw); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("want ErrNotInitialized, got %v", err)
	}
}

func TestHeaderTamperingDetected(t *testing.T) {
	_, path := newStore(t, "development")
	raw, _ := os.ReadFile(path)

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	// Bump the time cost: the derived key changes and the header no longer
	// matches the authenticated data, so this must not open.
	doc["kdf"].(map[string]any)["t"] = 2
	tampered, _ := json.Marshal(doc)
	os.WriteFile(path, tampered, 0o600)

	if _, err := Open(path, pw); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("want ErrWrongPassword for tampered header, got %v", err)
	}
}

func TestOpenRejectsWeakParams(t *testing.T) {
	_, path := newStore(t)
	raw, _ := os.ReadFile(path)
	var doc map[string]any
	json.Unmarshal(raw, &doc)
	doc["kdf"].(map[string]any)["m"] = 1
	tampered, _ := json.Marshal(doc)
	os.WriteFile(path, tampered, 0o600)

	if _, err := Open(path, pw); err == nil || errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected a parameter validation error, got %v", err)
	}
}

func TestStoreFileContainsNoPlaintext(t *testing.T) {
	s, path := newStore(t)
	s.Set("production", "STRIPE_KEY", "sk_live_supersecretvalue", "")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, needle := range []string{"STRIPE_KEY", "sk_live_supersecretvalue", "production"} {
		if strings.Contains(string(raw), needle) {
			t.Errorf("store file leaks %q", needle)
		}
	}
}

func TestSetPreservesCreatedAtAndNote(t *testing.T) {
	s, _ := newStore(t)
	if updated, _ := s.Set("dev", "K", "v1", "first note"); updated {
		t.Error("first Set should report a new key")
	}
	first, _ := s.Get("dev", "K")

	updated, err := s.Set("dev", "K", "v2", "")
	if err != nil || !updated {
		t.Fatalf("second Set: updated=%v err=%v", updated, err)
	}
	second, _ := s.Get("dev", "K")
	if second.Value != "v2" {
		t.Errorf("value = %q", second.Value)
	}
	if second.Note != "first note" {
		t.Errorf("empty note should keep existing note, got %q", second.Note)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Error("CreatedAt changed on update")
	}
}

func TestKeysSorted(t *testing.T) {
	s, _ := newStore(t)
	for _, k := range []string{"B_KEY", "A_KEY", "C_KEY"} {
		s.Set("dev", k, "v", "")
	}
	keys, err := s.Keys("dev")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(keys, ",") != "A_KEY,B_KEY,C_KEY" {
		t.Errorf("Keys() = %v", keys)
	}
}

func TestDelete(t *testing.T) {
	s, _ := newStore(t)
	s.Set("dev", "KEY", "value", "")
	if err := s.Delete("dev", "KEY"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("dev", "KEY"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: %v", err)
	}
	if err := s.Delete("dev", "KEY"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting a missing key should be ErrNotFound, got %v", err)
	}
}

func TestValidation(t *testing.T) {
	s, _ := newStore(t)
	badKeys := []string{"", "1ABC", "MY-KEY", "with space", "A=B", "ключ"}
	for _, k := range badKeys {
		if _, err := s.Set("dev", k, "v", ""); err == nil {
			t.Errorf("Set accepted invalid key %q", k)
		}
	}
	badEnvs := []string{"", "../etc", "a/b", `a\b`, ".hidden", strings.Repeat("x", 65)}
	for _, e := range badEnvs {
		if _, err := s.Set(e, "KEY", "v", ""); err == nil {
			t.Errorf("Set accepted invalid env %q", e)
		}
	}
	for _, e := range []string{"dev", "production", "pr-123", "eu.west_1"} {
		if err := ValidateEnv(e); err != nil {
			t.Errorf("ValidateEnv(%q): %v", e, err)
		}
	}
}

func TestChangePassword(t *testing.T) {
	s, path := newStore(t)
	s.Set("dev", "K", "v", "")
	if err := s.ChangePassword([]byte("new-password")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, pw); !errors.Is(err, ErrWrongPassword) {
		t.Error("old password still opens the store")
	}
	reopened, err := Open(path, []byte("new-password"))
	if err != nil {
		t.Fatal(err)
	}
	if sec, _ := reopened.Get("dev", "K"); sec.Value != "v" {
		t.Errorf("value lost after password change: %+v", sec)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	s, path := newStore(t)
	for i := 0; i < 3; i++ {
		s.Set("dev", "K", "v", "")
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only %s, found %v", FileName, names)
	}
}

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(PathIn(root), pw); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	os.MkdirAll(nested, 0o755)

	got, err := FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(root)
	if got != want {
		t.Errorf("FindRoot = %s, want %s", got, want)
	}

	if _, err := FindRoot(t.TempDir()); !errors.Is(err, ErrNotInitialized) {
		t.Errorf("want ErrNotInitialized, got %v", err)
	}
}

// writeLegacyStore produces a file exactly as envy 0.x wrote it.
func writeLegacyStore(t *testing.T, path string, password []byte) {
	t.Helper()
	body := `{"version":"1.0.0","environments":{"development":{"OLD_KEY":{"value":"legacy-value","updated_at":"2026-04-07T12:00:00Z","created_at":"2026-04-07T12:00:00Z"}}},"password_hash":"ignored"}`
	salt := make([]byte, 32)
	rand.Read(salt)
	key, _ := pbkdf2.Key(sha256.New, string(password), salt, 100_000, 32)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	ct := gcm.Seal(nonce, nonce, []byte(body), nil)
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(append(salt, ct...))), 0o600)
}

func TestLegacyMigration(t *testing.T) {
	path := PathIn(t.TempDir())
	writeLegacyStore(t, path, pw)

	if _, err := Open(path, []byte("wrong")); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("legacy wrong password: %v", err)
	}

	s, err := Open(path, pw)
	if err != nil {
		t.Fatalf("Open legacy: %v", err)
	}
	if !s.Migrated() {
		t.Error("Migrated() should be true for a legacy store")
	}
	if sec, _ := s.Get("development", "OLD_KEY"); sec.Value != "legacy-value" {
		t.Errorf("legacy value = %q", sec.Value)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), formatV2) {
		t.Error("store was not rewritten in the current format")
	}
	again, err := Open(path, pw)
	if err != nil {
		t.Fatal(err)
	}
	if again.Migrated() {
		t.Error("re-opened store should not be marked migrated")
	}
}
