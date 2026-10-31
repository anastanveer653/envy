package cmd

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
)

func newRunCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [flags] -- COMMAND [ARGS...]",
		Short: "Run a command with secrets injected as environment variables",
		Long: `Run COMMAND with the selected environment's secrets added to its
environment. Secrets override variables of the same name that are already
set. Nothing is written to disk, and ENVY_PASSWORD is removed from the
child's environment.

envy exits with the command's exit status.`,
		Example: `  envy run -- npm start
  envy run -e production -- ./migrate up
  envy run -- sh -c 'echo $DATABASE_URL'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.targetEnv()
			if err != nil {
				return err
			}
			s, err := a.unlock()
			if err != nil {
				return err
			}
			if err := requireEnv(s, env); err != nil {
				return err
			}
			secrets, _ := s.Secrets(env)

			path, err := exec.LookPath(args[0])
			if err != nil {
				return err
			}
			child := exec.Command(path, args[1:]...)
			child.Stdin = a.stdin
			child.Stdout = a.stdout
			child.Stderr = a.stderr

			vars := make(map[string]string, len(secrets))
			for k, sec := range secrets {
				vars[k] = sec.Value
			}
			child.Env = mergeEnv(os.Environ(), vars)

			// The terminal delivers Ctrl+C to the whole process group; let the
			// child decide how to handle it and report its own exit status.
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt)
			defer signal.Stop(sig)

			err = child.Run()
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return &exitError{code: ee.ExitCode()}
			}
			return err
		},
	}
	// Everything after the command name belongs to the command.
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// mergeEnv overlays vars onto base and strips ENVY_PASSWORD.
func mergeEnv(base []string, vars map[string]string) []string {
	out := make([]string, 0, len(base)+len(vars))
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if _, overridden := vars[name]; overridden || strings.EqualFold(name, envPassword) {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range vars {
		out = append(out, k+"="+v)
	}
	return out
}
