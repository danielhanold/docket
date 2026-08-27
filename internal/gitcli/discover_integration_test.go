//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// canon is the test-side oracle for "absolute + every-symlink-hop resolved",
// mirroring what Discover must return so /var -> /private/var on macOS is
// exercised in every comparison.
func canon(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestDiscoverCanonicalIdentityAcrossWorktrees proves that for BOTH topologies,
// discovery from the primary root, a nested dir under it, each linked worktree
// (docket mode), and a path reached through an extra symlink all resolve to the
// SAME canonical primary worktree and common dir.
func TestIntegrationRepoDiscoverCanonicalIdentityAcrossWorktrees(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()

	check := func(t *testing.T, wantPrimary, wantCommon, path string) {
		t.Helper()
		repo, err := c.Discover(ctx, DiscoverOptions{InvocationPath: path})
		if err != nil {
			t.Fatalf("Discover(%q) error: %v", path, err)
		}
		if repo.PrimaryWorktree != wantPrimary {
			t.Errorf("Discover(%q) PrimaryWorktree = %q, want %q", path, repo.PrimaryWorktree, wantPrimary)
		}
		if repo.CommonDir != wantCommon {
			t.Errorf("Discover(%q) CommonDir = %q, want %q", path, repo.CommonDir, wantCommon)
		}
	}

	t.Run("main", func(t *testing.T) {
		r := newMainModeRepos(t)
		wantPrimary := canon(t, r.Invocation)
		wantCommon := canon(t, filepath.Join(wantPrimary, ".git"))

		link := filepath.Join(t.TempDir(), "linktoprimary")
		if err := os.Symlink(r.Invocation, link); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{
			r.Invocation,                                // (a) primary root
			filepath.Join(r.Invocation, "docs"),         // (b) nested tracked dir
			filepath.Join(r.Invocation, "docs/changes"), // (b) deeper nested dir
			link, // (e) via an extra symlink
		} {
			check(t, wantPrimary, wantCommon, p)
		}
	})

	t.Run("docket", func(t *testing.T) {
		r := newDocketModeRepos(t)
		wantPrimary := canon(t, r.Invocation)
		wantCommon := canon(t, filepath.Join(wantPrimary, ".git"))

		nested := filepath.Join(r.Invocation, "nested", "deep")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(t.TempDir(), "linktoprimary")
		if err := os.Symlink(r.Invocation, link); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{
			r.Invocation,                           // (a) primary root
			nested,                                 // (b) nested dir under primary
			filepath.Join(r.Invocation, ".docket"), // (c) .docket linked worktree
			filepath.Join(r.Invocation, ".worktrees/feat-x"), // (d) feature linked worktree
			link, // (e) via an extra symlink
		} {
			check(t, wantPrimary, wantCommon, p)
		}
	})
}

// TestDiscoverRejections asserts the typed failure kinds for an empty path
// (invalid-request), a missing path, a plain non-repo dir, and a bare repo
// (all invalid-repository).
func TestIntegrationRepoDiscoverRejections(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)

	_, err := c.Discover(ctx, DiscoverOptions{InvocationPath: ""})
	assertKind(t, err, KindInvalidRequest)

	_, err = c.Discover(ctx, DiscoverOptions{InvocationPath: filepath.Join(t.TempDir(), "does-not-exist")})
	assertKind(t, err, KindInvalidRepository)

	_, err = c.Discover(ctx, DiscoverOptions{InvocationPath: t.TempDir()})
	assertKind(t, err, KindInvalidRepository)

	_, err = c.Discover(ctx, DiscoverOptions{InvocationPath: r.Origin})
	assertKind(t, err, KindInvalidRepository)
}

// TestDiscoverInconsistentIdentityOutput drives Discover with a helper-process
// git whose rev-parse answer is a single bogus line, proving a malformed
// identity read is reported as invalid-output.
func TestIntegrationRepoDiscoverInconsistentIdentityOutput(t *testing.T) {
	c := helperClient(t, "script", "GITCLI_HELPER_STDOUT=bogus-single-line")
	_, err := c.Discover(context.Background(), DiscoverOptions{InvocationPath: t.TempDir()})
	assertKind(t, err, KindInvalidOutput)
}

// TestDiscoverIsReadOnly captures the invocation repo's full porcelain status
// and HEAD before and after Discover and requires them byte-identical.
func TestIntegrationRepoDiscoverIsReadOnly(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)

	beforeStatus := gitOut(t, r.Invocation, "status", "--porcelain", "-z")
	beforeHead := gitOut(t, r.Invocation, "rev-parse", "HEAD")

	if _, err := c.Discover(ctx, DiscoverOptions{InvocationPath: r.Invocation}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	afterStatus := gitOut(t, r.Invocation, "status", "--porcelain", "-z")
	afterHead := gitOut(t, r.Invocation, "rev-parse", "HEAD")

	if beforeStatus != afterStatus {
		t.Errorf("status changed by Discover:\nbefore %q\nafter  %q", beforeStatus, afterStatus)
	}
	if beforeHead != afterHead {
		t.Errorf("HEAD changed by Discover: before %q after %q", beforeHead, afterHead)
	}
}
