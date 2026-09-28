//go:build !integration && !e2e

package app

// The default-build no-real-git guard (change 0465). Change 0333 moved the slow
// real-git, subprocess, and process-lifecycle corpus behind `//go:build integration`,
// but nothing stopped new real-git tests landing in the default corpus, which
// tests/test_go_race.sh instruments. This guard makes the partition an enforced
// invariant: the default-tag internal/app test corpus never starts a real `git`.
//
// Mechanism (keyed on the exec itself, never on spellings): installNoGitGuard, called
// from TestMain before m.Run, puts a directory holding a refusing `git` shim at the
// FRONT of PATH. Every PATH-resolved route to git (gitcli.NewClient's exec.LookPath,
// a bare exec.Command("git", …), a fixture helper, a child process inheriting PATH)
// resolves the shim. Known limits, none used by default tests today: a client built
// with gitcli.WithExecutable(<absolute path>), a test that replaces PATH wholesale
// rather than prepending to it, and a detached child that runs git after m.Run
// returns (once the shim dir is removed) all bypass the shim. The shim exits nogitGuardExit with the nogitGuardDiagnostic on stderr AND
// appends "<cwd>\t<argv>" to a violation log, so a test that tolerates the failure
// still turns the package red when nogitVerdict reads the log after m.Run.
//
// Only this guard's own proving tests may call the shim without recording a
// violation, by passing nogitGuardProbeArg as the first argument.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

const (
	nogitGuardProbeArg   = "docket-nogit-guard-probe"
	nogitGuardDiagnostic = "docket nogit guard: default internal/app tests must not run real git"
	nogitGuardExit       = 97
	nogitSelfProbeEnv    = "DOCKET_NOGIT_GUARD_SELF_PROBE"
)

// nogitGuardDir is the installed shim directory ("" when the guard is not installed).
var nogitGuardDir string

// installNoGitGuard installs the refusing git shim at the front of PATH and returns
// the finisher TestMain wraps around m.Run. Setup failure exits the binary non-zero:
// a guard that silently failed to install would certify nothing.
func installNoGitGuard() func(code int) int {
	// tempdir-exempt: TestMain installs the shim for the whole package run; there is no t to own a fixture dir.
	dir, err := os.MkdirTemp("", "docket-app-nogit-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot create the shim directory: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	logPath := filepath.Join(dir, "violations.log")
	if strings.ContainsAny(logPath, "'\n") {
		fmt.Fprintf(os.Stderr, "%s: shim log path %q is not single-quote safe\n", nogitGuardDiagnostic, logPath)
		os.Exit(1)
	}
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(nogitShimScript(logPath)), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot write the shim: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	// Explicit chmod: a create-time mode is masked by the umask.
	if err := os.Chmod(shim, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot chmod the shim: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot prepend the shim to PATH: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	nogitGuardDir = dir
	return func(code int) int {
		verdict := nogitVerdict(logPath, code, os.Stderr)
		_ = os.RemoveAll(dir)
		return verdict
	}
}

// nogitShimScript renders the refusing git: it records every non-probe invocation
// as "<cwd>\t<argv>" in logPath and always exits nogitGuardExit with the diagnostic
// and the remedy on stderr.
func nogitShimScript(logPath string) string {
	return "#!/bin/sh\n" +
		"if [ \"${1-}\" != '" + nogitGuardProbeArg + "' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '" + logPath + "'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a tests/test_go_integration_app_*.sh shard (change 0465; partition from change 0333)\\n' '" +
		nogitGuardDiagnostic + "' \"$*\" >&2\n" +
		fmt.Sprintf("exit %d\n", nogitGuardExit)
}

// nogitVerdict folds the violation log into m.Run's exit code. A missing or empty
// log is clean and leaves code unchanged. Any recorded attempt, or a log that exists
// but cannot be read, fails the package: a probe error is never clean absence.
func nogitVerdict(logPath string, code int, w io.Writer) int {
	raw, err := os.ReadFile(logPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(w, "%s: cannot read the violation log %s: %v\n", nogitGuardDiagnostic, logPath, err)
		return nogitFailCode(code)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return code
	}
	fmt.Fprintf(w, "%s: %d real-git exec attempt(s) reached the guard shim (a test that tolerated the failure still counts); <cwd>\\t<argv>:\n", nogitGuardDiagnostic, len(lines))
	for _, l := range lines {
		fmt.Fprintf(w, "  %s\n", l)
	}
	return nogitFailCode(code)
}

func nogitFailCode(code int) int {
	if code == 0 {
		return 1
	}
	return code
}

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` (gitcli.NewClient uses
// exec.LookPath) resolves the shim, not a real git.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	if nogitGuardDir == "" {
		t.Fatalf("the no-real-git guard is not installed (nogitGuardDir empty); TestMain must call installNoGitGuard before m.Run")
	}
	p, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath(git): %v", err)
	}
	if filepath.Dir(p) != nogitGuardDir {
		t.Fatalf("git resolves to %q, want the guard shim in %q", p, nogitGuardDir)
	}
}

// TestNoGitGuardRefusesBareExec pins the MECHANISM, not just "it failed": real git
// also fails on an unknown subcommand, so the assert is the guard's exit code AND
// its diagnostic (learning assert-pins-outcome-not-mechanism).
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	out, err := exec.Command("git", nogitGuardProbeArg).CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != nogitGuardExit {
		t.Fatalf("git exec must exit %d from the guard shim, got err=%v output=%q", nogitGuardExit, err, out)
	}
	if !strings.Contains(string(out), nogitGuardDiagnostic) {
		t.Fatalf("git exec output must carry the guard diagnostic %q, got %q", nogitGuardDiagnostic, out)
	}
}

// TestNoGitGuardVerdict covers the post-m.Run verdict over the violation log.
func TestNoGitGuardVerdict(t *testing.T) {
	dir := testsupport.TempDir(t) // bare X.TempDir() is banned by internal/repoguard tempdir_fixture_test.go
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name     string
		logPath  string
		code     int
		want     int
		wantText string
	}{
		{"missing log is clean", filepath.Join(dir, "absent.log"), 0, 0, ""},
		{"empty log is clean", write("empty.log", ""), 0, 0, ""},
		{"one violation fails a green run", write("one.log", "/tmp/x\tstatus --porcelain\n"), 0, 1, "1 real-git exec attempt(s)"},
		{"violation keeps an existing failure code", write("keep.log", "/tmp/x\tlog\n"), 2, 2, "/tmp/x\tlog"},
		{"unreadable log fails closed", dir, 0, 1, "cannot read the violation log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			got := nogitVerdict(tc.logPath, tc.code, &buf)
			if got != tc.want {
				t.Fatalf("nogitVerdict(%q, %d) = %d, want %d; output:\n%s", tc.logPath, tc.code, got, tc.want, buf.String())
			}
			if tc.wantText == "" && buf.Len() != 0 {
				t.Fatalf("clean verdict must print nothing, got:\n%s", buf.String())
			}
			if tc.wantText != "" && !strings.Contains(buf.String(), tc.wantText) {
				t.Fatalf("verdict output must contain %q, got:\n%s", tc.wantText, buf.String())
			}
		})
	}
}

// TestNoGitGuardFailsTolerantTest proves a test that SWALLOWS the git failure still
// fails the package: it re-execs this test binary running only itself in child
// mode, where it runs `git status` and ignores the error, then asserts the child
// binary exits non-zero and lists the violation.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	if os.Getenv(nogitSelfProbeEnv) == "1" {
		_ = exec.Command("git", "status").Run() // tolerated on purpose
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNoGitGuardFailsTolerantTest$", "-test.count=1")
	cmd.Env = append(os.Environ(), nogitSelfProbeEnv+"=1")
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() == 0 {
		t.Fatalf("a package whose test tolerated a git exec must exit non-zero, got err=%v output:\n%s", err, out)
	}
	for _, want := range []string{nogitGuardDiagnostic, "1 real-git exec attempt(s)", "\tstatus"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("child output must contain %q, got:\n%s", want, out)
		}
	}
}
