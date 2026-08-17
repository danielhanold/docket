package app

import (
	"context"
	"testing"

	"github.com/danielhanold/docket/internal/workspace"
)

// These are the real-git integration tests for the workspace prepare/inspect/
// publish app operations: they drive the operations through a real
// workspace.Service over the same bare-remote temporary topologies the planning
// integration tests use (newMainModeRepo / newDocketModeRepo via planRepoModes),
// so the ownership-safe allocation, resume, and CAS-lease publish behavior is
// exercised end-to-end rather than faked. The change record lives on the mode's
// metadata branch; the change resolves its effective base to the integration
// branch (main), so a fresh workspace is created from origin/main's tip.

// TestWorkspaceOpsGitLifecycle walks one change through prepare (fresh + resume),
// inspect, and publish (create, fast-forward, and a contended divergence) against
// a real bare remote, in both metadata modes.
func TestWorkspaceOpsGitLifecycle(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	featureRef := "refs/heads/feat/" + slug

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: lifecycleChange(id, slug, "in-progress"),
			})
			version := blobVersionAt(t, repo.origin, m.branch, recPath)
			mainTip := originTip(t, repo.origin, "main")

			node := planningDepsFor(t, repo.invocation)
			deps := node.deps
			svc, err := workspace.NewService(deps.Client)
			if err != nil {
				t.Fatalf("workspace.NewService: %v", err)
			}
			wdeps := WorkspaceDeps{Service: svc}
			ctx := context.Background()

			// --- prepare a fresh workspace at the resolved base ----------------
			prep := WorkspacePrepare(ctx, deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
			if prep.Result != ResultApplied || prep.Disposition != string(workspace.PrepareCreated) {
				t.Fatalf("fresh prepare = (%q, %q), want applied/created (reason %q msg %q)", prep.Result, prep.Disposition, prep.Reason, prep.Message)
			}
			if prep.BaseCommit != mainTip {
				t.Errorf("prepared base = %q, want origin main tip %q (a real fetch of the resolved base)", prep.BaseCommit, mainTip)
			}
			if prep.Path == "" || prep.Head == "" {
				t.Fatalf("prepared workspace missing path/head: %+v", prep)
			}
			workspacePath := prep.Path
			createdHead := prep.Head

			// --- inspect reports the ready state ------------------------------
			insp := WorkspaceInspect(ctx, deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id})
			if insp.Result != ResultApplied || insp.State != string(workspace.StateReady) {
				t.Fatalf("inspect = (%q, state %q), want applied/ready (reason %q)", insp.Result, insp.State, insp.Reason)
			}
			if insp.Head != createdHead {
				t.Errorf("inspect head = %q, want the created head %q", insp.Head, createdHead)
			}

			// --- resuming yields the existing disposition ---------------------
			resume := WorkspacePrepare(ctx, deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
			if resume.Disposition != string(workspace.PrepareExisting) {
				t.Fatalf("second prepare disposition = %q, want existing", resume.Disposition)
			}

			// --- publish the current head: creates the absent remote ref ------
			pub := WorkspacePublish(ctx, deps, wdeps, repo.invocation, WorkspacePublishRequest{ID: id, Head: createdHead})
			if pub.Result != ResultApplied || pub.Disposition != string(workspace.PublishPublished) {
				t.Fatalf("publish = (%q, %q), want applied/published (reason %q msg %q)", pub.Result, pub.Disposition, pub.Reason, pub.Message)
			}
			if got := originTip(t, repo.origin, featureRef); got != createdHead {
				t.Errorf("remote feature ref = %q, want the exact published head %q", got, createdHead)
			}

			// --- advance the head and publish the fast-forward ----------------
			writeRepoFile(t, workspacePath, "feature.txt", "feature work\n")
			runGit(t, workspacePath, "add", "-A")
			runGit(t, workspacePath, "commit", "-q", "-m", "feature work")
			advancedHead := runGit(t, workspacePath, "rev-parse", "HEAD")
			pub2 := WorkspacePublish(ctx, deps, wdeps, repo.invocation, WorkspacePublishRequest{ID: id, Head: advancedHead})
			if pub2.Result != ResultApplied || pub2.Disposition != string(workspace.PublishPublished) {
				t.Fatalf("fast-forward publish = (%q, %q), want applied/published", pub2.Result, pub2.Disposition)
			}
			if got := originTip(t, repo.origin, featureRef); got != advancedHead {
				t.Errorf("remote feature ref = %q after ff, want %q", got, advancedHead)
			}

			// --- diverge the remote, then publish → contended, remote untouched
			// An independent writer force-pushes a divergent commit onto the same
			// feature ref; the local head then advances along its own line, so the
			// remote commit is neither equal to nor an ancestor of it.
			runGit(t, repo.writer, "checkout", "-q", "-B", "feat/"+slug, "origin/main")
			writeRepoFile(t, repo.writer, "writer-side.txt", "independent writer\n")
			runGit(t, repo.writer, "add", "-A")
			runGit(t, repo.writer, "commit", "-q", "-m", "writer divergence")
			runGit(t, repo.writer, "push", "-q", "-f", "origin", "feat/"+slug)
			divergent := originTip(t, repo.origin, featureRef)

			writeRepoFile(t, workspacePath, "local2.txt", "more local work\n")
			runGit(t, workspacePath, "add", "-A")
			runGit(t, workspacePath, "commit", "-q", "-m", "more local work")
			localHead2 := runGit(t, workspacePath, "rev-parse", "HEAD")

			contended := WorkspacePublish(ctx, deps, wdeps, repo.invocation, WorkspacePublishRequest{ID: id, Head: localHead2})
			if contended.Result != ResultContended || contended.Disposition != string(workspace.PublishContended) {
				t.Fatalf("divergent publish = (%q, %q), want contended/contended", contended.Result, contended.Disposition)
			}
			if got := originTip(t, repo.origin, featureRef); got != divergent {
				t.Errorf("remote feature ref moved on a contended publish: %q -> %q (must be untouched, no force)", divergent, got)
			}
		})
	}
}
