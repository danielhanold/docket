package gitcli

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// mustDiscover resolves repository identity from an invocation path or fails the
// test; refs tests operate against the discovered primary worktree.
func mustDiscover(t *testing.T, c *Client, path string) Repository {
	t.Helper()
	repo, err := c.Discover(context.Background(), DiscoverOptions{InvocationPath: path})
	if err != nil {
		t.Fatalf("Discover(%q): %v", path, err)
	}
	return repo
}

// TestRemoteDefaultBranchAsksRemote proves RemoteDefaultBranch reports the
// remote's own HEAD symref: it returns refs/heads/main from the harness origin,
// and after the origin's HEAD is repointed to refs/heads/other it returns that
// instead — a cached local guess could never track the change.
func TestRemoteDefaultBranchAsksRemote(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	ref, err := c.RemoteDefaultBranch(ctx, repo, "origin")
	if err != nil {
		t.Fatalf("RemoteDefaultBranch error: %v", err)
	}
	if ref != "refs/heads/main" {
		t.Fatalf("default branch = %q, want refs/heads/main", ref)
	}

	// Create and push branch "other", then repoint origin's HEAD at it.
	r.writerCommit(t, "other", map[string]string{"other.md": "x\n"})
	gitOut(t, r.Origin, "symbolic-ref", "HEAD", "refs/heads/other")

	ref, err = c.RemoteDefaultBranch(ctx, repo, "origin")
	if err != nil {
		t.Fatalf("RemoteDefaultBranch (after repoint) error: %v", err)
	}
	if ref != "refs/heads/other" {
		t.Fatalf("default branch after repoint = %q, want refs/heads/other", ref)
	}
}

// TestFetchBranchTargetedUpdatesTrackingRef advances main in the writer, fetches
// exactly that branch, and asserts the returned revision equals the writer's new
// commit, the tracking ref moved to it, and no tags were pulled even though the
// writer pushed one.
func TestFetchBranchTargetedUpdatesTrackingRef(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	want := r.writerCommit(t, "main", map[string]string{"new.md": "new\n"})
	gitOut(t, r.Writer, "tag", "v-should-not-fetch")
	gitOut(t, r.Writer, "push", "-q", "origin", "v-should-not-fetch")

	rev, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("FetchBranch error: %v", err)
	}
	if rev.Commit != want {
		t.Fatalf("fetched commit = %q, want %q", rev.Commit, want)
	}
	if rev.Ref != "refs/heads/main" || rev.Remote != "origin" {
		t.Fatalf("revision identity = %+v, want ref=refs/heads/main remote=origin", rev)
	}
	if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "refs/remotes/origin/main")); got != want {
		t.Fatalf("tracking ref = %q, want %q", got, want)
	}
	if tags := gitOut(t, r.Invocation, "tag"); tags != "" {
		t.Fatalf("tags were fetched: %q", tags)
	}
}

// TestFetchBranchDoesNotFetchUnrelatedBranches pushes an unrelated branch to the
// origin, fetches only main, and asserts the unrelated tracking ref never
// materializes.
func TestFetchBranchDoesNotFetchUnrelatedBranches(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	r.writerCommit(t, "unrelated", map[string]string{"u.md": "u\n"})

	if _, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main"); err != nil {
		t.Fatalf("FetchBranch error: %v", err)
	}
	if _, err := gitTry(r.Invocation, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/unrelated"); err == nil {
		t.Fatal("refs/remotes/origin/unrelated exists after a targeted main fetch")
	}
}

// TestResolveRefFoundAndNotFound resolves refs/heads/main to the plumbing
// oracle's commit and reports ref-unavailable for a nonexistent ref.
func TestResolveRefFoundAndNotFound(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	id, err := c.ResolveRef(ctx, repo, "refs/heads/main")
	if err != nil {
		t.Fatalf("ResolveRef error: %v", err)
	}
	if err := validateObjectID(id); err != nil {
		t.Fatalf("ResolveRef returned malformed id %q: %v", id, err)
	}
	want := ObjectID(gitOut(t, r.Invocation, "rev-parse", "refs/heads/main"))
	if id != want {
		t.Fatalf("ResolveRef = %q, want %q", id, want)
	}

	_, err = c.ResolveRef(ctx, repo, "refs/heads/nope")
	assertKind(t, err, KindRefUnavailable)
}

// TestRefsFailureKinds exercises the typed failure classification: an
// unconfigured remote name is remote-unavailable (both RemoteDefaultBranch and
// FetchBranch), an absent source branch is ref-unavailable, and a configured
// remote whose URL points nowhere is command-failed.
func TestRefsFailureKinds(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	_, err := c.FetchBranch(ctx, repo, "nonexistent", "refs/heads/main")
	assertKind(t, err, KindRemoteUnavailable)

	_, err = c.RemoteDefaultBranch(ctx, repo, "nonexistent")
	assertKind(t, err, KindRemoteUnavailable)

	_, err = c.FetchBranch(ctx, repo, "origin", "refs/heads/absent-branch")
	assertKind(t, err, KindRefUnavailable)

	gitOut(t, r.Invocation, "remote", "add", "broken", filepath.Join(t.TempDir(), "no-such.git"))
	_, err = c.FetchBranch(ctx, repo, "broken", "refs/heads/main")
	assertKind(t, err, KindCommandFailed)
}

// TestRefsValidationBlocksSmuggling proves ref/remote validation rejects
// unqualified, option-shaped, pathspec-magic, and non-branch inputs with
// invalid-request before any git process starts. The client is a "dump" helper:
// had a spawn happened, the flow would surface a different kind (invalid-output
// from the bogus rev-parse answer), so an invalid-request verdict proves the
// short-circuit.
func TestRefsValidationBlocksSmuggling(t *testing.T) {
	c := helperClient(t, "dump")
	ctx := context.Background()
	repo := Repository{PrimaryWorktree: t.TempDir(), CommonDir: t.TempDir()}

	for _, br := range []RefName{"main", "-o", ":(top)x", "refs/tags/v1", "heads/main", "refs/heads/"} {
		_, err := c.FetchBranch(ctx, repo, "origin", br)
		assertKind(t, err, KindInvalidRequest)
	}
	for _, rem := range []RemoteName{"-o", "or/igin", ""} {
		_, err := c.RemoteDefaultBranch(ctx, repo, rem)
		assertKind(t, err, KindInvalidRequest)
		_, err = c.FetchBranch(ctx, repo, rem, "refs/heads/main")
		assertKind(t, err, KindInvalidRequest)
	}
	_, err := c.ResolveRef(ctx, repo, "main")
	assertKind(t, err, KindInvalidRequest)
}

// TestFetchFailureClassificationSharesOneNetworkBudget proves a failed fetch and
// the ls-remote probe that classifies it are bounded by one shared network
// budget, not one each. The fake remote burns most of the budget answering
// `fetch` non-zero and then hangs on the probe; with an unshared budget the
// probe would start a second full networkTimeout, so the operation would cost
// the caller roughly the fetch's delay plus a whole extra timeout.
func TestFetchFailureClassificationSharesOneNetworkBudget(t *testing.T) {
	const networkTimeout = 2 * time.Second
	c := helperClient(t, "fetchslowfail", "GITCLI_HELPER_FETCH_SLEEP_MS=1500")
	repo := Repository{PrimaryWorktree: t.TempDir()}

	// Calibrate one process spawn on this machine and toolchain: a
	// race-instrumented test binary re-execs far slower than a plain one, and
	// FetchBranch pays that cost per git process on top of the budget under
	// test. Without calibrating, the threshold below is a machine-speed
	// assertion rather than a budget-sharing one.
	// ResolveRef is exactly one spawn against this fake git and returns
	// immediately (the canned stdout is not an object id), so it times the
	// re-exec and nothing else.
	spawnStart := time.Now()
	if _, err := c.ResolveRef(context.Background(), repo, "refs/heads/main"); err == nil {
		t.Fatal("calibration call unexpectedly succeeded")
	}
	spawnCost := time.Since(spawnStart)

	start := time.Now()
	_, err := c.FetchBranch(context.Background(), repo, "origin", "refs/heads/main")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a failure from the unreachable remote")
	}
	// The shared budget is exhausted by the fetch, so the probe cannot run and
	// the operation surfaces as timed out.
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected a *Failure, got %v", err)
	}
	if f.Kind != KindTimedOut {
		t.Fatalf("kind = %q, want %q", f.Kind, KindTimedOut)
	}
	// One shared budget caps the two network processes at networkTimeout; an
	// unshared one would spend the fetch's 1.5s and then a whole further
	// networkTimeout on the probe. The midpoint between those two outcomes is
	// the line, plus the two spawns FetchBranch pays for beyond the budget.
	limit := networkTimeout + networkTimeout/2 + 2*spawnCost
	if elapsed >= limit {
		t.Fatalf("fetch failure cost more than one network budget: elapsed %v, limit %v (spawn %v)", elapsed, limit, spawnCost)
	}
}
