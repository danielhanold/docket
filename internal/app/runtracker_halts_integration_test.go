//go:build integration

package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// These are the keyed run.verdict tests for the halt count: a halt on a change a
// loop picked from the queue prints run-next while the count of halts in a row
// stays under run.max_consecutive_halts, and run-stop plus a reset note at it.
// Every other path — a named run, a resume, an unknown-mode claim, a count fault,
// observe mode — prints exactly today's line, and only a clean build resets the
// count. The halts are reader-only RunVerify short-circuits over a "## Run halted"
// in-progress blob (runTrackerHaltedInProgressBlob).

var haltLoopRunSeq atomic.Int64

// haltLoopRun mints a fresh started run for change id whose confirmed binding carries
// selection, and returns its key plus a WorkspaceDeps whose proof scanner names that
// claim. An empty selection reserves through the legacy ReserveRunTrackerClaim, the
// shape every binding written before the selection field had.
func haltLoopRun(t *testing.T, repo string, id int, selection string) (string, WorkspaceDeps) {
	t.Helper()
	hash := "halt-hash-" + strconv.FormatInt(haltLoopRunSeq.Add(1), 10)
	key := runTrackerMintStarted(t, repo, nil, 1, hash)
	requestID := "claim-" + strconv.Itoa(id) + "-" + key
	if selection == "" {
		must(t, ReserveRunTrackerClaim(repo, key, id, requestID))
	} else {
		must(t, ReserveRunTrackerClaimSelection(repo, key, id, requestID, selection))
	}
	revision := "r" + strconv.Itoa(id)
	must(t, ConfirmRunTrackerClaim(repo, key, id, requestID, revision, ""))
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: requestID, ChangeID: id, RunContextHash: hash, Revision: revision},
	}}}
	return key, wdeps
}

// haltVerdictDeps is the reader-only deps over halted changes 3 and 4, with the
// built-in run.max_consecutive_halts (2).
func haltVerdictDeps(t *testing.T) PlanningDeps {
	t.Helper()
	return runTrackerLightDeps(t, []StatusBlob{runTrackerHaltedInProgressBlob(3, rvSlug), runTrackerHaltedInProgressBlob(4, "other-change")})
}

// haltVerdictDepsLimit is haltVerdictDeps with run.max_consecutive_halts pinned to limit.
func haltVerdictDepsLimit(t *testing.T, limit int) PlanningDeps {
	t.Helper()
	pin := mainPin(t)
	pin.Config.Effective.Run.MaxConsecutiveHalts.Value = limit
	return PlanningDeps{Reader: &fakeReader{pin: pin, corpus: []StatusBlob{
		runTrackerHaltedInProgressBlob(3, rvSlug), runTrackerHaltedInProgressBlob(4, "other-change"),
	}}, Clock: testClock()}
}

func haltStoreOf(t *testing.T, repo string) haltCountStore {
	t.Helper()
	s, err := openHaltCountStore(repo)
	if err != nil {
		t.Fatalf("openHaltCountStore: %v", err)
	}
	return s
}

func haltCountOf(t *testing.T, repo string) int {
	t.Helper()
	n, err := haltStoreOf(t, repo).count()
	if err != nil {
		t.Fatalf("count(): %v", err)
	}
	return n
}

// seedHaltCount counts n unknown-mode halts under keys s0..s<n-1>.
func seedHaltCount(t *testing.T, repo string, n int) {
	t.Helper()
	s := haltStoreOf(t, repo)
	for i := 0; i < n; i++ {
		if _, err := s.recordHalt("s"+strconv.Itoa(i), false, 0); err != nil {
			t.Fatalf("seed recordHalt: %v", err)
		}
	}
	if got := haltCountOf(t, repo); got != n {
		t.Fatalf("seeded count = %d, want %d", got, n)
	}
}

// haltsDirAbsent asserts that nothing ever created the repository's halt-count folder.
func haltsDirAbsent(t *testing.T, repo string) {
	t.Helper()
	s := haltStoreOf(t, repo)
	if _, err := os.Stat(s.dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("halt-count folder %s exists (stat err %v): the count was opened", s.dir, err)
	}
}

func persistedDisposition(t *testing.T, repo, key string) string {
	t.Helper()
	rec, err := LoadRunTrackerRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	return rec.Disposition
}

func TestIntegrationRunVerdictHaltLoopPickedUnderLimitPrintsRunNext(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	if _, err := MintRunRecord(repo, key, "3"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}

	res := RunVerdict(context.Background(), haltVerdictDeps(t), wdeps, GitHubDeps{}, repo, key)
	want := "run-next " + key + " run-halted 3"
	if got := res.HumanText(); got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal || res.HaltsInARow != 1 || res.HaltLimit != 2 || res.Note != "" {
		t.Fatalf("terminal/halts/limit/note = %v/%d/%d/%q, want true/1/2/empty", res.Terminal, res.HaltsInARow, res.HaltLimit, res.Note)
	}
	if got := persistedDisposition(t, repo, key); got != want {
		t.Fatalf("persisted Disposition = %q, want %q", got, want)
	}
	used, err := RunTrackerRetryUsage(repo, key)
	if err != nil || used != 0 {
		t.Fatalf("RunTrackerRetryUsage = %d, %v; a halt must never spend a retry", used, err)
	}
	if st := loadRunState(t, repo, key); st != RunActive {
		t.Fatalf("run state = %q, want active (a halted run waits for a person)", st)
	}
	if got := haltCountOf(t, repo); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}

func TestIntegrationRunVerdictHaltLoopPickedAtLimitStopsWithNote(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := haltVerdictDeps(t)
	k1, w1 := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	if got := RunVerdict(context.Background(), deps, w1, GitHubDeps{}, repo, k1).HumanText(); got != "run-next "+k1+" run-halted 3" {
		t.Fatalf("first halt = %q, want run-next", got)
	}
	k2, w2 := haltLoopRun(t, repo, 4, ClaimSelectionQueue)
	res := RunVerdict(context.Background(), deps, w2, GitHubDeps{}, repo, k2)
	line := "run-stop " + k2 + " run-halted 4"
	if got := res.reportLine(); got != line {
		t.Fatalf("reportLine = %q, want %q", got, line)
	}
	if !strings.Contains(res.Note, "run.reset-halts") || !strings.Contains(res.Note, "Never") {
		t.Fatalf("Note = %q, want it to name run.reset-halts and forbid a reset without a yes", res.Note)
	}
	if got := res.HumanText(); got != line+"\n"+res.Note {
		t.Fatalf("HumanText = %q, want the line then the note", got)
	}
	if res.HaltsInARow != 2 || res.HaltLimit != 2 {
		t.Fatalf("halts/limit = %d/%d, want 2/2", res.HaltsInARow, res.HaltLimit)
	}
	if got := persistedDisposition(t, repo, k2); got != line {
		t.Fatalf("persisted Disposition = %q, want the report line only %q", got, line)
	}
}

func TestIntegrationRunVerdictHaltReplayNeverFlips(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := haltVerdictDeps(t)
	k1, w1 := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	first := RunVerdict(context.Background(), deps, w1, GitHubDeps{}, repo, k1)
	if first.Decision != RunDecisionNext {
		t.Fatalf("first k1 decision = %q, want run-next", first.Decision)
	}
	k2, w2 := haltLoopRun(t, repo, 4, ClaimSelectionQueue)
	atLimit := RunVerdict(context.Background(), deps, w2, GitHubDeps{}, repo, k2)
	if atLimit.Decision != RunDecisionStop || atLimit.Note == "" {
		t.Fatalf("first k2 = %q/%q, want run-stop with a note", atLimit.Decision, atLimit.Note)
	}

	if _, _, err := haltStoreOf(t, repo).resetAll(); err != nil {
		t.Fatalf("resetAll: %v", err)
	}
	// Replays after the count moved — and under a different limit — keep the saved line.
	for _, d := range []PlanningDeps{deps, haltVerdictDepsLimit(t, 1), haltVerdictDepsLimit(t, 9)} {
		if got := RunVerdict(context.Background(), d, w1, GitHubDeps{}, repo, k1); got.HumanText() != first.HumanText() || got.HaltsInARow != 1 {
			t.Fatalf("k1 replay = %q (halts %d), want %q (halts 1)", got.HumanText(), got.HaltsInARow, first.HumanText())
		}
		if got := RunVerdict(context.Background(), d, w2, GitHubDeps{}, repo, k2); got.HumanText() != atLimit.HumanText() || got.HaltsInARow != 2 {
			t.Fatalf("k2 replay = %q (halts %d), want %q (halts 2)", got.HumanText(), got.HaltsInARow, atLimit.HumanText())
		}
	}
	if got := haltCountOf(t, repo); got != 0 {
		t.Fatalf("count after replays = %d, want 0 (a replay never re-counts)", got)
	}
}

func TestIntegrationRunVerdictHaltNamedNeverTouchesCount(t *testing.T) {
	t.Run("high count", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		seedHaltCount(t, repo, 5)
		key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionNamed)
		res := RunVerdict(context.Background(), haltVerdictDeps(t), wdeps, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		if res.HaltsInARow != 0 || res.HaltLimit != 0 || res.Note != "" {
			t.Fatalf("named halt carried count fields: %d/%d/%q", res.HaltsInARow, res.HaltLimit, res.Note)
		}
		if got := haltCountOf(t, repo); got != 5 {
			t.Fatalf("count = %d, want 5 (a named halt never touches it)", got)
		}
	})
	t.Run("never opens the count", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionNamed)
		res := RunVerdict(context.Background(), haltVerdictDeps(t), wdeps, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		haltsDirAbsent(t, repo)
	})
	t.Run("resume-shaped record", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		key := runTrackerMintAttributedLimit(t, repo, 3, 2)
		res := RunVerdict(context.Background(), haltVerdictDeps(t), WorkspaceDeps{}, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		haltsDirAbsent(t, repo)
	})
}

func TestIntegrationRunVerdictHaltUnknownCountsButPrintsTodaysLine(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := haltVerdictDeps(t)
	for i, sel := range []string{"", "loop"} {
		key, wdeps := haltLoopRun(t, repo, 3, sel)
		res := RunVerdict(context.Background(), deps, wdeps, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
			t.Fatalf("selection %q: HumanText = %q, want %q", sel, got, want)
		}
		if res.HaltsInARow != 0 || res.HaltLimit != 0 || res.Note != "" {
			t.Fatalf("selection %q: unknown halt carried count fields: %d/%d/%q", sel, res.HaltsInARow, res.HaltLimit, res.Note)
		}
		if got := haltCountOf(t, repo); got != i+1 {
			t.Fatalf("selection %q: count = %d, want %d (an unknown halt still counts)", sel, got, i+1)
		}
	}
}

func TestIntegrationRunVerdictHaltCountFaultKeepsTodaysLine(t *testing.T) {
	repo := newRunTrackerRepo(t)
	s := haltStoreOf(t, repo)
	must(t, os.MkdirAll(s.dir, 0o700))
	countPath := filepath.Join(s.dir, haltCountFileName)
	must(t, os.WriteFile(countPath, []byte("{"), 0o600))
	deps := haltVerdictDeps(t)

	loopKey, loopW := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	loop := RunVerdict(context.Background(), deps, loopW, GitHubDeps{}, repo, loopKey)
	if loop.Decision != RunDecisionStop || loop.Outcome != VerdictRunHalted {
		t.Fatalf("decision/outcome = %q/%q, want run-stop/run-halted", loop.Decision, loop.Outcome)
	}
	if got, want := loop.HumanText(), "run-stop "+loopKey+" run-halted 3"; got != want {
		t.Fatalf("loop-picked HumanText = %q, want %q", got, want)
	}
	if loop.HaltsInARow != 0 || loop.Note != "" {
		t.Fatalf("a count fault carried count fields: %d/%q", loop.HaltsInARow, loop.Note)
	}

	namedKey, namedW := haltLoopRun(t, repo, 3, ClaimSelectionNamed)
	named := RunVerdict(context.Background(), deps, namedW, GitHubDeps{}, repo, namedKey)
	if got, want := named.HumanText(), "run-stop "+namedKey+" run-halted 3"; got != want {
		t.Fatalf("named HumanText = %q, want %q", got, want)
	}
	if strings.Replace(loop.HumanText(), loopKey, "K", 1) != strings.Replace(named.HumanText(), namedKey, "K", 1) {
		t.Fatalf("loop-picked and named lines differ under a fault: %q vs %q", loop.HumanText(), named.HumanText())
	}
	buf, err := os.ReadFile(countPath)
	if err != nil || string(buf) != "{" {
		t.Fatalf("count.json = %q, %v; a fault must leave it as it was", buf, err)
	}
}

func TestIntegrationRunVerdictHaltLockBusyKeepsTodaysLine(t *testing.T) {
	old := haltLockWait
	haltLockWait = 100 * time.Millisecond
	t.Cleanup(func() { haltLockWait = old })

	repo := newRunTrackerRepo(t)
	s := haltStoreOf(t, repo)
	must(t, os.MkdirAll(s.dir, 0o700))
	f, err := os.OpenFile(filepath.Join(s.dir, haltLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("flock: %v", err)
	}

	key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	start := time.Now()
	res := RunVerdict(context.Background(), haltVerdictDeps(t), wdeps, GitHubDeps{}, repo, key)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("verdict took %v under a held lock, want the bounded wait", elapsed)
	}
	if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if got := haltCountOf(t, repo); got != 0 {
		t.Fatalf("count = %d, want 0 (nothing counted without the lock)", got)
	}
}

func TestIntegrationRunVerdictHaltLimitOneStopsAtFirstHalt(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	res := RunVerdict(context.Background(), haltVerdictDepsLimit(t, 1), wdeps, GitHubDeps{}, repo, key)
	if got, want := res.reportLine(), "run-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("reportLine = %q, want %q", got, want)
	}
	if res.Note == "" || res.HaltsInARow != 1 || res.HaltLimit != 1 {
		t.Fatalf("note/halts/limit = %q/%d/%d, want a note and 1/1", res.Note, res.HaltsInARow, res.HaltLimit)
	}
}

func TestIntegrationRunVerdictHaltCountAboveLoweredLimitStops(t *testing.T) {
	repo := newRunTrackerRepo(t)
	seedHaltCount(t, repo, 3)
	key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	res := RunVerdict(context.Background(), haltVerdictDeps(t), wdeps, GitHubDeps{}, repo, key)
	if got, want := res.reportLine(), "run-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("reportLine = %q, want %q", got, want)
	}
	if res.Note == "" || res.HaltsInARow != 4 || res.HaltLimit != 2 {
		t.Fatalf("note/halts/limit = %q/%d/%d, want a note and 4/2", res.Note, res.HaltsInARow, res.HaltLimit)
	}
}

func TestIntegrationRunVerdictHaltUnreadableLimitCountsAsUnknown(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key, wdeps := haltLoopRun(t, repo, 3, ClaimSelectionQueue)
	res := RunVerdict(context.Background(), haltVerdictDepsLimit(t, 0), wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if res.Note != "" || res.HaltsInARow != 0 {
		t.Fatalf("note/halts = %q/%d, want none", res.Note, res.HaltsInARow)
	}
	if got := haltCountOf(t, repo); got != 1 {
		t.Fatalf("count = %d, want 1 (counted as unknown)", got)
	}
}

func TestIntegrationRunVerdictRunCompleteResetsHaltCount(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	seedHaltCount(t, fx.repo, 2)
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if got := haltCountOf(t, fx.repo); got != 0 {
		t.Fatalf("count after run-complete = %d, want 0", got)
	}
	if _, err := haltStoreOf(t, fx.repo).recordHalt("later", false, 0); err != nil {
		t.Fatalf("recordHalt: %v", err)
	}
	replay := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := replay.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("replay HumanText = %q, want %q", got, want)
	}
	if got := haltCountOf(t, fx.repo); got != 1 {
		t.Fatalf("count after replay = %d, want 1 (an old run-complete never re-zeroes)", got)
	}
}

func TestIntegrationRunVerdictBlockedCloseoutKeepsHaltCount(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	seedHaltCount(t, fx.repo, 2)
	must(t, RegisterRunParticipant(fx.repo, fx.key, RunParticipant{Kind: "raw-run", NativeHandle: "exec-live"}))
	fx.observer.defaultProven = false
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunCompletionUnaccounted {
		t.Fatalf("decision/reason = %q/%q, want run-stop/completion-unaccounted", res.Decision, res.Reason)
	}
	if got := haltCountOf(t, fx.repo); got != 2 {
		t.Fatalf("count = %d, want 2 (a blocked closeout resets nothing)", got)
	}
}

func TestIntegrationRunVerdictUnclaimedAndNoClaimKeepHaltCount(t *testing.T) {
	t.Run("run-unclaimed", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		seedHaltCount(t, repo, 2)
		key := runTrackerMintAttributedLimit(t, repo, 3, 2)
		deps := runTrackerLightDeps(t, []StatusBlob{runTrackerProposedBlob(3, rvSlug)})
		res := RunVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "run-done "+key+" run-unclaimed 3"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		if got := haltCountOf(t, repo); got != 2 {
			t.Fatalf("count = %d, want 2", got)
		}
	})
	t.Run("no-attributable-claim", func(t *testing.T) {
		fx := newNoClaimRunFixture(t)
		seedHaltCount(t, fx.repo, 2)
		res := RunVerdict(context.Background(), PlanningDeps{}, fx.wdeps, GitHubDeps{}, fx.repo, fx.key)
		if got, want := res.HumanText(), "run-done "+fx.key+" no-attributable-claim"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		if got := haltCountOf(t, fx.repo); got != 2 {
			t.Fatalf("count = %d, want 2", got)
		}
	})
}

func TestIntegrationRunVerdictObserveNeverTouchesHaltCount(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := runTrackerLightDeps(t, []StatusBlob{runTrackerHaltedInProgressBlob(3, rvSlug)})
	res := RunVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "run-observe run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	haltsDirAbsent(t, repo)
}

func TestIntegrationRunVerdictResetHaltsZeroesCount(t *testing.T) {
	repo := newRunTrackerRepo(t)
	seedHaltCount(t, repo, 2)
	res := RunResetHalts(repo)
	if res.Result != ResultApplied || res.Operation != OperationRunResetHalts {
		t.Fatalf("result/operation = %q/%q, want applied/%s (reason %q, message %q)", res.Result, res.Operation, OperationRunResetHalts, res.Reason, res.Message)
	}
	if res.Previous != 2 || res.PreviousUnreadable {
		t.Fatalf("previous/unreadable = %d/%v, want 2/false", res.Previous, res.PreviousUnreadable)
	}
	if got, want := res.HumanText(), "halt count reset to 0 (was 2)"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if got := haltCountOf(t, repo); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
}

func TestIntegrationRunVerdictResetHaltsRepairsCorruptCount(t *testing.T) {
	repo := newRunTrackerRepo(t)
	s := haltStoreOf(t, repo)
	must(t, os.MkdirAll(s.dir, 0o700))
	must(t, os.WriteFile(filepath.Join(s.dir, haltCountFileName), []byte("{"), 0o600))
	res := RunResetHalts(repo)
	if res.Result != ResultApplied || !res.PreviousUnreadable || res.Previous != 0 {
		t.Fatalf("result/unreadable/previous = %q/%v/%d, want applied/true/0", res.Result, res.PreviousUnreadable, res.Previous)
	}
	if got, want := res.HumanText(), "halt count reset to 0 (the previous count was unreadable)"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if got := haltCountOf(t, repo); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
}

func TestIntegrationRunVerdictResetHaltsLockBusy(t *testing.T) {
	old := haltLockWait
	haltLockWait = 100 * time.Millisecond
	t.Cleanup(func() { haltLockWait = old })

	repo := newRunTrackerRepo(t)
	seedHaltCount(t, repo, 2)
	s := haltStoreOf(t, repo)
	f, err := os.OpenFile(filepath.Join(s.dir, haltLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("flock: %v", err)
	}
	res := RunResetHalts(repo)
	if res.Result != ResultContended || res.Reason != "halt-count-busy" {
		t.Fatalf("result/reason = %q/%q, want contended/halt-count-busy", res.Result, res.Reason)
	}
	if got, want := res.HumanText(), "run.reset-halts: contended (halt-count-busy)"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if got := haltCountOf(t, repo); got != 2 {
		t.Fatalf("count = %d, want 2 (nothing reset without the lock)", got)
	}
}
