package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/anastanveer653/envy/internal/env"
)

var diffCmd = &cobra.Command{
	Use:     "diff <env1> <env2>",
	Short:   "Compare secrets between two environments",
	Example: "  envy diff development production\n  envy diff staging production",
	Args:    cobra.ExactArgs(2),
	RunE:    runDiff,
}

var pushCmd = &cobra.Command{
	Use:     "push <env>",
	Short:   "Export secrets as .env file for an environment",
	Long:    "Write decrypted secrets to a .env.<environment> file",
	Example: "  envy push production\n  envy push staging",
	Args:    cobra.ExactArgs(1),
	RunE:    runPush,
}

var pullCmd = &cobra.Command{
	Use:     "pull <env>",
	Short:   "Import from a .env file into an environment",
	Long:    "Read a .env file and import all variables into the specified environment",
	Example: "  envy pull production --file .env.production",
	Args:    cobra.ExactArgs(1),
	RunE:    runPull,
}

var importCmd = &cobra.Command{
	Use:     "import",
	Short:   "Import from existing .env file",
	Long:    "Import all variables from a .env file into the current environment",
	Example: "  envy import\n  envy import --file .env.local --env staging",
	RunE:    runImport,
}

var exportCmd = &cobra.Command{
	Use:     "export",
	Short:   "Export secrets to a plain .env file",
	Example: "  envy export\n  envy export --env production --output .env.prod",
	RunE:    runExport,
}

var (
	fileFlag   string
	outputFlag string
)

func init() {
	pullCmd.Flags().StringVarP(&fileFlag, "file", "f", "", "Source .env file path")
	pullCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Target environment")
	importCmd.Flags().StringVarP(&fileFlag, "file", "f", ".env", "Source .env file")
	importCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Target environment")
	exportCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment to export")
	exportCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path")
}

func runDiff(cmd *cobra.Command, args []string) error {
	env1, env2 := args[0], args[1]

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

	secrets1, err := store.GetAll(env1)
	if err != nil {
		return err
	}
	secrets2, err := store.GetAll(env2)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	fmt.Printf("\n  %s vs %s\n\n", cyan(env1), cyan(env2))

	// Find all keys
	allKeys := make(map[string]bool)
	for k := range secrets1 {
		allKeys[k] = true
	}
	for k := range secrets2 {
		allKeys[k] = true
	}

	hasDiff := false
	for k := range allKeys {
		s1, ok1 := secrets1[k]
		s2, ok2 := secrets2[k]

		if !ok1 {
			fmt.Printf("  %s %s %s\n", green("+"), green(k), dim("(only in "+env2+")"))
			hasDiff = true
		} else if !ok2 {
			fmt.Printf("  %s %s %s\n", red("-"), red(k), dim("(only in "+env1+")"))
			hasDiff = true
		} else if s1.Value != s2.Value {
			fmt.Printf("  %s %s %s\n", yellow("~"), yellow(k), dim("(values differ)"))
			hasDiff = true
		}
	}

	if !hasDiff {
		fmt.Printf("  %s Both environments are in sync\n", green("✓"))
	}
	fmt.Println()
	return nil
}

func runPush(cmd *cobra.Command, args []string) error {
	targetEnv := args[0]

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

	content, err := store.ExportEnv(targetEnv)
	if err != nil {
		return err
	}

	outFile := fmt.Sprintf(".env.%s", targetEnv)
	if err := os.WriteFile(outFile, []byte(content), 0600); err != nil {
		return err
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	fmt.Printf("\n  %s Written to %s\n\n", green("✓"), dim(outFile))
	return nil
}

func runPull(cmd *cobra.Command, args []string) error {
	targetEnv := args[0]
	srcFile := fileFlag
	if srcFile == "" {
		srcFile = fmt.Sprintf(".env.%s", targetEnv)
	}
	return importFromFile(srcFile, targetEnv)
}

func runImport(cmd *cobra.Command, args []string) error {
	targetEnv := resolveEnv(envFlag)
	return importFromFile(fileFlag, targetEnv)
}

func importFromFile(filePath, targetEnv string) error {
	root, err := env.FindProjectRoot()
	if err != nil {
		return err
	}

	vars, err := godotenv.Read(filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	password, err := env.PromptPassword("  Master password: ")
	if err != nil {
		return err
	}

	store, err := env.NewStore(root, password)
	if err != nil {
		return err
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	fmt.Printf("\n  Importing from %s → %s\n\n", dim(filePath), cyan(targetEnv))

	count := 0
	for k, v := range vars {
		if err := store.Set(targetEnv, k, v, ""); err != nil {
			return err
		}
		fmt.Printf("  %s %s\n", green("✓"), k)
		count++
	}

	fmt.Printf("\n  %s imported %d secrets\n\n", green("Done!"), count)
	return nil
}

func runExport(cmd *cobra.Command, args []string) error {
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

	content, err := store.ExportEnv(targetEnv)
	if err != nil {
		return err
	}

	outFile := outputFlag
	if outFile == "" {
		outFile = fmt.Sprintf(".env.%s", targetEnv)
	}

	// Safety check: warn if outputting to .env
	if outFile == ".env" || strings.HasSuffix(outFile, "/.env") {
		confirm, _ := env.PromptInput("  ⚠ Writing to .env — make sure it's in .gitignore. Continue? [y/N] ")
		if strings.ToLower(confirm) != "y" {
			fmt.Println("  Aborted.")
			return nil
		}
	}

	if err := os.WriteFile(outFile, []byte(content), 0600); err != nil {
		return err
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	fmt.Printf("\n  %s Exported %s to %s\n\n", green("✓"), targetEnv, dim(outFile))
	return nil
}




