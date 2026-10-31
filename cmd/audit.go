package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anastanveer653/envy/internal/audit"
	"github.com/anastanveer653/envy/internal/ui"
)

func newAuditCmd(a *app) *cobra.Command {
	var (
		maxCommits  int
		skipHistory bool
		asJSON      bool
		exitZero    bool
		staged      bool
	)
	cmd := &cobra.Command{
		Use:   "audit [PATH]",
		Short: "Scan a git repository for leaked secrets",
		Long: `Scan a git repository (default: the current directory) for:

  • credentials in any commit on any branch (AWS, GitHub, Stripe, OpenAI,
    Anthropic, Slack, private keys, connection strings and more)
  • .env files that were ever committed
  • .env files in the working tree that git does not ignore

Matches are redacted in the output. Add "envy:allow" to a line to silence
a known false positive. Exits with status 1 when anything is found, so it
can gate CI or a pre-commit hook.

--staged scans only the changes staged for the next commit, which is fast
enough for a pre-commit hook.`,
		Example: `  envy audit
  envy audit --staged
  envy audit --max-commits 200
  envy audit --json > findings.json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			ctx := cmd.Context()

			var findings []audit.Finding
			if staged {
				found, err := audit.ScanStaged(ctx, dir)
				if err != nil {
					return err
				}
				findings = found
			} else {
				tree, err := audit.WorkingTree(ctx, dir)
				if err != nil {
					return err
				}
				findings = append(findings, tree...)

				committed, err := audit.CommittedEnvFiles(ctx, dir)
				if err != nil {
					return err
				}
				findings = append(findings, committed...)
			}

			if !skipHistory && !staged {
				if !asJSON {
					fmt.Fprintln(a.stderr, ui.Dim("Scanning git history..."))
				}
				history, err := audit.ScanHistory(ctx, audit.Options{Dir: dir, MaxCommits: maxCommits})
				if err != nil {
					return err
				}
				findings = append(findings, history...)
			}

			if asJSON {
				if findings == nil {
					findings = []audit.Finding{}
				}
				enc := json.NewEncoder(a.stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(findings); err != nil {
					return err
				}
			} else {
				printFindings(a, findings)
			}

			if len(findings) > 0 && !exitZero {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&maxCommits, "max-commits", 0, "only scan the N most recent commits (0 = all)")
	f.BoolVar(&skipHistory, "skip-history", false, "only check .env files, not commit contents")
	f.BoolVar(&asJSON, "json", false, "output findings as JSON")
	f.BoolVar(&exitZero, "exit-zero", false, "exit 0 even when findings are reported")
	f.BoolVar(&staged, "staged", false, "scan only changes staged for commit (for pre-commit hooks)")
	return cmd
}

func printFindings(a *app, findings []audit.Finding) {
	if len(findings) == 0 {
		ui.Success(a.stdout, "No leaked secrets found")
		return
	}
	for _, f := range findings {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(a.stdout, "%s %s  %s\n", ui.Red("●"), ui.Bold(f.Description), ui.Dim("["+f.RuleID+"]"))
		fmt.Fprintf(a.stdout, "    %s\n", loc)
		if f.Commit != "" {
			fmt.Fprintf(a.stdout, "    %s %s\n", ui.Dim("commit"), shortHash(f.Commit))
		}
		if f.Secret != "" {
			fmt.Fprintf(a.stdout, "    %s %s\n", ui.Dim("match "), ui.Yellow(f.Secret))
		}
	}
	fmt.Fprintf(a.stdout, "\n%s %s\n\n", ui.Red("✗"), ui.Bold(fmt.Sprintf("%s found", plural(len(findings), "issue"))))
	fmt.Fprintln(a.stdout, "Next steps:")
	fmt.Fprintln(a.stdout, "  1. Rotate every exposed credential. Removing it from history is not enough.")
	fmt.Fprintln(a.stdout, "  2. Purge history with git filter-repo, then force-push and ask collaborators to re-clone.")
	fmt.Fprintln(a.stdout, "  3. Move the values into envy:  envy import .env && rm .env")
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
