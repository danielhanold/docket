//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_finalizeops.sh (prefix ^TestIntegrationFinalizeOps).

import (
	"context"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
)

func TestIntegrationFinalizeOpsFinalizeBlockUnrelatedInvalidRecordProgress(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModeDocket(), "in-progress")
	f.repo.writerAdvance(t, f.branch, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes})
	gh := &fakeBlockGitHub{repo: retargetRepo(), commentOutcome: githubcli.CommentCreated, commentURL: "https://example.test/c/9"}

	got := FinalizeBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation, blockTestRequest(f))
	if got.Result != ResultApplied || got.Disposition != BlockDispRecorded {
		t.Fatalf("finalize block beside an unrelated unparseable record = %q disp %q reason %q (findings %v), want applied recorded",
			got.Result, got.Disposition, got.Reason, got.Findings)
	}
	assertUnrelatedBrokenIntact(t, f.repo)
}

func TestIntegrationFinalizeOpsFinalizeClearBlockUnrelatedInvalidRecordProgress(t *testing.T) {
	f := setupBlockedFixture(t, planRepoModeDocket())
	f.repo.writerAdvance(t, f.branch, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes})
	gh := &fakeBlockGitHub{repo: retargetRepo(),
		openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {f.prForHead(f.head, greenEvidenceFor(t, f.head))}}}

	got := FinalizeClearBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation,
		ClearBlockRequest{ID: f.id, Revision: f.revision, Head: f.head, PRNumber: 1})
	if got.Result != ResultApplied || got.Disposition != BlockDispCleared {
		t.Fatalf("finalize clear-block beside an unrelated unparseable record = %q disp %q reason %q (findings %v), want applied cleared",
			got.Result, got.Disposition, got.Reason, got.Findings)
	}
	assertUnrelatedBrokenIntact(t, f.repo)
}

func TestIntegrationFinalizeOpsFinalizeBlockUnrelatedInvalidRecordRefusals(t *testing.T) {
	id, slug := rebaseFixtureID, rebaseFixtureSlug
	recPath := groomPath(id, slug)
	cases := unrelatedRefusalCases(t, id, recPath, lifecycleChange(id, slug, "in-progress"), lifecycleChange(id, "dupe", "in-progress"))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupRebaseFixtureStatus(t, planRepoModeDocket(), "in-progress")
			f.repo.writerAdvance(t, f.branch, c.files)
			f.revision = blobRevisionAt(t, f.repo.origin, f.branch, recPath)
			tip := originTip(t, f.repo.origin, f.branch)
			gh := &fakeBlockGitHub{repo: retargetRepo(), commentOutcome: githubcli.CommentCreated, commentURL: "https://example.test/c/9"}

			got := FinalizeBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation, blockTestRequest(f))
			if got.Result == ResultApplied {
				t.Fatalf("finalize block applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, got.Reason, got.Findings)
			if after := originTip(t, f.repo.origin, f.branch); after != tip {
				t.Errorf("a refused finalize block moved the metadata branch %s -> %s", tip, after)
			}
		})
	}
}
