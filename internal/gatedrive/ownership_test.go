package gatedrive

import (
	"sync"
	"testing"
)

// matchingFP is the fingerprint sampleRecord persists — the exact drive-start
// identity a handoff and claim must still match.
func matchingFP() Fingerprint { return sampleRecord().Fingerprint }

// mismatchFP mutates one dimension of the drive-start identity so it no longer
// matches — a single-dimension drift is enough for Equal to report false.
func mismatchFP() Fingerprint {
	fp := matchingFP()
	fp.Entries++
	return fp
}

// newHandedOffDrive persists a fresh drive owned by "owner-g0" and immediately
// hands it off, returning the drive id, the invalidated owner generation, and
// the single-use receipt. It is the common fixture for the claim-side tests.
func newHandedOffDrive(t *testing.T) (s *Store, id, oldOwner string, receipt handoffReceipt) {
	t.Helper()
	s = OpenStore(t.TempDir())
	rec := sampleRecord()
	oldOwner = rec.OwnerGeneration // "owner-g0"
	id, _, err := s.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	receipt, err = s.writeHandoffReceipt(id, oldOwner, matchingFP())
	if err != nil {
		t.Fatalf("writeHandoffReceipt: %v", err)
	}
	return s, id, oldOwner, receipt
}

// TestHandoffInvalidatesOldOwner proves the current owner can create a handoff
// and that doing so invalidates the old owner: after the handoff the persisted
// record no longer verifies the old generation, so an old-owner advance is
// rejected.
func TestHandoffInvalidatesOldOwner(t *testing.T) {
	s, id, oldOwner, receipt := newHandedOffDrive(t)

	if receipt.HandoffGeneration == "" {
		t.Fatalf("handoff must mint a non-empty single-use handoff generation")
	}
	if receipt.HandoffGeneration == oldOwner {
		t.Fatalf("the handoff generation must differ from the owner generation it supersedes")
	}
	if receipt.SupersededOwner != oldOwner {
		t.Fatalf("receipt must record the superseded owner %q, got %q", oldOwner, receipt.SupersededOwner)
	}
	if receipt.DriveID != id {
		t.Fatalf("receipt must carry the drive id %q, got %q", id, receipt.DriveID)
	}
	// The chain identity travels on the receipt.
	if receipt.ChangeID == "" || receipt.TaskID == "" || receipt.Phase == "" {
		t.Fatalf("receipt must carry the change/task/phase chain, got %+v", receipt)
	}

	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.OwnerGeneration != "" {
		t.Fatalf("handoff must invalidate the owner generation on disk, still %q", got.OwnerGeneration)
	}
	if got.HandoffGeneration != receipt.HandoffGeneration {
		t.Fatalf("handoff generation not persisted: got %q want %q", got.HandoffGeneration, receipt.HandoffGeneration)
	}
	// The old owner's authority is gone.
	if err := verifyOwner(&got, oldOwner); err == nil {
		t.Fatalf("old owner must fail verifyOwner after a handoff")
	} else if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrNotOwner {
		t.Fatalf("old-owner verify must be ErrNotOwner, got %v", err)
	}
}

// TestClaimConsumesReceiptForNewGeneration proves a fresh claimant whose
// fingerprint matches consumes the single-use receipt and receives a NEW owner
// generation distinct from both the old owner and the handoff token.
func TestClaimConsumesReceiptForNewGeneration(t *testing.T) {
	s, id, oldOwner, receipt := newHandedOffDrive(t)

	newOwner, err := s.consumeHandoffCAS(id, receipt.HandoffGeneration, matchingFP())
	if err != nil {
		t.Fatalf("consumeHandoffCAS: %v", err)
	}
	if newOwner == "" {
		t.Fatalf("claim must mint a non-empty new owner generation")
	}
	if newOwner == oldOwner {
		t.Fatalf("new owner generation must differ from the superseded owner")
	}
	if newOwner == receipt.HandoffGeneration {
		t.Fatalf("new owner generation must differ from the consumed handoff token")
	}

	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HandoffGeneration != "" {
		t.Fatalf("a consumed receipt must be cleared, still %q", got.HandoffGeneration)
	}
	if got.OwnerGeneration != newOwner {
		t.Fatalf("new owner not persisted: got %q want %q", got.OwnerGeneration, newOwner)
	}
	if err := verifyOwner(&got, newOwner); err != nil {
		t.Fatalf("fresh owner must verify after claim: %v", err)
	}
	if err := verifyOwner(&got, oldOwner); err == nil {
		t.Fatalf("old owner must still be rejected after claim")
	}

	// The receipt is single-use: a second claim finds nothing to consume.
	if _, err := s.consumeHandoffCAS(id, receipt.HandoffGeneration, matchingFP()); err == nil {
		t.Fatalf("a consumed receipt must not be claimable a second time")
	} else if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrNoHandoffOffered {
		t.Fatalf("second claim must be ErrNoHandoffOffered, got %v", err)
	}
}

// TestRaceOneReceiptSingleWinner races two claimants against one receipt and
// proves exactly one acquires a new generation; the loser acquires no partial
// authority. Run under -race.
func TestRaceOneReceiptSingleWinner(t *testing.T) {
	s, id, _, receipt := newHandedOffDrive(t)

	var wg sync.WaitGroup
	gens := make([]string, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			gens[idx], errs[idx] = s.consumeHandoffCAS(id, receipt.HandoffGeneration, matchingFP())
		}(i)
	}
	wg.Wait()

	winners := 0
	winnerGen := ""
	for i := 0; i < 2; i++ {
		if errs[i] == nil {
			winners++
			winnerGen = gens[i]
			continue
		}
		// The loser must fail with a receipt-gone ownership error and no token.
		oe, ok := AsOwnershipError(errs[i])
		if !ok || (oe.Kind != ErrNoHandoffOffered && oe.Kind != ErrHandoffMismatch) {
			t.Fatalf("loser must fail with a receipt-consumed ownership error, got %v", errs[i])
		}
		if gens[i] != "" {
			t.Fatalf("loser must acquire NO generation, got %q", gens[i])
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one claimant must win, got %d", winners)
	}

	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HandoffGeneration != "" {
		t.Fatalf("receipt must be consumed exactly once, handoff still %q", got.HandoffGeneration)
	}
	if got.OwnerGeneration != winnerGen {
		t.Fatalf("persisted owner %q must be the sole winner %q", got.OwnerGeneration, winnerGen)
	}
}

// TestPlainWaitingDriveCannotBeClaimed proves a drive with a live owner and no
// outstanding handoff cannot be claimed: seeing a running suite is not authority
// to take it over.
func TestPlainWaitingDriveCannotBeClaimed(t *testing.T) {
	s := OpenStore(t.TempDir())
	rec := sampleRecord()
	id, _, err := s.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}

	_, err = s.consumeHandoffCAS(id, "any-token", matchingFP())
	if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrNoHandoffOffered {
		t.Fatalf("claiming a plain WAITING drive must be ErrNoHandoffOffered, got %v", err)
	}
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.OwnerGeneration != rec.OwnerGeneration {
		t.Fatalf("a rejected claim must not disturb the owner, got %q", got.OwnerGeneration)
	}
}

// TestFingerprintMismatchAtClaimRejects proves a claimant whose fingerprint no
// longer matches the drive-start identity is rejected and — critically — does
// NOT consume the receipt, so a correct claimant can still claim afterward. A
// claim that no longer matches acquires no partial authority.
func TestFingerprintMismatchAtClaimRejects(t *testing.T) {
	s, id, _, receipt := newHandedOffDrive(t)

	drifted, err := s.consumeHandoffCAS(id, receipt.HandoffGeneration, mismatchFP())
	if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrFingerprintMismatch {
		t.Fatalf("a drifted claim must be ErrFingerprintMismatch, got %v", err)
	}
	if drifted != "" {
		t.Fatalf("a rejected claim must acquire no generation, got %q", drifted)
	}

	// The receipt survives the rejected claim.
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HandoffGeneration != receipt.HandoffGeneration {
		t.Fatalf("a fingerprint-rejected claim must preserve the receipt, got %q", got.HandoffGeneration)
	}
	if got.OwnerGeneration != "" {
		t.Fatalf("a fingerprint-rejected claim must not install an owner, got %q", got.OwnerGeneration)
	}

	// A matching claimant still succeeds against the preserved receipt.
	newOwner, err := s.consumeHandoffCAS(id, receipt.HandoffGeneration, matchingFP())
	if err != nil {
		t.Fatalf("matching claim after a rejected one: %v", err)
	}
	if newOwner == "" {
		t.Fatalf("matching claim must mint a new owner generation")
	}
}

// TestHandoffRejectsNonOwner proves only the current owner can offer a handoff;
// a stale or wrong generation acquires no authority to transfer the drive.
func TestHandoffRejectsNonOwner(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, _, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	if _, err := s.writeHandoffReceipt(id, "not-the-owner", matchingFP()); err == nil {
		t.Fatalf("a non-owner must not be able to hand off")
	} else if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrNotOwner {
		t.Fatalf("non-owner handoff must be ErrNotOwner, got %v", err)
	}
	// The drive is undisturbed: still owned, no outstanding handoff.
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.OwnerGeneration != sampleRecord().OwnerGeneration || got.HandoffGeneration != "" {
		t.Fatalf("a rejected handoff must not disturb the drive: %+v", got)
	}
}

// TestHandoffRejectsFingerprintDrift proves a handoff is an ownership boundary:
// if the owner's worktree has drifted from the drive-start identity, the handoff
// is refused rather than transferring a drive whose pass could no longer certify
// the original bytes.
func TestHandoffRejectsFingerprintDrift(t *testing.T) {
	s := OpenStore(t.TempDir())
	oldOwner := sampleRecord().OwnerGeneration
	id, _, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	if _, err := s.writeHandoffReceipt(id, oldOwner, mismatchFP()); err == nil {
		t.Fatalf("a drifted worktree must not be handed off")
	} else if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrFingerprintMismatch {
		t.Fatalf("drifted handoff must be ErrFingerprintMismatch, got %v", err)
	}
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.OwnerGeneration != oldOwner || got.HandoffGeneration != "" {
		t.Fatalf("a rejected handoff must leave the owner intact: %+v", got)
	}
}
