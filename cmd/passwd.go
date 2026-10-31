package cmd

import (
	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/ui"
)

func newPasswdCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "passwd",
		Short: "Change the master password",
		Long: `Re-encrypt the store under a new master password with a fresh salt.

Set ENVY_NEW_PASSWORD to change it non-interactively.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.unlock()
			if err != nil {
				return err
			}
			pw, err := a.newPassword("ENVY_NEW_PASSWORD", "New master password")
			if err != nil {
				return err
			}
			defer clear(pw)
			if err := s.ChangePassword(pw); err != nil {
				return err
			}
			ui.Success(a.stderr, "Master password changed")
			return nil
		},
	}
}
