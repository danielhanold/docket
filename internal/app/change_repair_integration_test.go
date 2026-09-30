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

func TestIntegrationRecordOpsRepairIdentityUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newWorkingRepo(t, map[string]string{
		recPath:             repairRecord(3, "widget", ""),
		unrelatedBrokenPath: unrelatedBrokenBytes,
	})
	repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})

	res := repairRealRun(t, repo, recPath)
	if res.Result != ResultApplied || res.Branch != "feat/renamed" {
		t.Fatalf("repair-identity beside an unrelated unparseable record = %q reason %q branch %q (findings %v), want applied feat/renamed",
			res.Result, res.Reason, res.Branch, res.Findings)
	}
	rec, _ := originFile(t, repo.origin, "docket", recPath)
	if !strings.Contains(rec, "branch: 'feat/renamed'") {
		t.Errorf("repaired record on origin does not carry the adopted branch:\n%s", rec)
	}
	assertUnrelatedBrokenIntact(t, repo)
}

func TestIntegrationRecordOpsRepairIdentityUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	for _, c := range unrelatedRefusalCases(t, 3, recPath, repairRecord(3, "widget", ""), repairRecord(3, "dupe", "")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})
			tip := originTip(t, repo.origin, "docket")

			res := repairRealRun(t, repo, recPath)
			if res.Result == ResultApplied {
				t.Fatalf("repair-identity applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, res.Reason, res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused repair-identity moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}

// TestIntegrationRecordOpsRepairAdoptPRHeadAppliesOnMalformedRecordedBranch proves the PR-case
// remedy status prints for a branch-malformed record (change 0454) actually
// applies: adopting the PR head over a recorded branch: git would reject lands
// applied, because recordedBranch refuses the malformed name and the workspace
// gate then skips the branch-keyed inspection instead of failing inside git.
// The workspace seam is the real service, so an inspection of the malformed
// name would reach gitcli and fail there. feat/a:b is the discriminating row —
// only the delegated gitcli predicate rejects it; without the delegation the
// gate inspects it and refuses as workspace-conflict.
func TestIntegrationRecordOpsRepairAdoptPRHeadAppliesOnMalformedRecordedBranch(t *testing.T) {
	requireRealGit(t)
	for _, recorded := range []string{"feat/a..parent", "feat/a:b"} {
		t.Run(recorded, func(t *testing.T) {
			recPath := groomPath(3, "widget")
			repo := newWorkingRepo(t, map[string]string{recPath: repairRecord(3, "widget", recorded)})
			repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})

			node := planningDepsFor(t, repo.invocation)
			svc, err := workspace.NewService(node.deps.Client)
			if err != nil {
				t.Fatalf("workspace.NewService: %v", err)
			}
			deps := FinalizeDeps{Planning: node.deps, GitHub: repairGitHub("feat/renamed"), Workspace: svc}
			res := RepairIdentity(context.Background(), deps, node.dir, RepairIdentityRequest{
				ID: 3, ExpectRevision: blobVersionAt(t, repo.origin, "docket", recPath),
				AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/renamed",
			})
			if res.Result != ResultApplied || res.Branch != "feat/renamed" {
				t.Fatalf("adopt-pr-head over recorded branch %q = %q reason %q branch %q (msg %q, findings %v), want applied feat/renamed",
					recorded, res.Result, res.Reason, res.Branch, res.Message, res.Findings)
			}
			rec, _ := originFile(t, repo.origin, "docket", recPath)
			if !strings.Contains(rec, "branch: 'feat/renamed'") {
				t.Errorf("repaired record on origin does not carry the adopted branch:\n%s", rec)
			}
		})
	}
}
