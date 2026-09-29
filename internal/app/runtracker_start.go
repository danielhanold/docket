package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `docket run start` operation (change 0334, Task 2): it
// ARMS the implement-next run gate. It re-syncs the metadata worktree to fresh
// origin, reads the current in-progress claim set, captures a dispatch epoch
// AFTER that read, and mints a durable gate record under the git common dir
// (runtracker_store.go). Its whole contract is the printed report line:
// `run-started <key> <epoch> <dispatch-context>` on success, `run-untracked
// <reason-token>` on any failure — both exit 0 (learning
// exit-code-encodes-a-non-failure). Only `implement-next` is an accepted
// target; anything else is a usage error that exits non-zero.
//
// It writes NO metadata: the fresh-origin re-sync and the change-file parse are
// the SAME plumbing the claim path uses (PinContext advances the remote-tracking
// ref; ReadCorpus + parseCorpus + repository.BuildSnapshot yield the snapshot),
// reused rather than reimplemented (learning duplicated-gate-copies-the-whole-
// predicate). The only durable write is the gate record.

// OperationRunStart is the operation key `run start` records in its
// envelope.
const OperationRunStart = "run.start"

// runStartAcceptedTarget is the sole accepted target argument (the workflow
// name, not the agent name); runStartStoredTarget is the canonical agent name
// the durable record stores as its Target — the thing a dispatch actually
// launches. Attribution downstream keys on the workflow the gate brackets, so
// the two spellings are pinned here, not derived.
const (
	runStartAcceptedTarget = "implement-next"
	runStartStoredTarget   = "docket-implement-next"
)

// The stable reason tokens a run-untracked line carries. Each names the arming
// step that failed; a consumer keys on the token, never the prose.
const (
	// ReasonRunInvalidTarget: the target argument was not `implement-next`. This
	// is a usage error (non-zero exit), not a run-untracked report line.
	ReasonRunInvalidTarget = "invalid-target"
	// ReasonRunSyncFailed: the fresh-origin metadata re-sync (PinContext) failed,
	// so the before-read could not be taken from authoritative state.
	ReasonRunSyncFailed = "sync-failed"
	// ReasonRunChangesUnreadable: the changes corpus could not be read or parsed
	// into a snapshot, so the in-progress claim set is unknown.
	ReasonRunChangesUnreadable = "changes-unreadable"
	// ReasonRunMintFailed: the durable gate record could not be minted (the git
	// common dir was unresolvable, or the write failed).
	ReasonRunMintFailed = "mint-failed"
	// ReasonRunResumeUnverified: a --resume id could not be verified as an
	// already-in-progress change with a valid workspace identity, so the gate
	// refuses to pre-bind attribution to it. No record is minted (change 0359).
	ReasonRunResumeUnverified = "resume-unverified"
	// ReasonRunScopeFailed: the outer recovery scope could not be prepared, so no
	// dispatch context exists to hand the child. No record is minted (change 0359).
	ReasonRunScopeFailed = "scope-failed"
	// ReasonRunResumeActiveRun: a --resume id names a change whose prior run epoch is
	// still ACTIVE — an earlier coordinator can still act on the worktree. Resume
	// refuses with a safe locator and the explicit cancel/continue remedy; it never
	// shuts the incumbent down (change 0375 Task 12, spec "If the prior run is still
	// active, refuse with its safe locator and explicit cancel/continue remedy").
	ReasonRunResumeActiveRun = "resume-active-run"
	// ReasonRunResumeCancellationPending: a --resume id's prior epoch is CANCELLING
	// or otherwise unresolved — cancellation cleanup is still in progress, so no
	// replacement is admitted (spec "If cancelling or unresolved, resume
	// cleanup/observation and admit no replacement"). Repeatable via run.cancel.
	ReasonRunResumeCancellationPending = CancelDispositionPending
	// ReasonRunResumeReplacementReserved: a --resume id's prior epoch was already
	// superseded and a replacement dispatch is reserved (a lost response, a repeat
	// arm, or the loser of a concurrent-resume race). It observes that reservation —
	// the reserved gate key is returned in Key — and reserves no second replacement
	// (spec "Lost responses and repeat arms observe/recover that reservation; they do
	// not create another epoch or re-dispatch").
	ReasonRunResumeReplacementReserved = "resume-replacement-reserved"
	// ReasonResumeRunRecordUnreadable: the prior run epoch could not be read or its
	// supersede transition faulted — fail closed rather than admit a replacement over
	// an unresolvable run (change 0375 Task 12).
	ReasonResumeRunRecordUnreadable = "resume-run-record-unreadable"
	// ReasonRunResumeRunCompleting: a --resume id's prior epoch is COMPLETING — a
	// keyed verdict verified the run complete and durably fenced the epoch, but
	// closeout is unfinished, so the epoch still owns its worktree (change 0441).
	// Resume never turns a completing run into a cancelled predecessor or reserves a
	// replacement; the remedy is the keyed 'docket run verdict' (finish closeout)
	// or an explicit 'docket run cancel'.
	ReasonRunResumeRunCompleting = "resume-run-completing"
	// ReasonRunResumeRunCompleted: a --resume id's prior epoch is COMPLETED — the
	// successful closeout finished and the run retired terminally (change 0441). There
	// is nothing to resume: the remedy is 'docket run verify' and finalize. Never
	// quiescence-checked into a supersede, never a replacement reservation.
	ReasonRunResumeRunCompleted = "resume-run-completed"
	// ReasonOwnerLifecycleUnavailable is the honest limitation an armed gate reports
	// (change 0375 Task 13): the default dispatch route has NO owner-death or Stop
	// lifecycle event that would cancel the run automatically, so a Stop is the
	// explicit `run.cancel` operation. Only the Codex `agent.enter` route carries a
	// signal-connected cancellation and a death guardian; every other route relies on
	// the human running `run.cancel` (named in the skills prose). It is a standing
	// caveat, never a refusal — an armed gate still arms.
	ReasonOwnerLifecycleUnavailable = "owner-lifecycle-unavailable"
)

// RunTrackerScopeDeps carries the outer-scope preparation seam run start composes
// so unit tests can fake the durable scope mint. Production wiring (internal/cli/
// run.go) composes gatedrive.OpenStore(<git-common-dir>).PrepareScope; a unit
// test injects a fake that records the request and returns a canned grant.
type RunTrackerScopeDeps struct {
	Prepare func(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error)
	// CancelSeams overrides the cancellation seams the resume path's old-epoch
	// quiescence validation composes (change 0435); nil composes
	// productionCancelSeams(repoDir). Unit tests inject permissive or adversarial
	// seams; production callers leave it nil.
	CancelSeams func(repoDir string) cancelSeams
}

// runTrackerHashToken returns the sha256 of a raw token as lowercase hex — the single
// boundary at which the printed dispatch context becomes the stored
// ChildContextHash, so the persisted record links to a nested drive's
// RunContextHash without carrying the raw capability. It matches gatedrive's own
// capHash so the two hashes compare equal.
func runTrackerHashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RunStartResult is the protocol-v1 document `run start` returns. On
// an armed gate Result is applied and Key names the durable record; on a
// run-untracked report Result is still applied (the report line exits 0) and
// Reason carries the stable token. A usage error (bad target) carries a
// non-applied Result and no report line. It never carries authored document
// bodies.
type RunStartResult struct {
	Envelope
	Started bool   `json:"started"`
	Key     string `json:"key,omitempty"`
	// RunContext is the outer scope's ChildCapability the parent copies into
	// the implement-next dispatch prompt; a nested drive carries its hash as the
	// RunContextHash. It is NOT secret from the child (change 0359). The parent
	// capability is deliberately absent from this result — it lives only in the
	// 0600-private gate record.
	RunContext string `json:"dispatch_context,omitempty"`
	// Epoch is the fresh run's PUBLIC epoch id, minted at arm time beside the gate
	// record (runtracker_run_record.go). It authorizes nothing (ADR-0111) but is the locator
	// the operator threads into `run.cancel --run-id <id>` — the primary human Stop —
	// and the dispatcher threads into each `--run-id` flag (agent.enter, gate drive
	// start, gate drive prepare-scope). Without it the documented Stop path names an
	// epoch the arm never surfaced (change 0375). Never empty on an armed result
	// (change 0463): startedRunResult refuses to arm without one, so the positional
	// `run-started <key> <epoch> <dispatch-context>` line always has three tokens.
	RunID   string `json:"run_id,omitempty"`
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

// HumanText renders the one report line. An armed gate prints `run-started <key>
// <epoch> <dispatch-context>`. That is always three tokens, because every armed
// result carries an epoch (startedRunResult, change 0463), so a positional
// parser can never read the dispatch context as the epoch. A run-untracked
// report prints `run-untracked <reason-token>`; a usage error (a non-applied
// result) names its reason instead of a report line. The parent capability
// never appears here — only the child dispatch context, which is meant for the
// child.
func (r RunStartResult) HumanText() string {
	if r.Result == ResultApplied {
		if r.Started {
			line := "run-started " + r.Key + " " + r.RunID + " " + r.RunContext
			if r.OwnerLifecycle != "" {
				// Honest standing caveat: the dispatched route cancels no run on owner
				// death; a Stop is the explicit `run.cancel` operation.
				line += "\n" + r.OwnerLifecycle
			}
			return line
		}
		return "run-untracked " + r.Reason
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newRunStartResult stamps the envelope for the operation.
func newRunStartResult(result Result, out RunStartResult) RunStartResult {
	out.Envelope = NewEnvelope(OperationRunStart, result)
	return out
}

// runUntracked builds a run-untracked report line: a success-shaped envelope
// (exit 0) carrying the stable reason token. It never mints a record.
func runUntracked(reason string) RunStartResult {
	return newRunStartResult(ResultApplied, RunStartResult{Started: false, Reason: reason})
}

// runUntrackedMsg is runUntracked with a bounded human Message — a safe locator and
// remedy for a resume refusal. The Message carries only public locators (a gate key,
// a public epoch id, a change id), never a capability or reservation token.
func runUntrackedMsg(reason, message string) RunStartResult {
	return newRunStartResult(ResultApplied, RunStartResult{Started: false, Reason: reason, Message: message})
}

// runTrackerResumeObserve builds the observe-the-reservation report a repeat arm or the
// loser of a concurrent-resume race returns: run-untracked with the winner's reserved
// gate key in Key, so the caller recovers the single reserved replacement rather than
// admitting a second (change 0375 Task 12). It mints no record and no epoch.
func runTrackerResumeObserve(reservedKey string) RunStartResult {
	return newRunStartResult(ResultApplied, RunStartResult{
		Started: false,
		Reason:  ReasonRunResumeReplacementReserved,
		Key:     reservedKey,
		Message: "a replacement dispatch is already reserved under gate key " + reservedKey + "; a second cannot be armed",
	})
}

// startedRunResult builds the armed report for key. Every armed gate carries a run
// epoch (change 0463): parents read the `run-started <key> <epoch> <dispatch-context>`
// line positionally, and both tokens are 32-hex, so the line is unambiguous only
// when the epoch slot is always filled. An empty epoch therefore fails closed as
// run-untracked mint-failed. It never prints a two-token line whose dispatch context
// a parent would read as the epoch.
func startedRunResult(key, runID, runContext string) RunStartResult {
	if runID == "" {
		return runUntracked(ReasonRunMintFailed)
	}
	return newRunStartResult(ResultApplied, RunStartResult{
		Started:        true,
		Key:            key,
		RunID:          runID,
		Target:         runStartStoredTarget,
		RunContext:     runContext,
		OwnerLifecycle: ReasonOwnerLifecycleUnavailable,
	})
}

// resumeActiveLocator renders the safe locator and explicit cancel/continue remedy
// a resume prints when the prior run is still active (change 0375 Task 12). It names
// only public locators — the change id, the public epoch id, and the gate key — never
// a capability or reservation token.
func resumeActiveLocator(runKey string, ep RunRecord) string {
	return "change " + ep.ChangeID + " has an active run (epoch " + ep.RunID +
		", gate key " + runKey + "); " + resumeIncumbentRemedy(runKey, ep.RunID)
}

// resumeIncumbentRemedy renders the two remedies for a resume refused over a live
// incumbent epoch. An epochless resume arm binds its epoch when armed (change 0463),
// so the incumbent may be an arm that was never dispatched. Nothing records whether an
// agent is using the epoch, so the remedy names both cases rather than guessing.
func resumeIncumbentRemedy(runKey, runID string) string {
	return "if it was never dispatched or its agent has exited, cancel it with 'docket run cancel --key " +
		runKey + " --run-id " + runID + " --reason <why>' and resume after confirmed cancellation; " +
		"if its agent is still running, continue the live run via 'docket run verdict'"
}

// acquireResumeLock takes the exclusive per-change resume lock that serializes
// `run start --resume` arms of one change (change 0463). It lives outside the
// rungate root, under <git-common-dir>/docket/run-tracker-resume/<change-id>, so the
// scanners that walk gate-key directories never see it. Closing the returned file
// releases the lock.
func acquireResumeLock(repoDir, changeID string) (*os.File, error) {
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(common, "docket", runTrackerResumeDirName, changeID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, runErr(ErrRunRecordIO, "resume-lock-dir", err)
	}
	return acquireRunLock(dir)
}

// resumeWorktreeOwnerRefusal checks whether a live run epoch already owns the
// verified worktree an epochless resume is about to bind (change 0463 decision 4).
// It resolves the owner the same way the mutation fence does (findRunByWorktree
// over the canonical path; an uncanonicalizable path is matched by its own
// spelling). An active, completing, or unrecognized owner refuses resume-active-run
// with that owner's locator. A fenced owner (cancelling, cancelled, superseded) is
// no live owner: a fresh active epoch outranks it, so it does not block. An
// ambiguous or unreadable owner set refuses resume-run-record-unreadable, fail-closed.
// It returns refused=false when the resume may mint.
func resumeWorktreeOwnerRefusal(repoDir, worktree string) (RunStartResult, bool) {
	canon := worktree
	if c, err := canonicalWorktree(worktree); err == nil {
		canon = c
	}
	ownerKey, found, err := findRunByWorktree(repoDir, canon)
	if err != nil {
		return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
			"the run epoch owning worktree "+worktree+" could not be resolved: "+err.Error()), true
	}
	if !found {
		return RunStartResult{}, false
	}
	owner, _, lerr := LoadRunRecord(repoDir, ownerKey)
	if lerr != nil {
		return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
			"the run epoch owning worktree "+worktree+" (gate key "+ownerKey+") could not be read"), true
	}
	switch owner.State {
	case RunCancelling, RunCancelled, RunSuperseded:
		return RunStartResult{}, false
	}
	return runUntrackedMsg(ReasonRunResumeActiveRun, resumeWorktreeOwnerLocator(worktree, ownerKey, owner)), true
}

// resumeWorktreeOwnerLocator renders the safe locator and the cancel/continue remedy
// for a live epoch that owns the resume's worktree without naming the resumed
// change. Like resumeActiveLocator it names only public locators.
func resumeWorktreeOwnerLocator(worktree, runKey string, ep RunRecord) string {
	owner := "a live run with no bound change"
	if ep.ChangeID != "" {
		owner = "a live run of change " + ep.ChangeID
	}
	return "worktree " + worktree + " is already owned by " + owner + " (state " + string(ep.State) +
		", epoch " + ep.RunID + ", gate key " + runKey + "); " + resumeIncumbentRemedy(runKey, ep.RunID)
}

// resumeReplacementParams carries the immutable arm facts armResumeReplacement mints
// the replacement gate record from — captured before the epoch branch so the winner
// and a repeat arm mint an identical-shaped record.
type resumeReplacementParams struct {
	createdAt     int64
	dispatchedAt  int64
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
func armResumeReplacement(repoDir string, sdeps RunTrackerScopeDeps, oldKey string, p resumeReplacementParams) RunStartResult {
	grant, serr := sdeps.Prepare(gatedrive.ScopeRequest{
		ChangeID: p.scopeChangeID,
		Branch:   p.branch,
		Worktree: p.worktree,
	})
	if serr != nil {
		return runUntracked(ReasonRunScopeFailed)
	}
	key, err := MintRunTrackerRecord(repoDir, RunTrackerRecord{
		Target:           runStartStoredTarget,
		CreatedAt:        p.createdAt,
		DispatchedAt:     p.dispatchedAt,
		BeforeIDs:        p.beforeIDs,
		AttributedID:     p.attributedID,
		Retry:            RetryUnused,
		Disposition:      "run-started",
		ScopeID:          grant.ScopeID,
		ParentCap:        grant.ParentCapability,
		ChildContextHash: runTrackerHashToken(grant.ChildCapability),
		AttemptLimit:     p.attemptLimit,
	})
	if err != nil {
		return runUntracked(ReasonRunMintFailed)
	}
	if serr2 := SupersedeCancelledRun(repoDir, oldKey, key); serr2 != nil {
		if errors.Is(serr2, errRunAlreadySuperseded) {
			// Lost the one-winner race: recover and report the winner's reservation.
			reserved, _, lerr := LoadRunRecord(repoDir, oldKey)
			if lerr != nil || reserved.ReplacementReserved == "" {
				return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
					"lost the replacement race but could not read the winner's reservation")
			}
			return runTrackerResumeObserve(reserved.ReplacementReserved)
		}
		return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
			"change "+p.scopeChangeID+" could not be superseded for resume")
	}
	runRec, eerr := MintRunRecord(repoDir, key, "")
	if eerr != nil {
		return runUntracked(ReasonRunMintFailed)
	}
	if werr := runRecordCAS(repoDir, key, func(rec *RunRecord) error {
		rec.Worktree = p.worktree
		return nil
	}); werr != nil {
		return runUntracked(ReasonRunMintFailed)
	}
	// The replacement dispatch gets a fresh live epoch; surface its public id so the
	// resumed run's Stop path (`run.cancel --run-id`) and `--run-id` flags are
	// followable, exactly as a fresh arm's are (change 0375).
	return startedRunResult(key, runRec.RunID, grant.ChildCapability)
}

// validateResumeQuiescence re-proves the OLD epoch's quiescence before resume may
// reserve a replacement (RunCancelled) or re-authorize a previously reserved one
// (RunSuperseded) — the same bounded proof terminal repair uses
// (verifyTerminalRunQuiescence; the cancellation command's last reported
// disposition is not durable authority). For a superseded epoch, whose Worktree
// supersession cleared, that proof resolves the replacement's worktree and runs the
// launch census with the old epoch id against it (change 0446 spec §4). When the
// evidence is accounted it retires, through the shared retirement
// (retireSlotOwnership), a RELEASED slot that still carries the old epoch's
// ownership, so the replacement's own reservation is not refused stale-run-id. It
// never cancels or alters an already-reserved successor: a successor-held slot is
// the shared successor outcome, returned as the detail of an ok result. Incomplete
// or unreadable proof returns ok=false with a bounded, credential-free detail for the
// run-untracked message; it creates no replacement and yields no dispatch
// authorization. worktree is the resume request's verified feature worktree — the one
// armResumeReplacement binds — which resolves a torn replacement chain (a replacement
// epoch never minted or never bound) instead of dead-ending every repeat resume.
func validateResumeQuiescence(seams cancelSeams, repoDir string, ep RunRecord, worktree string) (ok bool, detail string) {
	slotEp, quiescent, findings := verifyTerminalRunQuiescence(seams, repoDir, ep, worktree)
	if !quiescent {
		return false, strings.Join(findings, "; ")
	}
	r := retireSlotOwnership(seams, slotEp)
	return r.detached, r.finding
}

// RunStart arms the implement-next run gate. On a bad target it returns a
// usage error (non-zero exit); otherwise it re-syncs, reads the in-progress
// claim set, captures the dispatch epoch after that read, optionally verifies an
// explicit resume id, prepares the OUTER recovery scope (change 0359), mints the
// durable record and its run epoch, and returns `run-started <key> <epoch>
// <dispatch-context>` — degrading any arming failure to a `run-untracked <reason>`
// report line that still exits 0.
//
// resumeID (0 = none) requests explicit resume attribution: the id is pre-bound
// as the record's AttributedID ONLY when it is a verified in-progress change with
// a valid WorkspaceInspect identity — never by a timestamp game. A resumed change
// still sits in the fresh BeforeIDs and that is correct: attribution is already
// bound, so the verdict path never re-derives it.
func RunStart(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, sdeps RunTrackerScopeDeps, repoDir string, target string, resumeID int) RunStartResult {
	if target != runStartAcceptedTarget {
		return newRunStartResult(ResultInvalidInput, RunStartResult{
			Reason:  ReasonRunInvalidTarget,
			Message: fmt.Sprintf("unsupported target %q; only %q is an accepted gate target", target, runStartAcceptedTarget),
		})
	}

	// CreatedAt is stamped at the start of the arm; DispatchedAt is captured
	// AFTER the before-read below, so a claim landing at or after the dispatch is
	// distinguishable from one already present. Both are real wall-clock stamps
	// (never the injected transaction clock). Since change 0407 the verdict path
	// binds ownership at claim time (the verified dispatch-to-claim binding), so
	// DispatchedAt and BeforeIDs no longer feed attribution — they are retained as
	// diagnostics for a human reading the record and can never create retry
	// authority.
	createdAt := time.Now().Unix()

	// (1) Re-sync the metadata worktree to fresh origin. PinContext advances the
	// remote-tracking ref through a targeted fetch — the same fresh-origin re-sync
	// the claim path performs — and pins the metadata revision.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return runUntracked(ReasonRunSyncFailed)
	}

	// (2) Read the in-progress claim set (ids only) from the pinned metadata
	// source, through the same corpus read + parse the claim path uses. The full
	// per-id status is retained so a resume can verify the resume id is genuinely
	// in-progress (never a proposed or implemented id).
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return runUntracked(ReasonRunChangesUnreadable)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return runUntracked(ReasonRunChangesUnreadable)
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
	// this ordering: the resumed change stays in BeforeIDs and DispatchedAt stays
	// post-read. Neither value is consulted by the verdict path any more (change
	// 0407: keyed attribution binds at claim time); they are recorded as diagnostics
	// only, and a resumed change's ownership is bound below by verified identity.
	dispatchedAt := time.Now().Unix()

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
			return runUntracked(ReasonRunResumeUnverified)
		}
		insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: resumeID})
		if insp.Result != ResultApplied {
			return runUntracked(ReasonRunResumeUnverified)
		}
		attributedID = resumeID
		scopeChangeID = strconv.Itoa(resumeID)
		branch = insp.FeatureRef
		worktree = insp.Path

		// Serialize every resume arm of this change from here to the end of the arm
		// (change 0463). The checks below decide from the epochs that exist now, and
		// the arm then mints and binds one, so two arms must never interleave between
		// check and bind. The lock is keyed by change id; a resume's worktree is the
		// change's own feature worktree, so it also covers step 4b's worktree check.
		lock, lerr := acquireResumeLock(repoDir, scopeChangeID)
		if lerr != nil {
			return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
				"the resume lock for change "+scopeChangeID+" could not be taken: "+lerr.Error())
		}
		defer lock.Close()

		// (4a) Resume SHARES the run epoch's admission (change 0375 Task 12, spec
		// "run.start --resume and direct implement-next resume must share the same
		// admission path"). Locate the change's prior epoch; its state decides whether a
		// replacement may be admitted. No prior epoch (a legacy/pre-epoch resume, or a
		// first dispatch that was never armed) falls through to the ordinary arm below,
		// which mints and binds a fresh epoch for the resumed change (step 6a, change
		// 0463), so the armed line always carries one.
		oldKey, oldEp, foundEp, ferr := FindRunByChange(repoDir, scopeChangeID)
		if ferr != nil {
			return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
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
			case RunActive:
				// The prior run can still act on the worktree: refuse with the safe locator
				// and the explicit cancel/continue remedy; never shut the incumbent down.
				return runUntrackedMsg(ReasonRunResumeActiveRun, resumeActiveLocator(oldKey, oldEp))
			case RunCancelling:
				// Cancellation cleanup is still in progress: admit no replacement.
				return runUntrackedMsg(ReasonRunResumeCancellationPending,
					"change "+scopeChangeID+" is cancelling (epoch "+oldEp.RunID+
						"); finish cancellation with 'docket run cancel' before resuming")
			case RunSuperseded:
				// A replacement was already reserved (a lost response or a repeat arm):
				// observe that reservation rather than admitting a second. Re-authorizing
				// it still requires the old epoch to be quiescent (change 0435) — the same
				// bounded proof terminal repair uses; the cancellation command's last
				// reported disposition is not durable authority. The successor reservation
				// is never altered.
				if oldEp.ReplacementReserved == "" {
					return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
						"change "+scopeChangeID+" was superseded without a recorded replacement")
				}
				// A torn replacement (never minted or never bound) resolves through the
				// request's own verified worktree, so a repeat arm converges on the
				// reservation instead of dead-ending (resolveTerminalRunSlot).
				if qok, detail := validateResumeQuiescence(seams, repoDir, oldEp, worktree); !qok {
					return runUntrackedMsg(ReasonRunResumeCancellationPending,
						"change "+scopeChangeID+" has unresolved cancellation evidence ("+detail+
							"); the reserved replacement cannot be re-authorized until it is resolved — "+
							"settle it with 'docket run cancel --key "+oldKey+" --run-id "+oldEp.RunID+
							" --reason <why>', then re-run this resume (a record named unreadable or "+
							"cyclic is never inferred safe and must be readable again first)")
				}
				return runTrackerResumeObserve(oldEp.ReplacementReserved)
			case RunCancelled:
				// Confirmed cancellation: re-prove the old epoch's quiescence and retire a
				// released slot that still carries its ownership (change 0435), then
				// atomically supersede and reserve exactly one replacement dispatch (one
				// winner under a concurrent-resume race). Unresolved evidence refuses on the
				// existing run-untracked channel rather than reserving over an unquiesced run.
				if qok, detail := validateResumeQuiescence(seams, repoDir, oldEp, worktree); !qok {
					return runUntrackedMsg(ReasonRunResumeCancellationPending,
						"change "+scopeChangeID+" has unresolved cancellation evidence ("+detail+
							"); resume cannot reserve a replacement — resolve it with 'docket run cancel'")
				}
				return armResumeReplacement(repoDir, sdeps, oldKey, resumeReplacementParams{
					createdAt:     createdAt,
					dispatchedAt:  dispatchedAt,
					beforeIDs:     beforeIDs,
					attributedID:  attributedID,
					scopeChangeID: scopeChangeID,
					branch:        branch,
					worktree:      worktree,
					attemptLimit:  pin.Config.Effective.Run.MaxAttempts.Value,
				})
			case RunCompleting:
				// A verified successful run is durably fenced mid-closeout (change 0441):
				// the epoch still owns the worktree, so resume must not relabel it a
				// cancelled predecessor or reserve a replacement. Finish the closeout with
				// the keyed verdict, or cancel explicitly.
				return runUntrackedMsg(ReasonRunResumeRunCompleting,
					"change "+scopeChangeID+" completed its run and is closing out (epoch "+
						oldEp.RunID+"); re-run the keyed 'docket run verdict' to finish "+
						"closeout, or cancel explicitly with 'docket run cancel'")
			case RunCompleted:
				// The successful closeout finished (change 0441): terminal, nothing to
				// resume — never quiescence-checked into a supersede, never a replacement
				// reservation. Verify and finalize instead.
				return runUntrackedMsg(ReasonRunResumeRunCompleted,
					"change "+scopeChangeID+"'s run completed successfully (epoch "+
						oldEp.RunID+"); there is nothing to resume — verify with 'docket run "+
						"verify --id "+scopeChangeID+"' and finalize instead")
			default:
				return runUntrackedMsg(ReasonResumeRunRecordUnreadable,
					"change "+scopeChangeID+" has an unrecognized run epoch state")
			}
		}

		// (4b) No epoch names the change, so step 6a will mint one and bind it to the
		// verified worktree. FindRunByChange cannot see a live epoch that owns this
		// worktree under no change or another change. Minting over one would leave two
		// live owners of one worktree, and findRunByWorktree would then refuse every
		// fenced mutation there (PR publish, workspace publish) as
		// ErrRunOwnerAmbiguous. Refuse with the incumbent's locator instead.
		if refusal, refused := resumeWorktreeOwnerRefusal(repoDir, worktree); refused {
			return refusal
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
		return runUntracked(ReasonRunScopeFailed)
	}

	// (6) Mint the durable record. Schema and Repo are stamped by the store. The
	// ParentCap is persisted (0600) but never surfaces in the result or a report
	// line; ChildContextHash is the sha256 of the printed dispatch context.
	key, err := MintRunTrackerRecord(repoDir, RunTrackerRecord{
		Target:           runStartStoredTarget,
		CreatedAt:        createdAt,
		DispatchedAt:     dispatchedAt,
		BeforeIDs:        beforeIDs,
		AttributedID:     attributedID,
		Retry:            RetryUnused,
		Disposition:      "run-started",
		ScopeID:          grant.ScopeID,
		ParentCap:        grant.ParentCapability,
		ChildContextHash: runTrackerHashToken(grant.ChildCapability),
		// AttemptLimit snapshots run.max_attempts (change 0421) from the SAME
		// authoritative config load the arm already performed — pin.Config.Effective is
		// the resolved snapshot PinContext returned above and BuildSnapshot consumed, so
		// this is not a second resolver. The value is immutable once minted: a later
		// config edit never rewrites an already-owned budget (the snapshot rule). Config
		// validation floors run.max_attempts at 1, so this satisfies the v4 store guard.
		AttemptLimit: pin.Config.Effective.Run.MaxAttempts.Value,
	})
	if err != nil {
		return runUntracked(ReasonRunMintFailed)
	}

	// (6a) Every armed gate binds a run epoch beside the just-minted gate record,
	// keyed by the gate key (runtracker_run_record.go). The epoch is the durable coordinator
	// fence that a later human cancellation flips and a resume supersedes. Its RunID
	// travels onto each scoped start's worktree slot, so an omitted or stale epoch
	// cannot detach the worktree. Two arms reach this step: a FRESH arm, and a RESUME
	// whose change has no prior epoch (a legacy/pre-epoch run, or a first dispatch
	// that was never armed; change 0463). A resume that found a prior epoch never gets
	// here, because every found state has already returned above (a refusal, an
	// observed reservation, or armResumeReplacement, which mints its own).
	//
	// The epoch is minted unbound. A fresh arm binds ChangeID and Worktree later, at
	// claim confirmation (bindRunChange / bindRunWorktree). A resume has already
	// claimed, so it binds both NOW, in one runRecordCAS, the same way
	// armResumeReplacement binds its worktree. Why both are needed:
	//   - runLaunchGate refuses an active epoch that has no Worktree, so an unbound
	//     resume epoch would be refused on first use.
	//   - With ChangeID bound, a later resume of the same change finds this epoch
	//     active and refuses resume-active-run.
	// Binding both in one CAS means a failed bind leaves an UNBOUND orphan (inert,
	// like a fresh arm's), never an orphan that names the change. A mint or bind
	// failure unarms fail-closed; the orphan gate record left behind is inert, because
	// no key is returned and nothing dispatches against it.
	//
	// A resume reaches this mint and bind still holding the per-change resume lock it
	// took at step 4 (acquireResumeLock). Epoch locks are per gate key, so without it
	// concurrent epochless resumes of one change would each pass step 4's checks and
	// each mint and bind a live epoch here.
	runRec, eerr := MintRunRecord(repoDir, key, "")
	if eerr != nil {
		return runUntracked(ReasonRunMintFailed)
	}
	if resumeID != 0 {
		if werr := runRecordCAS(repoDir, key, func(rec *RunRecord) error {
			rec.ChangeID = scopeChangeID
			rec.Worktree = worktree
			return nil
		}); werr != nil {
			return runUntracked(ReasonRunMintFailed)
		}
	}

	// (7) Report the armed gate with its dispatch context, its run epoch id, and the
	// honest owner-lifecycle caveat: the dispatched route has no automatic Stop, so a
	// Stop is the explicit `run.cancel` operation keyed by this epoch (change 0375
	// Task 13). startedRunResult refuses an empty epoch, so the line is always three
	// tokens (change 0463).
	return startedRunResult(key, runRec.RunID, grant.ChildCapability)
}
