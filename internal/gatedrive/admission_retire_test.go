package gatedrive

import (
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// retireFixtureSlot reserves, confirms, and releases a slot owned by run epoch
// "ep-1", returning the slot's reservation token. It builds the owning-epoch
// released slot every retirement test starts from, reusing admission_test.go's
// OpenStore/mkWorktree fixture pattern rather than inventing a second style.
func retireFixtureSlot(t *testing.T, s *Store, worktree string) (token string) {
	t.Helper()
	token, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: worktree, Kind: "scopeless", RunEpochID: "ep-1",
	}, nil)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ConfirmWorktreeExecution(worktree, token, "run-1", filepath.Join(worktree, "rd")); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(worktree, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	return token
}

// TestRetireClearsOnlyRunEpochID: retiring the owning epoch on a released slot
// clears RunEpochID, keeps the state released, and preserves every historical
// field.
func TestRetireClearsOnlyRunEpochID(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)

	before, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if before.RunEpochID != "ep-1" {
		t.Fatalf("fixture RunEpochID = %q, want ep-1", before.RunEpochID)
	}

	if err := s.RetireWorktreeExecutionEpoch(wt, "ep-1", token); err != nil {
		t.Fatalf("retire: %v", err)
	}

	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunEpochID != "" {
		t.Fatalf("RunEpochID = %q, want cleared", after.RunEpochID)
	}
	if after.State != admissionReleased {
		t.Fatalf("state = %q, want released", after.State)
	}
	if after.DriveID != before.DriveID ||
		after.RawRunID != before.RawRunID ||
		after.RawRunDir != before.RawRunDir ||
		after.ExecutionGen != before.ExecutionGen ||
		after.ScopeID != before.ScopeID ||
		after.Kind != before.Kind ||
		after.ReservationToken != before.ReservationToken ||
		!after.ReservedAt.Equal(before.ReservedAt) ||
		after.LegacyInventoried != before.LegacyInventoried {
		t.Fatalf("retirement must preserve every historical field: before=%+v after=%+v", before, after)
	}
	if after.UpdatedAt.Before(before.UpdatedAt) {
		t.Fatalf("UpdatedAt went backwards: before=%v after=%v", before.UpdatedAt, after.UpdatedAt)
	}
}

// TestRetireIdempotentWhenAlreadyDetached: a second retire (RunEpochID already
// "") returns nil and writes nothing — the stored physical generation is stable
// across the no-op.
func TestRetireIdempotentWhenAlreadyDetached(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)

	if err := s.RetireWorktreeExecutionEpoch(wt, "ep-1", token); err != nil {
		t.Fatalf("first retire: %v", err)
	}
	_, gen1, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after first retire: %v", err)
	}

	if err := s.RetireWorktreeExecutionEpoch(wt, "ep-1", token); err != nil {
		t.Fatalf("second retire: %v", err)
	}
	_, gen2, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after second retire: %v", err)
	}
	if gen1 != gen2 {
		t.Fatalf("idempotent retire wrote (generation %q -> %q), want no write", gen1, gen2)
	}
}

// TestRetireRefusesForeignEpoch: an expectEpoch that does not own the slot is
// ErrStaleRunEpoch, and the record is untouched (same physical generation).
func TestRetireRefusesForeignEpoch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)
	_, genBefore, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}

	err = s.RetireWorktreeExecutionEpoch(wt, "ep-2", token)
	if !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("want ErrStaleRunEpoch, got %v", err)
	}

	after, genAfter, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if genBefore != genAfter {
		t.Fatalf("refused retire wrote (generation %q -> %q), want untouched", genBefore, genAfter)
	}
	if after.RunEpochID != "ep-1" {
		t.Fatalf("RunEpochID = %q, want retained ep-1", after.RunEpochID)
	}
}

// TestRetireRefusesTokenMismatch: the right epoch with a stale/changed token is
// ErrNotOwner — a raced replacement's reservation must never be cleared blindly.
func TestRetireRefusesTokenMismatch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	_ = retireFixtureSlot(t, s, wt)
	_, genBefore, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}

	err = s.RetireWorktreeExecutionEpoch(wt, "ep-1", "not-the-token")
	if !isOwnership(err, ErrNotOwner) {
		t.Fatalf("want ErrNotOwner, got %v", err)
	}
	if err := s.RetireWorktreeExecutionEpoch(wt, "ep-1", ""); !isOwnership(err, ErrNotOwner) {
		t.Fatalf("empty token: want ErrNotOwner, got %v", err)
	}

	after, genAfter, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if genBefore != genAfter {
		t.Fatalf("refused retire wrote (generation %q -> %q), want untouched", genBefore, genAfter)
	}
	if after.RunEpochID != "ep-1" {
		t.Fatalf("RunEpochID = %q, want retained ep-1", after.RunEpochID)
	}
}

// TestRetireRefusesNonReleasedState: an executing (confirmed, not released) owned
// slot refuses ErrWorktreeBusy — retirement requires released state.
func TestRetireRefusesNonReleasedState(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunEpochID: "ep-1",
	}, nil)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ConfirmWorktreeExecution(wt, token, "run-1", filepath.Join(wt, "rd")); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	err = s.RetireWorktreeExecutionEpoch(wt, "ep-1", token)
	if !isOwnership(err, ErrWorktreeBusy) {
		t.Fatalf("want ErrWorktreeBusy, got %v", err)
	}
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunEpochID != "ep-1" || after.State != admissionExecuting {
		t.Fatalf("executing owned slot must be untouched: RunEpochID=%q state=%q", after.RunEpochID, after.State)
	}
}

// TestRetireEmptyExpectEpochRefused: expectEpoch "" is ErrInvalidID — an empty
// epoch is never ownership proof.
func TestRetireEmptyExpectEpochRefused(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)

	err := s.RetireWorktreeExecutionEpoch(wt, "", token)
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrInvalidID {
		t.Fatalf("want ErrInvalidID, got %v", err)
	}
	// The empty-epoch guard fails before any store touch: the slot still owns ep-1.
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunEpochID != "ep-1" {
		t.Fatalf("RunEpochID = %q, want retained ep-1", after.RunEpochID)
	}
}

// TestRetireAbsentSlotIsNotFound: no slot record → typed ErrNotFound (the caller
// maps absence to an idempotent no-op only after independent accounting).
func TestRetireAbsentSlotIsNotFound(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	err := s.RetireWorktreeExecutionEpoch(wt, "ep-1", "any-token")
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestOrdinaryReleaseStillRetainsEpoch (AC2 regression): ordinary
// ReleaseWorktreeExecution keeps RunEpochID "ep-1", and both a different-epoch
// and an epoch-less reserve over that retained slot are fenced ErrStaleRunEpoch —
// the between-drives fence is untouched by this change.
func TestOrdinaryReleaseStillRetainsEpoch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	_ = retireFixtureSlot(t, s, wt)

	slot, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("state = %q, want released", slot.State)
	}
	if slot.RunEpochID != "ep-1" {
		t.Fatalf("released slot RunEpochID = %q, want retained ep-1", slot.RunEpochID)
	}

	if _, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunEpochID: "ep-2",
	}, nil); !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("different-epoch reserve over retained slot: want ErrStaleRunEpoch, got %v", err)
	}
	if _, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunEpochID: "",
	}, nil); !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("epoch-less reserve over retained slot: want ErrStaleRunEpoch, got %v", err)
	}
}

// TestAdmissionAfterRetirement (AC1, store half): after retirement a released
// slot is genuinely reusable — (a) a reserve carrying a NEW epoch admits, and (b)
// on a second retired fixture an epoch-less reserve admits — with ExecutionGen
// continuing monotonically.
func TestAdmissionAfterRetirement(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))

	// (a) new-epoch reserve after retirement.
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)
	before, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if err := s.RetireWorktreeExecutionEpoch(wt, "ep-1", token); err != nil {
		t.Fatalf("retire: %v", err)
	}
	newTok, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunEpochID: "ep-2",
	}, nil)
	if err != nil {
		t.Fatalf("new-epoch reserve after retirement: %v", err)
	}
	if newTok == "" {
		t.Fatalf("new-epoch reserve returned an empty token")
	}
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunEpochID != "ep-2" {
		t.Fatalf("readmitted slot RunEpochID = %q, want ep-2", after.RunEpochID)
	}
	if after.ExecutionGen <= before.ExecutionGen {
		t.Fatalf("ExecutionGen must continue monotonically: before=%d after=%d", before.ExecutionGen, after.ExecutionGen)
	}

	// (b) epoch-less reserve after retirement, on a distinct fixture.
	wt2 := mkWorktree(t)
	token2 := retireFixtureSlot(t, s, wt2)
	if err := s.RetireWorktreeExecutionEpoch(wt2, "ep-1", token2); err != nil {
		t.Fatalf("retire wt2: %v", err)
	}
	if _, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt2, Kind: "scopeless", RunEpochID: "",
	}, nil); err != nil {
		t.Fatalf("epoch-less reserve after retirement: %v", err)
	}
}

// TestReserveWorktreeExecutionForEpochRecordsOwnership: the exported epoch-carrying
// reserve records the owning RunEpochID (so the app boundary can create an
// epoch-owned slot without the unexported admissionRecord literal), and refuses an
// empty epoch with a typed ErrInvalidID.
func TestReserveWorktreeExecutionForEpochRecordsOwnership(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))

	wt := mkWorktree(t)
	token, err := s.ReserveWorktreeExecutionForEpoch("repo-1", wt, "ep-1", nil)
	if err != nil {
		t.Fatalf("ReserveWorktreeExecutionForEpoch: %v", err)
	}
	if token == "" {
		t.Fatalf("reserve returned an empty token")
	}
	slot, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if slot.RunEpochID != "ep-1" {
		t.Fatalf("RunEpochID = %q, want ep-1", slot.RunEpochID)
	}
	if slot.State != admissionReserved {
		t.Fatalf("state = %q, want reserved", slot.State)
	}
	if slot.Kind != "scopeless" {
		t.Fatalf("Kind = %q, want scopeless", slot.Kind)
	}

	wt2 := mkWorktree(t)
	_, err = s.ReserveWorktreeExecutionForEpoch("repo-1", wt2, "", nil)
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrInvalidID {
		t.Fatalf("empty epoch: want ErrInvalidID, got %v", err)
	}
	if _, _, lerr := s.LoadWorktreeExecution(wt2); lerr == nil {
		t.Fatalf("empty-epoch reserve must not create a slot")
	}
}
