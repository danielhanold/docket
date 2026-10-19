package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// haltStoreFixture returns a halt-count store rooted in a fresh temp state folder.
// It is pure filesystem: nothing here starts git.
func haltStoreFixture(t *testing.T) haltCountStore {
	t.Helper()
	s := haltCountStoreAt(t.TempDir())
	if filepath.Base(s.dir) != runTrackerHaltsDirName {
		t.Fatalf("store dir = %q, want it to end in %q", s.dir, runTrackerHaltsDirName)
	}
	return s
}

func mustRecordHalt(t *testing.T, s haltCountStore, key string, loopPicked bool, limit int) haltCounted {
	t.Helper()
	out, err := s.recordHalt(key, loopPicked, limit)
	if err != nil {
		t.Fatalf("recordHalt(%q, %v, %d): %v", key, loopPicked, limit, err)
	}
	return out
}

func mustHaltCount(t *testing.T, s haltCountStore, want int) {
	t.Helper()
	got, err := s.count()
	if err != nil {
		t.Fatalf("count(): %v", err)
	}
	if got != want {
		t.Fatalf("count() = %d, want %d", got, want)
	}
}

func TestHaltCountRecordHaltUnderLimitIsRunNext(t *testing.T) {
	s := haltStoreFixture(t)
	out := mustRecordHalt(t, s, "k1", true, 2)
	if out.Decision != RunDecisionNext || out.HaltsInARow != 1 || out.HaltLimit != 2 || out.Noted {
		t.Fatalf("recordHalt = %+v, want run-next at 1 of 2, not noted", out)
	}
	mustHaltCount(t, s, 1)
}

func TestHaltCountRecordHaltAtLimitIsRunStopNoted(t *testing.T) {
	s := haltStoreFixture(t)
	mustRecordHalt(t, s, "k1", true, 2)
	out := mustRecordHalt(t, s, "k2", true, 2)
	if out.Decision != RunDecisionStop || !out.Noted || out.HaltsInARow != 2 || out.HaltLimit != 2 {
		t.Fatalf("recordHalt at limit = %+v, want run-stop noted at 2 of 2", out)
	}
	mustHaltCount(t, s, 2)
}

func TestHaltCountRecordHaltOncePerKey(t *testing.T) {
	s := haltStoreFixture(t)
	first := mustRecordHalt(t, s, "k1", true, 2)
	second := mustRecordHalt(t, s, "k1", true, 2)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replay = %+v, want the saved %+v", second, first)
	}
	mustHaltCount(t, s, 1)

	if _, _, err := s.resetAll(); err != nil {
		t.Fatalf("resetAll: %v", err)
	}
	// A replay after the count moved returns the SAVED decision, never a fresh one,
	// and never counts the key a second time.
	replay := mustRecordHalt(t, s, "k1", true, 2)
	if replay.Decision != RunDecisionNext || replay.HaltsInARow != 1 || !reflect.DeepEqual(replay, first) {
		t.Fatalf("replay after reset = %+v, want the saved %+v", replay, first)
	}
	mustHaltCount(t, s, 0)
}

func TestHaltCountUnknownModeCountsWithoutDecision(t *testing.T) {
	s := haltStoreFixture(t)
	out := mustRecordHalt(t, s, "k1", false, 0)
	if out.Decision != "" || out.HaltsInARow != 1 || out.HaltLimit != 0 || out.Noted {
		t.Fatalf("unknown recordHalt = %+v, want no decision at 1", out)
	}
	mustHaltCount(t, s, 1)
}

func TestHaltCountLimitBelowCountStops(t *testing.T) {
	s := haltStoreFixture(t)
	for _, k := range []string{"k1", "k2", "k3"} {
		mustRecordHalt(t, s, k, false, 0)
	}
	out := mustRecordHalt(t, s, "k4", true, 2)
	if out.Decision != RunDecisionStop || !out.Noted || out.HaltsInARow != 4 {
		t.Fatalf("recordHalt above a lowered limit = %+v, want run-stop noted at 4", out)
	}
}

func TestHaltCountRecordResetOncePerKey(t *testing.T) {
	s := haltStoreFixture(t)
	mustRecordHalt(t, s, "k1", true, 5)
	mustRecordHalt(t, s, "k2", true, 5)
	mustHaltCount(t, s, 2)
	if err := s.recordReset("c1"); err != nil {
		t.Fatalf("recordReset: %v", err)
	}
	mustHaltCount(t, s, 0)
	mustRecordHalt(t, s, "k3", true, 5)
	mustHaltCount(t, s, 1)
	// An old run-complete replayed never re-zeroes the count.
	if err := s.recordReset("c1"); err != nil {
		t.Fatalf("replayed recordReset: %v", err)
	}
	mustHaltCount(t, s, 1)
}

func TestHaltCountCorruptCountIsAFault(t *testing.T) {
	s := haltStoreFixture(t)
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	countPath := filepath.Join(s.dir, haltCountFileName)
	if err := os.WriteFile(countPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.recordHalt("k1", true, 2); err == nil {
		t.Fatal("recordHalt over a corrupt count succeeded, want a fault (never read as 0)")
	}
	if got, _ := os.ReadFile(countPath); string(got) != "{" {
		t.Fatalf("count.json = %q after a faulted recordHalt, want it untouched", got)
	}
	if _, err := os.Stat(filepath.Join(s.dir, haltCountedDirName, "k1"+haltMarkerSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a faulted recordHalt left a marker (stat err %v)", err)
	}
	if _, err := s.count(); err == nil {
		t.Fatal("count() over a corrupt count succeeded, want an error")
	}

	previous, unreadable, err := s.resetAll()
	if err != nil || !unreadable || previous != 0 {
		t.Fatalf("resetAll = (%d, %v, %v), want (0, true, nil)", previous, unreadable, err)
	}
	mustHaltCount(t, s, 0)
}

func TestHaltCountRejectsOutOfShapeCounts(t *testing.T) {
	for name, body := range map[string]string{
		"wrong schema":   `{"schema":2,"halts_in_a_row":1}`,
		"negative":       `{"schema":1,"halts_in_a_row":-1}`,
		"missing count":  `{"schema":1}`,
		"missing schema": `{"halts_in_a_row":1}`,
		"trailing data":  `{"schema":1,"halts_in_a_row":1} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := haltStoreFixture(t)
			if err := os.MkdirAll(s.dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.dir, haltCountFileName), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.count(); err == nil {
				t.Fatalf("count() over %s succeeded, want an error", body)
			}
			if _, err := s.recordHalt("k1", true, 2); err == nil {
				t.Fatalf("recordHalt over %s succeeded, want a fault", body)
			}
		})
	}
}

func TestHaltCountLockIsBounded(t *testing.T) {
	saved := haltLockWait
	haltLockWait = 100 * time.Millisecond
	t.Cleanup(func() { haltLockWait = saved })

	s := haltStoreFixture(t)
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(s.dir, haltLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = s.recordHalt("k1", true, 2)
	if !errors.Is(err, errHaltCountBusy) {
		t.Fatalf("recordHalt under a held lock = %v, want errHaltCountBusy", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("recordHalt waited %v under a held lock, want it bounded near %v", elapsed, haltLockWait)
	}
	if err := s.recordReset("c1"); !errors.Is(err, errHaltCountBusy) {
		t.Fatalf("recordReset under a held lock = %v, want errHaltCountBusy", err)
	}
	if _, _, err := s.resetAll(); !errors.Is(err, errHaltCountBusy) {
		t.Fatalf("resetAll under a held lock = %v, want errHaltCountBusy", err)
	}
	mustHaltCount(t, s, 0)
}

func TestHaltCountConcurrentDistinctKeysAllCount(t *testing.T) {
	s := haltStoreFixture(t)
	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.recordHalt(fmt.Sprintf("k%d", i), true, 100); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent recordHalt: %v", err)
	}
	mustHaltCount(t, s, n)
}

func TestHaltCountRejectsMalformedKey(t *testing.T) {
	s := haltStoreFixture(t)
	if _, err := s.recordHalt("../x", true, 2); err == nil {
		t.Fatal("recordHalt accepted a malformed key")
	}
	if err := s.recordReset("../x"); err == nil {
		t.Fatal("recordReset accepted a malformed key")
	}
	if _, err := os.Stat(filepath.Join(s.dir, haltCountFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a malformed key wrote a count (stat err %v)", err)
	}
}

// TestRunVerdictHumanTextAppendsNote pins the report rendering: the note rides on a
// second line of HumanText, while reportLine — what the record persists — stays the
// one report line.
func TestRunVerdictHumanTextAppendsNote(t *testing.T) {
	r := RunVerdictResult{Decision: RunDecisionNext, Key: "k", Outcome: VerdictRunHalted, AttributedID: 3}
	if got, want := r.HumanText(), "run-next k run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if r.reportLine() != r.HumanText() {
		t.Fatalf("without a note reportLine %q != HumanText %q", r.reportLine(), r.HumanText())
	}
	r.Decision, r.Note = RunDecisionStop, "note: x"
	if got, want := r.HumanText(), "run-stop k run-halted 3\nnote: x"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if got, want := r.reportLine(), "run-stop k run-halted 3"; got != want {
		t.Fatalf("reportLine = %q, want %q", got, want)
	}
}

// TestHaltLimitNoteNamesTheResetAndForbidsDoingItAlone pins the at-limit note: one
// line naming the count, the limit, the setting and the reset operation, and
// forbidding a reset without the person's yes.
func TestHaltLimitNoteNamesTheResetAndForbidsDoingItAlone(t *testing.T) {
	n := haltLimitNote(2, 2)
	for _, want := range []string{"note: ", "2 changes picked from the queue", "limit of 2", "run.max_consecutive_halts", "run.reset-halts", "Never reset it on your own"} {
		if !strings.Contains(n, want) {
			t.Errorf("note %q lacks %q", n, want)
		}
	}
	if strings.Contains(n, "\n") {
		t.Errorf("note %q spans more than one line", n)
	}
	if one := haltLimitNote(1, 1); !strings.Contains(one, "1 change picked from the queue has halted") || !strings.Contains(one, "limit of 1") {
		t.Errorf("note at limit 1 = %q, want the singular wording", one)
	}
}
