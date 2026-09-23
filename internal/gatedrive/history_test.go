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
	s := OpenStore(testsupport.TempDir(t))
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
	s := OpenStore(testsupport.TempDir(t))
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
	s := OpenStore(testsupport.TempDir(t))
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

// fakeRecovery is a scripted recoverySeam: it records each abandoned-marker WRITE
// it performs in marks (an abandonable run it transitions to abandoned-marked
// under mark=true), so a preview/apply distinction and re-apply idempotency are
// both observable — a mark=true call over an already-terminal or already-abandoned
// run writes nothing and records nothing. It returns the entry scripted for the
// requested runDir. An unscripted runDir yields "invalid"; a non-nil err
// short-circuits every call — modelling a probe error that must never be read as
// clean absence.
type fakeRecovery struct {
	entries map[string]process.RecoveryEntry // key: runDir
	err     error
	marks   []string
}

func (f *fakeRecovery) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	if f.err != nil {
		return process.RecoveryEntry{}, f.err
	}
	e, ok := f.entries[runDir]
	if !ok {
		return process.RecoveryEntry{Disposition: "invalid", Reason: "no slot"}, nil
	}
	if mark && e.Disposition == "abandonable" {
		e.Disposition = "abandoned-marked"
		f.marks = append(f.marks, runDir)
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
	s := OpenStore(testsupport.TempDir(t))

	canon := func(p string) string {
		t.Helper()
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatalf("canonicalise %s: %v", p, err)
		}
		return r
	}
	wtA := testsupport.TempDir(t)
	wtB := testsupport.TempDir(t)
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

	// Case 4: WAITING whose worktree path is unresolvable and does not name the
	// requested worktree — failing to resolve some OTHER path establishes no
	// match, so it is a nonblocking diagnostic (change 0446 spec §1).
	h4 := waiting // fixture worktree /repo/.worktrees/old-feature is gone

	// Case 4b: WAITING whose stored path is unresolvable but spells the requested
	// root exactly (a removed worktree's recorded canonical identity) — a
	// positive binding, so it is retained.
	h4b := waiting
	h4b.WorktreePath = "/gone/requested-wt/"

	// Case 4c: WAITING with a live symlink alias of the requested worktree — a
	// positive binding through canonicalization.
	aliasB := filepath.Join(testsupport.TempDir(t), "alias-b")
	if err := os.Symlink(wtB, aliasB); err != nil {
		t.Fatal(err)
	}
	h4c := waiting
	h4c.WorktreePath = aliasB

	// Case 4d: an empty stored worktree never matches.
	h4d := waiting
	h4d.WorktreePath = ""

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
			name: "4-waiting-unresolvable-other-worktree-diagnostic",
			h:    h4, requested: canonA, apply: false,
			wantClass: LegacyNonblocking, reasonSub: "binding unresolvable",
		},
		{
			name: "4b-waiting-unresolvable-same-spelling-retained",
			h:    h4b, requested: "/gone/requested-wt", apply: false,
			wantClass: LegacyRetained, reasonSub: "nonterminal",
		},
		{
			name: "4c-waiting-symlink-alias-retained",
			h:    h4c, requested: canonB, apply: false,
			wantClass: LegacyRetained, reasonSub: "nonterminal",
		},
		{
			name: "4d-waiting-empty-worktree-diagnostic",
			h:    h4d, requested: canonA, apply: false,
			wantClass: LegacyNonblocking, reasonSub: "binding unresolvable",
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
			// A HALTED record whose (removed) worktree does not name the requested
			// one is a diagnostic before the process seam is consulted: an
			// abandonable run is never marked on another worktree's admission.
			name: "14-halted-worktree-gone-other-worktree-diagnostic-no-mark",
			h:    halted, requested: canonA, apply: true,
			fake:      fake(map[string]process.RecoveryEntry{halted.RawRunDir: {Disposition: "abandonable"}}),
			wantClass: LegacyNonblocking, reasonSub: "binding unresolvable", wantMarks: 0,
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
			if got.Worktree != c.h.WorktreePath {
				t.Errorf("Worktree = %q, want the stored identity %q", got.Worktree, c.h.WorktreePath)
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
// drive is recovered under apply, every unassessable record positively bound to the
// requested worktree refuses ONCE with a safe locator and the summary attached to
// the OwnershipError, and unreadable or stray history that binds no worktree stays
// a visible diagnostic that never refuses (change 0446).
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

	// Cases 3-4 (change 0446): a v1 (schema-1) record and a corrupt record are
	// unreadable for assessment, so no worktree binding can be established. They
	// stay RETAINED diagnostics named in the success summary — the history stays
	// inspectable — but never veto the admission (spec §§1-2; the ADR-0118
	// quiescence contract is the accepted residual).
	for _, fixture := range []string{"schema1", "corrupt"} {
		t.Run("3-4-unreadable-"+fixture+"-is-diagnostic", func(t *testing.T) {
			s := OpenStore(testsupport.TempDir(t))
			id := copyLegacyFixture(t, s, fixture)
			wt := mkWorktree(t)
			token, legacy, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
			if err != nil || token == "" {
				t.Fatalf("an unreadable unreferenced %s record must not veto admission, got %v", fixture, err)
			}
			if legacy == nil || len(legacy.Retained) != 1 || legacy.Retained[0].DriveID != id || legacy.Retained[0].Class != LegacyRetained {
				t.Fatalf("summary must keep the retained drive %s visible, got %+v", id, legacy)
			}
		})
	}

	// Case 5: a non-directory / invalid-name entry is a diagnostic; the raw entry
	// name never enters a diagnostic.
	t.Run("5-invalid-entry-is-safe-diagnostic", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		if err := os.MkdirAll(s.root, 0o700); err != nil {
			t.Fatal(err)
		}
		const rawName = "not-a-valid-drive-id"
		if err := os.WriteFile(filepath.Join(s.root, rawName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		wt := mkWorktree(t)
		_, legacy, err := s.reserveWorktreeExecution(sampleAdmission(wt), seamWith(nil))
		if err != nil {
			t.Fatalf("a stray registry entry must not veto admission, got %v", err)
		}
		if legacy == nil || len(legacy.Retained) != 1 || legacy.Retained[0].DriveID != "" {
			t.Fatalf("summary must record one retained finding with no drive id, got %+v", legacy)
		}
		for _, f := range legacy.Retained {
			if strings.Contains(f.Reason, rawName) || strings.Contains(f.Worktree, rawName) {
				t.Fatalf("the raw entry name must never enter a diagnostic: %+v", f)
			}
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

	// Case 8: a same-worktree HALTED drive the seam reports needs-inspection
	// refuses, naming the id.
	t.Run("8-halted-needs-inspection-refuses", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		id := copyLegacyFixture(t, s, "halted")
		wt := mkWorktree(t)
		rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": "`+wt+`"`)
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
		rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": "`+wt+`"`)
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
		rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": "`+wt+`"`)
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

// TestCleanupHistory drives the shared MANUAL recovery assessment (Store.cleanupHistory,
// behind Driver.CleanupHistory). Unlike the first-admission census it NEVER refuses —
// every candidate lands in Findings with its class — and it runs with
// requestedWorktree=="" so worktree resolution is never required even though the
// fixtures' worktrees do not exist.
func TestCleanupHistory(t *testing.T) {
	seamWith := func(entries map[string]process.RecoveryEntry) *fakeRecovery {
		return &fakeRecovery{entries: entries}
	}

	// Case 1: repo-wide scan over five records. Findings are sorted ascending by id;
	// classes and counts are pinned. The halted drive is recovered under apply; the
	// waiting drive is retained as a nonterminal state; the v1 record is retained as
	// an unknown schema. No worktree is ever resolved.
	t.Run("1-repo-wide-scan-sorted-classified", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		idPassed := copyLegacyFixture(t, s, "passed")
		idFailed := copyLegacyFixture(t, s, "failed")
		idHalted := copyLegacyFixture(t, s, "halted")
		idWaiting := copyLegacyFixture(t, s, "waiting")
		idSchema1 := copyLegacyFixture(t, s, "schema1")
		seam := seamWith(map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "abandonable"}})

		out, err := s.cleanupHistory(HistoryCleanupRequest{}, seam)
		if err != nil {
			t.Fatalf("a manual assessment never refuses, got %v", err)
		}
		wantIDs := []string{idPassed, idFailed, idHalted, idWaiting, idSchema1}
		if len(out.Findings) != len(wantIDs) {
			t.Fatalf("want %d findings, got %d: %+v", len(wantIDs), len(out.Findings), out.Findings)
		}
		for i, id := range wantIDs {
			if out.Findings[i].DriveID != id {
				t.Errorf("finding %d DriveID = %q, want %q (order must be ascending by id)", i, out.Findings[i].DriveID, id)
			}
		}
		wantClass := []string{LegacyNonblocking, LegacyNonblocking, LegacyRecovered, LegacyRetained, LegacyRetained}
		for i, c := range wantClass {
			if out.Findings[i].Class != c {
				t.Errorf("finding %d Class = %q, want %q (reason %q)", i, out.Findings[i].Class, c, out.Findings[i].Reason)
			}
		}
		if !strings.Contains(out.Findings[3].Reason, "nonterminal execution state") {
			t.Errorf("waiting reason = %q, want it to contain nonterminal execution state", out.Findings[3].Reason)
		}
		if !strings.Contains(out.Findings[4].Reason, "unknown schema") {
			t.Errorf("v1 reason = %q, want it to contain unknown schema", out.Findings[4].Reason)
		}
		if out.Checked != 5 || out.Recovered != 1 || out.Recoverable != 0 || out.Retained != 2 || out.Nonblocking != 2 {
			t.Fatalf("counts Checked=%d Recovered=%d Recoverable=%d Retained=%d Nonblocking=%d; want 5/1/0/2/2",
				out.Checked, out.Recovered, out.Recoverable, out.Retained, out.Nonblocking)
		}
	})

	// Case 2: DryRun over the same shape previews the halted drive as recoverable and
	// writes no marker — the seam sees mark=false only.
	t.Run("2-dry-run-previews-without-marking", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		copyLegacyFixture(t, s, "passed")
		copyLegacyFixture(t, s, "failed")
		idHalted := copyLegacyFixture(t, s, "halted")
		copyLegacyFixture(t, s, "waiting")
		copyLegacyFixture(t, s, "schema1")
		seam := seamWith(map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "abandonable"}})

		out, err := s.cleanupHistory(HistoryCleanupRequest{DryRun: true}, seam)
		if err != nil {
			t.Fatalf("dry run never refuses, got %v", err)
		}
		if out.Recoverable != 1 || out.Recovered != 0 {
			t.Fatalf("dry run must preview: Recoverable=%d Recovered=%d, want 1/0", out.Recoverable, out.Recovered)
		}
		var haltedF LegacyFinding
		for _, f := range out.Findings {
			if f.DriveID == idHalted {
				haltedF = f
			}
		}
		if haltedF.Class != LegacyRecoverable {
			t.Fatalf("halted finding class = %q, want %q", haltedF.Class, LegacyRecoverable)
		}
		if len(seam.marks) != 0 {
			t.Fatalf("dry run must write no marker, seam recorded marks %v", seam.marks)
		}
	})

	// Case 3: an applied run recovers the halted drive; a second applied run — the
	// seam now reporting the durable already-abandoned marker — reports it nonblocking
	// and writes zero additional markers. Idempotent.
	t.Run("3-applied-twice-idempotent", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		idHalted := copyLegacyFixture(t, s, "halted")
		seam := &fakeRecovery{entries: map[string]process.RecoveryEntry{haltedFixtureRunDir: {Disposition: "abandonable"}}}

		first, err := s.cleanupHistory(HistoryCleanupRequest{}, seam)
		if err != nil {
			t.Fatal(err)
		}
		if first.Recovered != 1 {
			t.Fatalf("first apply must recover, got %+v", first)
		}
		if len(seam.marks) != 1 || seam.marks[0] != haltedFixtureRunDir {
			t.Fatalf("first apply must write exactly one marker, got %v", seam.marks)
		}
		firstMarks := len(seam.marks)

		// The durable teardown marker is now present: the seam reports already-abandoned.
		seam.entries[haltedFixtureRunDir] = process.RecoveryEntry{Disposition: "already-abandoned"}
		second, err := s.cleanupHistory(HistoryCleanupRequest{}, seam)
		if err != nil {
			t.Fatal(err)
		}
		var haltedF LegacyFinding
		for _, f := range second.Findings {
			if f.DriveID == idHalted {
				haltedF = f
			}
		}
		if haltedF.Class != LegacyNonblocking || !strings.Contains(haltedF.Reason, "durable teardown evidence") {
			t.Fatalf("second run must be nonblocking with durable teardown evidence, got %+v", haltedF)
		}
		if len(seam.marks) != firstMarks {
			t.Fatalf("second run must write zero additional markers: marks grew %d -> %d", firstMarks, len(seam.marks))
		}
	})

	// Case 4: a non-empty DriveID assesses exactly that one record; a traversal id is
	// a typed ErrInvalidID rejected before anything is scanned.
	t.Run("4-single-drive-id-and-invalid-id", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		copyLegacyFixture(t, s, "passed")
		idFailed := copyLegacyFixture(t, s, "failed")
		copyLegacyFixture(t, s, "waiting")

		out, err := s.cleanupHistory(HistoryCleanupRequest{DriveID: idFailed}, seamWith(nil))
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Findings) != 1 || out.Findings[0].DriveID != idFailed {
			t.Fatalf("a DriveID scope must yield exactly that one finding, got %+v", out.Findings)
		}
		if out.Findings[0].Class != LegacyNonblocking || out.Checked != 1 || out.Nonblocking != 1 {
			t.Fatalf("failed drive: Class=%q Checked=%d Nonblocking=%d, want nonblocking/1/1", out.Findings[0].Class, out.Checked, out.Nonblocking)
		}

		if _, err := s.cleanupHistory(HistoryCleanupRequest{DriveID: "../evil"}, seamWith(nil)); !isStoreKind(err, ErrInvalidID) {
			t.Fatalf("a traversal drive id must be a typed ErrInvalidID, got %v", err)
		}
	})

	// Case 5: an empty/absent registry root yields zero findings and no error.
	t.Run("5-empty-registry-no-error", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t)) // root directory never created
		out, err := s.cleanupHistory(HistoryCleanupRequest{}, seamWith(nil))
		if err != nil {
			t.Fatalf("an absent registry root is not an error, got %v", err)
		}
		if len(out.Findings) != 0 || out.Checked != 0 {
			t.Fatalf("an empty registry yields no findings, got %+v Checked=%d", out.Findings, out.Checked)
		}
	})

	// Case 6: a record-less drive directory is skipped (a concurrent first-admission
	// creation window) and never counted, so only the real record remains.
	t.Run("6-recordless-dir-skipped", func(t *testing.T) {
		s := OpenStore(testsupport.TempDir(t))
		idPassed := copyLegacyFixture(t, s, "passed")
		const recordlessID = "abcdef0123456789abcdef0123456789"
		if err := os.MkdirAll(filepath.Join(s.root, recordlessID), 0o700); err != nil {
			t.Fatal(err)
		}
		out, err := s.cleanupHistory(HistoryCleanupRequest{}, seamWith(nil))
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Findings) != 1 || out.Findings[0].DriveID != idPassed {
			t.Fatalf("a record-less dir must be skipped, leaving only the real record, got %+v", out.Findings)
		}
		if out.Findings[0].Class != LegacyNonblocking || out.Checked != 1 {
			t.Fatalf("only the real drive counts, got Class=%q Checked=%d", out.Findings[0].Class, out.Checked)
		}
	})
}

// TestDriverCleanupHistoryDelegates proves Driver.CleanupHistory runs the shared
// assessment over the driver's own store with its own process seam. A passed
// fixture is nonblocking without ever consulting the seam, so the default fakeProc
// suffices to prove the wiring.
func TestDriverCleanupHistoryDelegates(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
	id := copyLegacyFixture(t, store, "passed")
	out, err := d.CleanupHistory(HistoryCleanupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 1 || out.Findings[0].DriveID != id || out.Findings[0].Class != LegacyNonblocking {
		t.Fatalf("Driver.CleanupHistory must delegate to the shared assessment, got %+v", out.Findings)
	}
}
