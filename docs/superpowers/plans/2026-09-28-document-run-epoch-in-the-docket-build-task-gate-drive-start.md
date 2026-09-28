<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0467 — Scoped gate starts inherit the run epoch; thread it through the build chain](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0467-document-run-epoch-in-the-docket-build-task-gate-drive-start.md)**
<!-- docket:backlink:end -->
# Scoped Gate Starts Inherit the Run Epoch — Implementation Plan

> **For agentic workers:** this plan is executed by the resolved build skill (`docket-build`),
> task-by-task: one named build-profile worker per task under the `docket-build-task` contract,
> strictly sequential, then one full-suite gate. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a scoped `gate.drive.start` take its run epoch from the scope it was prepared under,
and thread the parent's run epoch through the prose chain (run-gate block → implement-next →
docket-build) to every `gate.drive.prepare-scope` and build-owned `gate.drive.start`, so build-task
workers never handle the epoch.

**Architecture:** One driver change in `internal/gatedrive/driver.go`: `precheckScopedStart` (which
already loads the scope) resolves the *effective* run epoch and `Admit` substitutes it into its own
copy of the request before every downstream use (epoch gate, scoped worktree admission record,
finished-incumbent reconciliation, admission ticket). A presented epoch that differs from the
scope's pinned one fails the existing `scope-identity-mismatch` check. Prose edits thread the epoch
only as far as the calls that mint a scope or start a scope-less build-owned drive; a new repoguard
prose-contract test pins that, keyed on syntactic shape.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `go test`, markdown skill bodies,
`go generate` for the embedded asset tree, the `internal/document` managed-block patcher for the
committed `AGENTS.md` dispatch block.

**Spec:** `docs/superpowers/specs/2026-09-28-document-run-epoch-in-the-docket-build-task-gate-drive-start-design.md`
(on the `docket` metadata branch; read it before starting any task).

## Global Constraints

- Every test run whose purpose is to observe a change in outcome — RED/GREEN checks, mutation probes,
  manual re-verification — uses `go test -count=1`. A `(cached)` result is absence of evidence.
- Mutation-test restore is **copy-back**, never `git checkout --`: `cp "$f" "$f.bak"; <mutate>;
  <run>; mv -f "$f.bak" "$f"`. Confirm a prose mutation actually landed by counting over a
  whitespace-flattened copy (`tr -s '[:space:]' ' ' < "$f" | grep -o -F -- '<phrase>' | wc -l`),
  before and after — a count that does not drop means the mutation never applied.
- **Skill prose edits need embedded-copy regeneration**: after editing anything under `skills/`,
  `agents/`, or `cursor-rules/`, run `go generate ./internal/assets/` from the worktree root (the
  `//go:generate go run ../../cmd/genassets -repo ../..` directive in `internal/assets/generate.go`),
  which rewrites `internal/assets/embedded/tree/**` and `internal/assets/embedded/manifest.json`.
  Never hand-edit the embedded tree. `go test -count=1 ./internal/assets/` proves the mirror matches.
- **The managed run-gate block is regenerated, never hand-edited.** The committed `AGENTS.md`
  `docket:dispatch` block is the output of `harness.CodexDispatchInterior(harness.RunGate(catalog))`
  over the embedded catalog; `TestCommittedCodexDispatchMatchesGenerator`
  (`internal/repoguard/root_entry_dispatch_test.go`) fails on any byte difference. Task 3 gives the
  exact one-shot regeneration procedure. `CLAUDE.md` is a symlink to `AGENTS.md` — never replace it
  with a regular file, never write through `CLAUDE.md`.
- Size budgets are pinned at exact counts: `skillBudgets` and `dispatchBudget` in
  `internal/repoguard/budgets_test.go`. Re-baseline a row to the **exact** new `wc -l` / `wc -w`
  counts with a leading `0467:` note naming what grew; `dispatchBudget` must stay strictly below
  `dispatchOld` (1156).
- Out of scope — do not touch: the run-epoch fence in `reserveWorktreeExecution`, `stale-run-epoch`
  semantics, epoch settlement (`EpochSettledFunc`), scope-less start behaviour, and deriving the epoch
  from the dispatch-context token.
- The build-task worker's scoped `gate.drive.start` argv is **unchanged**; workers never receive or
  pass `--run-epoch`.
- Skill bodies ship into other repositories: no sentence may be true only in this repo.
- Cross-references in maintained source anchor on a symbol name or quoted clause, never a line number.
- Stage only the paths the task names; never `git add -A` / `git add .`.

## Review Focus

1. **A successor start in the same scope presenting no epoch** (a worker's RED/GREEN/verification
   drives after its baseline) must inherit the scope's epoch through the successor/rotation path, not
   only the first-start path — pinned by `TestScopedSuccessorStartInheritsScopeEpoch` (Task 1).
2. **A worker that improvises a different `--run-epoch`** must be refused `scope-identity-mismatch`
   with nothing reserved (slot bytes, scope slot, launch count untouched), on both a first start and a
   successor start — `TestScopedStartForeignEpochRefused` and the foreign-successor leg of
   `TestScopedSuccessorStartInheritsScopeEpoch` (Task 1).
3. **An epoch-less (legacy v2, or prepared-without) scope over an epoch-owned slot** keeps today's
   behaviour — the presented value governs, so an empty one is still refused `stale-run-epoch` —
   `TestEpochlessScopeKeepsPresentedEpoch` (Task 1).
4. **The WAITING-handoff continuation's re-prepared scope** in `docket-build` must carry the epoch too,
   or every continued worker start is refused `stale-run-epoch` — the prong-A per-file floor of 2
   prepare-scope sites in `skills/docket-build/SKILL.md` (Task 2).
5. **Re-flowed prose**: a `--run-epoch` flag or the run-gate binding phrase that wraps across a line
   break must still match — the whitespace-collapse cases in the `non_vacuity` subtests (Tasks 2, 3).

**Recorded residuals (not fixed by this change, stated so review does not rediscover them):**
- `GateDriveService.startBudgetedBuild` (`internal/app/gate_drive.go`) runs its advisory
  `ReconcileFinishedIncumbent(req.Worktree, req.RunEpochID)` with the *presented* epoch before
  `Admit`. For a scoped build-owned start that omits the epoch over a busy slot, the advisory check
  can refuse where the driver would have settled. Build-owned starts in the shipped prose are
  scope-less and now pass `--run-epoch` explicitly, so this path is not exercised by the workflow.
- The Codex `agent.enter` route carries the dispatch context in its request file; whether the epoch
  reaches the child's prompt on that route is governed by `agent.enter --run-epoch`, which this
  change does not alter.

---

### Task 1: Driver — a scoped start inherits its scope's run epoch

Risk note for routing: touches the scoped admission path (`Driver.Admit` /
`precheckScopedStart`) — consequential but correctable.

**Files:**
- Modify: `internal/gatedrive/driver.go` — `Driver.Admit`, `Driver.precheckScopedStart`,
  `scopedIdentityMatch`; add `scopedRunEpoch`; update the `StartRequest.RunEpochID` doc comment.
- Modify: `internal/gatedrive/scope.go` — `scopeRecord.RunEpochID` doc comment only (make it name
  the inheritance; the comment becomes true with this task).
- Test: `internal/gatedrive/epoch_test.go`

**Interfaces:**
- Consumes (existing test helpers, same package): `sampleStart() StartRequest`,
  `scopeReqFor(req StartRequest, gateContext string) ScopeRequest`, `scopedTestDriver(store *Store,
  clk *fakeClock, proc ProcessSeam, git GitSeam) *Driver`, `stableGit()`, `startEpoch()`,
  `releasedEpochSlot(t, s *Store, worktree, repoID string) admissionRecord` (seeds a released slot
  owned by `"epoch-e1"`), `readSlotBytes(t, s *Store, worktree string) []byte`,
  `isOwnershipKind(err error, kind OwnershipErrorKind) bool`, `(*Store).ownerCAS`, `fakeProc`
  (`launchN` counter; default launch/observe = running, so a first slice WAITs).
- Produces: `func scopedRunEpoch(scope scopeRecord, presented string) string`;
  `func (d *Driver) precheckScopedStart(req StartRequest) (string, error)` (was `error`). No exported
  API change.

- [ ] **Step 1: Write the failing tests**

Add `"sync"` to the imports of `internal/gatedrive/epoch_test.go`, then append:

```go
// ---------------------------------------------------------------------------
// Scoped starts inherit the scope's run epoch (change 0467). A scope prepared
// with epoch E hands E to every scoped start under it: a start presenting no
// epoch is admitted as E (it used to be refused stale-run-epoch against the
// E-owned slot), presenting E still admits, presenting F != E is refused
// scope-identity-mismatch before anything is reserved, and a scope with no
// epoch leaves the presented value governing, exactly as before.
// ---------------------------------------------------------------------------

// recordingEpochGate is a permissive EpochLaunchGate that records every epoch id
// it is asked to validate, so a test can prove which epoch the driver gated on.
type recordingEpochGate struct {
	mu   sync.Mutex
	seen []string
}

func (g *recordingEpochGate) gate() EpochLaunchGate {
	return func(epochID, _ string, reserve func() error) error {
		g.mu.Lock()
		g.seen = append(g.seen, epochID)
		g.mu.Unlock()
		return reserve()
	}
}

func (g *recordingEpochGate) epochs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.seen...)
}

// prepareEpochScopedStart prepares a scope pinned to scopeEpoch ("" for a scope
// with no epoch) over the sample worktree and returns a StartRequest wired to it
// that presents NO run epoch.
func prepareEpochScopedStart(t *testing.T, store *Store, scopeEpoch string) StartRequest {
	t.Helper()
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunEpochID = scopeEpoch
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	return req
}

// TestScopedStartInheritsScopeEpoch: a start that presents no epoch, under a
// scope pinned to epoch-e1, over a released slot epoch-e1 still owns, is
// admitted as epoch-e1 — the slot keeps epoch-e1 and every epoch-gate call the
// start made named epoch-e1 (an empty epoch would bypass the gate entirely).
func TestScopedStartInheritsScopeEpoch(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareEpochScopedStart(t, store, "epoch-e1")
	releasedEpochSlot(t, store, req.Worktree, req.RepoDir)

	g := &recordingEpochGate{}
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
	d.SetEpochLaunchGate(g.gate())

	doc, err := d.Start(req) // req.RunEpochID == ""
	if err != nil {
		t.Fatalf("a scoped start presenting no epoch must inherit the scope's, got %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionExecuting || slot.RunEpochID != "epoch-e1" {
		t.Fatalf("slot = %s/%q, want executing/epoch-e1", slot.State, slot.RunEpochID)
	}
	seen := g.epochs()
	if len(seen) == 0 {
		t.Fatal("the start never consulted the epoch gate: it ran epoch-less")
	}
	for _, e := range seen {
		if e != "epoch-e1" {
			t.Fatalf("epoch gate consulted with %q, want only epoch-e1 (all calls: %v)", e, seen)
		}
	}
}

// TestScopedStartPresentingScopeEpochAdmits: presenting the scope's own epoch
// still admits (regression pin — green before and after this change).
func TestScopedStartPresentingScopeEpochAdmits(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareEpochScopedStart(t, store, "epoch-e1")
	releasedEpochSlot(t, store, req.Worktree, req.RepoDir)
	req.RunEpochID = "epoch-e1"

	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
	doc, err := d.Start(req)
	if err != nil || doc.Outcome != WAITING {
		t.Fatalf("presenting the scope's epoch must admit and WAIT: doc=%+v err=%v", doc, err)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.RunEpochID != "epoch-e1" {
		t.Fatalf("slot epoch = %q, want epoch-e1", slot.RunEpochID)
	}
}

// TestScopedStartForeignEpochRefused: presenting an epoch that differs from the
// scope's pinned one is refused scope-identity-mismatch before anything is
// reserved — the worktree slot is byte-for-byte untouched, the scope's single
// slot stays empty, nothing launched, and the epoch gate was never consulted.
func TestScopedStartForeignEpochRefused(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareEpochScopedStart(t, store, "epoch-e1")
	releasedEpochSlot(t, store, req.Worktree, req.RepoDir)
	req.RunEpochID = "epoch-foreign"
	before := readSlotBytes(t, store, req.Worktree)

	g := &recordingEpochGate{}
	proc := &fakeProc{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	d.SetEpochLaunchGate(g.gate())

	if _, err := d.Start(req); !isOwnershipKind(err, ErrScopeIdentityMismatch) {
		t.Fatalf("a foreign presented epoch must refuse scope-identity-mismatch, got %v", err)
	}
	if string(readSlotBytes(t, store, req.Worktree)) != string(before) {
		t.Fatal("a refused start must not touch the worktree slot")
	}
	scope, err := store.LoadScope(req.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != "" || scope.DriveCount != 0 {
		t.Fatalf("a refused start must reserve no scope slot, got current=%q count=%d", scope.CurrentDriveID, scope.DriveCount)
	}
	if proc.launchN != 0 {
		t.Fatalf("a refused start must launch nothing, launched %d", proc.launchN)
	}
	if n := len(g.epochs()); n != 0 {
		t.Fatalf("a refused start must not reach the epoch gate, consulted %d times", n)
	}
}

// TestScopedSuccessorStartInheritsScopeEpoch: the worker's SEQUENCE of drives in
// one scope — a successor start presenting no epoch after a PASSED predecessor
// over the still-executing, epoch-e1-owned slot — is admitted (it rotates the
// slot), while a successor presenting a foreign epoch is refused
// scope-identity-mismatch without consuming the predecessor receipt.
func TestScopedSuccessorStartInheritsScopeEpoch(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareEpochScopedStart(t, store, "epoch-e1")
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())

	first, err := d.Start(req) // presents no epoch: inherits epoch-e1
	if err != nil || first.Outcome != WAITING {
		t.Fatalf("first start must inherit and WAIT: doc=%+v err=%v", first, err)
	}
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation

	foreign := succ
	foreign.RunEpochID = "epoch-foreign"
	if _, err := d.Start(foreign); !isOwnershipKind(err, ErrScopeIdentityMismatch) {
		t.Fatalf("a successor presenting a foreign epoch must refuse scope-identity-mismatch, got %v", err)
	}

	second, err := d.Start(succ) // presents no epoch: inherits epoch-e1
	if err != nil {
		t.Fatalf("a successor presenting no epoch must inherit the scope's, got %v", err)
	}
	if second.Outcome != WAITING || second.DriveID == first.DriveID {
		t.Fatalf("successor must be a NEW waiting drive, got %+v", second)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.RunEpochID != "epoch-e1" {
		t.Fatalf("successor slot epoch = %q, want epoch-e1", slot.RunEpochID)
	}
}

// TestEpochlessScopeKeepsPresentedEpoch: a scope with no pinned epoch (a legacy
// v2 scope, or one prepared without) supplies nothing — the presented value
// governs, unchanged from before. Presenting none over an epoch-e1-owned slot is
// still fenced stale-run-epoch; presenting epoch-e1 admits.
func TestEpochlessScopeKeepsPresentedEpoch(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareEpochScopedStart(t, store, "")
	releasedEpochSlot(t, store, req.Worktree, req.RepoDir)
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())

	if _, err := d.Start(req); !isOwnershipKind(err, ErrStaleRunEpoch) {
		t.Fatalf("an epoch-less scope must not supply an epoch: want stale-run-epoch, got %v", err)
	}
	req.RunEpochID = "epoch-e1"
	doc, err := d.Start(req)
	if err != nil || doc.Outcome != WAITING {
		t.Fatalf("presenting the owning epoch under an epoch-less scope must admit: doc=%+v err=%v", doc, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify the RED is the right one**

Run: `go test -count=1 ./internal/gatedrive/ -run 'TestScopedStartInheritsScopeEpoch|TestScopedStartPresentingScopeEpochAdmits|TestScopedStartForeignEpochRefused|TestScopedSuccessorStartInheritsScopeEpoch|TestEpochlessScopeKeepsPresentedEpoch' -v`

Expected:
- `TestScopedStartInheritsScopeEpoch` FAIL — error names `stale-run-epoch`.
- `TestScopedStartForeignEpochRefused` FAIL — got `stale-run-epoch`, wanted `scope-identity-mismatch`.
- `TestScopedSuccessorStartInheritsScopeEpoch` FAIL at the foreign-successor assert (`got <nil>`):
  the slot is fresh, so today the first start records an empty epoch and nothing compares the
  presented epoch with the scope's, so the foreign successor is admitted. Any FAIL whose message is
  not about the epoch (compile error, helper misuse) is a test bug: fix the test before continuing.
- `TestScopedStartPresentingScopeEpochAdmits` and `TestEpochlessScopeKeepsPresentedEpoch` PASS
  (regression pins for unchanged behaviour).

- [ ] **Step 3: Implement the inheritance in `internal/gatedrive/driver.go`**

(a) In `Driver.Admit`, replace the scope pre-check block:

```go
	if req.ScopeID != "" {
		if err := d.precheckScopedStart(req); err != nil {
			return nil, err
		}
	}
```

with:

```go
	if req.ScopeID != "" {
		epoch, err := d.precheckScopedStart(req)
		if err != nil {
			return nil, err
		}
		// A scoped start takes its run epoch from the scope it was prepared under
		// (change 0467): the scope's RunEpochID is written once by PrepareScope and
		// never mutated, so this unlocked read is authoritative. req is Admit's own
		// copy, so every later use — the epoch gate, the scoped worktree admission
		// record, finished-incumbent reconciliation, and the admission ticket — sees
		// the effective epoch rather than the caller-presented one.
		req.RunEpochID = epoch
	}
```

(b) Change `precheckScopedStart` to return the effective epoch. New signature
`func (d *Driver) precheckScopedStart(req StartRequest) (string, error)`; every existing
`return err` / `return ownershipErr(...)` / `return lerr` becomes `return "", <same error>`. The two
success exits become:

- first-start exit (currently the `return nil` after the empty-receipt `scopedIdentityMatch` check):
  `return scopedRunEpoch(scope, req.RunEpochID), nil`
- successor exit (currently `return predecessorReusableError(&prec, receipt.OwnerGen)`):

```go
	if err := predecessorReusableError(&prec, receipt.OwnerGen); err != nil {
		return "", err
	}
	return scopedRunEpoch(scope, req.RunEpochID), nil
```

Extend its doc comment with one sentence: "On success it returns the start's effective run epoch
(scopedRunEpoch)."

(c) In `scopedIdentityMatch`, before the final `return true`, add:

```go
	// A scope that pinned a run epoch accepts a start presenting none (it inherits
	// the scope's — scopedRunEpoch) or the same one; a different presented epoch is
	// an altered identity (change 0467).
	if scope.RunEpochID != "" && req.RunEpochID != "" && req.RunEpochID != scope.RunEpochID {
		return false
	}
```

and extend its doc comment: "…plus the gate-context token when the scope pinned one, and the run
epoch when both the scope and the request carry one."

(d) Add, directly after `scopedIdentityMatch`:

```go
// scopedRunEpoch resolves the effective run epoch of a scoped start (change 0467):
// a scope that pinned an epoch supplies it — scopedIdentityMatch has already
// refused a start presenting a different one — and a scope with no epoch (a legacy
// v2 scope, or one prepared without) leaves the presented value governing,
// unchanged from before.
func scopedRunEpoch(scope scopeRecord, presented string) string {
	if scope.RunEpochID != "" {
		return scope.RunEpochID
	}
	return presented
}
```

(e) Doc comments. In `StartRequest.RunEpochID` (driver.go), append: "A scoped start inherits the
epoch its scope pinned when it presents none, and presenting a different one is refused
ErrScopeIdentityMismatch (change 0467)." In `scopeRecord.RunEpochID` (scope.go), replace "travels
onto each scoped start's worktree execution slot" with "is inherited by each scoped start (the
driver's scopedRunEpoch) and travels onto its worktree execution slot". Do not touch the
`ScopeRequest.RunEpochID` comment (already accurate).

- [ ] **Step 4: Run the focused tests to verify GREEN**

Run the Step 2 command again. Expected: all five PASS.

- [ ] **Step 5: Mutation-test the two load-bearing lines**

```bash
cd /Users/homer/dev/docket/.worktrees/document-run-epoch-in-the-docket-build-task-gate-drive-start
f=internal/gatedrive/driver.go
# M1: drop the substitution — the inheritance tests must redden.
cp "$f" "$f.bak"
perl -0pi -e 's/\n\t\treq\.RunEpochID = epoch\n/\n/' "$f"
grep -c 'req.RunEpochID = epoch' "$f"   # expect 0 (was 1): the mutation landed
go test -count=1 ./internal/gatedrive/ -run 'TestScopedStartInheritsScopeEpoch|TestScopedSuccessorStartInheritsScopeEpoch' 2>&1 | tail -5
mv -f "$f.bak" "$f"
# M2: drop the epoch clause in scopedIdentityMatch — the foreign-epoch tests must redden.
cp "$f" "$f.bak"
perl -0pi -e 's/\tif scope\.RunEpochID != "" && req\.RunEpochID != "" && req\.RunEpochID != scope\.RunEpochID \{\n\t\treturn false\n\t\}\n//' "$f"
grep -c 'req.RunEpochID != scope.RunEpochID' "$f"   # expect 0
go test -count=1 ./internal/gatedrive/ -run 'TestScopedStartForeignEpochRefused|TestScopedSuccessorStartInheritsScopeEpoch' 2>&1 | tail -5
mv -f "$f.bak" "$f"
git diff --stat   # only the intended driver.go/scope.go/epoch_test.go edits remain
```

Expected: M1 → both named tests FAIL (`stale-run-epoch`); M2 → both named tests FAIL (admitted
instead of `scope-identity-mismatch`). If a mutation leaves its target green, stop and investigate —
that is a finding about the code or the test, not a residual.

- [ ] **Step 6: Run the whole gatedrive package plus its consumers**

Run: `go test -count=1 ./internal/gatedrive/ ./internal/app/ ./internal/cli/`
Expected: PASS. A pre-existing test that now fails with `scope-identity-mismatch` presented an epoch
different from its scope's on purpose: read what it guards before touching it — change its
expectation only if its premise is exactly the old "presented epoch wins over the scope's" behaviour
this change replaces (spec §1 table, row 3), and say so in the commit body.

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive/driver.go internal/gatedrive/scope.go internal/gatedrive/epoch_test.go
git commit -m "fix(gatedrive): scoped starts inherit the scope's run epoch (change 0467)"
```

---

### Task 2: Thread the run epoch through the build-chain skill prose, guarded

**Files:**
- Create: `internal/repoguard/gatedrive_run_epoch_thread_test.go`
- Modify: `skills/docket-implement-next/SKILL.md` (the *Verify the run* paragraph's dispatch-context
  sentence; Step 6's *Validate the build evidence* re-mint argv)
- Modify: `skills/docket-build/SKILL.md` (the `feature-dispatch` block's prepare-scope argv and bundle
  sentence; the WAITING-continuation re-prepare; the final gate's `build_gate: local` item)
- Modify: `skills/docket-build/references/gate-caller-loop.md` (the `start` and `prepare-scope`
  operation rows)
- Modify: `skills/docket-build-task/SKILL.md` (the scoped task-start paragraph)
- Regenerate: `internal/assets/embedded/tree/**`, `internal/assets/embedded/manifest.json`
- Modify: `internal/repoguard/budgets_test.go` (`skillBudgets` rows for the four edited files)

**Interfaces:**
- Consumes (existing, package `repoguard`): `guardRoot`, `maintainedPop`, `readMaintained`,
  `isWorkflowMD`, `paragraphs` (splits on blank lines, collapses whitespace), `startOpRe`,
  `isScopedTaskStartSite`, `buildSkillRel`, `buildTaskSkillRel`, `sharedContractRel`.
- Produces: `prepareScopeOpRe`, `ownerBuildRe`, `runEpochFlagRe`, `isPrepareScopeSite(p string) bool`,
  `isBuildOwnerStartSite(p string) bool`, `carriesRunEpoch(p string) bool`,
  `implementNextSkillRel` — Task 3 appends a second test function to the same file and reuses nothing
  else from it.

- [ ] **Step 1: Write the failing guard**

Create `internal/repoguard/gatedrive_run_epoch_thread_test.go`:

```go
package repoguard

// Change 0467: the run epoch the gated parent's arm prints must reach every call
// that mints a recovery scope or starts a build-owned drive, while build-task
// workers never handle it — the driver hands a scoped start the epoch its scope
// pinned. Prongs over maintained workflow markdown (isWorkflowMD, so the
// embedded mirrors are scanned too):
//   (A) every paragraph referencing gate.drive.prepare-scope carries --run-epoch;
//   (B) every build-owned gate.drive.start paragraph (--owner build) carries
//       --run-epoch;
//   (C) no scoped task-owned start paragraph (--owner task) carries --run-epoch —
//       the worker passes none; the scope supplies it.
// TestRunGateCopiesEpochIntoDispatchPrompt (below) binds the managed run-gate
// source to copying the epoch into the dispatch prompt.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// shared caller contract (sharedContractRel) is the operation reference, not a
// caller, and is excluded exactly as in TestGateDriveScopedStartIdentity.
// Residual risk, recorded not hidden: an instruction that names the operation
// without its `gate.drive.` prefix (a bare `prepare-scope`), or a build-owned
// start without the --owner build token in the same paragraph, is not a site;
// at run time the driver still fences such an epoch-less start against an
// epoch-owned worktree (stale-run-epoch).

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const implementNextSkillRel = "skills/docket-implement-next/SKILL.md"

var (
	prepareScopeOpRe = regexp.MustCompile(`gate\.drive\.prepare-scope`)
	ownerBuildRe     = regexp.MustCompile(`--owner build(?:[^a-z-]|$)`)
	runEpochFlagRe   = regexp.MustCompile(`--run-epoch(?:[^a-z-]|$)`)
)

// isPrepareScopeSite: a collapsed paragraph that references the
// gate.drive.prepare-scope operation.
func isPrepareScopeSite(p string) bool { return prepareScopeOpRe.MatchString(p) }

// isBuildOwnerStartSite: a collapsed paragraph that references gate.drive.start
// AND carries the --owner build token.
func isBuildOwnerStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerBuildRe.MatchString(p)
}

// carriesRunEpoch: the paragraph carries the --run-epoch flag token.
func carriesRunEpoch(p string) bool { return runEpochFlagRe.MatchString(p) }

func TestGateDriveRunEpochThreaded(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	prepSites := map[string]int{}
	buildSites := map[string]int{}
	taskSites := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, sharedContractRel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if isPrepareScopeSite(p) {
				prepSites[rel]++
				if !carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: gate.drive.prepare-scope instruction lacks --run-epoch: %.160s", rel, p))
				}
			}
			if isBuildOwnerStartSite(p) {
				buildSites[rel]++
				if !carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: build-owned gate.drive.start instruction lacks --run-epoch: %.160s", rel, p))
				}
			}
			if isScopedTaskStartSite(p) {
				taskSites[rel]++
				if carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: scoped task-owned start must not pass --run-epoch (the scope supplies it): %.160s", rel, p))
				}
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative).
	mirror := func(rel string) []string { return []string{rel, "internal/assets/embedded/tree/" + rel} }
	for _, rel := range append(mirror(buildSkillRel), mirror(implementNextSkillRel)...) {
		if prepSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no gate.drive.prepare-scope site (scan or corpus drifted)", rel)
		}
		if buildSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no build-owned gate.drive.start site (scan or corpus drifted)", rel)
		}
	}
	for _, rel := range mirror(buildSkillRel) {
		// The per-dispatch scope AND the WAITING-continuation re-prepare.
		if prepSites[rel] < 2 {
			t.Errorf("coverage floor: %s must carry both the per-dispatch and the continuation prepare-scope sites, found %d", rel, prepSites[rel])
		}
	}
	for _, rel := range mirror(buildTaskSkillRel) {
		if taskSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no scoped task-start site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("run-epoch threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		prep := "run the `gate.drive.prepare-scope` operation with `--change-id <id> --worktree <w> --gate-context <g> --run-epoch <epoch> --json`"
		if !isPrepareScopeSite(prep) || !carriesRunEpoch(prep) {
			t.Fatalf("a complete prepare-scope invocation was misclassified")
		}
		if carriesRunEpoch(strings.Replace(prep, "--run-epoch <epoch> ", "", 1)) {
			t.Errorf("stripping --run-epoch from a prepare-scope invocation was not detected")
		}
		build := "the `gate.drive.start` operation with `--owner build --run-epoch <epoch> --json`"
		if !isBuildOwnerStartSite(build) || !carriesRunEpoch(build) {
			t.Fatalf("a complete build-owned start was misclassified")
		}
		if carriesRunEpoch(strings.Replace(build, "--run-epoch <epoch> ", "", 1)) {
			t.Errorf("stripping --run-epoch from a build-owned start was not detected")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner builder --json`") {
			t.Errorf("--owner build token boundary failed: 'builder' matched")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as build-owned")
		}
		if carriesRunEpoch("pass `--run-epoch-id <x>`") {
			t.Errorf("--run-epoch token boundary failed: '--run-epoch-id' matched")
		}
		task := "the `gate.drive.start` operation with `--owner task --scope-id <s> --child-cap <c> --run-epoch <e> --json`"
		if !isScopedTaskStartSite(task) || !carriesRunEpoch(task) {
			t.Errorf("a worker start that passes --run-epoch must be classified and flagged")
		}
		wrapped := "run `gate.drive.prepare-scope` again\nfor the same change (and `--run-epoch\n<epoch>`)"
		if got := paragraphs(wrapped); len(got) != 1 || !isPrepareScopeSite(got[0]) || !carriesRunEpoch(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped prepare-scope site did not match as one paragraph")
		}
	})
}
```

- [ ] **Step 2: Run the guard to verify RED**

Run: `go test -count=1 ./internal/repoguard/ -run TestGateDriveRunEpochThreaded -v`
Expected: FAIL with violations naming, for both `skills/…` and `internal/assets/embedded/tree/skills/…`:
`docket-build/SKILL.md` (2 prepare-scope sites + 1 build-owned start), `docket-implement-next/SKILL.md`
(1 prepare-scope site + 1 build-owned start). No coverage-floor errors (the floors hold on today's
corpus). `non_vacuity` PASS. A coverage-floor error at this step means the scan is miskeyed — fix the
test, not the prose.

- [ ] **Step 3: Edit `skills/docket-implement-next/SKILL.md`**

Both edits are inside single-line paragraphs; replace the exact substrings (each fenced block is
the literal text, on one line in the file).

(a) In the *Verify the run* paragraph, replace

```
and into the Step-2 claim's --gate-context.
```

with

```
and into the Step-2 claim's --gate-context. It may also carry the **run epoch** id: pass it as `--run-epoch <epoch>` into every `gate.drive.prepare-scope` and every build-owned `gate.drive.start` (`--owner build` — Step 6's evidence re-mint and the build role's final suite gate) this run performs, omitting the flag only when the prompt carried none (a `gate-unarmed` or ungated run). Scoped task-owned starts inherit the epoch from their scope, so build-task workers are never handed it.
```

(b) In Step 6's *Validate the build evidence* paragraph, replace

```
the `gate.drive.start` operation with `--repo-dir <absolute-feature-worktree> --run-root <absolute-run-root> --owner build --json` (`--owner build` resolves the build-owned command; no suite argv)
```

with

```
the `gate.drive.start` operation with `--repo-dir <absolute-feature-worktree> --run-root <absolute-run-root> --owner build --run-epoch <epoch> --json` (`--owner build` resolves the build-owned command; no suite argv; `--run-epoch` carries the run epoch from your dispatch prompt, omitted only when the prompt carried none)
```

- [ ] **Step 4: Edit `skills/docket-build/SKILL.md`**

(a) In the `feature-dispatch` block, replace

```
<worktree> --gate-context <dispatch-context> --json` (the dispatch context arrived in *your* prompt from
the gated parent — pass its value through). Capture the scope id and **both** capabilities from the
```

with

```
<worktree> --gate-context <dispatch-context> --run-epoch <epoch> --json` (the dispatch context and
the run epoch arrived in *your* prompt from the gated parent — pass each value through, omitting a
flag only when your prompt carried no such value). Capture the scope id and **both** capabilities from the
```

(b) In the same block, replace

```
worker to pass through to `gate.drive.start` unchanged. One scope now carries the worker's whole
```

with

```
worker to pass through to `gate.drive.start` unchanged. The bundle carries no run epoch: the scope
pinned it, and every scoped start inherits it. One scope now carries the worker's whole
```

(c) In the WAITING-continuation paragraph, replace

```
for the same change, task, phase, branch, and worktree (and dispatch context) and include the new
```

with

```
for the same change, task, phase, branch, and worktree (and dispatch context and
`--run-epoch <epoch>`, as for the first scope) and include the new
```

(d) In the final gate's item 2, replace

```
   **driver**: the `gate.drive.start` operation with `--owner build --json` — capture that first response into `gate_reply` (its exit
```

with

```
   **driver**: the `gate.drive.start` operation with `--owner build --run-epoch <epoch> --json`
   (`--run-epoch` only when your prompt carried a run epoch) — capture that first response into `gate_reply` (its exit
```

- [ ] **Step 5: Edit `skills/docket-build/references/gate-caller-loop.md`**

Both rows are single table lines; replace the exact substrings.

(a) `prepare-scope` row: replace

```
--worktree <dir> [--gate-context <token>]`: mint a recovery scope
```

with

```
--worktree <dir> [--gate-context <token>] [--run-epoch <id>]`: mint a recovery scope
```

and replace

```
the child receives only the scope id and child capability.
```

with

```
the child receives only the scope id and child capability. A run epoch given here is pinned on the scope and inherited by every scoped start under it.
```

(b) `start` row: replace

```
and the driver rejects a start whose identity does not match the prepared scope.
```

with

```
and the driver rejects a start whose identity does not match the prepared scope. A scope-bound start takes its run epoch from the scope: it passes none, and a presented `--run-epoch` that differs from the pinned one is refused `scope-identity-mismatch`.
```

- [ ] **Step 6: Edit `skills/docket-build-task/SKILL.md`**

In the scoped task-start paragraph, replace

```
rejects a start that omits or alters any of it. The run root is a scratch dir you pick and read from.
```

with

```
rejects a start that omits or alters any of it. The run epoch is not in the bundle: it rides on the
prepared scope, so you neither receive nor pass one — a start that invents a different epoch is
refused `scope-identity-mismatch`. The run root is a scratch dir you pick and read from.
```

Do **not** write the `--run-epoch` token in this paragraph (prong C flags it) and do not change the
argv.

- [ ] **Step 7: Regenerate the embedded copies**

```bash
cd /Users/homer/dev/docket/.worktrees/document-run-epoch-in-the-docket-build-task-gate-drive-start
go generate ./internal/assets/
git status --short internal/assets/embedded   # the four mirrored skill files + manifest.json, nothing else
go test -count=1 ./internal/assets/
```

Expected: exactly the four mirrored files and `manifest.json` modified; assets tests PASS.

- [ ] **Step 8: Run the guard to verify GREEN**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestGateDriveRunEpochThreaded|TestGateDriveScopedStartIdentity|TestGateDriveJSONCapture' -v`
Expected: PASS.

- [ ] **Step 9: Mutation-test the guard against the real corpus**

```bash
cd /Users/homer/dev/docket/.worktrees/document-run-epoch-in-the-docket-build-task-gate-drive-start
f=skills/docket-build/SKILL.md
count() { tr -s '[:space:]' ' ' < "$f" | grep -o -F -- '--run-epoch <epoch>' | wc -l; }
before=$(count)
cp "$f" "$f.bak"
# Strip the continuation re-prepare's flag (the site Review Focus 4 names).
perl -0pi -e 's/\(and dispatch context and\s+`--run-epoch <epoch>`, as for the first scope\)/(and dispatch context)/' "$f"
after=$(count); echo "before=$before after=$after"   # after must be before-1, else MUTATION DID NOT LAND
go test -count=1 ./internal/repoguard/ -run TestGateDriveRunEpochThreaded 2>&1 | grep -F 'docket-build/SKILL.md'
mv -f "$f.bak" "$f"
f=skills/docket-build-task/SKILL.md
cp "$f" "$f.bak"
perl -0pi -e 's/--run-root\n<task-scratch-dir> --json/--run-epoch <e> --run-root\n<task-scratch-dir> --json/' "$f"
grep -c -F -- '--run-epoch <e>' "$f"   # expect 1: the mutation landed
go test -count=1 ./internal/repoguard/ -run TestGateDriveRunEpochThreaded 2>&1 | grep -F 'must not pass --run-epoch'
mv -f "$f.bak" "$f"
git diff --stat -- skills   # only the intended Step 3–6 edits
```

Expected: the first probe reports a violation naming `skills/docket-build/SKILL.md` (prepare-scope
lacks `--run-epoch`); the second reports the prong-C violation. If `perl` did not match (count
unchanged), re-derive the pattern from the file's actual wrapping — never read a no-op as "the guard
survived".

- [ ] **Step 10: Re-baseline the skill size budgets**

```bash
wc -lw skills/docket-build/SKILL.md skills/docket-build-task/SKILL.md \
  skills/docket-implement-next/SKILL.md skills/docket-build/references/gate-caller-loop.md
```

In `internal/repoguard/budgets_test.go` `skillBudgets`, set each of the four rows' line and word
ceilings to the exact measured counts (pre-change ceilings: `docket-build/SKILL.md` 432/4391,
`docket-build-task/SKILL.md` 204/2151, `docket-implement-next/SKILL.md` 214/8080,
`docket-build/references/gate-caller-loop.md` 175/1826 — the gate-caller-loop line ceiling stays 175
if its line count did not grow). Prefix each row's comment with a note, e.g.
`// 0467: +run-epoch threading on prepare-scope and the build-owned start; the bundle carries no epoch (432/4391 -> L/W); 0459: …`
(keep the existing notes after it).

Run: `go test -count=1 ./internal/repoguard/`
Expected: PASS (whole package — prose pins elsewhere may quote an edited sentence; a red pin is
relocated to the new wording only if it still asserts the same claim, never restored by re-adding
old text).

- [ ] **Step 11: Commit**

```bash
git add internal/repoguard/gatedrive_run_epoch_thread_test.go internal/repoguard/budgets_test.go \
  skills/docket-implement-next/SKILL.md skills/docket-build/SKILL.md \
  skills/docket-build/references/gate-caller-loop.md skills/docket-build-task/SKILL.md \
  internal/assets/embedded
git commit -m "docs(skills): thread the run epoch to prepare-scope and build-owned starts; guard it (change 0467)"
```

---

### Task 3: Parent run-gate block copies the epoch into the dispatch prompt

**Files:**
- Modify: `cursor-rules/run-gate.md` (step 1)
- Regenerate: `internal/assets/embedded/tree/cursor-rules/run-gate.md`,
  `internal/assets/embedded/manifest.json`
- Regenerate: `AGENTS.md` `docket:dispatch` block (`CLAUDE.md` follows via its symlink)
- Modify: `internal/repoguard/gatedrive_run_epoch_thread_test.go` (append one test function)
- Modify: `internal/repoguard/budgets_test.go` (`dispatchBudget`)

**Interfaces:**
- Consumes: `guardRoot`, `readMaintained` (package `repoguard`); `document.Parse`,
  `(*document.PatchSet).ReplaceBlock`, `doc.Apply`, `assets.EmbeddedCatalog`, `harness.RunGate`,
  `harness.CodexDispatchInterior` — used exactly as `TestCommittedCodexDispatchMatchesGenerator` uses
  them.
- Produces: `runGateEpochCopyRe`, `TestRunGateCopiesEpochIntoDispatchPrompt`.

- [ ] **Step 1: Write the failing guard**

Append to `internal/repoguard/gatedrive_run_epoch_thread_test.go`:

```go
// runGateEpochCopyRe binds the copy instruction to the epoch AND to its
// destination with one bounded, sentence-local gap: a rewrite that keeps the
// word <epoch> elsewhere but drops "copy it into the dispatch prompt" reddens.
var runGateEpochCopyRe = regexp.MustCompile("copy the [^.]{0,60}`<epoch>` into the dispatch prompt")

// TestRunGateCopiesEpochIntoDispatchPrompt: the managed run-gate source, its
// embedded mirror, and the committed AGENTS.md rendering all tell the parent to
// copy the arm's <epoch> into the implement-next dispatch prompt (change 0467) —
// otherwise the epoch never reaches the build chain.
func TestRunGateCopiesEpochIntoDispatchPrompt(t *testing.T) {
	root := guardRoot(t)
	for _, rel := range []string{
		"cursor-rules/run-gate.md",
		"internal/assets/embedded/tree/cursor-rules/run-gate.md",
		"AGENTS.md",
	} {
		if !runGateEpochCopyRe.MatchString(collapseWS(readMaintained(t, root, rel))) {
			t.Errorf("%s: run-gate step 1 does not copy the `<epoch>` into the dispatch prompt", rel)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := "keep all three and copy the `<dispatch-context>` and the `<epoch>`\n   into the dispatch prompt."
		if !runGateEpochCopyRe.MatchString(collapseWS(good)) {
			t.Fatalf("the intended (wrapped) wording did not match")
		}
		old := "copy the `<dispatch-context>` into the dispatch prompt. The `<epoch>` is the run epoch id"
		if runGateEpochCopyRe.MatchString(collapseWS(old)) {
			t.Errorf("the pre-0467 wording (epoch not copied) matched")
		}
		if runGateEpochCopyRe.MatchString("copy the `<dispatch-context>`. Later, `<epoch>` into the dispatch prompt") {
			t.Errorf("bounded gap failed: the binding must not span sentences")
		}
	})
}
```

(`collapseWS` already exists in package `repoguard`, in `finalize_rebuild_test.go`.)

- [ ] **Step 2: Run to verify RED**

Run: `go test -count=1 ./internal/repoguard/ -run TestRunGateCopiesEpochIntoDispatchPrompt -v`
Expected: FAIL for all three files; `non_vacuity` PASS.

- [ ] **Step 3: Edit `cursor-rules/run-gate.md` step 1**

Replace the whole step-1 list item:

```
1. Before dispatching `docket-implement-next`, run `run.gate-before` with `implement-next`. It prints
   `gate-armed <key> <epoch> <dispatch-context>`; keep all three (they won't survive the next tool
   call) and copy the `<dispatch-context>` into the dispatch prompt. The `<epoch>` is the run epoch id
   you thread into `run.cancel --epoch` (below) and every `--run-epoch` dispatch flag (`agent.enter`,
   `gate drive start`, `gate drive prepare-scope`). Add `--resume <id>` to arm for resuming an
   already-in-progress change. `gate-unarmed` still lets you dispatch, but keyless (step 2's fallback)
   and can never authorize a re-dispatch.
```

with

```
1. Before dispatching `docket-implement-next`, run `run.gate-before` with `implement-next`. It prints
   `gate-armed <key> <epoch> <dispatch-context>`; keep all three (they won't survive the next tool
   call) and copy the `<dispatch-context>` and the `<epoch>` into the dispatch prompt. The `<epoch>`
   is the run epoch id you thread into `run.cancel --epoch` (below) and every `--run-epoch` dispatch
   flag (`agent.enter`, `gate drive start`, `gate drive prepare-scope`). Add `--resume <id>` to arm
   for resuming an already-in-progress change. `gate-unarmed` still lets you dispatch, but keyless
   (step 2's fallback) and can never authorize a re-dispatch.
```

(+3 words: "and the `<epoch>`".)

- [ ] **Step 4: Regenerate the embedded copy**

```bash
cd /Users/homer/dev/docket/.worktrees/document-run-epoch-in-the-docket-build-task-gate-drive-start
go generate ./internal/assets/
git status --short internal/assets/embedded   # run-gate.md mirror + manifest.json only
```

- [ ] **Step 5: Regenerate the committed `AGENTS.md` dispatch block (never hand-edit it)**

The block is generator output; regenerate it with a throwaway, uncommitted test that applies the same
patch `TestCommittedCodexDispatchMatchesGenerator` checks, then delete it:

```bash
cd /Users/homer/dev/docket/.worktrees/document-run-epoch-in-the-docket-build-task-gate-drive-start
cat > internal/repoguard/zz_regen_agents_0467_test.go <<'EOF'
package repoguard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
)

func TestZZRegenAgentsDispatch0467(t *testing.T) {
	if os.Getenv("DOCKET_REGEN_AGENTS_0467") != "1" {
		t.Skip("one-shot regeneration only")
	}
	p := filepath.Join(guardRoot(t), "AGENTS.md")
	src, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := document.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := harness.RunGate(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var patch document.PatchSet
	patch.ReplaceBlock("dispatch", harness.CodexDispatchInterior(gate))
	out, err := doc.Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	tmp := p + ".regen"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, p); err != nil {
		t.Fatal(err)
	}
}
EOF
DOCKET_REGEN_AGENTS_0467=1 go test -count=1 ./internal/repoguard/ -run TestZZRegenAgentsDispatch0467 -v
rm -f internal/repoguard/zz_regen_agents_0467_test.go
test -L CLAUDE.md && [ "$(readlink CLAUDE.md)" = "AGENTS.md" ] && echo "CLAUDE.md symlink intact"
git diff --stat -- AGENTS.md   # only the step-1 lines inside the docket:dispatch block
git status --short internal/repoguard   # the throwaway file must NOT appear
```

Expected: `AGENTS.md` changes only inside the `docket:dispatch` markers, mirroring the Step 3 edit;
`CLAUDE.md` is still the symlink; no `zz_regen_*` file remains.

- [ ] **Step 6: Re-baseline the dispatch-block budget**

Run: `go test -count=1 ./internal/repoguard/ -run TestDispatchBlockBudget -v` — it reports the
block's new word count (expected 1140) over the 1137 budget. Set `dispatchBudget` in
`internal/repoguard/budgets_test.go` to the exact reported count with a leading note, e.g.
`dispatchBudget = 1140 // 0467: step 1 copies the <epoch> into the dispatch prompt alongside the dispatch context (was 1137); 0375: …`
(keep the existing note after it). It must remain strictly below `dispatchOld` (1156).

- [ ] **Step 7: Verify GREEN and mutation-test**

```bash
cd /Users/homer/dev/docket/.worktrees/document-run-epoch-in-the-docket-build-task-gate-drive-start
go test -count=1 ./internal/repoguard/ -run 'TestRunGateCopiesEpochIntoDispatchPrompt|TestCommittedCodexDispatchMatchesGenerator|TestDispatchBlockBudget' -v
go test -count=1 ./internal/assets/ ./internal/harness/...
f=cursor-rules/run-gate.md
cp "$f" "$f.bak"
perl -0pi -e 's/ and the `<epoch>` into the dispatch prompt/ into the dispatch prompt/' "$f"
grep -c -F 'and the `<epoch>` into' "$f"   # expect 0: the mutation landed
go test -count=1 ./internal/repoguard/ -run TestRunGateCopiesEpochIntoDispatchPrompt 2>&1 | grep -F 'cursor-rules/run-gate.md'
mv -f "$f.bak" "$f"
```

Expected: first three tests PASS; assets/harness PASS; the mutation probe reports the
`cursor-rules/run-gate.md` failure.

- [ ] **Step 8: Run the whole repoguard package**

Run: `go test -count=1 ./internal/repoguard/ ./internal/install/...`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add cursor-rules/run-gate.md AGENTS.md internal/assets/embedded \
  internal/repoguard/gatedrive_run_epoch_thread_test.go internal/repoguard/budgets_test.go
git commit -m "docs(run-gate): copy the run epoch into the implement-next dispatch prompt (change 0467)"
```

---

## After the last task

The build controller runs the whole suite once through its build-owned gate (the command
`build.test_command` resolves to — never only the tests named above) and reads the budget report
even on a green run.

## Self-review against the spec

- Spec §1 table rows 1–4 → Task 1 tests (`…InheritsScopeEpoch`, `…PresentingScopeEpochAdmits`,
  `…ForeignEpochRefused`, `EpochlessScopeKeepsPresentedEpoch`); "replaces req.RunEpochID everywhere
  the scoped start uses it" → the single substitution in `Admit` feeds `epochGated`,
  `admitScoped`/`admitScopedWorktree`, `reconcileFinishedIncumbent`, and `ticket.runEpochID`; the
  drive record carries no epoch field — `resolveDriveEpoch` already reads the scope's. Mutation check
  → Task 1 Step 5.
- Spec §2 parent → Task 3; implement-next, docket-build, docket-build-task, embedded regeneration →
  Task 2 (plus the shared contract's rows, so the reference agrees with the driver).
- Spec §3 guard (prepare-scope sites, build-owned starts, run-gate copy; derived by scan;
  mutation-tested) → Task 2 prongs A/B + Task 3; prong C additionally pins "workers never handle the
  epoch".
- Out-of-scope items are untouched by every task.
