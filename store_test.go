package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSetGet(t *testing.T) {
	dir := t.TempDir()
	password := "test-password-123"

	store, err := NewStore(dir, password)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Set("development", "API_KEY", "sk-test123", "Test key"); err != nil {
		t.Fatal(err)
	}

	secret, err := store.Get("development", "API_KEY")
	if err != nil {
		t.Fatal(err)
	}

	if secret.Value != "sk-test123" {
		t.Errorf("Expected sk-test123, got %s", secret.Value)
	}
	if secret.Note != "Test key" {
		t.Errorf("Expected 'Test key', got %s", secret.Note)
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	password := "test-password-123"

	store1, err := NewStore(dir, password)
	if err != nil {
		t.Fatal(err)
	}
	store1.Set("development", "DB_URL", "postgres://localhost/test", "")

	// Reopen
	store2, err := NewStore(dir, password)
	if err != nil {
		t.Fatal(err)
	}

	secret, err := store2.Get("development", "DB_URL")
	if err != nil {
		t.Fatal(err)
	}
	if secret.Value != "postgres://localhost/test" {
		t.Errorf("Expected persisted value, got %s", secret.Value)
	}
}

func TestStoreWrongPassword(t *testing.T) {
	dir := t.TempDir()

	store, _ := NewStore(dir, "correct-password")
	store.Set("development", "KEY", "value", "")

	_, err := NewStore(dir, "wrong-password")
	if err == nil {
		t.Error("Expected error with wrong password")
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir, "password")

	store.Set("development", "B_KEY", "val2", "")
	store.Set("development", "A_KEY", "val1", "")
	store.Set("development", "C_KEY", "val3", "")

	keys, err := store.List("development")
	if err != nil {
		t.Fatal(err)
	}

	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Should be sorted
	expected := []string{"A_KEY", "B_KEY", "C_KEY"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Key %d: expected %s, got %s", i, expected[i], k)
		}
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir, "password")

	store.Set("development", "KEY", "value", "")
	store.Delete("development", "KEY")

	_, err := store.Get("development", "KEY")
	if err == nil {
		t.Error("Expected error after deleting key")
	}
}

func TestExportEnv(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir, "password")

	store.Set("development", "APP_NAME", "test-app", "App name")
	store.Set("development", "DB_URL", "postgres://localhost", "")

	content, err := store.ExportEnv("development")
	if err != nil {
		t.Fatal(err)
	}

	if len(content) == 0 {
		t.Error("Export should not be empty")
	}

	// Should contain the keys
	if !contains(content, "APP_NAME=test-app") {
		t.Error("Export should contain APP_NAME")
	}
	if !contains(content, "DB_URL=postgres://localhost") {
		t.Error("Export should contain DB_URL")
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcd", "****"},
		{"sk-test123", "sk******23"},
		{"ab", "**"},
	}

	for _, tt := range tests {
		got := MaskValue(tt.input)
		if got != tt.expected {
			t.Errorf("MaskValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFindProjectRoot(t *testing.T) {
	dir := t.TempDir()
	envyDir := filepath.Join(dir, EnvyDir)
	os.MkdirAll(envyDir, 0700)

	storeFile := filepath.Join(envyDir, StoreFile)
	os.WriteFile(storeFile, []byte("placeholder"), 0600)

	os.Chdir(dir)
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot failed: %v", err)
	}
	if root != dir {
		t.Errorf("Expected %s, got %s", dir, root)
	}
}




