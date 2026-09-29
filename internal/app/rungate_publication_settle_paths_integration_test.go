//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_gatecompletion.sh (prefix ^TestIntegrationGateCompletion).

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// seedSettleablePair journals a settleable pair on the fixture's epoch: an uncertain
// workspace publish followed by a verified completed identical retry.
func seedSettleablePair(t *testing.T, repo, key string) {
	t.Helper()
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
}

// epochRecordBytes reads the durable epoch record file verbatim.
func epochRecordBytes(t *testing.T, repo, key string) []byte {
	t.Helper()
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(common, "docket", runTrackerDirName, key, epochRecordFileName))
	if err != nil {
		t.Fatalf("read epoch record: %v", err)
	}
	return b
}

// TestIntegrationGateCompletionVerdictRunCompleteSettlesUncertainPublication (change 0444 acceptance 3): the
// REAL keyed verdict over a settleable pair settles the original through the
// attributed closeout and ends in gate-done run-complete, with the original durably
// completed and the epoch completed. Without settlement the uncertain original would
// block the closeout (completion-unaccounted), so gate-done proves the settle ran on
// this path.
func TestIntegrationGateCompletionVerdictRunCompleteSettlesUncertainPublication(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	seedSettleablePair(t, fx.repo, fx.key)

	res := RunGateVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "gate-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (findings %v)", got, want, res.CompletionFindings)
	}
	if countFinding(res.CompletionFindings, "mutation-settled:"+OperationWorkspacePublish) != 1 {
		t.Fatalf("CompletionFindings = %v, want exactly one mutation-settled:%s",
			res.CompletionFindings, OperationWorkspacePublish)
	}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.State != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", ep.State)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatalf("original status = %q, want durably completed by the keyed verdict", ep.AdmittedMutations[0].Status)
	}
}

// TestIntegrationGateCompletionReadOnlyVerdictPathsNeverSettleSettleablePair (change 0444 acceptance 3): the
// unattributed observe verdict and RunVerify, run over the same settleable pair on an
// ACTIVE and on a COMPLETING epoch, leave the epoch record byte-identical — neither
// is an authorized settlement writer.
func TestIntegrationGateCompletionReadOnlyVerdictPathsNeverSettleSettleablePair(t *testing.T) {
	for _, state := range []string{"active", "completing"} {
		t.Run(state, func(t *testing.T) {
			fx := newVerdictCompletionFixture(t)
			seedSettleablePair(t, fx.repo, fx.key)
			if state == "completing" {
				if _, err := FenceEpochCompleting(fx.repo, fx.key, ""); err != nil {
					t.Fatalf("FenceEpochCompleting: %v", err)
				}
			}
			before := epochRecordBytes(t, fx.repo, fx.key)

			obs := RunGateVerdictObserve(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, []string{"3"})
			if got, want := obs.HumanText(), "gate-observe run-complete 3"; got != want {
				t.Fatalf("observe HumanText = %q, want %q", got, want)
			}
			if after := epochRecordBytes(t, fx.repo, fx.key); !bytes.Equal(before, after) {
				t.Fatal("the unattributed observe verdict wrote the epoch record")
			}

			v := RunVerify(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, RunVerifyRequest{ID: 3})
			if v.Verdict != VerdictRunComplete {
				t.Fatalf("RunVerify verdict = %q, want %q", v.Verdict, VerdictRunComplete)
			}
			if after := epochRecordBytes(t, fx.repo, fx.key); !bytes.Equal(before, after) {
				t.Fatal("RunVerify wrote the epoch record")
			}
			ep, _, err := LoadEpochRecord(fx.repo, fx.key)
			if err != nil {
				t.Fatalf("LoadEpochRecord: %v", err)
			}
			if ep.AdmittedMutations[0].Status != mutationStatusUncertain {
				t.Fatalf("original status = %q, want still uncertain", ep.AdmittedMutations[0].Status)
			}
		})
	}
}
