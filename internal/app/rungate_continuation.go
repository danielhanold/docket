// The continuation seam for the run gate's `gate-continue` decision (change 0359).
//
// A tracked gate drive that a dispatched implement-next run left live (or wrote a
// verdict for and then died before its parent consumed it) is HEALTHY work to
// CONTINUE, not a quiescent incomplete to stop and spend the one retry on. This
// file is the drive-layer surface RunGateVerdict needs to recognize and recover
// such a drive:
//
//   - LocateOuterDrive resolves the candidate drives nested under this dispatch's
//     outer recovery scope (matched by change id + the outer child-context hash);
//     exactly one authorizes an outer takeover.
//   - TakeoverAndHandoff performs the event-authorized outer takeover (the CALLER
//     asserts the direct child's dispatch-return event simply by calling) and then
//     an immediate NORMAL single-use handoff by the fresh owner. That synthesis is
//     the whole point: it produces exactly the cooperative-handoff shape the
//     UNCHANGED RunVerify run-waiting predicate already validates, so the verdict
//     path re-runs RunVerify to certify the continuation rather than adding a
//     generic liveness branch to RunVerify.
//   - ExistingHandoffToken reads the unclaimed handoff token of a drive a worker
//     already handed off cooperatively (nothing to take over) so the record can
//     carry it into the continuation triple.
//
// The production impl composes the gatedrive Store + Driver from the repository's
// Git common dir and this binary's path; unit tests fake the whole seam. A
// capability, owner generation, or handoff token NEVER appears in human text — it
// travels only in the private record and the protocol document.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
)

// GateDecisionContinue is the NONTERMINAL gate decision (change 0359): the same
// implement-next attempt owns live or terminal-unconsumed tracked work, so the
// gate keeps the same key and spends no retry. It joins gate-done / gate-retry-once
// / gate-stop (attributed) and gate-observe (unattributed) as a leading report
// token, but unlike gate-retry-once it is a continuation of the SAME attempt, not
// a second attempt.
const GateDecisionContinue = "gate-continue"

// Continuation-path gate-unavailable reason tokens. A takeover that HALTs passes
// the driver's own cause token through instead of these.
const (
	// ReasonGateContinuationUnavailable: a run-waiting continuation could not be
	// recorded (no continuation seam wired, or the drive's handoff token could not
	// be read). Fail closed rather than emit an unredeemable continuation.
	ReasonGateContinuationUnavailable = "continuation-unavailable"
	// ReasonGateContinuationUnverified: an outer takeover synthesized a handoff, but
	// the re-run of the unchanged RunVerify predicate did not certify it as
	// run-waiting — an unsafe state that earns neither retry nor continuation.
	ReasonGateContinuationUnverified = "continuation-unverified"
	// ReasonGateLocateFailed: the outer-drive candidate scan itself faulted.
	ReasonGateLocateFailed = "locate-failed"
	// ReasonGateTakeoverError: the outer takeover returned a command error (not a
	// HALTED document) — an operational fault, fail closed.
	ReasonGateTakeoverError = "takeover-error"
	// ReasonGateTakeoverAmbiguous: more than one candidate drive resolved for the
	// outer scope — unsafe ownership, never continued or retried. Mirrors the
	// driver's own takeover-ambiguous cause token.
	ReasonGateTakeoverAmbiguous = gatedrive.CauseTakeoverAmbiguous
)

// ContinuationSeam is the drive-layer surface the verdict path needs. The
// production impl (gatedriveContinuationSeam) composes gatedrive Store+Driver from
// the Git common dir + exe path; unit tests fake it.
type ContinuationSeam interface {
	// LocateOuterDrive returns the candidate drive ids for (changeID,
	// childContextHash) whose outcome is nonterminal OR terminal-unconsumed; exactly
	// one is required upstream to authorize a takeover.
	LocateOuterDrive(changeID int, childContextHash string) ([]string, error)
	// TakeoverAndHandoff performs the event-authorized outer takeover of driveID
	// under (scopeID, parentCap), then an immediate NORMAL Handoff by the fresh
	// owner — returning the single-use handoff token. A HALTED takeover (unsafe
	// ownership) returns halted=true with the driver's cause; a command fault
	// returns a non-nil err.
	TakeoverAndHandoff(scopeID, parentCap, driveID string) (handoffToken string, halted bool, cause string, err error)
	// ExistingHandoffToken returns the unclaimed handoff token of driveID (a worker
	// already handed off cooperatively — nothing to take over). It fails typed when
	// no unclaimed handoff exists.
	ExistingHandoffToken(driveID string) (string, error)
}

// gatedriveContinuationSeam is the production ContinuationSeam over the durable
// gate-drive store and the native process supervisor. internal/app is the only
// layer allowed to reach internal/process, so the CLI composes this through the
// app boundary rather than importing the supervisor itself.
type gatedriveContinuationSeam struct {
	store  *gatedrive.Store
	driver *gatedrive.Driver
}

// NewContinuationSeam composes the production continuation seam, rooting the
// durable drive store at the repository's Git common directory and binding the
// native supervisor at exePath. A supervisor construction failure is returned so
// the caller can leave the continuation path unwired (a nil seam) rather than
// failing the whole verdict.
func NewContinuationSeam(gitCommonDir, exePath string) (ContinuationSeam, error) {
	proc, err := process.NewService(exePath)
	if err != nil {
		return nil, err
	}
	store := gatedrive.OpenStore(gitCommonDir)
	return &gatedriveContinuationSeam{store: store, driver: gatedrive.NewSystemDriver(store, proc)}, nil
}

func (s *gatedriveContinuationSeam) LocateOuterDrive(changeID int, childContextHash string) ([]string, error) {
	return s.store.FindScopeDriveIDs(strconv.Itoa(changeID), childContextHash)
}

func (s *gatedriveContinuationSeam) ExistingHandoffToken(driveID string) (string, error) {
	return s.store.ContinuationHandle(driveID)
}

func (s *gatedriveContinuationSeam) TakeoverAndHandoff(scopeID, parentCap, driveID string) (string, bool, string, error) {
	// Event-authorized takeover: on success the returned document's Generation is
	// the fresh owner generation the parent now holds. A HALTED document is unsafe
	// ownership (ambiguity, identity drift, expired deadline, outstanding handoff),
	// never a launch or a red result.
	doc, err := s.driver.Takeover(scopeID, parentCap, driveID)
	if err != nil {
		return "", false, "", err
	}
	if doc.Outcome == gatedrive.HALTED {
		return "", true, doc.Cause, nil
	}
	// Immediately hand off by the fresh owner to synthesize the normal single-use
	// handoff the unchanged run-waiting predicate validates. Its Generation is the
	// handoff token a resumed controller later claims.
	hdoc, herr := s.driver.Handoff(doc.DriveID, doc.Generation)
	if herr != nil {
		return "", false, "", herr
	}
	if hdoc.Outcome == gatedrive.HALTED {
		return "", true, hdoc.Cause, nil
	}
	return hdoc.Generation, false, "", nil
}

// newContinuationID mints a fresh single-use continuation redemption token — an
// opaque lookup token (not encoded state), 16 lowercase hex chars.
func newContinuationID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
