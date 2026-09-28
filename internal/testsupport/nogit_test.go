package testsupport

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoGitShimScriptKeepsInternalAppBytes pins the hoist as byte-identical for
// internal/app (change 0466): the literal was proven against the pre-hoist
// internal/app nogitShimScript before that function was deleted.
func TestNoGitShimScriptKeepsInternalAppBytes(t *testing.T) {
	const want = "#!/bin/sh\n" +
		"if [ \"${1-}\" != 'docket-nogit-guard-probe' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '/x/violations.log'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a tests/test_go_integration_app_*.sh shard (change 0465; partition from change 0333)\\n' 'docket nogit guard: default internal/app tests must not run real git' \"$*\" >&2\n" +
		"exit 97\n"
	if got := NoGitShimScript("internal/app", "tests/test_go_integration_app_*.sh", "/x/violations.log"); got != want {
		t.Fatalf("internal/app shim bytes changed:\n got %q\nwant %q", got, want)
	}
	if got, want := NoGitGuardDiagnostic("internal/app"), "docket nogit guard: default internal/app tests must not run real git"; got != want {
		t.Fatalf("NoGitGuardDiagnostic(internal/app) = %q, want %q", got, want)
	}
}

// TestNoGitShimScriptRefusesAndLogs EXECUTES a rendered shim: a non-probe call is
// logged as "<cwd>\t<argv>" and refused with the package's diagnostic, its shard
// glob, and exit NoGitGuardExit; a probe call is refused but not logged.
func TestNoGitShimScriptRefusesAndLogs(t *testing.T) {
	dir := TempDir(t)
	logPath := filepath.Join(dir, "violations.log")
	shim := filepath.Join(dir, "git")
	pkg, glob := "internal/repository/transaction", "tests/test_go_integration_transaction_*.sh"
	if err := os.WriteFile(shim, []byte(NoGitShimScript(pkg, glob, logPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(shim, "status", "--porcelain").CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != NoGitGuardExit {
		t.Fatalf("shim must exit %d, got err=%v output=%q", NoGitGuardExit, err, out)
	}
	for _, want := range []string{NoGitGuardDiagnostic(pkg), "(git status --porcelain)", glob} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("shim stderr must contain %q, got %q", want, out)
		}
	}
	logged, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(logged), "\tstatus --porcelain") {
		t.Fatalf("violation log must record the call, got %q (err %v)", logged, err)
	}
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command(shim, NoGitGuardProbeArg).CombinedOutput(); err == nil {
		t.Fatalf("a probe call must still be refused")
	}
	if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a probe call must not be logged (stat err %v)", err)
	}
}

// TestNoGitVerdict covers the post-m.Run verdict over the violation log (moved
// from internal/app's TestNoGitGuardVerdict by change 0466).
func TestNoGitVerdict(t *testing.T) {
	dir := TempDir(t)
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
			got := NoGitVerdict("internal/app", tc.logPath, tc.code, &buf)
			if got != tc.want {
				t.Fatalf("NoGitVerdict(%q, %d) = %d, want %d; output:\n%s", tc.logPath, tc.code, got, tc.want, buf.String())
			}
			if tc.wantText == "" && buf.Len() != 0 {
				t.Fatalf("clean verdict must print nothing, got:\n%s", buf.String())
			}
			if tc.wantText != "" && !strings.Contains(buf.String(), tc.wantText) {
				t.Fatalf("verdict output must contain %q, got:\n%s", tc.wantText, buf.String())
			}
			if tc.wantText != "" && !strings.Contains(buf.String(), NoGitGuardDiagnostic("internal/app")) {
				t.Fatalf("verdict output must carry the package diagnostic, got:\n%s", buf.String())
			}
		})
	}
}

// TestValidateNoGitGuardArgs: the shim embeds pkg and shardGlob inside a
// single-quoted printf, so a quote, percent sign, backslash, or newline (or an
// empty value) is refused rather than rendered into a broken shim.
func TestValidateNoGitGuardArgs(t *testing.T) {
	okGlob := "tests/test_go_integration_app_*.sh"
	cases := []struct {
		name, pkg, glob string
		ok              bool
	}{
		{"app", "internal/app", okGlob, true},
		{"transaction", "internal/repository/transaction", "tests/test_go_integration_transaction_*.sh", true},
		{"empty package", "", okGlob, false},
		{"empty glob", "internal/app", "", false},
		{"quote in package", "internal/a'pp", okGlob, false},
		{"percent in glob", "internal/app", "tests/%s.sh", false},
		{"backslash in glob", "internal/app", `tests\x.sh`, false},
		{"newline in package", "internal/app\nx", okGlob, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNoGitGuardArgs(tc.pkg, tc.glob)
			if tc.ok && err != nil {
				t.Fatalf("validateNoGitGuardArgs(%q, %q) = %v, want nil", tc.pkg, tc.glob, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("validateNoGitGuardArgs(%q, %q) = nil, want a refusal", tc.pkg, tc.glob)
			}
		})
	}
}
