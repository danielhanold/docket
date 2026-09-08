package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the shared app-layer join that the four enforcement boundaries
// (FinalizeRebase, FinalizePublish, FinalizeMerge, and stacked closeout) reuse:
// it takes the descendants a live carrying branch PROMISES to carry
// (domain.DeriveCarriedSet over authoritative GitHub facts) and proves, in Git,
// that each one's merged work is preserved at exactly the immutable target commit
// (gitcli.ProvePreserved). A merged PR destination proves a relationship only;
// preservation of the CONTENT is this helper's separate obligation. Every refusal
// names its category and its ids so a stuck gate is self-diagnosing, and an
// observation error is returned as an error — never laundered into a verdict.

// The closed per-descendant preservation finding categories. Each names WHY one
// carried descendant's preservation could not be established, keeping the missing
// evidence case distinct from an observed mismatch (a green suite hid this class
// of gap precisely because the two were conflated).
const (
	// CarryFindingRelationship: DeriveCarriedSet surfaced a refusal token — the
	// descendant's carry RELATIONSHIP into this branch is not proven, so no content
	// proof is even attempted.
	CarryFindingRelationship = "carry-relationship-unproven"
	// CarryFindingMissingMerge: the descendant is carried, but its facts carry no
	// usable merge-result commit id — missing evidence, never a mismatch.
	CarryFindingMissingMerge = "carry-merge-id-missing"
	// CarryFindingUnpreserved: the preservation proof RAN and observed the merged
	// content is not preserved at the target commit.
	CarryFindingUnpreserved = "carry-content-unpreserved"
)

// ReasonCarryUnproven is the shared refusal reason the rebase, publish, merge,
// and stacked-closeout boundaries report when a carried descendant's preservation
// is not proven. Every carry finding carries it as its Code so a caller branches
// on one stable token regardless of which boundary raised it.
const ReasonCarryUnproven = "carried-descendant-unproven"

// carryProof is proveCarriedOnHead's closed return: Proven is true only when
// every promised descendant is preserved at the target (an empty carried set is
// vacuously proven); Findings carries one per unproven descendant and is empty
// when proven. It is never both proven and non-empty.
type carryProof struct {
	Proven   bool
	Findings []StatusFinding
}

// proveCarriedOnHead proves every descendant the parent's branch promises to
// carry is preserved at exactly the immutable target commit. No stack descendants
// means nothing to carry — vacuously proven with ZERO external probes. A returned error is an
// observation/external failure the caller maps to unknown and retains; it is
// never a verdict, and it carries no findings. Findings distinguish a missing
// carry relationship (CarryFindingRelationship) and missing merge evidence
// (CarryFindingMissingMerge) from an observed content mismatch
// (CarryFindingUnpreserved); each names the descendant id, its PR, the source
// merge id, and the target id. The result is valid ONLY for this target id — a
// proof does not carry to any other commit.
func proveCarriedOnHead(ctx context.Context, deps FinalizeDeps, repoDir string, gitRepo gitcli.Repository, snap domain.Snapshot, parent domain.Change, target gitcli.ObjectID) (carryProof, error) {
	// Vacuous proof: no descendant means nothing to carry, so no GitHub identity is
	// resolved and no PR is probed (the zero-probe promise).
	if len(domain.StackDescendantsParentFirst(snap, parent.ID())) == 0 {
		return carryProof{Proven: true, Findings: []StatusFinding{}}, nil
	}
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return carryProof{}, err
	}
	facts, err := probeDescendantFacts(ctx, deps, ghRepo, snap, parent.ID())
	if err != nil {
		return carryProof{}, err
	}
	set, polFail := domain.DeriveCarriedSet(snap, parent.ID(), facts)
	if polFail != nil {
		// A structural problem with the parent argument itself — surfaced as a single
		// unproven finding, never a proof.
		return carryProof{Findings: []StatusFinding{{
			Code:     ReasonCarryUnproven,
			Severity: string(domain.SeverityError),
			Message:  "carried-set derivation refused: " + polFail.Reason,
		}}}, nil
	}

	findings := []StatusFinding{}
	for _, d := range set {
		pr := facts[d.ID].Number
		if d.Proof != "" {
			// DeriveCarriedSet already refused the relationship; do not probe content.
			findings = append(findings, carryFinding(d.ID, pr, "", string(target), CarryFindingRelationship, d.Proof))
			continue
		}
		mergeID := facts[d.ID].MergeCommit
		if !validFullObjectID(mergeID) {
			findings = append(findings, carryFinding(d.ID, pr, mergeID, string(target), CarryFindingMissingMerge,
				"no usable merge-result commit id"))
			continue
		}
		check, perr := deps.Planning.Client.ProvePreserved(ctx, gitRepo, originRemote, gitcli.ObjectID(mergeID), target)
		if perr != nil {
			// Inability to observe is an error, never a verdict: retain, emit no finding.
			return carryProof{}, perr
		}
		if check.Outcome != gitcli.PreservationProven {
			findings = append(findings, carryFinding(d.ID, pr, mergeID, string(target), CarryFindingUnpreserved,
				check.Detail+pathsSuffix(check.Paths)))
		}
	}
	return carryProof{Proven: len(findings) == 0, Findings: findings}, nil
}

// carryFinding builds one carried-descendant refusal finding: its Code is the
// shared ReasonCarryUnproven, its Severity error, and its Message names the
// descendant id, its PR, the finding category, the detail, and the source→target
// commit pair — so a stuck gate names exactly which merge is unproven where.
func carryFinding(id domain.ChangeID, pr, sourceMerge, target, category, detail string) StatusFinding {
	return StatusFinding{
		Code:     ReasonCarryUnproven,
		Severity: string(domain.SeverityError),
		Message: fmt.Sprintf("change %04d (PR %s): %s: %s [source %s -> target %s]",
			int(id), pr, category, detail, sourceMerge, target),
	}
}

// pathsSuffix renders the bounded differing-path diagnostic a content-unpreserved
// proof carries, or "" when there is none, so the finding names the exact paths
// that differ without unbounding the message.
func pathsSuffix(paths []gitcli.RepoPath) string {
	if len(paths) == 0 {
		return ""
	}
	ss := make([]string, len(paths))
	for i, p := range paths {
		ss[i] = string(p)
	}
	return " (paths: " + strings.Join(ss, ", ") + ")"
}
