package app

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// These tests pin change 0444 acceptance 3 at the real entry points: settlement of a
// retry-proven uncertain publication happens through the attributed keyed verdict
// (RunGateVerdict → gateCompleteRun → completeSuccessfulRun) and cancellation
// teardown ONLY; the unattributed observe verdict and RunVerify over the very same
// settleable pair write nothing. TestSettleUncertainPublicationsAuthorizedCallers
// pins the write to its two authorized callers by deriving every reference from
// source.

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
	b, err := os.ReadFile(filepath.Join(common, "docket", "rungate", key, epochRecordFileName))
	if err != nil {
		t.Fatalf("read epoch record: %v", err)
	}
	return b
}

// TestVerdictRunCompleteSettlesUncertainPublication (change 0444 acceptance 3): the
// REAL keyed verdict over a settleable pair settles the original through the
// attributed closeout and ends in gate-done run-complete, with the original durably
// completed and the epoch completed. Without settlement the uncertain original would
// block the closeout (completion-unaccounted), so gate-done proves the settle ran on
// this path.
func TestVerdictRunCompleteSettlesUncertainPublication(t *testing.T) {
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

// TestReadOnlyVerdictPathsNeverSettleSettleablePair (change 0444 acceptance 3): the
// unattributed observe verdict and RunVerify, run over the same settleable pair on an
// ACTIVE and on a COMPLETING epoch, leave the epoch record byte-identical — neither
// is an authorized settlement writer.
func TestReadOnlyVerdictPathsNeverSettleSettleablePair(t *testing.T) {
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

// TestSettleUncertainPublicationsAuthorizedCallers is change 0444's shape guard: the
// settlement WRITE may be reached only from cancellation teardown
// (reconcileEpochTeardown) and the attributed successful closeout
// (completeSuccessfulRun). It derives every reference to the identifier
// settleUncertainPublications from the package's production source via the AST —
// any use (a call, or the function taken as a value) counts, keyed on the
// identifier, not a spelling list — and resolves the enclosing top-level
// declaration. A reference anywhere else, or either authorized caller losing its
// reference, reddens.
//
// Mutation probes: (a) add a call inside RunVerify -> "unauthorized referrer";
// (b) delete the call in completeSuccessfulRun -> the referrer set shrinks.
func TestSettleUncertainPublicationsAuthorizedCallers(t *testing.T) {
	const target = "settleUncertainPublications"
	want := []string{"completeSuccessfulRun", "reconcileEpochTeardown"}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	referrers := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			owner := "<package-level declaration in " + name + ">"
			var skip *ast.Ident
			if fd, ok := decl.(*ast.FuncDecl); ok {
				owner = fd.Name.Name
				if fd.Recv == nil && fd.Name.Name == target {
					skip = fd.Name // the definition itself is not a reference
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id == skip || id.Name != target {
					return true
				}
				referrers[owner] = true
				return true
			})
		}
	}
	got := make([]string, 0, len(referrers))
	for r := range referrers {
		got = append(got, r)
	}
	sort.Strings(got)
	allowed := map[string]bool{}
	for _, w := range want {
		allowed[w] = true
	}
	for _, r := range got {
		if !allowed[r] {
			t.Errorf("unauthorized referrer of %s: %s — settlement is a write reachable only from %v",
				target, r, want)
		}
	}
	for _, w := range want {
		if !referrers[w] {
			t.Errorf("authorized caller %s no longer references %s (got %v)", w, target, got)
		}
	}
}
