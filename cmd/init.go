package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/anastanveer653/envy/internal/env"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize envy in your project",
	Long:  "Set up encrypted secret management in the current directory",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	envyDir := filepath.Join(cwd, env.EnvyDir)
	storeFile := filepath.Join(envyDir, env.StoreFile)

	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	if _, err := os.Stat(storeFile); err == nil {
		fmt.Printf("%s envy is already initialized in this project\n", yellow("⚠"))
		return nil
	}

	fmt.Printf("\n%s Initializing envy in %s\n\n", cyan("⚡"), dim(cwd))

	password, err := env.PromptPassword("  Create master password: ")
	if err != nil {
		return err
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	confirm, err := env.PromptPassword("  Confirm master password: ")
	if err != nil {
		return err
	}

	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}

	// Create the store with default environments
	store, err := env.NewStore(cwd, password)
	if err != nil {
		return err
	}

	// Initialize with default environments
	defaultEnvs := []string{"development", "staging", "production"}
	for _, e := range defaultEnvs {
		if err := store.Set(e, "APP_NAME", "my-app", "Application name"); err != nil {
			return err
		}
	}

	// Add .envy to .gitignore
	gitignorePath := filepath.Join(cwd, ".gitignore")
	addToGitignore(gitignorePath)

	// Create .envy/.gitkeep so directory is tracked but store isn't
	if err := os.MkdirAll(envyDir, 0700); err != nil {
		return err
	}

	fmt.Printf("\n  %s Encrypted store created\n", green("✓"))
	fmt.Printf("  %s Environments: development, staging, production\n", green("✓"))
	fmt.Printf("  %s Added .envy/store.enc to .gitignore\n\n", green("✓"))
	fmt.Printf("  %s\n\n", dim("Run 'envy set KEY value' to add your first secret"))

	return nil
}

func addToGitignore(path string) {
	entry := "\n# envy secrets store\n.envy/store.enc\n"

	content, err := os.ReadFile(path)
	if err == nil {
		if contains(string(content), ".envy/store.enc") {
			return
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}




