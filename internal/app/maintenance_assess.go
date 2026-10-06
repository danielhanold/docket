package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the maintenance sweep's SNAPSHOT ASSESSMENT of historical records
// (full scope only). Before the sweep dispatches a fresh cleanup for every
// done/stacked-merged record it discovered in the pinned inventory, it resolves
// each candidate LOCALLY — from the pinned corpus, one shared remote-heads
// advertisement, one shared worktree list, and the local workspace inspection —
// and enqueues only the records that actually warrant a cleanup attempt. A record
// whose every destructive leg is provably a no-op is reported as a pre-dispatch
// observation with an EMPTY Operation (never a fabricated cleanup result) and is
// NOT re-probed per item. The assessment fans out into no per-item metadata, PR,
// or remote-ref reads: the remote inventory is gathered once and shared, and a
// FAILED shared input yields UNKNOWN for the legs that depend on it, never a
// clean absence and never a fan-out into individual probes
// (learning probe-error-is-not-clean-absence).
//
// Every actionable record still flows through the normal fresh prepare +
// FinalizeCleanup with every live proof (Task 7): the assessment only DECLINES to
// dispatch a record it can prove needs nothing, and never certifies a mutation.

// The stable reasons the snapshot assessment reports for a non-actionable
// historical record (allowed-value additions). Message text is explanatory
// and must not be parsed.
const (
	// ReasonSweepSnapshotNoWork: every destructive leg is provably a no-op at the
	// pinned inventory — a cleaned tombstone, absent local/remote refs, and an
	// already-correct PR backlink. Nothing was dispatched.
	ReasonSweepSnapshotNoWork = "snapshot-no-work"
	// ReasonSweepSnapshotRetained: a stacked-merged record is retained until its
	// stack root reaches the integration branch; a cleanup is never dispatched
	// merely to rediscover the retention.
	ReasonSweepSnapshotRetained = "snapshot-retained"
	// ReasonSweepSnapshotBlocked: a local blocker (a missing or foreign workspace
	// manifest that does not certify clean) prevents every remaining effect, and no
	// independent leg has provable work. Nothing was dispatched; the message names
	// the blocker.
	ReasonSweepSnapshotBlocked = "snapshot-blocked"
	// ReasonSweepSnapshotUnknown: an inspection could not be resolved (a failed
	// shared remote inventory, an unreadable workspace probe, or a malformed PR
	// backlink block), so absence could not be proven and no leg established work.
	// Nothing was dispatched; the message names the unresolved leg(s).
	ReasonSweepSnapshotUnknown = "snapshot-unknown"
	// ReasonSweepSnapshotInvalid: the record's identity is not usable for
	// assessment (an unusable recorded branch, no canonical PR reference, an
	// unresolved base, or a malformed workspace target). Distinguished from a clean
	// absence; nothing was dispatched.
	ReasonSweepSnapshotInvalid = "snapshot-invalid"
)

// sweepSharedFacts are the invocation-shared read-only observations the snapshot
// assessment reasons over. Each leg records failure separately; a failed input
// yields UNKNOWN assessments for the legs that depend on it, never a fan-out into
// individual probes. remoteHeads is the complete branch-heads advertisement
// (gitcli.ListRemoteHeads) gathered once when there are done candidates; worktrees
// is the one worktree list. A set error means that inventory is unknown this
// invocation — a ref's absence is unprovable, so the dependent leg is unresolved.
type sweepSharedFacts struct {
	remoteHeads    map[gitcli.RefName]gitcli.ObjectID
	remoteHeadsErr error // set ⇒ remote-ref absence is unprovable this invocation
	worktrees      []gitcli.WorktreeInfo
	worktreesErr   error // set ⇒ local-ref absence is unprovable this invocation
	// prBodiesGathered is set when the invocation read its done candidates' PR
	// bodies (one batched pass). When set, a number in neither prBodies nor
	// prBodiesUnknown is still unknown — the leg never reads absence as no-work.
	prBodiesGathered bool
	prBodies         map[int]string // PR number -> body (never logged)
	prBodiesUnknown  map[int]bool   // PR numbers the batched read could not resolve
}

// The leg names the assessment reports in its diagnostic messages. They are
// explanatory labels, not a parsed vocabulary.
const (
	sweepLegWorkspace  = "workspace"
	sweepLegLocalRef   = "local-ref"
	sweepLegRemoteRef  = "remote-ref"
	sweepLegPRBacklink = "pr-backlink"
)

// sweepAssessHistorical resolves every full-scope done/stacked-merged candidate
// from the shared snapshots and local inspection. It returns the non-actionable
// entries (Disposition skipped/blocked/unknown, EMPTY Operation — a pre-dispatch
// observation, never a fabricated cleanup result) and the subset of candidates
// that warrant one normal fresh cleanup attempt. It dispatches no metadata, PR, or
// remote-ref read of its own: the remote/worktree inventories and the batched
// PR-body inventory arrive in shared, and the workspace leg is a local-only
// inspection.
func sweepAssessHistorical(ctx context.Context, deps FinalizeDeps, wdeps WorkspaceDeps,
	inv sweepInventory, pin StatusPin, shared sweepSharedFacts,
	candidates []sweepWorkItem) (entries []MaintenanceEntry, actionable []sweepWorkItem) {

	repo := sweepAssessRepo(deps)
	link := linkContextOf(pin)

	for _, it := range candidates {
		c, out := inv.snap.Change(domain.ChangeID(it.id))
		if out != domain.LookupFound {
			// Absent or ambiguous in the pinned inventory: never a clean absence.
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotInvalid,
				"the record is absent or ambiguous in the pinned inventory"))
			continue
		}

		// Stacked-merged is unconditionally retained until its root closes — the
		// existing cleanup contract. Never dispatch merely to rediscover it, and do
		// NOT layer the done-record identity prerequisites onto this path.
		if c.Status() == domain.StatusStackedMerged {
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotRetained,
				"stacked-merged: retained until its stack root reaches the integration branch"))
			continue
		}
		if c.Status() != domain.StatusDone {
			// The worklist only enqueues done/stacked-merged records as cleanups; any
			// other status reaching here is an unusable identity, never a clean state.
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotInvalid,
				"the record is neither done nor stacked-merged"))
			continue
		}

		// Validate the done record's identity: an unusable branch, a missing
		// canonical PR reference, an unresolved base, or a malformed workspace target
		// is snapshot-invalid — distinguishable from a clean absence, never a no-op.
		branch, berr := recordedBranch(c)
		if berr != nil {
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotInvalid,
				"the recorded feature branch is unusable ("+berr.Error()+")"))
			continue
		}
		if _, ok := parsePRNumber(c.PR().Value); !finalizeHasPRRef(c) || !ok {
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotInvalid,
				"the record carries no canonical pull-request reference to verify"))
			continue
		}
		base := domain.ResolveEffectiveBase(inv.snap, c, domain.NewBranchFacts(nil))
		if base.Kind != domain.BaseResolved {
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotInvalid,
				"the record's effective base did not resolve"))
			continue
		}
		target, terr := workspace.NewTarget(c.ID(), c.Slug(), base, branch)
		if terr != nil {
			entries = append(entries, sweepEntry(it.id, it.kind, SweepDispSkipped, "", ReasonSweepSnapshotInvalid,
				"the record's workspace target is malformed"))
			continue
		}

		// Assess every INDEPENDENT destructive leg. Each leg is evaluated on its own
		// facts: a blocker on one leg never hides another leg's provable work, and an
		// unresolved leg never launders into a clean no-op.
		featureRef := gitcli.RefName(branchRefPrefix + branch)
		var a sweepLegAssessment
		sweepAssessPRBacklinkLeg(shared, c, link, &a)
		sweepAssessWorkspaceLeg(ctx, wdeps, repo, target, &a)
		sweepAssessLocalRefLeg(shared, featureRef, &a)
		sweepAssessRemoteRefLeg(shared, featureRef, &a)

		if a.hasWork {
			// One or more legs may have work: dispatch one normal fresh cleanup
			// attempt with every live proof (Task 7). No pre-dispatch entry.
			actionable = append(actionable, it)
			continue
		}
		disp, reason, msg := a.verdict()
		entries = append(entries, sweepEntry(it.id, it.kind, disp, "", reason, msg))
	}
	return entries, actionable
}

// sweepAssessRepo recovers the discovered repository the production reader pinned,
// so the workspace leg can inspect against it without a re-pin. A reader that does
// not implement pinnedRepository (a unit-test fake whose workspace service ignores
// the repository) yields the zero repository.
func sweepAssessRepo(deps FinalizeDeps) gitcli.Repository {
	if pr, ok := deps.Planning.Reader.(pinnedRepository); ok {
		if _, repo, bound := pr.pinnedRepo(); bound {
			return repo
		}
	}
	return gitcli.Repository{}
}

// sweepLegAssessment accumulates the independent legs' verdicts for one record:
// whether any leg has provable work, and the diagnostics of the legs that could
// not be resolved (unknown) or that are blocked. hasWork short-circuits the
// verdict — a record with any actionable leg is dispatched regardless of another
// leg's unknown or blocked state.
type sweepLegAssessment struct {
	hasWork bool
	unknown []string
	blocked []string
}

func (a *sweepLegAssessment) markWork() { a.hasWork = true }
func (a *sweepLegAssessment) markUnknown(leg, detail string) {
	a.unknown = append(a.unknown, leg+": "+detail)
}
func (a *sweepLegAssessment) markBlocked(leg, detail string) {
	a.blocked = append(a.blocked, leg+": "+detail)
}

// verdict maps the accumulated legs onto the non-actionable disposition. Unresolved
// dominates blocked: an unknown leg means absence could not be proven, so a record
// with any unknown leg is snapshot-unknown even when another leg is blocked — and
// the unknown leg stays named in the message. A record with only a blocker is
// snapshot-blocked; a record whose every leg is provably a no-op is snapshot-no-work.
func (a sweepLegAssessment) verdict() (disposition, reason, message string) {
	if len(a.unknown) > 0 {
		msg := "unresolved — " + strings.Join(a.unknown, "; ")
		if len(a.blocked) > 0 {
			msg += " (also blocked — " + strings.Join(a.blocked, "; ") + ")"
		}
		return SweepDispUnknown, ReasonSweepSnapshotUnknown, msg
	}
	if len(a.blocked) > 0 {
		return SweepDispBlocked, ReasonSweepSnapshotBlocked, "blocked — " + strings.Join(a.blocked, "; ")
	}
	return SweepDispSkipped, ReasonSweepSnapshotNoWork,
		"every destructive leg is provably a no-op at the pinned inventory"
}

// sweepAssessPRBacklinkLeg resolves the PR-backlink leg from the shared, batched
// PR-body inventory: a body whose docket:backlink block does not name the
// archived record path has work (cleanup repoints it). A body already naming it,
// or with no block, is no-effect. An unread body or malformed markers is
// unresolved, never a clean no-op. Not gathered (an orchestration seam) is not
// assessed.
func sweepAssessPRBacklinkLeg(shared sweepSharedFacts, c domain.Change, link render.LinkContext, a *sweepLegAssessment) {
	if !shared.prBodiesGathered {
		return
	}
	n, ok := parsePRNumber(c.PR().Value)
	if !ok {
		return // identity already validated by the caller
	}
	body, read := shared.prBodies[n]
	if !read || shared.prBodiesUnknown[n] {
		a.markUnknown(sweepLegPRBacklink, fmt.Sprintf("PR #%d's body could not be read in the shared batch", n))
		return
	}
	block, err := render.BacklinkContent(c, link)
	if err != nil {
		a.markUnknown(sweepLegPRBacklink, "the record's archived backlink could not be rendered")
		return
	}
	_, _, _, needs, err := planPRBacklinkRepoint([]byte(body), c.Path(), backlinkInterior(block))
	if err != nil {
		a.markUnknown(sweepLegPRBacklink, fmt.Sprintf("PR #%d carries a malformed docket:backlink block", n))
		return
	}
	if needs {
		a.markWork()
	}
}

// sweepAssessWorkspaceLeg resolves the workspace leg from a local-only inspection
// (workspace.Service.Inspect never fetches). A cleaned tombstone certifies the
// checkout is already gone; a missing or foreign manifest does NOT certify clean —
// it is a blocker, never a no-op; a probe error is unresolved; any other legible
// owned state is possible work.
func sweepAssessWorkspaceLeg(ctx context.Context, wdeps WorkspaceDeps, repo gitcli.Repository, target workspace.Target, a *sweepLegAssessment) {
	if wdeps.Service == nil {
		a.markUnknown(sweepLegWorkspace, "no workspace service is wired to inspect the checkout")
		return
	}
	insp, err := wdeps.Service.Inspect(ctx, workspace.InspectRequest{Repository: repo, Target: target})
	if err != nil {
		a.markUnknown(sweepLegWorkspace, "the feature workspace could not be inspected")
		return
	}
	switch insp.Kind {
	case workspace.StateCleaned:
		// An owned tombstone with no registration: the checkout is provably gone.
	case workspace.StateForeign, workspace.StateAbsent:
		// Foreign, malformed, unowned, or proven cleanly absent: never certifies
		// clean. StateAbsent is a current local observation, not a cleaned
		// tombstone and not authority to delete — pre-0368 an absent manifest
		// inspected as StateForeign and hit this same blocked leg (change 0368).
		a.markBlocked(sweepLegWorkspace, "a missing or foreign workspace manifest does not certify a clean checkout")
	default:
		// A registered, ready/dirty/allocating/mismatched/branch-gone checkout is a
		// live owned workspace the cleanup may still remove: possible work.
		a.markWork()
	}
}

// sweepAssessLocalRefLeg resolves the local-ref leg from the shared worktree list:
// a worktree still checked out on the feature ref is a leftover local ref the
// cleanup would remove. A failed worktree list is unresolved — a ref's absence is
// unprovable — never a clean absence.
func sweepAssessLocalRefLeg(shared sweepSharedFacts, featureRef gitcli.RefName, a *sweepLegAssessment) {
	if shared.worktreesErr != nil {
		a.markUnknown(sweepLegLocalRef, "the worktree list could not be read; the local ref's state is unprovable")
		return
	}
	for _, wi := range shared.worktrees {
		if wi.Branch == featureRef {
			a.markWork()
			return
		}
	}
}

// sweepAssessRemoteRefLeg resolves the remote-ref leg from the shared complete
// remote-heads advertisement: the feature ref present on the remote is a leftover
// remote ref the cleanup would delete. A FAILED advertisement is unresolved — the
// ref's absence is unprovable this invocation (a partial read would understate the
// inventory), never a clean absence and never a fan-out into per-ref probes. A
// clean, complete advertisement (including an empty one) proves absence.
func sweepAssessRemoteRefLeg(shared sweepSharedFacts, featureRef gitcli.RefName, a *sweepLegAssessment) {
	if shared.remoteHeadsErr != nil {
		a.markUnknown(sweepLegRemoteRef, "the remote head advertisement could not be read; the remote ref's absence is unprovable")
		return
	}
	if _, ok := shared.remoteHeads[featureRef]; ok {
		a.markWork()
	}
}
