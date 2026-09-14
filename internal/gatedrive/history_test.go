package gatedrive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// copyLegacyFixture installs testdata/legacy-v2/<name>.json as drive <id>'s
// record in store s (creating the 0700 dir), returning the id.
func copyLegacyFixture(t *testing.T, s *Store, name string) string {
	t.Helper()
	id := "0428aaaaaaaaaaaaaaaaaaaaaaaaaa" + map[string]string{
		"passed": "01", "failed": "02", "halted": "03", "waiting": "04",
		"schema1": "05", "missing-worktree": "06", "corrupt": "07"}[name]
	dir := filepath.Join(s.root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(filepath.Join("testdata", "legacy-v2", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

// rewriteRecordField reads drive id's record.json, replaces exactly one
// occurrence of old with new, and writes it back.
func rewriteRecordField(t *testing.T, s *Store, id, old, newv string) {
	t.Helper()
	p := filepath.Join(s.root, id, recordFileName)
	buf, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(buf), old); n != 1 {
		t.Fatalf("want exactly one occurrence of %q, got %d", old, n)
	}
	out := strings.Replace(string(buf), old, newv, 1)
	if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadHistoricalDriveReadsSchema2(t *testing.T) {
	s := OpenStore(t.TempDir())
	id := copyLegacyFixture(t, s, "passed")
	h, err := s.loadHistoricalDrive(id)
	if err != nil {
		t.Fatalf("schema-2 must load for assessment: %v", err)
	}
	if h.SchemaVersion != 2 || h.LastOutcome != PASSED || h.WorktreePath == "" || h.RepoIdentity == "" {
		t.Fatalf("bad view: %+v", h)
	}
	// The EXECUTABLE reader still refuses it — no new compatibility granted.
	if _, err := s.Load(id); !isStoreKind(err, ErrUnknownSchema) {
		t.Fatalf("execution reader must still refuse v2, got %v", err)
	}
}

func TestLoadHistoricalDriveFailsClosed(t *testing.T) {
	s := OpenStore(t.TempDir())
	for name, kind := range map[string]StoreErrorKind{
		"schema1": ErrUnknownSchema,
		"corrupt": ErrCorruptRecord,
	} {
		id := copyLegacyFixture(t, s, name)
		if _, err := s.loadHistoricalDrive(id); !isStoreKind(err, kind) {
			t.Fatalf("%s: want %s, got %v", name, kind, err)
		}
	}
}

// A v2 document with a required identity/outcome field missing must be refused
// as corrupt — never zero-value-decoded into a trustworthy record.
func TestLoadHistoricalDriveValidatesRequiredFields(t *testing.T) {
	s := OpenStore(t.TempDir())
	id := copyLegacyFixture(t, s, "passed")
	rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": ""`)
	if _, err := s.loadHistoricalDrive(id); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("empty worktree_path must fail closed, got %v", err)
	}
	id2 := copyLegacyFixture(t, s, "halted")
	rewriteRecordField(t, s, id2, `"last_outcome": "HALTED"`, `"last_outcome": "EXPLODED"`)
	if _, err := s.loadHistoricalDrive(id2); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("unknown outcome vocabulary must fail closed, got %v", err)
	}
}

// fakeRecovery is a scripted recoverySeam: it records every mark=true call in
// marks (so a preview/apply distinction is observable) and returns the entry
// scripted for the requested runDir. An unscripted runDir yields "invalid"; a
// non-nil err short-circuits every call — modelling a probe error that must
// never be read as clean absence.
type fakeRecovery struct {
	entries map[string]process.RecoveryEntry // key: runDir
	err     error
	marks   []string
}

func (f *fakeRecovery) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	if mark {
		f.marks = append(f.marks, runDir)
	}
	if f.err != nil {
		return process.RecoveryEntry{}, f.err
	}
	e, ok := f.entries[runDir]
	if !ok {
		return process.RecoveryEntry{Disposition: "invalid", Reason: "no slot"}, nil
	}
	if mark && e.Disposition == "abandonable" {
		e.Disposition = "abandoned-marked"
	}
	return e, nil
}

// TestClassifyLegacyDrive exercises the shared legacy-drive classifier. The
// step ordering is load-bearing (terminal recognized BEFORE path resolution;
// HALTED assessed through the process predicate with BOTH run dirs proving
// teardown; a probe error retains, never recovers), so the cases pin the class,
// selected reasons, and — where the apply distinction matters — the exact
// number of marking calls the seam saw.
func TestClassifyLegacyDrive(t *testing.T) {
	s := OpenStore(t.TempDir())

	canon := func(p string) string {
		t.Helper()
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatalf("canonicalise %s: %v", p, err)
		}
		return r
	}
	wtA := t.TempDir()
	wtB := t.TempDir()
	canonA := canon(wtA)
	canonB := canon(wtB)

	load := func(name string) historicalDrive {
		id := copyLegacyFixture(t, s, name)
		h, err := s.loadHistoricalDrive(id)
		if err != nil {
			t.Fatalf("load %s fixture: %v", name, err)
		}
		return h
	}
	passed := load("passed")
	failed := load("failed")
	waiting := load("waiting")
	halted := load("halted")
	missingWt := load("missing-worktree")

	fake := func(entries map[string]process.RecoveryEntry) *fakeRecovery {
		return &fakeRecovery{entries: entries}
	}

	// Case 1: PASSED bound to a resolvable but DIFFERENT worktree. Terminal
	// history is recognised before path resolution, so the class AND reason are
	// the terminal ones — the seam is never consulted (apply=true would record a
	// mark had it been). Swapping the terminal check after path resolution flips
	// the reason to "bound to a different worktree": this case is the
	// terminal-before-path guard.
	h1 := passed
	h1.WorktreePath = wtB

	// Case 1b: a supervisor-committed PASSED outcome survives a REMOVED worktree —
	// durable evidence a gone directory cannot undo.
	h1b := missingWt

	// Case 2: FAILED, worktree same as requested — terminal wins regardless.
	h2 := failed
	h2.WorktreePath = wtB

	// Case 3: WAITING bound to a different real worktree (admission pass).
	h3 := waiting
	h3.WorktreePath = wtB

	// Case 4: WAITING whose worktree path is unresolvable — NOT proof of
	// unrelatedness, so it is retained, never declared irrelevant.
	h4 := waiting // fixture worktree /repo/.worktrees/old-feature is gone

	// Case 5: WAITING same worktree — live/nonterminal is never guessed dead.
	h5 := waiting
	h5.WorktreePath = wtB

	// Cases 6-11,15: HALTED assessed via the process predicate (cleanup pass).
	// Case 12: empty RawRunDir — missing evidence certifies nothing.
	h12 := halted
	h12.RawRunDir = ""
	// Case 13: both recorded attempts must prove teardown; the prior run's
	// needs-inspection retains even though the current run is terminal.
	h13 := halted
	h13.RawRunDir = "cur-run"
	h13.PriorRawRunDir = "prior-run"

	cases := []struct {
		name      string
		h         historicalDrive
		requested string
		seam      recoverySeam
		fake      *fakeRecovery // when non-nil, len(marks) is asserted against wantMarks
		apply     bool
		wantClass string
		reasonSub string
		wantMarks int
	}{
		{
			name: "1-passed-different-worktree-terminal-before-path",
			h:    h1, requested: canonA, apply: true,
			fake:      fake(nil),
			wantClass: LegacyNonblocking, reasonSub: "completed terminal outcome", wantMarks: 0,
		},
		{
			name: "1b-passed-removed-worktree-durable",
			h:    h1b, requested: canonA, apply: true,
			fake:      fake(nil),
			wantClass: LegacyNonblocking, reasonSub: "completed terminal outcome", wantMarks: 0,
		},
		{
			name: "2-failed-same-worktree",
			h:    h2, requested: canonB, apply: false,
			wantClass: LegacyNonblocking, reasonSub: "completed terminal outcome",
		},
		{
			name: "3-waiting-different-worktree-nonblocking",
			h:    h3, requested: canonA, apply: true,
			fake:      fake(nil),
			wantClass: LegacyNonblocking, reasonSub: "different worktree", wantMarks: 0,
		},
		{
			name: "4-waiting-unresolvable-worktree-retained",
			h:    h4, requested: canonA, apply: false,
			wantClass: LegacyRetained, reasonSub: "nonterminal",
		},
		{
			name: "5-waiting-same-worktree-retained",
			h:    h5, requested: canonB, apply: false,
			wantClass: LegacyRetained, reasonSub: "nonterminal",
		},
		{
			name: "6-halted-terminal-nonblocking-no-mark",
			h:    halted, requested: "", apply: false,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "terminal"}}),
			wantClass: LegacyNonblocking, reasonSub: "durable teardown", wantMarks: 0,
		},
		{
			name: "7-halted-already-abandoned-nonblocking",
			h:    halted, requested: "", apply: false,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "already-abandoned"}}),
			wantClass: LegacyNonblocking, wantMarks: 0,
		},
		{
			name: "8-halted-abandonable-apply-recovered-one-mark",
			h:    halted, requested: "", apply: true,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "abandonable"}}),
			wantClass: LegacyRecovered, wantMarks: 1,
		},
		{
			name: "9-halted-abandonable-preview-recoverable-zero-marks",
			h:    halted, requested: "", apply: false,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "abandonable"}}),
			wantClass: LegacyRecoverable, wantMarks: 0,
		},
		{
			name: "10-halted-needs-inspection-retained",
			h:    halted, requested: "", apply: false,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "needs-inspection"}}),
			wantClass: LegacyRetained, reasonSub: "not provably torn down", wantMarks: 0,
		},
		{
			name: "11-halted-seam-error-retained",
			h:    halted, requested: "", apply: false,
			fake:      &fakeRecovery{err: errors.New("probe failed")},
			wantClass: LegacyRetained, reasonSub: "evidence unprovable", wantMarks: 0,
		},
		{
			name: "12-halted-empty-run-dir-retained",
			h:    h12, requested: "", apply: false,
			fake:      fake(nil),
			wantClass: LegacyRetained, reasonSub: "no assessable run evidence", wantMarks: 0,
		},
		{
			name: "13-halted-prior-run-needs-inspection-retained",
			h:    h13, requested: "", apply: false,
			fake: fake(map[string]process.RecoveryEntry{
				"cur-run":   {Disposition: "terminal"},
				"prior-run": {Disposition: "needs-inspection"},
			}),
			wantClass: LegacyRetained, reasonSub: "not provably torn down", wantMarks: 0,
		},
		{
			name: "14-halted-worktree-gone-seam-terminal-nonblocking",
			h:    halted, requested: canonA, apply: false,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "terminal"}}),
			wantClass: LegacyNonblocking, reasonSub: "durable teardown", wantMarks: 0,
		},
		{
			name: "15-nil-seam-halted-retained",
			h:    halted, requested: "", apply: false,
			seam:      nil,
			wantClass: LegacyRetained, reasonSub: "no assessable run evidence",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var seam recoverySeam
			if c.fake != nil {
				seam = c.fake
			} else {
				seam = c.seam // nil for the nil-seam case
			}
			got := s.classifyLegacyDrive(c.h, c.requested, seam, c.apply)
			if got.DriveID != c.h.ID {
				t.Errorf("DriveID = %q, want %q", got.DriveID, c.h.ID)
			}
			if got.Class != c.wantClass {
				t.Errorf("Class = %q, want %q (reason %q)", got.Class, c.wantClass, got.Reason)
			}
			if c.reasonSub != "" && !strings.Contains(got.Reason, c.reasonSub) {
				t.Errorf("Reason = %q, want it to contain %q", got.Reason, c.reasonSub)
			}
			if c.fake != nil && len(c.fake.marks) != c.wantMarks {
				t.Errorf("marks = %v (len %d), want %d entries", c.fake.marks, len(c.fake.marks), c.wantMarks)
			}
		})
	}
}

// haltedFixtureRunDir is the raw_run_dir recorded in testdata/legacy-v2/halted.json;
// a scripted seam keys on it to drive the HALTED assessment.
const haltedFixtureRunDir = "/tmp/docket-gate-old/run-root/00000000000000000000000000000003"

// TestReserveInventoriesLegacyHistoryThroughClassifier drives the first-admission
// legacy census through the exact reserve chokepoint admission_test.go uses today
// (store.reserveWorktreeExecution + mkWorktree/sampleAdmission), proving the
// census now assesses ALL records via the shared classifier: a terminal PASSED
// drive bound to a removed worktree no longer blocks an unrelated worktree (the
// reported defect), completed history admits and is counted, an abandonable HALTED
// drive is recovered under apply, and every unassessable record refuses ONCE with a
// safe locator and the summary attached to the OwnershipError.
func TestReserveInventoriesLegacyHistoryThroughClassifier(t *testing.T) {
	seamWith := func(entries map[string]process.RecoveryEntry) *fakeRecovery {
		return &fakeRecovery{entries: entries}
	}

	// Case 1: THE REPORTED REFUSAL, then the FIX. A supervisor-committed PASSED
	// drive bound to a since-removed worktree must not block an unrelated worktree.
	t.Run("1-passed-removed-worktree-admits-unrelated", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		copyLegacyFixture(t, s, "passed") // worktree_path is a removed /repo/.worktrees/old-feature
		wt := mkWorktree(t)               // a different, unrelated, real worktree
		token, legacy, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
		if err != nil {
			t.Fatalf("a completed legacy drive bound to a removed worktree must not block an unrelated worktree, got %v", err)
		}
		if token == "" {
			t.Fatal("a successful admission must mint a reservation token")
		}
		if legacy == nil || legacy.Checked != 1 || len(legacy.Retained) != 0 {
			t.Fatalf("want a summary with Checked==1 and no retained findings, got %+v", legacy)
		}
	})

	// Case 2: multiple completed legacy drives (PASSED + FAILED + terminal HALTED)
	// all admit; every one is counted.
	t.Run("2-multiple-completed-admit-checked-3", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		copyLegacyFixture(t, s, "passed")
		copyLegacyFixture(t, s, "failed")
		copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		seam := seamWith(map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "terminal"}})
		token, legacy, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
		if err != nil {
			t.Fatalf("three completed legacy drives must admit, got %v", err)
		}
		if token == "" {
			t.Fatal("expected a reservation token")
		}
		if legacy == nil || legacy.Checked != 3 || len(legacy.Retained) != 0 {
			t.Fatalf("want Checked==3 with no retained findings, got %+v", legacy)
		}
	})

	// Case 3: a v1 (schema-1) record is unreadable for assessment: refuse ONCE with
	// the drive's locator and name it in the summary.
	t.Run("3-v1-schema-refuses-with-locator", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "schema1")
		wt := mkWorktree(t)
		_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrUnresolvedExecution {
			t.Fatalf("a v1 record must refuse with ErrUnresolvedExecution, got %v", err)
		}
		if oe.Op != "inventory-legacy-drive-"+id {
			t.Fatalf("locator Op = %q, want inventory-legacy-drive-%s", oe.Op, id)
		}
		if oe.Legacy == nil || len(oe.Legacy.Retained) != 1 || oe.Legacy.Retained[0].DriveID != id {
			t.Fatalf("summary must name the retained drive %s, got %+v", id, oe.Legacy)
		}
	})

	// Case 4: a corrupt record refuses with the same shape.
	t.Run("4-corrupt-refuses-with-locator", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "corrupt")
		wt := mkWorktree(t)
		_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrUnresolvedExecution || oe.Op != "inventory-legacy-drive-"+id {
			t.Fatalf("a corrupt record must refuse naming its id, got %v", err)
		}
		if oe.Legacy == nil || len(oe.Legacy.Retained) != 1 || oe.Legacy.Retained[0].DriveID != id {
			t.Fatalf("summary must name the retained drive, got %+v", oe.Legacy)
		}
	})

	// Case 5: a non-directory / invalid-name entry refuses with the SAFE
	// inventory-level locator; the raw entry name never enters a diagnostic.
	t.Run("5-invalid-entry-uses-safe-locator", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		if err := os.MkdirAll(s.root, 0o700); err != nil {
			t.Fatal(err)
		}
		const rawName = "not-a-valid-drive-id"
		if err := os.WriteFile(filepath.Join(s.root, rawName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		wt := mkWorktree(t)
		_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrUnresolvedExecution || oe.Op != "inventory-legacy-drives" {
			t.Fatalf("an invalid entry must refuse with the safe inventory-level locator, got %v", err)
		}
		if strings.Contains(err.Error(), rawName) {
			t.Fatalf("the raw entry name must never enter a diagnostic: %q", err.Error())
		}
		if oe.Legacy == nil || len(oe.Legacy.Retained) != 1 || oe.Legacy.Retained[0].DriveID != "" {
			t.Fatalf("summary must record one retained finding with no drive id, got %+v", oe.Legacy)
		}
	})

	// Case 6: a record-less drive directory is still skipped (a concurrent
	// first-admission creation window), so a store holding only such a directory
	// admits with no summary narration.
	t.Run("6-recordless-dir-skipped", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		const recordlessID = "abcdef0123456789abcdef0123456789"
		if err := os.MkdirAll(filepath.Join(s.root, recordlessID), 0o700); err != nil {
			t.Fatal(err)
		}
		wt := mkWorktree(t)
		token, legacy, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
		if err != nil {
			t.Fatalf("a record-less in-flight directory must be skipped, got %v", err)
		}
		if token == "" {
			t.Fatal("expected a reservation token")
		}
		if legacy != nil {
			t.Fatalf("no assessable legacy history must yield no summary, got %+v", legacy)
		}
	})

	// Case 7: a same-worktree HALTED drive the seam reports abandonable is RECOVERED
	// under apply; the seam sees mark=true exactly once per run dir.
	t.Run("7-halted-abandonable-recovered", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": "`+wt+`"`)
		seam := seamWith(map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "abandonable"}})
		token, legacy, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
		if err != nil {
			t.Fatalf("an abandonable HALTED drive must recover and admit, got %v", err)
		}
		if token == "" {
			t.Fatal("expected a reservation token")
		}
		if legacy == nil || legacy.Checked != 1 || len(legacy.Retained) != 0 {
			t.Fatalf("want Checked==1 with no retained findings, got %+v", legacy)
		}
		if len(legacy.Recovered) != 1 || legacy.Recovered[0] != id {
			t.Fatalf("want Recovered==[%s], got %+v", id, legacy.Recovered)
		}
		if len(seam.marks) != 1 || seam.marks[0] != haltedFixtureRunDir {
			t.Fatalf("seam must mark exactly once per run dir, got %v", seam.marks)
		}
	})

	// Case 8: a HALTED drive the seam reports needs-inspection refuses, naming the id.
	t.Run("8-halted-needs-inspection-refuses", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		seam := seamWith(map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "needs-inspection"}})
		_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrUnresolvedExecution || oe.Op != "inventory-legacy-drive-"+id {
			t.Fatalf("a needs-inspection HALTED drive must refuse naming its id, got %v", err)
		}
		if oe.Legacy == nil || len(oe.Legacy.Retained) != 1 || oe.Legacy.Retained[0].DriveID != id {
			t.Fatalf("summary must name the retained drive, got %+v", oe.Legacy)
		}
	})

	// Case 8b: a scripted seam PROBE ERROR is not clean absence — it refuses too.
	t.Run("8b-halted-seam-error-refuses", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		seam := &fakeRecovery{err: errors.New("probe failed")}
		_, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrUnresolvedExecution || oe.Op != "inventory-legacy-drive-"+id {
			t.Fatalf("a probe error must refuse naming the id, got %v", err)
		}
	})

	// Case 9: with a nil seam (the exported ReserveWorktreeExecution), a HALTED
	// drive fails closed unchanged.
	t.Run("9-nil-seam-halted-refuses", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		_, err := s.ReserveWorktreeExecution(sampleAdmission(wt))
		oe, ok := AsOwnershipError(err)
		if !ok || oe.Kind != ErrUnresolvedExecution || oe.Op != "inventory-legacy-drive-"+id {
			t.Fatalf("a nil seam must fail a HALTED drive closed, got %v", err)
		}
	})

	// Case 10: the census is first-admission-only. A second reservation after a
	// release reads the existing slot record and does NOT re-run the inventory, so
	// the seam is never consulted again.
	t.Run("10-second-reservation-skips-inventory", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": "`+wt+`"`)
		seam := seamWith(map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "abandonable"}})
		token, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam)
		if err != nil {
			t.Fatalf("first admission must recover the HALTED drive, got %v", err)
		}
		firstMarks := len(seam.marks)
		if firstMarks != 1 {
			t.Fatalf("first admission should have marked exactly once, got %d", firstMarks)
		}
		if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
			t.Fatalf("release: %v", err)
		}
		if _, _, err := s.reserveWorktreeExecution(sampleAdmission(wt), seam); err != nil {
			t.Fatalf("readmit over a released slot: %v", err)
		}
		if len(seam.marks) != firstMarks {
			t.Fatalf("second reservation must not re-run the inventory: marks grew %d -> %d", firstMarks, len(seam.marks))
		}
	})
}
