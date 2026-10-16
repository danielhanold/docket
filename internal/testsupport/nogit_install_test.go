//go:build !integration && !e2e

package testsupport

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstallNoGitGuardReturnsSetupError: a refused install is an error the
// calling TestMain prints and exits on, not a process exit inside the library
// (change 0466 repair; cmd/docket's TestProcessExitSitesAreAllowlisted). The error
// carries the same "<diagnostic>: <cause>" text the TestMain writes to stderr, and
// a refusal installs nothing: PATH and NoGitGuardDir stay untouched.
func TestInstallNoGitGuardReturnsSetupError(t *testing.T) {
	pathBefore, dirBefore := os.Getenv("PATH"), NoGitGuardDir()
	finish, err := InstallNoGitGuard("internal/app", "")
	if err == nil {
		t.Fatal("InstallNoGitGuard with an empty shard glob = nil error, want a refusal")
	}
	if finish != nil {
		t.Fatal("a refused install must return a nil finisher")
	}
	if want := NoGitGuardDiagnostic("internal/app") + ": the shard glob is empty"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
	if got := os.Getenv("PATH"); got != pathBefore {
		t.Fatalf("a refused install changed PATH: %q -> %q", pathBefore, got)
	}
	if got := NoGitGuardDir(); got != dirBefore {
		t.Fatalf("a refused install recorded a shim dir %q", got)
	}
}

// TestInstallNoGitGuardRecordsCleanDir: a TMPDIR with a trailing or doubled
// slash still yields a guard dir in clean form, so the shadow check compares
// one spelling of the path.
func TestInstallNoGitGuardRecordsCleanDir(t *testing.T) {
	for _, suffix := range []string{"/", "//"} {
		t.Run(suffix, func(t *testing.T) {
			t.Setenv("TMPDIR", TempDir(t)+suffix)
			t.Setenv("PATH", os.Getenv("PATH"))
			saved := noGitGuardDir
			t.Cleanup(func() { noGitGuardDir = saved })
			finish, err := InstallNoGitGuard("internal/app", "tests/test_go_integration_app_*.sh")
			if err != nil {
				t.Fatalf("InstallNoGitGuard: %v", err)
			}
			t.Cleanup(func() { finish(0) })
			if got := NoGitGuardDir(); got != filepath.Clean(got) {
				t.Fatalf("NoGitGuardDir() = %q, want clean form %q", got, filepath.Clean(got))
			}
			AssertNoGitGuardShadowsGit(t)
		})
	}
}
