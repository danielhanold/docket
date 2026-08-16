package gitcli

import (
	"context"
	"testing"
)

// TestProbeRemoteBranchFoundReturnsExactID proves a probe of an existing remote
// branch reports found and the exact FULL object id the origin's own ref holds —
// an oracle read directly from origin, never from the adapter under test.
func TestProbeRemoteBranchFoundReturnsExactID(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	want := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main"))
	rr, err := c.ProbeRemoteBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("ProbeRemoteBranch: %v", err)
	}
	if rr.State != RemoteRefFound {
		t.Fatalf("State = %q, want %q", rr.State, RemoteRefFound)
	}
	if rr.Commit != want {
		t.Errorf("Commit = %q, want %q", rr.Commit, want)
	}
}

// TestProbeRemoteBranchAbsent proves a probe of a never-created branch reports a
// clean absence — the empty id, no error.
func TestProbeRemoteBranchAbsent(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	rr, err := c.ProbeRemoteBranch(ctx, repo, "origin", "refs/heads/never-created")
	if err != nil {
		t.Fatalf("ProbeRemoteBranch: %v", err)
	}
	if rr.State != RemoteRefAbsent {
		t.Fatalf("State = %q, want %q", rr.State, RemoteRefAbsent)
	}
	if rr.Commit != "" {
		t.Errorf("Commit = %q, want empty on absence", rr.Commit)
	}
}

// TestProbeRemoteBranchUnconfiguredRemoteErrorsNotAbsent proves an unconfigured
// remote name is an error (remote-unavailable), NEVER a clean absence — an
// errored probe must never share a branch with clean absence
// (learnings: probe-error-is-not-clean-absence).
func TestProbeRemoteBranchUnconfiguredRemoteErrorsNotAbsent(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	rr, err := c.ProbeRemoteBranch(ctx, repo, "nonexistent", "refs/heads/main")
	if err == nil {
		t.Fatalf("want error for unconfigured remote, got RemoteRef %+v", rr)
	}
	if rr.State == RemoteRefAbsent {
		t.Fatalf("unconfigured remote reported as clean absence")
	}
	assertKind(t, err, KindRemoteUnavailable)
}

// TestProbeRemoteBranchValidatesInput proves malformed remote/ref inputs are
// rejected as invalid-request before any git process runs.
func TestProbeRemoteBranchValidatesInput(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	repo := Repository{PrimaryWorktree: t.TempDir()}

	for _, rem := range []RemoteName{"", "-o", "or/igin"} {
		_, err := c.ProbeRemoteBranch(ctx, repo, rem, "refs/heads/main")
		assertKind(t, err, KindInvalidRequest)
	}
	for _, ref := range []RefName{"main", "-o", "refs/heads/", ":(top)x"} {
		_, err := c.ProbeRemoteBranch(ctx, repo, "origin", ref)
		assertKind(t, err, KindInvalidRequest)
	}
}

// TestProbeRemoteBranchMatchesExactRefNotPrefix proves the probe matches the
// fully qualified ref exactly: with both refs/heads/feat/x and
// refs/heads/feat/x2 on origin, probing feat/x returns only feat/x's id, never
// feat/x2's — a prefix collision does not leak the wrong id.
func TestProbeRemoteBranchMatchesExactRefNotPrefix(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	r.writerCommit(t, "feat/x", map[string]string{"x.md": "x\n"})
	r.writerCommit(t, "feat/x2", map[string]string{"x2.md": "x2\n"})
	wantX := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/feat/x"))
	wantX2 := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/feat/x2"))
	if wantX == wantX2 {
		t.Fatal("fixture branches share a commit; cannot disambiguate prefix from exact")
	}

	rr, err := c.ProbeRemoteBranch(ctx, repo, "origin", "refs/heads/feat/x")
	if err != nil {
		t.Fatalf("ProbeRemoteBranch: %v", err)
	}
	if rr.State != RemoteRefFound || rr.Commit != wantX {
		t.Fatalf("probe feat/x = %+v, want found %q (must not be feat/x2 %q)", rr, wantX, wantX2)
	}
}
