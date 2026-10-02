//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_runcompletion.sh (prefix ^TestIntegrationRunCompletion).

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// seedSettleablePair journals a settleable pair on the fixture's run: an uncertain
// workspace publish followed by a verified completed identical retry.
func seedSettleablePair(t *testing.T, repo, key string) {
	t.Helper()
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
}

// runRecordBytes reads the durable run record file verbatim.
func runRecordBytes(t *testing.T, repo, key string) []byte {
	t.Helper()
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(common, "docket", runTrackerDirName, key, runRecordFileName))
	if err != nil {
		t.Fatalf("read run record: %v", err)
	}
	return b
}

// TestIntegrationRunCompletionVerdictRunCompleteSettlesUncertainPublication (change 0444 acceptance 3): the
// REAL keyed verdict over a settleable pair settles the original through the
// attributed closeout and ends in run-done run-complete, with the original durably
// completed and the run completed. Without settlement the uncertain original would
// block the closeout (completion-unaccounted), so run-done proves the settle ran on
// this path.
func TestIntegrationRunCompletionVerdictRunCompleteSettlesUncertainPublication(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	seedSettleablePair(t, fx.repo, fx.key)

	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (findings %v)", got, want, res.CompletionFindings)
	}
	if countFinding(res.CompletionFindings, "mutation-settled:"+OperationWorkspacePublish) != 1 {
		t.Fatalf("CompletionFindings = %v, want exactly one mutation-settled:%s",
			res.CompletionFindings, OperationWorkspacePublish)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCompleted {
		t.Fatalf("run state = %q, want completed", ep.State)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatalf("original status = %q, want durably completed by the keyed verdict", ep.AdmittedMutations[0].Status)
	}
}

// TestIntegrationRunCompletionReadOnlyVerdictPathsNeverSettleSettleablePair (change 0444 acceptance 3): the
// unattributed observe verdict and RunVerify, run over the same settleable pair on an
// ACTIVE and on a COMPLETING run, leave the run record byte-identical — neither
// is an authorized settlement writer.
func TestIntegrationRunCompletionReadOnlyVerdictPathsNeverSettleSettleablePair(t *testing.T) {
	for _, state := range []string{"active", "completing"} {
		t.Run(state, func(t *testing.T) {
			fx := newVerdictCompletionFixture(t)
			seedSettleablePair(t, fx.repo, fx.key)
			if state == "completing" {
				if _, err := FenceRunCompleting(fx.repo, fx.key); err != nil {
					t.Fatalf("FenceRunCompleting: %v", err)
				}
			}
			before := runRecordBytes(t, fx.repo, fx.key)

			obs := RunVerdictObserve(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, []string{"3"})
			if got, want := obs.HumanText(), "run-observe run-complete 3"; got != want {
				t.Fatalf("observe HumanText = %q, want %q", got, want)
			}
			if after := runRecordBytes(t, fx.repo, fx.key); !bytes.Equal(before, after) {
				t.Fatal("the unattributed observe verdict wrote the run record")
			}

			v := RunVerify(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, RunVerifyRequest{ID: 3})
			if v.Verdict != VerdictRunComplete {
				t.Fatalf("RunVerify verdict = %q, want %q", v.Verdict, VerdictRunComplete)
			}
			if after := runRecordBytes(t, fx.repo, fx.key); !bytes.Equal(before, after) {
				t.Fatal("RunVerify wrote the run record")
			}
			ep, _, err := LoadRunRecord(fx.repo, fx.key)
			if err != nil {
				t.Fatalf("LoadRunRecord: %v", err)
			}
			if ep.AdmittedMutations[0].Status != mutationStatusUncertain {
				t.Fatalf("original status = %q, want still uncertain", ep.AdmittedMutations[0].Status)
			}
		})
	}
}
