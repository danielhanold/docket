//go:build integration

package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
)

// This file drives proveCarriedOnHead — the shared app helper that joins
// domain.DeriveCarriedSet (which descendants a live parent branch promises to
// carry) with gitcli.ProvePreserved (is that carried merge preserved at exactly
// this immutable target commit?) over authoritative GitHub facts. It runs over
// the REAL closeout metadata topology plus real Git objects: the positive rows
// carry real 40-hex merge OIDs that EXIST in the fixture repo (green-suite-
// untested-branch: a fixture whose discriminating input is a real object that is
// dropped, distinct from one that is merely missing), and every assertion pins
// the outcome, the finding category, and the named ids — never a bare
// non-success (assert-pins-outcome-not-mechanism). The tests are prefixed
// TestIntegrationFinalizeRebaseCarryHelper so the existing app rebase shard
// (tests/test_go_integration_app_rebase.sh, prefix ^TestIntegrationFinalizeRebase)
// routes them.

// carryChildPlan is the descendant's plan pointer; the artifact need not exist —
// the helper reads facts and Git objects, never the plan bytes.
const carryChildPlan = "docs/superpowers/plans/2026-08-16-gadget-plan.md"

// seedCarryChild adds a descendant (id 6, gadget) stacked on the root (id 5) at
// the given lifecycle status, carrying PR #8, onto the metadata branch.
func seedCarryChild(t *testing.T, f *closeoutFixture, status string) {
	t.Helper()
	recPath := groomPath(6, "gadget")
	desc := closeoutRecord(6, "gadget", status, "github.com/acme/widget#8", "", carryChildPlan, "")
	desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{recPath: desc})
}

// carryCommit creates a commit on branch br (new branches start from startRef;
// an existing branch is extended) adding files, pushes it to origin so
// ProvePreserved can fetch the exact object, and returns its 40-hex OID.
func (f *closeoutFixture) carryCommit(t *testing.T, br, startRef string, files map[string]string) string {
	t.Helper()
	w := f.repo.writer
	if _, err := tryGit(w, "rev-parse", "--verify", "--quiet", "refs/heads/"+br); err == nil {
		runGit(t, w, "checkout", "-q", br)
	} else {
		runGit(t, w, "checkout", "-q", "-b", br, startRef)
	}
	for rel, content := range files {
		writeRepoFile(t, w, rel, content)
	}
	runGit(t, w, "add", "-A")
	runGit(t, w, "commit", "-q", "-m", "carry "+br)
	runGit(t, w, "push", "-q", "origin", br)
	return runGit(t, w, "rev-parse", "HEAD")
}

// fetchAllIntoInvocation makes every origin branch's objects local in the
// invocation clone the helper's gitcli client operates, so ProvePreserved reads
// its operands from the local store.
func (f *closeoutFixture) fetchAllIntoInvocation(t *testing.T) {
	t.Helper()
	runGit(t, f.repo.invocation, "fetch", "-q", "origin", "+refs/heads/*:refs/remotes/origin/*")
}

// carryFake scripts PR #8 as merged into baseRef at mergeCommit — the carried
// descendant's authoritative post-merge facts.
func carryFake(head, baseRef, mergeCommit string) *fakeCloseoutGitHub {
	return &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(head, baseRef, mergeCommit)},
		},
	}
}

// TestIntegrationFinalizeRebaseCarryHelperEmptySetZeroProbes proves an empty
// carried set is vacuously proven with ZERO external GitHub probes, and the
// companion that the probe witness fires on a stack that DOES carry a
// descendant — so the zero-probe assert is a real guard, not a vacuous one.
func TestIntegrationFinalizeRebaseCarryHelperEmptySetZeroProbes(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()

	// No descendants: vacuously proven, zero probes.
	f := setupCloseoutFixture(t, planRepoModes()[0])
	base := originTip(t, f.repo.origin, "main")
	gh := f.baselineMergedFake(f.head, base)
	cc, refusal := loadCloseoutContext(ctx, f.closeoutDeps(gh), f.repo.invocation, f.id)
	if refusal != nil {
		t.Fatalf("loadCloseoutContext refused: %+v", *refusal)
	}
	proof, err := proveCarriedOnHead(ctx, f.closeoutDeps(gh), f.repo.invocation, cc.repo, cc.snap, cc.change, gitcli.ObjectID(base))
	if err != nil {
		t.Fatalf("empty carried set errored: %v", err)
	}
	if !proof.Proven || len(proof.Findings) != 0 {
		t.Fatalf("empty carried set = proven %v findings %+v, want proven with none", proof.Proven, proof.Findings)
	}
	if gh.probes != 0 {
		t.Fatalf("an empty carried set made %d GitHub probes; a vacuous proof must probe zero times", gh.probes)
	}

	// Companion: a real carried descendant DOES probe (the witness fires).
	f2 := setupCloseoutFixture(t, planRepoModes()[0])
	seedCarryChild(t, f2, "stacked-merged")
	anc := f2.carryCommit(t, "line-anc", "main", map[string]string{"anc.txt": "anc\n"})
	f2.fetchAllIntoInvocation(t)
	gh2 := carryFake(f2.head, "feat/widget", anc)
	cc2, refusal := loadCloseoutContext(ctx, f2.closeoutDeps(gh2), f2.repo.invocation, f2.id)
	if refusal != nil {
		t.Fatalf("loadCloseoutContext (companion) refused: %+v", *refusal)
	}
	if _, err := proveCarriedOnHead(ctx, f2.closeoutDeps(gh2), f2.repo.invocation, cc2.repo, cc2.snap, cc2.change, gitcli.ObjectID(anc)); err != nil {
		t.Fatalf("companion carried proof errored: %v", err)
	}
	if gh2.probes == 0 {
		t.Fatalf("a carried descendant made zero probes; the zero-probe witness never fires and is vacuous")
	}
}

// TestIntegrationFinalizeRebaseCarryHelperPreservation drives every per-descendant
// finding category over ONE stacked fixture: an ancestor-preserved carry, a
// content-preserved (rebased) carry, and each unproven category — relationship,
// missing merge id, observed content mismatch, and an observation error. The
// snapshot is loaded once; only the scripted facts and the immutable target id
// vary per row.
func TestIntegrationFinalizeRebaseCarryHelperPreservation(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()

	f := setupCloseoutFixture(t, planRepoModes()[0])
	seedCarryChild(t, f, "stacked-merged")

	// Real Git objects for the rows. All are pushed to origin, then fetched local.
	//   ancestor line: mcAnc <- tgtAnc (tgtAnc descends mcAnc)
	mcAnc := f.carryCommit(t, "line-anc", "main", map[string]string{"anc1.txt": "1\n"})
	tgtAnc := f.carryCommit(t, "line-anc", "", map[string]string{"anc2.txt": "2\n"})
	//   content-preserved: two siblings off main both adding catalog.yaml=X
	mcContent := f.carryCommit(t, "line-src", "main", map[string]string{"catalog.yaml": "X\n"})
	tgtContent := f.carryCommit(t, "line-tgt", "main", map[string]string{"catalog.yaml": "X\n"})
	//   content-dropped: source adds catalog.yaml=Y; target drops it (adds only other.txt)
	mcDropped := f.carryCommit(t, "line-srcd", "main", map[string]string{"catalog.yaml": "Y\n"})
	tgtDropped := f.carryCommit(t, "line-tgtd", "main", map[string]string{"other.txt": "o\n"})
	f.fetchAllIntoInvocation(t)

	// A 40-hex object that exists NOWHERE (repo or remote): an observation error,
	// never a verdict — distinct from a dropped object that exists.
	absent := strings.Repeat("e", 40)

	cc, refusal := loadCloseoutContext(ctx, f.closeoutDeps(f.baselineMergedFake(f.head, mcAnc)), f.repo.invocation, f.id)
	if refusal != nil {
		t.Fatalf("loadCloseoutContext refused: %+v", *refusal)
	}
	childID := fmt.Sprintf("%04d", 6)

	run := func(gh *fakeCloseoutGitHub, target string) (carryProof, error) {
		return proveCarriedOnHead(ctx, f.closeoutDeps(gh), f.repo.invocation, cc.repo, cc.snap, cc.change, gitcli.ObjectID(target))
	}

	t.Run("ancestor-preserved-proven", func(t *testing.T) {
		proof, err := run(carryFake(f.head, "feat/widget", mcAnc), tgtAnc)
		if err != nil {
			t.Fatalf("ancestor carry errored: %v", err)
		}
		if !proof.Proven || len(proof.Findings) != 0 {
			t.Fatalf("ancestor carry = proven %v findings %+v, want proven with none", proof.Proven, proof.Findings)
		}
	})

	t.Run("content-preserved-proven", func(t *testing.T) {
		proof, err := run(carryFake(f.head, "feat/widget", mcContent), tgtContent)
		if err != nil {
			t.Fatalf("content carry errored: %v", err)
		}
		if !proof.Proven || len(proof.Findings) != 0 {
			t.Fatalf("content-preserved carry = proven %v findings %+v, want proven with none", proof.Proven, proof.Findings)
		}
	})

	t.Run("relationship-unproven", func(t *testing.T) {
		// #8 is not scripted merged, so the descendant's carry relationship cannot
		// be confirmed (pr-unknown) — a relationship refusal, never a content probe.
		gh := &fakeCloseoutGitHub{repo: retargetRepo(), merged: map[int]closeoutProbe{}}
		proof, err := run(gh, tgtAnc)
		if err != nil {
			t.Fatalf("relationship row errored: %v", err)
		}
		if proof.Proven || len(proof.Findings) != 1 {
			t.Fatalf("relationship row = proven %v findings %+v, want unproven with one finding", proof.Proven, proof.Findings)
		}
		fd := proof.Findings[0]
		if fd.Code != ReasonCarryUnproven {
			t.Errorf("finding code = %q, want %q", fd.Code, ReasonCarryUnproven)
		}
		if !strings.Contains(fd.Message, CarryFindingRelationship) {
			t.Errorf("finding does not name the relationship category:\n%s", fd.Message)
		}
		if !strings.Contains(fd.Message, childID) {
			t.Errorf("finding does not name the descendant id %s:\n%s", childID, fd.Message)
		}
	})

	t.Run("missing-merge-id", func(t *testing.T) {
		// Carried by relationship, but the merge facts carry no usable merge id:
		// missing evidence, distinct from an observed mismatch.
		proof, err := run(carryFake(f.head, "feat/widget", ""), tgtAnc)
		if err != nil {
			t.Fatalf("missing-merge row errored: %v", err)
		}
		if proof.Proven || len(proof.Findings) != 1 {
			t.Fatalf("missing-merge row = proven %v findings %+v, want unproven with one finding", proof.Proven, proof.Findings)
		}
		if !strings.Contains(proof.Findings[0].Message, CarryFindingMissingMerge) {
			t.Errorf("finding does not name the missing-merge category:\n%s", proof.Findings[0].Message)
		}
	})

	t.Run("content-unpreserved-names-source-and-target", func(t *testing.T) {
		// The source merge object EXISTS in the repo (pin drop vs missing-object)
		// yet the target dropped its content: an observed mismatch.
		if _, err := tryGit(f.repo.invocation, "cat-file", "-e", mcDropped); err != nil {
			t.Fatalf("the source merge object must exist to pin a drop (not a missing object): %v", err)
		}
		proof, err := run(carryFake(f.head, "feat/widget", mcDropped), tgtDropped)
		if err != nil {
			t.Fatalf("content-dropped row errored: %v", err)
		}
		if proof.Proven || len(proof.Findings) != 1 {
			t.Fatalf("content-dropped row = proven %v findings %+v, want unproven with one finding", proof.Proven, proof.Findings)
		}
		fd := proof.Findings[0]
		if !strings.Contains(fd.Message, CarryFindingUnpreserved) {
			t.Errorf("finding does not name the unpreserved category:\n%s", fd.Message)
		}
		if !strings.Contains(fd.Message, mcDropped) {
			t.Errorf("finding does not name the source merge id %s:\n%s", mcDropped, fd.Message)
		}
		if !strings.Contains(fd.Message, tgtDropped) {
			t.Errorf("finding does not name the target id %s:\n%s", tgtDropped, fd.Message)
		}
	})

	t.Run("observation-error-is-not-a-verdict", func(t *testing.T) {
		// The merge id is absent from repo AND remote: ProvePreserved cannot see its
		// operand, so the helper returns an error and emits NO finding.
		proof, err := run(carryFake(f.head, "feat/widget", absent), tgtAnc)
		if err == nil {
			t.Fatalf("an unobservable merge object returned no error: %+v", proof)
		}
		if len(proof.Findings) != 0 {
			t.Fatalf("an observation error emitted findings (a verdict): %+v", proof.Findings)
		}
	})
}
