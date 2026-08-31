<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0389 — Speed up implement-next status sweeps and retire completed status children](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0389-speed-up-implement-next-status-sweeps-and-retire-completed-s.md)**
<!-- docket:backlink:end -->
# Speed Up Implement-Next Status Sweeps and Retire Completed Status Children — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a closed `--scope` flag to `docket maintenance sweep` so implementation startup recovers current merged work without retrying cleanup for the whole historical archive, and rewrite the docket-status / docket-implement-next / convention prose so a backgrounded sweep is observed to its terminal result at both the command and agent boundaries.

**Architecture:** One typed scope value resolved once in `internal/cli/maintenance.go` and threaded into `internal/app/maintenance.go`'s worklist derivation; in `implementation` scope the done/stacked-merged records present at the pinned inventory are counted and deferred rather than enqueued as independent cleanup items, while merged-implemented closeouts, their cleanup suffixes, and reclaim gating are untouched. The prose half wires the caller (`--scope implementation` at implement-next Step 0) and installs two completion barriers: the docket-status skill owns observing the sweep process to its terminal protocol-v1 envelope; the implement-next skill owns accepting the child only with terminal evidence. Prose claims are pinned by new `internal/repoguard` prose-contract rows.

**Tech Stack:** Go (cobra CLI, table-driven tests with recording fakes), maintained markdown skill bodies mirrored into `internal/assets/embedded` via `cmd/genassets`.

**Spec:** `docs/superpowers/specs/2026-08-31-speed-up-implement-next-status-sweeps-and-retire-completed-s-design.md` (synchronized on the `docket` metadata branch; read it — every task below argues from it).

## Global Constraints

- Closed scope vocabulary: `full` (default when `--scope` is omitted) and `implementation`. Empty or unknown explicit values are refused **before** any repo/network/mutation work.
- Scope is resolved **once** in the CLI to a typed value and passed into the app layer. Never branch on caller names, model prose, age cutoffs, or config-file presence.
- Preserve exactly: `protocol_version` 1, `operation` `maintenance.sweep`, the existing per-item dispositions/reasons and field meanings, per-item isolation, the unknown-prerequisite suffix withholding, descendant-before-ancestor closeout order, reclaim gating on `reclaim.auto`, and the fresh reload-before-every-mutation invariant. New envelope fields are additive only.
- `full` scope retains today's behavior byte-for-byte (same worklist, same entries).
- Do NOT redesign `FinalizeCleanup` or weaken its ownership/merge/backlink/exact-ref proofs. Do NOT change agent-launch topology, add runner fallbacks, or introduce a new run-gate state.
- Prose edits touch **tracked sources only**: `skills/*/SKILL.md` (and, where the derivation task proves it necessary, `skills/docket-convention/references/*`, `agents/*.md`, `cursor-rules/*`). Never hand-edit the installed copies under `~/.claude/skills/` (machine-local, refreshed by `docket development install` after merge) and never hand-edit `internal/assets/embedded/**` — regenerate it with `go generate ./internal/assets/` after every skills/agents edit, in the same commit.
- Do NOT rewrite accepted ADRs, archived changes, archived specs, frozen plans, or results files — they are point-in-time records.
- Every mutation probe and manual re-verification uses `go test -count=1` (the Go test cache serves stale verdicts otherwise).
- The build gate runs the WHOLE suite via the resolved `finalize.test_command`: `go run ./cmd/docket development test` from the feature worktree root.
- Commit messages use the repo's `<type>(0389): <summary>` style, staging by explicit path.

## Learnings that bind this plan (read before building)

From `docs/changes/learnings/` on the `docket` branch — pulled for this change:

- `optimization-needs-a-measured-oracle` — the correctness suite cannot judge the speedup; the measured-performance acceptance is a **human close-out item** (Task 10), and the growth test (Task 2) pins the mechanism (zero per-historical dispatches), not wall clock.
- `defaulted-param-hides-caller-wiring` — assert the **resolved non-default** scope arrives (`"scope":"implementation"` in CLI output), not just that a default works.
- `mutation-target-needs-a-forced-exit` — any fixture that waits on a guarded condition needs an independent hard stop so a removed guard reds boundedly instead of hanging (relevant to the human lifecycle certification; the Go tests here are all straight-line).
- `cached-runner-serves-a-mutated-tree` — `-count=1` on every mutation probe.
- `restatement-accumulates-its-own-guards` — before rewording any skill sentence, grep `internal/repoguard/` and `tests/` for the exact clause; repoint dependent rows in the same commit.
- `phrase-grep-over-wrapped-prose` — every new prose-contract anchor phrase must sit unwrapped on one physical line of the markdown it guards; verify with `grep -F` after final re-flow.
- `generated-artifact-loaded-at-process-start` — skill bodies are loaded at harness process start; the session that edits them cannot runtime-validate them. Record the restart precondition in the results file (Task 10).
- `yielded-worker-return-closes-every-door` — the bug this change fixes; the barrier prose in Tasks 5–7 is its remedy, and no worker on THIS build may background the suite gate and yield either.

---

### Task 1: App layer — typed `SweepScope`, scope-filtered worklist, scope-stamped envelope

**Files:**
- Modify: `internal/app/maintenance.go`
- Test: `internal/app/maintenance_test.go`

**Interfaces:**
- Produces: `type SweepScope string` with `SweepScopeFull SweepScope = "full"` and `SweepScopeImplementation SweepScope = "implementation"`; `MaintenanceSweep(ctx context.Context, deps FinalizeDeps, repoDir string, scope SweepScope) MaintenanceResult`; `maintenanceSweep(ctx, deps, repoDir, ops, scope)`; `sweepWorklist(snap, queue, eff, now, scope) (items []sweepWorkItem, deferredHistorical int)`; `MaintenanceResult.Scope string` (json `scope`) and `MaintenanceResult.DeferredHistoricalCleanups int` (json `deferred_historical_cleanups`), both always emitted; new reason constant `ReasonSweepScopeInvalid = "sweep-scope-invalid"`.
- Consumes: existing test helpers `finalizeBlob`, `sweepInProgressBlob`, `sweepPin`, `sweepDeps`, `fakeReader`, `fakeFinalizeProber`, `recordingSweepOps`, `mergedFacts`, `withHead`, `prRefFor`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/app/maintenance_test.go`. First update every existing `maintenanceSweep(...)` call in the file to pass `SweepScopeFull` as the new final argument (they must stay green — full scope retains today's behavior exactly). Then add:

```go
// TestSweepImplementationScopeDefersHistorical: implementation scope schedules
// current merged-implemented closeouts (whose cleanup SUFFIX still runs) and
// gated reclaims, but enqueues NO independent cleanup item for a record that was
// already done/stacked-merged at the pinned inventory. Those are counted as a
// scope summary, never rendered as per-item outcomes. Full scope on the same
// corpus retains the historical retries.
func TestSweepImplementationScopeDefersHistorical(t *testing.T) {
	corpus := []StatusBlob{
		finalizeBlob(30, "current", "implemented", "high", prRefFor(30), ""),
		finalizeBlob(40, "completed", "stacked-merged", "high", prRefFor(40), ""),
		finalizeBlob(41, "archived", "done", "high", prRefFor(41), ""),
		sweepInProgressBlob(50, "stale"),
	}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(30): withHead(mergedFacts(30, "main"), "feat/current"),
	}}

	t.Run("implementation defers historical, keeps closeout+suffix+reclaim", func(t *testing.T) {
		reader := &fakeReader{pin: sweepPin(t, true, 24), corpus: corpus}
		ops := &recordingSweepOps{}
		res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam(), SweepScopeImplementation)

		if !ops.called(sweepKindCloseout, 30) {
			t.Errorf("current merged closeout must still dispatch; calls=%v", ops.calls)
		}
		if !ops.called(sweepKindCleanup, 30) {
			t.Errorf("the closeout's cleanup SUFFIX must still run in implementation scope; calls=%v", ops.calls)
		}
		if !ops.called(sweepKindReclaim, 50) {
			t.Errorf("reclaim gating must be preserved (auto=true dispatches); calls=%v", ops.calls)
		}
		for _, id := range []int{40, 41} {
			if ops.called(sweepKindCleanup, id) {
				t.Errorf("historical %d must NOT get an independent cleanup dispatch; calls=%v", id, ops.calls)
			}
			for _, e := range res.Entries {
				if e.ID == id {
					t.Errorf("historical %d must have no per-item entry, got %+v", id, e)
				}
			}
		}
		if res.Scope != "implementation" {
			t.Errorf("scope = %q, want implementation", res.Scope)
		}
		if res.DeferredHistoricalCleanups != 2 {
			t.Errorf("deferred_historical_cleanups = %d, want 2", res.DeferredHistoricalCleanups)
		}
	})

	t.Run("full retains historical retries and reports zero deferred", func(t *testing.T) {
		reader := &fakeReader{pin: sweepPin(t, true, 24), corpus: corpus}
		ops := &recordingSweepOps{}
		res := maintenanceSweep(context.Background(), sweepDeps(reader, prober), "repo", ops.seam(), SweepScopeFull)
		if !ops.called(sweepKindCleanup, 40) || !ops.called(sweepKindCleanup, 41) {
			t.Errorf("full scope must retry historical cleanups; calls=%v", ops.calls)
		}
		if res.Scope != "full" || res.DeferredHistoricalCleanups != 0 {
			t.Errorf("scope=%q deferred=%d, want full/0", res.Scope, res.DeferredHistoricalCleanups)
		}
	})
}

// TestSweepInvalidScopeRefusesBeforeAnyRead: a typed scope outside the closed
// vocabulary is a fail-closed input refusal that dispatches nothing and reads
// nothing (defense in depth behind the CLI's own validation).
func TestSweepInvalidScopeRefusesBeforeAnyRead(t *testing.T) {
	reader := &fakeReader{pin: sweepPin(t, true, 24), corpus: nil}
	ops := &recordingSweepOps{}
	res := maintenanceSweep(context.Background(), sweepDeps(reader, &fakeFinalizeProber{}), "repo", ops.seam(), SweepScope("bogus"))
	if res.Result != ResultInvalidInput || res.Reason != ReasonSweepScopeInvalid {
		t.Fatalf("want invalid-input/sweep-scope-invalid, got %q/%q", res.Result, res.Reason)
	}
	if len(ops.calls) != 0 {
		t.Fatalf("invalid scope must dispatch nothing; calls=%v", ops.calls)
	}
	if res.Scope != "bogus" {
		t.Errorf("refusal must echo the rejected scope, got %q", res.Scope)
	}
}

// TestSweepHumanTextNamesScopeAndDeferred: the human summary carries the
// resolved scope on every path, and in implementation scope a clearly-labeled
// deferred-count clause pointing at full maintenance.
func TestSweepHumanTextNamesScopeAndDeferred(t *testing.T) {
	r := newMaintenanceResult(ResultApplied, MaintenanceResult{Entries: []MaintenanceEntry{
		{ID: 30, Kind: sweepKindCloseout, Disposition: SweepDispApplied},
	}})
	r.Scope = "implementation"
	r.DeferredHistoricalCleanups = 234
	got := r.HumanText()
	if !strings.Contains(got, "scope implementation") || !strings.Contains(got, "234 historical cleanup(s) deferred") {
		t.Errorf("summary must name scope and deferred count, got %q", got)
	}
	full := newMaintenanceResult(ResultNoOp, MaintenanceResult{})
	full.Scope = "full"
	if !strings.Contains(full.HumanText(), "scope full") {
		t.Errorf("full-scope summary must name its scope, got %q", full.HumanText())
	}
}
```

Add `"strings"` to the test file's imports if absent.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestSweep' -count=1`
Expected: compile FAILURE (`maintenanceSweep` takes no scope argument; `SweepScopeImplementation`, `ReasonSweepScopeInvalid`, `res.Scope` undefined).

- [ ] **Step 3: Implement**

In `internal/app/maintenance.go`:

1. Type + constants, next to the sweep-kind constants:

```go
// SweepScope is the closed maintenance-sweep scope vocabulary (change 0389).
// full is the whole worklist — today's behavior, the default when the flag is
// omitted. implementation is the implementation-startup preflight: current
// merged-work closeouts (with their safe cleanup suffixes) and reclaim gating,
// with independent cleanup retries for records that were ALREADY terminal at the
// pinned inventory deferred to explicit full maintenance. The CLI resolves the
// scope once; the app layer never re-derives it from anything else.
type SweepScope string

const (
	SweepScopeFull           SweepScope = "full"
	SweepScopeImplementation SweepScope = "implementation"
)
```

2. New reason constant beside the existing `ReasonSweep*` block:

```go
	// ReasonSweepScopeInvalid: the typed scope was outside the closed
	// vocabulary; the sweep read nothing and dispatched nothing.
	ReasonSweepScopeInvalid = "sweep-scope-invalid"
```

3. `MaintenanceResult` gains two always-emitted fields (additive; protocol_version stays 1):

```go
	Scope                      string `json:"scope"`
	DeferredHistoricalCleanups int    `json:"deferred_historical_cleanups"`
```

4. Thread scope: `MaintenanceSweep(ctx, deps, repoDir, scope)` passes it to `maintenanceSweep(ctx, deps, repoDir, ops, scope)`. At the very top of `maintenanceSweep`, before `PinContext`:

```go
	stamp := func(r MaintenanceResult, deferred int) MaintenanceResult {
		r.Scope = string(scope)
		r.DeferredHistoricalCleanups = deferred
		return r
	}
	if scope != SweepScopeFull && scope != SweepScopeImplementation {
		return stamp(maintenanceRefusal(ResultInvalidInput, ReasonSweepScopeInvalid,
			fmt.Sprintf("unknown sweep scope %q: must be full or implementation", scope)), 0)
	}
```

Wrap every subsequent `return` in `stamp(..., 0)` for the refusal paths, and the final success return in `stamp(newMaintenanceResult(result, MaintenanceResult{Entries: entries}), deferred)`.

5. `sweepWorklist` gains the scope parameter and a second return. In the status switch:

```go
		case c.Status() == domain.StatusDone || c.Status() == domain.StatusStackedMerged:
			// Implementation scope defers records that were already terminal at
			// the pinned inventory: they are counted, never enqueued, so the
			// worklist stays independent of the historical population. A record
			// a closeout archives DURING this invocation is untouched by this
			// filter — its cleanup rides sweepRunCloseout's suffix, not this list.
			if scope == SweepScopeImplementation {
				deferredHistorical++
				continue
			}
			cleanups = append(cleanups, sweepWorkItem{id: int(c.ID()), kind: sweepKindCleanup})
```

6. `HumanText` names the scope on every path and, in implementation scope, appends the deferred clause. Replace the method body:

```go
func (r MaintenanceResult) HumanText() string {
	op := r.Operation
	if r.Scope != "" {
		op = fmt.Sprintf("%s (scope %s)", r.Operation, r.Scope)
	}
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		applied := 0
		for _, e := range r.Entries {
			if e.Disposition == SweepDispApplied {
				applied++
			}
		}
		out := fmt.Sprintf("%s: %d item(s), %d applied", op, len(r.Entries), applied)
		if r.Scope == string(SweepScopeImplementation) {
			// A count of candidates deliberately NOT probed — never a claim
			// they are dirty or blocked; explicit full maintenance owns them.
			out += fmt.Sprintf("; %d historical cleanup(s) deferred to `docket maintenance sweep --scope full`", r.DeferredHistoricalCleanups)
		}
		return out
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", op, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", op, r.Result)
}
```

7. Update the sole production caller `internal/cli/maintenance.go` minimally so the tree compiles — pass `app.SweepScopeFull` for now (Task 3 replaces this with real flag resolution).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ ./internal/cli/ -count=1`
Expected: PASS (including every pre-existing sweep test, now scoped full).

- [ ] **Step 5: Mutation-probe the scope filter**

Temporarily delete the `if scope == SweepScopeImplementation { deferredHistorical++; continue }` block; run `go test ./internal/app/ -run TestSweepImplementationScopeDefersHistorical -count=1` — it MUST fail (historical work invoked). Restore with `git checkout -- internal/app/maintenance.go`? **No** — the edit is uncommitted; restore by re-adding the block by hand (a `git checkout` here would destroy the whole task, per `mutation-restore-needs-a-backup-copy`). Re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/maintenance.go internal/app/maintenance_test.go internal/cli/maintenance.go
git commit -m "feat(0389): typed implementation scope for the maintenance sweep worklist"
```

---

### Task 2: App layer — growth test proving zero per-historical amplification

**Files:**
- Test: `internal/app/maintenance_test.go`

**Interfaces:**
- Consumes: Task 1's `maintenanceSweep(..., scope)` and `MaintenanceResult.DeferredHistoricalCleanups`; the `StatusReader` and `FinalizePRProber` interfaces as declared in `internal/app` (read their declarations — implement every method by delegation).
- Produces: `countingReader`, `countingProber` test doubles reusable by later tests.

- [ ] **Step 1: Write the failing test**

```go
// countingReader delegates to an inner StatusReader and counts authority reads.
// Implement EVERY method of the StatusReader interface by delegation (grep
// `type StatusReader interface` in this package for the current method set),
// incrementing pins on PinContext and corpusReads on ReadCorpus.
type countingReader struct {
	inner       StatusReader
	pins        int
	corpusReads int
}

// countingProber delegates to an inner FinalizePRProber counting every probe.
// Implement the interface's method set by delegation, incrementing probes once
// per call.
type countingProber struct {
	inner  FinalizePRProber
	probes int
}

// TestSweepImplementationScopeDoesNotGrowWithHistory: growing ONLY the
// historical done population 0 -> 300 -> 1000 must not change the cleanup
// dispatch count, the per-item authority reload count, or the remote probe
// count in implementation scope. Reading/parsing the larger corpus is allowed —
// the invariant is per-item work, not corpus size. Full scope on the same
// corpora DOES grow, proving the filter is scope-keyed rather than dead.
func TestSweepImplementationScopeDoesNotGrowWithHistory(t *testing.T) {
	type counts struct{ cleanups, pins, probes, deferred int }
	measure := func(t *testing.T, historical int, scope SweepScope) counts {
		t.Helper()
		corpus := []StatusBlob{
			finalizeBlob(30, "current", "implemented", "high", prRefFor(30), ""),
			sweepInProgressBlob(50, "stale"),
		}
		for i := 0; i < historical; i++ {
			id := 1000 + i
			corpus = append(corpus, finalizeBlob(id, fmt.Sprintf("hist%04d", i), "done", "high", prRefFor(id), ""))
		}
		cr := &countingReader{inner: &fakeReader{pin: sweepPin(t, true, 24), corpus: corpus}}
		cp := &countingProber{inner: &fakeFinalizeProber{facts: map[string]domain.PRFacts{
			prRefFor(30): withHead(mergedFacts(30, "main"), "feat/current"),
		}}}
		ops := &recordingSweepOps{}
		deps := FinalizeDeps{Planning: PlanningDeps{Reader: cr, Clock: testClock()}, PRProber: cp}
		res := maintenanceSweep(context.Background(), deps, "repo", ops.seam(), scope)
		return counts{
			cleanups: len(ops.callIDs(sweepKindCleanup)),
			pins:     cr.pins,
			probes:   cp.probes,
			deferred: res.DeferredHistoricalCleanups,
		}
	}

	base := measure(t, 0, SweepScopeImplementation)
	for _, n := range []int{300, 1000} {
		got := measure(t, n, SweepScopeImplementation)
		if got.cleanups != base.cleanups || got.pins != base.pins || got.probes != base.probes {
			t.Errorf("historical=%d amplified work: got %+v, base %+v", n, got, base)
		}
		if got.deferred != n {
			t.Errorf("historical=%d: deferred = %d, want %d", n, got.deferred, n)
		}
	}

	// Scope-keyed, not dead: full scope grows with the archive.
	full0, full300 := measure(t, 0, SweepScopeFull), measure(t, 300, SweepScopeFull)
	if full300.cleanups-full0.cleanups != 300 {
		t.Errorf("full scope must retain historical retries: cleanups %d -> %d", full0.cleanups, full300.cleanups)
	}
}
```

Note: a probe is issued per non-terminal PR-bearing change — the historical done records are terminal, so `probes` is already flat; the assert pins that this stays true. If `StatusReader`/`FinalizePRProber` have additional methods, delegate them verbatim.

- [ ] **Step 2: Run the test to verify it fails for the right reason**

Run: `go test ./internal/app/ -run TestSweepImplementationScopeDoesNotGrowWithHistory -count=1`
Expected: compile FAIL only until the doubles implement their interfaces; once compiling, PASS is expected immediately (Task 1 already ships the filter). To prove the test can fail, run Step 3's mutation probe — a growth test that has never been red is decoration.

- [ ] **Step 3: Mutation-probe**

Temporarily change the Task 1 filter from `scope == SweepScopeImplementation` to `scope == SweepScope("never")`; run the test with `-count=1` — it MUST redden on the implementation-scope legs (pins and cleanups grow with n). Restore the constant by hand; re-run to green.

- [ ] **Step 4: Commit**

```bash
git add internal/app/maintenance_test.go
git commit -m "test(0389): growth guard — implementation scope is flat in the historical population"
```

---

### Task 3: CLI — `--scope` flag, closed-vocabulary refusal before side effects

**Files:**
- Modify: `internal/cli/maintenance.go`
- Test: `internal/cli/maintenance_test.go`

**Interfaces:**
- Consumes: `app.SweepScope`, `app.SweepScopeFull`, `app.SweepScopeImplementation`, `app.MaintenanceSweep(ctx, deps, repoDir, scope)` from Task 1.

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/maintenance_test.go`:

```go
// TestMaintenanceSweepScopeFlag: the closed --scope flag is registered with
// full as its omitted default, and the RESOLVED scope is echoed in the
// envelope — asserting the non-default value proves the wiring, since a
// defaulted parameter hides a dropped argument (learnings:
// defaulted-param-hides-caller-wiring).
func TestMaintenanceSweepScopeFlag(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"maintenance", "sweep"})
	if err != nil || cmd == nil {
		t.Fatalf("maintenance sweep not registered: %v", err)
	}
	f := cmd.Flags().Lookup("scope")
	if f == nil || f.DefValue != "full" {
		t.Fatalf("--scope must exist with default full, got %+v", f)
	}

	out, errS, _ := runCLI(t, "maintenance", "sweep", "--repo-dir", t.TempDir(), "--json")
	if errS != "" || !strings.Contains(out, `"scope":"full"`) {
		t.Errorf("omitted scope must resolve and echo full: out=%q err=%q", out, errS)
	}
	out, errS, _ = runCLI(t, "maintenance", "sweep", "--scope", "implementation", "--repo-dir", t.TempDir(), "--json")
	if errS != "" || !strings.Contains(out, `"scope":"implementation"`) {
		t.Errorf("explicit implementation must reach the operation and echo back: out=%q err=%q", out, errS)
	}
}

// TestMaintenanceSweepScopeRefusedBeforeWork: an unknown or empty explicit
// scope is refused before any repo/network/mutation work — no maintenance.sweep
// document is ever produced.
func TestMaintenanceSweepScopeRefusedBeforeWork(t *testing.T) {
	for _, bad := range []string{"bogus", ""} {
		out, errS, code := runCLI(t, "maintenance", "sweep", "--scope", bad, "--repo-dir", t.TempDir(), "--json")
		if code == 0 {
			t.Errorf("scope %q must fail, exit=0", bad)
		}
		if strings.Contains(out, `"operation":"maintenance.sweep"`) {
			t.Errorf("scope %q must refuse BEFORE dispatching the operation: %q", bad, out)
		}
		if !strings.Contains(errS, "scope") {
			t.Errorf("scope %q: diagnostic must name the flag, got %q", bad, errS)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli/ -run TestMaintenanceSweepScope -count=1`
Expected: FAIL (`--scope` flag not registered).

- [ ] **Step 3: Implement**

In `newMaintenanceSweepSubcommand`'s `RunE`, resolve the scope FIRST — before `resolveRepoDir` and `newFinalizeDeps` — so the refusal precedes every repo/network seam:

```go
		RunE: func(c *cobra.Command, _ []string) error {
			// Scope resolves once, here, to a typed value — the app layer never
			// re-derives it — and an unknown/empty value refuses before any
			// repo, network, or mutation work is even wired.
			scopeStr, err := c.Flags().GetString("scope")
			if err != nil {
				return err
			}
			var scope app.SweepScope
			switch scopeStr {
			case "full":
				scope = app.SweepScopeFull
			case "implementation":
				scope = app.SweepScopeImplementation
			default:
				return fmt.Errorf("invalid --scope %q: must be full or implementation", scopeStr)
			}
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.MaintenanceSweep(c.Context(), deps, repoDir, scope))
			return nil
		},
```

Register the flag with its documented default:

```go
	cmd.Flags().String("scope", "full", "sweep scope: full (whole worklist, the default) or implementation (startup preflight; defers independent historical cleanup retries)")
```

Add `"fmt"` to imports. Update the file's header comment (the "Only the target directory rides on a flag" sentence in `newMaintenanceSweepSubcommand`'s doc comment is now false — reword to name the two flags).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS (note `TestMaintenanceSweepRegistered` still passes — it checks `--repo-dir` presence only).

- [ ] **Step 5: Mutation-probe the wiring**

Temporarily change `setResult(app.MaintenanceSweep(..., scope))` to pass `app.SweepScopeFull` unconditionally; run `go test ./internal/cli/ -run TestMaintenanceSweepScopeFlag -count=1` — MUST redden on the `"scope":"implementation"` assert. Restore by hand; re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/maintenance.go internal/cli/maintenance_test.go
git commit -m "feat(0389): closed --scope flag on maintenance sweep, refused before side effects"
```

---

### Task 4: Repo-wide derivation of affected `maintenance sweep` sites

This task derives the prose surface Tasks 5–8 edit — never hand-list the sites (AGENTS.md).

**Files:**
- Create: nothing durable — the classified list lands in this task's worker report and drives Tasks 5–8's "verify coverage" steps.

- [ ] **Step 1: Enumerate every site**

Run from the worktree root (capture into a variable first — never pipe into an early-exiting consumer under pipefail):

```bash
hits=$(grep -rn "maintenance sweep" . \
  --include='*.md' --include='*.go' --include='*.sh' --include='*.yml' \
  | grep -v -E '^\./(\.git|docs/changes/archive|docs/superpowers/specs|docs/superpowers/plans|docs/results|docs/adrs|internal/assets/embedded|\.worktrees)')
printf '%s\n' "$hits"
```

Also sweep the sibling spellings: `grep -rn -e "--scope" -e "step-0 safety net" -e "self-cleaning safety net" -e "never an in-context return"` over the same include set — the last one because Task 6/7 rewrite that contract clause and dependent guards may grep it.

- [ ] **Step 2: Classify prose vs executable**

Sort every hit into: (a) **maintained instruction/executable** — skill bodies, convention references, `agents/*.md`, `cursor-rules/*`, README, Go doc comments (an agent runs command blocks in these; they are executable surface, per `agent-executed-markdown-is-code`); (b) **point-in-time record** — anything under the excluded dirs plus any archived/frozen file the grep still caught: leave byte-untouched; (c) **generated mirror** — `internal/assets/embedded/**`: never hand-edited, regenerated by Tasks 5–8's genassets steps. Expected category-(a) population from authoring-time derivation: `skills/docket-status/SKILL.md` (6 sites), `skills/docket-convention/SKILL.md` (1), `skills/docket-convention/references/stacked-changes.md` (2), `skills/docket-implement-next/SKILL.md` (step-0 prose), `agents/docket-status.md` + `cursor-rules/dispatch/docket-status.md` (sweep-adjacent wording), Go comments in `internal/app/{maintenance,change_reclaim,repository_prepare}.go`. The grep, not this list, is authoritative — report any site the list misses.

- [ ] **Step 3: Judge each category-(a) site**

For each: does it *imply a full historical sweep at every implementation startup*, or *instruct running the sweep* without scope? If yes it belongs to Task 5 (docket-status), Task 6 (implement-next), Task 7 (convention + its references), or Task 8 (everything else). A site that stays true under both scopes (e.g. `stacked-changes.md`'s "the merge sweep is the only producer of the state" — closeouts run in both scopes) is recorded as verified-unchanged with one line of reasoning. Hand the classified list to the remaining tasks.

- [ ] **Step 4: No commit** — this task produces the report only.

---

### Task 5: Prose — docket-status skill: three-way mode choice + command completion barrier

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Regenerate: `internal/assets/embedded/**` (via `go generate ./internal/assets/`)

- [ ] **Step 1: Grep for dependents of every sentence you will touch**

Before editing, for each clause being rewritten (the two `## Mode choice` bullets, the `## Run the pass` command block, the `### Merge sweep` "Runs automatically at `docket-implement-next` step 0" sentence), run `grep -rn -F "<distinctive fragment>" internal/repoguard/ tests/` and note dependents. Repoint any dependent row in the same commit (`restatement-accumulates-its-own-guards`).

- [ ] **Step 2: Rewrite `## Mode choice` to three modes**

Replace the second bullet with:

```markdown
- **`docket-implement-next`'s step-0 implementation preflight** ⇒ run `docket maintenance sweep --scope implementation --json` first — current merged-work recovery (closeouts with their safe cleanup suffixes) plus reclaim gating, with independent historical cleanup retries deferred and counted in the envelope's `deferred_historical_cleanups` field — then read the refreshed state with `docket status --json`.
- **An explicit refresh/cleanup request** — or a post-merge cleanup after a PR merged via the GitHub button ⇒ run `docket maintenance sweep --scope full --json` first (merge sweep + historical cleanup retries + health checks + judgment lines + integration sync), then read the refreshed state with `docket status --json`.
```

Keep the see-only bullet as is.

- [ ] **Step 3: Update the sweep section and command block**

In `## Maintenance sweep — the merged-PR recovery mutation (only when asked)`, after the paragraph describing what the sweep does, add:

```markdown
`--scope` is a closed vocabulary. `full` — the default when omitted — is the whole worklist,
including cleanup retries for every `done`/`stacked-merged` record. `implementation` is the
implementation-startup preflight: it still schedules every current merged-implemented closeout, the
cleanup suffix such a closeout carries in the same invocation, and the `reclaim.auto`-gated
reclaims, but it does not enqueue an independent cleanup for a record that was already terminal at
the pinned inventory. Those are reported as a deferred COUNT — a population deliberately not
probed, never evidence anything is dirty or blocked; explicit full maintenance owns those retries,
and a failed or interrupted suffix stays recoverable through `--scope full` or the targeted
finalize cleanup.
```

Update the `## Run the pass` code block to show the scoped forms:

```
docket maintenance sweep --scope <full|implementation> --json   # mutation, scope per Mode choice
docket status --json                                            # write-free read over the refreshed state
```

In `### Merge sweep`, change "Runs automatically at `docket-implement-next` step 0 and on any explicit refresh/cleanup `docket maintenance sweep` invocation." to "Runs at `docket-implement-next` step 0 in implementation scope, and in full scope on any explicit refresh/cleanup invocation."

- [ ] **Step 4: Add the command completion barrier section**

Insert a new section between `## Run the pass` and `## Read the report — it is the only channel you need`:

```markdown
## Completion barrier — observe the sweep to its terminal result

Starting the sweep command is not finishing it. If the shell tool's foreground window expires and
the harness moves the still-running sweep to the background, that is
a liveness transition, not completion — and not a failed sweep. You stay responsible for that
exact task: keep the task identity the harness returned and collect that task's terminal result
through the harness's native observation/wait mechanism. Never start a second shell watcher, never
poll the output file's size, never sleep-and-tail, never re-run the sweep, and never return a
success report while the process remains unobserved. An output file turning nonempty, metadata
commits appearing on `origin/docket`, elapsed time, or some separate command succeeding are not
completion signals.

Only the sweep's actual terminal protocol-v1 envelope completes the command. Validate it and every
entry: `protocol_version` `1`, `operation` `maintenance.sweep`, a `scope` equal to the one you
requested, and each entry's closed disposition. Retain the original structured output — a harness
result handle or a task-local output artifact — and extract the compact summary plus any problem
entries in one read/parse rather than reopening the full output repeatedly; stdout is the JSON
document, and any progress diagnostics belong on a separate channel. Only after that terminal
validation run the post-sweep `docket status --json` read. A read taken after a failed or
unvalidated sweep is diagnostic only — label it as diagnostic; it can never authorize selection.

The envelope's top-level `applied` means some work applied,
never that every item succeeded. A `blocked`, `failed`, or `unknown` entry — or a `contended`
entry on work this preflight required — is a failed preflight even under an `applied` envelope:
surface it per *Read the report* below and stop, per the *Scope of this stop* rule above.
Intentional policy skips (`reclaim-auto-disabled`) and genuine no-ops remain non-errors — never
collapse arbitrary `skipped` reasons into success. On cancellation, request cancellation of the
exact owned task where the harness supports it, then observe its termination; never broadly kill
processes, abandon a watcher, or spawn a replacement sweep while the prior one may still run. If
quiescence cannot be established, halt naming the exact live task identity and preserve its
output — an explicit failure here beats claiming an orphan-free exit.

When a caller dispatched this skill, the final report must name: the resolved scope, the sweep's
terminal envelope result, every problem entry, where the original sweep and status outputs live,
and the post-sweep metadata revision (`git -C <metadata worktree dir> rev-parse HEAD`). The prose
re-summary is never a second authority — the caller verifies against the originals.
```

Keep each of these three anchor fragments unwrapped on one physical line (they become repoguard rows in Task 9): `a liveness transition, not completion`; `never that every item succeeded`; `never start a second shell watcher`.

- [ ] **Step 5: Regenerate the embedded mirror and verify**

Run: `go generate ./internal/assets/` then `go run ./cmd/genassets -check` (expect "matches the authored roots"), then `go test ./internal/repoguard/ -count=1` (the structural scanners — config-read-channel, inline-role-stop — must stay green; if the stop-scoping scanner flags the new "stop" mentions, follow the scanner's own remedy and scope them the way the existing *Scope of this stop* block is scoped, rather than weakening the scanner).

- [ ] **Step 6: Commit**

```bash
git add skills/docket-status/SKILL.md internal/assets/embedded
git commit -m "docs(0389): docket-status — scoped mode choice + sweep completion barrier"
```

---

### Task 6: Prose — docket-implement-next Step 0: scoped dispatch + agent completion barrier

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (the Step 0 second paragraph)
- Regenerate: `internal/assets/embedded/**`

- [ ] **Step 1: Grep for dependents**

`grep -rn -F "git state, not an in-context return" internal/repoguard/ tests/ skills/` and likewise for "self-cleaning safety net" — repoint any dependent assert in this commit.

- [ ] **Step 2: Replace Step 0's dispatch paragraph**

Replace the entire second paragraph of `### Step 0 — Sync & sweep` ("Then, before selection, **dispatch the `docket-status` subagent** …") with:

```markdown
Then, before selection, **dispatch the `docket-status` subagent** (foreground, at the model/effort
its wrapper resolves) with an explicit **implementation-preflight** request: the child runs
`docket maintenance sweep --scope implementation` — current merged-work recovery, without the
independent historical cleanup retries that explicit full maintenance owns — and then the
post-sweep `docket status --json` read. The dispatch is **unconditional**, and its proof is
twofold: commits on `origin/docket`, surfaced by a fresh metadata re-sync, **and** the child's
terminal evidence in its return. **Accept the return only with terminal sweep evidence for implementation scope**:
the sweep's terminal protocol-v1 envelope result whose `scope` is `implementation`, its problem
entries, references to the original sweep/status outputs, and a successful post-sweep status read.
A bare `completed` task label, plausible-looking git effects, or a report that the sweep is "still
running" does **not** satisfy this barrier — a no-op sweep still needs its completion evidence. A
`blocked`/`failed`/`unknown` entry — or a `contended` entry on required preflight work — is a
failed preflight handoff: **halt before claiming**, through the pre-claim run-reporting path
(before a claim there is no owned change record to write a halt on — never mutate an unrelated
proposed record to carry it). A protocol error, a scope mismatch, missing output, an incomplete
child return, or an unavailable completion-observation mechanism halts the same way; no new
run-gate state exists for any of this. Correlate any late completion notification by exact
child/task identity: a genuine duplicate of an already-consumed terminal result is dismissed
without re-running Step 0 and without disturbing the current build worker, but a FIRST terminal
result arriving after the parent advanced is a contract violation, not a dismissable duplicate —
halt and surface it. After consuming the result, verify the child is terminal or retired through
the harness's actual lifecycle mechanism (where retirement is automatic, verifying the terminal
state suffices — a retained historical UI row is not a leak, and no cross-harness close API is to
be invented): success requires that no sweep or watcher owned by this preflight still runs. Then
re-sync (`docket repository prepare`) and derive readiness and claim state from fresh origin — the
child's evidence is control-flow proof, never a substitute for metadata authority, and its
pre-sweep ready data is stale by construction. If no dispatch mechanism resolves per the
convention's *Dispatch-capability resolution* — never from a tool name — the `docket-status`
dispatch is **Tier A**: run the same scoped sweep inline under the same completion barrier, an
equivalent path, neither a degradation nor a warning.
```

Keep these anchors unwrapped on one physical line each (Task 9 rows): `terminal sweep evidence for implementation scope` (bolded fragment above); `a contract violation, not a dismissable duplicate`; `--scope implementation`.

- [ ] **Step 3: Regenerate, guard-check, commit**

Run `go generate ./internal/assets/`, `go run ./cmd/genassets -check`, `go test ./internal/repoguard/ -count=1` (repoint any reddened row in this commit — the existing rows anchored on this file, e.g. `docket-plan-writer` and the loop-continuation phrases, live in untouched sections and must stay green).

```bash
git add skills/docket-implement-next/SKILL.md internal/assets/embedded
git commit -m "docs(0389): implement-next step 0 — implementation-scope dispatch + agent barrier"
```

---

### Task 7: Prose — convention composition clause: hybrid status contract, scoped startup

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the `**Composition (change 0017).**` paragraph)
- Modify (only if Task 4 classified them as implying a full startup sweep): `skills/docket-convention/references/stacked-changes.md`
- Regenerate: `internal/assets/embedded/**`

- [ ] **Step 1: Grep for dependents**

The composition paragraph carries phrases existing repoguard rows anchor (`to await a task-notification` must survive verbatim — do not touch that sentence). `grep -rn -F "never an in-context return" internal/repoguard/ tests/` before editing.

- [ ] **Step 2: Edit the composition paragraph**

In the sentence "These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket`, re-read after a re-sync — never an in-context return." — split the two dispatches:

```markdown
These dispatches are **foreground** (the parent suspends until the child returns) and
**unconditional**. The `docket-adr` dispatch's contract is **git state** on `origin/docket`,
re-read after a re-sync — never an in-context return. The `docket-status` step-0 dispatch is
**hybrid** (change 0389): its mutations are git state on `origin/docket`, re-read after a re-sync,
**and** its return must carry the scoped sweep's terminal protocol-v1 evidence plus a successful
post-sweep status read — the parent accepts no bare `completed`, and a first terminal result
arriving after the parent advanced is a contract violation, never a dismissable duplicate. The
step-0 sweep runs at implementation scope (`docket maintenance sweep --scope implementation`) —
current merged-work recovery and reclaim gating with independent historical cleanup retries
deferred to explicit full maintenance, so implementation startup no longer implies a full historical sweep.
```

Keep `no longer implies a full historical sweep` unwrapped on one line (Task 9 anchor). Leave the rest of the paragraph (plan-writer hybrid, auto-groom-critic, foreground/never-yield sentences) untouched.

- [ ] **Step 3: Apply Task 4's verdicts on the convention references**

For each `stacked-changes.md` site Task 4 judged: edit only if it instructs or implies an unscoped/full sweep at implementation startup; otherwise record verified-unchanged. (Authoring-time expectation: both sites describe the sweep as producer/consumer of stacked state, true under both scopes — likely unchanged.)

- [ ] **Step 4: Regenerate, guard-check, commit**

`go generate ./internal/assets/`; `go run ./cmd/genassets -check`; `go test ./internal/repoguard/ -count=1`.

```bash
git add skills/docket-convention internal/assets/embedded
git commit -m "docs(0389): convention — hybrid status dispatch contract, implementation-scope startup"
```

---

### Task 8: Prose — residual maintained sites from the Task 4 derivation

**Files:**
- Modify: whatever category-(a) sites Task 4 assigned here (expected: `agents/docket-status.md` / `cursor-rules/dispatch/docket-status.md` sweep wording if it implies an unscoped pass; Go doc comments in `internal/app/change_reclaim.go` / `internal/app/repository_prepare.go` if their `maintenance sweep` references became scope-ambiguous; `README.md` if it surfaced)
- Regenerate: `internal/assets/embedded/**` if any `skills/` or `agents/` file changes

- [ ] **Step 1: Work the residual list**

For each remaining site: apply the same test — does it instruct running the sweep at implementation startup, or imply the startup pass covers the whole archive? Fix instruction sites to be scope-aware (e.g. a dispatch example gains "implementation preflight" or "--scope full" wording matching Task 5's mode names); leave true-under-both-scopes descriptions with a one-line verified-unchanged note in the worker report. Go comment edits must keep symbol-anchored cross-references (never line numbers).

- [ ] **Step 2: Verify nothing was missed**

Re-run Task 4 Step 1's grep; every remaining hit must now be classified (b) point-in-time or (c) generated, or carry a verified-unchanged verdict. Paste the final classification into the worker report.

- [ ] **Step 3: Regenerate if needed, run gofmt, commit**

If `skills/` or `agents/` changed: `go generate ./internal/assets/` + `go run ./cmd/genassets -check`. If Go files changed: `gofmt -l internal/` must print nothing.

```bash
git add -A -- <each explicit path touched> internal/assets/embedded
git commit -m "docs(0389): scope-aware wording at residual maintained sweep sites"
```

---

### Task 9: Guard the new prose — repoguard prose-contract rows, mutation-tested

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go`

**Interfaces:**
- Consumes: the exact anchor phrases Tasks 5–7 kept unwrapped (listed per task above).

- [ ] **Step 1: Add the rows**

Append to the `proseContracts` table, following its house comment style:

```go
	// change 0389 — implementation-scope sweep + the two completion barriers.
	// docket-status owns the COMMAND barrier: a backgrounded sweep is observed
	// to its terminal envelope, never declared done by proxy signals; and an
	// applied envelope is never read as all-items-succeeded.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-status/SKILL.md",
		present: []string{"--scope implementation", "a liveness transition, not completion",
			"never start a second shell watcher", "never that every item succeeded"}},
	// docket-implement-next owns the AGENT barrier: terminal evidence for the
	// requested scope, and a first-late terminal result is a violation.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"--scope implementation", "terminal sweep evidence for implementation scope",
			"a contract violation, not a dismissable duplicate"}},
	// The convention no longer implies a full historical sweep at startup, and
	// the status dispatch contract is hybrid.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-convention/SKILL.md",
		present: []string{"no longer implies a full historical sweep"}},
```

If the table's population-floor assert (search "Population floor" in the same file) is an exact count rather than a floor, update it per its own remedy comment; if it is a `>=` floor, leave it.

- [ ] **Step 2: Run to verify the rows pass against the real files**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1` (adjust `-run` to the actual test name in the file). Expected: PASS. If any phrase fails, the Task 5–7 prose wrapped it — fix the wrap in the skill file (and regenerate embedded), never by weakening the phrase.

- [ ] **Step 3: Mutation-test every row**

Tasks 5–7 are committed, so `git checkout --` is now a safe restore. For each of the three files: delete the anchored clause from the skill file, run the test with `-count=1`, confirm RED naming that file, then `git checkout -- <file>`. All three probes must redden. (Do not regenerate embedded during probes — the guard reads the authored source.)

- [ ] **Step 4: Commit**

```bash
git add internal/repoguard/prose_contracts_test.go
git commit -m "test(0389): prose-contract rows for scope wiring and both completion barriers"
```

---

### Task 10: Full-suite gate, evidence, and the human close-out register

**Files:**
- No new source files. The worker report from this task feeds the run's results file and PR body.

- [ ] **Step 1: Run the whole suite at the build gate**

Run from the feature worktree root: `go run ./cmd/docket development test`
Expected: the suite summary line green. Treat any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative finding to act on; a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is a screening finding to record. Read the summary line, never the piped exit code. Never background this gate and yield — drive it to completion inline.

- [ ] **Step 2: Record the mutation evidence**

The report lists each probe already performed (Task 1 Step 5, Task 2 Step 3, Task 3 Step 5, Task 9 Step 3) with its red output line — a mutation that never reddened is an unresolved defect, not a footnote.

- [ ] **Step 3: Write the human-verified close-out register**

These are **acceptance criteria a human verifies at the merge gate** — record them in the results file and PR body as open close-out items, never as automated build steps and never as claims already achieved:

1. **Measured performance** (spec §6, "Measured performance"): same isolated fixture repo/machine/command version, seeded ≥ the reported 388-change shape with 234 historical cleanup candidates and no current closeout/reclaim work; prepare/sweep/read/coordination timed separately, ≥3 runs per variant, median + individual timings; implementation scope must eliminate all 234 historical cleanup attempts and cut median sweep time ≥90% vs the pre-change full baseline; a second fixture with real current closeouts proves they still run; fresh-Claude startup duration recorded separately. Never benchmarked against the live backlog with destructive cleanup. A green correctness suite does NOT stand in for this (`optimization-needs-a-measured-oracle`); missing the target is unresolved performance work.
2. **Fresh-Claude lifecycle certification** (spec §6, "Fresh Claude certification"): disposable fixture repo + controlled sweep command forced past the foreground window with a controlled release; prove no early child success, no selection before both barriers, no surviving sweep/watcher after retirement; repeat with an applied-envelope-containing-blocked-entry, with cancellation, and with a late duplicate notification. Any timing fixture used here needs an independent hard stop so a removed guard fails boundedly instead of hanging (`mutation-target-needs-a-forced-exit`). Record Claude version, mode, installed skill revision, task identities, timestamps. If the harness prevents a required observation, record the attempt and limitation and halt that run — do not mark runtime behavior verified on unit-test evidence.
3. **Restart precondition**: skill bodies are loaded at harness process start (`generated-artifact-loaded-at-process-start`) — the barrier prose is runtime-exercised only in a session started after `docket development install` refreshes the installed copies. The editing session's own behavior is not evidence.

- [ ] **Step 4: ADR note for the implementer**

The scope design introduces one candidate non-obvious decision: *the implementation/full scope split is resolved once in the CLI to a closed typed value, deferral is keyed solely on terminal-at-pinned-inventory status (never age/config/caller identity), and deferred candidates are reported as an unprobed count rather than per-item outcomes.* At the run's normal ADR step (implement-next Step 6, via the `docket-adr` dispatch), raise this for a recorded decision if judged non-obvious; do not edit any existing ADR.

---

## Self-review (performed at authoring)

- Spec §1 (explicit scope) → Tasks 1–3. §2 (caller wiring, tracked sources, derivation) → Tasks 4–8; tracked source of truth confirmed as `skills/*/SKILL.md` with `internal/assets/embedded` a genassets-generated mirror and `~/.claude/skills/` machine-local installs. §3 (two barriers) → Tasks 5–6, convention reconciliation Task 7. §4 (failure/cancellation) → prose in Tasks 5–6; data-layer applied≠all-success pinned by the existing `TestSweepItemIsolation` plus Task 1's dispositions preserved unchanged. §5 (bounded surfaces) → file lists above; `FinalizeCleanup` untouched. §6 (verification) → Go regression in Tasks 1–3, growth in Task 2, prose guards Task 9, whole suite + human register Task 10.
- Type consistency: `SweepScope`/`SweepScopeFull`/`SweepScopeImplementation`, `maintenanceSweep(ctx, deps, repoDir, ops, scope)`, `sweepWorklist(..., scope) (items, deferredHistorical)`, JSON `scope` / `deferred_historical_cleanups` are used identically in Tasks 1, 2, 3, and 5.
- No placeholders remain; every prose task carries the actual replacement text, every code step the actual code.
