package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/anastanveer653/envy/internal/env"
)

var (
	envFlag  string
	noteFlag string
)

var setCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set an encrypted secret",
	Long:  "Store an encrypted secret in your envy store",
	Example: `  envy set DATABASE_URL postgres://localhost/mydb
  envy set API_KEY sk-abc123 --env production
  envy set JWT_SECRET mysecret --note "JWT signing key"`,
	Args: cobra.ExactArgs(2),
	RunE: runSet,
}

var getCmd = &cobra.Command{
	Use:     "get KEY",
	Short:   "Get a secret value",
	Example: "  envy get DATABASE_URL\n  envy get API_KEY --env production",
	Args:    cobra.ExactArgs(1),
	RunE:    runGet,
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all secrets (values hidden)",
	Example: "  envy list\n  envy list --env production",
	RunE:    runList,
}

var deleteCmd = &cobra.Command{
	Use:     "delete KEY",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a secret",
	Example: "  envy delete OLD_API_KEY\n  envy delete SECRET --env staging",
	Args:    cobra.ExactArgs(1),
	RunE:    runDelete,
}

func init() {
	setCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment (default: current env)")
	setCmd.Flags().StringVarP(&noteFlag, "note", "n", "", "Optional note/description for this secret")
	getCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment")
	listCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment")
	deleteCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment")
	rootCmd.AddCommand(deleteCmd)
}

func runSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	targetEnv := resolveEnv(envFlag)

	root, err := env.FindProjectRoot()
	if err != nil {
		return err
	}

	password, err := env.PromptPassword("  Master password: ")
	if err != nil {
		return err
	}

	store, err := env.NewStore(root, password)
	if err != nil {
		return err
	}

	if err := store.Set(targetEnv, key, value, noteFlag); err != nil {
		return err
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	fmt.Printf("\n  %s %s %s %s\n\n",
		green("✓"),
		cyan(key),
		dim("→"),
		dim("[encrypted] in "+targetEnv),
	)
	return nil
}

func runGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	targetEnv := resolveEnv(envFlag)

	root, err := env.FindProjectRoot()
	if err != nil {
		return err
	}

	password, err := env.PromptPassword("  Master password: ")
	if err != nil {
		return err
	}

	store, err := env.NewStore(root, password)
	if err != nil {
		return err
	}

	secret, err := store.Get(targetEnv, key)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n  %s = %s\n", cyan(key), yellow(secret.Value))
	if secret.Note != "" {
		fmt.Printf("  %s\n", dim("note: "+secret.Note))
	}
	fmt.Printf("  %s\n\n", dim("updated: "+secret.UpdatedAt.Format("Jan 02, 2006 15:04")))
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	targetEnv := resolveEnv(envFlag)

	root, err := env.FindProjectRoot()
	if err != nil {
		return err
	}

	password, err := env.PromptPassword("  Master password: ")
	if err != nil {
		return err
	}

	store, err := env.NewStore(root, password)
	if err != nil {
		return err
	}

	keys, err := store.List(targetEnv)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n  %s %s %s\n\n",
		cyan("Environment:"),
		yellow(targetEnv),
		dim(fmt.Sprintf("(%d secrets)", len(keys))),
	)

	for _, k := range keys {
		secret, _ := store.Get(targetEnv, k)
		masked := env.MaskValue(secret.Value)
		note := ""
		if secret.Note != "" {
			note = dim(" # "+secret.Note)
		}
		fmt.Printf("  %s = %s%s\n", cyan(k), dim(masked), note)
	}
	fmt.Println()
	return nil
}

func runDelete(cmd *cobra.Command, args []string) error {
	key := args[0]
	targetEnv := resolveEnv(envFlag)

	root, err := env.FindProjectRoot()
	if err != nil {
		return err
	}

	password, err := env.PromptPassword("  Master password: ")
	if err != nil {
		return err
	}

	store, err := env.NewStore(root, password)
	if err != nil {
		return err
	}

	if err := store.Delete(targetEnv, key); err != nil {
		return err
	}

	red := color.New(color.FgRed).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	fmt.Printf("\n  %s %s %s\n\n", red("✗"), key, dim("deleted from "+targetEnv))
	return nil
}

func resolveEnv(flag string) string {
	if flag != "" {
		return flag
	}
	return env.GetCurrentEnv()
}




