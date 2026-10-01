//go:build !integration

package testsupport

// RefuseUnfilteredIntegrationRun is the no-op twin of the integration build's guard
// (unfiltered_guard.go, change 0479). The default and e2e builds never refuse.
// Exactly one of the two files compiles for any tag set.
func RefuseUnfilteredIntegrationRun(pkg, shardGlob string) error { return nil }
