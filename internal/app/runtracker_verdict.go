package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `docket run verdict <key>` operation in ATTRIBUTED mode
// (change 0334, Task 3): it reads the durable gate record started by run start,
// attributes exactly one new in-progress claim to the dispatched run, delegates
// the run predicate to RunVerify, and maps that verdict onto one line of the
// attributed vocabulary — spending from the counted retry budget atomically (change
// 0421) so a wrong grant (the one unrecoverable move) cannot happen twice.
//
// It NEVER re-derives a run-* verdict: RunVerify (run_verify.go) is the sole
// authority for run-complete / run-unclaimed / run-incomplete / run-halted /
// run-waiting, and this mapper only translates that verdict plus the record's
// retry accounting into a gate decision. It fails CLOSED: any load fault, any
// unrecognized verdict, maps to `run-stop <key> run-tracker-unavailable <reason>` and
// never authorizes a retry.
//
// COMPLETION (spec §successful-completion-flow, change 0441). The verdict remains
// the sole authority MAPPER — it never re-derives RunVerify's run-complete. On a
// keyed run-complete it additionally drives the successful-run ownership closeout
// (runTrackerCompleteRun → completeSuccessfulRun) so a standalone finalize gate can admit
// on the same worktree without a stale-run-id refusal or a human cancellation. A
// BLOCKED or lost closeout maps to `run-stop <key> run-tracker-unavailable <reason>` on the
// existing run-tracker-unavailable channel with the new bounded reason tokens (run-cancelled
// / stale-run-id / completion-unaccounted / completion-unpersisted /
// report-unpersisted / run-record-unreadable) and never reports success — RunVerify's own
// verdict is reported as fact through those tokens, never re-derived. The closeout is
// observation-only, fails closed on missing evidence, and consumes no retry. A
// keyless/standalone/legacy dispatch (no run beside the record) keeps EXACTLY the
// prior behavior. Unattributed observe mode is structurally unable to reach any of
// this.
//
// OWNERSHIP (spec §run verdict, change 0407). A fresh gate resolves the verified
// dispatch-to-claim binding — never a before-set/cardinality snapshot — so a keyed
// verdict can never attribute a concurrent loop's change. resolveRunTrackerOwnership
// (below) "replace[s] before-set/cardinality attribution with the verified
// dispatch-to-claim binding": a confirmed binding's continuity is checked against
// committed claim proofs, an unconfirmed reservation recovers only from its exact
// committed receipt, an absent binding adopts the SOLE proof matching the record's
// context hash, and every missing / conflicting / corrupt / unprovable case fails
// CLOSED to run-stop run-tracker-unavailable (or run-done no-attributable-claim for a
// provably absent claim). The record's BeforeIDs and DispatchedAt are retained as
// diagnostics only and can never create retry authority. A record that already
// names an AttributedID with no claim binding is the `run start --resume` shape:
// its id was pre-bound by verified WorkspaceInspect identity, so ownership returns
// it directly and continuity is RunVerify's job, exactly as today.
//
// RETRY ORDERING (spec: "a lost retry is the safe failure"). On a run-incomplete
// verdict the retry permit is consumed BEFORE the report is chosen, and the CAS
// return — not the record's readable Retry mirror — decides retry-once vs stop.
// Since change 0421 the budget is counted: the current attempt derives from the
// marker authority (attempt = 1 + RunTrackerRetryUsage), and ConsumeRunTrackerRetry grants at
// most AttemptLimit-1 markers (the snapshotted run.max_attempts, default 2 => one
// retry) via a per-attempt O_EXCL create. Of any number of concurrent observers of
// the same completed attempt exactly one creates that attempt's marker, so racing
// verdicts yield exactly one run-retry-once and a counted budget never spends
// several future attempts at once. The report TOKENS are unchanged; the used/limit
// surface is the additive AttemptsUsed/AttemptLimit result fields.
//
// CONTINUATION (change 0359). A tracked gate drive left live (or handed off
// cooperatively) is a CONTINUATION of the same attempt, not a stop: a RunVerify
// run-waiting maps to a nonterminal run-continue directly (runTrackerContinueFromWaiting),
// and a run-incomplete whose recovery scope still binds a tracked drive is taken
// over (runTrackerOuterContinuation) BEFORE the retry CAS is reached — so healthy work
// never spends the retry. A continuation keeps the same key, records the
// single-use continuation triple, and reports `run-continue <key> run-waiting
// <id> <continuation-id> <phase>` with Terminal false. Unsafe ownership
// (ambiguous or halted takeover) earns neither retry nor continuation.

// OperationRunVerdict is the operation key `run verdict` records in its
// envelope.
const OperationRunVerdict = "run.verdict"

// The run-tracker decision tokens — the leading word of every attributed report line.
const (
	RunDecisionDone      = "run-done"
	RunDecisionRetryOnce = "run-retry-once"
	RunDecisionStop      = "run-stop"
)

// The run-tracker outcome tokens that are not themselves RunVerify verdicts. The run-*
// outcomes reuse the VerdictRun* spellings from run_verify.go verbatim (a report
// carries RunVerify's own verdict word, never a re-spelling).
const (
	RunOutcomeNoAttributableClaim = "no-attributable-claim"
	RunOutcomeAmbiguousClaims     = "ambiguous-claims"
	RunOutcomeUnavailable         = "run-tracker-unavailable"
)

// RunReasonUnknownVerdict is the fail-closed reason for a RunVerify outcome this
// mapper does not recognize — a verdict spelling outside the closed set, or an
// operational error carrying no verdict at all.
const RunReasonUnknownVerdict = "unknown-verdict"

// runTrackerReasonStoreError is the fallback reason for a non-typed store error; every
// real store fault is a *RunTrackerStoreError whose Kind is the token.
const runTrackerReasonStoreError = "store-error"

// The fail-closed ownership-resolution reason tokens (change 0407). Each names a
// case where the verified dispatch-to-claim binding cannot be resolved, so the
// verdict refuses to authorize anything: no retry, no sibling id, no fallback to
// global claim inference.
const (
	// ReasonRunBindingUnreadable: the durable claim-binding file exists but could
	// not be read or parsed — never a silent fall-through to attribution.
	ReasonRunBindingUnreadable = "binding-unreadable"
	// ReasonRunBindingConflict: more than one committed claim proof matches the
	// record's context hash with no binding to arbitrate — unsafe ownership.
	ReasonRunBindingConflict = "binding-conflict"
	// ReasonRunClaimReplaced: the bound change was reclaimed and re-claimed by
	// another run — the old gate must detect that replacement and cannot retry or
	// take over the new claim (spec: "the old gate must detect that replacement and
	// cannot retry or take over the new claim").
	ReasonRunClaimReplaced = "claim-replaced"
	// ReasonRunProofUnavailable: committed claim proofs could not be read (no
	// scanner wired, a scan fault, or the binding's own receipt unseeable) — without
	// proof access ownership can never proceed.
	ReasonRunProofUnavailable = "proof-unavailable"
)

// The successful-run closeout reason tokens (change 0441). Each rides the existing
// `run-stop <key> run-tracker-unavailable <reason>` channel when the keyed run-complete
// verdict cannot report success: the report-line vocabulary is unchanged; only these
// bounded reason spellings are new. The first four are RE-USED verbatim from Task 7's
// completeSuccessfulRun return values (which flow straight through as r.Reason), so a
// caller and the engine agree on one spelling; the last two are minted at this
// mapping boundary. None consumes a retry.
const (
	// ReasonRunCancelled: a cancelling/cancelled run — never relabelled successful
	// (an explicit human cancellation won, from active or from completing).
	ReasonRunCancelled = "run-cancelled"
	// ReasonStaleRunID: a superseded run — the run this key named is stale.
	ReasonStaleRunID = "stale-run-id"
	// ReasonRunCompletionUnaccounted: a live/busy/pending/uncertain obligation blocks
	// completion (fail closed). The run stays durably completing; the remedy — named
	// in the result's CompletionFindings — is to settle the evidence and repeat the same
	// keyed verdict, or cancel explicitly.
	ReasonRunCompletionUnaccounted = "completion-unaccounted"
	// ReasonRunCompletionUnpersisted: the completing→completed transition could not be
	// persisted for a reason other than a winning cancellation (fail closed, reportable).
	ReasonRunCompletionUnpersisted = "completion-unpersisted"
	// ReasonRunReportUnpersisted: the closeout finished and the run is durably
	// completed, but the terminal gate REPORT mirror could not be saved. The failure is
	// REPORTED, not hidden by the best-effort save (spec: "completion-path persistence
	// failures must be reported"); the run is already completed, so a repeat of the
	// same keyed verdict replays to run-done run-complete once the fault clears.
	ReasonRunReportUnpersisted = "report-unpersisted"
	// ReasonRunRecordUnreadable: the run record beside the key could not be read
	// (a store fault, corruption, or schema mismatch — anything but a clean absence,
	// which is the keyless/standalone/legacy shape). A record the store cannot read is
	// never a free closeout; fail closed.
	ReasonRunRecordUnreadable = "run-record-unreadable"
)

// RunVerdictResult is the protocol-v1 document `run verdict` returns. It
// renders exactly one attributed report line and always exits 0 (a produced
// report line is not a process failure — learning exit-code-encodes-a-non-failure).
type RunVerdictResult struct {
	Envelope
	Key          string   `json:"key,omitempty"`
	Decision     string   `json:"decision,omitempty"` // run-done | run-retry-once | run-stop | run-continue
	Outcome      string   `json:"outcome,omitempty"`  // run-* | no-attributable-claim | ambiguous-claims | run-tracker-unavailable
	AttributedID int      `json:"attributed_id,omitempty"`
	Unmet        []string `json:"unmet,omitempty"`
	HandoffID    string   `json:"handoff_id,omitempty"`
	Phase        string   `json:"phase,omitempty"`
	// ContinuationID is the single-use redemption token minted on a run-continue
	// decision (change 0359); it is the middle field of the continue line and the
	// token a resumed controller presents to `run continue`.
	ContinuationID string `json:"continuation_id,omitempty"`
	AmbiguousIDs   []int  `json:"ambiguous_ids,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Terminal       bool   `json:"terminal"`
	// AttemptsUsed and AttemptLimit surface the counted outer-retry budget (change
	// 0421) on the run-incomplete path: AttemptsUsed is the 1-based attempt this
	// verdict observed (initial dispatch plus retries granted so far, up to and
	// including this one), and AttemptLimit is the snapshotted run.max_attempts. They
	// are ADDITIVE diagnostics — omitempty keeps them off every other path — and never
	// change the run-retry-once / run-stop report TOKENS, so existing report-line
	// parsing is untouched.
	AttemptsUsed int `json:"attempts_used,omitempty"`
	AttemptLimit int `json:"attempt_limit,omitempty"`
	// CompletionFindings carries the bounded, credential-free diagnostics the
	// successful-run ownership closeout produced (change 0441): on a blocked closeout
	// it names every unsettled obligation the operator must resolve before repeating
	// the keyed verdict; on success it is empty (or carries only informational notes).
	// It is ADDITIVE and diagnostic — omitempty keeps it off every other path — and
	// never changes the report-line TOKENS, so existing report parsing is untouched.
	CompletionFindings []string `json:"completion_findings,omitempty"`
}

// HumanText renders the single attributed report line. The field layout after
// `<decision> <key> <outcome>` is chosen by the outcome token.
func (r RunVerdictResult) HumanText() string {
	fields := []string{r.Decision, r.Key, r.Outcome}
	// A run-continue reuses the run-waiting outcome word but a DISTINCT field
	// layout — [id, continuation-id, phase] — so it is keyed on the DECISION, not
	// the outcome, and never falls into the outcome switch below (which would print
	// the run-stop run-waiting shape). The continuation id, not the opaque drive
	// id, is what the parent redeems.
	if r.Decision == RunDecisionContinue {
		fields = append(fields, strconv.Itoa(r.AttributedID), r.ContinuationID, r.Phase)
		return strings.Join(fields, " ")
	}
	switch r.Outcome {
	case RunOutcomeNoAttributableClaim:
		// no trailing fields
	case RunOutcomeUnavailable:
		fields = append(fields, r.Reason)
	case RunOutcomeAmbiguousClaims:
		for _, id := range r.AmbiguousIDs {
			fields = append(fields, strconv.Itoa(id))
		}
	case VerdictRunComplete, VerdictRunUnclaimed, VerdictRunHalted:
		fields = append(fields, strconv.Itoa(r.AttributedID))
	case VerdictRunWaiting:
		fields = append(fields, strconv.Itoa(r.AttributedID), r.HandoffID, r.Phase)
	case VerdictRunIncomplete:
		fields = append(fields, strconv.Itoa(r.AttributedID))
		fields = append(fields, r.Unmet...)
	}
	return strings.Join(fields, " ")
}

// runVerdictLine builds an applied (exit-0) report result. Every run verdict
// outcome is a report line, so the envelope result is always ResultApplied.
func runVerdictLine(key, decision, outcome string, id int, terminal bool, set func(*RunVerdictResult)) RunVerdictResult {
	r := RunVerdictResult{Key: key, Decision: decision, Outcome: outcome, AttributedID: id, Terminal: terminal}
	if set != nil {
		set(&r)
	}
	r.Envelope = NewEnvelope(OperationRunVerdict, ResultApplied)
	return r
}

// persistRunVerdict records the report line's disposition and terminal flag onto
// the (already-loaded) record and returns the result. The write is best-effort:
// the report line is the contract, and the retry permit's durability rests on the
// O_EXCL marker (authority), not on this mirror save.
func persistRunVerdict(repoDir, key string, rec RunTrackerRecord, res RunVerdictResult) RunVerdictResult {
	rec.Disposition = res.HumanText()
	rec.Terminal = res.Terminal
	_ = SaveRunTrackerRecord(repoDir, key, rec)
	return res
}

// runTrackerStoreReason projects a store error onto its stable run-tracker-unavailable reason
// token. Every real load fault is a *RunTrackerStoreError whose Kind IS the token.
func runTrackerStoreReason(err error) string {
	if gse, ok := AsRunTrackerStoreError(err); ok {
		return string(gse.Kind)
	}
	return runTrackerReasonStoreError
}

// RunVerdict reports the attributed run-tracker verdict for one dispatched
// implement-next run. See the file header for the attribution, retry-ordering,
// and fail-closed contracts.
func RunVerdict(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir, key string) RunVerdictResult {
	// Load the durable record. Any load fault fails closed to run-tracker-unavailable
	// with the store's typed reason token — there is no record to persist to.
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		reason := runTrackerStoreReason(err)
		return runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, 0, true, func(r *RunVerdictResult) {
			r.Reason = reason
		})
	}

	// Resolve ownership from the verified dispatch-to-claim binding (change 0407)
	// before delegating. A non-nil return is the terminal report to emit; nil means
	// the bound change is resolved and rec now carries its AttributedID.
	if stop := resolveRunTrackerOwnership(ctx, deps, wdeps, repoDir, key, &rec); stop != nil {
		return *stop
	}

	id := rec.AttributedID

	// Capture the retry-marker count BEFORE the (comparatively slow) RunVerify
	// delegation, so concurrent verdicts observing the SAME completed attempt capture
	// the SAME used value and therefore target the SAME per-attempt marker — the
	// O_EXCL CAS in ConsumeRunTrackerRetry then grants exactly one of them (change 0421).
	// Reading it after RunVerify would let RunVerify's per-process timing skew
	// separate the reads: a late reader would see an earlier grant's marker, derive a
	// higher attempt number, and double-grant a FUTURE attempt at limit >= 3. It is
	// consulted only on the quiescent run-incomplete path; usedErr is handled there.
	usedBefore, usedErr := RunTrackerRetryUsage(repoDir, key)

	// Delegate the run predicate. RunVerify is the sole authority for the run-*
	// verdicts; this mapper never re-derives one.
	v := RunVerify(ctx, deps, wdeps, gdeps, repoDir, RunVerifyRequest{ID: id})

	switch v.Verdict {
	case VerdictRunComplete:
		// A verified run-complete additionally drives the successful-run ownership
		// closeout (change 0441) so a standalone finalize gate can admit on the same
		// worktree. The seam bundle is injectable (unit tests fake the observers);
		// production composes productionCancelSeams(repoDir). RunVerify's verdict is
		// still reported as fact — runTrackerCompleteRun never re-derives it.
		seams := productionCancelSeams(repoDir)
		if wdeps.CancelSeams != nil {
			seams = wdeps.CancelSeams(repoDir)
		}
		return runTrackerCompleteRun(repoDir, key, rec, id, seams)
	case VerdictRunUnclaimed:
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionDone, VerdictRunUnclaimed, id, true, nil))
	case VerdictRunHalted:
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionStop, VerdictRunHalted, id, true, nil))
	case VerdictRunWaiting:
		// A cooperatively handed-off drive is a live continuation, not a stop: emit
		// a NONTERMINAL run-continue that keeps the key and spends no retry (change
		// 0359). v.HandoffID is the opaque drive locator; its unclaimed handoff token
		// is read through the continuation seam and persisted into the triple.
		return runTrackerContinueFromWaiting(wdeps.Continuation, repoDir, key, rec, id, v.HandoffID, v.Phase)
	case VerdictRunIncomplete:
		// OUTER TAKEOVER BEFORE ANY RETRY CONSUMPTION (order load-bearing, change
		// 0359): a run-incomplete whose recovery scope still binds a tracked drive is
		// HEALTHY work to continue, not a quiescent incomplete to spend the one retry
		// on. Only a genuinely quiescent incomplete — no scope, or zero candidate
		// drives — falls through to the retry CAS below. [ORDERING MUTATION: moving
		// the ConsumeRunTrackerRetry call above this check spends the retry on a continuable
		// run — see TestIntegrationRunVerdictVerdictIncompleteWithTrackedDriveContinuesWithoutRetry.]
		if rec.ScopeID != "" {
			if res, handled := runTrackerOuterContinuation(ctx, deps, wdeps, gdeps, repoDir, key, rec, id); handled {
				return res
			}
		}
		unmet := runTrackerUnmetTokens(v)
		// Derive the current attempt from the marker authority (captured as usedBefore
		// above, ahead of RunVerify) and consume BEFORE choosing the report (a lost
		// retry is the safe failure). attempt = 1 + used (the initial dispatch is
		// attempt 1; each granted retry marker moves the count forward), and the
		// per-attempt O_CREATE|O_EXCL CAS in ConsumeRunTrackerRetry decides retry-once vs
		// stop — of any number of concurrent observers of the SAME completed attempt
		// exactly one creates that attempt's marker, so a counted budget grants exactly
		// once per transition and never spends several future attempts at once.
		// [MUTATION: choosing the report from rec.Retry and consuming afterward
		// double-grants under concurrency — see
		// TestRaceIntegrationAppConcurrencyRunVerdictConcurrentRetryGrantsOnce.]
		// The budget is at most AttemptLimit-1 retries; AttemptLimit is the snapshot
		// (default 2 => one retry). A record that somehow bypassed the save guard with
		// AttemptLimit 0 is floored to the safe minimum 1 (no grant), never unlimited.
		if usedErr != nil {
			reason := runTrackerStoreReason(usedErr)
			return persistRunVerdict(repoDir, key, rec,
				runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
					r.Reason = reason
				}))
		}
		limit := rec.AttemptLimit
		if limit < 1 {
			limit = 1
		}
		attempt := 1 + usedBefore
		granted, cerr := ConsumeRunTrackerRetry(repoDir, key, attempt, limit)
		if cerr != nil {
			reason := runTrackerStoreReason(cerr)
			return persistRunVerdict(repoDir, key, rec,
				runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
					r.Reason = reason
				}))
		}
		if granted {
			// Mirror the consumption ONLY when a retry was actually granted (a marker
			// now exists). On a no-grant stop (e.g. AttemptLimit == 1) nothing was
			// consumed, so leaving the mirror RetryUnused keeps the readable record
			// honest — LoadRunTrackerRecord only ever upgrades the mirror from markers and
			// never downgrades, so a mirror set here on a no-grant stop would read
			// "consumed" forever though RunTrackerRetryUsage stays 0.
			rec.Retry = RetryConsumed
			return persistRunVerdict(repoDir, key, rec,
				runVerdictLine(key, RunDecisionRetryOnce, VerdictRunIncomplete, id, false, func(r *RunVerdictResult) {
					r.Unmet = unmet
					r.AttemptsUsed = attempt
					r.AttemptLimit = limit
				}))
		}
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionStop, VerdictRunIncomplete, id, true, func(r *RunVerdictResult) {
				r.Unmet = unmet
				r.AttemptsUsed = attempt
				r.AttemptLimit = limit
			}))
	default:
		// Any verdict outside the closed set — including a RunVerify operational
		// error carrying an empty verdict — fails closed (spec table: the
		// "anything else / malformed" row → unknown-verdict).
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
				r.Reason = RunReasonUnknownVerdict
			}))
	}
}

// runTrackerCompleteRun maps a verified keyed run-complete onto the successful-run
// ownership closeout (change 0441). The caller (RunVerdict) has already resolved
// the confirmed claim binding and delegated the run predicate to RunVerify; this owns
// only the ownership retirement.
//
//  1. Locate the run beside the run-tracker record. A clean ABSENCE (ErrRunNotFound)
//     is the keyless/standalone/legacy shape: EXACTLY the prior behavior — best-effort
//     report mirror + run-done run-complete. Any OTHER load fault fails closed
//     (run-record-unreadable): a record the store cannot read is never a free closeout.
//  2. Drive completeSuccessfulRun. A blocked/lost closeout maps to run-stop
//     run-tracker-unavailable with the engine's bounded reason token and the diagnostic
//     findings; completion loses without reporting success, and no retry is consumed
//     (runTrackerStopUnavailable leaves the permit untouched).
//  3. On success the run is durably completed. The terminal report mirror is saved
//     as a CHECKED write — a persistence failure is REPORTED (report-unpersisted),
//     never hidden by the best-effort save; the run is already completed, so the
//     remedy is the idempotent replay (a repeat of the same keyed verdict).
func runTrackerCompleteRun(repoDir, key string, rec RunTrackerRecord, id int, seams cancelSeams) RunVerdictResult {
	// (1) Locate the run. Absence is the keyless/standalone/legacy shape; any
	// other fault fails closed.
	if _, _, lerr := LoadRunRecord(repoDir, key); lerr != nil {
		if ee, ok := AsRunError(lerr); ok && ee.Kind == ErrRunNotFound {
			return persistRunVerdict(repoDir, key, rec,
				runVerdictLine(key, RunDecisionDone, VerdictRunComplete, id, true, nil))
		}
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
				r.Reason = ReasonRunRecordUnreadable
			}))
	}

	// (2) Drive the ownership closeout. completeSuccessfulRun's reason is a bounded
	// run-tracker-unavailable token (run-cancelled / stale-run-id / completion-unaccounted /
	// completion-unpersisted / run-record-unreadable), passed through verbatim; the findings
	// name what to settle. A blocked closeout never reports success and spends no retry.
	ok, reason, findings := completeSuccessfulRun(seams, repoDir, key)
	if !ok {
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
				r.Reason = reason
				r.CompletionFindings = findings
			}))
	}

	// (3) Closeout succeeded; the run is durably completed. Save the terminal report
	// mirror as a CHECKED write — a completion-path persistence failure is reported, not
	// hidden. On failure the run stays completed, so the same keyed verdict replays to
	// run-done run-complete once the fault clears.
	res := runVerdictLine(key, RunDecisionDone, VerdictRunComplete, id, true, func(r *RunVerdictResult) {
		r.CompletionFindings = findings
	})
	rec.Disposition = res.HumanText()
	rec.Terminal = res.Terminal
	if serr := SaveRunTrackerRecord(repoDir, key, rec); serr != nil {
		return persistRunVerdict(repoDir, key, rec,
			runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
				r.Reason = ReasonRunReportUnpersisted
				r.CompletionFindings = findings
			}))
	}
	return res
}

// runTrackerContinueFromWaiting emits the nonterminal run-continue for a RunVerify
// run-waiting (a worker cooperatively handed off): it reads the drive's unclaimed
// handoff token through the continuation seam and records the continuation triple.
// A nil seam or an unreadable handoff fails CLOSED to a terminal run-stop
// run-tracker-unavailable rather than emitting a continuation no controller can redeem —
// the pre-0359 terminal shape is never resurrected (migration is atomic).
func runTrackerContinueFromWaiting(seam ContinuationSeam, repoDir, key string, rec RunTrackerRecord, id int, driveID, phase string) RunVerdictResult {
	if seam == nil {
		return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunContinuationUnavailable)
	}
	handoff, err := seam.ExistingHandoffToken(driveID)
	if err != nil {
		return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunContinuationUnavailable)
	}
	return emitContinue(repoDir, key, rec, id, driveID, handoff, phase)
}

// runTrackerOuterContinuation handles a run-incomplete that carries a recovery scope: it
// locates the tracked drive(s) nested under the outer scope and, for exactly one,
// takes it over (event-authorized) and synthesizes a normal handoff, then re-runs
// the UNCHANGED RunVerify predicate to certify the continuation. It returns
// handled=false ONLY for the genuinely quiescent case (no seam wired, or zero
// candidates), so the caller falls through to the ordinary retry path; every other
// outcome — a certified continuation, or an unsafe/ambiguous/erred takeover — is a
// terminal decision it returns with handled=true. Unsafe ownership never earns
// retry OR continuation, and this whole path runs BEFORE the retry CAS.
func runTrackerOuterContinuation(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir, key string, rec RunTrackerRecord, id int) (RunVerdictResult, bool) {
	seam := wdeps.Continuation
	if seam == nil {
		// No continuation seam wired: cannot recover a tracked drive; treat as
		// quiescent and take the ordinary retry path (the pre-0359 behavior).
		return RunVerdictResult{}, false
	}
	ids, err := seam.LocateOuterDrive(id, rec.ChildContextHash)
	if err != nil {
		return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunLocateFailed), true
	}
	switch len(ids) {
	case 0:
		// Genuinely quiescent: fall through to the retry CAS.
		return RunVerdictResult{}, false
	case 1:
		handoff, halted, cause, terr := seam.TakeoverAndHandoff(rec.ScopeID, rec.ParentCap, ids[0])
		if terr != nil {
			return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunTakeoverError), true
		}
		if halted {
			// A halted takeover is fail-closed: run-stop run-tracker-unavailable, no retry
			// spent (a human is needed). One halt cause is intentional and bounded:
			// the outer recovery scope is single-use per run START (it is minted
			// once by run start), so the FIRST accepted outer takeover closes it and
			// a SECOND detached-crash takeover under the same key halts scope-closed
			// here. That once-per-start outer-takeover limit is by design — the human
			// recovers by restarting a fresh scope via `run start --resume`; see
			// claimScopeForTakeover (internal/gatedrive/takeover.go) and the spec's §5
			// continuation clause.
			reason := cause
			if reason == "" {
				reason = ReasonRunTakeoverError
			}
			return runTrackerStopUnavailable(repoDir, key, rec, id, reason), true
		}
		// The synthesized normal handoff must validate through the UNCHANGED
		// run-waiting predicate: re-run RunVerify and require run-waiting.
		v2 := RunVerify(ctx, deps, wdeps, gdeps, repoDir, RunVerifyRequest{ID: id})
		if v2.Verdict != VerdictRunWaiting {
			return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunContinuationUnverified), true
		}
		return emitContinue(repoDir, key, rec, id, v2.HandoffID, handoff, v2.Phase), true
	default:
		// More than one candidate for one outer scope is unsafe ownership.
		return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunTakeoverAmbiguous), true
	}
}

// emitContinue records the continuation triple onto rec and returns the
// nonterminal run-continue report line `run-continue <key> run-waiting <id>
// <continuation-id> <phase>`. It spends no retry — the retry mirror is untouched.
// A continuation-id minting fault fails closed to a terminal run-stop.
func emitContinue(repoDir, key string, rec RunTrackerRecord, id int, driveID, handoff, phase string) RunVerdictResult {
	cid, err := newContinuationID()
	if err != nil {
		return runTrackerStopUnavailable(repoDir, key, rec, id, ReasonRunContinuationUnavailable)
	}
	rec.ContinuationID = cid
	rec.ContinuationDrive = driveID
	rec.ContinuationHandoff = handoff
	return persistRunVerdict(repoDir, key, rec,
		runVerdictLine(key, RunDecisionContinue, VerdictRunWaiting, id, false, func(r *RunVerdictResult) {
			r.HandoffID = driveID
			r.Phase = phase
			r.ContinuationID = cid
		}))
}

// runTrackerStopUnavailable builds a terminal run-stop run-tracker-unavailable report carrying
// reason, and persists it. It never consumes the retry permit — a fail-closed stop
// on the continuation path leaves the permit exactly as it found it.
func runTrackerStopUnavailable(repoDir, key string, rec RunTrackerRecord, id int, reason string) RunVerdictResult {
	return persistRunVerdict(repoDir, key, rec,
		runVerdictLine(key, RunDecisionStop, RunOutcomeUnavailable, id, true, func(r *RunVerdictResult) {
			r.Reason = reason
		}))
}

// resolveRunTrackerOwnership resolves the ONE change this dispatched run owns, from the
// verified dispatch-to-claim binding and the committed claim proofs — never from a
// before-set/cardinality snapshot of the current claim set (change 0407). It
// mutates rec in place to carry the resolved AttributedID (and, for a lagging
// record, the mirror + scope bind) and returns nil when ownership is established;
// otherwise it returns the terminal report to emit. Every missing / conflicting /
// corrupt / unprovable case fails CLOSED — no retry, no sibling id, no fallback to
// global claim inference.
//
// The two recovery legs (the unconfirmed reservation and the sole-proof adoption)
// resolve the recovered change's logical feature worktree through
// runTrackerRecoveredWorktree before confirming, so a run recovered solely through the
// verdict path binds the run worktree exactly as the fresh-claim path does and
// stays fenceable and cancellable; an unresolvable identity refuses through
// ReasonRunProofUnavailable rather than confirming with an empty path (change 0427).
func resolveRunTrackerOwnership(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir, key string, rec *RunTrackerRecord) *RunVerdictResult {
	// Resume-verified shape: an AttributedID with no claim binding was pre-bound by
	// `run start --resume` through WorkspaceInspect identity. Continuity for it is
	// RunVerify's job, exactly as today — the proof continuity check never runs.
	if rec.resumeAttributed() {
		return nil
	}

	// Load the durable claim binding. An unreadable/corrupt binding fails closed —
	// never a silent fall-through to inference.
	binding, hasBinding, berr := LoadRunTrackerClaimBinding(repoDir, key)
	if berr != nil {
		return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunBindingUnreadable)
	}

	// Every remaining branch resolves ownership from committed claim proofs, so scan
	// once up front. Unlike the continuation seam, ownership can never proceed
	// without proof access: a nil scanner or a scan fault fails closed.
	if wdeps.ClaimProofs == nil {
		return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunProofUnavailable)
	}
	proofs, serr := wdeps.ClaimProofs.ScanClaimProofs(ctx, repoDir)
	if serr != nil {
		return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunProofUnavailable)
	}

	switch {
	case hasBinding && binding.Confirmed:
		// Continuity: the NEWEST proof naming the bound change must still be this
		// binding's own claim. A newer proof under a different request id means the
		// change was reclaimed and re-claimed by another run.
		newest, found := runTrackerNewestProofForChange(proofs, binding.ChangeID)
		if !found {
			// The binding's own confirmed receipt could not even be seen (truncated
			// history): fail closed, no retry.
			return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunProofUnavailable)
		}
		if newest.RequestID != binding.RequestID {
			return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunClaimReplaced)
		}
		runTrackerAdoptOwnership(wdeps, repoDir, key, rec, binding.ChangeID, binding.RequestID, binding.Revision)
		return nil

	case hasBinding: // an unconfirmed reservation
		// Recover ONLY from the exact committed receipt for this dispatch (same
		// request id, same context hash).
		proof, found := runTrackerProofForClaim(proofs, binding.RequestID, rec.ChildContextHash)
		if !found {
			// The reserved claim never committed. Leave the reservation refused — never
			// released, never confirmed — and report no attributable claim.
			return runTrackerOwnershipDone(repoDir, key, *rec)
		}
		// The committed receipt is authority, so a confirm error still proceeds on the
		// proof (best-effort mirror) — but the worktree it binds must be real: resolve
		// the recovered change's logical feature worktree first (change 0427), and
		// refuse (fail closed, before confirming) when identity cannot be resolved,
		// never confirming with an empty path that would leave the run
		// unfenceable and uncancellable.
		wt, ok := runTrackerRecoveredWorktree(ctx, deps, repoDir, binding.ChangeID)
		if !ok {
			return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunProofUnavailable)
		}
		_ = ConfirmRunTrackerClaim(repoDir, key, binding.ChangeID, binding.RequestID, proof.Revision, wt)
		runTrackerAdoptOwnership(wdeps, repoDir, key, rec, binding.ChangeID, binding.RequestID, proof.Revision)
		return nil

	default: // no binding file
		// A hashless record cannot prove ownership from a committed receipt.
		if rec.ChildContextHash == "" {
			return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunProofUnavailable)
		}
		matches := runTrackerProofsForContext(proofs, rec.ChildContextHash)
		switch len(matches) {
		case 0:
			return runTrackerOwnershipDone(repoDir, key, *rec)
		case 1:
			p := matches[0]
			// Adopt the sole proof: resolve the change's logical feature worktree
			// FIRST (change 0427) — a refusal here writes nothing, neither
			// reservation nor confirm — then reserve + confirm best-effort with
			// that worktree, then mirror. Confirming with an empty path would
			// leave the recovered run unfenceable and uncancellable.
			wt, ok := runTrackerRecoveredWorktree(ctx, deps, repoDir, p.ChangeID)
			if !ok {
				return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunProofUnavailable)
			}
			_ = ReserveRunTrackerClaim(repoDir, key, p.ChangeID, p.RequestID)
			_ = ConfirmRunTrackerClaim(repoDir, key, p.ChangeID, p.RequestID, p.Revision, wt)
			runTrackerAdoptOwnership(wdeps, repoDir, key, rec, p.ChangeID, p.RequestID, p.Revision)
			return nil
		default:
			return runTrackerOwnershipStop(repoDir, key, *rec, ReasonRunBindingConflict)
		}
	}
}

// runTrackerNewestProofForChange returns the first (newest, since proofs are newest-first)
// committed proof naming changeID.
func runTrackerNewestProofForChange(proofs []ClaimProof, changeID int) (ClaimProof, bool) {
	for _, p := range proofs {
		if p.ChangeID == changeID {
			return p, true
		}
	}
	return ClaimProof{}, false
}

// runTrackerProofForClaim returns the committed proof for exactly this dispatch's claim:
// the same request id under the same context hash.
func runTrackerProofForClaim(proofs []ClaimProof, requestID, contextHash string) (ClaimProof, bool) {
	for _, p := range proofs {
		if p.RequestID == requestID && p.RunContextHash == contextHash {
			return p, true
		}
	}
	return ClaimProof{}, false
}

// runTrackerProofsForContext returns every committed proof carrying contextHash — the
// filter that keeps a keyed verdict off a concurrent loop's claim (a sibling
// dispatch's proof carries a DIFFERENT context hash).
func runTrackerProofsForContext(proofs []ClaimProof, contextHash string) []ClaimProof {
	var out []ClaimProof
	for _, p := range proofs {
		if p.RunContextHash == contextHash {
			out = append(out, p)
		}
	}
	return out
}

// runTrackerRecoveredWorktree resolves the LOGICAL feature worktree for a change the
// verdict path is recovering — filepath.Join(repo.PrimaryWorktree, ".worktrees",
// slug), the same derivation the fresh-claim path binds (change_claim.go,
// "featureWorktree") and the workspace service's intendedPath use. It reads the
// selected change's authoritative metadata (pin + corpus snapshot) and the
// canonical primary repository identity — never the caller's directory, a branch
// spelling, or a scan for candidate worktrees. The directory need not exist yet:
// the mutation fence canonicalizes the stored value at compare time
// (runOwnsWorktree), once workspace.prepare has created it. ok=false means
// repository/change identity could not be resolved; the caller must refuse
// (fail closed) rather than confirm with an empty path.
func runTrackerRecoveredWorktree(ctx context.Context, deps PlanningDeps, repoDir string, changeID int) (string, bool) {
	if deps.Reader == nil || deps.Client == nil {
		return "", false
	}
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return "", false
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return "", false
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return "", false
	}
	c, out := build.Snapshot.Change(domain.ChangeID(changeID))
	if out != domain.LookupFound {
		return "", false
	}
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return "", false
	}
	return filepath.Join(repo.PrimaryWorktree, ".worktrees", c.Slug()), true
}

// runTrackerAdoptOwnership sets the resolved change id on rec and, ONLY when the record
// lags (AttributedID still 0 — the confirmed-binding mirror never persisted, or an
// adopted proof), mirrors the binding fields and binds the outer recovery scope.
// Defense-in-depth (spec §3): a fresh run's outer scope begins unbound and binds
// ONCE to the resolved claim, so a later outer takeover's scopeIdentityMatch pins
// the change id instead of skipping the check on an empty scope field. The mirror
// save and the bind are best-effort — the committed claim receipt is authority, and
// production is already protected by the resolved change id + context-hash filter
// and the verified parent capability. [MUTATION: dropping the BindScopeChange call
// leaves the fresh-run scope unbound — see TestIntegrationRunVerdictVerdictFreshRunBindsScopeChange.]
func runTrackerAdoptOwnership(wdeps WorkspaceDeps, repoDir, key string, rec *RunTrackerRecord, changeID int, requestID, revision string) {
	if rec.AttributedID != 0 {
		return
	}
	rec.AttributedID = changeID
	rec.BoundRequestID = requestID
	rec.BoundRevision = revision
	_ = SaveRunTrackerRecord(repoDir, key, *rec)
	if rec.ScopeID != "" && wdeps.Continuation != nil {
		_ = wdeps.Continuation.BindScopeChange(rec.ScopeID, changeID)
	}
}

// runTrackerOwnershipStop builds the terminal run-stop run-tracker-unavailable report for a
// fail-closed ownership case, persists it, and returns it. It never consumes the
// retry permit.
func runTrackerOwnershipStop(repoDir, key string, rec RunTrackerRecord, reason string) *RunVerdictResult {
	res := runTrackerStopUnavailable(repoDir, key, rec, rec.AttributedID, reason)
	return &res
}

// runTrackerOwnershipDone builds the terminal run-done no-attributable-claim report for a
// dispatch that provably claimed nothing, persists it, and returns it.
func runTrackerOwnershipDone(repoDir, key string, rec RunTrackerRecord) *RunVerdictResult {
	res := persistRunVerdict(repoDir, key, rec,
		runVerdictLine(key, RunDecisionDone, RunOutcomeNoAttributableClaim, 0, true, nil))
	return &res
}

// runTrackerUnmetTokens projects RunVerify's unmet conditions onto their stable reason
// tokens, preserving RunVerify's order (the report echoes the predicate's own
// enumeration, never a re-sort).
func runTrackerUnmetTokens(v RunVerifyResult) []string {
	out := make([]string, 0, len(v.Unmet))
	for _, u := range v.Unmet {
		out = append(out, u.Reason)
	}
	return out
}

// ---------------------------------------------------------------------------
// UNATTRIBUTED (observe-only) mode — `docket run verdict --unattributed
// [<id>...]` (change 0334, Task 4).
//
// This mode holds NO key, reads and writes NO gate record, and consumes NO retry
// permit: it never mints, saves, or calls ConsumeRunTrackerRetry, so the run-tracker root
// is never even created. It re-syncs to fresh origin, then verifies either the
// supplied hint ids (each a hint to verify, NEVER attribution evidence) or, when
// none are supplied, every current in-progress id, and renders one line per id
// using RunVerify's verdict verbatim. An empty backlog with no hints reports
// `run-observe no-current-run`; a re-sync/read fault fails closed to a single
// `run-observe run-tracker-unavailable <reason>` line.
//
// STRUCTURAL SEPARATION (spec, CRITICAL): observe rendering is a SEPARATE render
// path (runTrackerObserveLine) that only knows the `run-observe` prefix and the
// observe outcome set. It has no access to RunDecisionRetryOnce and no branch
// that could emit it — there is, by construction, NO code path from
// --unattributed to run-retry-once. The attributed retry accounting above is
// unreachable from here.

// RunDecisionObserve is the leading word of every unattributed report line. It
// is the ONLY decision token the observe renderer knows; the retry/done/stop
// tokens are structurally out of reach on this path.
const RunDecisionObserve = "run-observe"

// RunOutcomeNoCurrentRun is the observe outcome when there is no run to observe:
// no in-progress ids and no hints supplied.
const RunOutcomeNoCurrentRun = "no-current-run"

// ReasonRunUnattributedKey is the usage-error reason when --unattributed is
// given a non-integer positional: hints are change ids, and a run key can never
// be one. This is a usage error (non-zero exit), never a report line.
const ReasonRunUnattributedKey = "unattributed-key"

// RunObservation is one observed id's outcome: a RunVerify verdict (verbatim),
// or a whole-report outcome (no-current-run / run-tracker-unavailable) that carries no
// id. It is rendered by runTrackerObserveLine, which is structurally unable to emit a
// retry.
type RunObservation struct {
	Outcome   string   `json:"outcome"`
	ID        int      `json:"id,omitempty"`
	Unmet     []string `json:"unmet,omitempty"`
	HandoffID string   `json:"handoff_id,omitempty"`
	Phase     string   `json:"phase,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// RunVerdictObserveResult is the protocol-v1 document the unattributed mode
// returns. On the report path Result is applied (exit 0) and Observations holds
// one entry per rendered line; a usage error carries a non-applied Result with a
// Reason and no observations.
type RunVerdictObserveResult struct {
	Envelope
	Observations []RunObservation `json:"observations,omitempty"`
	Reason       string           `json:"reason,omitempty"`
	Message      string           `json:"message,omitempty"`
}

// HumanText renders the observe report: one `run-observe …` line per
// observation. A usage error (non-applied result) names its reason instead.
func (r RunVerdictObserveResult) HumanText() string {
	if r.Result != ResultApplied {
		if r.Reason != "" {
			return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
		}
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
	lines := make([]string, 0, len(r.Observations))
	for _, o := range r.Observations {
		lines = append(lines, runTrackerObserveLine(o))
	}
	return strings.Join(lines, "\n")
}

// runTrackerObserveLine renders ONE observe report line. Its leading token is always
// the RunDecisionObserve literal, and the outcome word is one of the observe
// outcome set (a RunVerify verdict, no-current-run, or run-tracker-unavailable). It has
// no knowledge of and no branch to RunDecisionRetryOnce OR RunDecisionContinue
// — the structural guarantee that --unattributed can authorize neither a retry
// (change 0334) nor a nonterminal continuation (change 0359). A run-waiting id
// observed here renders the plain observe run-waiting line, never a run-continue.
func runTrackerObserveLine(o RunObservation) string {
	fields := []string{RunDecisionObserve, o.Outcome}
	switch o.Outcome {
	case RunOutcomeNoCurrentRun:
		// no trailing fields
	case RunOutcomeUnavailable:
		fields = append(fields, o.Reason)
	case VerdictRunComplete, VerdictRunUnclaimed, VerdictRunHalted:
		fields = append(fields, strconv.Itoa(o.ID))
	case VerdictRunWaiting:
		fields = append(fields, strconv.Itoa(o.ID), o.HandoffID, o.Phase)
	case VerdictRunIncomplete:
		fields = append(fields, strconv.Itoa(o.ID))
		fields = append(fields, o.Unmet...)
	}
	return strings.Join(fields, " ")
}

// newRunTrackerObserveReport builds an applied (exit-0) observe report over the given
// observation lines.
func newRunTrackerObserveReport(obs ...RunObservation) RunVerdictObserveResult {
	r := RunVerdictObserveResult{Observations: obs}
	r.Envelope = NewEnvelope(OperationRunVerdict, ResultApplied)
	return r
}

// RunVerdictObserve reports the unattributed (observe-only) run-tracker verdicts.
// hints are the raw positional arguments; each must parse as an integer change id
// (a non-integer — for example a run key — is a usage error). See the section
// header for the no-writes / no-retry structural contract.
func RunVerdictObserve(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir string, hints []string) RunVerdictObserveResult {
	// Parse the hints. A non-integer positional is a run key (or garbage), not a
	// change-id hint: usage error, non-zero exit, never a report line.
	hintIDs := make([]int, 0, len(hints))
	for _, h := range hints {
		id, err := strconv.Atoi(strings.TrimSpace(h))
		if err != nil {
			out := RunVerdictObserveResult{
				Reason:  ReasonRunUnattributedKey,
				Message: fmt.Sprintf("--unattributed takes change-id hints, not %q; a run key is not a hint", h),
			}
			out.Envelope = NewEnvelope(OperationRunVerdict, ResultInvalidInput)
			return out
		}
		hintIDs = append(hintIDs, id)
	}

	// Re-sync + read the current in-progress set. A sync/read fault fails closed
	// to a single run-tracker-unavailable line. This is the ONLY read; it writes nothing.
	inProgress, reason := observeInProgressIDs(ctx, deps, repoDir)
	if reason != "" {
		return newRunTrackerObserveReport(RunObservation{Outcome: RunOutcomeUnavailable, Reason: reason})
	}

	// Hints win when supplied (verified in input order); otherwise verify every
	// current in-progress id (sorted). No ids and no hints → no-current-run.
	ids := hintIDs
	if len(ids) == 0 {
		ids = inProgress
	}
	if len(ids) == 0 {
		return newRunTrackerObserveReport(RunObservation{Outcome: RunOutcomeNoCurrentRun})
	}

	obs := make([]RunObservation, 0, len(ids))
	for _, id := range ids {
		v := RunVerify(ctx, deps, wdeps, gdeps, repoDir, RunVerifyRequest{ID: id})
		obs = append(obs, observeFromVerdict(id, v))
	}
	return newRunTrackerObserveReport(obs...)
}

// observeInProgressIDs re-syncs to fresh origin and returns the current
// in-progress change ids (sorted), or a non-empty run-tracker-unavailable reason token
// on a re-sync / corpus-read fault (fail closed). It writes nothing — the same
// read-only plumbing run start and attribution use.
func observeInProgressIDs(ctx context.Context, deps PlanningDeps, repoDir string) (ids []int, reason string) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return nil, ReasonRunSyncFailed
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return nil, ReasonRunChangesUnreadable
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return nil, ReasonRunChangesUnreadable
	}
	for _, c := range build.Snapshot.Changes() {
		if c.Status() == domain.StatusInProgress {
			ids = append(ids, int(c.ID()))
		}
	}
	sort.Ints(ids)
	return ids, ""
}

// observeFromVerdict maps ONE RunVerify result onto an observation, using its
// verdict verbatim. An operational error (no verdict) or an unrecognized verdict
// becomes a run-tracker-unavailable observation carrying RunVerify's own reason (or
// unknown-verdict) — never a retry: this path has no retry to grant.
func observeFromVerdict(id int, v RunVerifyResult) RunObservation {
	switch v.Verdict {
	case VerdictRunComplete, VerdictRunUnclaimed, VerdictRunHalted:
		return RunObservation{Outcome: v.Verdict, ID: id}
	case VerdictRunWaiting:
		// Handoff id and phase pass through verbatim — never reformatted.
		return RunObservation{Outcome: v.Verdict, ID: id, HandoffID: v.HandoffID, Phase: v.Phase}
	case VerdictRunIncomplete:
		return RunObservation{Outcome: v.Verdict, ID: id, Unmet: runTrackerUnmetTokens(v)}
	default:
		reason := v.Reason
		if reason == "" {
			reason = RunReasonUnknownVerdict
		}
		return RunObservation{Outcome: RunOutcomeUnavailable, Reason: reason}
	}
}
