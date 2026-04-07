package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Scan for .env leaks in git history",
	Long: `Scan your git history for accidentally committed .env files
or secrets that may have been exposed in your repository.`,
	RunE: runAudit,
}

// Common secret patterns to detect
var secretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"AWS Access Key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"AWS Secret Key", regexp.MustCompile(`(?i)aws.{0,20}secret.{0,20}['"=][0-9a-zA-Z/+]{40}`)},
	{"GitHub Token", regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`)},
	{"GitHub OAuth", regexp.MustCompile(`gho_[0-9a-zA-Z]{36}`)},
	{"Stripe Secret Key", regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24,}`)},
	{"Stripe Restricted Key", regexp.MustCompile(`rk_live_[0-9a-zA-Z]{24,}`)},
	{"SendGrid API Key", regexp.MustCompile(`SG\.[a-zA-Z0-9_-]{22}\.[a-zA-Z0-9_-]{43}`)},
	{"Twilio Account SID", regexp.MustCompile(`AC[a-z0-9]{32}`)},
	{"JWT Token", regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]+`)},
	{"Private Key", regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY`)},
	{"Generic Secret", regexp.MustCompile(`(?i)(password|passwd|secret|api_key|apikey|access_token)\s*[=:]\s*['"]?[^\s'"]{8,}`)},
	{"Generic .env Pattern", regexp.MustCompile(`(?m)^[A-Z_]+=.{8,}$`)},
}

type Finding struct {
	Commit  string
	File    string
	Line    string
	Pattern string
}

func runAudit(cmd *cobra.Command, args []string) error {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	fmt.Printf("\n  %s Scanning git history for secrets...\n\n", cyan("🔍"))

	// Check if git repo
	if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
		return fmt.Errorf("not a git repository")
	}

	findings := []Finding{}

	// 1. Check for .env files ever committed
	fmt.Printf("  %s Checking for committed .env files...\n", dim("→"))
	envFiles, _ := exec.Command("git", "log", "--all", "--full-history", "--", "*.env", ".env", ".env.*").Output()
	if len(envFiles) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(envFiles)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "commit ") {
				commitHash := strings.TrimPrefix(line, "commit ")
				if len(commitHash) > 8 {
					findings = append(findings, Finding{
						Commit:  commitHash[:8],
						File:    ".env file",
						Pattern: ".env file committed",
					})
				}
			}
		}
	}

	// 2. Scan recent commits for secret patterns
	fmt.Printf("  %s Scanning recent commits for secret patterns...\n", dim("→"))
	logOutput, err := exec.Command("git", "log", "--oneline", "-50").Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(logOutput)))
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 1 {
				continue
			}
			commitHash := parts[0]
			scanCommitForSecrets(commitHash, &findings)
		}
	}

	// 3. Check current working directory for .env files not in .gitignore
	fmt.Printf("  %s Checking working directory...\n", dim("→"))
	checkWorkingDir(&findings)

	// Print results
	fmt.Println()
	if len(findings) == 0 {
		fmt.Printf("  %s No secrets found in git history!\n\n", green("✓"))
		fmt.Printf("  %s\n\n", dim("Your repository looks clean."))
		return nil
	}

	fmt.Printf("  %s Found %d potential issue(s):\n\n", red("⚠"), len(findings))
	for i, f := range findings {
		fmt.Printf("  %s %s\n", yellow(fmt.Sprintf("[%d]", i+1)), red(f.Pattern))
		if f.Commit != "" {
			fmt.Printf("      %s %s\n", dim("commit:"), dim(f.Commit))
		}
		if f.File != "" {
			fmt.Printf("      %s %s\n", dim("file:"), dim(f.File))
		}
		if f.Line != "" {
			fmt.Printf("      %s %s\n", dim("match:"), dim(truncate(f.Line, 60)))
		}
		fmt.Println()
	}

	fmt.Printf("  %s\n", yellow("Recommendations:"))
	fmt.Printf("  %s Rotate any exposed secrets immediately\n", dim("•"))
	fmt.Printf("  %s Use 'git filter-repo' to remove secrets from history\n", dim("•"))
	fmt.Printf("  %s Add .env files to .gitignore\n", dim("•"))
	fmt.Printf("  %s Use envy to manage secrets securely going forward\n\n", dim("•"))

	return nil
}

func scanCommitForSecrets(commitHash string, findings *[]Finding) {
	diffOutput, err := exec.Command("git", "show", commitHash, "--unified=0").Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(diffOutput)))
	var currentFile string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}

		if !strings.HasPrefix(line, "+") {
			continue
		}

		addedLine := strings.TrimPrefix(line, "+")
		for _, p := range secretPatterns {
			if p.pattern.MatchString(addedLine) {
				*findings = append(*findings, Finding{
					Commit:  commitHash,
					File:    currentFile,
					Line:    addedLine,
					Pattern: p.name,
				})
				break
			}
		}
	}
}

func checkWorkingDir(findings *[]Finding) {
	// Check for .env files not in .gitignore
	envFiles := []string{".env", ".env.local", ".env.development", ".env.production", ".env.staging"}
	for _, f := range envFiles {
		if _, err := os.Stat(f); err == nil {
			// Check if it's gitignored
			out, _ := exec.Command("git", "check-ignore", "-q", f).CombinedOutput()
			if len(out) == 0 {
				*findings = append(*findings, Finding{
					File:    f,
					Pattern: f + " exists but may not be gitignored",
				})
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}




