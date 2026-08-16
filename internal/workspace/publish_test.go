package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// The PublishHead tests drive the idempotent feature-branch publication flow
// against real bare origins. PublishHead reinspects the owned ready workspace,
// refuses a dirty or inconsistent one, probes the authoritative remote feature
// ref, and reaches the exact local HEAD onto the exact remote ref under an
// absent-ref or expected-old lease — never a force, reset, merge, or rebase. The
// idempotency key is the remote state (the exact commit at the exact remote
// ref), never a clean tree, a local branch, or an upstream configuration.

// commitInWorkspace commits one file on the workspace's feature branch and
// returns the new HEAD, so a test can advance the branch past its base.
func commitInWorkspace(t *testing.T, ws, rel, content string) gitcli.ObjectID {
	t.Helper()
	writeWorktreeFile(t, ws, rel, content)
	gitOut(t, ws, "add", rel)
	gitOut(t, ws, "commit", "-q", "-m", "work: "+rel)
	return gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
}

// originFeatCommit returns the origin's feat/<slug> ref commit and whether it
// exists. Origin is bare, so it is read directly with rev-parse.
func originFeatCommit(t *testing.T, r *wsRepos) (gitcli.ObjectID, bool) {
	t.Helper()
	out, err := gitTry(r.Origin, "rev-parse", "--verify", "--quiet", string(prepFeatureRef()))
	if err != nil {
		return "", false
	}
	return gitcli.ObjectID(strings.TrimSpace(out)), true
}

// chmodTree recursively sets mode on dir and everything beneath it. Directories
// are chmod'd after their contents so a read-only parent does not block
// descent on the way down; on restore the parent must be writable first, so it
// is applied to dir itself last regardless.
func chmodTree(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	// Ensure directories are traversable/writable enough to walk when restoring.
	restore := mode&0o200 != 0
	if restore {
		_ = os.Chmod(dir, 0o700)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// On the read-only pass the dir may already be unreadable; that is fine.
		if restore {
			t.Fatalf("ReadDir %s: %v", dir, err)
		}
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			chmodTree(t, p, mode)
			continue
		}
		if err := os.Chmod(p, mode); err != nil && restore {
			t.Fatalf("chmod %s: %v", p, err)
		}
	}
	if err := os.Chmod(dir, mode); err != nil && restore {
		t.Fatalf("chmod %s: %v", dir, err)
	}
}

// publishHead runs PublishHead and returns the result plus error verbatim.
func publishHead(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) (PublishResult, error) {
	t.Helper()
	return svc.PublishHead(context.Background(), PublishRequest{Repository: repo, Remote: "origin", Target: tgt})
}

// TestPublishAbsentRefCreates proves an absent remote feature ref is created
// under the absent-ref lease: the disposition is published and the origin ref
// equals the exact local HEAD. Proven on both topologies.
func TestPublishAbsentRefCreates(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)
		head := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

		if _, ok := originFeatCommit(t, r); ok {
			t.Fatalf("fixture: origin feat ref already exists before publish")
		}
		res, err := publishHead(t, svc, repo, tgt)
		if err != nil {
			t.Fatalf("PublishHead: %v", err)
		}
		if res.Disposition != PublishPublished {
			t.Errorf("Disposition = %q; want published", res.Disposition)
		}
		if res.Head != head {
			t.Errorf("Head = %q; want local head %q", res.Head, head)
		}
		remote, ok := originFeatCommit(t, r)
		if !ok {
			t.Fatalf("origin feat ref absent after publish")
		}
		if remote != head {
			t.Errorf("origin feat ref = %q; want local head %q", remote, head)
		}
		if res.Remote != head {
			t.Errorf("result Remote = %q; want %q", res.Remote, head)
		}
	})
}

// TestPublishRepeatAlreadyPublished proves a second PublishHead with the remote
// already at the local HEAD returns already-published and performs no second
// update: the origin ref value is unchanged.
func TestPublishRepeatAlreadyPublished(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)
		head := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

		if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
			t.Fatalf("first PublishHead = %q err=%v; want published", res.Disposition, err)
		}
		before, ok := originFeatCommit(t, r)
		if !ok {
			t.Fatalf("origin feat ref absent after first publish")
		}

		res, err := publishHead(t, svc, repo, tgt)
		if err != nil {
			t.Fatalf("second PublishHead: %v", err)
		}
		if res.Disposition != PublishAlreadyPublished {
			t.Errorf("Disposition = %q; want already-published", res.Disposition)
		}
		if res.Remote != head {
			t.Errorf("Remote = %q; want %q", res.Remote, head)
		}
		after, _ := originFeatCommit(t, r)
		if after != before {
			t.Errorf("origin feat ref changed on already-published: before=%q after=%q", before, after)
		}
	})
}

// TestPublishFastForward proves an existing remote ref that is an ancestor of
// the local HEAD is fast-forwarded under an expected-old lease: after a first
// publish and a further local commit, a second publish moves the origin ref to
// the new HEAD.
func TestPublishFastForward(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)

	head1 := commitInWorkspace(t, ws, "feature.txt", "first\n")
	if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
		t.Fatalf("first publish = %q err=%v; want published", res.Disposition, err)
	}
	if got, _ := originFeatCommit(t, r); got != head1 {
		t.Fatalf("origin after first publish = %q; want %q", got, head1)
	}

	head2 := commitInWorkspace(t, ws, "feature.txt", "first\nsecond\n")
	res, err := publishHead(t, svc, repo, tgt)
	if err != nil {
		t.Fatalf("second PublishHead: %v", err)
	}
	if res.Disposition != PublishPublished {
		t.Errorf("Disposition = %q; want published (fast-forward)", res.Disposition)
	}
	if res.Head != head2 {
		t.Errorf("Head = %q; want %q", res.Head, head2)
	}
	if got, _ := originFeatCommit(t, r); got != head2 {
		t.Errorf("origin after fast-forward = %q; want %q", got, head2)
	}
}

// TestPublishDivergentContended proves a remote ref holding a commit that is
// neither equal to nor an ancestor of the local HEAD is refused as contended:
// PublishHead never force-pushes, and the origin ref keeps the interloper.
func TestPublishDivergentContended(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	localHead := commitInWorkspace(t, ws, "feature.txt", "local work\n")

	// The writer publishes a divergent commit onto the feature ref out-of-band.
	gitOut(t, r.Writer, "checkout", "-q", "-B", "feat/"+prepSlug, "origin/main")
	writeWorktreeFile(t, r.Writer, "divergent.txt", "divergent work\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "divergent")
	divergent := gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
	gitOut(t, r.Writer, "push", "-f", "-q", "origin", "feat/"+prepSlug)
	gitOut(t, r.Writer, "checkout", "-q", "main")

	res, err := publishHead(t, svc, repo, tgt)
	if err != nil {
		t.Fatalf("PublishHead: %v", err)
	}
	if res.Disposition != PublishContended {
		t.Errorf("Disposition = %q; want contended", res.Disposition)
	}
	if res.Head != localHead {
		t.Errorf("Head = %q; want %q", res.Head, localHead)
	}
	if res.Remote != divergent {
		t.Errorf("Remote = %q; want observed divergent %q", res.Remote, divergent)
	}
	// Origin still holds the interloper: no force push overwrote it.
	if got, _ := originFeatCommit(t, r); got != divergent {
		t.Errorf("origin feat ref = %q; want unchanged divergent %q", got, divergent)
	}
}

// TestPublishLostResponseAdopted proves the idempotency key is the remote state:
// a HEAD already pushed out-of-band (a lost push response) is adopted as
// already-published, not pushed again.
func TestPublishLostResponseAdopted(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)
		head := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

		// Simulate our own push whose response was lost: the ref is already at HEAD.
		gitOut(t, ws, "push", "-q", "origin", "feat/"+prepSlug)

		res, err := publishHead(t, svc, repo, tgt)
		if err != nil {
			t.Fatalf("PublishHead: %v", err)
		}
		if res.Disposition != PublishAlreadyPublished {
			t.Errorf("Disposition = %q; want already-published (adopted)", res.Disposition)
		}
		if res.Remote != head {
			t.Errorf("Remote = %q; want %q", res.Remote, head)
		}
	})
}

// TestPublishLocalProxiesNotConsulted repeats the lost-response case with a
// clean tree AND an upstream configured, proving PublishHead keys on remote
// equality and never on those local proxies (absent from its signature).
func TestPublishLocalProxiesNotConsulted(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	head := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

	gitOut(t, ws, "push", "-q", "origin", "feat/"+prepSlug)
	// Configure the upstream and confirm the tree is clean: pure local proxies.
	gitOut(t, ws, "branch", "--set-upstream-to=origin/feat/"+prepSlug, "feat/"+prepSlug)
	if status := gitOut(t, ws, "status", "--porcelain"); status != "" {
		t.Fatalf("fixture: workspace not clean:\n%s", status)
	}

	res, err := publishHead(t, svc, repo, tgt)
	if err != nil {
		t.Fatalf("PublishHead: %v", err)
	}
	if res.Disposition != PublishAlreadyPublished {
		t.Errorf("Disposition = %q; want already-published regardless of local proxies", res.Disposition)
	}
	if res.Remote != head {
		t.Errorf("Remote = %q; want %q", res.Remote, head)
	}
}

// TestPublishDirtyRefused proves a dirty workspace is refused as an invalid-state
// failure and the origin ref is untouched.
func TestPublishDirtyRefused(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	// An untracked file makes the workspace dirty.
	writeWorktreeFile(t, ws, "scratch.txt", "unsaved\n")

	res, err := publishHead(t, svc, repo, tgt)
	if err == nil {
		t.Fatalf("PublishHead on dirty workspace = nil error; want invalid-state failure")
	}
	if res.Disposition != PublishFailed {
		t.Errorf("Disposition = %q; want failed", res.Disposition)
	}
	f, ok := AsFailure(err)
	if !ok || f.Kind != KindInvalidState {
		t.Errorf("error = %v; want *Failure invalid-state", err)
	}
	if _, ok := originFeatCommit(t, r); ok {
		t.Errorf("origin feat ref created for a dirty workspace; must be untouched")
	}
}

// TestPublishDetachedRefused proves a workspace whose HEAD is detached (off the
// feature ref) is refused as an invalid-state failure with the origin untouched.
func TestPublishDetachedRefused(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	gitOut(t, ws, "checkout", "-q", "--detach", "HEAD")

	res, err := publishHead(t, svc, repo, tgt)
	if err == nil {
		t.Fatalf("PublishHead on detached HEAD = nil error; want invalid-state failure")
	}
	if res.Disposition != PublishFailed {
		t.Errorf("Disposition = %q; want failed", res.Disposition)
	}
	if f, ok := AsFailure(err); !ok || f.Kind != KindInvalidState {
		t.Errorf("error = %v; want *Failure invalid-state", err)
	}
	if _, ok := originFeatCommit(t, r); ok {
		t.Errorf("origin feat ref created for a detached workspace; must be untouched")
	}
}

// TestPublishUnprobeableRemoteUnknown proves an unobservable remote yields
// unknown with no fabricated Remote id. The remote URL is broken after
// preparation, so the authoritative probe cannot establish the remote state.
func TestPublishUnprobeableRemoteUnknown(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	head := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

	// Break the remote URL: the name stays configured, but ls-remote/push fail.
	gitOut(t, r.Primary, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "nonexistent.git"))

	res, err := publishHead(t, svc, repo, tgt)
	if err != nil {
		t.Fatalf("PublishHead: %v", err)
	}
	if res.Disposition != PublishUnknown {
		t.Errorf("Disposition = %q; want unknown", res.Disposition)
	}
	if res.Head != head {
		t.Errorf("Head = %q; want %q", res.Head, head)
	}
	if res.Remote != "" {
		t.Errorf("Remote = %q; want empty (no fabricated id)", res.Remote)
	}
}

// TestPublishPushFailsRefAbsentFailed drives the not-structurally-conclusive
// push branch: an unwritable origin makes the create push fail, and the
// re-probe shows the ref still cleanly absent — a definite failed, never a
// false published or a silent unknown.
func TestPublishPushFailsRefAbsentFailed(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	commitInWorkspace(t, ws, "feature.txt", "feature work\n")

	// Make the origin recursively read-only so a push cannot write objects while
	// ls-remote can still read the refs. Restored on cleanup so TempDir removal
	// succeeds.
	chmodTree(t, r.Origin, 0o500)
	t.Cleanup(func() { chmodTree(t, r.Origin, 0o700) })

	res, err := publishHead(t, svc, repo, tgt)
	if err == nil {
		t.Fatalf("PublishHead with unwritable origin = nil error; want failed")
	}
	if res.Disposition != PublishFailed {
		t.Errorf("Disposition = %q; want failed", res.Disposition)
	}
	if _, ok := originFeatCommit(t, r); ok {
		t.Errorf("origin feat ref exists; the push must not have landed")
	}
}
