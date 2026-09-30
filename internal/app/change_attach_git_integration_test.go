//go:build integration

package app

import (
	"context"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"strings"
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
	return attachSetupWith(t, nil)
}

// attachSetupWith is attachSetup with extra metadata files seeded beside the
// in-progress change (change 0449 seeds an unrelated unparseable record).
func attachSetupWith(t *testing.T, extra map[string]string) *attachFixture {
	t.Helper()
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	files := map[string]string{recPath: lifecycleChange(id, slug, "in-progress")}
	for rel, content := range extra {
		files[rel] = content
	}
	repo := newWorkingRepo(t, files)
	version := blobVersionAt(t, repo.origin, "docket", recPath)

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ctx := context.Background()

	prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Revision: version})
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

// --- 0449: unrelated invalid records never block a named attach -------------
// Shares the unrelated-broken-record fixtures with change_claim_test.go. The
// refusal rows inject their defect onto the origin AFTER the workspace is
// prepared, so the attach transaction itself is what must refuse.

// advanceDocketOrigin commits files onto the origin's metadata branch through
// the writer clone, first syncing the writer to the origin tip so an engine
// commit made since setup never turns the push into a non-fast-forward.
func advanceDocketOrigin(t *testing.T, repo *gitRepo, files map[string]string) {
	t.Helper()
	runGit(t, repo.writer, "fetch", "-q", "origin", "docket")
	runGit(t, repo.writer, "checkout", "-q", "-B", "docket", "FETCH_HEAD")
	for rel, content := range files {
		writeRepoFile(t, repo.writer, rel, content)
	}
	runGit(t, repo.writer, "add", "-A")
	runGit(t, repo.writer, "commit", "-q", "-m", "advance docket")
	runGit(t, repo.writer, "push", "-q", "origin", "docket")
}

func TestIntegrationRecordOpsChangeAttachUnrelatedInvalidRecordProgress(t *testing.T) {
	f := attachSetupWith(t, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes})
	head := f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)

	res := ChangeAttachPlan(f.ctx, f.deps, f.wdeps, f.invocation, ChangeAttachRequest{
		ID: f.id, Revision: blobVersionAt(t, f.repo.origin, "docket", f.recPath), Path: f.planPath, Commit: head,
	})
	if res.Result != ResultApplied {
		t.Fatalf("attach-plan beside an unrelated unparseable record = %q (reason %q findings %v), want applied",
			res.Result, res.Reason, res.Findings)
	}
	rec, _ := originFile(t, f.repo.origin, "docket", f.recPath)
	if !strings.Contains(rec, "plan: '"+f.planPath+"'") && !strings.Contains(rec, "plan: "+f.planPath) {
		t.Errorf("attached record on origin does not carry the plan path:\n%s", rec)
	}
	assertUnrelatedBrokenIntact(t, f.repo)
}

func TestIntegrationRecordOpsChangeAttachUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	src := lifecycleChange(3, "widget", "in-progress")
	cases := unrelatedRefusalCases(t, 3, groomPath(3, "widget"), src, lifecycleChange(3, "dupe", "in-progress"))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := attachSetupWith(t, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes})
			head := f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
			advanceDocketOrigin(t, f.repo, c.files)
			tip := originTip(t, f.repo.origin, "docket")

			res := ChangeAttachPlan(f.ctx, f.deps, f.wdeps, f.invocation, ChangeAttachRequest{
				ID: f.id, Revision: blobVersionAt(t, f.repo.origin, "docket", f.recPath), Path: f.planPath, Commit: head,
			})
			if res.Result == ResultApplied {
				t.Fatalf("attach-plan applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, res.Reason, res.Findings)
			if got := originTip(t, f.repo.origin, "docket"); got != tip {
				t.Errorf("a refused attach moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}
