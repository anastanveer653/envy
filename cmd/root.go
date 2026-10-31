// Package cmd implements the envy command-line interface.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/store"
	"github.com/anastanveer653/envy/internal/ui"
)

const (
	envPassword = "ENVY_PASSWORD"
	envTarget   = "ENVY_ENV"
	minPassword = 8
)

// BuildInfo is stamped into the binary at release time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// app carries the process environment so commands can be tested without
// touching the real terminal.
type app struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	getenv func(string) string
	prompt func(prompt string) ([]byte, error)
	isTTY  func() bool // whether stdin is interactive

	env     string
	noColor bool
}

// exitError ends the process with code. A nil err exits silently, which
// commands use when they have already reported the problem.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit status %d", e.code)
	}
	return e.err.Error()
}

// Execute runs envy and returns the process exit code.
func Execute(info BuildInfo) int {
	a := &app{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		getenv: os.Getenv,
		prompt: ui.ReadSecret,
		isTTY:  func() bool { return ui.IsTerminal(os.Stdin) },
	}
	return a.run(newRootCmd(a, info), os.Args[1:])
}

func (a *app) run(root *cobra.Command, args []string) int {
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			ui.Error(a.stderr, "%v", ee.err)
		}
		return ee.code
	}
	ui.Error(a.stderr, "%v", err)
	return 1
}

func newRootCmd(a *app, info BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:   "envy",
		Short: "Encrypted, multi-environment secrets for your projects",
		Long: ui.Cyan("envy") + ` keeps your project's secrets in one encrypted file instead of a
pile of plaintext .env files. Secrets are grouped by environment, unlocked
with a master password, and injected into processes with 'envy run' so they
never have to touch the disk.`,
		Example: `  envy init
  envy set DATABASE_URL            # value is read from a hidden prompt
  envy set API_KEY sk-123 -e production
  envy run -- npm start
  envy diff development production
  envy audit`,
		Version:       info.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if a.noColor {
				ui.DisableColor()
			}
		},
	}
	root.SetIn(a.stdin)
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)
	root.SetVersionTemplate("envy {{.Version}}\n")

	pf := root.PersistentFlags()
	pf.StringVarP(&a.env, "env", "e", "", "environment to use (default $ENVY_ENV or \"development\")")
	pf.BoolVar(&a.noColor, "no-color", false, "disable colored output (also honors $NO_COLOR)")

	root.AddGroup(
		&cobra.Group{ID: "secrets", Title: "Secrets:"},
		&cobra.Group{ID: "envs", Title: "Environments:"},
		&cobra.Group{ID: "files", Title: "Files and processes:"},
		&cobra.Group{ID: "security", Title: "Security:"},
	)
	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	add("secrets", newSetCmd(a), newGetCmd(a), newListCmd(a), newDeleteCmd(a))
	add("envs", newEnvsCmd(a), newDiffCmd(a))
	add("files", newRunCmd(a), newImportCmd(a), newExportCmd(a))
	add("security", newInitCmd(a), newPasswdCmd(a), newAuditCmd(a))
	root.AddCommand(newVersionCmd(a, info))
	return root
}

// targetEnv resolves --env, then $ENVY_ENV, then "development".
func (a *app) targetEnv() (string, error) {
	name := a.env
	if name == "" {
		name = a.getenv(envTarget)
	}
	if name == "" {
		name = "development"
	}
	return name, store.ValidateEnv(name)
}

// password returns the master password from $ENVY_PASSWORD or a prompt.
func (a *app) password(prompt string) ([]byte, error) {
	if pw := a.getenv(envPassword); pw != "" {
		return []byte(pw), nil
	}
	return a.prompt(prompt)
}

// newPassword asks for a password twice, or takes it from envVar.
func (a *app) newPassword(envVar, label string) ([]byte, error) {
	pw := []byte(a.getenv(envVar))
	if len(pw) == 0 {
		var err error
		if pw, err = a.prompt(label + ": "); err != nil {
			return nil, err
		}
		confirm, err := a.prompt("Confirm " + strings.ToLower(label) + ": ")
		if err != nil {
			return nil, err
		}
		if string(pw) != string(confirm) {
			return nil, errors.New("passwords do not match")
		}
	}
	if len([]rune(string(pw))) < minPassword {
		return nil, fmt.Errorf("password must be at least %d characters", minPassword)
	}
	return pw, nil
}

// unlock finds the project store and opens it, upgrading old formats.
func (a *app) unlock() (*store.Store, error) {
	root, err := store.FindRoot(".")
	if err != nil {
		return nil, err
	}
	pw, err := a.password("Master password: ")
	if err != nil {
		return nil, err
	}
	defer clear(pw)

	s, err := store.Open(store.PathIn(root), pw)
	if err != nil {
		return nil, err
	}
	if s.Migrated() {
		if err := s.Save(); err != nil {
			return nil, fmt.Errorf("upgrade store format: %w", err)
		}
		ui.Warn(a.stderr, "Upgraded %s to the envy/v2 format (Argon2id)", store.Dir+"/"+store.FileName)
	}
	return s, nil
}

// requireEnv returns a helpful error if env does not exist.
func requireEnv(s *store.Store, env string) error {
	if s.HasEnvironment(env) {
		return nil
	}
	return fmt.Errorf("environment %q does not exist (have: %s); create it with 'envy envs add %s'",
		env, strings.Join(s.Environments(), ", "), env)
}
