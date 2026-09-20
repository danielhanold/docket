package gatedrive

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// sampleAdmission builds an admissionRecord for worktreeRoot carrying a value in
// every caller-supplied field group, so a reserve proves the slot persists the
// whole request. The store overwrites SchemaVersion/State/ExecutionGen/
// ReservationToken/RawRunID/RawRunDir/WorktreeRoot itself.
func sampleAdmission(worktreeRoot string) admissionRecord {
	return admissionRecord{
		RepoIdentity: "repo-x",
		WorktreeRoot: worktreeRoot,
		DriveID:      "drive-1",
		ScopeID:      "scope-1",
		RunEpochID:   "",
		Kind:         "scoped",
	}
}

// mkWorktree creates a real directory to stand in for a canonical worktree root,
// so filepath.EvalSymlinks (the store's canonicalisation) resolves it.
func mkWorktree(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(testsupport.TempDir(t), "wt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkWorktree: %v", err)
	}
	return dir
}

// admissionRecordPath returns the on-disk record path the store keys for
// worktreeRoot, so a fault test can seed a raw or malformed document there.
func admissionRecordPath(t *testing.T, s *Store, worktreeRoot string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(worktreeRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	key := admissionKey(canonical)
	if key == "" {
		t.Fatalf("admissionKey returned empty for %q", canonical)
	}
	dir := filepath.Join(s.admissionRoot, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	return filepath.Join(dir, recordFileName)
}

// TestAdmissionReserveThenBusy proves the slot admits the first reserve of any
// Kind and refuses a second with a typed ErrWorktreeBusy that leaks no token.
func TestAdmissionReserveThenBusy(t *testing.T) {
	for _, kind := range []string{"scoped", "scopeless", "raw"} {
		t.Run(kind, func(t *testing.T) {
			s := OpenStore(testsupport.TempDir(t))
			wt := mkWorktree(t)
			rec := sampleAdmission(wt)
			rec.Kind = kind

			token, err := s.ReserveWorktreeExecution(rec)
			if err != nil {
				t.Fatalf("first reserve: %v", err)
			}
			if token == "" {
				t.Fatalf("reserve returned an empty token")
			}

			_, err = s.ReserveWorktreeExecution(rec)
			oe, ok := AsOwnershipError(err)
			if !ok || oe.Kind != ErrWorktreeBusy {
				t.Fatalf("second reserve: want ErrWorktreeBusy, got %v", err)
			}
			if oe.Op == "" {
				t.Fatalf("busy error carries no op detail: %+v", oe)
			}
			if strings.Contains(err.Error(), token) {
				t.Fatalf("busy error leaks the incumbent token %q: %q", token, err.Error())
			}
		})
	}
}

// TestAdmissionReleaseRequiresToken proves a release verifies the reservation
// token: a wrong or empty token is refused and the slot stays occupied, while
// the correct token releases it.
func TestAdmissionReleaseRequiresToken(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token, err := s.ReserveWorktreeExecution(sampleAdmission(wt))
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	if err := s.ReleaseWorktreeExecution(wt, "not-the-token"); err == nil {
		t.Fatalf("release with a wrong token must be refused")
	}
	if err := s.ReleaseWorktreeExecution(wt, ""); err == nil {
		t.Fatalf("release with an empty token must be refused")
	}
	// A wrong-token release changed nothing: the slot is still busy.
	if _, err := s.ReserveWorktreeExecution(sampleAdmission(wt)); !isOwnership(err, ErrWorktreeBusy) {
		t.Fatalf("slot must remain busy after a refused release, got %v", err)
	}

	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release with the correct token: %v", err)
	}
}

// TestAdmissionUnknownSchemaFailsClosed proves an unknown persisted schema
// version fails a reserve closed rather than being read as a free slot.
func TestAdmissionUnknownSchemaFailsClosed(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	path := admissionRecordPath(t, s, wt)
	if err := os.WriteFile(path, []byte(`{"Generation":"x","Record":{"SchemaVersion":99}}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := s.ReserveWorktreeExecution(sampleAdmission(wt))
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrUnknownSchema {
		t.Fatalf("want ErrUnknownSchema, got %v", err)
	}
}

// TestAdmissionCorruptRecordFailsClosed proves an undecodable persisted record
// fails a reserve closed rather than being read as a free slot.
func TestAdmissionCorruptRecordFailsClosed(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	path := admissionRecordPath(t, s, wt)
	if err := os.WriteFile(path, []byte(`{not json`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := s.ReserveWorktreeExecution(sampleAdmission(wt))
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrCorruptRecord {
		t.Fatalf("want ErrCorruptRecord, got %v", err)
	}
}

// TestAdmissionReleasedSlotReadmits proves a released slot is admittable again
// and the readmission monotonically bumps the execution generation.
func TestAdmissionReleasedSlotReadmits(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	token, err := s.ReserveWorktreeExecution(sampleAdmission(wt))
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	first, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after first reserve: %v", err)
	}
	if first.State != admissionReserved {
		t.Fatalf("state after reserve = %q, want %q", first.State, admissionReserved)
	}

	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	released, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after release: %v", err)
	}
	if released.State != admissionReleased {
		t.Fatalf("state after release = %q, want %q", released.State, admissionReleased)
	}
	// Releasing preserves the historical facts (spec: released records remain
	// historical evidence).
	if released.DriveID != first.DriveID {
		t.Fatalf("release dropped DriveID: got %q want %q", released.DriveID, first.DriveID)
	}

	if _, err := s.ReserveWorktreeExecution(sampleAdmission(wt)); err != nil {
		t.Fatalf("reserve over a released slot: %v", err)
	}
	second, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after readmit: %v", err)
	}
	if second.ExecutionGen <= first.ExecutionGen {
		t.Fatalf("ExecutionGen must increase on readmit: first=%d second=%d", first.ExecutionGen, second.ExecutionGen)
	}
}

// TestAdmissionSymlinkAliasSameSlot proves two spellings of one worktree resolve
// to a single slot, so a path alias cannot buy a second admission, and that the
// key helper refuses non-canonical (non-absolute) input.
func TestAdmissionSymlinkAliasSameSlot(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	// testsupport.TempDir roots under $TMPDIR (/var/folders on macOS, itself a
	// symlink to /private/var), so EvalSymlinks yields a second name for one dir.
	twin, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	if _, err := s.ReserveWorktreeExecution(sampleAdmission(wt)); err != nil {
		t.Fatalf("reserve via first spelling: %v", err)
	}
	// The alias must land on the same occupied slot, not a second one.
	if _, err := s.ReserveWorktreeExecution(sampleAdmission(twin)); !isOwnership(err, ErrWorktreeBusy) {
		t.Fatalf("reserve via the symlink twin must be ErrWorktreeBusy (same slot), got %v", err)
	}

	// The key helper is only ever fed canonical (absolute) roots: a relative
	// spelling yields no key, so a non-canonical caller cannot mint a slot.
	if k := admissionKey("relative/worktree"); k != "" {
		t.Fatalf("admissionKey accepted a non-absolute root, got %q", k)
	}
	if k := admissionKey(twin); k == "" {
		t.Fatalf("admissionKey rejected a canonical absolute root %q", twin)
	}
}

// TestAdmissionConcurrentReserveOneWinner proves that under a barrier, exactly
// one of two racing reserves wins and the loser is refused ErrWorktreeBusy —
// the one-winner concurrency guarantee.
func TestAdmissionConcurrentReserveOneWinner(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	rec := sampleAdmission(wt)

	var barrier sync.WaitGroup
	barrier.Add(2)
	var wg sync.WaitGroup
	wg.Add(2)

	tokens := make([]string, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			barrier.Done()
			barrier.Wait() // rendezvous so both contend on the slot at once
			tokens[i], errs[i] = s.ReserveWorktreeExecution(rec)
		}(i)
	}
	wg.Wait()

	winners, losers := 0, 0
	for i := 0; i < 2; i++ {
		switch {
		case errs[i] == nil && tokens[i] != "":
			winners++
		case isOwnership(errs[i], ErrWorktreeBusy):
			losers++
		default:
			t.Fatalf("goroutine %d: unexpected token=%q err=%v", i, tokens[i], errs[i])
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("want exactly one winner and one busy loser, got winners=%d losers=%d", winners, losers)
	}
}

// TestFirstAdmissionInventoriesLegacyWaitingDrive proves an old drive record is
// still admission evidence on the first use of the worktree slot. A WAITING
// record cannot be silently bypassed merely because it predates admission.
func TestFirstAdmissionInventoriesLegacyWaitingDrive(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	legacy := seedRecord(t)
	legacy.WorktreePath = wt
	legacy.LastOutcome = WAITING
	legacy.RawRunDir = "/runs/legacy-waiting"
	id, _, err := s.NewDrive(legacy)
	if err != nil {
		t.Fatalf("seed legacy drive: %v", err)
	}

	_, err = s.ReserveWorktreeExecution(sampleAdmission(wt))
	if !isOwnership(err, ErrUnresolvedExecution) || !strings.Contains(err.Error(), id) {
		t.Fatalf("first admission must refuse the legacy waiting drive %s, got %v", id, err)
	}
}

// TestFirstAdmissionInventoriesLegacyTerminalProvenDead proves a terminal
// HALTED legacy drive is admissible only when an observation proves its raw
// execution has gone away.
func TestFirstAdmissionInventoriesLegacyTerminalProvenDead(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	seam := &fakeRecovery{entries: map[string]process.RecoveryEntry{
		"/runs/legacy-terminal": {Disposition: "terminal"},
	}}
	legacy := seedRecord(t)
	legacy.WorktreePath = wt
	legacy.LastOutcome = HALTED
	legacy.RawRunDir = "/runs/legacy-terminal"
	if _, _, err := s.NewDrive(legacy); err != nil {
		t.Fatalf("seed legacy terminal drive: %v", err)
	}

	if _, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam); err != nil {
		t.Fatalf("first admission must admit a proven-dead legacy terminal drive: %v", err)
	}
}

// TestLegacyUnreadableRecordBlocks proves the first inventory never skips a
// malformed historical record: unreadable history is uncertainty, not a free
// slot.
func TestLegacyUnreadableRecordBlocks(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	const corruptID = "0123456789abcdef0123456789abcdef"
	legacyDir := filepath.Join(s.root, corruptID)
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("make corrupt legacy directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, recordFileName), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt legacy record: %v", err)
	}

	_, err := s.ReserveWorktreeExecution(sampleAdmission(wt))
	if !isOwnership(err, ErrUnresolvedExecution) || !strings.Contains(err.Error(), corruptID) {
		t.Fatalf("corrupt legacy record must block admission with its locator, got %v", err)
	}
}

// TestLegacyRecordlessDirDoesNotBlock proves the first inventory tolerates a drive
// directory that carries no record file yet: writeNewDrive creates the directory
// before it atomically writes the record, so a concurrent FIRST admission for a
// DIFFERENT worktree can observe an in-flight (or crashed-mid-creation) directory
// during its global census. Such a directory has no worktree binding and has
// launched no process, so it must never fail an unrelated worktree's admission
// closed — the exact spurious worktree-busy/unresolved refusal that broke two
// concurrent gates on distinct worktrees of one repo. Contrast
// TestLegacyUnreadableRecordBlocks: a PRESENT but corrupt record still blocks.
func TestLegacyRecordlessDirDoesNotBlock(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	const recordlessID = "fedcba9876543210fedcba9876543210"
	if err := os.MkdirAll(filepath.Join(s.root, recordlessID), 0o700); err != nil {
		t.Fatalf("make record-less legacy directory: %v", err)
	}

	if _, err := s.ReserveWorktreeExecution(sampleAdmission(wt)); err != nil {
		t.Fatalf("first admission must skip a record-less in-flight directory, got %v", err)
	}
}

// isOwnership reports whether err is an OwnershipError of the given kind.
func isOwnership(err error, kind OwnershipErrorKind) bool {
	oe, ok := AsOwnershipError(err)
	return ok && oe.Kind == kind
}

// newAdmissionFixture opens a store over a temp git common dir and mints a real
// canonical worktree dir, returning the store, worktree root, and a repo
// identity — the setup the reserve tests above repeat, extracted so the
// snapshot tests do not restate it three more times.
func newAdmissionFixture(t *testing.T) (store *Store, worktree, repoID string) {
	t.Helper()
	return OpenStore(testsupport.TempDir(t)), mkWorktree(t), "repo-x"
}

// TestReserveRefusalCarriesIncumbentSnapshot proves a worktree-busy refusal
// carries the incumbent's credential-free projection, captured from the exact
// record the refusal was decided on: kind, state, run identity — and never the
// reservation token.
func TestReserveRefusalCarriesIncumbentSnapshot(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	token, err := store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if err := store.ConfirmWorktreeExecution(worktree, token, "0123456789abcdef0123456789abcdef", "/runs/0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	_, err = store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrWorktreeBusy {
		t.Fatalf("second reserve err = %v, want worktree-busy ownership error", err)
	}
	inc := oe.Incumbent
	if inc == nil {
		t.Fatal("worktree-busy refusal carries no incumbent snapshot")
	}
	if inc.Kind != "raw" || inc.State != "executing" {
		t.Fatalf("snapshot kind/state = %q/%q, want raw/executing", inc.Kind, inc.State)
	}
	if inc.RawRunID != "0123456789abcdef0123456789abcdef" || inc.RawRunDir != "/runs/0123456789abcdef0123456789abcdef" {
		t.Fatalf("snapshot run identity = %q %q", inc.RawRunID, inc.RawRunDir)
	}
	if inc.DriveID != "" || inc.EpochOwned {
		t.Fatalf("raw snapshot leaked drive/epoch facts: %+v", inc)
	}
}

// TestReserveUnresolvedRefusalCarriesSnapshot proves the unresolved-execution
// refusal also snapshots the incumbent, and TestReserveStaleEpochCarriesSnapshot
// proves the stale-run-epoch fence does (EpochOwned true, epoch id NOT projected).
func TestReserveUnresolvedRefusalCarriesSnapshot(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	token, err := store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := store.MarkWorktreeExecutionUnresolved(worktree, token); err != nil {
		t.Fatalf("mark unresolved: %v", err)
	}
	_, err = store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrUnresolvedExecution {
		t.Fatalf("err = %v, want unresolved-execution", err)
	}
	if oe.Incumbent == nil || oe.Incumbent.State != "unresolved" || oe.Incumbent.Kind != "raw" {
		t.Fatalf("unresolved snapshot = %+v", oe.Incumbent)
	}
}

func TestReserveStaleEpochCarriesSnapshot(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	_, err := store.ReserveWorktreeExecutionForEpoch(repoID, worktree, "epoch-a", nil)
	if err != nil {
		t.Fatalf("epoch reserve: %v", err)
	}
	_, err = store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrStaleRunEpoch {
		t.Fatalf("err = %v, want stale-run-epoch", err)
	}
	if oe.Incumbent == nil || !oe.Incumbent.EpochOwned {
		t.Fatalf("stale-epoch snapshot = %+v", oe.Incumbent)
	}
}
