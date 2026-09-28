package workspace

import (
	"fmt"
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (change 0466): the default-tag internal/workspace test corpus never starts a real
// git; real-git tests live behind //go:build integration in the
// tests/test_go_integration_workspace_*.sh shards.
const (
	nogitPkg       = "internal/workspace"
	nogitShardGlob = "tests/test_go_integration_workspace_*.sh"
)

// TestMain installs the no-real-git guard (testsupport.InstallNoGitGuard) around
// m.Run in the default build; the integration build gets testsupport's identity
// finisher, so the tagged shards run real git as before.
func TestMain(m *testing.M) {
	finish, err := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	if err != nil {
		// The library returns the setup failure; this TestMain ends the process.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(finish(m.Run()))
}
