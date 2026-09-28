//go:build integration || e2e

package app

// installNoGitGuard is the tagged builds' no-op twin of the default-build guard in
// nogit_guard_test.go (change 0465). The integration and e2e corpora exist to run
// real git, so they install no shim and the finisher returns m.Run's code as-is.
// Exactly one of the two files compiles for any tag set, so TestMain stays
// single-sourced.
func installNoGitGuard() func(code int) int { return func(code int) int { return code } }
