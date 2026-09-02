// Continuation redemption — `docket run gate-claim <key> <continuation-id>`
// (change 0359).
//
// A `gate-continue` verdict hands the resumed implement-next controller two
// tokens: the durable gate key and a single-use continuation id. This file is the
// redemption verb: it loads the durable gate record, verifies the presented
// continuation id against the stored one with a CONSTANT-TIME compare
// (crypto/subtle, so a mismatch leaks no timing signal about the secret), and, on
// a match, consumes the recovered drive's single-use handoff through the
// commandless gate-drive service. The claim's CAS is single-use at the DRIVE
// layer; clearing the record's continuation triple on success is single-use at the
// RECORD layer, so a replayed gate-claim finds no continuation to redeem.
//
// It fails CLOSED at every step: no stored continuation → no-continuation; a
// mismatch → continuation-mismatch; a HALTED claim (a raced owner, a drifted
// fingerprint) → a gate-stop-shaped refusal carrying the driver's own cause; a
// command fault or an unwired seam → gate-unavailable. The triple is cleared ONLY
// on a successful claim — every refusal leaves it intact so a legitimate retry with
// the correct id can still succeed. On success the JSON document carries the fresh
// owner generation the controller advances with; the human text names the drive id
// and the drive's recorded outcome ONLY — never the generation (spec:
// "Capabilities and owner generations never appear in human text").
package app

import (
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// OperationRunGateClaim is the operation key `run gate-claim` records in its
// envelope.
const OperationRunGateClaim = "run.gate-claim"

// GateClaimDecisionClaimed is the leading token of a successful claim report line.
// It joins gate-done / gate-retry-once / gate-stop / gate-continue / gate-observe
// as a decision word; a fail-closed refusal reuses GateDecisionStop.
const GateClaimDecisionClaimed = "gate-claimed"

// Fail-closed reason tokens for the claim path. A HALTED claim carries the
// driver's own cause alongside ReasonGateHaltedClaim.
const (
	// ReasonGateNoContinuation: the record carries no continuation triple — either
	// none was ever recorded, or a prior successful claim already cleared it
	// (single-use).
	ReasonGateNoContinuation = "no-continuation"
	// ReasonGateContinuationMismatch: the presented continuation id does not equal
	// the stored one (constant-time compare).
	ReasonGateContinuationMismatch = "continuation-mismatch"
	// ReasonGateHaltedClaim: the drive-layer claim returned a HALTED document
	// (unsafe ownership — a raced owner or a drifted fingerprint). The driver's own
	// cause travels alongside.
	ReasonGateHaltedClaim = "halted-claim"
	// ReasonGateClaimUnavailable: the claim seam could not be composed (no drive
	// store / supervisor), so no continuation can be redeemed. Fail closed.
	ReasonGateClaimUnavailable = "claim-unavailable"
	// ReasonGateClaimError: the drive-layer claim returned a command fault (an
	// unparseable request or an unknown drive) distinct from a HALTED refusal.
	ReasonGateClaimError = "claim-error"
)

// GateClaimOutcome is the drive-layer result RunGateClaim needs back from the
// claim seam: the fresh owner generation (JSON only), the drive's phase and
// recorded outcome, and — on an unsafe claim — the HALTED flag with the driver's
// cause.
type GateClaimOutcome struct {
	Generation string
	Phase      string
	Outcome    string
	Halted     bool
	Cause      string
}

// ClaimSeam is the drive-layer surface RunGateClaim needs: it consumes the
// single-use handoff of a recovered drive and reports the fresh owner generation,
// the drive's phase, and the recorded outcome. The production impl
// (gatedriveClaimSeam) composes the commandless gate-drive service for the claim
// CAS and the durable store for the phase projection; unit tests fake it.
type ClaimSeam interface {
	// Claim consumes handoffToken for driveID. A produced document — including a
	// HALTED refusal (Halted true, Cause set) — returns a nil error; a command
	// fault (unknown/malformed drive) returns a non-nil error.
	Claim(driveID, handoffToken string) (GateClaimOutcome, error)
}

// gatedriveClaimSeam is the production ClaimSeam. It reads the drive's phase from
// the durable store BEFORE the claim consumes the handoff, then delegates the
// claim CAS to the commandless gate-drive service (which reaches the process
// supervisor through the app boundary).
type gatedriveClaimSeam struct {
	svc   *GateDriveService
	store *gatedrive.Store
}

// NewClaimSeam composes the production ClaimSeam over the durable drive store at
// the repository's Git common directory and the native supervisor at exePath. A
// service-construction failure is returned so the caller can leave the claim path
// unwired (a nil seam) and RunGateClaim fails closed to claim-unavailable rather
// than panicking.
func NewClaimSeam(gitCommonDir, exePath string) (ClaimSeam, error) {
	svc, res, reason := NewCommandlessGateDriveService(gitCommonDir, exePath)
	if svc == nil {
		return nil, fmt.Errorf("gate drive service unavailable: %s (%s)", res, reason)
	}
	return &gatedriveClaimSeam{svc: svc, store: gatedrive.OpenStore(gitCommonDir)}, nil
}

// Claim reads the drive's phase (best-effort — a read fault leaves it empty),
// then consumes the handoff through the commandless service. A nil drive document
// is a command fault (surfaced as an error); a HALTED document is unsafe
// ownership (Halted true, Cause set); otherwise the fresh owner generation and
// recorded outcome are returned.
func (s *gatedriveClaimSeam) Claim(driveID, handoffToken string) (GateClaimOutcome, error) {
	phase := ""
	if rec, err := s.store.Load(driveID); err == nil {
		phase = rec.Phase
	}
	res := s.svc.Claim(driveID, handoffToken)
	if res.Drive == nil {
		// A command fault (no drive document): surface the bounded reason as an error
		// so RunGateClaim maps it to claim-error, never a fabricated success.
		return GateClaimOutcome{}, fmt.Errorf("gate drive claim failed: %s", res.Reason)
	}
	if res.Drive.Outcome == gatedrive.HALTED {
		return GateClaimOutcome{Phase: phase, Outcome: string(res.Drive.Outcome), Halted: true, Cause: res.Drive.Cause}, nil
	}
	return GateClaimOutcome{Generation: res.Drive.Generation, Phase: phase, Outcome: string(res.Drive.Outcome)}, nil
}

// RunGateClaimResult is the protocol-v1 document `run gate-claim` returns. It
// renders one report line and always exits 0 (a produced report line is not a
// process failure). Generation travels ONLY in the JSON document — HumanText never
// emits it (spec: ownership generations never appear in human text).
type RunGateClaimResult struct {
	Envelope
	Key      string `json:"key,omitempty"`
	Decision string `json:"decision,omitempty"` // gate-claimed | gate-stop
	// Outcome is the drive's recorded outcome on a successful claim; a refusal
	// carries no outcome (the reason token carries the disposition instead).
	Outcome    string `json:"outcome,omitempty"`
	DriveID    string `json:"drive_id,omitempty"`
	Generation string `json:"generation,omitempty"` // JSON ONLY — never HumanText
	Phase      string `json:"phase,omitempty"`
	Reason     string `json:"reason,omitempty"` // fail-closed reason token
	Cause      string `json:"cause,omitempty"`  // driver cause on a halted claim
	Terminal   bool   `json:"terminal"`
}

// HumanText renders the single claim report line. A success names the drive id and
// the drive's recorded outcome ONLY — never the generation, which is authority and
// travels solely in the JSON document. A refusal renders a gate-stop line carrying
// the reason token (and the driver's cause on a halted claim).
func (r RunGateClaimResult) HumanText() string {
	if r.Decision == GateClaimDecisionClaimed {
		return strings.Join([]string{r.Decision, r.Key, r.Outcome, r.DriveID}, " ")
	}
	fields := []string{GateDecisionStop, r.Key, r.Reason}
	if r.Cause != "" {
		fields = append(fields, r.Cause)
	}
	return strings.Join(fields, " ")
}

// RunGateClaim redeems a single-use continuation for the resumed implement-next
// controller. See the file header for the constant-time comparison, single-use,
// and fail-closed contracts. seam may be nil (an unwired drive layer) — the claim
// then fails closed to claim-unavailable without clearing the triple.
func RunGateClaim(repoDir, key, continuationID string, seam ClaimSeam) RunGateClaimResult {
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		// No record to persist to: fail closed to a gate-stop carrying the store's
		// typed reason token.
		return newGateClaimStop(key, gateStoreReason(err), "")
	}

	// No stored continuation — nothing to redeem. This is also the single-use
	// post-state a replayed claim reads after a prior success cleared the triple.
	if rec.ContinuationID == "" {
		return persistGateClaimStop(repoDir, key, rec, ReasonGateNoContinuation, "")
	}

	// Constant-time comparison of the presented id against the stored id: a
	// mismatch (including a length difference) leaks no timing signal about the
	// stored secret.
	if subtle.ConstantTimeCompare([]byte(continuationID), []byte(rec.ContinuationID)) != 1 {
		return persistGateClaimStop(repoDir, key, rec, ReasonGateContinuationMismatch, "")
	}

	// An unwired seam cannot redeem the continuation: fail closed WITHOUT clearing
	// the triple (a later invocation over a wired seam may still succeed).
	if seam == nil {
		return persistGateClaimStop(repoDir, key, rec, ReasonGateClaimUnavailable, "")
	}

	out, cerr := seam.Claim(rec.ContinuationDrive, rec.ContinuationHandoff)
	if cerr != nil {
		return persistGateClaimStop(repoDir, key, rec, ReasonGateClaimError, "")
	}
	if out.Halted {
		// Unsafe ownership: the triple is NOT cleared (only a success clears it), so
		// the driver's fail-closed HALT is surfaced with its own cause.
		return persistGateClaimStop(repoDir, key, rec, ReasonGateHaltedClaim, out.Cause)
	}

	// Success: clear the continuation triple (single-use at the record layer) and
	// return the fresh owner generation for the resumed controller to advance with.
	driveID := rec.ContinuationDrive
	rec.ContinuationID = ""
	rec.ContinuationDrive = ""
	rec.ContinuationHandoff = ""

	res := RunGateClaimResult{
		Key:        key,
		Decision:   GateClaimDecisionClaimed,
		Outcome:    out.Outcome,
		DriveID:    driveID,
		Generation: out.Generation,
		Phase:      out.Phase,
		Terminal:   false,
	}
	res.Envelope = NewEnvelope(OperationRunGateClaim, ResultApplied)

	// Persist the cleared triple best-effort: the claim's drive-layer CAS already
	// made the redemption durable, so a save fault does not un-redeem it.
	rec.Disposition = res.HumanText()
	rec.Terminal = res.Terminal
	_ = SaveGateRecord(repoDir, key, rec)
	return res
}

// newGateClaimStop builds a terminal gate-stop claim refusal (no record to
// persist — the load itself failed).
func newGateClaimStop(key, reason, cause string) RunGateClaimResult {
	r := RunGateClaimResult{Key: key, Decision: GateDecisionStop, Reason: reason, Cause: cause, Terminal: true}
	r.Envelope = NewEnvelope(OperationRunGateClaim, ResultApplied)
	return r
}

// persistGateClaimStop builds a terminal gate-stop claim refusal and records its
// disposition onto the (already-loaded) record. It never clears the continuation
// triple — a refusal leaves the record exactly as it found it so a legitimate
// retry with the correct id can still succeed.
func persistGateClaimStop(repoDir, key string, rec GateRecord, reason, cause string) RunGateClaimResult {
	res := newGateClaimStop(key, reason, cause)
	rec.Disposition = res.HumanText()
	rec.Terminal = res.Terminal
	_ = SaveGateRecord(repoDir, key, rec)
	return res
}

// Compile-time seam assertions.
var (
	_ ClaimSeam       = (*gatedriveClaimSeam)(nil)
	_ OperationResult = RunGateClaimResult{}
)
