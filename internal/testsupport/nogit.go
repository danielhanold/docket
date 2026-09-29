package testsupport

// The default-build no-real-git guard (change 0465, hoisted here from internal/app
// by change 0466). Change 0333 moved the slow real-git, subprocess, and
// process-lifecycle corpus behind `//go:build integration`; this guard makes that
// partition an enforced invariant for a package: its default-tag test corpus never
// starts a real `git`. Installed from the TestMain of internal/app,
// internal/repository/transaction, internal/workspace (change 0466), and
// internal/gatedrive (change 0470).
//
// Mechanism (keyed on the exec itself, never on spellings): InstallNoGitGuard,
// called from TestMain before m.Run, puts a directory holding a refusing `git` shim
// at the FRONT of PATH. Every PATH-resolved route to git (gitcli.NewClient's
// exec.LookPath, a bare exec.Command("git", …), a fixture helper, a child process
// inheriting PATH) resolves the shim. Known limits, none used by default tests
// today: a client built with gitcli.WithExecutable(<absolute path>), a test that
// replaces PATH wholesale rather than prepending to it, and a detached child that
// runs git after m.Run returns (once the shim dir is removed) all bypass the shim.
// The shim exits NoGitGuardExit with NoGitGuardDiagnostic(pkg) on stderr AND
// appends "<cwd>\t<argv>" to a violation log, so a test that tolerates the failure
// still turns the package red when NoGitVerdict reads the log after m.Run.
//
// Only a package's own proving tests may call the shim without recording a
// violation, by passing NoGitGuardProbeArg as the first argument.
//
// Build split: InstallNoGitGuard is real only in the default build
// (nogit_install.go, `//go:build !integration && !e2e`); the tagged corpora exist
// to run real git and get the identity finisher (nogit_install_off.go). A build
// tag applies to every package compiled into the test binary, testsupport
// included, so the split lives here once instead of as a twin file in each
// guarded package.

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
)

const (
	// NoGitGuardProbeArg as git's first argument marks a proving-test call the
	// shim refuses without logging a violation.
	NoGitGuardProbeArg = "docket-nogit-guard-probe"
	// NoGitGuardExit is the shim's exit code.
	NoGitGuardExit = 97
	// NoGitSelfProbeEnv routes NoGitGuardTolerantProbe's re-exec'd child.
	NoGitSelfProbeEnv = "DOCKET_NOGIT_GUARD_SELF_PROBE"
)

// noGitGuardDir is the installed shim directory ("" when the guard is not installed).
var noGitGuardDir string

// NoGitGuardDir returns the installed shim directory, or "" when no guard is installed.
func NoGitGuardDir() string { return noGitGuardDir }

// NoGitGuardDiagnostic is the guard's stderr diagnostic for package pkg.
func NoGitGuardDiagnostic(pkg string) string {
	return "docket nogit guard: default " + pkg + " tests must not run real git"
}

// validateNoGitGuardArgs refuses a pkg or shardGlob the shim's single-quoted printf
// cannot carry verbatim: empty, or holding a quote, percent sign, backslash, or newline.
func validateNoGitGuardArgs(pkg, shardGlob string) error {
	for _, v := range []struct{ name, val string }{{"package", pkg}, {"shard glob", shardGlob}} {
		if v.val == "" {
			return fmt.Errorf("the %s is empty", v.name)
		}
		if strings.ContainsAny(v.val, "'%\\\n") {
			return fmt.Errorf("the %s %q contains a quote, percent sign, backslash, or newline, which the shim's single-quoted printf cannot carry", v.name, v.val)
		}
	}
	return nil
}

// NoGitShimScript renders the refusing git for package pkg: it records every
// non-probe invocation as "<cwd>\t<argv>" in logPath and always exits
// NoGitGuardExit with the diagnostic and the remedy (naming shardGlob) on stderr.
func NoGitShimScript(pkg, shardGlob, logPath string) string {
	return "#!/bin/sh\n" +
		"if [ \"${1-}\" != '" + NoGitGuardProbeArg + "' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '" + logPath + "'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a " + shardGlob + " shard (change 0465; partition from change 0333)\\n' '" +
		NoGitGuardDiagnostic(pkg) + "' \"$*\" >&2\n" +
		fmt.Sprintf("exit %d\n", NoGitGuardExit)
}

// NoGitVerdict folds the violation log into m.Run's exit code. A missing or empty
// log is clean and leaves code unchanged. Any recorded attempt, or a log that exists
// but cannot be read, fails the package: a probe error is never clean absence.
func NoGitVerdict(pkg, logPath string, code int, w io.Writer) int {
	diag := NoGitGuardDiagnostic(pkg)
	raw, err := os.ReadFile(logPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(w, "%s: cannot read the violation log %s: %v\n", diag, logPath, err)
		return noGitFailCode(code)
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
	fmt.Fprintf(w, "%s: %d real-git exec attempt(s) reached the guard shim (a test that tolerated the failure still counts); <cwd>\\t<argv>:\n", diag, len(lines))
	for _, l := range lines {
		fmt.Fprintf(w, "  %s\n", l)
	}
	return noGitFailCode(code)
}

func noGitFailCode(code int) int {
	if code == 0 {
		return 1
	}
	return code
}

// AssertNoGitGuardShadowsGit: every PATH lookup of `git` (gitcli.NewClient uses
// exec.LookPath) resolves the installed shim, not a real git.
func AssertNoGitGuardShadowsGit(t testing.TB) {
	t.Helper()
	if noGitGuardDir == "" {
		t.Fatalf("the no-real-git guard is not installed (NoGitGuardDir empty); TestMain must call testsupport.InstallNoGitGuard before m.Run")
	}
	p, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath(git): %v", err)
	}
	if filepath.Dir(p) != noGitGuardDir {
		t.Fatalf("git resolves to %q, want the guard shim in %q", p, noGitGuardDir)
	}
}

// AssertNoGitGuardRefusesBareExec pins the MECHANISM, not just "it failed": real
// git also fails on an unknown subcommand, so the assert is the guard's exit code
// AND pkg's diagnostic (learning assert-pins-outcome-not-mechanism).
func AssertNoGitGuardRefusesBareExec(t testing.TB, pkg string) {
	t.Helper()
	out, err := exec.Command("git", NoGitGuardProbeArg).CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != NoGitGuardExit {
		t.Fatalf("git exec must exit %d from the guard shim, got err=%v output=%q", NoGitGuardExit, err, out)
	}
	if !strings.Contains(string(out), NoGitGuardDiagnostic(pkg)) {
		t.Fatalf("git exec output must carry the guard diagnostic %q, got %q", NoGitGuardDiagnostic(pkg), out)
	}
}

// NoGitGuardTolerantProbe proves a test that SWALLOWS the git failure still fails
// the package. testName must be the calling test's exact name: the helper re-execs
// the test binary running only that test in child mode, where it runs `git status`
// and ignores the error, then asserts the child binary exits non-zero and lists the
// violation.
func NoGitGuardTolerantProbe(t *testing.T, pkg, testName string) {
	t.Helper()
	if os.Getenv(NoGitSelfProbeEnv) == "1" {
		_ = exec.Command("git", "status").Run() // tolerated on purpose
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), NoGitSelfProbeEnv+"=1")
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() == 0 {
		t.Fatalf("a package whose test tolerated a git exec must exit non-zero, got err=%v output:\n%s", err, out)
	}
	for _, want := range []string{NoGitGuardDiagnostic(pkg), "1 real-git exec attempt(s)", "\tstatus"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("child output must contain %q, got:\n%s", want, out)
		}
	}
}
