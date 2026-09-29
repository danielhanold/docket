//go:build !integration && !e2e

package gatedrive

import (
	"fmt"
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (change 0470, reapplying change 0466's wiring): the default-tag
// internal/gatedrive test corpus never starts a real git; real-git and
// real-process tests live behind //go:build integration in the
// tests/test_go_integration_gatedrive_*.sh shards.
const (
	nogitPkg       = "internal/gatedrive"
	nogitShardGlob = "tests/test_go_integration_gatedrive_*.sh"
)

// TestMain installs the no-real-git guard (testsupport.InstallNoGitGuard) around
// m.Run in the default build. Unlike internal/workspace and
// internal/repository/transaction, this file carries a build constraint: the
// integration build already has a TestMain (supervisor_integration_test.go, which
// routes the supervisor and child re-exec roles), and the tagged corpora exist to
// run real git, so exactly one TestMain compiles for any tag set.
func TestMain(m *testing.M) {
	finish, err := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	if err != nil {
		// The library returns the setup failure; this TestMain ends the process.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(finish(m.Run()))
}
