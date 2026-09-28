//go:build !integration && !e2e

package transaction

// The no-real-git guard's proving tests for internal/repository/transaction (change
// 0466). The guard lives in internal/testsupport (InstallNoGitGuard) and is
// installed from TestMain in main_test.go. These tests prove it is installed in
// THIS package's binary and fails the package on any real-git exec, even one a test
// tolerates. Real-git tests live behind //go:build integration in the
// tests/test_go_integration_transaction_*.sh shards.

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` resolves the shim.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	testsupport.AssertNoGitGuardShadowsGit(t)
}

// TestNoGitGuardRefusesBareExec: a bare git exec gets the guard's exit code and diagnostic.
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	testsupport.AssertNoGitGuardRefusesBareExec(t, nogitPkg)
}

// TestNoGitGuardFailsTolerantTest: a test that swallows the git failure still fails the package.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	testsupport.NoGitGuardTolerantProbe(t, nogitPkg, "TestNoGitGuardFailsTolerantTest")
}
