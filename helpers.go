package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// FindProjectRoot walks up from cwd looking for .envy directory
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, EnvyDir)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no envy project found — run 'envy init' first")
}

// PromptPassword prompts for a password without echoing
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(password)), nil
}

// PromptInput prompts for visible text input
func PromptInput(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// IsInitialized checks if envy is set up in the given directory
func IsInitialized(root string) bool {
	_, err := os.Stat(filepath.Join(root, EnvyDir, StoreFile))
	return err == nil
}

// MaskValue masks a secret value for display
func MaskValue(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

// GetCurrentEnv returns the current environment from ENVY_ENV or default
func GetCurrentEnv() string {
	if env := os.Getenv("ENVY_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("NODE_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("APP_ENV"); env != "" {
		return env
	}
	return DefaultEnv
}




