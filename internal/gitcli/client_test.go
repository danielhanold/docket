package gitcli

import (
	"os/exec"
	"testing"
)

// requireGit skips the test when no real git is on PATH. CI and dev machines
// have it; a skip only fires when git is genuinely absent.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

// newRealClient builds a Client bound to the real git on PATH for the
// discovery tests that exercise genuine repositories.
func newRealClient(t *testing.T) *Client {
	t.Helper()
	requireGit(t)
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// assertKind fails unless err is a *Failure of the wanted kind.
func assertKind(t *testing.T, err error, want FailureKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error of kind %q, got nil", want)
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not a *Failure: %v", err)
	}
	if f.Kind != want {
		t.Fatalf("Failure kind = %q, want %q (%v)", f.Kind, want, err)
	}
}
