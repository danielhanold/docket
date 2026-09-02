package app

import (
	"strings"
	"testing"
)

// These are the `docket run gate-claim <key> <continuation-id>` (change 0359)
// tests. gate-claim redeems the single-use continuation a gate-continue verdict
// recorded: it loads the durable record, constant-time-compares the presented id
// against the stored one, and consumes the recovered drive's handoff through a
// (faked here) claim seam. It fails closed on no-continuation / mismatch /
// halted-claim / command-fault / unwired-seam, clears the triple ONLY on success
// (single-use), and never emits the fresh owner generation in human text.

// fakeClaimSeam fakes the drive-layer surface RunGateClaim needs: it records the
// (driveID, handoffToken) it was called with and returns a canned outcome/error.
type fakeClaimSeam struct {
	out        GateClaimOutcome
	err        error
	gotDriveID string
	gotHandoff string
	calls      int
}

func (s *fakeClaimSeam) Claim(driveID, handoffToken string) (GateClaimOutcome, error) {
	s.calls++
	s.gotDriveID = driveID
	s.gotHandoff = handoffToken
	return s.out, s.err
}

// gateMintWithContinuation mints an armed record carrying a full continuation
// triple (all three fields set — a partial triple is a corrupt record), the state
// a gate-continue verdict leaves for the resumed controller to redeem.
func gateMintWithContinuation(t *testing.T, repoDir, cid, drive, handoff string) string {
	t.Helper()
	key := gateMintArmed(t, repoDir, nil, 1)
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = 3
	rec.ContinuationID = cid
	rec.ContinuationDrive = drive
	rec.ContinuationHandoff = handoff
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// TestGateClaimSuccessRedeemsAndClearsTriple: a matching continuation id claims
// the recovered drive, clears the triple (single-use at the record layer), and
// returns the fresh owner generation in JSON. The seam is called with the exact
// drive id + handoff token from the triple.
func TestGateClaimSuccessRedeemsAndClearsTriple(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: GateClaimOutcome{Generation: "freshgen", Phase: "build", Outcome: "WAITING"}}

	res := RunGateClaim(repo, key, "cid-abc", seam)

	if res.Decision != GateClaimDecisionClaimed {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateClaimDecisionClaimed)
	}
	if seam.gotDriveID != "d0opaque" || seam.gotHandoff != "h0token" {
		t.Errorf("seam called with (%q,%q), want (d0opaque,h0token)", seam.gotDriveID, seam.gotHandoff)
	}
	if res.DriveID != "d0opaque" || res.Generation != "freshgen" || res.Phase != "build" || res.Outcome != "WAITING" {
		t.Errorf("result = {drive:%q gen:%q phase:%q outcome:%q}, want {d0opaque freshgen build WAITING}",
			res.DriveID, res.Generation, res.Phase, res.Outcome)
	}
	if res.Terminal {
		t.Errorf("a successful claim must be nonterminal (the run continues)")
	}
	// The triple is cleared on disk (single-use).
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.ContinuationID != "" || rec.ContinuationDrive != "" || rec.ContinuationHandoff != "" {
		t.Errorf("triple not cleared: {%q,%q,%q}", rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff)
	}
}

// TestGateClaimRedactsGeneration: the generation travels only in the JSON
// document — HumanText names the drive id and outcome, never the generation.
func TestGateClaimRedactsGeneration(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: GateClaimOutcome{Generation: "secretgen", Phase: "build", Outcome: "WAITING"}}

	res := RunGateClaim(repo, key, "cid-abc", seam)
	human := res.HumanText()
	if strings.Contains(human, "secretgen") {
		t.Fatalf("HumanText leaked the generation: %q", human)
	}
	want := "gate-claimed " + key + " WAITING d0opaque"
	if human != want {
		t.Fatalf("HumanText = %q, want %q", human, want)
	}
}

// TestGateClaimSingleUse: a second claim after a successful redemption finds no
// continuation (the triple was cleared) and fails closed to no-continuation.
func TestGateClaimSingleUse(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: GateClaimOutcome{Generation: "freshgen", Phase: "build", Outcome: "WAITING"}}

	if first := RunGateClaim(repo, key, "cid-abc", seam); first.Decision != GateClaimDecisionClaimed {
		t.Fatalf("first claim Decision = %q, want claimed", first.Decision)
	}
	second := RunGateClaim(repo, key, "cid-abc", seam)
	if second.Decision != GateDecisionStop || second.Reason != ReasonGateNoContinuation {
		t.Fatalf("second claim = {%q,%q}, want {gate-stop,no-continuation}", second.Decision, second.Reason)
	}
	if seam.calls != 1 {
		t.Errorf("seam.Claim called %d times, want 1 (the second claim never reaches the drive layer)", seam.calls)
	}
}

// TestGateClaimNoContinuation: a record with no continuation triple fails closed
// to no-continuation and never touches the drive layer.
func TestGateClaimNoContinuation(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1) // armed, no triple
	seam := &fakeClaimSeam{}

	res := RunGateClaim(repo, key, "cid-abc", seam)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateNoContinuation {
		t.Fatalf("result = {%q,%q}, want {gate-stop,no-continuation}", res.Decision, res.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("seam.Claim called %d times, want 0", seam.calls)
	}
}

// TestGateClaimMismatch: a wrong continuation id fails closed to
// continuation-mismatch and leaves the triple intact for a legitimate retry.
func TestGateClaimMismatch(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-right", "d0opaque", "h0token")
	seam := &fakeClaimSeam{}

	res := RunGateClaim(repo, key, "cid-wrong", seam)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateContinuationMismatch {
		t.Fatalf("result = {%q,%q}, want {gate-stop,continuation-mismatch}", res.Decision, res.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("seam.Claim called %d times, want 0 (a mismatch never reaches the drive layer)", seam.calls)
	}
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.ContinuationID != "cid-right" || rec.ContinuationDrive != "d0opaque" || rec.ContinuationHandoff != "h0token" {
		t.Errorf("triple mutated on a mismatch: {%q,%q,%q}", rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff)
	}
}

// TestGateClaimMismatchDifferentLength: a length-differing id also fails closed
// (crypto/subtle returns 0 on unequal lengths) rather than panicking or matching.
func TestGateClaimMismatchDifferentLength(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	res := RunGateClaim(repo, key, "cid-abc-longer", &fakeClaimSeam{})
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateContinuationMismatch {
		t.Fatalf("result = {%q,%q}, want {gate-stop,continuation-mismatch}", res.Decision, res.Reason)
	}
}

// TestGateClaimHaltedCarriesCause: a HALTED drive-layer claim (unsafe ownership)
// fails closed to halted-claim carrying the driver's cause, and leaves the triple
// intact.
func TestGateClaimHaltedCarriesCause(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: GateClaimOutcome{Halted: true, Cause: "fingerprint-mismatch", Outcome: "HALTED"}}

	res := RunGateClaim(repo, key, "cid-abc", seam)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateHaltedClaim || res.Cause != "fingerprint-mismatch" {
		t.Fatalf("result = {%q,%q,cause=%q}, want {gate-stop,halted-claim,fingerprint-mismatch}", res.Decision, res.Reason, res.Cause)
	}
	if got := res.HumanText(); got != "gate-stop "+key+" halted-claim fingerprint-mismatch" {
		t.Errorf("HumanText = %q", got)
	}
	rec, _ := LoadGateRecord(repo, key)
	if rec.ContinuationID == "" {
		t.Errorf("triple cleared on a halted claim — only a success may clear it")
	}
}

// TestGateClaimCommandError: a command fault from the drive layer fails closed to
// claim-error and leaves the triple intact.
func TestGateClaimCommandError(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{err: errFake}

	res := RunGateClaim(repo, key, "cid-abc", seam)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateClaimError {
		t.Fatalf("result = {%q,%q}, want {gate-stop,claim-error}", res.Decision, res.Reason)
	}
	rec, _ := LoadGateRecord(repo, key)
	if rec.ContinuationID == "" {
		t.Errorf("triple cleared on a command fault — only a success may clear it")
	}
}

// TestGateClaimNilSeam: an unwired seam fails closed to claim-unavailable without
// clearing the triple.
func TestGateClaimNilSeam(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")

	res := RunGateClaim(repo, key, "cid-abc", nil)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateClaimUnavailable {
		t.Fatalf("result = {%q,%q}, want {gate-stop,claim-unavailable}", res.Decision, res.Reason)
	}
	rec, _ := LoadGateRecord(repo, key)
	if rec.ContinuationID == "" {
		t.Errorf("triple cleared with an unwired seam — nothing was redeemed")
	}
}

// TestGateClaimLoadErrorFailsClosed: a malformed key never touches the filesystem
// and fails closed to a gate-stop carrying the store's typed reason token.
func TestGateClaimLoadErrorFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	res := RunGateClaim(repo, "Bad/Key", "cid-abc", &fakeClaimSeam{})
	if res.Decision != GateDecisionStop {
		t.Fatalf("Decision = %q, want gate-stop", res.Decision)
	}
	if res.Reason != string(ErrGateMalformedKey) {
		t.Fatalf("Reason = %q, want %q", res.Reason, ErrGateMalformedKey)
	}
}

// errFake is a sentinel command-fault error for the claim seam.
var errFake = fakeErr("fake command fault")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
