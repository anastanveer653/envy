package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures are assembled at runtime so this file never trips a scanner,
// including envy's own audit of this repository.
var (
	fakeAWS     = "AKIA" + "Z7Q2MXK4P9R3T8WB"
	fakeAWS2    = "AKIA" + "Q8W3NYL5R0S4U9XC"
	fakeGitHub  = "ghp" + "_" + "a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8"
	fakeStripe  = "sk" + "_live_" + "4eC39HqLyjWDarjtT1zdp7dc9X"
	fakeGeneric = "Xk9#mQ2$vL7pR4wZ" + "tN8bF3yH6"
)

func scan(t *testing.T, patch string) []Finding {
	t.Helper()
	var got []Finding
	if err := ScanPatch(strings.NewReader(patch), DefaultRules, func(f Finding) { got = append(got, f) }); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestScanPatchFindsAndRedacts(t *testing.T) {
	patch := strings.Join([]string{
		"commit 1111111111111111111111111111111111111111",
		"diff --git a/config.yml b/config.yml",
		"--- /dev/null",
		"+++ b/config.yml",
		"@@ -0,0 +10,4 @@",
		"+aws_access_key_id: " + fakeAWS,
		"+harmless: true",
		"+github: " + fakeGitHub,
		"+-----BEGIN RSA " + "PRIVATE KEY-----",
		"commit 2222222222222222222222222222222222222222",
		"diff --git a/src/pay.js b/src/pay.js",
		"--- a/src/pay.js",
		"+++ b/src/pay.js",
		"@@ -5 +7,2 @@",
		"-const old = 1",
		"+const stripe = '" + fakeStripe + "'",
		"+const fine = '" + fakeStripe + "' // " + AllowMarker,
	}, "\n")

	got := scan(t, patch)
	want := []struct {
		rule, file, commit string
		line               int
	}{
		{"aws-access-key-id", "config.yml", "1111111111111111111111111111111111111111", 10},
		{"github-token", "config.yml", "1111111111111111111111111111111111111111", 12},
		{"private-key", "config.yml", "1111111111111111111111111111111111111111", 13},
		{"stripe-live-key", "src/pay.js", "2222222222222222222222222222222222222222", 7},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.RuleID != w.rule || g.File != w.file || g.Commit != w.commit || g.Line != w.line {
			t.Errorf("finding %d = %+v, want %+v", i, g, w)
		}
	}
	for _, f := range got {
		for _, secret := range []string{fakeAWS, fakeGitHub, fakeStripe} {
			if strings.Contains(f.Secret, secret) {
				t.Errorf("finding %s leaks the full secret", f.RuleID)
			}
		}
	}
}

func TestScanPatchSkipsLockfilesAndRemovals(t *testing.T) {
	patch := strings.Join([]string{
		"commit 3333333333333333333333333333333333333333",
		"+++ b/package-lock.json",
		"@@ -0,0 +1 @@",
		"+\"token\": \"" + fakeGitHub + "\"",
		"+++ /dev/null",
		"@@ -1 +0,0 @@",
		"-" + fakeAWS,
	}, "\n")
	if got := scan(t, patch); len(got) != 0 {
		t.Fatalf("expected no findings, got %+v", got)
	}
}

func TestGenericRuleUsesEntropyAndPlaceholders(t *testing.T) {
	cases := map[string]bool{
		"API_KEY=" + fakeGeneric:                    true,
		"password: \"" + fakeGeneric + "\"":         true,
		"API_KEY=aaaaaaaaaaaaaaaaaaaa":              false, // low entropy
		"API_KEY=your_api_key_goes_here_1234":       false, // placeholder
		"SECRET=${PROD_SECRET_FROM_VAULT}":          false, // reference, not a value
		"token = short":                             false, // too short
		"PasswordHash: crypto.HashPassword(pw)":     false, // code, not a value
		"apiToken = config.GetAccessTokenFromEnv()": false,
	}
	for line, want := range cases {
		patch := "commit 4444\n+++ b/app.env.sample\n@@ -0,0 +1 @@\n+" + line + "\n"
		got := len(scan(t, patch)) > 0
		if got != want {
			t.Errorf("%q: flagged=%v, want %v", line, got, want)
		}
	}
}

func TestIsEnvFile(t *testing.T) {
	yes := []string{".env", ".env.local", ".env.production", "config/.env", "prod.env"}
	no := []string{".env.example", ".env.sample", "config/.env.template", "envy.go", "environment.ts", ".envrc"}
	for _, f := range yes {
		if !isEnvFile(f) {
			t.Errorf("isEnvFile(%q) = false", f)
		}
	}
	for _, f := range no {
		if isEnvFile(f) {
			t.Errorf("isEnvFile(%q) = true", f)
		}
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("short"); got != "*****" {
		t.Errorf("Redact(short) = %q", got)
	}
	if got := Redact(fakeGitHub); got != "ghp_********" {
		t.Errorf("Redact(token) = %q", got)
	}
}

func TestEntropy(t *testing.T) {
	if e := entropy("aaaa"); e != 0 {
		t.Errorf("entropy(aaaa) = %v", e)
	}
	if e := entropy("abcd"); e != 2 {
		t.Errorf("entropy(abcd) = %v", e)
	}
}

func TestHunkStart(t *testing.T) {
	for header, want := range map[string]int{
		"@@ -0,0 +1 @@":         1,
		"@@ -5,2 +17,3 @@ func": 17,
		"@@ -1 +0,0 @@":         0,
		"garbage":               0,
	} {
		if got := hunkStart(header); got != want {
			t.Errorf("hunkStart(%q) = %d, want %d", header, got, want)
		}
	}
}

// TestGitRepository runs the full scan against a real throwaway repository.
func TestGitRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_CONFIG_GLOBAL="+filepath.Join(dir, "no-global"), "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	// Two distinct keys that redact to the same prefix must stay separate.
	write("app.go", "package main\nvar key = \""+fakeAWS+"\"\nvar key2 = \""+fakeAWS2+"\"\n")
	write(".env", "STRIPE="+fakeStripe+"\n")
	write(".env.example", "STRIPE=\n")
	run("add", ".")
	run("commit", "-q", "-m", "oops")
	run("rm", "-q", "--cached", ".env")
	write(".gitignore", ".env\n")
	write(".env.local", "X=1\n")
	// Re-adding the same key in a later commit is not a new finding.
	write("app.go", "package main\nvar key = \""+fakeAWS+"\"\nvar key2 = \""+fakeAWS2+"\"\nvar again = \""+fakeAWS+"\"\n")
	run("add", ".gitignore", "app.go")
	run("commit", "-q", "-m", "ignore env")

	ctx := context.Background()
	history, err := ScanHistory(ctx, Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]int{}
	for _, f := range history {
		rules[f.RuleID]++
	}
	if rules["aws-access-key-id"] != 2 || rules["stripe-live-key"] != 1 {
		t.Errorf("history findings = %+v", history)
	}

	write("deploy.yml", "token: "+fakeGitHub+"\n")
	write(".env.prod", "X=1\n")
	run("add", "deploy.yml")
	run("add", "-f", ".env.prod")
	staged, err := ScanStaged(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	stagedRules := map[string]bool{}
	for _, f := range staged {
		stagedRules[f.RuleID] = true
	}
	if len(staged) != 2 || !stagedRules["github-token"] || !stagedRules["staged-env-file"] {
		t.Errorf("staged findings = %+v", staged)
	}
	run("rm", "-q", "--cached", "deploy.yml", ".env.prod")
	os.Remove(filepath.Join(dir, "deploy.yml"))
	os.Remove(filepath.Join(dir, ".env.prod"))

	committed, err := CommittedEnvFiles(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(committed) != 1 || committed[0].File != ".env" {
		t.Errorf("committed env files = %+v", committed)
	}

	tree, err := WorkingTree(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].File != ".env.local" || tree[0].RuleID != "unignored-env-file" {
		t.Errorf("working tree findings = %+v", tree)
	}

	if _, err := ScanHistory(ctx, Options{Dir: t.TempDir()}); err == nil {
		t.Error("expected an error outside a git repository")
	}
}
