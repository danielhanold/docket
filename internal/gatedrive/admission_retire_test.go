package gatedrive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// retireFixtureSlot reserves, confirms, and releases a slot owned by run
// "ep-1", returning the slot's reservation token. It builds the owning-run
// released slot every retirement test starts from, reusing admission_test.go's
// OpenStore/mkWorktree fixture pattern rather than inventing a second style.
func retireFixtureSlot(t *testing.T, s *Store, worktree string) (token string) {
	t.Helper()
	token, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: worktree, Kind: "scopeless", RunID: "ep-1",
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

// TestRetireClearsOnlyRunID: retiring the owning run on a released slot
// clears RunID, keeps the state released, and preserves every historical
// field.
func TestRetireClearsOnlyRunID(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)

	before, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if before.RunID != "ep-1" {
		t.Fatalf("fixture RunID = %q, want ep-1", before.RunID)
	}

	if err := s.RetireWorktreeExecutionRun(wt, "ep-1", token); err != nil {
		t.Fatalf("retire: %v", err)
	}

	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunID != "" {
		t.Fatalf("RunID = %q, want cleared", after.RunID)
	}
	if after.State != admissionReleased {
		t.Fatalf("state = %q, want released", after.State)
	}
	if after.DriveID != before.DriveID ||
		after.RawRunID != before.RawRunID ||
		after.RawRunDir != before.RawRunDir ||
		after.ExecutionGen != before.ExecutionGen ||
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

// TestRetireIdempotentWhenAlreadyDetached: a second retire (RunID already
// "") returns nil and writes nothing — the stored physical generation is stable
// across the no-op.
func TestRetireIdempotentWhenAlreadyDetached(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)

	if err := s.RetireWorktreeExecutionRun(wt, "ep-1", token); err != nil {
		t.Fatalf("first retire: %v", err)
	}
	_, gen1, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after first retire: %v", err)
	}

	if err := s.RetireWorktreeExecutionRun(wt, "ep-1", token); err != nil {
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

// TestRetireRefusesForeignRun: an expectRunID that does not own the slot is
// ErrStaleRunID, and the record is untouched (same physical generation).
func TestRetireRefusesForeignRun(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)
	_, genBefore, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}

	err = s.RetireWorktreeExecutionRun(wt, "ep-2", token)
	if !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("want ErrStaleRunID, got %v", err)
	}

	after, genAfter, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if genBefore != genAfter {
		t.Fatalf("refused retire wrote (generation %q -> %q), want untouched", genBefore, genAfter)
	}
	if after.RunID != "ep-1" {
		t.Fatalf("RunID = %q, want retained ep-1", after.RunID)
	}
}

// TestRetireRefusesTokenMismatch: the right run with a stale/changed token is
// ErrNotOwner — a raced replacement's reservation must never be cleared blindly.
func TestRetireRefusesTokenMismatch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	_ = retireFixtureSlot(t, s, wt)
	_, genBefore, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}

	err = s.RetireWorktreeExecutionRun(wt, "ep-1", "not-the-token")
	if !isOwnership(err, ErrNotOwner) {
		t.Fatalf("want ErrNotOwner, got %v", err)
	}
	if err := s.RetireWorktreeExecutionRun(wt, "ep-1", ""); !isOwnership(err, ErrNotOwner) {
		t.Fatalf("empty token: want ErrNotOwner, got %v", err)
	}

	after, genAfter, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if genBefore != genAfter {
		t.Fatalf("refused retire wrote (generation %q -> %q), want untouched", genBefore, genAfter)
	}
	if after.RunID != "ep-1" {
		t.Fatalf("RunID = %q, want retained ep-1", after.RunID)
	}
}

// TestRetireRefusesNonReleasedState: an executing (confirmed, not released) owned
// slot refuses ErrWorktreeBusy — retirement requires released state.
func TestRetireRefusesNonReleasedState(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunID: "ep-1",
	}, nil)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ConfirmWorktreeExecution(wt, token, "run-1", filepath.Join(wt, "rd")); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	err = s.RetireWorktreeExecutionRun(wt, "ep-1", token)
	if !isOwnership(err, ErrWorktreeBusy) {
		t.Fatalf("want ErrWorktreeBusy, got %v", err)
	}
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunID != "ep-1" || after.State != admissionExecuting {
		t.Fatalf("executing owned slot must be untouched: RunID=%q state=%q", after.RunID, after.State)
	}
}

// TestRetireEmptyExpectRunRefused: expectRunID "" is ErrInvalidID — an empty
// run is never ownership proof.
func TestRetireEmptyExpectRunRefused(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)

	err := s.RetireWorktreeExecutionRun(wt, "", token)
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrInvalidID {
		t.Fatalf("want ErrInvalidID, got %v", err)
	}
	// The empty-run guard fails before any store touch: the slot still owns ep-1.
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunID != "ep-1" {
		t.Fatalf("RunID = %q, want retained ep-1", after.RunID)
	}
}

// TestRetireAbsentSlotIsNotFound: no slot record → typed ErrNotFound (the caller
// maps absence to an idempotent no-op only after independent accounting).
func TestRetireAbsentSlotIsNotFound(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	err := s.RetireWorktreeExecutionRun(wt, "ep-1", "any-token")
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestOrdinaryReleaseStillRetainsRun (AC2 regression): ordinary
// ReleaseWorktreeExecution keeps RunID "ep-1", and both a different-run
// and a no-run-record reserve over that retained slot are fenced ErrStaleRunID —
// the between-drives fence is untouched by this change.
func TestOrdinaryReleaseStillRetainsRun(t *testing.T) {
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
	if slot.RunID != "ep-1" {
		t.Fatalf("released slot RunID = %q, want retained ep-1", slot.RunID)
	}

	if _, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunID: "ep-2",
	}, nil); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("different-run reserve over retained slot: want ErrStaleRunID, got %v", err)
	}
	if _, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunID: "",
	}, nil); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("no-run-record reserve over retained slot: want ErrStaleRunID, got %v", err)
	}
}

// TestAdmissionAfterRetirement (AC1, store half): after retirement a released
// slot is genuinely reusable — (a) a reserve carrying a NEW run admits, and (b)
// on a second retired fixture a no-run-record reserve admits — with ExecutionGen
// continuing monotonically.
func TestAdmissionAfterRetirement(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))

	// (a) new-run reserve after retirement.
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)
	before, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if err := s.RetireWorktreeExecutionRun(wt, "ep-1", token); err != nil {
		t.Fatalf("retire: %v", err)
	}
	newTok, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt, Kind: "scopeless", RunID: "ep-2",
	}, nil)
	if err != nil {
		t.Fatalf("new-run reserve after retirement: %v", err)
	}
	if newTok == "" {
		t.Fatalf("new-run reserve returned an empty token")
	}
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunID != "ep-2" {
		t.Fatalf("readmitted slot RunID = %q, want ep-2", after.RunID)
	}
	if after.ExecutionGen <= before.ExecutionGen {
		t.Fatalf("ExecutionGen must continue monotonically: before=%d after=%d", before.ExecutionGen, after.ExecutionGen)
	}

	// (b) no-run-record reserve after retirement, on a distinct fixture.
	wt2 := mkWorktree(t)
	token2 := retireFixtureSlot(t, s, wt2)
	if err := s.RetireWorktreeExecutionRun(wt2, "ep-1", token2); err != nil {
		t.Fatalf("retire wt2: %v", err)
	}
	if _, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: wt2, Kind: "scopeless", RunID: "",
	}, nil); err != nil {
		t.Fatalf("no-run-record reserve after retirement: %v", err)
	}
}

// TestReserveWorktreeExecutionForRunRecordsOwnership: the exported run-carrying
// reserve records the owning RunID (so the app boundary can create an
// run-owned slot without the unexported admissionRecord literal), and refuses an
// empty run with a typed ErrInvalidID.
func TestReserveWorktreeExecutionForRunRecordsOwnership(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))

	wt := mkWorktree(t)
	token, err := s.ReserveWorktreeExecutionForRun("repo-1", wt, "ep-1", nil)
	if err != nil {
		t.Fatalf("ReserveWorktreeExecutionForRun: %v", err)
	}
	if token == "" {
		t.Fatalf("reserve returned an empty token")
	}
	slot, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if slot.RunID != "ep-1" {
		t.Fatalf("RunID = %q, want ep-1", slot.RunID)
	}
	if slot.State != admissionReserved {
		t.Fatalf("state = %q, want reserved", slot.State)
	}
	if slot.Kind != "scopeless" {
		t.Fatalf("Kind = %q, want scopeless", slot.Kind)
	}

	wt2 := mkWorktree(t)
	_, err = s.ReserveWorktreeExecutionForRun("repo-1", wt2, "", nil)
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrInvalidID {
		t.Fatalf("empty run: want ErrInvalidID, got %v", err)
	}
	if _, _, lerr := s.LoadWorktreeExecution(wt2); lerr == nil {
		t.Fatalf("empty-run reserve must not create a slot")
	}
}

// removedWorktreeSlot builds the removed-worktree fixture every stored-identity
// addressing test (change 0446 Task 2) starts from: it reserves a slot for a real
// worktree through the exported run entry point, releases it with its token,
// reads the slot's STORED canonical identity (WorktreeRoot — EvalSymlinks output
// by construction), then removes the worktree directory. It returns the logical
// spelling the caller created, the stored identity, and the reservation token.
func removedWorktreeSlot(t *testing.T, s *Store) (logical, stored, token string) {
	t.Helper()
	logical = mkWorktree(t)
	token, err := s.ReserveWorktreeExecutionForRun("repo-1", logical, "ep-1", nil)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(logical, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	slot, _, err := s.LoadWorktreeExecution(logical)
	if err != nil {
		t.Fatalf("load before removal: %v", err)
	}
	stored = slot.WorktreeRoot
	if err := os.RemoveAll(logical); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}
	if _, err := os.Lstat(stored); !os.IsNotExist(err) {
		t.Fatalf("fixture: stored identity %q still exists after removal (err=%v)", stored, err)
	}
	return logical, stored, token
}

// TestRetireRunAfterWorktreeRemoved: cancellation completion of a run whose
// worktree was removed addresses the slot through the canonical identity already
// stored on it rather than re-canonicalizing a path that no longer exists (spec
// §2) — retirement finds the slot and clears RunID, preserving the rest.
func TestRetireRunAfterWorktreeRemoved(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	_, stored, token := removedWorktreeSlot(t, s)

	if err := s.RetireWorktreeExecutionRun(stored, "ep-1", token); err != nil {
		t.Fatalf("retire after worktree removal: %v", err)
	}
	after, _, err := s.LoadWorktreeExecution(stored)
	if err != nil {
		t.Fatalf("load after retire: %v", err)
	}
	if after.RunID != "" {
		t.Fatalf("RunID = %q, want cleared", after.RunID)
	}
	if after.State != admissionReleased || after.ReservationToken != token {
		t.Fatalf("retire must only detach the run: got %+v", after)
	}
}

// TestLoadWorktreeExecutionAfterRemovalFindsSlot: after the worktree directory is
// removed, LoadWorktreeExecution(storedRoot) returns the slot record, not
// ErrInvalidID. The logical spelling the run may have bound (bindRunWorktree
// stores the LOGICAL path, e.g. under /var/folders → /private/var on macOS) keys to
// the same slot: its surviving ancestors still resolve, so the missing tail is
// appended to their canonical form.
func TestLoadWorktreeExecutionAfterRemovalFindsSlot(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	logical, stored, token := removedWorktreeSlot(t, s)

	for _, spelling := range []string{stored, logical} {
		slot, _, err := s.LoadWorktreeExecution(spelling)
		if err != nil {
			t.Fatalf("load %q after removal: %v", spelling, err)
		}
		if slot.ReservationToken != token || slot.RunID != "ep-1" || slot.WorktreeRoot != stored {
			t.Fatalf("load %q returned a different slot: %+v", spelling, slot)
		}
	}
}

// TestReserveAfterWorktreeRemovedStaysStrict: admitting a NEW execution still
// requires an existing, resolvable worktree — only read/CAS entry points that
// receive stored identities tolerate a missing path.
func TestReserveAfterWorktreeRemovedStaysStrict(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	_, stored, _ := removedWorktreeSlot(t, s)

	_, err := s.ReserveWorktreeExecutionForRun("repo-1", stored, "ep-1", nil)
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrInvalidID {
		t.Fatalf("reserve on a removed worktree: want ErrInvalidID, got %v", err)
	}
}

// TestLoadWorktreeExecutionProbeErrorIsNotAbsence: a canonicalization failure
// other than a missing path (here ENOTDIR — a path component is a regular file) is
// a typed ErrInvalidID, never keyed on the spelling: a probe error is not clean
// absence.
func TestLoadWorktreeExecutionProbeErrorIsNotAbsence(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	file := filepath.Join(testsupport.TempDir(t), "plain-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, _, err := s.LoadWorktreeExecution(filepath.Join(file, "wt"))
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrInvalidID {
		t.Fatalf("ENOTDIR probe: want ErrInvalidID, got %v", err)
	}
}

// TestLoadWorktreeExecutionSymlinkAliasStillResolves: a live symlink alias of the
// worktree still resolves to the same slot (canonicalise every symlink hop) — the
// stored-identity path changes nothing for a path that exists.
func TestLoadWorktreeExecutionSymlinkAliasStillResolves(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token := retireFixtureSlot(t, s, wt)
	alias := filepath.Join(testsupport.TempDir(t), "alias")
	if err := os.Symlink(wt, alias); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	slot, _, err := s.LoadWorktreeExecution(alias)
	if err != nil {
		t.Fatalf("load via alias: %v", err)
	}
	if slot.ReservationToken != token {
		t.Fatalf("alias resolved to a different slot: %+v", slot)
	}
	if err := s.RetireWorktreeExecutionRun(alias, "ep-1", token); err != nil {
		t.Fatalf("retire via alias: %v", err)
	}
}
