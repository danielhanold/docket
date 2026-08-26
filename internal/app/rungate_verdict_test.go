package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/repository"
)

// These are the `docket run gate-verdict <key>` (attributed mode) tests (change
// 0334, Task 3). gate-verdict loads the durable gate record, attributes exactly
// one new in-progress claim through the three filters (id not in the before-set;
// claimed_at parses; claimed_at >= dispatch epoch), then delegates the run
// predicate to RunVerify and maps its verdict onto the attributed vocabulary —
// consuming the single retry permit BEFORE emitting so a lost retry is the safe
// failure, and failing closed to gate-unavailable on any load error or
// unrecognized verdict. Every outcome is a report line that exits 0.
//
// The store is rooted at a real temp git repo (newGateRepo / the run-verify
// fixture's invocation clone); attribution reads the fakeReader corpus; the
// RunVerify delegation is driven by the run_verify_test.go fixtures (rvFixture,
// rvRecord, rvInProgressRecord, rvPR, rvAgreeingReceipt).

const gateDefaultClaimedAt = "2026-08-02T00:00:00Z"

// gateClaimEpoch is the Unix epoch of gateDefaultClaimedAt — the claim instant
// lifecycleChange stamps on an in-progress record. Attribution filter (c)
// compares a candidate's claimed_at against the record's DispatchEpoch, so tests
// straddle this value.
func gateClaimEpoch(t *testing.T) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, gateDefaultClaimedAt)
	if err != nil {
		t.Fatalf("parse %q: %v", gateDefaultClaimedAt, err)
	}
	return tm.Unix()
}

// gateInProgressBlob builds an in-progress change blob whose claimed_at is the
// default stamp ("keep"), removed (""), or replaced with the given raw value.
func gateInProgressBlob(id int, slug, claimedAt string) StatusBlob {
	src := lifecycleChange(id, slug, "in-progress")
	const def = "claimed_at: " + gateDefaultClaimedAt
	switch claimedAt {
	case "keep":
		// leave the default stamp in place
	case "":
		src = strings.Replace(src, def+"\n", "", 1)
	default:
		src = strings.Replace(src, def, "claimed_at: "+claimedAt, 1)
	}
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(src),
	}
}

// gateIncompleteRecord renders an in-progress change 3 carrying a valid pr: and
// linkage, so the ONLY unmet postcondition RunVerify reports is not-implemented
// (the run is claimed but not yet marked implemented). It lets the retry-mapping
// tests assert an exact single-conjunct report line.
func gateIncompleteRecord() []byte {
	src := string(rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug))
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+rvRecordedPR()+"'\nblocked_by:\n", 1)
	return []byte(src)
}

// gateLightDeps wires a planning-only deps set over the given corpus (no git
// client, no workspace/github seams) for attribution paths that never reach
// RunVerify.
func gateLightDeps(t *testing.T, corpus []StatusBlob) PlanningDeps {
	t.Helper()
	return PlanningDeps{Reader: &fakeReader{pin: mainPin(t), corpus: corpus}, Clock: testClock()}
}

// gateMintArmed mints an armed record (Retry unused, no attribution yet) with the
// given before-set and dispatch epoch, as gate-before would.
func gateMintArmed(t *testing.T, repoDir string, beforeIDs []int, dispatchEpoch int64) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:        "docket-implement-next",
		CreatedAt:     1,
		DispatchEpoch: dispatchEpoch,
		BeforeIDs:     beforeIDs,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// gateMintAttributed mints a record already attributed to id — the state a second
// gate-verdict call reads after a first call attributed the claim.
func gateMintAttributed(t *testing.T, repoDir string, id int) string {
	t.Helper()
	key := gateMintArmed(t, repoDir, nil, 1)
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = id
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// --- attribution filters (never reach RunVerify) ---------------------------

// TestRunGateVerdictNoAttributableClaim: with every in-progress id already in the
// before-set, zero claims are attributable and the gate reports a terminal
// gate-done no-attributable-claim.
func TestRunGateVerdictNoAttributableClaim(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateInProgressBlob(3, "alpha", "keep")})
	key := gateMintArmed(t, repo, []int{3}, 1)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)

	if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("no-attributable-claim is terminal, got Terminal=false")
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("exit code = %d, want 0 (report line)", code)
	}
	// The durable record records the terminal disposition.
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if !rec.Terminal {
		t.Errorf("record Terminal = false, want true after a terminal outcome")
	}
}

// TestRunGateVerdictThreeFilters: each attribution filter rejects its candidate
// independently, collapsing to no-attributable-claim.
func TestRunGateVerdictThreeFilters(t *testing.T) {
	claimEpoch := gateClaimEpoch(t)
	rows := []struct {
		name          string
		claimedAt     string
		beforeIDs     []int
		dispatchEpoch int64
	}{
		{name: "id present in before-set", claimedAt: "keep", beforeIDs: []int{3}, dispatchEpoch: 1},
		{name: "claimed_at missing", claimedAt: "", beforeIDs: nil, dispatchEpoch: 1},
		{name: "claimed_at malformed", claimedAt: "not-a-timestamp", beforeIDs: nil, dispatchEpoch: 1},
		{name: "claimed_at before dispatch", claimedAt: "keep", beforeIDs: nil, dispatchEpoch: claimEpoch + 1},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			repo := newGateRepo(t)
			deps := gateLightDeps(t, []StatusBlob{gateInProgressBlob(3, "alpha", row.claimedAt)})
			key := gateMintArmed(t, repo, row.beforeIDs, row.dispatchEpoch)

			res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
			if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
				t.Fatalf("HumanText = %q, want %q", got, want)
			}
			if res.AttributedID != 0 {
				t.Errorf("AttributedID = %d, want 0 (nothing attributed)", res.AttributedID)
			}
		})
	}
}

// TestRunGateVerdictClaimAtDispatchEpochAttributes: the filter is >= (claimed_at
// AT the dispatch epoch is attributable, not before it). A claim exactly at the
// epoch survives — proving the boundary is inclusive — and, being the sole
// survivor over a not-implemented run, yields gate-retry-once.
func TestRunGateVerdictClaimAtDispatchEpochAttributes(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	// DispatchEpoch equal to the claim instant: the >= filter admits it.
	key := gateMintArmed(t, f.repo.invocation, nil, gateClaimEpoch(t))

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.AttributedID != 3 {
		t.Fatalf("AttributedID = %d, want 3 (claim at the epoch is attributable)", res.AttributedID)
	}
	if res.Decision != GateDecisionRetryOnce {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateDecisionRetryOnce)
	}
}

// TestRunGateVerdictAmbiguousClaims: two survivors cannot be told apart, so the
// gate refuses with a terminal ambiguous-claims listing every survivor id.
func TestRunGateVerdictAmbiguousClaims(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{
		gateInProgressBlob(7, "bravo", "keep"),
		gateInProgressBlob(3, "alpha", "keep"),
	})
	key := gateMintArmed(t, repo, nil, 1)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-stop "+key+" ambiguous-claims 3 7"; got != want {
		t.Fatalf("HumanText = %q, want %q (survivors sorted)", got, want)
	}
	if !res.Terminal {
		t.Errorf("ambiguous-claims is terminal, got Terminal=false")
	}
}

// --- RunVerify delegation (mapping table) ----------------------------------

// TestRunGateVerdictRunIncompleteRetryThenStop drives the one-retry accounting:
// a not-implemented run yields gate-retry-once on the first call (retry permit
// consumed, non-terminal), and gate-stop run-incomplete on the second (permit
// spent, terminal). The post-pass durable state — not merely the emitted line —
// records the consumed marker and the attributed id.
func TestRunGateVerdictRunIncompleteRetryThenStop(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	res1 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res1.HumanText(), "gate-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("first call HumanText = %q, want %q", got, want)
	}
	if res1.Terminal {
		t.Errorf("gate-retry-once must be non-terminal")
	}

	// POST-PASS DURABLE STATE: reload the record and prove the retry marker is
	// present and the claim is attributed — a fresh load, not the emitted line.
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.Retry != RetryConsumed {
		t.Errorf("reloaded Retry = %q, want %q (marker present)", rec.Retry, RetryConsumed)
	}
	if rec.AttributedID != 3 {
		t.Errorf("reloaded AttributedID = %d, want 3 (durably attributed)", rec.AttributedID)
	}

	res2 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res2.HumanText(), "gate-stop "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("second call HumanText = %q, want %q", got, want)
	}
	if !res2.Terminal {
		t.Errorf("second gate-stop run-incomplete must be terminal")
	}
}

// TestRunGateVerdictRunComplete: the attributed-id short-circuit verifies the
// stored id directly. An implemented change 3 (which fresh attribution could
// never pick, being no longer in-progress) yields gate-done run-complete —
// proving the short-circuit bypassed attribution.
func TestRunGateVerdictRunComplete(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintAttributed(t, f.repo.invocation, 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("run-complete is terminal")
	}
	if res.AttributedID != 3 {
		t.Errorf("AttributedID = %d, want 3 (short-circuit kept the stored id)", res.AttributedID)
	}
}

// TestRunGateVerdictRunUnclaimed: the attributed-id short-circuit over a change
// that is now proposed (never-claimed) maps RunVerify's run-unclaimed to a
// terminal gate-done.
func TestRunGateVerdictRunUnclaimed(t *testing.T) {
	repo := newGateRepo(t)
	deps := rvProposedDeps(t) // corpus: change 3 proposed
	key := gateMintAttributed(t, repo, 3)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-unclaimed 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("run-unclaimed is terminal")
	}
}

// TestRunGateVerdictRunHalted: a halted in-progress change is attributed, then
// RunVerify's run-halted maps to a terminal gate-stop (a human is needed).
func TestRunGateVerdictRunHalted(t *testing.T) {
	repo := newGateRepo(t)
	src := strings.TrimRight(lifecycleChange(3, "widget", "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	corpus := []StatusBlob{{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(3, "widget"),
		Version:  miVersion,
		Data:     []byte(src),
	}}
	deps := gateLightDeps(t, corpus)
	key := gateMintArmed(t, repo, nil, 1)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("run-halted is terminal")
	}
}

// TestRunGateVerdictRunWaiting: a fully-agreeing local waiting receipt over an
// in-progress change yields a terminal gate-stop run-waiting that passes the
// opaque handoff id and phase through verbatim.
func TestRunGateVerdictRunWaiting(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-stop "+key+" run-waiting 3 d0opaque build"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if res.HandoffID != "d0opaque" || res.Phase != "build" {
		t.Errorf("handoff/phase = %q/%q, want d0opaque/build (verbatim pass-through)", res.HandoffID, res.Phase)
	}
	if !res.Terminal {
		t.Errorf("run-waiting is terminal")
	}
}

// --- fail-closed: load errors and unknown verdicts -------------------------

// TestRunGateVerdictLoadErrorsFailClosed: every store load fault maps to a
// terminal gate-stop gate-unavailable carrying the store's typed reason token —
// never a retry.
func TestRunGateVerdictLoadErrorsFailClosed(t *testing.T) {
	t.Run("malformed key", func(t *testing.T) {
		repo := newGateRepo(t)
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repo, "../escape")
		if got, want := res.HumanText(), "gate-stop ../escape gate-unavailable malformed-key"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		if !res.Terminal {
			t.Errorf("gate-unavailable is terminal")
		}
	})

	t.Run("record not found", func(t *testing.T) {
		repo := newGateRepo(t)
		key := "implement-next-nope"
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable not-found"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
	})

	t.Run("corrupt record", func(t *testing.T) {
		repo := newGateRepo(t)
		key := gateMintArmed(t, repo, nil, 1)
		root, err := gateRoot(repo)
		if err != nil {
			t.Fatalf("gateRoot: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, key, "record.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable corrupt-record"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
	})

	t.Run("wrong repo", func(t *testing.T) {
		repoA := newGateRepo(t)
		repoB := newGateRepo(t)
		key := gateMintArmed(t, repoA, nil, 1)
		rootA, _ := gateRoot(repoA)
		rootB, _ := gateRoot(repoB)
		if err := os.MkdirAll(filepath.Join(rootB, key), 0o755); err != nil {
			t.Fatal(err)
		}
		buf, err := os.ReadFile(filepath.Join(rootA, key, "record.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootB, key, "record.json"), buf, 0o644); err != nil {
			t.Fatal(err)
		}
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repoB, key)
		if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable wrong-repo"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
	})
}

// TestRunGateVerdictUnknownVerdictFailsClosed: a RunVerify outcome with no
// recognized verdict (here an operational unknown-change error over an
// attributed id absent from the corpus) fails closed to gate-unavailable
// unknown-verdict — never a retry, never a silent pass.
func TestRunGateVerdictUnknownVerdictFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{}) // empty corpus: id 999 is unknown
	key := gateMintAttributed(t, repo, 999)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable unknown-verdict"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("gate-unavailable is terminal")
	}
}

// --- concurrency, isolation, restart durability ----------------------------

// TestRunGateVerdictConcurrentRetryGrantsOnce is the mutation target: two
// concurrent verdict calls on one not-implemented run must grant EXACTLY ONE
// gate-retry-once (the O_EXCL CAS in ConsumeGateRetry, consumed before the report
// is chosen). Reversing the consume-then-emit order — deciding from the record's
// stale Retry mirror and consuming afterward — double-grants and reddens here.
func TestRunGateVerdictConcurrentRetryGrantsOnce(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	ev := string(prEvidenceBytes(t, f.head))
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	// Each goroutine gets its OWN deps triple: in production the two concurrent
	// verdict calls are separate processes, each with its own reader/workspace/
	// GitHub adapters. The in-memory fakes record their calls without locks, so
	// sharing one triple across both goroutines races under -race on that
	// bookkeeping — a test-double artifact, not the behavior under test. The only
	// resource the two calls genuinely contend on is the on-disk gate record under
	// f.repo.invocation, whose single-grant guarantee is the O_EXCL CAS in
	// ConsumeGateRetry — that contention is preserved.
	var wg sync.WaitGroup
	results := make([]RunGateVerdictResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		deps, wdeps, gdeps := f.deps(
			rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
			rvPR(f.head, ev),
		)
		go func(idx int, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps) {
			defer wg.Done()
			results[idx] = RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
		}(i, deps, wdeps, gdeps)
	}
	wg.Wait()

	retryOnce, stop := 0, 0
	for _, r := range results {
		switch r.Decision {
		case GateDecisionRetryOnce:
			retryOnce++
		case GateDecisionStop:
			stop++
		default:
			t.Fatalf("unexpected decision %q (%q)", r.Decision, r.HumanText())
		}
	}
	if retryOnce != 1 || stop != 1 {
		t.Fatalf("gate-retry-once=%d gate-stop=%d, want exactly 1 and 1 (single retry permit)", retryOnce, stop)
	}
}

// TestRunGateVerdictTwoKeysIsolated: two distinct keys in one repository hold
// independent retry permits — consuming one never touches the other.
func TestRunGateVerdictTwoKeysIsolated(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	keyA := gateMintArmed(t, f.repo.invocation, nil, 1)
	keyB := gateMintArmed(t, f.repo.invocation, nil, 1)

	resA := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, keyA)
	resB := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, keyB)
	if resA.Decision != GateDecisionRetryOnce {
		t.Errorf("key A decision = %q, want gate-retry-once", resA.Decision)
	}
	if resB.Decision != GateDecisionRetryOnce {
		t.Errorf("key B decision = %q, want gate-retry-once (independent permit)", resB.Decision)
	}
}

// TestRunGateVerdictRestartDurability: gate-before and gate-verdict share nothing
// but the repository directory and the key. Minting the record through the store
// (as a separate gate-before process would) and then reading a verdict through a
// fresh RunGateVerdict call — no record value passed between them — still resolves
// the attributed run.
func TestRunGateVerdictRestartDurability(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	// Simulate the arming process: mint and forget (nothing carried in memory).
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	// A fresh call sharing only repoDir + key attributes and reports.
	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.AttributedID != 3 {
		t.Fatalf("AttributedID = %d, want 3 (attributed from the durable record alone)", res.AttributedID)
	}
	if res.Decision != GateDecisionRetryOnce {
		t.Fatalf("Decision = %q, want gate-retry-once", res.Decision)
	}
}

// --- unattributed (observe-only) mode --------------------------------------
//
// RunGateVerdictObserve is the `--unattributed` mode (change 0334, Task 4): NO
// key, NO record, NO writes. It re-syncs, verifies each supplied hint id (a hint
// is an id to verify, never attribution evidence) — or every current in-progress
// id when none are supplied — and renders one `gate-observe <verdict> <id>` line
// per id using RunVerify's verdict verbatim, through a SEPARATE render function
// that only knows the `gate-observe` prefix and is structurally unable to emit
// gate-retry-once.

// gateHaltedInProgressBlob builds an in-progress change carrying a durable
// "## Run halted" body section — status stays in-progress (so observeInProgress
// counts it) while RunVerify short-circuits it to run-halted with reader-only
// deps, letting the observe tests assert exact lines without the full git path.
func gateHaltedInProgressBlob(id int, slug string) StatusBlob {
	src := strings.TrimRight(lifecycleChange(id, slug, "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(src),
	}
}

// gateProposedBlob builds a proposed (never-claimed) change — RunVerify maps it
// to run-unclaimed with reader-only deps.
func gateProposedBlob(id int, slug string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(lifecycleChange(id, slug, "proposed")),
	}
}

// TestRunGateVerdictObserveHintsMixedVerdicts: supplied hint ids are each verified
// and rendered as one `gate-observe <verdict> <id>` line, in the INPUT order given
// (never re-sorted), using RunVerify's verdict verbatim.
func TestRunGateVerdictObserveHintsMixedVerdicts(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{
		gateProposedBlob(3, "alpha"),
		gateHaltedInProgressBlob(7, "bravo"),
	})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, []string{"7", "3"})
	if got, want := res.HumanText(), "gate-observe run-halted 7\ngate-observe run-unclaimed 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (one line per hint, input order)", got, want)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("exit code = %d, want 0 (report lines)", code)
	}
}

// TestRunGateVerdictObserveNoHintsAllInProgress: with no hints, every current
// in-progress id is verified (sorted), one line each.
func TestRunGateVerdictObserveNoHintsAllInProgress(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{
		gateHaltedInProgressBlob(7, "bravo"),
		gateHaltedInProgressBlob(3, "alpha"),
		gateProposedBlob(9, "charlie"), // proposed: not in-progress, never observed
	})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "gate-observe run-halted 3\ngate-observe run-halted 7"; got != want {
		t.Fatalf("HumanText = %q, want %q (all in-progress ids, sorted)", got, want)
	}
}

// TestRunGateVerdictObserveEmptyBacklogNoCurrentRun: no in-progress ids and no
// hints → a single terminal-shaped `gate-observe no-current-run` line.
func TestRunGateVerdictObserveEmptyBacklogNoCurrentRun(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateProposedBlob(9, "charlie")})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "gate-observe no-current-run"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// TestRunGateVerdictObserveIncompleteWritesNothing: an incomplete run observed
// unattributed emits `gate-observe run-incomplete <id> <unmet...>` and writes NO
// record and consumes NOTHING — the rungate root is never created (no mint, no
// save, no retry consumption on the observe path).
func TestRunGateVerdictObserveIncompleteWritesNothing(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)

	res := RunGateVerdictObserve(context.Background(), deps, wdeps, gdeps, f.repo.invocation, []string{"3"})
	if got, want := res.HumanText(), "gate-observe run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if strings.HasPrefix(res.HumanText(), GateDecisionRetryOnce) {
		t.Fatalf("observe must never emit %q", GateDecisionRetryOnce)
	}

	// NO record, NO writes: the rungate root is never created by the observe path.
	root, err := gateRoot(f.repo.invocation)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("rungate root %q exists (stat err %v); observe must write nothing", root, statErr)
	}
}

// TestRunGateVerdictObserveKeyIsUsageError: `--unattributed` combined with a key
// (a non-integer positional) is a usage error — a non-zero exit, never a report
// line. Hints are change ids; a key can never be one.
func TestRunGateVerdictObserveKeyIsUsageError(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateHaltedInProgressBlob(3, "alpha")})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, []string{"implement-next-20260826T000000Z-1-abcd"})
	if res.Env().Result != ResultInvalidInput {
		t.Fatalf("Result = %q, want ResultInvalidInput (a key is not a hint)", res.Env().Result)
	}
	if code := ExitCode(res.Env().Result); code == 0 {
		t.Fatalf("exit code = %d, want non-zero (usage error)", code)
	}
}

// TestRunGateVerdictObserveSyncFailureUnavailable: a re-sync/read fault fails
// closed to a single `gate-observe gate-unavailable <reason>` line.
func TestRunGateVerdictObserveSyncFailureUnavailable(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: &fakeReader{pinErr: errors.New("boom")}, Clock: testClock()}

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "gate-observe gate-unavailable "+ReasonGateSyncFailed; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
}
