package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `docket run gate-verdict <key>` operation in ATTRIBUTED mode
// (change 0334, Task 3): it reads the durable gate record armed by gate-before,
// attributes exactly one new in-progress claim to the dispatched run, delegates
// the run predicate to RunVerify, and maps that verdict onto one line of the
// attributed vocabulary — spending the single retry permit atomically so a wrong
// grant (the one unrecoverable move) cannot happen twice.
//
// It NEVER re-derives a run-* verdict: RunVerify (run_verify.go) is the sole
// authority for run-complete / run-unclaimed / run-incomplete / run-halted /
// run-waiting, and this mapper only translates that verdict plus the record's
// retry accounting into a gate decision. It fails CLOSED: any load fault, any
// unrecognized verdict, maps to `gate-stop <key> gate-unavailable <reason>` and
// never authorizes a retry.
//
// ATTRIBUTION (spec §gate-verdict). If the record already names an AttributedID,
// that id is verified directly — attribution never re-runs against a later claim
// set, so a claim that appears after the first verdict can never be mistaken for
// this run's. Otherwise the current in-progress claim set is read fresh (the same
// PinContext/ReadCorpus/BuildSnapshot plumbing gate-before uses) and each
// candidate passes THREE filters: (a) its id is absent from the record's
// before-set; (b) its claimed_at parses (a domain FieldPresent stamp — an absent
// or unparseable stamp is excluded); (c) its claimed_at is at or after the record's
// DispatchEpoch. Zero survivors → no-attributable-claim; more than one →
// ambiguous-claims; exactly one → attribute, persist, delegate.
//
// RETRY ORDERING (spec: "a lost retry is the safe failure"). On a run-incomplete
// verdict the retry permit is consumed BEFORE the report is chosen, and the CAS
// return — not the record's readable Retry mirror — decides retry-once vs stop.
// The O_EXCL create in ConsumeGateRetry grants at most one caller across any
// number of concurrent verdicts, so two racing calls yield exactly one
// gate-retry-once.
//
// CONTINUATION (change 0359). A tracked gate drive left live (or terminal but
// unconsumed) is a CONTINUATION of the same attempt, not a stop: a RunVerify
// run-waiting maps to a nonterminal gate-continue directly (gateContinueFromWaiting),
// and a run-incomplete whose recovery scope still binds a tracked drive is taken
// over (gateOuterContinuation) BEFORE the retry CAS is reached — so healthy work
// never spends the retry. A continuation keeps the same key, records the
// single-use continuation triple, and reports `gate-continue <key> run-waiting
// <id> <continuation-id> <phase>` with Terminal false. Unsafe ownership
// (ambiguous or halted takeover) earns neither retry nor continuation.

// OperationRunGateVerdict is the operation key `run gate-verdict` records in its
// envelope.
const OperationRunGateVerdict = "run.gate-verdict"

// The gate decision tokens — the leading word of every attributed report line.
const (
	GateDecisionDone      = "gate-done"
	GateDecisionRetryOnce = "gate-retry-once"
	GateDecisionStop      = "gate-stop"
)

// The gate outcome tokens that are not themselves RunVerify verdicts. The run-*
// outcomes reuse the VerdictRun* spellings from run_verify.go verbatim (a report
// carries RunVerify's own verdict word, never a re-spelling).
const (
	GateOutcomeNoAttributableClaim = "no-attributable-claim"
	GateOutcomeAmbiguousClaims     = "ambiguous-claims"
	GateOutcomeUnavailable         = "gate-unavailable"
)

// GateReasonUnknownVerdict is the fail-closed reason for a RunVerify outcome this
// mapper does not recognize — a verdict spelling outside the closed set, or an
// operational error carrying no verdict at all.
const GateReasonUnknownVerdict = "unknown-verdict"

// gateReasonStoreError is the fallback reason for a non-typed store error; every
// real store fault is a *GateStoreError whose Kind is the token.
const gateReasonStoreError = "store-error"

// RunGateVerdictResult is the protocol-v1 document `run gate-verdict` returns. It
// renders exactly one attributed report line and always exits 0 (a produced
// report line is not a process failure — learning exit-code-encodes-a-non-failure).
type RunGateVerdictResult struct {
	Envelope
	Key          string   `json:"key,omitempty"`
	Decision     string   `json:"decision,omitempty"` // gate-done | gate-retry-once | gate-stop | gate-continue
	Outcome      string   `json:"outcome,omitempty"`  // run-* | no-attributable-claim | ambiguous-claims | gate-unavailable
	AttributedID int      `json:"attributed_id,omitempty"`
	Unmet        []string `json:"unmet,omitempty"`
	HandoffID    string   `json:"handoff_id,omitempty"`
	Phase        string   `json:"phase,omitempty"`
	// ContinuationID is the single-use redemption token minted on a gate-continue
	// decision (change 0359); it is the middle field of the continue line and the
	// token a resumed controller presents to `run gate-claim`.
	ContinuationID string `json:"continuation_id,omitempty"`
	AmbiguousIDs   []int  `json:"ambiguous_ids,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Terminal       bool   `json:"terminal"`
}

// HumanText renders the single attributed report line. The field layout after
// `<decision> <key> <outcome>` is chosen by the outcome token.
func (r RunGateVerdictResult) HumanText() string {
	fields := []string{r.Decision, r.Key, r.Outcome}
	// A gate-continue reuses the run-waiting outcome word but a DISTINCT field
	// layout — [id, continuation-id, phase] — so it is keyed on the DECISION, not
	// the outcome, and never falls into the outcome switch below (which would print
	// the gate-stop run-waiting shape). The continuation id, not the opaque drive
	// id, is what the parent redeems.
	if r.Decision == GateDecisionContinue {
		fields = append(fields, strconv.Itoa(r.AttributedID), r.ContinuationID, r.Phase)
		return strings.Join(fields, " ")
	}
	switch r.Outcome {
	case GateOutcomeNoAttributableClaim:
		// no trailing fields
	case GateOutcomeUnavailable:
		fields = append(fields, r.Reason)
	case GateOutcomeAmbiguousClaims:
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

// gateVerdictLine builds an applied (exit-0) report result. Every gate-verdict
// outcome is a report line, so the envelope result is always ResultApplied.
func gateVerdictLine(key, decision, outcome string, id int, terminal bool, set func(*RunGateVerdictResult)) RunGateVerdictResult {
	r := RunGateVerdictResult{Key: key, Decision: decision, Outcome: outcome, AttributedID: id, Terminal: terminal}
	if set != nil {
		set(&r)
	}
	r.Envelope = NewEnvelope(OperationRunGateVerdict, ResultApplied)
	return r
}

// persistGateVerdict records the report line's disposition and terminal flag onto
// the (already-loaded) record and returns the result. The write is best-effort:
// the report line is the contract, and the retry permit's durability rests on the
// O_EXCL marker (authority), not on this mirror save.
func persistGateVerdict(repoDir, key string, rec GateRecord, res RunGateVerdictResult) RunGateVerdictResult {
	rec.Disposition = res.HumanText()
	rec.Terminal = res.Terminal
	_ = SaveGateRecord(repoDir, key, rec)
	return res
}

// gateStoreReason projects a store error onto its stable gate-unavailable reason
// token. Every real load fault is a *GateStoreError whose Kind IS the token.
func gateStoreReason(err error) string {
	if gse, ok := AsGateStoreError(err); ok {
		return string(gse.Kind)
	}
	return gateReasonStoreError
}

// RunGateVerdict reports the attributed run-gate verdict for one dispatched
// implement-next run. See the file header for the attribution, retry-ordering,
// and fail-closed contracts.
func RunGateVerdict(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir, key string) RunGateVerdictResult {
	// Load the durable record. Any load fault fails closed to gate-unavailable
	// with the store's typed reason token — there is no record to persist to.
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		reason := gateStoreReason(err)
		return gateVerdictLine(key, GateDecisionStop, GateOutcomeUnavailable, 0, true, func(r *RunGateVerdictResult) {
			r.Reason = reason
		})
	}

	// Attribute a claim if one is not already bound to this record.
	if rec.AttributedID == 0 {
		survivors, reason := attributeGateClaim(ctx, deps, repoDir, rec)
		if reason != "" {
			return persistGateVerdict(repoDir, key, rec,
				gateVerdictLine(key, GateDecisionStop, GateOutcomeUnavailable, 0, true, func(r *RunGateVerdictResult) {
					r.Reason = reason
				}))
		}
		switch len(survivors) {
		case 0:
			return persistGateVerdict(repoDir, key, rec,
				gateVerdictLine(key, GateDecisionDone, GateOutcomeNoAttributableClaim, 0, true, nil))
		case 1:
			// Store the attribution BEFORE delegating (spec) so the binding is
			// durable even if the delegation or the final save is interrupted.
			rec.AttributedID = survivors[0]
			_ = SaveGateRecord(repoDir, key, rec)
		default:
			return persistGateVerdict(repoDir, key, rec,
				gateVerdictLine(key, GateDecisionStop, GateOutcomeAmbiguousClaims, 0, true, func(r *RunGateVerdictResult) {
					r.AmbiguousIDs = survivors
				}))
		}
	}

	id := rec.AttributedID

	// Delegate the run predicate. RunVerify is the sole authority for the run-*
	// verdicts; this mapper never re-derives one.
	v := RunVerify(ctx, deps, wdeps, gdeps, repoDir, RunVerifyRequest{ID: id})

	switch v.Verdict {
	case VerdictRunComplete:
		return persistGateVerdict(repoDir, key, rec,
			gateVerdictLine(key, GateDecisionDone, VerdictRunComplete, id, true, nil))
	case VerdictRunUnclaimed:
		return persistGateVerdict(repoDir, key, rec,
			gateVerdictLine(key, GateDecisionDone, VerdictRunUnclaimed, id, true, nil))
	case VerdictRunHalted:
		return persistGateVerdict(repoDir, key, rec,
			gateVerdictLine(key, GateDecisionStop, VerdictRunHalted, id, true, nil))
	case VerdictRunWaiting:
		// A cooperatively handed-off drive is a live continuation, not a stop: emit
		// a NONTERMINAL gate-continue that keeps the key and spends no retry (change
		// 0359). v.HandoffID is the opaque drive locator; its unclaimed handoff token
		// is read through the continuation seam and persisted into the triple.
		return gateContinueFromWaiting(wdeps.Continuation, repoDir, key, rec, id, v.HandoffID, v.Phase)
	case VerdictRunIncomplete:
		// OUTER TAKEOVER BEFORE ANY RETRY CONSUMPTION (order load-bearing, change
		// 0359): a run-incomplete whose recovery scope still binds a tracked drive is
		// HEALTHY work to continue, not a quiescent incomplete to spend the one retry
		// on. Only a genuinely quiescent incomplete — no scope, or zero candidate
		// drives — falls through to the retry CAS below. [ORDERING MUTATION: moving
		// the ConsumeGateRetry call above this check spends the retry on a continuable
		// run — see TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry.]
		if rec.ScopeID != "" {
			if res, handled := gateOuterContinuation(ctx, deps, wdeps, gdeps, repoDir, key, rec, id); handled {
				return res
			}
		}
		unmet := gateUnmetTokens(v)
		// Consume the retry permit BEFORE choosing the report (a lost retry is the
		// safe failure). The CAS return decides retry-once vs stop, so two
		// concurrent callers grant at most one retry. [MUTATION: deciding from
		// rec.Retry and consuming afterward double-grants under concurrency — see
		// TestRunGateVerdictConcurrentRetryGrantsOnce.]
		granted, cerr := ConsumeGateRetry(repoDir, key)
		if cerr != nil {
			reason := gateStoreReason(cerr)
			return persistGateVerdict(repoDir, key, rec,
				gateVerdictLine(key, GateDecisionStop, GateOutcomeUnavailable, id, true, func(r *RunGateVerdictResult) {
					r.Reason = reason
				}))
		}
		rec.Retry = RetryConsumed
		if granted {
			return persistGateVerdict(repoDir, key, rec,
				gateVerdictLine(key, GateDecisionRetryOnce, VerdictRunIncomplete, id, false, func(r *RunGateVerdictResult) {
					r.Unmet = unmet
				}))
		}
		return persistGateVerdict(repoDir, key, rec,
			gateVerdictLine(key, GateDecisionStop, VerdictRunIncomplete, id, true, func(r *RunGateVerdictResult) {
				r.Unmet = unmet
			}))
	default:
		// Any verdict outside the closed set — including a RunVerify operational
		// error carrying an empty verdict — fails closed (spec table: the
		// "anything else / malformed" row → unknown-verdict).
		return persistGateVerdict(repoDir, key, rec,
			gateVerdictLine(key, GateDecisionStop, GateOutcomeUnavailable, id, true, func(r *RunGateVerdictResult) {
				r.Reason = GateReasonUnknownVerdict
			}))
	}
}

// gateContinueFromWaiting emits the nonterminal gate-continue for a RunVerify
// run-waiting (a worker cooperatively handed off): it reads the drive's unclaimed
// handoff token through the continuation seam and records the continuation triple.
// A nil seam or an unreadable handoff fails CLOSED to a terminal gate-stop
// gate-unavailable rather than emitting a continuation no controller can redeem —
// the pre-0359 terminal shape is never resurrected (migration is atomic).
func gateContinueFromWaiting(seam ContinuationSeam, repoDir, key string, rec GateRecord, id int, driveID, phase string) RunGateVerdictResult {
	if seam == nil {
		return gateStopUnavailable(repoDir, key, rec, id, ReasonGateContinuationUnavailable)
	}
	handoff, err := seam.ExistingHandoffToken(driveID)
	if err != nil {
		return gateStopUnavailable(repoDir, key, rec, id, ReasonGateContinuationUnavailable)
	}
	return emitContinue(repoDir, key, rec, id, driveID, handoff, phase)
}

// gateOuterContinuation handles a run-incomplete that carries a recovery scope: it
// locates the tracked drive(s) nested under the outer scope and, for exactly one,
// takes it over (event-authorized) and synthesizes a normal handoff, then re-runs
// the UNCHANGED RunVerify predicate to certify the continuation. It returns
// handled=false ONLY for the genuinely quiescent case (no seam wired, or zero
// candidates), so the caller falls through to the ordinary retry path; every other
// outcome — a certified continuation, or an unsafe/ambiguous/erred takeover — is a
// terminal decision it returns with handled=true. Unsafe ownership never earns
// retry OR continuation, and this whole path runs BEFORE the retry CAS.
func gateOuterContinuation(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir, key string, rec GateRecord, id int) (RunGateVerdictResult, bool) {
	seam := wdeps.Continuation
	if seam == nil {
		// No continuation seam wired: cannot recover a tracked drive; treat as
		// quiescent and take the ordinary retry path (the pre-0359 behavior).
		return RunGateVerdictResult{}, false
	}
	ids, err := seam.LocateOuterDrive(id, rec.ChildContextHash)
	if err != nil {
		return gateStopUnavailable(repoDir, key, rec, id, ReasonGateLocateFailed), true
	}
	switch len(ids) {
	case 0:
		// Genuinely quiescent: fall through to the retry CAS.
		return RunGateVerdictResult{}, false
	case 1:
		handoff, halted, cause, terr := seam.TakeoverAndHandoff(rec.ScopeID, rec.ParentCap, ids[0])
		if terr != nil {
			return gateStopUnavailable(repoDir, key, rec, id, ReasonGateTakeoverError), true
		}
		if halted {
			// A halted takeover is fail-closed: gate-stop gate-unavailable, no retry
			// spent (a human is needed). One halt cause is intentional and bounded:
			// the outer recovery scope is single-use per gate ARMING (it is minted
			// once by gate-before), so the FIRST accepted outer takeover closes it and
			// a SECOND detached-crash takeover under the same key halts scope-closed
			// here. That once-per-arming outer-takeover limit is by design — the human
			// recovers by re-arming a fresh scope via `gate-before --resume`; see
			// claimScopeForTakeover (internal/gatedrive/takeover.go) and the spec's §5
			// continuation clause.
			reason := cause
			if reason == "" {
				reason = ReasonGateTakeoverError
			}
			return gateStopUnavailable(repoDir, key, rec, id, reason), true
		}
		// The synthesized normal handoff must validate through the UNCHANGED
		// run-waiting predicate: re-run RunVerify and require run-waiting.
		v2 := RunVerify(ctx, deps, wdeps, gdeps, repoDir, RunVerifyRequest{ID: id})
		if v2.Verdict != VerdictRunWaiting {
			return gateStopUnavailable(repoDir, key, rec, id, ReasonGateContinuationUnverified), true
		}
		return emitContinue(repoDir, key, rec, id, v2.HandoffID, handoff, v2.Phase), true
	default:
		// More than one candidate for one outer scope is unsafe ownership.
		return gateStopUnavailable(repoDir, key, rec, id, ReasonGateTakeoverAmbiguous), true
	}
}

// emitContinue records the continuation triple onto rec and returns the
// nonterminal gate-continue report line `gate-continue <key> run-waiting <id>
// <continuation-id> <phase>`. It spends no retry — the retry mirror is untouched.
// A continuation-id minting fault fails closed to a terminal gate-stop.
func emitContinue(repoDir, key string, rec GateRecord, id int, driveID, handoff, phase string) RunGateVerdictResult {
	cid, err := newContinuationID()
	if err != nil {
		return gateStopUnavailable(repoDir, key, rec, id, ReasonGateContinuationUnavailable)
	}
	rec.ContinuationID = cid
	rec.ContinuationDrive = driveID
	rec.ContinuationHandoff = handoff
	return persistGateVerdict(repoDir, key, rec,
		gateVerdictLine(key, GateDecisionContinue, VerdictRunWaiting, id, false, func(r *RunGateVerdictResult) {
			r.HandoffID = driveID
			r.Phase = phase
			r.ContinuationID = cid
		}))
}

// gateStopUnavailable builds a terminal gate-stop gate-unavailable report carrying
// reason, and persists it. It never consumes the retry permit — a fail-closed stop
// on the continuation path leaves the permit exactly as it found it.
func gateStopUnavailable(repoDir, key string, rec GateRecord, id int, reason string) RunGateVerdictResult {
	return persistGateVerdict(repoDir, key, rec,
		gateVerdictLine(key, GateDecisionStop, GateOutcomeUnavailable, id, true, func(r *RunGateVerdictResult) {
			r.Reason = reason
		}))
}

// attributeGateClaim reads the current in-progress claim set fresh and applies the
// three attribution filters against rec's before-set and dispatch epoch. It
// returns the surviving candidate ids (sorted) or, on a re-sync / corpus-read
// fault, a non-empty gate-unavailable reason token (fail closed). It writes
// nothing.
func attributeGateClaim(ctx context.Context, deps PlanningDeps, repoDir string, rec GateRecord) (survivors []int, reason string) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return nil, ReasonGateSyncFailed
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return nil, ReasonGateChangesUnreadable
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return nil, ReasonGateChangesUnreadable
	}

	before := make(map[int]bool, len(rec.BeforeIDs))
	for _, id := range rec.BeforeIDs {
		before[id] = true
	}
	for _, c := range build.Snapshot.Changes() {
		if c.Status() != domain.StatusInProgress {
			continue
		}
		id := int(c.ID())
		// (a) A claim already present in the before-set is not this run's.
		if before[id] {
			continue
		}
		// (b) claimed_at must parse: a domain FieldPresent stamp. An absent or
		// unparseable (FieldAbsent / FieldEmpty / FieldMalformed) stamp is excluded.
		ca := c.ClaimedAt()
		if ca.State != domain.FieldPresent {
			continue
		}
		// (c) The claim must be at or after the dispatch epoch.
		if ca.Value.Unix() < rec.DispatchEpoch {
			continue
		}
		survivors = append(survivors, id)
	}
	sort.Ints(survivors)
	return survivors, ""
}

// gateUnmetTokens projects RunVerify's unmet conjuncts onto their stable reason
// tokens, preserving RunVerify's order (the report echoes the predicate's own
// enumeration, never a re-sort).
func gateUnmetTokens(v RunVerifyResult) []string {
	out := make([]string, 0, len(v.Unmet))
	for _, u := range v.Unmet {
		out = append(out, u.Reason)
	}
	return out
}

// ---------------------------------------------------------------------------
// UNATTRIBUTED (observe-only) mode — `docket run gate-verdict --unattributed
// [<id>...]` (change 0334, Task 4).
//
// This mode holds NO key, reads and writes NO gate record, and consumes NO retry
// permit: it never mints, saves, or calls ConsumeGateRetry, so the rungate root
// is never even created. It re-syncs to fresh origin, then verifies either the
// supplied hint ids (each a hint to verify, NEVER attribution evidence) or, when
// none are supplied, every current in-progress id, and renders one line per id
// using RunVerify's verdict verbatim. An empty backlog with no hints reports
// `gate-observe no-current-run`; a re-sync/read fault fails closed to a single
// `gate-observe gate-unavailable <reason>` line.
//
// STRUCTURAL SEPARATION (spec, CRITICAL): observe rendering is a SEPARATE render
// path (gateObserveLine) that only knows the `gate-observe` prefix and the
// observe outcome set. It has no access to GateDecisionRetryOnce and no branch
// that could emit it — there is, by construction, NO code path from
// --unattributed to gate-retry-once. The attributed retry accounting above is
// unreachable from here.

// GateDecisionObserve is the leading word of every unattributed report line. It
// is the ONLY decision token the observe renderer knows; the retry/done/stop
// tokens are structurally out of reach on this path.
const GateDecisionObserve = "gate-observe"

// GateOutcomeNoCurrentRun is the observe outcome when there is no run to observe:
// no in-progress ids and no hints supplied.
const GateOutcomeNoCurrentRun = "no-current-run"

// ReasonGateUnattributedKey is the usage-error reason when --unattributed is
// given a non-integer positional: hints are change ids, and a gate key can never
// be one. This is a usage error (non-zero exit), never a report line.
const ReasonGateUnattributedKey = "unattributed-key"

// GateObservation is one observed id's outcome: a RunVerify verdict (verbatim),
// or a whole-report outcome (no-current-run / gate-unavailable) that carries no
// id. It is rendered by gateObserveLine, which is structurally unable to emit a
// retry.
type GateObservation struct {
	Outcome   string   `json:"outcome"`
	ID        int      `json:"id,omitempty"`
	Unmet     []string `json:"unmet,omitempty"`
	HandoffID string   `json:"handoff_id,omitempty"`
	Phase     string   `json:"phase,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// RunGateVerdictObserveResult is the protocol-v1 document the unattributed mode
// returns. On the report path Result is applied (exit 0) and Observations holds
// one entry per rendered line; a usage error carries a non-applied Result with a
// Reason and no observations.
type RunGateVerdictObserveResult struct {
	Envelope
	Observations []GateObservation `json:"observations,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	Message      string            `json:"message,omitempty"`
}

// HumanText renders the observe report: one `gate-observe …` line per
// observation. A usage error (non-applied result) names its reason instead.
func (r RunGateVerdictObserveResult) HumanText() string {
	if r.Result != ResultApplied {
		if r.Reason != "" {
			return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
		}
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
	lines := make([]string, 0, len(r.Observations))
	for _, o := range r.Observations {
		lines = append(lines, gateObserveLine(o))
	}
	return strings.Join(lines, "\n")
}

// gateObserveLine renders ONE observe report line. Its leading token is always
// the GateDecisionObserve literal, and the outcome word is one of the observe
// outcome set (a RunVerify verdict, no-current-run, or gate-unavailable). It has
// no knowledge of and no branch to GateDecisionRetryOnce OR GateDecisionContinue
// — the structural guarantee that --unattributed can authorize neither a retry
// (change 0334) nor a nonterminal continuation (change 0359). A run-waiting id
// observed here renders the plain observe run-waiting line, never a gate-continue.
func gateObserveLine(o GateObservation) string {
	fields := []string{GateDecisionObserve, o.Outcome}
	switch o.Outcome {
	case GateOutcomeNoCurrentRun:
		// no trailing fields
	case GateOutcomeUnavailable:
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

// newGateObserveReport builds an applied (exit-0) observe report over the given
// observation lines.
func newGateObserveReport(obs ...GateObservation) RunGateVerdictObserveResult {
	r := RunGateVerdictObserveResult{Observations: obs}
	r.Envelope = NewEnvelope(OperationRunGateVerdict, ResultApplied)
	return r
}

// RunGateVerdictObserve reports the unattributed (observe-only) run-gate verdicts.
// hints are the raw positional arguments; each must parse as an integer change id
// (a non-integer — for example a gate key — is a usage error). See the section
// header for the no-writes / no-retry structural contract.
func RunGateVerdictObserve(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir string, hints []string) RunGateVerdictObserveResult {
	// Parse the hints. A non-integer positional is a gate key (or garbage), not a
	// change-id hint: usage error, non-zero exit, never a report line.
	hintIDs := make([]int, 0, len(hints))
	for _, h := range hints {
		id, err := strconv.Atoi(strings.TrimSpace(h))
		if err != nil {
			out := RunGateVerdictObserveResult{
				Reason:  ReasonGateUnattributedKey,
				Message: fmt.Sprintf("--unattributed takes change-id hints, not %q; a gate key is not a hint", h),
			}
			out.Envelope = NewEnvelope(OperationRunGateVerdict, ResultInvalidInput)
			return out
		}
		hintIDs = append(hintIDs, id)
	}

	// Re-sync + read the current in-progress set. A sync/read fault fails closed
	// to a single gate-unavailable line. This is the ONLY read; it writes nothing.
	inProgress, reason := observeInProgressIDs(ctx, deps, repoDir)
	if reason != "" {
		return newGateObserveReport(GateObservation{Outcome: GateOutcomeUnavailable, Reason: reason})
	}

	// Hints win when supplied (verified in input order); otherwise verify every
	// current in-progress id (sorted). No ids and no hints → no-current-run.
	ids := hintIDs
	if len(ids) == 0 {
		ids = inProgress
	}
	if len(ids) == 0 {
		return newGateObserveReport(GateObservation{Outcome: GateOutcomeNoCurrentRun})
	}

	obs := make([]GateObservation, 0, len(ids))
	for _, id := range ids {
		v := RunVerify(ctx, deps, wdeps, gdeps, repoDir, RunVerifyRequest{ID: id})
		obs = append(obs, observeFromVerdict(id, v))
	}
	return newGateObserveReport(obs...)
}

// observeInProgressIDs re-syncs to fresh origin and returns the current
// in-progress change ids (sorted), or a non-empty gate-unavailable reason token
// on a re-sync / corpus-read fault (fail closed). It writes nothing — the same
// read-only plumbing gate-before and attribution use.
func observeInProgressIDs(ctx context.Context, deps PlanningDeps, repoDir string) (ids []int, reason string) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return nil, ReasonGateSyncFailed
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return nil, ReasonGateChangesUnreadable
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return nil, ReasonGateChangesUnreadable
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
// becomes a gate-unavailable observation carrying RunVerify's own reason (or
// unknown-verdict) — never a retry: this path has no retry to grant.
func observeFromVerdict(id int, v RunVerifyResult) GateObservation {
	switch v.Verdict {
	case VerdictRunComplete, VerdictRunUnclaimed, VerdictRunHalted:
		return GateObservation{Outcome: v.Verdict, ID: id}
	case VerdictRunWaiting:
		// Handoff id and phase pass through verbatim — never reformatted.
		return GateObservation{Outcome: v.Verdict, ID: id, HandoffID: v.HandoffID, Phase: v.Phase}
	case VerdictRunIncomplete:
		return GateObservation{Outcome: v.Verdict, ID: id, Unmet: gateUnmetTokens(v)}
	default:
		reason := v.Reason
		if reason == "" {
			reason = GateReasonUnknownVerdict
		}
		return GateObservation{Outcome: GateOutcomeUnavailable, Reason: reason}
	}
}
