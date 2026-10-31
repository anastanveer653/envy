// Package audit scans git history and the working tree for leaked secrets.
package audit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// AllowMarker on a line suppresses findings for that line.
const AllowMarker = "envy:allow"

// Finding is one potential leak. Secret is already redacted.
type Finding struct {
	RuleID      string `json:"rule"`
	Description string `json:"description"`
	Commit      string `json:"commit,omitempty"`
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Secret      string `json:"secret,omitempty"`
}

// Options controls a history scan.
type Options struct {
	Dir        string // repository directory
	MaxCommits int    // 0 scans every commit
	Rules      []Rule // defaults to DefaultRules
}

// skipFiles are generated files that routinely contain long random strings.
var skipFiles = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "Cargo.lock": true, "poetry.lock": true, "composer.lock": true,
}

// ScanPatch scans the output of `git log -p --unified=0 --format=commit %H`
// or `git diff` and calls report for every added line that matches a rule.
func ScanPatch(r io.Reader, rules []Rule, report func(Finding)) error {
	return scanPatch(r, rules, func(f Finding, _ string) { report(f) })
}

// scanPatch is ScanPatch that also passes the unredacted match, which is
// needed to de-duplicate findings without merging distinct secrets.
func scanPatch(r io.Reader, rules []Rule, report func(f Finding, raw string)) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var commit, file string
	var lineNo int
	skip := false

	for {
		raw, err := br.ReadString('\n')
		if len(raw) > 0 {
			line := strings.TrimRight(raw, "\r\n")
			switch {
			case strings.HasPrefix(line, "commit "):
				commit = strings.TrimSpace(strings.TrimPrefix(line, "commit "))
				file = ""
			case strings.HasPrefix(line, "+++ "):
				file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
				skip = file == "/dev/null" || skipFiles[path.Base(file)]
			case strings.HasPrefix(line, "@@ "):
				lineNo = hunkStart(line)
			case strings.HasPrefix(line, "+") && file != "" && !skip:
				added := line[1:]
				if !strings.Contains(added, AllowMarker) {
					for _, rule := range rules {
						if secret, ok := rule.match(added); ok {
							report(Finding{
								RuleID:      rule.ID,
								Description: rule.Description,
								Commit:      commit,
								File:        file,
								Line:        lineNo,
								Secret:      Redact(secret),
							}, secret)
							break
						}
					}
				}
				lineNo++
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// hunkStart extracts the new-file start line from "@@ -a,b +c,d @@".
func hunkStart(header string) int {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0
	}
	start, _, _ := strings.Cut(fields[2][1:], ",")
	n, _ := strconv.Atoi(start)
	return n
}

var patchFlags = []string{"--unified=0", "--no-color", "--no-ext-diff", "--no-textconv"}

// ScanHistory scans every commit reachable from any ref.
func ScanHistory(ctx context.Context, opts Options) ([]Finding, error) {
	args := append([]string{"log", "--all", "-p", "--format=commit %H"}, patchFlags...)
	if opts.MaxCommits > 0 {
		args = append(args, "-n", strconv.Itoa(opts.MaxCommits))
	}
	return scanGit(ctx, opts.Dir, opts.Rules, args)
}

// ScanStaged scans the changes staged for the next commit, which is what a
// pre-commit hook needs.
func ScanStaged(ctx context.Context, dir string) ([]Finding, error) {
	findings, err := scanGit(ctx, dir, nil, append([]string{"diff", "--cached"}, patchFlags...))
	if err != nil {
		return nil, err
	}
	names, err := git(ctx, dir, "diff", "--cached", "--name-only", "--diff-filter=AM")
	if err != nil {
		return nil, err
	}
	for _, f := range strings.Split(names, "\n") {
		if f != "" && isEnvFile(f) {
			findings = append(findings, Finding{RuleID: "staged-env-file", Description: "dotenv file is staged for commit", File: f})
		}
	}
	return findings, nil
}

// scanGit streams a git patch into the scanner. A secret that survives many
// commits is reported once per file.
func scanGit(ctx context.Context, dir string, rules []Rule, args []string) ([]Finding, error) {
	if err := ensureRepo(ctx, dir); err != nil {
		return nil, err
	}
	if rules == nil {
		rules = DefaultRules
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var findings []Finding
	seen := make(map[string]bool)
	scanErr := scanPatch(stdout, rules, func(f Finding, raw string) {
		k := f.RuleID + "\x00" + f.File + "\x00" + raw
		if !seen[k] {
			seen[k] = true
			findings = append(findings, f)
		}
	})
	if err := cmd.Wait(); err != nil {
		if strings.Contains(stderr.String(), "does not have any commits") {
			return nil, nil
		}
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return findings, scanErr
}

// isEnvFile reports whether name is a dotenv file that should not be
// committed. Templates such as .env.example are fine.
func isEnvFile(name string) bool {
	base := path.Base(name)
	if base != ".env" && !strings.HasPrefix(base, ".env.") && !strings.HasSuffix(base, ".env") {
		return false
	}
	for _, safe := range []string{"example", "sample", "template", "dist", "defaults"} {
		if strings.HasSuffix(base, "."+safe) {
			return false
		}
	}
	return true
}

// CommittedEnvFiles lists .env files that were ever added to history.
func CommittedEnvFiles(ctx context.Context, dir string) ([]Finding, error) {
	if err := ensureRepo(ctx, dir); err != nil {
		return nil, err
	}
	out, err := git(ctx, dir, "log", "--all", "--diff-filter=A", "--name-only", "--format=commit %H")
	if err != nil {
		return nil, err
	}
	var findings []Finding
	var commit string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "commit "):
			commit = strings.TrimPrefix(line, "commit ")
		case line != "" && isEnvFile(line):
			findings = append(findings, Finding{
				RuleID:      "committed-env-file",
				Description: "dotenv file committed to history",
				Commit:      commit,
				File:        line,
			})
		}
	}
	return findings, nil
}

// WorkingTree reports .env files in the working tree that are tracked by
// git or are untracked and not covered by .gitignore.
func WorkingTree(ctx context.Context, dir string) ([]Finding, error) {
	if err := ensureRepo(ctx, dir); err != nil {
		return nil, err
	}
	var findings []Finding
	tracked, err := git(ctx, dir, "ls-files")
	if err != nil {
		return nil, err
	}
	for _, f := range strings.Split(tracked, "\n") {
		if f != "" && isEnvFile(f) {
			findings = append(findings, Finding{RuleID: "tracked-env-file", Description: "dotenv file is tracked by git", File: f})
		}
	}
	untracked, err := git(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, f := range strings.Split(untracked, "\n") {
		if f != "" && isEnvFile(f) {
			findings = append(findings, Finding{RuleID: "unignored-env-file", Description: "dotenv file is not in .gitignore", File: f})
		}
	}
	return findings, nil
}

func ensureRepo(ctx context.Context, dir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git is not installed or not on PATH")
	}
	if _, err := git(ctx, dir, "rev-parse", "--git-dir"); err != nil {
		return errors.New("not a git repository")
	}
	return nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
