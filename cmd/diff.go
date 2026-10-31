package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/ui"
)

func newDiffCmd(a *app) *cobra.Command {
	var exitCode bool
	cmd := &cobra.Command{
		Use:   "diff ENV_A ENV_B",
		Short: "Compare the keys of two environments",
		Long: `Show keys that exist in only one environment, or whose values differ.
Values themselves are never printed.

With --exit-code, envy exits with status 1 when the environments differ,
which makes it easy to fail a CI job when production is missing a key.`,
		Example: "  envy diff development production\n  envy diff staging production --exit-code",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			envA, envB := args[0], args[1]
			s, err := a.unlock()
			if err != nil {
				return err
			}
			for _, e := range args {
				if err := requireEnv(s, e); err != nil {
					return err
				}
			}
			secA, _ := s.Secrets(envA)
			secB, _ := s.Secrets(envB)

			keys := make([]string, 0, len(secA)+len(secB))
			for k := range secA {
				keys = append(keys, k)
			}
			for k := range secB {
				if _, ok := secA[k]; !ok {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)

			var onlyA, onlyB, changed int
			for _, k := range keys {
				a1, inA := secA[k]
				b1, inB := secB[k]
				switch {
				case !inB:
					onlyA++
					fmt.Fprintf(a.stdout, "%s %s %s\n", ui.Red("-"), k, ui.Dim("only in "+envA))
				case !inA:
					onlyB++
					fmt.Fprintf(a.stdout, "%s %s %s\n", ui.Green("+"), k, ui.Dim("only in "+envB))
				case a1.Value != b1.Value:
					changed++
				}
			}
			if onlyA+onlyB == 0 {
				ui.Success(a.stdout, "%s and %s have the same keys", envA, envB)
			} else {
				fmt.Fprintf(a.stdout, "\n%s only in %s, %s only in %s\n",
					plural(onlyA, "key"), envA, plural(onlyB, "key"), envB)
			}
			fmt.Fprintln(a.stdout, ui.Dim(fmt.Sprintf("%s with different values", plural(changed, "shared key"))))

			if exitCode && onlyA+onlyB > 0 {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "exit with status 1 if the key sets differ")
	return cmd
}
