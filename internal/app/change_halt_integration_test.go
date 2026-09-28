//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
)

func TestIntegrationRecordOpsChangeHaltUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	repo := newWorkingRepo(t, map[string]string{
		recPath:             lifecycleChange(id, "widget", "in-progress"),
		unrelatedBrokenPath: unrelatedBrokenBytes,
	})
	node := planningDepsFor(t, repo.invocation)

	res := ChangeHalt(context.Background(), node.deps, node.dir, HaltRequest{
		ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath), Report: "Blocked on infra; see run 7.\n",
	})
	if res.Result != ResultApplied || res.Disposition != HaltDispHalted {
		t.Fatalf("halt beside an unrelated unparseable record = %q disp %q reason %q (findings %v), want applied halted",
			res.Result, res.Disposition, res.Reason, res.Findings)
	}
	rec, _ := originFile(t, repo.origin, "docket", recPath)
	if !strings.Contains(rec, "## Run halted") {
		t.Errorf("halted record on origin lacks the marker:\n%s", rec)
	}
	assertUnrelatedBrokenIntact(t, repo)
}

func TestIntegrationRecordOpsChangeHaltUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, c := range unrelatedRefusalCases(t, id, recPath, lifecycleChange(id, "widget", "in-progress"), lifecycleChange(id, "dupe", "in-progress")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeHalt(context.Background(), node.deps, node.dir, HaltRequest{
				ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath), Report: "Paused.\n",
			})
			if res.Result == ResultApplied {
				t.Fatalf("halt applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, res.Reason, res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused halt moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}

func TestIntegrationRecordOpsChangeResumeHaltedUnrelatedInvalidRecordProgress(t *testing.T) {
	f := setupHaltedFixture(t, planRepoModes()[0])
	advanceDocketOrigin(t, f.repo, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes})
	recPath := groomPath(f.id, f.slug)

	got := ChangeResumeHalted(context.Background(), f.deps,
		WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
		ResumeRequest{ID: f.id, Version: blobVersionAt(t, f.repo.origin, "docket", recPath), AcknowledgeQuiescent: true})
	if got.Result != ResultApplied || got.Disposition != HaltDispResumed {
		t.Fatalf("resume-halted beside an unrelated unparseable record = %q disp %q reason %q (findings %v), want applied resumed",
			got.Result, got.Disposition, got.Reason, got.Findings)
	}
	rec, _ := originFile(t, f.repo.origin, "docket", recPath)
	if strings.Contains(rec, "## Run halted") {
		t.Errorf("marker not removed on resume:\n%s", rec)
	}
	assertUnrelatedBrokenIntact(t, f.repo)
}

func TestIntegrationRecordOpsChangeResumeHaltedUnrelatedInvalidRecordRefusals(t *testing.T) {
	cases := unrelatedRefusalCases(t, rebaseFixtureID, groomPath(rebaseFixtureID, rebaseFixtureSlug),
		haltedRecord(rebaseFixtureID, rebaseFixtureSlug), lifecycleChange(rebaseFixtureID, "dupe", "in-progress"))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupHaltedFixture(t, planRepoModes()[0])
			advanceDocketOrigin(t, f.repo, c.files)
			recPath := groomPath(f.id, f.slug)
			tip := originTip(t, f.repo.origin, "docket")

			got := ChangeResumeHalted(context.Background(), f.deps,
				WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
				ResumeRequest{ID: f.id, Version: blobVersionAt(t, f.repo.origin, "docket", recPath), AcknowledgeQuiescent: true})
			if got.Result == ResultApplied {
				t.Fatalf("resume-halted applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, got.Reason, got.Findings)
			if now := originTip(t, f.repo.origin, "docket"); now != tip {
				t.Errorf("a refused resume-halted moved the metadata branch %s -> %s", tip, now)
			}
		})
	}
}
