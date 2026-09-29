//go:build integration

package app

import (
	"strings"
	"testing"
)

// These are the `docket run continue <key> <continuation-id>` (change 0359)
// tests. run continue redeems the single-use continuation a run-continue verdict
// recorded: it loads the durable record, constant-time-compares the presented id
// against the stored one, and consumes the recovered drive's handoff through a
// (faked here) claim seam. It fails closed on no-continuation / mismatch /
// halted-claim / command-fault / unwired-seam, clears the triple ONLY on success
// (single-use), and never emits the fresh owner generation in human text.

// fakeClaimSeam fakes the drive-layer surface RunContinue needs: it records the
// (driveID, handoffToken) it was called with and returns a canned outcome/error.
type fakeClaimSeam struct {
	out        RunContinueOutcome
	err        error
	gotDriveID string
	gotHandoff string
	calls      int
}

func (s *fakeClaimSeam) Claim(driveID, handoffToken string) (RunContinueOutcome, error) {
	s.calls++
	s.gotDriveID = driveID
	s.gotHandoff = handoffToken
	return s.out, s.err
}

// runTrackerMintWithContinuation mints an armed record carrying a full continuation
// triple (all three fields set — a partial triple is a corrupt record), the state
// a run-continue verdict leaves for the resumed controller to redeem.
func runTrackerMintWithContinuation(t *testing.T, repoDir, cid, drive, handoff string) string {
	t.Helper()
	key := runTrackerMintStarted(t, repoDir, nil, 1, "")
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.AttributedID = 3
	rec.ContinuationID = cid
	rec.ContinuationDrive = drive
	rec.ContinuationHandoff = handoff
	if err := SaveRunTrackerRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}
	return key
}

// TestIntegrationRunStartGateClaimSuccessRedeemsAndClearsTriple: a matching continuation id claims
// the recovered drive, clears the triple (single-use at the record layer), and
// returns the fresh owner generation in JSON. The seam is called with the exact
// drive id + handoff token from the triple.
func TestIntegrationRunStartGateClaimSuccessRedeemsAndClearsTriple(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: RunContinueOutcome{Generation: "freshgen", Phase: "build", Outcome: "WAITING"}}

	res := RunContinue(repo, key, "cid-abc", seam)

	if res.Decision != RunContinueDecisionContinued {
		t.Fatalf("Decision = %q, want %q", res.Decision, RunContinueDecisionContinued)
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
	rec, err := LoadRunTrackerRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.ContinuationID != "" || rec.ContinuationDrive != "" || rec.ContinuationHandoff != "" {
		t.Errorf("triple not cleared: {%q,%q,%q}", rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff)
	}
}

// TestIntegrationRunStartGateClaimRedactsGeneration: the generation travels only in the JSON
// document — HumanText names the drive id and outcome, never the generation.
func TestIntegrationRunStartGateClaimRedactsGeneration(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: RunContinueOutcome{Generation: "secretgen", Phase: "build", Outcome: "WAITING"}}

	res := RunContinue(repo, key, "cid-abc", seam)
	human := res.HumanText()
	if strings.Contains(human, "secretgen") {
		t.Fatalf("HumanText leaked the generation: %q", human)
	}
	want := "run-continued " + key + " WAITING d0opaque"
	if human != want {
		t.Fatalf("HumanText = %q, want %q", human, want)
	}
}

// TestIntegrationRunStartGateClaimSingleUse: a second claim after a successful redemption finds no
// continuation (the triple was cleared) and fails closed to no-continuation.
func TestIntegrationRunStartGateClaimSingleUse(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: RunContinueOutcome{Generation: "freshgen", Phase: "build", Outcome: "WAITING"}}

	if first := RunContinue(repo, key, "cid-abc", seam); first.Decision != RunContinueDecisionContinued {
		t.Fatalf("first claim Decision = %q, want claimed", first.Decision)
	}
	second := RunContinue(repo, key, "cid-abc", seam)
	if second.Decision != RunDecisionStop || second.Reason != ReasonRunNoContinuation {
		t.Fatalf("second claim = {%q,%q}, want {run-stop,no-continuation}", second.Decision, second.Reason)
	}
	if seam.calls != 1 {
		t.Errorf("seam.Claim called %d times, want 1 (the second claim never reaches the drive layer)", seam.calls)
	}
}

// TestIntegrationRunStartGateClaimNoContinuation: a record with no continuation triple fails closed
// to no-continuation and never touches the drive layer.
func TestIntegrationRunStartGateClaimNoContinuation(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "") // armed, no triple
	seam := &fakeClaimSeam{}

	res := RunContinue(repo, key, "cid-abc", seam)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunNoContinuation {
		t.Fatalf("result = {%q,%q}, want {run-stop,no-continuation}", res.Decision, res.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("seam.Claim called %d times, want 0", seam.calls)
	}
}

// TestIntegrationRunStartGateClaimMismatch: a wrong continuation id fails closed to
// continuation-mismatch and leaves the triple intact for a legitimate retry.
func TestIntegrationRunStartGateClaimMismatch(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-right", "d0opaque", "h0token")
	seam := &fakeClaimSeam{}

	res := RunContinue(repo, key, "cid-wrong", seam)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunContinuationMismatch {
		t.Fatalf("result = {%q,%q}, want {run-stop,continuation-mismatch}", res.Decision, res.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("seam.Claim called %d times, want 0 (a mismatch never reaches the drive layer)", seam.calls)
	}
	rec, err := LoadRunTrackerRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.ContinuationID != "cid-right" || rec.ContinuationDrive != "d0opaque" || rec.ContinuationHandoff != "h0token" {
		t.Errorf("triple mutated on a mismatch: {%q,%q,%q}", rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff)
	}
}

// TestIntegrationRunStartGateClaimMismatchDifferentLength: a length-differing id also fails closed
// (crypto/subtle returns 0 on unequal lengths) rather than panicking or matching.
func TestIntegrationRunStartGateClaimMismatchDifferentLength(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	res := RunContinue(repo, key, "cid-abc-longer", &fakeClaimSeam{})
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunContinuationMismatch {
		t.Fatalf("result = {%q,%q}, want {run-stop,continuation-mismatch}", res.Decision, res.Reason)
	}
}

// TestIntegrationRunStartGateClaimHaltedCarriesCause: a HALTED drive-layer claim (unsafe ownership)
// fails closed to halted-claim carrying the driver's cause, and leaves the triple
// intact.
func TestIntegrationRunStartGateClaimHaltedCarriesCause(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{out: RunContinueOutcome{Halted: true, Cause: "fingerprint-mismatch", Outcome: "HALTED"}}

	res := RunContinue(repo, key, "cid-abc", seam)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunHaltedClaim || res.Cause != "fingerprint-mismatch" {
		t.Fatalf("result = {%q,%q,cause=%q}, want {run-stop,halted-claim,fingerprint-mismatch}", res.Decision, res.Reason, res.Cause)
	}
	if got := res.HumanText(); got != "run-stop "+key+" halted-claim fingerprint-mismatch" {
		t.Errorf("HumanText = %q", got)
	}
	rec, _ := LoadRunTrackerRecord(repo, key)
	if rec.ContinuationID == "" {
		t.Errorf("triple cleared on a halted claim — only a success may clear it")
	}
}

// TestIntegrationRunStartGateClaimCommandError: a command fault from the drive layer fails closed to
// claim-error and leaves the triple intact.
func TestIntegrationRunStartGateClaimCommandError(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")
	seam := &fakeClaimSeam{err: errFake}

	res := RunContinue(repo, key, "cid-abc", seam)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunClaimError {
		t.Fatalf("result = {%q,%q}, want {run-stop,claim-error}", res.Decision, res.Reason)
	}
	rec, _ := LoadRunTrackerRecord(repo, key)
	if rec.ContinuationID == "" {
		t.Errorf("triple cleared on a command fault — only a success may clear it")
	}
}

// TestIntegrationRunStartGateClaimNilSeam: an unwired seam fails closed to claim-unavailable without
// clearing the triple.
func TestIntegrationRunStartGateClaimNilSeam(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintWithContinuation(t, repo, "cid-abc", "d0opaque", "h0token")

	res := RunContinue(repo, key, "cid-abc", nil)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunClaimUnavailable {
		t.Fatalf("result = {%q,%q}, want {run-stop,claim-unavailable}", res.Decision, res.Reason)
	}
	rec, _ := LoadRunTrackerRecord(repo, key)
	if rec.ContinuationID == "" {
		t.Errorf("triple cleared with an unwired seam — nothing was redeemed")
	}
}

// TestIntegrationRunStartGateClaimLoadErrorFailsClosed: a malformed key never touches the filesystem
// and fails closed to a run-stop carrying the store's typed reason token.
func TestIntegrationRunStartGateClaimLoadErrorFailsClosed(t *testing.T) {
	repo := newRunTrackerRepo(t)
	res := RunContinue(repo, "Bad/Key", "cid-abc", &fakeClaimSeam{})
	if res.Decision != RunDecisionStop {
		t.Fatalf("Decision = %q, want run-stop", res.Decision)
	}
	if res.Reason != string(ErrRunTrackerMalformedKey) {
		t.Fatalf("Reason = %q, want %q", res.Reason, ErrRunTrackerMalformedKey)
	}
}

// errFake is a sentinel command-fault error for the claim seam.
var errFake = fakeErr("fake command fault")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
