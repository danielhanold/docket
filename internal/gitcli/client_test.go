package gitcli

import (
	"os/exec"
	"testing"
	"time"
)

// Network sites gated by the read/write budget split (mechanical enumeration,
// `grep -rn "network: *true" internal/gitcli/ --include='*.go' | grep -v _test`):
//
//	refs.go:37       RemoteDefaultBranch ls-remote --symref  READ
//	refs.go:90       FetchBranch fetch                       READ
//	refs.go:200      classifyFetchFailure ls-remote probe    READ
//	refdelete.go:97  DeleteRemoteRefLease push               WRITE
//	push.go:67       PushLease push                          WRITE
//	push.go:135      PushCreateLease push                    WRITE
//	remoteref.go:48  ProbeRemoteBranch ls-remote             READ
//
// push.go (PushLease, PushCreateLease) and refdelete.go (DeleteRemoteRefLease)
// are writes; every other site (fetch, ls-remote, discovery probes,
// classifyFetchFailure) is a read.

// TestNetworkReadWriteTimeoutOptions asserts the two new options resolve to
// their explicit values on the accessors.
func TestNetworkReadWriteTimeoutOptions(t *testing.T) {
	c, err := NewClient(WithNetworkReadTimeout(30*time.Second), WithNetworkWriteTimeout(60*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.NetworkReadTimeout(); got != 30*time.Second {
		t.Fatalf("read timeout = %v", got)
	}
	if got := c.NetworkWriteTimeout(); got != 60*time.Second {
		t.Fatalf("write timeout = %v", got)
	}
}

// TestNetworkTimeoutDefaultsInheritBase proves that without the new options both
// budgets are the existing network default, so every standalone client is
// behaviorally unchanged.
func TestNetworkTimeoutDefaultsInheritBase(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.NetworkReadTimeout() != defaultNetworkTimeout || c.NetworkWriteTimeout() != defaultNetworkTimeout {
		t.Fatalf("defaults changed: read=%v write=%v", c.NetworkReadTimeout(), c.NetworkWriteTimeout())
	}
}

// TestNetworkReadWriteTimeoutInheritExplicitBase proves the budgets inherit the
// explicitly-set WithNetworkTimeout base when the read/write options are absent,
// so a client tuned only through the legacy option keeps one shared budget.
func TestNetworkReadWriteTimeoutInheritExplicitBase(t *testing.T) {
	c, err := NewClient(WithNetworkTimeout(90 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if c.NetworkReadTimeout() != 90*time.Second || c.NetworkWriteTimeout() != 90*time.Second {
		t.Fatalf("did not inherit base: read=%v write=%v", c.NetworkReadTimeout(), c.NetworkWriteTimeout())
	}
}

// TestNonPositiveReadWriteTimeoutRejected asserts an explicitly non-positive
// read or write budget is rejected at construction — the zero default only
// inherits when the option is absent, never when it is passed explicitly.
func TestNonPositiveReadWriteTimeoutRejected(t *testing.T) {
	if _, err := NewClient(WithNetworkReadTimeout(0)); err == nil {
		t.Fatal("zero read timeout accepted")
	}
	if _, err := NewClient(WithNetworkWriteTimeout(-time.Second)); err == nil {
		t.Fatal("negative write timeout accepted")
	}
}

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
