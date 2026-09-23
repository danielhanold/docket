package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `docket run gate-before` operation (change 0334, Task 2): it
// ARMS the implement-next run gate. It re-syncs the metadata worktree to fresh
// origin, reads the current in-progress claim set, captures a dispatch epoch
// AFTER that read, and mints a durable gate record under the git common dir
// (rungate_store.go). Its whole contract is the printed report line:
// `gate-armed <key>` on success, `gate-unarmed <reason-token>` on any failure —
// both exit 0 (learning exit-code-encodes-a-non-failure). Only `implement-next`
// is an accepted target; anything else is a usage error that exits non-zero.
//
// It writes NO metadata: the fresh-origin re-sync and the change-file parse are
// the SAME plumbing the claim path uses (PinContext advances the remote-tracking
// ref; ReadCorpus + parseCorpus + repository.BuildSnapshot yield the snapshot),
// reused rather than reimplemented (learning duplicated-gate-copies-the-whole-
// predicate). The only durable write is the gate record.

// OperationRunGateBefore is the operation key `run gate-before` records in its
// envelope.
const OperationRunGateBefore = "run.gate-before"

// gateBeforeAcceptedTarget is the sole accepted target argument (the workflow
// name, not the agent name); gateBeforeStoredTarget is the canonical agent name
// the durable record stores as its Target — the thing a dispatch actually
// launches. Attribution downstream keys on the workflow the gate brackets, so
// the two spellings are pinned here, not derived.
const (
	gateBeforeAcceptedTarget = "implement-next"
	gateBeforeStoredTarget   = "docket-implement-next"
)

// The stable reason tokens a gate-unarmed line carries. Each names the arming
// step that failed; a consumer keys on the token, never the prose.
const (
	// ReasonGateInvalidTarget: the target argument was not `implement-next`. This
	// is a usage error (non-zero exit), not a gate-unarmed report line.
	ReasonGateInvalidTarget = "invalid-target"
	// ReasonGateSyncFailed: the fresh-origin metadata re-sync (PinContext) failed,
	// so the before-read could not be taken from authoritative state.
	ReasonGateSyncFailed = "sync-failed"
	// ReasonGateChangesUnreadable: the changes corpus could not be read or parsed
	// into a snapshot, so the in-progress claim set is unknown.
	ReasonGateChangesUnreadable = "changes-unreadable"
	// ReasonGateMintFailed: the durable gate record could not be minted (the git
	// common dir was unresolvable, or the write failed).
	ReasonGateMintFailed = "mint-failed"
	// ReasonGateResumeUnverified: a --resume id could not be verified as an
	// already-in-progress change with a valid workspace identity, so the gate
	// refuses to pre-bind attribution to it. No record is minted (change 0359).
	ReasonGateResumeUnverified = "resume-unverified"
	// ReasonGateScopeFailed: the outer recovery scope could not be prepared, so no
	// dispatch context exists to hand the child. No record is minted (change 0359).
	ReasonGateScopeFailed = "scope-failed"
	// ReasonGateResumeActiveRun: a --resume id names a change whose prior run epoch is
	// still ACTIVE — an earlier coordinator can still act on the worktree. Resume
	// refuses with a safe locator and the explicit cancel/continue remedy; it never
	// shuts the incumbent down (change 0375 Task 12, spec "If the prior run is still
	// active, refuse with its safe locator and explicit cancel/continue remedy").
	ReasonGateResumeActiveRun = "resume-active-run"
	// ReasonGateResumeCancellationPending: a --resume id's prior epoch is CANCELLING
	// or otherwise unresolved — cancellation cleanup is still in progress, so no
	// replacement is admitted (spec "If cancelling or unresolved, resume
	// cleanup/observation and admit no replacement"). Repeatable via run.cancel.
	ReasonGateResumeCancellationPending = CancelDispositionPending
	// ReasonGateResumeReplacementReserved: a --resume id's prior epoch was already
	// superseded and a replacement dispatch is reserved (a lost response, a repeat
	// arm, or the loser of a concurrent-resume race). It observes that reservation —
	// the reserved gate key is returned in Key — and reserves no second replacement
	// (spec "Lost responses and repeat arms observe/recover that reservation; they do
	// not create another epoch or re-dispatch").
	ReasonGateResumeReplacementReserved = "resume-replacement-reserved"
	// ReasonGateResumeEpochUnreadable: the prior run epoch could not be read or its
	// supersede transition faulted — fail closed rather than admit a replacement over
	// an unresolvable run (change 0375 Task 12).
	ReasonGateResumeEpochUnreadable = "resume-epoch-unreadable"
	// ReasonGateResumeRunCompleting: a --resume id's prior epoch is COMPLETING — a
	// keyed verdict verified the run complete and durably fenced the epoch, but
	// closeout is unfinished, so the epoch still owns its worktree (change 0441).
	// Resume never turns a completing run into a cancelled predecessor or reserves a
	// replacement; the remedy is the keyed 'docket run gate-verdict' (finish closeout)
	// or an explicit 'docket run cancel'.
	ReasonGateResumeRunCompleting = "resume-run-completing"
	// ReasonGateResumeRunCompleted: a --resume id's prior epoch is COMPLETED — the
	// successful closeout finished and the run retired terminally (change 0441). There
	// is nothing to resume: the remedy is 'docket run verify' and finalize. Never
	// quiescence-checked into a supersede, never a replacement reservation.
	ReasonGateResumeRunCompleted = "resume-run-completed"
	// ReasonOwnerLifecycleUnavailable is the honest limitation an armed gate reports
	// (change 0375 Task 13): the default dispatch route has NO owner-death or Stop
	// lifecycle event that would cancel the run automatically, so a Stop is the
	// explicit `run.cancel` operation. Only the Codex `agent.enter` route carries a
	// signal-connected cancellation and a death guardian; every other route relies on
	// the human running `run.cancel` (named in the skills prose). It is a standing
	// caveat, never a refusal — an armed gate still arms.
	ReasonOwnerLifecycleUnavailable = "owner-lifecycle-unavailable"
)

// GateScopeDeps carries the outer-scope preparation seam gate-before composes
// so unit tests can fake the durable scope mint. Production wiring (internal/cli/
// run.go) composes gatedrive.OpenStore(<git-common-dir>).PrepareScope; a unit
// test injects a fake that records the request and returns a canned grant.
type GateScopeDeps struct {
	Prepare func(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error)
	// CancelSeams overrides the cancellation seams the resume path's old-epoch
	// quiescence validation composes (change 0435); nil composes
	// productionCancelSeams(repoDir). Unit tests inject permissive or adversarial
	// seams; production callers leave it nil.
	CancelSeams func(repoDir string) cancelSeams
}

// gateHashToken returns the sha256 of a raw token as lowercase hex — the single
// boundary at which the printed dispatch context becomes the stored
// ChildContextHash, so the persisted record links to a nested drive's
// GateContextHash without carrying the raw capability. It matches gatedrive's own
// capHash so the two hashes compare equal.
func gateHashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RunGateBeforeResult is the protocol-v1 document `run gate-before` returns. On
// an armed gate Result is applied and Key names the durable record; on a
// gate-unarmed report Result is still applied (the report line exits 0) and
// Reason carries the stable token. A usage error (bad target) carries a
// non-applied Result and no report line. It never carries authored document
// bodies.
type RunGateBeforeResult struct {
	Envelope
	Armed bool   `json:"armed"`
	Key   string `json:"key,omitempty"`
	// DispatchContext is the outer scope's ChildCapability the parent copies into
	// the implement-next dispatch prompt; a nested drive carries its hash as the
	// GateContextHash. It is NOT secret from the child (change 0359). The parent
	// capability is deliberately absent from this result — it lives only in the
	// 0600-private gate record.
	DispatchContext string `json:"dispatch_context,omitempty"`
	// Epoch is the fresh run's PUBLIC epoch id, minted at arm time beside the gate
	// record (rungate_epoch.go). It authorizes nothing (ADR-0111) but is the locator
	// the operator threads into `run.cancel --epoch <id>` — the primary human Stop —
	// and the dispatcher threads into each `--run-epoch` flag (agent.enter, gate drive
	// start, gate drive prepare-scope). Without it the documented Stop path names an
	// epoch the arm never surfaced (change 0375). Empty only on a legacy resume arm
	// that shares no epoch; the resume-active locator already prints the epoch there.
	Epoch   string `json:"epoch,omitempty"`
	Target  string `json:"target,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	// OwnerLifecycle is the honest owner-lifecycle limitation of the dispatched
	// route (change 0375 Task 13). On an armed gate it carries
	// `owner-lifecycle-unavailable`: the default dispatch route has no automatic
	// Stop/owner-death cancellation, so a Stop is the explicit `run.cancel`
	// operation. Empty when no gate is armed.
	OwnerLifecycle string `json:"owner_lifecycle,omitempty"`
}

// HumanText renders the one report line. An armed gate prints `gate-armed <key>
// <epoch> <dispatch-context>` (the epoch is omitted only on a legacy resume arm
// that shares no epoch); a gate-unarmed report prints `gate-unarmed
// <reason-token>`; a usage error (a non-applied result) names its reason instead
// of a report line. The parent capability never appears here — only the child
// dispatch context, which is meant for the child.
func (r RunGateBeforeResult) HumanText() string {
	if r.Result == ResultApplied {
		if r.Armed {
			line := "gate-armed " + r.Key
			if r.Epoch != "" {
				line += " " + r.Epoch
			}
			line += " " + r.DispatchContext
			if r.OwnerLifecycle != "" {
				// Honest standing caveat: the dispatched route cancels no run on owner
				// death; a Stop is the explicit `run.cancel` operation.
				line += "\n" + r.OwnerLifecycle
			}
			return line
		}
		return "gate-unarmed " + r.Reason
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newRunGateBeforeResult stamps the envelope for the operation.
func newRunGateBeforeResult(result Result, out RunGateBeforeResult) RunGateBeforeResult {
	out.Envelope = NewEnvelope(OperationRunGateBefore, result)
	return out
}

// gateUnarmed builds a gate-unarmed report line: a success-shaped envelope
// (exit 0) carrying the stable reason token. It never mints a record.
func gateUnarmed(reason string) RunGateBeforeResult {
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{Armed: false, Reason: reason})
}

// gateUnarmedMsg is gateUnarmed with a bounded human Message — a safe locator and
// remedy for a resume refusal. The Message carries only public locators (a gate key,
// a public epoch id, a change id), never a capability or reservation token.
func gateUnarmedMsg(reason, message string) RunGateBeforeResult {
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{Armed: false, Reason: reason, Message: message})
}

// gateResumeObserve builds the observe-the-reservation report a repeat arm or the
// loser of a concurrent-resume race returns: gate-unarmed with the winner's reserved
// gate key in Key, so the caller recovers the single reserved replacement rather than
// admitting a second (change 0375 Task 12). It mints no record and no epoch.
func gateResumeObserve(reservedKey string) RunGateBeforeResult {
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{
		Armed:   false,
		Reason:  ReasonGateResumeReplacementReserved,
		Key:     reservedKey,
		Message: "a replacement dispatch is already reserved under gate key " + reservedKey + "; a second cannot be armed",
	})
}

// resumeActiveLocator renders the safe locator and explicit cancel/continue remedy
// a resume prints when the prior run is still active (change 0375 Task 12). It names
// only public locators — the change id, the public epoch id, and the gate key — never
// a capability or reservation token.
func resumeActiveLocator(gateKey string, ep EpochRecord) string {
	return "change " + ep.ChangeID + " has an active run (epoch " + ep.EpochID +
		", gate key " + gateKey + "); cancel it with 'docket run cancel --key " + gateKey +
		" --epoch " + ep.EpochID + " --reason <why>' and resume after confirmed cancellation, " +
		"or continue the live run via 'docket run gate-verdict'"
}

// resumeReplacementParams carries the immutable arm facts armResumeReplacement mints
// the replacement gate record from — captured before the epoch branch so the winner
// and a repeat arm mint an identical-shaped record.
type resumeReplacementParams struct {
	createdAt     int64
	dispatchEpoch int64
	beforeIDs     []int
	attributedID  int
	scopeChangeID string
	branch        string
	worktree      string
	attemptLimit  int
}

// armResumeReplacement admits EXACTLY ONE replacement dispatch after a confirmed
// cancellation (change 0375 Task 12, spec "After confirmed cancellation, atomically
// supersede the old epoch and reserve one replacement dispatch. Two concurrent
// resumes produce one winner"). It prepares the replacement's outer scope, mints its
// gate record, then atomically supersedes the confirmed-cancelled epoch reserving
// THIS key. The supersede CAS is the one-winner serialization point: a loser observes
// the winner's reservation (its own scope/record are inert orphans). The winner binds
// a fresh active epoch to the replacement key — its ChangeID left unbound so a repeat
// resume still resolves the superseded predecessor's reservation — and binds the
// canonical feature worktree so the mutation fence and run.cancel activate for the
// resumed run.
func armResumeReplacement(repoDir string, sdeps GateScopeDeps, oldKey string, p resumeReplacementParams) RunGateBeforeResult {
	grant, serr := sdeps.Prepare(gatedrive.ScopeRequest{
		ChangeID: p.scopeChangeID,
		Branch:   p.branch,
		Worktree: p.worktree,
	})
	if serr != nil {
		return gateUnarmed(ReasonGateScopeFailed)
	}
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:           gateBeforeStoredTarget,
		CreatedAt:        p.createdAt,
		DispatchEpoch:    p.dispatchEpoch,
		BeforeIDs:        p.beforeIDs,
		AttributedID:     p.attributedID,
		Retry:            RetryUnused,
		Disposition:      "gate-armed",
		ScopeID:          grant.ScopeID,
		ParentCap:        grant.ParentCapability,
		ChildContextHash: gateHashToken(grant.ChildCapability),
		AttemptLimit:     p.attemptLimit,
	})
	if err != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}
	if serr2 := SupersedeCancelledEpoch(repoDir, oldKey, key); serr2 != nil {
		if errors.Is(serr2, errEpochAlreadySuperseded) {
			// Lost the one-winner race: recover and report the winner's reservation.
			reserved, _, lerr := LoadEpochRecord(repoDir, oldKey)
			if lerr != nil || reserved.ReplacementReserved == "" {
				return gateUnarmedMsg(ReasonGateResumeEpochUnreadable,
					"lost the replacement race but could not read the winner's reservation")
			}
			return gateResumeObserve(reserved.ReplacementReserved)
		}
		return gateUnarmedMsg(ReasonGateResumeEpochUnreadable,
			"change "+p.scopeChangeID+" could not be superseded for resume")
	}
	epochRec, eerr := MintEpochRecord(repoDir, key, "")
	if eerr != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}
	if werr := epochCAS(repoDir, key, func(rec *EpochRecord) error {
		rec.Worktree = p.worktree
		return nil
	}); werr != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}
	// The replacement dispatch gets a fresh live epoch; surface its public id so the
	// resumed run's Stop path (`run.cancel --epoch`) and `--run-epoch` flags are
	// followable, exactly as a fresh arm's are (change 0375).
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{
		Armed:           true,
		Key:             key,
		Epoch:           epochRec.EpochID,
		Target:          gateBeforeStoredTarget,
		DispatchContext: grant.ChildCapability,
		OwnerLifecycle:  ReasonOwnerLifecycleUnavailable,
	})
}

// validateResumeQuiescence re-proves the OLD epoch's quiescence before resume may
// reserve a replacement (EpochCancelled) or re-authorize a previously reserved one
// (EpochSuperseded) — the same bounded proof terminal repair uses
// (verifyTerminalEpochQuiescence; the cancellation command's last reported
// disposition is not durable authority). For a superseded epoch, whose Worktree
// supersession cleared, that proof resolves the replacement's worktree and runs the
// launch census with the old epoch id against it (change 0446 spec §4). When the
// evidence is accounted it retires, through the shared retirement
// (retireSlotOwnership), a RELEASED slot that still carries the old epoch's
// ownership, so the replacement's own reservation is not refused stale-run-epoch. It
// never cancels or alters an already-reserved successor: a successor-held slot is
// the shared successor outcome, returned as the detail of an ok result. Incomplete
// or unreadable proof returns ok=false with a bounded, credential-free detail for the
// gate-unarmed message; it creates no replacement and yields no dispatch
// authorization.
func validateResumeQuiescence(seams cancelSeams, repoDir string, ep EpochRecord) (ok bool, detail string) {
	slotEp, quiescent, findings := verifyTerminalEpochQuiescence(seams, repoDir, ep)
	if !quiescent {
		return false, strings.Join(findings, "; ")
	}
	r := retireSlotOwnership(seams, slotEp)
	return r.detached, r.finding
}

// RunGateBefore arms the implement-next run gate. On a bad target it returns a
// usage error (non-zero exit); otherwise it re-syncs, reads the in-progress
// claim set, captures the dispatch epoch after that read, optionally verifies an
// explicit resume id, prepares the OUTER recovery scope (change 0359), mints the
// durable record, and returns `gate-armed <key> <dispatch-context>` — degrading
// any arming failure to a `gate-unarmed <reason>` report line that still exits 0.
//
// resumeID (0 = none) requests explicit resume attribution: the id is pre-bound
// as the record's AttributedID ONLY when it is a verified in-progress change with
// a valid WorkspaceInspect identity — never by a timestamp game. A resumed change
// still sits in the fresh BeforeIDs and that is correct: attribution is already
// bound, so the verdict path never re-derives it.
func RunGateBefore(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, sdeps GateScopeDeps, repoDir string, target string, resumeID int) RunGateBeforeResult {
	if target != gateBeforeAcceptedTarget {
		return newRunGateBeforeResult(ResultInvalidInput, RunGateBeforeResult{
			Reason:  ReasonGateInvalidTarget,
			Message: fmt.Sprintf("unsupported target %q; only %q is an accepted gate target", target, gateBeforeAcceptedTarget),
		})
	}

	// CreatedAt is stamped at the start of the arm; DispatchEpoch is captured
	// AFTER the before-read below, so a claim landing at or after the dispatch is
	// distinguishable from one already present. Both are real wall-clock stamps
	// (never the injected transaction clock). Since change 0407 the verdict path
	// binds ownership at claim time (the verified dispatch-to-claim binding), so
	// DispatchEpoch and BeforeIDs no longer feed attribution — they are retained as
	// diagnostics for a human reading the record and can never create retry
	// authority.
	createdAt := time.Now().Unix()

	// (1) Re-sync the metadata worktree to fresh origin. PinContext advances the
	// remote-tracking ref through a targeted fetch — the same fresh-origin re-sync
	// the claim path performs — and pins the metadata revision.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return gateUnarmed(ReasonGateSyncFailed)
	}

	// (2) Read the in-progress claim set (ids only) from the pinned metadata
	// source, through the same corpus read + parse the claim path uses. The full
	// per-id status is retained so a resume can verify the resume id is genuinely
	// in-progress (never a proposed or implemented id).
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return gateUnarmed(ReasonGateChangesUnreadable)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return gateUnarmed(ReasonGateChangesUnreadable)
	}
	var beforeIDs []int
	statusByID := map[int]domain.Status{}
	for _, c := range build.Snapshot.Changes() {
		id := int(c.ID())
		statusByID[id] = c.Status()
		if c.Status() == domain.StatusInProgress {
			beforeIDs = append(beforeIDs, id)
		}
	}
	sort.Ints(beforeIDs)

	// (3) Capture the dispatch epoch AFTER the before-read. A resume never touches
	// this ordering: the resumed change stays in BeforeIDs and DispatchEpoch stays
	// post-read. Neither value is consulted by the verdict path any more (change
	// 0407: keyed attribution binds at claim time); they are recorded as diagnostics
	// only, and a resumed change's ownership is bound below by verified identity.
	dispatchEpoch := time.Now().Unix()

	// (4) Verify an explicit resume id, if requested. Attribution is pre-bound ONLY
	// for a change that is genuinely in-progress AND resolves a valid workspace
	// identity (WorkspaceInspect applied). Anything else — a proposed/implemented
	// id, or a failed inspect — is resume-unverified: no record is minted.
	var (
		attributedID  int
		scopeChangeID string
		branch        string
		worktree      string
	)
	if resumeID != 0 {
		if statusByID[resumeID] != domain.StatusInProgress {
			return gateUnarmed(ReasonGateResumeUnverified)
		}
		insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: resumeID})
		if insp.Result != ResultApplied {
			return gateUnarmed(ReasonGateResumeUnverified)
		}
		attributedID = resumeID
		scopeChangeID = strconv.Itoa(resumeID)
		branch = insp.FeatureRef
		worktree = insp.Path

		// (4a) Resume SHARES the run epoch's admission (change 0375 Task 12, spec
		// "run.gate-before --resume and direct implement-next resume must share the same
		// admission path"). Locate the change's prior epoch; its state decides whether a
		// replacement may be admitted. No prior epoch (a legacy/pre-epoch resume, or an
		// unclaimed run that never bound one) falls through to the existing resume arm,
		// which shares no epoch and reserves no replacement.
		oldKey, oldEp, foundEp, ferr := FindEpochByChange(repoDir, scopeChangeID)
		if ferr != nil {
			return gateUnarmedMsg(ReasonGateResumeEpochUnreadable,
				"the prior run epoch for change "+scopeChangeID+" could not be resolved")
		}
		if foundEp {
			// Resume's old-epoch quiescence validation shares the cancellation seams
			// (change 0435); production composes them over repoDir, unit tests inject
			// permissive or adversarial seams. Resolved once inside this already-serialized
			// resume decision — the epoch-state read plus the supersede CAS's one-winner
			// point — never as a separate unlocked preflight.
			seams := productionCancelSeams(repoDir)
			if sdeps.CancelSeams != nil {
				seams = sdeps.CancelSeams(repoDir)
			}
			switch oldEp.State {
			case EpochActive:
				// The prior run can still act on the worktree: refuse with the safe locator
				// and the explicit cancel/continue remedy; never shut the incumbent down.
				return gateUnarmedMsg(ReasonGateResumeActiveRun, resumeActiveLocator(oldKey, oldEp))
			case EpochCancelling:
				// Cancellation cleanup is still in progress: admit no replacement.
				return gateUnarmedMsg(ReasonGateResumeCancellationPending,
					"change "+scopeChangeID+" is cancelling (epoch "+oldEp.EpochID+
						"); finish cancellation with 'docket run cancel' before resuming")
			case EpochSuperseded:
				// A replacement was already reserved (a lost response or a repeat arm):
				// observe that reservation rather than admitting a second. Re-authorizing
				// it still requires the old epoch to be quiescent (change 0435) — the same
				// bounded proof terminal repair uses; the cancellation command's last
				// reported disposition is not durable authority. The successor reservation
				// is never altered.
				if oldEp.ReplacementReserved == "" {
					return gateUnarmedMsg(ReasonGateResumeEpochUnreadable,
						"change "+scopeChangeID+" was superseded without a recorded replacement")
				}
				if qok, detail := validateResumeQuiescence(seams, repoDir, oldEp); !qok {
					return gateUnarmedMsg(ReasonGateResumeCancellationPending,
						"change "+scopeChangeID+" has unresolved cancellation evidence ("+detail+
							"); the reserved replacement cannot be re-authorized until it is resolved")
				}
				return gateResumeObserve(oldEp.ReplacementReserved)
			case EpochCancelled:
				// Confirmed cancellation: re-prove the old epoch's quiescence and retire a
				// released slot that still carries its ownership (change 0435), then
				// atomically supersede and reserve exactly one replacement dispatch (one
				// winner under a concurrent-resume race). Unresolved evidence refuses on the
				// existing gate-unarmed channel rather than reserving over an unquiesced run.
				if qok, detail := validateResumeQuiescence(seams, repoDir, oldEp); !qok {
					return gateUnarmedMsg(ReasonGateResumeCancellationPending,
						"change "+scopeChangeID+" has unresolved cancellation evidence ("+detail+
							"); resume cannot reserve a replacement — resolve it with 'docket run cancel'")
				}
				return armResumeReplacement(repoDir, sdeps, oldKey, resumeReplacementParams{
					createdAt:     createdAt,
					dispatchEpoch: dispatchEpoch,
					beforeIDs:     beforeIDs,
					attributedID:  attributedID,
					scopeChangeID: scopeChangeID,
					branch:        branch,
					worktree:      worktree,
					attemptLimit:  pin.Config.Effective.Run.MaxAttempts.Value,
				})
			case EpochCompleting:
				// A verified successful run is durably fenced mid-closeout (change 0441):
				// the epoch still owns the worktree, so resume must not relabel it a
				// cancelled predecessor or reserve a replacement. Finish the closeout with
				// the keyed verdict, or cancel explicitly.
				return gateUnarmedMsg(ReasonGateResumeRunCompleting,
					"change "+scopeChangeID+" completed its run and is closing out (epoch "+
						oldEp.EpochID+"); re-run the keyed 'docket run gate-verdict' to finish "+
						"closeout, or cancel explicitly with 'docket run cancel'")
			case EpochCompleted:
				// The successful closeout finished (change 0441): terminal, nothing to
				// resume — never quiescence-checked into a supersede, never a replacement
				// reservation. Verify and finalize instead.
				return gateUnarmedMsg(ReasonGateResumeRunCompleted,
					"change "+scopeChangeID+"'s run completed successfully (epoch "+
						oldEp.EpochID+"); there is nothing to resume — verify with 'docket run "+
						"verify --id "+scopeChangeID+"' and finalize instead")
			default:
				return gateUnarmedMsg(ReasonGateResumeEpochUnreadable,
					"change "+scopeChangeID+" has an unrecognized run epoch state")
			}
		}
	}

	// (5) Prepare the OUTER recovery scope. The grant's ChildCapability becomes the
	// dispatch context the parent hands the child; its hash links every nested
	// drive to this outer gate. The ParentCapability is retained in the 0600
	// record and never printed. When resuming, the scope's ChangeID is pre-bound to
	// the verified id, and its Branch/Worktree carry the resumed change's identity.
	grant, serr := sdeps.Prepare(gatedrive.ScopeRequest{
		ChangeID: scopeChangeID,
		Branch:   branch,
		Worktree: worktree,
	})
	if serr != nil {
		return gateUnarmed(ReasonGateScopeFailed)
	}

	// (6) Mint the durable record. Schema and Repo are stamped by the store. The
	// ParentCap is persisted (0600) but never surfaces in the result or a report
	// line; ChildContextHash is the sha256 of the printed dispatch context.
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:           gateBeforeStoredTarget,
		CreatedAt:        createdAt,
		DispatchEpoch:    dispatchEpoch,
		BeforeIDs:        beforeIDs,
		AttributedID:     attributedID,
		Retry:            RetryUnused,
		Disposition:      "gate-armed",
		ScopeID:          grant.ScopeID,
		ParentCap:        grant.ParentCapability,
		ChildContextHash: gateHashToken(grant.ChildCapability),
		// AttemptLimit snapshots run.max_attempts (change 0421) from the SAME
		// authoritative config load the arm already performed — pin.Config.Effective is
		// the resolved snapshot PinContext returned above and BuildSnapshot consumed, so
		// this is not a second resolver. The value is immutable once minted: a later
		// config edit never rewrites an already-owned budget (the snapshot rule). Config
		// validation floors run.max_attempts at 1, so this satisfies the v4 store guard.
		AttemptLimit: pin.Config.Effective.Run.MaxAttempts.Value,
	})
	if err != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}

	// (6a) A fresh (non-resume) arm binds a NEW run epoch beside the just-minted gate
	// record, keyed by the gate key (rungate_epoch.go). The epoch is the durable
	// coordinator fence a later human cancellation flips and a resume supersedes; its
	// EpochID travels onto each scoped start's worktree slot so an omitted or stale
	// epoch cannot detach the worktree. A mint failure unarms fail-closed: an armed
	// gate must carry a live epoch (the orphan gate record left behind is inert — no
	// key is returned, so nothing dispatches against it). A resume arm does NOT mint
	// here: it shares the change's existing epoch, whose supersede-and-reserve is
	// Task 12's; for change 0375 Task 9 only the fresh arm binds an epoch.
	var epochID string
	if resumeID == 0 {
		epochRec, eerr := MintEpochRecord(repoDir, key, scopeChangeID)
		if eerr != nil {
			return gateUnarmed(ReasonGateMintFailed)
		}
		// Surface the just-minted public epoch id so the documented Stop path is
		// followable: `run.cancel --epoch <id>` and every `--run-epoch` dispatch flag
		// consume exactly this value (change 0375).
		epochID = epochRec.EpochID
	}

	// (7) Report the armed gate with its dispatch context, its run epoch id, and the
	// honest owner-lifecycle caveat: the dispatched route has no automatic Stop, so a
	// Stop is the explicit `run.cancel` operation keyed by this epoch (change 0375
	// Task 13).
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{
		Armed:           true,
		Key:             key,
		Epoch:           epochID,
		Target:          gateBeforeStoredTarget,
		DispatchContext: grant.ChildCapability,
		OwnerLifecycle:  ReasonOwnerLifecycleUnavailable,
	})
}
