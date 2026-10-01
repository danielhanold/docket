//go:build integration

package testsupport

import "flag"

// RefuseUnfilteredIntegrationRun is the integration build's real unfiltered-run
// guard (change 0479; see unfiltered.go). Call it from TestMain after any re-exec
// routing and before m.Run. It parses the testing flags if they are not parsed yet
// (the documented TestMain pattern; m.Run skips a second parse) and returns the
// remedy as an error on refusal. The caller prints it and exits non-zero; a
// library never ends the process.
func RefuseUnfilteredIntegrationRun(pkg, shardGlob string) error {
	if !flag.Parsed() {
		flag.Parse()
	}
	return checkUnfilteredRun(flag.CommandLine, pkg, shardGlob)
}
