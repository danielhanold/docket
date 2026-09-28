//go:build !integration && !e2e

package testsupport

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// InstallNoGitGuard installs the refusing git shim for package pkg (its
// module-relative dir, e.g. "internal/app") at the front of PATH and returns the
// finisher TestMain wraps around m.Run. shardGlob names the package's integration
// shard runners in the remedy text. Setup failure, or a pkg/shardGlob the shim
// cannot carry, returns an error already prefixed with NoGitGuardDiagnostic(pkg)
// and a nil finisher; the calling TestMain prints it and exits non-zero (a guard
// that silently failed to install would certify nothing). A library never ends
// the process itself (cmd/docket's TestProcessExitSitesAreAllowlisted).
func InstallNoGitGuard(pkg, shardGlob string) (func(code int) int, error) {
	diag := NoGitGuardDiagnostic(pkg)
	if err := validateNoGitGuardArgs(pkg, shardGlob); err != nil {
		return nil, fmt.Errorf("%s: %w", diag, err)
	}
	// tempdir-exempt: TestMain installs the shim for the whole package run; there is no t to own a fixture dir.
	dir, err := os.MkdirTemp("", "docket-"+path.Base(pkg)+"-nogit-")
	if err != nil {
		return nil, fmt.Errorf("%s: cannot create the shim directory: %w", diag, err)
	}
	fail := func(err error) (func(code int) int, error) {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	logPath := filepath.Join(dir, "violations.log")
	if strings.ContainsAny(logPath, "'\n") {
		return fail(fmt.Errorf("%s: shim log path %q is not single-quote safe", diag, logPath))
	}
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(NoGitShimScript(pkg, shardGlob, logPath)), 0o755); err != nil {
		return fail(fmt.Errorf("%s: cannot write the shim: %w", diag, err))
	}
	// Explicit chmod: a create-time mode is masked by the umask.
	if err := os.Chmod(shim, 0o755); err != nil {
		return fail(fmt.Errorf("%s: cannot chmod the shim: %w", diag, err))
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return fail(fmt.Errorf("%s: cannot prepend the shim to PATH: %w", diag, err))
	}
	noGitGuardDir = dir
	return func(code int) int {
		verdict := NoGitVerdict(pkg, logPath, code, os.Stderr)
		_ = os.RemoveAll(dir)
		return verdict
	}, nil
}
