//go:build integration || e2e

package testsupport

// InstallNoGitGuard is the tagged builds' no-op twin of the default-build guard in
// nogit_install.go (change 0466, formerly internal/app/nogit_guard_off_test.go).
// The integration and e2e corpora exist to run real git, so they install no shim
// and the finisher returns m.Run's code as-is (never an error). Exactly one of the two files
// compiles for any tag set, so every guarded TestMain stays single-sourced.
func InstallNoGitGuard(pkg, shardGlob string) (func(code int) int, error) {
	return func(code int) int { return code }, nil
}
