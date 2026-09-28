//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"github.com/danielhanold/docket/internal/githubcli"
	"testing"
)

// TestIntegrationRecordOpsMarkImplementedAcceptsSkippedEvidence: a build.gate: off repository marks a
// change implemented on truthful skipped evidence certifying the exact head. The
// evidence conjunct accepts VerdictSkipped exactly as VerdictVerified; the happy
// fixture is TestIntegrationChangeRuntimeMarkImplementedAppliesEndToEnd with skipped
// evidence substituted.
func TestIntegrationRecordOpsMarkImplementedAcceptsSkippedEvidence(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := miAdvanceHead(t, repo)
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"

	deps, wdeps, gdeps, inv, req, _ := buildMI(t, client, repo.invocation, miKit{
		reconciled: true, plan: miPlanPath(), results: miResultsPath, version: miVersion, reqVersion: miVersion,
		reqHead: head, localHead: head, evidence: prSkippedEvidenceBytes(t, head),
		probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: pr,
	})

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied — skipped evidence at the exact head must certify implemented (findings %v)", res.Result, res.Findings)
	}
}

func TestIntegrationRecordOpsMarkImplementedUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, miSlug)
	repo := newWorkingRepo(t, map[string]string{
		recPath:             miRecord(3, miSlug, miPlanPath(), miResultsPath, true, false),
		unrelatedBrokenPath: unrelatedBrokenBytes,
	})
	head := miAdvanceHead(t, repo)

	res := miRealRun(t, repo, recPath, head)
	if res.Result != ResultApplied || res.Status != "implemented" {
		t.Fatalf("mark-implemented beside an unrelated unparseable record = %q status %q (findings %v), want applied implemented",
			res.Result, res.Status, res.Findings)
	}
	assertUnrelatedBrokenIntact(t, repo)
}

func TestIntegrationRecordOpsMarkImplementedUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, miSlug)
	src := miRecord(3, miSlug, miPlanPath(), miResultsPath, true, false)
	for _, c := range unrelatedRefusalCases(t, 3, recPath, src, lifecycleChange(3, "dupe", "in-progress")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			head := miAdvanceHead(t, repo)
			tip := originTip(t, repo.origin, "docket")

			res := miRealRun(t, repo, recPath, head)
			if res.Result == ResultApplied {
				t.Fatalf("mark-implemented applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, "", res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused mark-implemented moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}
