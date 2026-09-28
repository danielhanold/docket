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
// cannot carry, exits the binary non-zero: a guard that silently failed to install
// would certify nothing.
func InstallNoGitGuard(pkg, shardGlob string) func(code int) int {
	diag := NoGitGuardDiagnostic(pkg)
	if err := validateNoGitGuardArgs(pkg, shardGlob); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", diag, err)
		os.Exit(1)
	}
	// tempdir-exempt: TestMain installs the shim for the whole package run; there is no t to own a fixture dir.
	dir, err := os.MkdirTemp("", "docket-"+path.Base(pkg)+"-nogit-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot create the shim directory: %v\n", diag, err)
		os.Exit(1)
	}
	logPath := filepath.Join(dir, "violations.log")
	if strings.ContainsAny(logPath, "'\n") {
		fmt.Fprintf(os.Stderr, "%s: shim log path %q is not single-quote safe\n", diag, logPath)
		os.Exit(1)
	}
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(NoGitShimScript(pkg, shardGlob, logPath)), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot write the shim: %v\n", diag, err)
		os.Exit(1)
	}
	// Explicit chmod: a create-time mode is masked by the umask.
	if err := os.Chmod(shim, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot chmod the shim: %v\n", diag, err)
		os.Exit(1)
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot prepend the shim to PATH: %v\n", diag, err)
		os.Exit(1)
	}
	noGitGuardDir = dir
	return func(code int) int {
		verdict := NoGitVerdict(pkg, logPath, code, os.Stderr)
		_ = os.RemoveAll(dir)
		return verdict
	}
}
