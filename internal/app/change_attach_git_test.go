package app

import (
	"context"
	"fmt"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"testing"
)

// symlinkRepoFile creates a symlink at a repo-relative path (creating parents)
// pointing at target, so a committed artifact can be a symlink (mode 120000).
func symlinkRepoFile(t *testing.T, root, rel, target string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
}

// These are the real-git pre-transaction verification tests for change
// attach-plan: they drive the real ChangeAttachPlan operation over a real
// prepared workspace and a real bare metadata remote, so the from-Git guards
// (head, descent, single-artifact delta, trailer, tracked/regular file,
// balanced-and-targeted backlink, no placeholder token) are exercised end-to-end
// rather than faked. The guard table is a MUTATION test: every row is the happy
// fixture with exactly one property corrupted, and each asserts its own stable
// reason string — proof the guard reddens for the reason it names, not merely
// that something failed (learning assert-pins-outcome-not-mechanism).

// attachBacklinkBlock renders the docket:backlink block the operation expects at
// the head of an artifact, targeting change id/title at recPath. It mirrors
// render.BacklinkContent's repo-relative shape exactly (no RepoWebURL is
// configured in these fixtures), so a happy plan round-trips through verification.
func attachBacklinkBlock(id int, title, recPath string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		fmt.Sprintf("> ↩ **Change %04d — %s** — `%s`\n", id, title, recPath) +
		"<!-- docket:backlink:end -->\n"
}

// attachHappyPlan renders a well-formed plan artifact: the correct backlink plus
// an authored body carrying no placeholder token.
func attachHappyPlan(id int, title, recPath string) string {
	return attachBacklinkBlock(id, title, recPath) + "\n# Implementation Plan\n\nConcrete steps here.\n"
}

// attachSetup builds a main-mode repo with one in-progress change, prepares its
// feature workspace against the resolved base, and returns everything a
// verification row needs. The workspace sits on the feature ref at the base tip;
// a row commits its own plan variant, then runs ChangeAttachPlan.
type attachFixture struct {
	ctx        context.Context
	deps       PlanningDeps
	wdeps      WorkspaceDeps
	repo       *gitRepo
	invocation string
	wp         string // feature workspace path
	id         int
	slug       string
	recPath    string
	planPath   string
	version    string
	base       string
}

func attachSetup(t *testing.T) *attachFixture {
	t.Helper()
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	repo := newWorkingRepo(t, map[string]string{
		recPath: lifecycleChange(id, slug, "in-progress"),
	})
	version := blobVersionAt(t, repo.origin, "docket", recPath)

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ctx := context.Background()

	prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
	if prep.Result != ResultApplied {
		t.Fatalf("prepare workspace = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
	}
	return &attachFixture{
		ctx: ctx, deps: node.deps, wdeps: wdeps, repo: repo, invocation: repo.invocation,
		wp: prep.Path, id: id, slug: slug, recPath: recPath,
		planPath: "docs/superpowers/plans/2026-08-17-widget-plan.md",
		version:  version, base: prep.BaseCommit,
	}
}

// reset returns the feature workspace to a clean checkout of the prepared base.
func (f *attachFixture) reset(t *testing.T) {
	t.Helper()
	runGit(t, f.wp, "reset", "-q", "--hard", f.base)
	runGit(t, f.wp, "clean", "-fdq")
}

// commitPlan writes files into the workspace and commits them, optionally adding
// the plan-path trailer, and returns the new head.
func (f *attachFixture) commitPlan(t *testing.T, files map[string]string, trailerPath string) string {
	t.Helper()
	for rel, content := range files {
		writeRepoFile(t, f.wp, rel, content)
	}
	runGit(t, f.wp, "add", "-A")
	args := []string{"commit", "-q", "-m", "write plan"}
	if trailerPath != "" {
		args = append(args, "--trailer", "Docket-Plan-Path: "+trailerPath)
	}
	runGit(t, f.wp, args...)
	return runGit(t, f.wp, "rev-parse", "HEAD")
}
