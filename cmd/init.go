package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/store"
	"github.com/anastanveer653/envy/internal/ui"
)

const gitignoreEntry = store.Dir + "/"

func newInitCmd(a *app) *cobra.Command {
	var envs []string
	var noGitignore bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create an encrypted store in the current directory",
		Long: `Create .envy/store.enc in the current directory, protected by a new master
password, and add .envy/ to .gitignore.

Set ENVY_PASSWORD to initialise non-interactively.`,
		Example: `  envy init
  envy init --envs dev,prod`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			path := store.PathIn(cwd)
			if _, err := os.Stat(path); err == nil {
				return store.ErrExists
			}

			pw, err := a.newPassword(envPassword, "Create master password")
			if err != nil {
				return err
			}
			defer clear(pw)

			if _, err := store.Create(path, pw, envs...); err != nil {
				return err
			}
			ui.Success(a.stderr, "Created %s", filepath.Join(store.Dir, store.FileName))
			ui.Success(a.stderr, "Environments: %s", strings.Join(envs, ", "))

			if !noGitignore {
				added, err := ensureGitignored(filepath.Join(cwd, ".gitignore"))
				if err != nil {
					ui.Warn(a.stderr, "Could not update .gitignore: %v", err)
				} else if added {
					ui.Success(a.stderr, "Added %s to .gitignore", gitignoreEntry)
				}
			}
			fmt.Fprintf(a.stderr, "\n%s\n", ui.Dim("Next: envy set KEY   or   envy import .env"))
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&envs, "envs", []string{"development", "staging", "production"}, "environments to create")
	cmd.Flags().BoolVar(&noGitignore, "no-gitignore", false, "do not modify .gitignore")
	return cmd
}

// ensureGitignored appends the store directory to .gitignore unless an
// equivalent line is already present.
func ensureGitignored(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(content)))
	for sc.Scan() {
		switch strings.TrimSpace(sc.Text()) {
		case store.Dir, gitignoreEntry, "/" + store.Dir, "/" + gitignoreEntry:
			return false, nil
		}
	}

	var b strings.Builder
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		b.WriteString("\n")
	}
	if len(content) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("# envy encrypted secret store\n" + gitignoreEntry + "\n")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err == nil, err
}
