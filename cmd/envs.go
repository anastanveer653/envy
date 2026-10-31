package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/store"
	"github.com/anastanveer653/envy/internal/ui"
)

func newEnvsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envs",
		Short: "List, add or remove environments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.unlock()
			if err != nil {
				return err
			}
			current, _ := a.targetEnv()
			tw := tabwriter.NewWriter(a.stdout, 0, 0, 3, ' ', 0)
			for _, name := range s.Environments() {
				keys, _ := s.Keys(name)
				marker := " "
				if name == current {
					marker = ui.Green("*")
				}
				fmt.Fprintf(tw, "%s %s\t%s\n", marker, name, ui.Dim(plural(len(keys), "secret")))
			}
			return tw.Flush()
		},
	}
	cmd.AddCommand(newEnvsAddCmd(a), newEnvsRemoveCmd(a))
	return cmd
}

func newEnvsAddCmd(a *app) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:     "add NAME...",
		Short:   "Create environments",
		Example: "  envy envs add preview\n  envy envs add qa --from staging",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.unlock()
			if err != nil {
				return err
			}
			var source map[string]store.Secret
			if from != "" {
				if err := requireEnv(s, from); err != nil {
					return err
				}
				source, _ = s.Secrets(from)
			}
			for _, name := range args {
				if s.HasEnvironment(name) {
					return fmt.Errorf("environment %q already exists", name)
				}
				if err := s.AddEnvironment(name); err != nil {
					return err
				}
				for k, sec := range source {
					if _, err := s.Set(name, k, sec.Value, sec.Note); err != nil {
						return err
					}
				}
			}
			if err := s.Save(); err != nil {
				return err
			}
			for _, name := range args {
				if from != "" {
					ui.Success(a.stderr, "Created %s with %s copied from %s", ui.Bold(name), plural(len(source), "secret"), from)
				} else {
					ui.Success(a.stderr, "Created %s", ui.Bold(name))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "copy every secret from an existing environment")
	return cmd
}

func newEnvsRemoveCmd(a *app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "remove NAME",
		Aliases: []string{"rm"},
		Short:   "Delete an environment and its secrets",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := a.unlock()
			if err != nil {
				return err
			}
			if err := requireEnv(s, name); err != nil {
				return err
			}
			if keys, _ := s.Keys(name); len(keys) > 0 && !force {
				return fmt.Errorf("environment %q has %s; pass --force to delete it", name, plural(len(keys), "secret"))
			}
			if err := s.RemoveEnvironment(name); err != nil {
				return err
			}
			if err := s.Save(); err != nil {
				return err
			}
			ui.Success(a.stderr, "Removed %s", ui.Bold(name))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "delete even if the environment has secrets")
	return cmd
}
