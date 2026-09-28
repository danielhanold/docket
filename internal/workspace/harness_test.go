package workspace

import (
	"os/exec"
	"testing"
)

// The real-git fixture builders (wsRepos, mainModeRepo, docketModeRepo, and the
// git oracle helpers) live behind //go:build integration in
// harness_integration_test.go (change 0466); only requireGit is shared with the
// default corpus.

// requireGit skips when no real git is on PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}
