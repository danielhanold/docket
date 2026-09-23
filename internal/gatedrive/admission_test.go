package gatedrive

import (
	"go/ast"
	"go/parser"
	"go/token"
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

// TestLegacyUnreadableUnreferencedRecordIsDiagnostic proves the first inventory
// never skips a malformed historical record — it stays a retained finding in the
// summary — but, because its worktree binding cannot be established, it is
// discovery rather than required evidence and does not veto an admission
// (change 0446 spec §§1-2, relying on ADR-0118's upgrade-quiescence contract).
func TestLegacyUnreadableUnreferencedRecordIsDiagnostic(t *testing.T) {
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

	_, sum, err := s.reserveWorktreeExecution(sampleAdmission(wt), nil)
	if err != nil {
		t.Fatalf("an unreadable unreferenced legacy record must not block admission, got %v", err)
	}
	if sum == nil || len(sum.Retained) != 1 || sum.Retained[0].DriveID != corruptID {
		t.Fatalf("the unreadable record must stay a retained diagnostic, got %+v", sum)
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
// TestLegacyUnreadableUnreferencedRecordIsDiagnostic: a PRESENT but corrupt
// record is still counted and reported as a retained diagnostic.
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

// snapshotRegistry reads every regular file below the drive registry root into a
// path->bytes map, so a test can prove an admission rewrote or deleted nothing.
func snapshotRegistry(t *testing.T, s *Store) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(s.root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			buf, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			out[p] = buf
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot registry: %v", err)
	}
	return out
}

// seedLegacyDrive persists an executable-range drive record bound to worktree
// with the given outcome, cause, and raw run dir, returning its id.
func seedLegacyDrive(t *testing.T, s *Store, worktree string, outcome Outcome, cause, runDir string) string {
	t.Helper()
	rec := seedRecord(t)
	rec.WorktreePath = worktree
	rec.LastOutcome = outcome
	rec.LastCause = cause
	rec.RawRunDir = runDir
	id, _, err := s.NewDrive(rec)
	if err != nil {
		t.Fatalf("seed legacy drive: %v", err)
	}
	return id
}

// writeRawDriveRecord installs raw bytes as drive id's record.json.
func writeRawDriveRecord(t *testing.T, s *Store, id string, body []byte) {
	t.Helper()
	dir := filepath.Join(s.root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestFirstAdmissionUnrelatedHistoryIsDiagnostic (change 0446 spec §§1-2): history
// that is not positively bound to the requested worktree — records bound to a
// DIFFERENT worktree, records whose binding cannot be resolved at all (a removed
// worktree, a never-existing path), unreadable records, unknown schemas, a stray
// registry entry, and a schema-2 historical record — is diagnostic, never a veto.
// A fresh worktree's FIRST reservation must succeed, the damaged records must stay
// visible in the returned summary, and no seeded byte may change.
func TestFirstAdmissionUnrelatedHistoryIsDiagnostic(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	other := mkWorktree(t)
	removed := mkWorktree(t)
	if err := os.RemoveAll(removed); err != nil {
		t.Fatal(err)
	}

	seedLegacyDrive(t, s, other, PASSED, "", "/runs/gone-passed")
	seedLegacyDrive(t, s, other, FAILED, "", "/runs/gone-failed")
	for i, cause := range []string{"deadline-expired", "stopped-not-initiated", "launch-unresolved", "identity-mismatch"} {
		seedLegacyDrive(t, s, other, HALTED, cause, "/runs/gone-halted-other-"+string(rune('a'+i)))
		seedLegacyDrive(t, s, removed, HALTED, cause, "/runs/gone-halted-removed-"+string(rune('a'+i)))
	}
	seedLegacyDrive(t, s, other, WAITING, "", "/runs/gone-waiting-other")
	seedLegacyDrive(t, s, removed, WAITING, "", "/runs/gone-waiting-removed")
	seedLegacyDrive(t, s, "/repo", HALTED, "deadline-expired", "") // never-existing path, no run evidence
	unsupportedID := "0446aaaaaaaaaaaaaaaaaaaaaaaaaa99"
	writeRawDriveRecord(t, s, unsupportedID, []byte(`{"generation":"g","record":{"schema_version":99}}`))
	malformedID := "0446aaaaaaaaaaaaaaaaaaaaaaaaaa98"
	writeRawDriveRecord(t, s, malformedID, []byte("not-json"))
	if err := os.WriteFile(filepath.Join(s.root, "stray-entry"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyLegacyFixture(t, s, "halted") // schema-2, HALTED, removed worktree

	before := snapshotRegistry(t, s)
	wt := mkWorktree(t)
	seam := &fakeRecovery{} // every run dir answers "invalid": no teardown proof anywhere
	token, sum, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
	if err != nil {
		t.Fatalf("unrelated history must not veto a fresh worktree's first admission, got %v", err)
	}
	if token == "" {
		t.Fatal("a successful admission must mint a reservation token")
	}
	if sum == nil {
		t.Fatal("assessed history must ride the success summary")
	}
	retained := map[string]LegacyFinding{}
	for _, f := range sum.Retained {
		retained[f.DriveID] = f
	}
	for _, id := range []string{unsupportedID, malformedID, ""} {
		if _, ok := retained[id]; !ok {
			t.Fatalf("damaged record %q must stay visible in the summary, got %+v", id, sum.Retained)
		}
	}
	if len(seam.marks) != 0 {
		t.Fatalf("unrelated history must never be marked, got %v", seam.marks)
	}
	after := snapshotRegistry(t, s)
	if len(after) != len(before) {
		t.Fatalf("registry file count changed %d -> %d", len(before), len(after))
	}
	for p, b := range before {
		if string(after[p]) != string(b) {
			t.Fatalf("historical record %s was rewritten or deleted", p)
		}
	}
}

// TestFirstAdmissionOwnBoundHistoryStillBlocks: a record positively bound to the
// requested worktree keeps today's full assessment — a nonterminal record and an
// unprovable HALTED record each refuse with the drive's locator, and the finding
// names the matched worktree. A live symlink alias of the requested worktree is a
// positive binding too.
func TestFirstAdmissionOwnBoundHistoryStillBlocks(t *testing.T) {
	cases := []struct {
		name    string
		outcome Outcome
		alias   bool
	}{
		{"waiting", WAITING, false},
		{"halted-unprovable", HALTED, false},
		{"waiting-via-symlink-alias", WAITING, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := OpenStore(testsupport.TempDir(t))
			wt := mkWorktree(t)
			bound := wt
			if c.alias {
				bound = filepath.Join(testsupport.TempDir(t), "alias")
				if err := os.Symlink(wt, bound); err != nil {
					t.Fatal(err)
				}
			}
			id := seedLegacyDrive(t, s, bound, c.outcome, "deadline-expired", "/runs/own-bound")
			_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), &fakeRecovery{})
			oe, ok := AsOwnershipError(err)
			if !ok || oe.Kind != ErrUnresolvedExecution {
				t.Fatalf("own-bound history must refuse ErrUnresolvedExecution, got %v", err)
			}
			if oe.Op != "inventory-legacy-drive-"+id {
				t.Fatalf("locator Op = %q, want inventory-legacy-drive-%s", oe.Op, id)
			}
			if oe.Legacy == nil || len(oe.Legacy.Retained) != 1 {
				t.Fatalf("summary must carry exactly the bound finding, got %+v", oe.Legacy)
			}
			if f := oe.Legacy.Retained[0]; f.DriveID != id || f.Worktree != bound {
				t.Fatalf("finding must name the drive and its matched worktree (%s / %s), got %+v", id, bound, f)
			}
		})
	}
}

// TestFirstAdmissionLiveIncumbentSameWorktreeBlocks: a HALTED record for the
// requested worktree whose run the process seam reports live still refuses.
func TestFirstAdmissionLiveIncumbentSameWorktreeBlocks(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	id := seedLegacyDrive(t, s, wt, HALTED, "deadline-expired", "/runs/live")
	seam := &fakeRecovery{entries: map[string]process.RecoveryEntry{"/runs/live": {Disposition: "live"}}}
	_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrUnresolvedExecution || oe.Op != "inventory-legacy-drive-"+id {
		t.Fatalf("a live same-worktree HALTED incumbent must refuse naming %s, got %v", id, err)
	}
	if len(seam.marks) != 0 {
		t.Fatalf("a live run must never be marked, got %v", seam.marks)
	}
}

// TestAdmissionSlotDriveIDHasNoProductionWriter pins the premise behind the app
// layer's incumbentRemedyMessage having no drive-id branch (change 0446 spec
// "Admission slot facts"): no production code in this package — the only package
// that can name the unexported admissionRecord — ever sets a slot's DriveID. It
// scans every non-test source file for the two syntactic write shapes: an
// admissionRecord composite literal with a DriveID key, and an assignment whose
// left side is a .DriveID selector. The selector shape is deliberately broad (no
// type information): should a different type's DriveID field ever be assigned
// here, narrow this guard rather than deleting it.
func TestAdmissionSlotDriveIDHasNoProductionWriter(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		scanned++
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if id, ok := x.Type.(*ast.Ident); ok && id.Name == "admissionRecord" {
					for _, el := range x.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "DriveID" {
								t.Errorf("%s: admissionRecord literal sets DriveID", fset.Position(kv.Pos()))
							}
						}
					}
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "DriveID" {
						t.Errorf("%s: assignment to a .DriveID field", fset.Position(sel.Pos()))
					}
				}
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("scanned no production source files: the guard is vacuous")
	}
}
