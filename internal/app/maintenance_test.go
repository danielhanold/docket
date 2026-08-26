package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// This file drives `maintenance sweep` — the batch driver that reclaims
// docket's terminal half — over a fake corpus reader, a scriptable PR prober,
// and RECORDING operation seams. The three verified operations (closeout,
// cleanup, reclaim) are heavy real-git effects proved in their own tasks; here
// they are injected fakes so the sweep's orchestration is proved in isolation:
// the deterministic worklist, children-before-ancestors closeout order, the
// per-item destructive-suffix rule, reclaim gating on reclaim.auto, item
// isolation, the structured sorted report, and — the premium-risk property — a
// fresh authority reload before EVERY dispatched mutation.

// --- recording ops seam ---------------------------------------------------

type sweepCall struct {
	kind    string
	id      int
	version string
}

// recordingSweepOps records every dispatch and answers from scripted results,
// defaulting each op to an applied success so a test scripts only the outcomes
// it cares about.
type recordingSweepOps struct {
	calls    []sweepCall
	closeout map[int]CloseoutResult
	cleanup  map[int]CleanupOpResult
	reclaim  map[int]ChangeReclaimResult
}

func (r *recordingSweepOps) seam() sweepOps {
	return sweepOps{
		closeout: func(_ context.Context, id int) CloseoutResult {
			r.calls = append(r.calls, sweepCall{kind: sweepKindCloseout, id: id})
			if res, ok := r.closeout[id]; ok {
				return res
			}
			return newCloseoutResult(ResultApplied, CloseoutResult{ID: id, Disposition: CloseoutDispDoneArchived})
		},
		cleanup: func(_ context.Context, id int) CleanupOpResult {
			r.calls = append(r.calls, sweepCall{kind: sweepKindCleanup, id: id})
			if res, ok := r.cleanup[id]; ok {
				return res
			}
			return newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{ID: id, Disposition: CleanupDispCleaned})
		},
		reclaim: func(_ context.Context, id int, version string) ChangeReclaimResult {
			r.calls = append(r.calls, sweepCall{kind: sweepKindReclaim, id: id, version: version})
			if res, ok := r.reclaim[id]; ok {
				return res
			}
			return newChangeReclaimResult(ResultApplied, ChangeReclaimResult{ID: id, Disposition: ReclaimDispReclaimed})
		},
	}
}

func (r *recordingSweepOps) callIDs(kind string) []int {
	var out []int
	for _, c := range r.calls {
		if c.kind == kind {
			out = append(out, c.id)
		}
	}
	return out
}

func (r *recordingSweepOps) called(kind string, id int) bool {
	for _, c := range r.calls {
		if c.kind == kind && c.id == id {
			return true
		}
	}
	return false
}

// --- corpus + deps helpers ------------------------------------------------

// sweepPin clones the docket-mode pin and sets reclaim policy for the run.
func sweepPin(t *testing.T, auto bool, ttlHours int) StatusPin {
	t.Helper()
	p := docketPin(t)
	p.Config.Effective.Reclaim.Auto.Value = auto
	p.Config.Effective.Reclaim.LeaseTTL.Value = ttlHours
	return p
}

// sweepInProgressBlob builds an in-progress record with an old claim stamp
// (2026-08-02, well before the fixed test clock's 2026-08-16), so its lease is
// strictly expired under any small TTL.
func sweepInProgressBlob(id int, slug string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobip%04d", id),
		Data:     []byte(lifecycleChange(id, slug, "in-progress")),
	}
}

func sweepDeps(reader *fakeReader, prober FinalizePRProber) FinalizeDeps {
	return FinalizeDeps{
		Planning: PlanningDeps{Reader: reader, Clock: testClock()},
		PRProber: prober,
	}
}

// mergedFacts is the domain facts of a merged PR needing closeout recovery.
func mergedFacts(number int, base string) domain.PRFacts {
	return domain.PRFacts{
		Number:      fmt.Sprintf("%d", number),
		Version:     fmt.Sprintf("v%d", number),
		State:       "merged",
		HeadOID:     fmt.Sprintf("h%d", number),
		BaseRef:     base,
		MergedAtUTC: "2026-08-10T00:00:00Z",
		MergeCommit: fmt.Sprintf("m%d", number),
	}
}

var sweepDispositionSet = map[string]bool{
	SweepDispApplied: true, SweepDispNoOp: true, SweepDispContended: true,
	SweepDispBlocked: true, SweepDispUnknown: true, SweepDispFailed: true, SweepDispSkipped: true,
}

// --- tests ----------------------------------------------------------------

// TestSweepFindsMergedImplemented: active implemented changes with merged PRs
// are closed out, stacked children before their ancestor, and the root closeout
// carries the descendants. The order is asserted on the recorded dispatch log.
func TestSweepFindsMergedImplemented(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(30, "root", "implemented", "high", prRefFor(30), ""),
		finalizeBlob(31, "childa", "implemented", "high", prRefFor(31), "stacked_on: 30\n"),
		finalizeBlob(32, "childb", "implemented", "high", prRefFor(32), "stacked_on: 30\n"),
	}
	reader := &fakeReader{pin: sweepPin(t, false, 24), corpus: corpus}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(30): withHead(mergedFacts(30, "main"), "feat/root"),
		prRefFor(31): withHead(mergedFacts(31, "feat/root"), "feat/childa"),
		prRefFor(32): withHead(mergedFacts(32, "feat/root"), "feat/childb"),
	}}
	ops := &recordingSweepOps{closeout: map[int]CloseoutResult{
		30: newCloseoutResult(ResultApplied, CloseoutResult{ID: 30, Disposition: CloseoutDispRootArchived, CarriedIDs: []int{31, 32}}),
		31: newCloseoutResult(ResultApplied, CloseoutResult{ID: 31, Disposition: CloseoutDispStackedMerged}),
		32: newCloseoutResult(ResultApplied, CloseoutResult{ID: 32, Disposition: CloseoutDispStackedMerged}),
	}}

	res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied; entries=%+v", res.Result, res.Entries)
	}
	order := ops.callIDs(sweepKindCloseout)
	if len(order) != 3 {
		t.Fatalf("closeout dispatches = %v, want three (30,31,32)", order)
	}
	// Children (31,32) must both precede the root (30).
	rootPos, childMax := -1, -1
	for i, id := range order {
		if id == 30 {
			rootPos = i
		} else if i > childMax {
			childMax = i
		}
	}
	if rootPos < 0 || rootPos < childMax {
		t.Fatalf("root 30 must close AFTER its children; order=%v", order)
	}
	// The root's entry carries its proven descendants.
	var rootEntry *MaintenanceEntry
	for i := range res.Entries {
		if res.Entries[i].ID == 30 && res.Entries[i].Kind == sweepKindCloseout {
			rootEntry = &res.Entries[i]
		}
	}
	if rootEntry == nil || len(rootEntry.CarriedIDs) != 2 {
		t.Fatalf("root closeout entry must carry descendants; got %+v", rootEntry)
	}
}

// TestSweepRetriesSuffixes: archived/done records and completed stacks get the
// terminal backlink repair + ownership-safe cleanup retried (a cleanup dispatch)
// even though they need no closeout.
func TestSweepRetriesSuffixes(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(40, "completed", "stacked-merged", "high", prRefFor(40), ""),
		finalizeBlob(41, "archived", "done", "high", prRefFor(41), ""),
	}
	reader := &fakeReader{pin: sweepPin(t, false, 24), corpus: corpus}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{}}
	ops := &recordingSweepOps{}

	res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())

	if !ops.called(sweepKindCleanup, 40) {
		t.Errorf("completed stack 40 must have cleanup retried; calls=%v", ops.calls)
	}
	if !ops.called(sweepKindCleanup, 41) {
		t.Errorf("done record 41 must have cleanup retried; calls=%v", ops.calls)
	}
	if len(ops.callIDs(sweepKindCloseout)) != 0 {
		t.Errorf("terminal records must not be closed out; closeouts=%v", ops.callIDs(sweepKindCloseout))
	}
	if res.Result != ResultApplied {
		t.Errorf("result = %q, want applied", res.Result)
	}
}

// TestSweepReclaimGatedOnAuto: an eligible (in-progress, strictly-expired) record
// is reclaimed only when reclaim.auto is true; with auto false it is surfaced as
// skipped with the reclaim-auto-disabled reason and no reclaim is dispatched.
func TestSweepReclaimGatedOnAuto(t *testing.T) {
	corpus := []StatusBlob{sweepInProgressBlob(50, "stale")}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{}}

	t.Run("auto true reclaims", func(t *testing.T) {
		reader := &fakeReader{pin: sweepPin(t, true, 24), corpus: corpus}
		ops := &recordingSweepOps{}
		maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())
		if !ops.called(sweepKindReclaim, 50) {
			t.Fatalf("auto reclaim must dispatch reclaim for 50; calls=%v", ops.calls)
		}
	})

	t.Run("auto false skips with reason", func(t *testing.T) {
		reader := &fakeReader{pin: sweepPin(t, false, 24), corpus: corpus}
		ops := &recordingSweepOps{}
		res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())
		if ops.called(sweepKindReclaim, 50) {
			t.Fatalf("reclaim.auto:false must NOT dispatch reclaim; calls=%v", ops.calls)
		}
		var entry *MaintenanceEntry
		for i := range res.Entries {
			if res.Entries[i].ID == 50 && res.Entries[i].Kind == sweepKindReclaim {
				entry = &res.Entries[i]
			}
		}
		if entry == nil || entry.Disposition != SweepDispSkipped || entry.Reason != ReasonSweepReclaimAutoDisabled {
			t.Fatalf("want skipped/reclaim-auto-disabled entry, got %+v", entry)
		}
	})
}

// TestSweepNeverEscalates: the sweep acts only on merged-recovery closeout work;
// an implemented change whose PR is still OPEN is left entirely alone (merging it
// is the attended finalize flow's job, never the sweep's). A merged implemented
// change is recovered regardless — a finalize marker never suppresses recovering
// an out-of-band-merged PR.
func TestSweepNeverEscalates(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(60, "merged", "implemented", "high", prRefFor(60), ""),
		finalizeBlob(61, "open", "implemented", "high", prRefFor(61), ""),
	}
	reader := &fakeReader{pin: sweepPin(t, false, 24), corpus: corpus}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(60): withHead(mergedFacts(60, "main"), "feat/merged"),
		prRefFor(61): openFacts(61, "MERGEABLE", 2, 20),
	}}
	ops := &recordingSweepOps{}

	res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())

	if !ops.called(sweepKindCloseout, 60) {
		t.Errorf("out-of-band-merged 60 must be recovered; calls=%v", ops.calls)
	}
	for _, c := range ops.calls {
		if c.id == 61 {
			t.Errorf("open-PR change 61 must be left alone, got dispatch %+v", c)
		}
	}
	for _, e := range res.Entries {
		if e.ID == 61 {
			t.Errorf("open-PR change 61 must produce no entry, got %+v", e)
		}
	}
}

// TestSweepItemIsolation: one item's failure never stops an independent item,
// and within a single item a destructive suffix never runs after an unresolved
// (unknown) prerequisite.
func TestSweepItemIsolation(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(70, "unknownitem", "implemented", "high", prRefFor(70), ""),
		finalizeBlob(71, "gooditem", "implemented", "high", prRefFor(71), ""),
	}
	reader := &fakeReader{pin: sweepPin(t, false, 24), corpus: corpus}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(70): withHead(mergedFacts(70, "main"), "feat/unknownitem"),
		prRefFor(71): withHead(mergedFacts(71, "main"), "feat/gooditem"),
	}}
	ops := &recordingSweepOps{closeout: map[int]CloseoutResult{
		70: newCloseoutResult(ResultExternalFailed, CloseoutResult{ID: 70, Disposition: CloseoutDispUnknown}),
	}}

	res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())

	// Isolation: 71's closeout still ran despite 70's unknown outcome.
	if !ops.called(sweepKindCloseout, 71) {
		t.Errorf("independent item 71 must still be processed; calls=%v", ops.calls)
	}
	// Within-item rule: 70's cleanup suffix must NOT run after an unknown closeout.
	if ops.called(sweepKindCleanup, 70) {
		t.Errorf("destructive suffix ran after an unknown prerequisite; calls=%v", ops.calls)
	}
	// 71 succeeded, so its cleanup suffix does run.
	if !ops.called(sweepKindCleanup, 71) {
		t.Errorf("successful item 71 must run its cleanup suffix; calls=%v", ops.calls)
	}
	// The withheld suffix is surfaced, not silently dropped.
	var suffix *MaintenanceEntry
	for i := range res.Entries {
		if res.Entries[i].ID == 70 && res.Entries[i].Kind == sweepKindCleanup {
			suffix = &res.Entries[i]
		}
	}
	if suffix == nil || suffix.Disposition != SweepDispSkipped || suffix.Reason != ReasonSweepPrerequisiteUnresolved {
		t.Fatalf("withheld suffix must be a skipped/prerequisite-unresolved entry, got %+v", suffix)
	}
}

// TestSweepStructuredReport: every processed item is reported as a structured
// entry carrying a closed disposition token, sorted deterministically — never
// collapsed to a single boolean.
func TestSweepStructuredReport(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(80, "merged", "implemented", "high", prRefFor(80), ""),
		finalizeBlob(81, "stack", "stacked-merged", "high", prRefFor(81), ""),
		sweepInProgressBlob(82, "stale"),
	}
	reader := &fakeReader{pin: sweepPin(t, true, 24), corpus: corpus}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(80): withHead(mergedFacts(80, "main"), "feat/merged"),
	}}
	ops := &recordingSweepOps{}

	res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())

	if len(res.Entries) == 0 {
		t.Fatal("a sweep that processed items must report entries")
	}
	for _, e := range res.Entries {
		if !sweepDispositionSet[e.Disposition] {
			t.Errorf("entry %+v carries a non-closed disposition", e)
		}
		if e.Kind == "" {
			t.Errorf("entry %+v carries no kind", e)
		}
	}
	// Entries are sorted by (ID, Kind).
	for i := 1; i < len(res.Entries); i++ {
		a, b := res.Entries[i-1], res.Entries[i]
		if a.ID > b.ID || (a.ID == b.ID && a.Kind > b.Kind) {
			t.Errorf("entries not sorted at %d: %+v then %+v", i, a, b)
		}
	}
}

// TestSweepReloadsBeforeMutation: the sweep pins one inventory, then reloads
// fresh authority before every dispatched mutation — exactly one reload per
// dispatch — and reloads for nothing it does not dispatch.
func TestSweepReloadsBeforeMutation(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(90, "merged", "implemented", "high", prRefFor(90), ""),
		sweepInProgressBlob(91, "stale"), // reclaim.auto false ⇒ skipped, no reload
	}
	reader := &fakeReader{pin: sweepPin(t, false, 24), corpus: corpus}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(90): withHead(mergedFacts(90, "main"), "feat/merged"),
	}}
	ops := &recordingSweepOps{}

	maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam())

	// 90: closeout (applied) + cleanup suffix = 2 dispatches. 91: skipped, none.
	dispatches := len(ops.calls)
	if dispatches != 2 {
		t.Fatalf("dispatches = %d, want 2 (closeout+suffix for 90); calls=%v", dispatches, ops.calls)
	}
	// One inventory pin + one fresh reload per dispatch.
	wantPins := 1 + dispatches
	if reader.pinCount != wantPins {
		t.Fatalf("reader pinned %d times, want %d (1 inventory + %d reloads)", reader.pinCount, wantPins, dispatches)
	}
}
