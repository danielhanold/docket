package app

import (
	"context"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/process"
)

// This file is the `evidence record` and `evidence verify` operations: the thin
// app-layer wiring that turns a landed gate run into an immutable build-evidence
// record, and re-checks a record's canonical bytes against an exact head. The
// evidence codec, the process supervisor, and the workspace service all stay in
// their landed packages and are only wired here.
//
// Two properties are load-bearing and enforced here, not delegated:
//
//   - Evidence is created ONLY from a `passed` terminal observation whose head
//     equals the current feature head. A failed, running, stopped, signalled, or
//     vanished run produces no record, and a probe error — a run dir the process
//     layer cannot read or parse — is its own typed failure, NEVER folded into
//     the clean "vanished" absence (learning probe-error-is-not-clean-absence).
//   - The recorded command is the OBSERVED gate command: the resolved
//     build.test_command, read from authoritative config. There is no
//     agent-supplied command and no agent-supplied `passed` boolean in the
//     request shapes — the terminal record and the config are the only inputs.
//
// EvidenceRecord is BUILD-owned (change 0374): it "validates against build
// configuration" and "no longer re-resolves finalize.test_command". An explicit
// build.gate: off mints truthful skipped evidence with no run observed; a local
// build gate with no build.test_command is a typed setup refusal.

// Operation names the two evidence operations record in their envelopes.
const (
	OperationEvidenceRecord = "evidence.record"
	OperationEvidenceVerify = "evidence.verify"
)

// Stable machine reasons for the evidence operations' typed refusals. Message
// text is explanatory and must not be parsed.
const (
	// ReasonEvidenceGateFailed / -Running / -Stopped / -Signaled / -Vanished are
	// the clean, well-formed non-passed terminal (or live) observations. None
	// produces a record; each is its own reason so a caller can tell them apart.
	ReasonEvidenceGateFailed   = "gate-failed"
	ReasonEvidenceGateRunning  = "gate-running"
	ReasonEvidenceGateStopped  = "gate-stopped"
	ReasonEvidenceGateSignaled = "gate-signaled"
	ReasonEvidenceGateVanished = "gate-vanished"
	// ReasonEvidenceProbeMalformed / -Unreadable / -Blocked / -Error are probe
	// FAILURES: the process layer could not read or parse the run dir. They are
	// distinct from every "gate-*" reason above — a corrupt or unreadable run is
	// never reported as a clean absence (probe-error-is-not-clean-absence).
	ReasonEvidenceProbeMalformed  = "probe-malformed"
	ReasonEvidenceProbeUnreadable = "probe-unreadable"
	ReasonEvidenceProbeBlocked    = "probe-blocked"
	ReasonEvidenceProbeError      = "probe-error"
	// ReasonEvidenceHeadMismatch: the run passed, but the request head is not the
	// current feature head, so recording it would certify the wrong commit.
	ReasonEvidenceHeadMismatch = "head-mismatch"
	// ReasonEvidenceUnconfiguredGate: build.gate is local but build.test_command
	// is unconfigured, so there is no gate command to run or record.
	ReasonEvidenceUnconfiguredGate = "unconfigured-gate-command"
	// ReasonEvidenceMissingRunDir: build.gate is local but the request named no
	// run dir to observe. Only the local-gate path needs one; gate-off skips it.
	ReasonEvidenceMissingRunDir = "missing-run-dir"
	// ReasonEvidenceInvalidRecord: NewRecord rejected the assembled command/head
	// — a defensive internal guard, not a caller-reachable path.
	ReasonEvidenceInvalidRecord = "invalid-record"
	// Verify verdicts that are not "verified" carry these reasons.
	ReasonEvidenceStale     = "stale-head"
	ReasonEvidenceMissing   = "missing-evidence"
	ReasonEvidenceMalformed = "malformed-evidence"
)

// EvidenceRecordRequest is the closed request for `evidence record`. There is
// deliberately no command field (the gate command is observed from config) and
// no `passed` boolean (the terminal record decides). RunDir is absolute.
type EvidenceRecordRequest struct {
	ID     int    `json:"id"`
	RunDir string `json:"run_dir"`
	Head   string `json:"head"`
}

// EvidenceVerifyRequest is the closed request for `evidence verify`: the raw
// canonical evidence-record bytes and the exact head to check them against.
type EvidenceVerifyRequest struct {
	RecordFile []byte `json:"-"`
	Head       string `json:"head"`
}

// EvidenceOpResult is the protocol-v1 document both evidence operations return.
// A recorded outcome carries the immutable record fields and the canonical
// rendered block; a verify outcome carries the verdict; a refusal carries a
// stable reason and explanatory message. The block is Docket-owned canonical
// bytes, not an authored document body, so it is safe to carry.
type EvidenceOpResult struct {
	Envelope
	ID      int    `json:"id,omitempty"`
	Command string `json:"command,omitempty"`
	Head    string `json:"head,omitempty"`
	RanAt   string `json:"ran_at,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Block   string `json:"block,omitempty"`
	Verdict string `json:"verdict,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// HumanText renders a one-line summary naming identity and the canonical facts
// only — never an authored document body.
func (r EvidenceOpResult) HumanText() string {
	if r.Result == ResultApplied {
		if r.Verdict != "" {
			return r.Operation + ": " + r.Verdict + " (head " + shortCommit(r.Head) + ")"
		}
		if r.Command == "" {
			// A skipped record carries no command; name its skip reason instead.
			return r.Operation + ": recorded " + r.Outcome + " at " + shortCommit(r.Head) + " (" + r.Reason + ")"
		}
		return r.Operation + ": recorded " + r.Outcome + " at " + shortCommit(r.Head) + " (" + r.Command + ")"
	}
	if r.Verdict != "" {
		return r.Operation + ": " + string(r.Result) + " (" + r.Verdict + ")"
	}
	if r.Reason != "" {
		return r.Operation + ": " + string(r.Result) + " (" + r.Reason + ")"
	}
	return r.Operation + ": " + string(r.Result)
}

// newEvidenceRefusal stamps a typed refusal envelope for opKey.
func newEvidenceRefusal(opKey string, result Result, reason, message string, id int) EvidenceOpResult {
	return EvidenceOpResult{Envelope: NewEnvelope(opKey, result), ID: id, Reason: reason, Message: message}
}

// EvidenceRecord resolves the BUILD gate policy, then either mints truthful
// skipped evidence (build.gate: off) or, for a local gate, requires a `passed`
// terminal at the current feature head and records build.test_command. It
// returns the immutable typed record plus its canonical rendered block. It
// writes no second evidence store: the block travels as bytes and becomes the
// durable record only after `pr publish`.
func EvidenceRecord(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req EvidenceRecordRequest) EvidenceOpResult {
	// (1) Pin authoritative config FIRST — the build gate policy decides
	// everything downstream, including whether a run is observed at all.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, r := classifyStatusError(ctx, err)
		return newEvidenceRefusal(OperationEvidenceRecord, result, r, err.Error(), req.ID)
	}
	build := pin.Config.Effective.Build

	// (2) build.gate: off — an explicit no-gate policy. Mint truthful skipped
	// evidence at the verified current feature head; observe no run.
	if build.Gate.Value == "off" {
		if refusal, ok := verifyFeatureHead(ctx, deps, wdeps, repoDir, req); !ok {
			return refusal
		}
		rec, err := evidence.NewSkippedRecord(req.Head, deps.Clock.Now())
		if err != nil {
			return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidInput, ReasonEvidenceInvalidRecord, err.Error(), req.ID)
		}
		return EvidenceOpResult{
			Envelope: NewEnvelope(OperationEvidenceRecord, ResultApplied),
			ID:       req.ID,
			Head:     rec.Head,
			RanAt:    rec.RanAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Outcome:  string(rec.Result),
			Reason:   rec.Reason,
			Block:    evidence.Render(rec),
		}
	}

	// (3) A local build gate records build.test_command — read from
	// authoritative config, never the request, never finalize.test_command.
	command := build.TestCommand.Value
	if command == "" {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultUnsupportedConfig, ReasonEvidenceUnconfiguredGate,
			"build.gate is local but build.test_command is unconfigured; run `docket repository configure-tests` and review the pending edit", req.ID)
	}

	// (4) A local gate must observe a real run dir.
	if req.RunDir == "" {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidInput, ReasonEvidenceMissingRunDir,
			"build.gate is local; --run must name the gate run directory to observe", req.ID)
	}

	// (5) Observe the run. A probe error is a distinct typed failure, never a
	// clean absence (probe-error-is-not-clean-absence).
	svc, res, reason := gateService()
	if svc == nil {
		return newEvidenceRefusal(OperationEvidenceRecord, res, reason, "cannot build the process service", req.ID)
	}
	obs, err := svc.Observe(req.RunDir)
	if err != nil {
		return mapProbeError(req.ID, err)
	}
	if obs.State != process.StatePassed {
		result, r := nonPassedRefusal(obs.State)
		return newEvidenceRefusal(OperationEvidenceRecord, result, r,
			"the gate run is "+string(obs.State)+", not passed; no evidence is created", req.ID)
	}

	// (6) The request head must be the CURRENT feature head — the same predicate
	// the gate-off path enforces (shared: verifyFeatureHead).
	if refusal, ok := verifyFeatureHead(ctx, deps, wdeps, repoDir, req); !ok {
		return refusal
	}

	// (7) Build the immutable green record and render its canonical block.
	rec, err := evidence.NewRecord(command, req.Head, deps.Clock.Now())
	if err != nil {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidInput, ReasonEvidenceInvalidRecord, err.Error(), req.ID)
	}
	return EvidenceOpResult{
		Envelope: NewEnvelope(OperationEvidenceRecord, ResultApplied),
		ID:       req.ID,
		Command:  rec.Command,
		Head:     rec.Head,
		RanAt:    rec.RanAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Outcome:  string(rec.Result),
		Block:    evidence.Render(rec),
	}
}

// verifyFeatureHead confirms req.Head is the CURRENT feature head via the landed
// workspace service — the shared precondition of both the green record path and
// the gate-off skipped path (change 0374). A mismatch means a fix moved HEAD
// since the gate, so the run no longer certifies this commit. Extracting it
// keeps the two callers from duplicating the predicate
// (learning duplicated-gate-copies-the-whole-predicate). It returns a typed
// refusal and false when the workspace cannot be inspected or the head moved.
func verifyFeatureHead(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req EvidenceRecordRequest) (EvidenceOpResult, bool) {
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: req.ID})
	if insp.Result != ResultApplied {
		// Propagate the workspace refusal verbatim under the evidence operation.
		return newEvidenceRefusal(OperationEvidenceRecord, insp.Result, insp.Reason, insp.Message, req.ID), false
	}
	if insp.Head != req.Head {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidState, ReasonEvidenceHeadMismatch,
			"the request head is not the current feature head; the run no longer certifies this commit", req.ID), false
	}
	return EvidenceOpResult{}, true
}

// EvidenceVerify reparses the supplied evidence bytes and checks them against an
// exact head. It trusts no prior command result — only the bytes and the head.
// A verified record is applied; stale (the invalidate-on-fix property), missing,
// and malformed are non-applied verdicts a caller must act on before publishing.
func EvidenceVerify(req EvidenceVerifyRequest) EvidenceOpResult {
	verdict := evidence.Verify(req.RecordFile, req.Head)
	base := EvidenceOpResult{
		Envelope: NewEnvelope(OperationEvidenceVerify, ResultApplied),
		Head:     req.Head,
		Verdict:  string(verdict),
	}
	switch verdict {
	case evidence.VerdictVerified:
		return base
	case evidence.VerdictStale:
		base.Result = ResultInvalidState
		base.Reason = ReasonEvidenceStale
		base.Message = "the evidence is green but names a different head; a fix moved HEAD, so it must be re-gated"
	case evidence.VerdictMissing:
		base.Result = ResultInvalidInput
		base.Reason = ReasonEvidenceMissing
		base.Message = "the record file carries no build-evidence block"
	default: // VerdictMalformed
		base.Result = ResultInvalidInput
		base.Reason = ReasonEvidenceMalformed
		base.Message = "the record file carries a build-evidence block that does not parse"
	}
	return base
}

// nonPassedRefusal maps a clean, well-formed non-passed observation to its
// protocol result and stable reason. These are absences of a pass, distinct
// from a probe failure (mapProbeError).
func nonPassedRefusal(state process.State) (Result, string) {
	switch state {
	case process.StateFailed:
		return ResultInvalidState, ReasonEvidenceGateFailed
	case process.StateRunning:
		return ResultInvalidState, ReasonEvidenceGateRunning
	case process.StateStopped:
		return ResultInvalidState, ReasonEvidenceGateStopped
	case process.StateSignaled:
		return ResultInterrupted, ReasonEvidenceGateSignaled
	case process.StateVanished:
		return ResultInvalidState, ReasonEvidenceGateVanished
	default:
		return ResultInternalError, ReasonEvidenceProbeError
	}
}

// mapProbeError folds a process Observe FAILURE onto the protocol taxonomy with
// a probe-specific reason. A malformed run dir (invalid-state) and an unreadable
// one (external) get distinct reasons so neither is mistaken for a clean
// absence. The failure's own bounded safe text rides through as the message.
func mapProbeError(id int, err error) EvidenceOpResult {
	result := ResultInternalError
	reason := ReasonEvidenceProbeError
	message := err.Error()
	if f, ok := process.AsFailure(err); ok {
		message = f.Error()
		switch f.Class {
		case process.FailInvalidState:
			result, reason = ResultInvalidState, ReasonEvidenceProbeMalformed
		case process.FailExternal:
			result, reason = ResultExternalFailed, ReasonEvidenceProbeUnreadable
		case process.FailBlocked:
			result, reason = ResultBlocked, ReasonEvidenceProbeBlocked
		case process.FailInvalidInput:
			result, reason = ResultInvalidInput, ReasonEvidenceProbeError
		}
	}
	return newEvidenceRefusal(OperationEvidenceRecord, result, reason, message, id)
}
