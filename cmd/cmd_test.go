package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/anastanveer653/envy/internal/crypto"
	"github.com/anastanveer653/envy/internal/dotenv"
	"github.com/anastanveer653/envy/internal/store"
	"github.com/anastanveer653/envy/internal/ui"
)

const testPassword = "correct-horse-battery"

func TestMain(m *testing.M) {
	// Acts as the child process for TestRun.
	if os.Getenv("ENVY_TEST_HELPER") == "1" {
		helperProcess()
		return
	}
	store.KDFParams = crypto.Params{Time: 1, Memory: 8 * 1024, Threads: 1}
	color.NoColor = true
	os.Exit(m.Run())
}

type harness struct {
	t   *testing.T
	dir string
	env map[string]string
}

type result struct {
	stdout, stderr string
	code           int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return &harness{t: t, dir: dir, env: map[string]string{envPassword: testPassword}}
}

func (h *harness) run(stdin string, args ...string) result {
	h.t.Helper()
	var out, errOut bytes.Buffer
	a := &app{
		stdin:  strings.NewReader(stdin),
		stdout: &out,
		stderr: &errOut,
		getenv: func(k string) string { return h.env[k] },
		prompt: func(string) ([]byte, error) { return nil, ui.ErrNoTTY },
		isTTY:  func() bool { return false },
	}
	code := a.run(newRootCmd(a, BuildInfo{Version: "test"}), args)
	return result{out.String(), errOut.String(), code}
}

// ok runs a command and fails the test if it does not exit 0.
func (h *harness) ok(stdin string, args ...string) result {
	h.t.Helper()
	r := h.run(stdin, args...)
	if r.code != 0 {
		h.t.Fatalf("envy %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.code, r.stdout, r.stderr)
	}
	return r
}

func (h *harness) fails(wantInStderr string, args ...string) result {
	h.t.Helper()
	r := h.run("", args...)
	if r.code == 0 {
		h.t.Fatalf("envy %s: expected failure, got exit 0\nstdout: %s", strings.Join(args, " "), r.stdout)
	}
	if !strings.Contains(r.stderr, wantInStderr) {
		h.t.Fatalf("envy %s: stderr %q does not contain %q", strings.Join(args, " "), r.stderr, wantInStderr)
	}
	return r
}

func TestWorkflow(t *testing.T) {
	h := newHarness(t)
	h.ok("", "init")

	gi, _ := os.ReadFile(filepath.Join(h.dir, ".gitignore"))
	if !strings.Contains(string(gi), ".envy/") {
		t.Errorf(".gitignore = %q", gi)
	}
	h.fails("already exists", "init")

	// Values from an argument and from stdin, including a multi-line PEM.
	h.ok("", "set", "DATABASE_URL", "postgres://u:p@localhost/db", "--note", "primary db")
	pem := "-----BEGIN KEY-----\nabc\ndef\n-----END KEY-----"
	h.ok(pem+"\n", "set", "TLS_KEY")

	if got := h.ok("", "get", "DATABASE_URL").stdout; got != "postgres://u:p@localhost/db\n" {
		t.Errorf("get DATABASE_URL = %q", got)
	}
	if got := h.ok("", "get", "TLS_KEY").stdout; got != pem+"\n" {
		t.Errorf("get TLS_KEY = %q", got)
	}

	list := h.ok("", "list").stdout
	if !strings.Contains(list, "DATABASE_URL") || !strings.Contains(list, "primary db") {
		t.Errorf("list output missing keys:\n%s", list)
	}
	if strings.Contains(list, "postgres://") || strings.Contains(list, "abc") {
		t.Errorf("list leaked a value:\n%s", list)
	}

	var listed []listedSecret
	if err := json.Unmarshal([]byte(h.ok("", "list", "--json").stdout), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Key != "DATABASE_URL" || listed[1].Key != "TLS_KEY" {
		t.Errorf("list --json = %+v", listed)
	}

	// Export round-trips through a .env parser.
	exported := h.ok("", "export").stdout
	parsed, err := dotenv.Parse(strings.NewReader(exported))
	if err != nil {
		t.Fatalf("parse export: %v\n%s", err, exported)
	}
	if parsed["TLS_KEY"] != pem || parsed["DATABASE_URL"] != "postgres://u:p@localhost/db" {
		t.Errorf("export round trip = %+v", parsed)
	}

	h.ok("", "export", "-o", "out.env")
	h.fails("already exists", "export", "-o", "out.env")
	h.ok("", "export", "-o", "out.env", "--force")

	tmpl := h.ok("", "export", "--template").stdout
	if strings.Contains(tmpl, "postgres://") || !strings.Contains(tmpl, "DATABASE_URL=\n") {
		t.Errorf("template export:\n%s", tmpl)
	}

	// Import into staging, then compare.
	os.WriteFile("staging.env", []byte("DATABASE_URL=postgres://staging\nREDIS_URL=redis://cache\n"), 0o600)
	imp := h.ok("", "import", "staging.env", "-e", "staging")
	if !strings.Contains(imp.stderr, "2 added") {
		t.Errorf("import summary: %s", imp.stderr)
	}

	diff := h.run("", "diff", "development", "staging", "--exit-code")
	if diff.code != 1 {
		t.Errorf("diff --exit-code = %d, want 1", diff.code)
	}
	for _, want := range []string{"- TLS_KEY", "+ REDIS_URL", "1 shared key with different values"} {
		if !strings.Contains(diff.stdout, want) {
			t.Errorf("diff output missing %q:\n%s", want, diff.stdout)
		}
	}

	// Environments.
	h.ok("", "envs", "add", "qa", "--from", "staging")
	if got := h.ok("", "get", "REDIS_URL", "-e", "qa").stdout; got != "redis://cache\n" {
		t.Errorf("copied env value = %q", got)
	}
	h.fails("--force", "envs", "remove", "qa")
	h.ok("", "envs", "remove", "qa", "--force")
	envs := h.ok("", "envs").stdout
	if strings.Contains(envs, "qa") || !strings.Contains(envs, "* development") {
		t.Errorf("envs output:\n%s", envs)
	}

	h.ok("", "delete", "TLS_KEY")
	h.fails("not found", "get", "TLS_KEY")
	h.fails("not found", "delete", "TLS_KEY")
}

func TestEnvSelection(t *testing.T) {
	h := newHarness(t)
	h.ok("", "init", "--envs", "dev,prod")

	h.fails(`environment "development" does not exist`, "set", "K", "v")
	h.fails("envy envs add prdo", "set", "K", "v", "-e", "prdo")
	h.fails("invalid environment", "set", "K", "v", "-e", "../../etc")

	h.env[envTarget] = "prod"
	h.ok("", "set", "K", "from-env-var")
	if got := h.ok("", "get", "K", "-e", "prod").stdout; got != "from-env-var\n" {
		t.Errorf("ENVY_ENV not honoured: %q", got)
	}
}

func TestPasswordHandling(t *testing.T) {
	h := newHarness(t)

	h.env[envPassword] = "short"
	h.fails("at least 8 characters", "init")

	h.env[envPassword] = testPassword
	h.ok("", "init")

	h.env[envPassword] = "wrong-password"
	h.fails("incorrect master password", "list")

	delete(h.env, envPassword)
	h.fails("ENVY_PASSWORD", "list")

	h.env[envPassword] = testPassword
	h.env["ENVY_NEW_PASSWORD"] = "a-brand-new-password"
	h.ok("", "passwd")
	h.fails("incorrect master password", "list")
	h.env[envPassword] = "a-brand-new-password"
	h.ok("", "list")
}

func TestInvalidKey(t *testing.T) {
	h := newHarness(t)
	h.ok("", "init")
	h.fails("invalid key", "set", "MY-KEY", "v")

	// The parser rejects some names; envy's own validation catches the rest.
	os.WriteFile("bad.env", []byte("GOOD=1\nBAD-KEY=2\n"), 0o600)
	h.fails("bad.env", "import", "bad.env")
	os.WriteFile("bad.env", []byte("GOOD=1\nBAD.KEY=2\n"), 0o600)
	h.fails("invalid key", "import", "bad.env")
	h.fails("not found", "get", "GOOD") // nothing was imported
}

func TestNotInitialized(t *testing.T) {
	h := newHarness(t)
	h.fails("envy init", "list")
}

func TestRun(t *testing.T) {
	h := newHarness(t)
	h.ok("", "init")
	h.ok("", "set", "GREETING", "hello from envy")

	t.Setenv("ENVY_TEST_HELPER", "1")
	t.Setenv("GREETING", "overridden")
	t.Setenv(envPassword, "must-not-leak")

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := h.run("", "run", "--", self, "GREETING", envPassword)
	if r.code != 0 {
		t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if r.stdout != "GREETING=hello from envy\nENVY_PASSWORD=\n" {
		t.Errorf("child saw:\n%s", r.stdout)
	}

	if r := h.run("", "run", "--", self, "exit:7"); r.code != 7 {
		t.Errorf("exit code = %d, want 7", r.code)
	}
	h.fails("not found", "run", "--", "definitely-not-a-real-command-xyz")
}

// helperProcess prints the requested variables, or exits with a code.
func helperProcess() {
	for _, arg := range os.Args[1:] {
		if code, ok := strings.CutPrefix(arg, "exit:"); ok {
			var n int
			fmt.Sscan(code, &n)
			os.Exit(n)
		}
		fmt.Printf("%s=%s\n", arg, os.Getenv(arg))
	}
	os.Exit(0)
}

func TestAudit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	h := newHarness(t)
	if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	r := h.ok("", "audit", "--json")
	if strings.TrimSpace(r.stdout) != "[]" {
		t.Errorf("clean repo audit = %s", r.stdout)
	}

	os.WriteFile(".env.local", []byte("X=1\n"), 0o600)
	r = h.run("", "audit")
	if r.code != 1 || !strings.Contains(r.stdout, ".env.local") {
		t.Errorf("audit with unignored .env: exit %d\n%s", r.code, r.stdout)
	}
	h.ok("", "audit", "--exit-zero")
}

func TestVersion(t *testing.T) {
	h := newHarness(t)
	if out := h.ok("", "--version").stdout; out != "envy test\n" {
		t.Errorf("--version = %q", out)
	}
	if out := h.ok("", "version").stdout; !strings.HasPrefix(out, "envy test\n") {
		t.Errorf("version = %q", out)
	}
}

func TestMergeEnv(t *testing.T) {
	got := mergeEnv(
		[]string{"PATH=/bin", "API=old", "ENVY_PASSWORD=x", "envy_password=y"},
		map[string]string{"API": "new"},
	)
	joined := strings.Join(got, ";")
	if strings.Contains(joined, "API=old") || !strings.Contains(joined, "API=new") ||
		strings.Contains(strings.ToUpper(joined), "ENVY_PASSWORD") || !strings.Contains(joined, "PATH=/bin") {
		t.Errorf("mergeEnv = %v", got)
	}
}

func TestEnsureGitignored(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	os.WriteFile(path, []byte("node_modules"), 0o644)

	for i := 0; i < 2; i++ {
		if _, err := ensureGitignored(path); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(path)
	if strings.Count(string(got), ".envy/") != 1 || !strings.HasPrefix(string(got), "node_modules\n") {
		t.Errorf(".gitignore = %q", got)
	}
}
