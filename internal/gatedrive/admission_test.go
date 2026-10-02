package gatedrive

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
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
		RunID:        "",
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
	if err := os.WriteFile(path, []byte(`{"generation":"x","record":{"schema_version":99}}`), 0o600); err != nil {
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
	if !isOwnership(err, ErrLaunchUnconfirmed) || !strings.Contains(err.Error(), id) {
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
	if inc.DriveID != "" || inc.RunOwned {
		t.Fatalf("raw snapshot leaked drive/run facts: %+v", inc)
	}
}

// TestReserveUnresolvedRefusalCarriesSnapshot proves the launch-unconfirmed
// refusal also snapshots the incumbent, and TestReserveStaleRunCarriesSnapshot
// proves the stale-run-id fence does (RunOwned true, run id NOT projected).
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
	if !ok || oe.Kind != ErrLaunchUnconfirmed {
		t.Fatalf("err = %v, want launch-unconfirmed", err)
	}
	if oe.Incumbent == nil || oe.Incumbent.State != "unresolved" || oe.Incumbent.Kind != "raw" {
		t.Fatalf("unresolved snapshot = %+v", oe.Incumbent)
	}
}

func TestReserveStaleRunCarriesSnapshot(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	_, err := store.ReserveWorktreeExecutionForRun(repoID, worktree, "run-a", nil)
	if err != nil {
		t.Fatalf("run reserve: %v", err)
	}
	_, err = store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrStaleRunID {
		t.Fatalf("err = %v, want stale-run-id", err)
	}
	if oe.Incumbent == nil || !oe.Incumbent.RunOwned {
		t.Fatalf("stale-run snapshot = %+v", oe.Incumbent)
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
	for i, cause := range []string{"deadline-expired", "stopped-not-initiated", "launch-unresolved", "worktree-changed"} {
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
			if !ok || oe.Kind != ErrLaunchUnconfirmed {
				t.Fatalf("own-bound history must refuse ErrLaunchUnconfirmed, got %v", err)
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
	if !ok || oe.Kind != ErrLaunchUnconfirmed || oe.Op != "inventory-legacy-drive-"+id {
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

// settledSeam is a scripted RunSettledFunc: it answers from settled (run id →
// verdict), returns err when set, and records every run id it was asked about so
// a test can prove the seam was (or was never) consulted.
type settledSeam struct {
	mu      sync.Mutex
	settled map[string]bool
	err     error
	calls   []string
}

func (f *settledSeam) resolve(runID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, runID)
	if f.err != nil {
		return false, f.err
	}
	return f.settled[runID], nil
}

func (f *settledSeam) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// releasedRunSlot reserves worktree for run "run-e1" and releases it, leaving
// the realistic between-drives shape: a released slot whose RunID survives.
func releasedRunSlot(t *testing.T, s *Store, worktree, repoID string) admissionRecord {
	t.Helper()
	token, err := s.ReserveWorktreeExecutionForRun(repoID, worktree, "run-e1", nil)
	if err != nil {
		t.Fatalf("run reserve: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(worktree, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	slot, _, err := s.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("load released slot: %v", err)
	}
	if slot.State != admissionReleased || slot.RunID != "run-e1" {
		t.Fatalf("fixture slot = %q/%q, want released/run-e1", slot.State, slot.RunID)
	}
	return slot
}

// readSlotBytes returns the raw slot document so a refusal test can prove the
// fence left the slot byte-for-byte untouched.
func readSlotBytes(t *testing.T, s *Store, worktree string) []byte {
	t.Helper()
	b, err := os.ReadFile(admissionRecordPath(t, s, worktree))
	if err != nil {
		t.Fatalf("read slot: %v", err)
	}
	return b
}

// TestReleasedSlotWithCompletedRunReadmits: a released slot whose leftover
// RunID names a run the settlement seam proves settled (completed, or
// confirmed-cancelled) is retired through the exact-token retirement and the
// reservation admits — for a new run and for a raw (no-run-record) start alike
// (change 0446 spec §§2, 5). The seam is asked about exactly the slot's run.
func TestReleasedSlotWithCompletedRunReadmits(t *testing.T) {
	cases := []struct {
		name    string
		reserve func(s *Store, wt, repoID string) error
		wantRun string
	}{
		{"new-run", func(s *Store, wt, repoID string) error {
			_, err := s.ReserveWorktreeExecutionForRun(repoID, wt, "run-e2", nil)
			return err
		}, "run-e2"},
		{"raw", func(s *Store, wt, repoID string) error {
			_, err := s.ReserveRawWorktreeExecution(repoID, wt, nil)
			return err
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, worktree, repoID := newAdmissionFixture(t)
			prior := releasedRunSlot(t, store, worktree, repoID)
			seam := &settledSeam{settled: map[string]bool{"run-e1": true}}
			store.SetRunSettledResolver(seam.resolve)

			if err := tc.reserve(store, worktree, repoID); err != nil {
				t.Fatalf("reserve over a settled run's released slot: %v", err)
			}
			got, _, err := store.LoadWorktreeExecution(worktree)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got.RunID != tc.wantRun {
				t.Fatalf("RunID = %q, want %q", got.RunID, tc.wantRun)
			}
			if got.State != admissionReserved || got.ExecutionGen != prior.ExecutionGen+1 {
				t.Fatalf("slot = %q gen %d, want reserved gen %d", got.State, got.ExecutionGen, prior.ExecutionGen+1)
			}
			if len(seam.calls) != 1 || seam.calls[0] != "run-e1" {
				t.Fatalf("seam calls = %v, want exactly [run-e1]", seam.calls)
			}
		})
	}
}

// TestReleasedSlotWithLiveRunStillFenced: an unsettled run (the seam answers
// false — active, cancelling, or completing) and an absent seam both keep today's
// ErrStaleRunID refusal and leave the slot byte-for-byte untouched: a live
// run owns its worktree between drives.
func TestReleasedSlotWithLiveRunStillFenced(t *testing.T) {
	for _, withSeam := range []bool{true, false} {
		name := "seam-unsettled"
		if !withSeam {
			name = "no-seam"
		}
		t.Run(name, func(t *testing.T) {
			store, worktree, repoID := newAdmissionFixture(t)
			releasedRunSlot(t, store, worktree, repoID)
			seam := &settledSeam{settled: map[string]bool{"run-e1": false}}
			if withSeam {
				store.SetRunSettledResolver(seam.resolve)
			}
			before := readSlotBytes(t, store, worktree)

			_, err := store.ReserveWorktreeExecutionForRun(repoID, worktree, "run-e2", nil)
			if !isOwnership(err, ErrStaleRunID) {
				t.Fatalf("err = %v, want stale-run-id", err)
			}
			if string(readSlotBytes(t, store, worktree)) != string(before) {
				t.Fatal("a fenced reservation must not touch the slot")
			}
			if withSeam && seam.callCount() != 1 {
				t.Fatalf("seam calls = %d, want 1", seam.callCount())
			}
		})
	}
}

// TestReleasedSlotSeamErrorFailsClosed: a settlement-seam error (an unreadable
// run registry) is never proof of settlement — ErrStaleRunID, slot untouched.
func TestReleasedSlotSeamErrorFailsClosed(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	releasedRunSlot(t, store, worktree, repoID)
	seam := &settledSeam{settled: map[string]bool{"run-e1": true}, err: os.ErrPermission}
	store.SetRunSettledResolver(seam.resolve)
	before := readSlotBytes(t, store, worktree)

	_, err := store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	if !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("err = %v, want stale-run-id", err)
	}
	if string(readSlotBytes(t, store, worktree)) != string(before) {
		t.Fatal("a seam error must leave the slot untouched")
	}
}

// TestReleasedSlotUnresolvedRunMarksIncumbent: when the settlement seam reports
// that NO readable record carries the released slot's run (ErrRunRecordUnresolved),
// the refusal stays ErrStaleRunID with the slot untouched, and its incumbent
// snapshot says the run is unresolved — so the printed remedy never points at a
// run.cancel that cannot resolve that run. An unsettled run and any other seam
// error leave the flag clear.
func TestReleasedSlotUnresolvedRunMarksIncumbent(t *testing.T) {
	cases := []struct {
		name string
		seam *settledSeam
		want bool
	}{
		{"unresolved", &settledSeam{err: fmt.Errorf("lookup: %w", ErrRunRecordUnresolved)}, true},
		{"unsettled", &settledSeam{settled: map[string]bool{"run-e1": false}}, false},
		{"other-error", &settledSeam{err: os.ErrPermission}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, worktree, repoID := newAdmissionFixture(t)
			releasedRunSlot(t, store, worktree, repoID)
			store.SetRunSettledResolver(tc.seam.resolve)
			before := readSlotBytes(t, store, worktree)

			_, err := store.ReserveWorktreeExecutionForRun(repoID, worktree, "run-e2", nil)
			oe, ok := AsOwnershipError(err)
			if !ok || oe.Kind != ErrStaleRunID || oe.Incumbent == nil {
				t.Fatalf("err = %v, want stale-run-id with an incumbent snapshot", err)
			}
			if oe.Incumbent.RunUnresolved != tc.want {
				t.Fatalf("RunUnresolved = %v, want %v", oe.Incumbent.RunUnresolved, tc.want)
			}
			if string(readSlotBytes(t, store, worktree)) != string(before) {
				t.Fatal("a fenced reservation must not touch the slot")
			}
		})
	}
}

// TestBusySlotNeverConsultsSettledSeam: the run fence on a busy slot is
// unchanged — an executing slot another run owns refuses ErrStaleRunID and
// the settlement seam is never asked, even when it would answer settled.
func TestBusySlotNeverConsultsSettledSeam(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	token, err := store.ReserveWorktreeExecutionForRun(repoID, worktree, "run-e1", nil)
	if err != nil {
		t.Fatalf("run reserve: %v", err)
	}
	if err := store.ConfirmWorktreeExecution(worktree, token, "0123456789abcdef0123456789abcdef", "/runs/0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	seam := &settledSeam{settled: map[string]bool{"run-e1": true}}
	store.SetRunSettledResolver(seam.resolve)

	_, err = store.ReserveWorktreeExecutionForRun(repoID, worktree, "run-e2", nil)
	if !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("err = %v, want stale-run-id", err)
	}
	if seam.callCount() != 0 {
		t.Fatalf("a busy slot consulted the settlement seam %d times", seam.callCount())
	}
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil || slot.State != admissionExecuting || slot.RunID != "run-e1" {
		t.Fatalf("busy slot changed: %+v err=%v", slot, err)
	}
}

// TestRecreatedWorktreePathInheritsAndSettles (spec AC2): a worktree deleted and
// recreated at the same path re-canonicalizes to its old released slot, whose
// RunID still names the settled predecessor. The next reserve settles it and
// admits, as a readmit — the legacy inventory is not re-run (LegacyInventoried
// stays false on the new record).
func TestRecreatedWorktreePathInheritsAndSettles(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	prior := releasedRunSlot(t, store, worktree, repoID)
	if err := os.RemoveAll(worktree); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatalf("recreate worktree: %v", err)
	}
	seam := &settledSeam{settled: map[string]bool{"run-e1": true}}
	store.SetRunSettledResolver(seam.resolve)

	if _, err := store.ReserveWorktreeExecutionForRun(repoID, worktree, "run-e2", nil); err != nil {
		t.Fatalf("reserve on recreated path: %v", err)
	}
	got, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RunID != "run-e2" || got.ExecutionGen != prior.ExecutionGen+1 {
		t.Fatalf("slot = run %q gen %d, want run-e2 gen %d", got.RunID, got.ExecutionGen, prior.ExecutionGen+1)
	}
	if got.LegacyInventoried {
		t.Fatal("a readmit over the inherited slot must not re-run the legacy inventory")
	}
}

// ---------------------------------------------------------------------------
// Change 0446 Task 10 — acceptance matrices (spec "Acceptance tests" AC1, AC3,
// and the quarantine fixture note).
// ---------------------------------------------------------------------------

// quarantineFixtureID is the registry id the incident record was quarantined
// under; the fixture is installed under the same id so the frozen bytes are
// exercised exactly as they sat in the live registry.
const quarantineFixtureID = "b66ce1405cd813e5519c344360f61bd4"

// installQuarantineFixture installs testdata/quarantine-0368-record.json — a
// VERBATIM copy of the sanitized record quarantined from the 0368 incident (a
// schema-4 HALTED stopped-not-initiated drive whose worktree was removed and whose
// scratch run root is gone) — into store s under its original id, and returns the
// installed bytes. The fixture is only ever copied into an isolated test store,
// never into a live registry. Note: the manually edited live CANCELLED run state
// that accompanied the incident is not a test oracle; only this frozen record is.
func installQuarantineFixture(t *testing.T, s *Store) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", "quarantine-0368-record.json"))
	if err != nil {
		t.Fatalf("read quarantine fixture: %v", err)
	}
	writeRawDriveRecord(t, s, quarantineFixtureID, buf)
	rec, err := s.Load(quarantineFixtureID)
	if err != nil {
		t.Fatalf("the quarantine fixture must load through the executable reader: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "stopped-not-initiated" || rec.AdmissionToken == "" {
		t.Fatalf("fixture drifted: outcome %q cause %q token-present %v", rec.LastOutcome, rec.LastCause, rec.AdmissionToken != "")
	}
	return buf
}

// TestQuarantinedHaltedRecordIsNonblockingForUnrelatedWorktree: the exact record
// that blocked every new worktree's first gate admission in the 0368 incident is
// seeded into an isolated store; a fresh, unrelated worktree then admits through
// every start shape — a no-run (finalize-style) drive, a run-carrying
// (build-style) drive, and a participating raw reservation — and the run launch census of an unrelated run is accounted.
// The frozen record's bytes never change.
func TestQuarantinedHaltedRecordIsNonblockingForUnrelatedWorktree(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	fixture := installQuarantineFixture(t, s)
	proc := &fakeProc{} // ClassifyRun answers "invalid": no teardown proof for anything
	d := storeTestDriver(s, &fakeClock{now: startRun()}, proc, stableGit())

	scopeless := sampleStart()
	scopeless.Worktree = mkWorktree(t)
	if doc, err := d.Start(scopeless); err != nil || doc.Outcome != WAITING {
		t.Fatalf("scopeless start over the quarantined record: doc=%+v err=%v", doc, err)
	}
	build := sampleStart()
	build.Worktree = mkWorktree(t)
	build.RunID = "run-fresh"
	if doc, err := d.Start(build); err != nil || doc.Outcome != WAITING {
		t.Fatalf("run-carrying start over the quarantined record: doc=%+v err=%v", doc, err)
	}
	if _, err := s.ReserveRawWorktreeExecution("repo-x", mkWorktree(t), proc); err != nil {
		t.Fatalf("raw reservation over the quarantined record: %v", err)
	}

	report, err := d.ObserveRunLaunches(capHash("run-unrelated-context"))
	if err != nil {
		t.Fatalf("ObserveRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("the quarantined record must not leave an unrelated run unaccounted, findings=%v", report.Findings)
	}

	after, err := os.ReadFile(filepath.Join(s.root, quarantineFixtureID, recordFileName))
	if err != nil {
		t.Fatalf("re-read fixture: %v", err)
	}
	if string(after) != string(fixture) {
		t.Fatal("admission rewrote the quarantined historical record")
	}
}

// TestQuarantinedHaltedRecordStillInspectable: nonblocking is not invisible — the
// repository-wide cleanup assessment still reports the quarantined record as
// retained (its run cannot be proven torn down), naming its stored worktree, and a
// dry run leaves it byte-for-byte untouched.
func TestQuarantinedHaltedRecordStillInspectable(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	fixture := installQuarantineFixture(t, s)
	d := storeTestDriver(s, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())

	out, err := d.CleanupHistory(HistoryCleanupRequest{DryRun: true})
	if err != nil {
		t.Fatalf("CleanupHistory: %v", err)
	}
	if len(out.Findings) != 1 || out.Checked != 1 || out.Retained != 1 {
		t.Fatalf("want exactly the one retained quarantine finding, got %+v", out)
	}
	f := out.Findings[0]
	if f.DriveID != quarantineFixtureID || f.Class != LegacyRetained {
		t.Fatalf("finding = %+v, want %s retained", f, quarantineFixtureID)
	}
	if f.Worktree != "/Users/homer/dev/docket/.worktrees/resume-halted-preallocation-recovery" {
		t.Fatalf("finding must name the record's stored worktree, got %q", f.Worktree)
	}
	after, err := os.ReadFile(filepath.Join(s.root, quarantineFixtureID, recordFileName))
	if err != nil || string(after) != string(fixture) {
		t.Fatalf("inspection must not rewrite the record (err=%v)", err)
	}
}

// scratchAwareProc is a fakeProc whose ClassifyRun answers from the REAL
// filesystem: a run dir that still exists reports "live" — the worst case an
// unrelated historical record can present — and a deleted one "invalid". Every
// probe is recorded so a test can prove admission never consulted unrelated
// history at all.
type scratchAwareProc struct {
	*fakeProc
	mu     sync.Mutex
	probes []string
}

func (p *scratchAwareProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	p.mu.Lock()
	p.probes = append(p.probes, runDir)
	p.mu.Unlock()
	if _, err := os.Stat(runDir); err == nil {
		return process.RecoveryEntry{Disposition: "live"}, nil
	}
	return process.RecoveryEntry{Disposition: "invalid"}, nil
}

func (p *scratchAwareProc) probeCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.probes)
}

// mixedHistoryHaltCauses seeds one HALTED drive per cause; seedMixedHistory derives
// both the drive run dirs and the scratch dirs it creates from it so they cannot drift.
var mixedHistoryHaltCauses = []string{"deadline-expired", "stopped-not-initiated", "launch-unresolved", "worktree-changed", "run-cancelled"}

// seedMixedHistory seeds several records of every history class in the spec's AC1
// matrix, none positively bound to a fresh worktree: for each of a live OTHER
// worktree and a REMOVED one — PASSED, FAILED, HALTED under several causes (run
// dirs under scratch), HALTED with no run dir, WAITING, a record naming a missing
// scope, and a record carrying a token no slot holds — plus unsupported-schema and
// malformed records, both schema-2 historical fixtures, the quarantined incident
// record, and a stray registry entry. Every drive directory is then renamed to an
// id drawn from a seeded permutation, so each seed exercises a different registry
// (sorted id) order. It returns the final drive ids.
func seedMixedHistory(t *testing.T, s *Store, seed int64, other, removed, scratch string) []string {
	t.Helper()
	for _, wt := range []string{other, removed} {
		seedLegacyDrive(t, s, wt, PASSED, "", filepath.Join(scratch, "passed"))
		seedLegacyDrive(t, s, wt, FAILED, "", filepath.Join(scratch, "failed"))
		for _, cause := range mixedHistoryHaltCauses {
			seedLegacyDrive(t, s, wt, HALTED, cause, filepath.Join(scratch, "halted-"+cause))
		}
		seedLegacyDrive(t, s, wt, HALTED, "deadline-expired", "")
		seedLegacyDrive(t, s, wt, WAITING, "", filepath.Join(scratch, "waiting"))
		for _, linked := range []struct{ legacyScopeID, token string }{
			{legacyScopeID: "0446dddddddddddddddddddddddddd01"}, // a pre-0489 drive naming a missing scope
			{token: "0446eeeeeeeeeeeeeeeeeeeeeeeeee02"},         // mismatched token
		} {
			rec := seedRecord(t)
			rec.WorktreePath = wt
			rec.LastOutcome = WAITING
			rec.RawRunDir = filepath.Join(scratch, "linked")
			rec.AdmissionToken = linked.token
			id, _, err := s.NewDrive(rec)
			if err != nil {
				t.Fatalf("seed linked drive: %v", err)
			}
			if linked.legacyScopeID != "" {
				stampLegacyScopeID(t, s, id, linked.legacyScopeID)
			}
		}
	}
	writeRawDriveRecord(t, s, "0446aaaaaaaaaaaaaaaaaaaaaaaaaa99", []byte(`{"generation":"g","record":{"schema_version":99}}`))
	writeRawDriveRecord(t, s, "0446aaaaaaaaaaaaaaaaaaaaaaaaaa98", []byte("not-json"))
	copyLegacyFixture(t, s, "halted")
	copyLegacyFixture(t, s, "waiting")
	installQuarantineFixture(t, s)
	if err := os.WriteFile(filepath.Join(s.root, "stray-entry"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	scratchDirs := []string{"passed", "failed", "waiting", "linked"}
	for _, cause := range mixedHistoryHaltCauses {
		scratchDirs = append(scratchDirs, "halted-"+cause)
	}
	for _, dir := range scratchDirs {
		if err := os.MkdirAll(filepath.Join(scratch, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	perm := rand.New(rand.NewSource(seed)).Perm(len(ids))
	final := make([]string, len(ids))
	for i, id := range ids {
		final[i] = fmt.Sprintf("0446%028x", perm[i])
		if err := os.Rename(filepath.Join(s.root, id), filepath.Join(s.root, final[i])); err != nil {
			t.Fatalf("reorder %s: %v", id, err)
		}
	}
	return final
}

// TestFirstAdmissionMixedHistorySweep (spec AC1): many mixed historical records —
// every class, several of each, under several registry orderings, and both before
// and after their scratch run dirs are deleted — never veto an unrelated worktree.
// With scratch present every recorded run even LOOKS live to the process seam, the
// worst case an unrelated record can present. Every start shape admits (no-run,
// run-carrying, participating raw), admission probes no unrelated history,
// no historical byte changes, and the repository-wide cleanup assessment still
// reports every record with its class counters partitioning the findings exactly.
func TestFirstAdmissionMixedHistorySweep(t *testing.T) {
	for _, seed := range []int64{1, 2, 3} {
		for _, deleteScratch := range []bool{false, true} {
			t.Run(fmt.Sprintf("order-%d/scratch-deleted-%v", seed, deleteScratch), func(t *testing.T) {
				s := OpenStore(testsupport.TempDir(t))
				other := mkWorktree(t)
				removed := mkWorktree(t)
				if err := os.RemoveAll(removed); err != nil {
					t.Fatal(err)
				}
				scratch := testsupport.TempDir(t)
				ids := seedMixedHistory(t, s, seed, other, removed, scratch)
				if deleteScratch {
					if err := os.RemoveAll(scratch); err != nil {
						t.Fatal(err)
					}
				}
				before := snapshotRegistry(t, s)

				proc := &scratchAwareProc{fakeProc: &fakeProc{}}
				d := storeTestDriver(s, &fakeClock{now: startRun()}, proc, stableGit())
				scopeless := sampleStart()
				scopeless.Worktree = mkWorktree(t)
				if doc, err := d.Start(scopeless); err != nil || doc.Outcome != WAITING {
					t.Fatalf("scopeless start: doc=%+v err=%v", doc, err)
				}
				build := sampleStart()
				build.Worktree = mkWorktree(t)
				build.RunID = "run-fresh"
				if doc, err := d.Start(build); err != nil || doc.Outcome != WAITING {
					t.Fatalf("run-carrying start: doc=%+v err=%v", doc, err)
				}
				if _, err := s.ReserveRawWorktreeExecution("repo-x", mkWorktree(t), proc); err != nil {
					t.Fatalf("raw reservation: %v", err)
				}
				if n := proc.probeCount(); n != 0 {
					t.Fatalf("admission probed unrelated history %d times: %v", n, proc.probes)
				}

				after := snapshotRegistry(t, s)
				for p, b := range before {
					if string(after[p]) != string(b) {
						t.Fatalf("historical record %s was rewritten or deleted", p)
					}
				}

				out, err := s.cleanupHistory(HistoryCleanupRequest{DryRun: true}, proc)
				if err != nil {
					t.Fatalf("cleanupHistory: %v", err)
				}
				if sum := out.Recovered + out.Recoverable + out.Retained + out.Nonblocking; sum != len(out.Findings) {
					t.Fatalf("class counters sum %d != %d findings", sum, len(out.Findings))
				}
				seen := map[string]bool{}
				stray := 0
				for _, f := range out.Findings {
					if f.DriveID == "" {
						stray++
						continue
					}
					seen[f.DriveID] = true
				}
				if stray != 1 {
					t.Fatalf("want exactly one id-less stray finding, got %d", stray)
				}
				if out.Checked != len(out.Findings)-stray {
					t.Fatalf("Checked = %d, want every id-bearing finding (%d)", out.Checked, len(out.Findings)-stray)
				}
				for _, id := range ids {
					if !seen[id] {
						t.Fatalf("seeded record %s missing from the cleanup assessment", id)
					}
				}
				final := snapshotRegistry(t, s)
				for p, b := range before {
					if string(final[p]) != string(b) {
						t.Fatalf("a dry-run assessment rewrote %s", p)
					}
				}
			})
		}
	}
}

// TestTargetedCorruptionRefusesLocallyCompanionProceeds (spec AC3): corrupting the
// EXACT drive a current reservation names refuses that worktree — with the
// incumbent snapshot and the bounded reconciliation locator — while its run census
// reports the unreadable record as history (change 0490 attributes by run context),
// and an unrelated corrupt record and a companion worktree in
// the same store are unaffected. Covered in the two windows where the current
// reference is the only link: reserved-before-launch (between Admit and
// StartAdmitted) and a reserved relaunch (replacement reserved, not yet attached).
func TestTargetedCorruptionRefusesLocallyCompanionProceeds(t *testing.T) {
	setup := func(t *testing.T) (*Driver, *Store, *fakeProc, StartRequest) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		writeRawDriveRecord(t, store, "0446aaaaaaaaaaaaaaaaaaaaaaaaaa98", []byte("not-json")) // unrelated corruption
		req := sampleStart()
		req.Worktree = mkWorktree(t)
		req.RunID = "e1"
		req.RunContext = "ctx-e1"
		return d, store, proc, req
	}
	assertLocalRefusal := func(t *testing.T, d *Driver, store *Store, req StartRequest, id, wantState string) {
		t.Helper()
		slotBefore := readSlotBytes(t, store, req.Worktree)
		_, err := d.Admit(req)
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrWorktreeBusy {
			t.Fatalf("a start over a corrupt current incumbent must refuse worktree-busy, got %v", err)
		}
		if oe.Incumbent == nil || oe.Incumbent.State != wantState {
			t.Fatalf("refusal must carry the incumbent snapshot (state %s), got %+v", wantState, oe.Incumbent)
		}
		if oe.Reconciliation != findingDriveUnresolved {
			t.Fatalf("reconciliation locator = %q, want %q", oe.Reconciliation, findingDriveUnresolved)
		}
		if string(readSlotBytes(t, store, req.Worktree)) != string(slotBefore) {
			t.Fatal("a refused start must leave the corrupt incumbent's slot untouched")
		}
		// The census attributes by run context (change 0490): a record it cannot read
		// names no run, so it is informational history, never a census veto.
		report, err := d.ObserveRunLaunches(capHash(req.RunContext))
		if err != nil {
			t.Fatalf("ObserveRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "history-unattributed", id) {
			t.Fatalf("census must account the unreadable record as history-unattributed:%s, got %+v", id, report)
		}
	}
	assertCompanionProceeds := func(t *testing.T, d *Driver, proc *fakeProc) {
		t.Helper()
		launches := proc.launchN
		companion := sampleStart()
		companion.Worktree = mkWorktree(t)
		doc, err := d.Start(companion)
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("the companion worktree must proceed: doc=%+v err=%v", doc, err)
		}
		if proc.launchN != launches+1 {
			t.Fatalf("companion launches = %d, want exactly one", proc.launchN-launches)
		}
	}

	t.Run("reserved-before-launch", func(t *testing.T) {
		d, store, proc, req := setup(t)
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}
		corruptFile(t, filepath.Join(store.root, ticket.id, recordFileName))

		if _, err := d.StartAdmitted(ticket); !isStoreKind(err, ErrCorruptRecord) {
			t.Fatalf("the delayed launch of a corrupt reserved drive must refuse typed, got %v", err)
		}
		if proc.launchN != 0 {
			t.Fatalf("a corrupt reserved drive must never launch, got %d launches", proc.launchN)
		}
		assertLocalRefusal(t, d, store, req, ticket.id, string(admissionReserved))
		assertCompanionProceeds(t, d, proc)
	})

	t.Run("reserved-relaunch", func(t *testing.T) {
		d, store, proc, req := setup(t)
		doc, err := d.Start(req)
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("Start: doc=%+v err=%v", doc, err)
		}
		claim, err := store.reserveRelaunch(doc.DriveID, doc.Generation)
		if err != nil {
			t.Fatalf("reserveRelaunch: %v", err)
		}
		claim.close()
		corruptFile(t, filepath.Join(store.root, doc.DriveID, recordFileName))

		launches := proc.launchN
		if adv, err := d.Advance(doc.DriveID, doc.Generation); err == nil && adv.Outcome != HALTED {
			t.Fatalf("advancing a corrupt drive must halt or fail, got %+v", adv)
		}
		if proc.launchN != launches {
			t.Fatal("a corrupt drive's reserved relaunch must never launch")
		}
		assertLocalRefusal(t, d, store, req, doc.DriveID, string(admissionExecuting))
		assertCompanionProceeds(t, d, proc)
	})
}

// TestOrdinaryReleasePreservesRunID pins that an ordinary release of a slot's
// own token preserves RunID on the released record (the between-drives fence
// depends on it; change 0437 must not weaken it).
func TestOrdinaryReleasePreservesRunID(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	rec := sampleAdmission(wt)
	rec.RunID = "E-keep"
	token, err := s.ReserveWorktreeExecution(rec)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ConfirmWorktreeExecution(wt, token, "run-x", "/runs/x"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	got, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.State != admissionReleased {
		t.Fatalf("state after release = %q, want %q", got.State, admissionReleased)
	}
	if got.RunID != "E-keep" {
		t.Fatalf("ordinary release must preserve RunID, got %q", got.RunID)
	}
}
