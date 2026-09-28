package transaction

import (
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (change 0466): the default-tag internal/repository/transaction test corpus never
// starts a real git; real-git tests live behind //go:build integration in the
// tests/test_go_integration_transaction_*.sh shards.
const (
	nogitPkg       = "internal/repository/transaction"
	nogitShardGlob = "tests/test_go_integration_transaction_*.sh"
)

// TestMain installs the no-real-git guard (testsupport.InstallNoGitGuard) around
// m.Run in the default build; the integration build gets testsupport's identity
// finisher, so the tagged shards run real git as before.
func TestMain(m *testing.M) {
	finish := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	os.Exit(finish(m.Run()))
}
