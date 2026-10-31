package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(a *app, info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(a.stdout, "envy %s\n", info.Version)
			if info.Commit != "" {
				fmt.Fprintf(a.stdout, "  commit: %s\n", info.Commit)
			}
			if info.Date != "" {
				fmt.Fprintf(a.stdout, "  built:  %s\n", info.Date)
			}
			fmt.Fprintf(a.stdout, "  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
