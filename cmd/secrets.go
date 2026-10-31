package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/ui"
)

func newSetCmd(a *app) *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "set KEY [VALUE]",
		Short: "Add or update a secret",
		Long: `Add or update a secret in the selected environment.

If VALUE is omitted it is read from a hidden prompt, or from stdin when stdin
is not a terminal. Prefer this over passing VALUE as an argument, which
leaves the secret in your shell history and the process list.`,
		Example: `  envy set DATABASE_URL
  envy set STRIPE_KEY -e production --note "live key, rotate quarterly"
  envy set TLS_KEY < server.key
  envy set LOG_LEVEL debug`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			env, err := a.targetEnv()
			if err != nil {
				return err
			}

			var value string
			if len(args) == 2 {
				value = args[1]
			} else if value, err = a.readValue(key); err != nil {
				return err
			}

			s, err := a.unlock()
			if err != nil {
				return err
			}
			if err := requireEnv(s, env); err != nil {
				return err
			}
			updated, err := s.Set(env, key, value, note)
			if err != nil {
				return err
			}
			if err := s.Save(); err != nil {
				return err
			}
			verb := "Added"
			if updated {
				verb = "Updated"
			}
			ui.Success(a.stderr, "%s %s in %s", verb, ui.Cyan(key), ui.Bold(env))
			return nil
		},
	}
	cmd.Flags().StringVarP(&note, "note", "n", "", "description stored alongside the secret")
	return cmd
}

// readValue reads a secret value from a hidden prompt or from piped stdin.
func (a *app) readValue(key string) (string, error) {
	if a.isTTY() {
		v, err := a.prompt(fmt.Sprintf("Value for %s: ", key))
		return string(v), err
	}
	data, err := io.ReadAll(io.LimitReader(a.stdin, 1<<20))
	if err != nil {
		return "", err
	}
	// Drop the single trailing newline that echo and most editors add.
	v := strings.TrimSuffix(string(data), "\n")
	return strings.TrimSuffix(v, "\r"), nil
}

func newGetCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Print a secret's value",
		Long:  "Print a secret's value to stdout with no decoration, so it can be captured by scripts.",
		Example: `  envy get DATABASE_URL
  psql "$(envy get DATABASE_URL -e staging)"`,
		Args: cobra.ExactArgs(1),
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
			sec, err := s.Get(env, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(a.stdout, sec.Value)
			return nil
		},
	}
}

type listedSecret struct {
	Key       string    `json:"key"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newListCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List secret names (values are never shown)",
		Example: "  envy list\n  envy ls -e production --json",
		Args:    cobra.NoArgs,
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
			keys, _ := s.Keys(env)

			if asJSON {
				out := make([]listedSecret, 0, len(keys))
				for _, k := range keys {
					sec := secrets[k]
					out = append(out, listedSecret{Key: k, Note: sec.Note, CreatedAt: sec.CreatedAt, UpdatedAt: sec.UpdatedAt})
				}
				enc := json.NewEncoder(a.stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(a.stdout, "%s %s\n\n", ui.Bold(env), ui.Dim(plural(len(keys), "secret")))
			if len(keys) == 0 {
				fmt.Fprintln(a.stdout, ui.Dim("  (empty) add one with: envy set KEY"))
				return nil
			}
			tw := tabwriter.NewWriter(a.stdout, 0, 0, 3, ' ', 0)
			for _, k := range keys {
				sec := secrets[k]
				fmt.Fprintf(tw, "  %s\t%s\t%s\n", ui.Cyan(k), ui.Dim("updated "+sec.UpdatedAt.Local().Format("2006-01-02")), sec.Note)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return cmd
}

func newDeleteCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "delete KEY...",
		Aliases: []string{"rm"},
		Short:   "Delete one or more secrets",
		Example: "  envy delete OLD_API_KEY\n  envy rm A B C -e staging",
		Args:    cobra.MinimumNArgs(1),
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
			for _, key := range args {
				if err := s.Delete(env, key); err != nil {
					return err
				}
			}
			if err := s.Save(); err != nil {
				return err
			}
			for _, key := range args {
				ui.Success(a.stderr, "Deleted %s from %s", ui.Cyan(key), ui.Bold(env))
			}
			return nil
		},
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
