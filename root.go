package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

var logo = `
  ███████╗███╗   ██╗██╗   ██╗██╗   ██╗
  ██╔════╝████╗  ██║██║   ██║╚██╗ ██╔╝
  █████╗  ██╔██╗ ██║██║   ██║ ╚████╔╝ 
  ██╔══╝  ██║╚██╗██║╚██╗ ██╔╝  ╚██╔╝  
  ███████╗██║ ╚████║ ╚████╔╝    ██║   
  ╚══════╝╚═╝  ╚═══╝  ╚═══╝     ╚═╝   
`

var rootCmd = &cobra.Command{
	Use:   "envy",
	Short: "Smart .env manager for developers",
	Long:  formatLong(),
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func formatLong() string {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	return fmt.Sprintf(`%s
%s — Smart .env manager for developers

%s
  envy init              Initialize envy in your project
  envy set KEY value     Set and encrypt a secret
  envy get KEY           Get a secret value
  envy list              List all keys (values hidden)
  envy push <env>        Push secrets to an environment
  envy pull <env>        Pull secrets from an environment
  envy diff <e1> <e2>    Compare two environments
  envy audit             Scan for .env leaks in git history
  envy import            Import from existing .env file
  envy export            Export to plain .env file

%s
  Made with ♥ for developers who care about security`,
		cyan(logo),
		yellow("envy v"+version),
		dim("Commands:"),
		dim(""),
	)
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print envy version",
	Run: func(cmd *cobra.Command, args []string) {
		cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
		fmt.Printf("%s v%s\n", cyan("envy"), version)
		os.Exit(0)
	},
}




