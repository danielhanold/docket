package suiterunner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// ctxKey renders a context key for the tests' fixed execution context (j8/c8,
// Darwin/arm64), so a record and the observations that feed it agree on the key.
func ctxKey(path string, ceil int) string {
	return ContextKey(path, 8, 8, "Darwin", "arm64", ceil, ModeParallel)
}

// writeDurations drops a DOCKET_RUNTESTS_TEST_DURATIONS TSV (base.sh<TAB>parallel<TAB>solo)
// and returns its path, so a solo confirmation reaches a deterministic verdict
// without sleeping (column 3 is the injected solo seconds).
func writeDurations(t *testing.T, rows [][3]string) string {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r[0] + "\t" + r[1] + "\t" + r[2] + "\n")
	}
	p := filepath.Join(testsupport.TempDir(t), "durations.tsv")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write durations: %v", err)
	}
	return p
}

// setLockTiming shrinks the advisory-lock retry budget so the fail-open lock
// tests do not spend the production ~3s. Restores on the returned func.
func setLockTiming(attempts int, interval time.Duration) func() {
	oa, oi := lockAttempts, lockInterval
	lockAttempts, lockInterval = attempts, interval
	return func() { lockAttempts, lockInterval = oa, oi }
}

func TestContextKeyRendersOracleFormat(t *testing.T) {
	got := ContextKey("tests/test_a.sh", 8, 8, "Darwin", "arm64", 60, ModeParallel)
	want := "tests/test_a.sh|j8|c8|Darwin|arm64|b60|mparallel|s1"
	if got != want {
		t.Fatalf("ContextKey = %q, want %q", got, want)
	}
}

func TestStreakArmsAtFiveAndCleanResets(t *testing.T) {
	s := newBudgetState(8)
	key := ctxKey("tests/test_a.sh", 60)
	over := ScreenObs{Key: key, Path: "tests/test_a.sh", Ceiling: 60, Secs: 200, Over: true}
	clean := ScreenObs{Key: key, Path: "tests/test_a.sh", Ceiling: 60, Secs: 10, Over: false}

	for i := 0; i < 4; i++ {
		s.ApplyScreenObservations([]ScreenObs{over})
	}
	if r := s.records[key]; r.streak != 4 || r.dueSeq != "-" {
		t.Fatalf("after 4 overruns streak=%d due=%q; want streak 4, not due", r.streak, r.dueSeq)
	}
	// A clean measurement in watching resets the streak AND drops any due stamp.
	s.ApplyScreenObservations([]ScreenObs{clean})
	if r := s.records[key]; r.streak != 0 || r.dueSeq != "-" {
		t.Fatalf("after clean streak=%d due=%q; want reset to 0, not due", r.streak, r.dueSeq)
	}
	for i := 0; i < 5; i++ {
		s.ApplyScreenObservations([]ScreenObs{over})
	}
	r := s.records[key]
	if r.streak != 5 || r.dueSeq == "-" {
		t.Fatalf("after 5 consecutive overruns streak=%d due=%q; want streak 5, due", r.streak, r.dueSeq)
	}
	if r.dueSeq != "1" || s.nextSeq != 2 {
		t.Fatalf("due stamped exactly once: dueSeq=%q nextSeq=%d; want dueSeq 1, nextSeq 2", r.dueSeq, s.nextSeq)
	}
}

// TestSensitiveSinceCounterNeverResetsOnClean is a named mutation guard: the
// recheck since-counter for a parallel-sensitive/confirmed-breach record must
// accumulate across cleans and never reset, or a required serial confirmation is
// silently skipped. Mutating the clean branch to reset since (the asymmetry with
// the streak) reddens this.
func TestSensitiveSinceCounterNeverResetsOnClean(t *testing.T) {
	s := newBudgetState(8)
	key := ctxKey("tests/test_s.sh", 60)
	s.records[key] = &bsRecord{state: "parallel-sensitive", ceiling: 60, path: "tests/test_s.sh", lastPar: "-", lastSolo: "-", confRes: "cleared", dueSeq: "-"}
	over := ScreenObs{Key: key, Path: "tests/test_s.sh", Ceiling: 60, Secs: 200, Over: true}
	clean := ScreenObs{Key: key, Path: "tests/test_s.sh", Ceiling: 60, Secs: 10, Over: false}

	for i := 0; i < 9; i++ {
		s.ApplyScreenObservations([]ScreenObs{over})
		s.ApplyScreenObservations([]ScreenObs{clean})
	}
	if r := s.records[key]; r.since != 9 || r.dueSeq != "-" {
		t.Fatalf("interleaved 9 overruns/cleans: since=%d due=%q; want since 9 (cleans must not reset), not due", r.since, r.dueSeq)
	}
	s.ApplyScreenObservations([]ScreenObs{over})
	if r := s.records[key]; r.since != 10 || r.dueSeq == "-" {
		t.Fatalf("10th overrun: since=%d due=%q; want since 10, due", r.since, r.dueSeq)
	}
}

// TestConfirmationPrecedesAuthoritativeBreach is a named mutation guard: a
// parallel screening crossing is never authoritative — only a solo confirmation
// (announced by SERIAL CONFIRMATION DUE) may declare SERIAL CONFIRMED OVER
// BUDGET, and the DUE line MUST precede the breach. Mutating the scheduler to
// declare a breach from the contended measurement (skipping the confirmation)
// reddens this.
func TestConfirmationPrecedesAuthoritativeBreach(t *testing.T) {
	scripts := testsupport.TempDir(t)
	tgt := writeScript(t, scripts, "cpb", "echo 'ok - cpb'\n")
	ceil := 10
	key := ctxKey(tgt.Path, ceil)

	s := newBudgetState(8)
	over := ScreenObs{Key: key, Path: tgt.Path, Ceiling: ceil, Secs: 30, Over: true}
	for i := 0; i < 5; i++ {
		s.ApplyScreenObservations([]ScreenObs{over})
	}
	if s.records[key].dueSeq == "-" {
		t.Fatal("record must be due before confirmation")
	}
	// Injected solo 20s over ceiling*3/2 = 15s -> a confirmed breach.
	dur := writeDurations(t, [][3]string{{"test_cpb.sh", "30", "20"}})
	cfg := Config{Bash: bashPath(t), Work: testsupport.TempDir(t), DurationsPath: dur}

	var buf bytes.Buffer
	s.ScheduleConfirmation(context.Background(), cfg, &buf)
	out := buf.String()

	di := strings.Index(out, "SERIAL CONFIRMATION DUE: "+tgt.Path)
	bi := strings.Index(out, "SERIAL CONFIRMED OVER BUDGET: "+tgt.Path)
	if di < 0 || bi < 0 || di >= bi {
		t.Fatalf("DUE must precede CONFIRMED OVER BUDGET (di=%d bi=%d):\n%s", di, bi, out)
	}
	if !strings.Contains(out, "30s under -j8") || !strings.Contains(out, "20s solo") || !strings.Contains(out, "solo threshold 15s") {
		t.Fatalf("breach line must carry parallel secs, solo secs, and threshold:\n%s", out)
	}
	if r := s.records[key]; r.state != "confirmed-breach" || r.confRes != "breached" || r.dueSeq != "-" {
		t.Fatalf("post-confirmation record: state=%q confRes=%q due=%q; want confirmed-breach/breached/not-due", r.state, r.confRes, r.dueSeq)
	}
}

func TestFailedConfirmationClearsNothing(t *testing.T) {
	scripts := testsupport.TempDir(t)
	tgt := writeScript(t, scripts, "fcc", "echo 'NOT OK - boom'\nexit 2\n")
	ceil := 10
	key := ctxKey(tgt.Path, ceil)

	s := newBudgetState(8)
	over := ScreenObs{Key: key, Path: tgt.Path, Ceiling: ceil, Secs: 30, Over: true}
	for i := 0; i < 5; i++ {
		s.ApplyScreenObservations([]ScreenObs{over})
	}
	dueBefore := s.records[key].dueSeq
	streakBefore := s.records[key].streak
	cfg := Config{Bash: bashPath(t), Work: testsupport.TempDir(t)}

	var buf bytes.Buffer
	s.ScheduleConfirmation(context.Background(), cfg, &buf)
	out := buf.String()

	if !strings.Contains(out, "SERIAL CONFIRMATION FAILED: "+tgt.Path) {
		t.Fatalf("want SERIAL CONFIRMATION FAILED:\n%s", out)
	}
	r := s.records[key]
	if r.dueSeq == "-" || r.dueSeq != dueBefore {
		t.Fatalf("failed confirmation must leave the candidate due: due=%q before=%q", r.dueSeq, dueBefore)
	}
	if r.streak != streakBefore {
		t.Fatalf("failed confirmation must not touch the streak: %d != %d", r.streak, streakBefore)
	}
	if r.confRes != "failed" {
		t.Fatalf("last_confirmation_result = %q, want failed", r.confRes)
	}
}

func TestOnePerRunAndDeferredLines(t *testing.T) {
	scripts := testsupport.TempDir(t)
	t1 := writeScript(t, scripts, "one", "echo 'ok - one'\n")
	t2 := writeScript(t, scripts, "two", "echo 'ok - two'\n")
	ceil := 10
	k1 := ctxKey(t1.Path, ceil)
	k2 := ctxKey(t2.Path, ceil)

	s := newBudgetState(8)
	o1 := ScreenObs{Key: k1, Path: t1.Path, Ceiling: ceil, Secs: 30, Over: true}
	o2 := ScreenObs{Key: k2, Path: t2.Path, Ceiling: ceil, Secs: 30, Over: true}
	for i := 0; i < 5; i++ {
		s.ApplyScreenObservations([]ScreenObs{o1, o2})
	}
	cfg := Config{Bash: bashPath(t), Work: testsupport.TempDir(t)}

	var buf bytes.Buffer
	s.ScheduleConfirmation(context.Background(), cfg, &buf)
	out := buf.String()

	if n := strings.Count(out, "SERIAL CONFIRMATION DUE:"); n != 1 {
		t.Fatalf("exactly one DUE per run, got %d:\n%s", n, out)
	}
	if n := strings.Count(out, "SERIAL CONFIRMATION DEFERRED:"); n != 1 {
		t.Fatalf("exactly one DEFERRED, got %d:\n%s", n, out)
	}
	// test_one.sh sorts before test_two.sh and is stamped first, so it is chosen;
	// test_two.sh is deferred with the verbatim wording.
	want := "SERIAL CONFIRMATION DEFERRED: " + t2.Path + " — Recheck is due; another test consumed this run's confirmation slot"
	if !strings.Contains(out, want) {
		t.Fatalf("verbatim deferred wording missing:\nwant line: %s\ngot:\n%s", want, out)
	}
}

// TestScreeningNeverAuthoritative is a named mutation guard: a screening
// candidate is reported (BUDGET WATCH) but never labeled OVER BUDGET and never
// gates the exit. Mutating EmitScreenReport to treat a parallel screen crossing
// as authoritative (an OVER BUDGET line) reddens this.
func TestScreeningNeverAuthoritative(t *testing.T) {
	s := newBudgetState(8)
	key := ctxKey("tests/test_scr.sh", 60)
	over := ScreenObs{Key: key, Path: "tests/test_scr.sh", Ceiling: 60, Secs: 200, Over: true}
	s.ApplyScreenObservations([]ScreenObs{over})

	var buf bytes.Buffer
	s.EmitScreenReport(&buf)
	out := buf.String()

	if !strings.Contains(out, "BUDGET WATCH: tests/test_scr.sh") {
		t.Fatalf("want a BUDGET WATCH line:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "OVER BUDGET") {
			t.Fatalf("a parallel screen crossing must never be authoritative: %q", line)
		}
	}
	if code := ExitCode(Tally{Files: 1, Passed: 1}, 0, false); code != 0 {
		t.Fatalf("screening advisory must exit 0, got %d", code)
	}
}

func TestStrictConfirmsAllAndArms(t *testing.T) {
	scripts := testsupport.TempDir(t)
	a := writeScript(t, scripts, "sa", "echo 'ok - sa'\n")
	b := writeScript(t, scripts, "sb", "echo 'ok - sb'\n")
	ceil := 10
	ka := ctxKey(a.Path, ceil)
	kb := ctxKey(b.Path, ceil)

	s := newBudgetState(8)
	s.candidates = []ScreenObs{
		{Key: ka, Path: a.Path, Ceiling: ceil, Secs: 30, Over: true},
		{Key: kb, Path: b.Path, Ceiling: ceil, Secs: 30, Over: true},
	}
	// a breaches (solo 20 > 15), b clears (solo 1 <= 15).
	dur := writeDurations(t, [][3]string{{"test_sa.sh", "30", "20"}, {"test_sb.sh", "30", "1"}})
	cfg := Config{Bash: bashPath(t), Work: testsupport.TempDir(t), DurationsPath: dur}

	var buf bytes.Buffer
	armed := s.StrictConfirmCandidates(context.Background(), cfg, &buf)
	out := buf.String()

	if n := strings.Count(out, "SERIAL CONFIRMATION DUE:"); n != 2 {
		t.Fatalf("strict confirms EVERY candidate: want 2 DUE, got %d:\n%s", n, out)
	}
	if !armed {
		t.Fatal("a confirmed breach must arm strict (exit 4)")
	}
	if !strings.Contains(out, "SERIAL CONFIRMED OVER BUDGET: "+a.Path) {
		t.Fatalf("the breached candidate must be reported:\n%s", out)
	}
	// Strict persists only confirmation outcomes, never screening counters.
	if r := s.records[ka]; r.streak != 0 || r.since != 0 {
		t.Fatalf("strict must not advance screening counters: streak=%d since=%d", r.streak, r.since)
	}
	if r := s.records[kb]; r.streak != 0 || r.since != 0 {
		t.Fatalf("strict must not advance screening counters: streak=%d since=%d", r.streak, r.since)
	}
}

func TestStoreFailOpen(t *testing.T) {
	defer setLockTiming(2, time.Millisecond)()

	t.Run("corrupt header discards state", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		path := filepath.Join(dir, "state.tsv")
		if err := os.WriteFile(path, []byte("GARBAGE HEADER\njunk\tjunk\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var warn bytes.Buffer
		s := Load(path, 8, &warn)
		if len(s.records) != 0 {
			t.Fatalf("corrupt header must discard state, got %d records", len(s.records))
		}
		s.Save() // must not panic or fail the run
	})

	t.Run("held lock yields no history and a warning", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		path := filepath.Join(dir, "state.tsv")
		valid := "# docket-run-tests-budget-state v1\n# next_due_sequence 3\n" +
			"k1\twatching\t2\t0\t100\t-\t60\t-\t-\ttests/test_x.sh\n"
		if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path+".lock", 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(path + ".lock") })
		var warn bytes.Buffer
		s := Load(path, 8, &warn)
		if len(s.records) != 0 {
			t.Fatalf("a held lock reads no history, got %d records", len(s.records))
		}
		if !strings.Contains(warn.String(), "lock not acquired") {
			t.Fatalf("want a lock-not-acquired warning, got %q", warn.String())
		}
		s.Save() // an unlocked state writes nothing, fails nothing
	})

	t.Run("unwritable dir warns and fails open", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		ro := filepath.Join(testsupport.TempDir(t), "ro")
		if err := os.Mkdir(ro, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(ro, 0o755) })
		path := filepath.Join(ro, "state.tsv")
		var warn bytes.Buffer
		s := Load(path, 8, &warn)
		if len(s.records) != 0 {
			t.Fatalf("an unlockable store reads no history, got %d records", len(s.records))
		}
		s.Save() // must not panic; the run proceeds without budget history
	})
}

// TestStorePathIsNotTheBashRunners is the documented-intentional-deviation
// contract test: the Go runner's advisory store lives at its OWN default path,
// never the Bash runner's run-tests-budget-state.tsv, so two writers never
// corrupt one advisory file (change 0318).
func TestStorePathIsNotTheBashRunners(t *testing.T) {
	dir := testsupport.TempDir(t)
	cmd := exec.Command("git", "-C", dir, "init")
	// Background-off GIT_CONFIG_GLOBAL (testsupport.GitEnv) so this init's git
	// spawns no detached housekeeping child that races the fixture's RemoveAll
	// into "directory not empty" under parallel load (change 0373).
	cmd.Env = append(os.Environ(), testsupport.GitEnv(t)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	p, err := DefaultStatePath(dir)
	if err != nil {
		t.Fatalf("DefaultStatePath: %v", err)
	}
	if !strings.HasSuffix(p, "/docket/development-test-budget-state.tsv") {
		t.Fatalf("default store path %q must end /docket/development-test-budget-state.tsv", p)
	}
	if strings.Contains(p, "run-tests-budget-state.tsv") {
		t.Fatalf("default store path %q must never be the Bash runner's store", p)
	}
}
