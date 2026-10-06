//go:build integration

package app

import (
	"context"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
)

// These are the real-git attach fixtures: they drive the real ChangeAttachPlan
// and ChangeAttachResults operations over a real prepared workspace and a real
// bare metadata remote. Both write their artifact on the metadata branch, so
// their guards (path containment, body shape, a field naming another path, an
// occupied path) are exercised against the remote; the prepared workspace is
// only there to prove the feature branch is never touched.
// Each refusal row asserts its own stable reason string — proof the guard
// reddens for the reason it names, not merely that something failed (learning
// assert-pins-outcome-not-mechanism).

// attachSetup builds a repo with one in-progress change, prepares its feature
// workspace against the resolved base, and returns everything a row needs. The
// workspace sits on the feature ref at the base tip.
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
	revision   string
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
	revision := blobRevisionAt(t, repo.origin, "docket", recPath)

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ctx := context.Background()

	prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Revision: revision})
	if prep.Result != ResultApplied {
		t.Fatalf("prepare workspace = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
	}
	return &attachFixture{
		ctx: ctx, deps: node.deps, wdeps: wdeps, repo: repo, invocation: repo.invocation,
		wp: prep.Path, id: id, slug: slug, recPath: recPath,
		planPath: "docs/superpowers/plans/2026-08-17-widget-plan.md",
		revision: revision, base: prep.BaseCommit,
	}
}

// reset returns the feature workspace to a clean checkout of the prepared base.
func (f *attachFixture) reset(t *testing.T) {
	t.Helper()
	runGit(t, f.wp, "reset", "-q", "--hard", f.base)
	runGit(t, f.wp, "clean", "-fdq")
}

// commitArtifact writes files into the feature workspace and commits them,
// returning the new head.
func (f *attachFixture) commitArtifact(t *testing.T, files map[string]string) string {
	t.Helper()
	for rel, content := range files {
		writeRepoFile(t, f.wp, rel, content)
	}
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", "write artifact")
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

	res := ChangeAttachPlan(f.ctx, f.deps, f.invocation, ChangeAttachRequest{
		ID: f.id, Revision: blobRevisionAt(t, f.repo.origin, "docket", f.recPath), Path: f.planPath,
		Markdown: []byte(attachHappyPlanBody()),
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
			advanceDocketOrigin(t, f.repo, c.files)
			tip := originTip(t, f.repo.origin, "docket")

			res := ChangeAttachPlan(f.ctx, f.deps, f.invocation, ChangeAttachRequest{
				ID: f.id, Revision: blobRevisionAt(t, f.repo.origin, "docket", f.recPath), Path: f.planPath,
				Markdown: []byte(attachHappyPlanBody()),
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
