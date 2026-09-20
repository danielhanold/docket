<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0439 — Leaked worktree gate-admission slot stuck in "executing" blocks finalize.rebase with a swallowed unavailable error](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0439-leaked-worktree-gate-admission-slot-stuck-in-executing-block.md)**
<!-- docket:backlink:end -->
# Occupied Worktree Gate-Slot Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a worktree gate-admission slot refuses a new gate (finalize's local gate included), surface the actual typed refusal, a credential-free incumbent identity, and the applicable existing recovery route — instead of a bare `unavailable` halt.

**Architecture:** The incumbent's facts are snapshotted onto the typed `OwnershipError` at the one place they are read under the admission lock (`reserveWorktreeExecution`), so a later-changed slot can never be misrepresented as the cause. The app layer maps that snapshot into the existing `GateDriveResult` reason/message/stage/locator fields (stage `worktree-admission`, `incumbent-drive:<id>` / `incumbent-run:<id>` locators), and finalize carries those four fields through `LocalGateResult` into `GateReport` so both JSON and human output name the refusal and remedy. No admission, teardown, cancellation, or cleanup policy changes.

**Tech Stack:** Go (internal/gatedrive, internal/app), Go stdlib testing. Suite gate: the configured `build.test_command` (currently `go run ./cmd/docket development test`) — read from config, never a second copy.

**Spec:** `docs/superpowers/specs/2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/...` from the primary tree). The change file is `docs/changes/active/0439-leaked-worktree-gate-admission-slot-stuck-in-executing-block.md` on the same branch.

## Global Constraints

- No new CLI command, configuration key, persistent schema change, lifecycle state, daemon, liveness implementation, automatic recovery policy, retry layer, or ADR. (Spec "Scope and metadata".)
- The `admissionRecord` on-disk schema is untouched: `admissionSchemaVersion` stays 1; diagnosis performs no process signaling, slot release, or launch retry, and never mutates the admission record.
- Never expose reservation tokens, owner generations, capabilities, command argv, environment values, or arbitrary record contents in any refusal — JSON or human. Locators carry validated ids only.
- Never suggest raw `gate stop` for a driven or epoch-owned slot, fabricate missing credentials, or recommend presenting an old epoch as a bypass. Missing/ambiguous identity yields an honest bounded diagnostic with no guessed command.
- Do not overload `GateReport.RunDir` (it describes the requested gate run) with an incumbent's path; the incumbent path travels only inside the human guidance message, safely quoted.
- Preserve the existing top-level blocked/gate-halted result and the closed halt-cause vocabulary: `unavailable` remains the coarse `HaltCause`; the new fields add precision beside it. A genuinely detail-less failure keeps the current generic fallback exactly.
- An admission refusal is never suite failure, gate evidence, a continuation credential, a charged attempt, or permission to retry automatically.
- Extend existing topical test files only (`internal/gatedrive/admission_test.go`, `internal/app/gate_test.go`, `internal/app/gate_drive_test.go`, `internal/app/finalize_rebase_test.go`); no new test harness, no broad source guard.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Keep existing legacy-inventory diagnostics (`Stage: "legacy-inventory"`) distinct and untouched.

---

### Task 1: Incumbent snapshot rides the typed admission refusal (`internal/gatedrive`)

The admission store already reads the incumbent record under the per-slot flock before refusing. Today it throws that knowledge away (`ownershipErr(ErrWorktreeBusy, op)`). Attach a small, credential-free projection to the `OwnershipError` so every consumer diagnoses from the exact record that caused the refusal — never a later re-read (TOCTOU).

**Files:**
- Modify: `internal/gatedrive/ownership.go` (add `IncumbentSnapshot` type + field on `OwnershipError`, next to the existing `Legacy` field)
- Modify: `internal/gatedrive/admission.go` (`reserveWorktreeExecution`: populate the snapshot on the `worktree-busy`, `unresolved-execution`, and `stale-run-epoch` refusal legs)
- Test: `internal/gatedrive/admission_test.go`

**Interfaces:**
- Consumes: existing `admissionRecord` fields (`Kind`, `State`, `DriveID`, `RawRunID`, `RawRunDir`, `RunEpochID`), existing `ownershipErr` constructor, existing `OwnershipError`/`AsOwnershipError`.
- Produces (later tasks rely on these exact names):

```go
// IncumbentSnapshot is a bounded, credential-free projection of the execution
// that occupied a worktree admission slot at the moment a reservation was
// refused. It is captured under the slot's flock from the exact record the
// refusal was decided on, so a later-changed slot is never represented as the
// cause. It carries identity and route facts only — never a reservation token,
// owner generation, capability, argv, or environment.
type IncumbentSnapshot struct {
	Kind       string // "scoped" | "scopeless" | "raw" | "" (unknown)
	State      string // admission state at refusal: "reserved"|"executing"|"stopping"|"unresolved"
	DriveID    string // "" for raw launches
	RawRunID   string // "" until a raw launch was confirmed
	RawRunDir  string // "" until a raw launch was confirmed
	EpochOwned bool   // a run epoch owns the slot (the epoch id itself is not projected)
}
```

and `OwnershipError` gains `Incumbent *IncumbentSnapshot` (nil everywhere except the three admission-refusal legs; `Kind`/`Op`/`Legacy` semantics unchanged so every existing consumer compiles and behaves identically).

- [ ] **Step 1: Write the failing tests**

Append to `internal/gatedrive/admission_test.go` (follow the file's existing fixture helpers for opening a store over a temp git dir and reserving/confirming a slot — reuse whatever helper `TestReserveWorktreeExecution*` tests in that file already use; do not invent a new harness):

```go
// TestReserveRefusalCarriesIncumbentSnapshot proves a worktree-busy refusal
// carries the incumbent's credential-free projection, captured from the exact
// record the refusal was decided on: kind, state, run identity — and never the
// reservation token.
func TestReserveRefusalCarriesIncumbentSnapshot(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t) // reuse/extract the file's existing setup helper
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
	if inc.DriveID != "" || inc.EpochOwned {
		t.Fatalf("raw snapshot leaked drive/epoch facts: %+v", inc)
	}
}

// TestReserveUnresolvedRefusalCarriesSnapshot proves the unresolved-execution
// refusal also snapshots the incumbent, and TestReserveStaleEpochCarriesSnapshot
// proves the stale-run-epoch fence does (EpochOwned true, epoch id NOT projected).
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
	if !ok || oe.Kind != ErrUnresolvedExecution {
		t.Fatalf("err = %v, want unresolved-execution", err)
	}
	if oe.Incumbent == nil || oe.Incumbent.State != "unresolved" || oe.Incumbent.Kind != "raw" {
		t.Fatalf("unresolved snapshot = %+v", oe.Incumbent)
	}
}

func TestReserveStaleEpochCarriesSnapshot(t *testing.T) {
	store, worktree, repoID := newAdmissionFixture(t)
	_, err := store.ReserveWorktreeExecutionForEpoch(repoID, worktree, "epoch-a", nil)
	if err != nil {
		t.Fatalf("epoch reserve: %v", err)
	}
	_, err = store.ReserveRawWorktreeExecution(repoID, worktree, nil)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrStaleRunEpoch {
		t.Fatalf("err = %v, want stale-run-epoch", err)
	}
	if oe.Incumbent == nil || !oe.Incumbent.EpochOwned {
		t.Fatalf("stale-epoch snapshot = %+v", oe.Incumbent)
	}
}
```

Note on fixtures: if `admission_test.go` has no single shared `newAdmissionFixture` helper, extract one from the setup its existing reserve tests repeat (a temp dir as git common dir via `OpenStore`, plus a canonicalized temp worktree dir) rather than duplicating that setup three more times. Real symlink canonicalization means the worktree dir must exist (`t.TempDir()` then `filepath.EvalSymlinks`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gatedrive/ -run 'TestReserve.*Snapshot' -count=1`
Expected: FAIL — `oe.Incumbent undefined` (compile error) or nil snapshot.

- [ ] **Step 3: Implement**

In `internal/gatedrive/ownership.go`, add the `IncumbentSnapshot` type (exact shape from the Interfaces block above) and the field on `OwnershipError`:

```go
	// Incumbent is the credential-free projection of the execution occupying a
	// worktree admission slot, populated ONLY on the worktree-admission refusal
	// legs (worktree-busy, unresolved-execution, stale-run-epoch) from the exact
	// record read under the slot's flock. Nil for every other OwnershipError.
	// Kind/Op/Legacy are unchanged by its presence.
	Incumbent *IncumbentSnapshot
```

In `internal/gatedrive/admission.go`, add a projector next to `reserveWorktreeExecution`:

```go
// incumbentSnapshot projects the refusing slot's record into the bounded,
// credential-free facts a diagnostic may carry. The reservation token, owner
// generations, and the epoch id itself are deliberately excluded.
func incumbentSnapshot(rec admissionRecord) *IncumbentSnapshot {
	return &IncumbentSnapshot{
		Kind:       rec.Kind,
		State:      string(rec.State),
		DriveID:    rec.DriveID,
		RawRunID:   rec.RawRunID,
		RawRunDir:  rec.RawRunDir,
		EpochOwned: rec.RunEpochID != "",
	}
}
```

and thread it onto the three refusal legs inside `reserveWorktreeExecution` (all inside the `rerr == nil` case, so `stored.Record` is exactly the record decided on):

```go
		if stored.Record.RunEpochID != "" && stored.Record.RunEpochID != rec.RunEpochID {
			oe := ownershipErr(ErrStaleRunEpoch, op)
			oe.Incumbent = incumbentSnapshot(stored.Record)
			return "", nil, oe
		}
		switch stored.Record.State {
		case admissionReleased:
			prevGen = stored.Record.ExecutionGen // readmit over a released slot
		case admissionUnresolved:
			oe := ownershipErr(ErrUnresolvedExecution, op)
			oe.Incumbent = incumbentSnapshot(stored.Record)
			return "", nil, oe
		default:
			// reserved, executing, stopping, or any unrecognized non-released
			// state: fail closed as busy — never a free slot.
			oe := ownershipErr(ErrWorktreeBusy, op)
			oe.Incumbent = incumbentSnapshot(stored.Record)
			return "", nil, oe
		}
```

Do NOT touch the `ErrNotFound` (first-admission inventory) leg or the fail-closed `default:` read-error leg — an unreadable record has no trustworthy facts to project (probe error is not clean absence), so those refusals stay snapshot-free.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gatedrive/ -count=1`
Expected: PASS (whole package — the existing admission/driver/epoch-fence tests must stay green; the snapshot is additive).

- [ ] **Step 5: Mutation-check the guard direction**

Temporarily blank one projected field (e.g. set `RawRunID: ""` in `incumbentSnapshot`) and re-run `go test ./internal/gatedrive/ -run 'TestReserve.*Snapshot' -count=1`; it must go RED. Restore the field (undo the edit by hand — do not `git checkout` over uncommitted work) and re-run green.

- [ ] **Step 6: Commit**

```bash
git add internal/gatedrive/ownership.go internal/gatedrive/admission.go internal/gatedrive/admission_test.go
git commit -m "feat(gatedrive): admission refusals carry a credential-free incumbent snapshot (change 0439)"
```

---

### Task 2: Map the snapshot into stage/locator/message on `GateDriveResult` (`internal/app/gate_drive.go`)

`mapDriveResult` already routes typed ownership refusals into `Reason` + a next-action `Message`, and the legacy-inventory family into `Stage`/`Locator`. Extend the same seam: a worktree-admission refusal carrying an incumbent snapshot gets `Stage: "worktree-admission"`, the existing `incumbent-drive:<id>` / `incumbent-run:<id>` locator convention, and a per-kind remedy message. Also correct the blanket `worktree-busy` wording that equates occupancy with a running process.

**Files:**
- Modify: `internal/app/gate_drive.go` (`mapDriveResult`, `ownershipNextAction`; new helpers below)
- Test: `internal/app/gate_drive_test.go` (the file already injects fake engines returning crafted ownership errors — see the tests around `mapDriveResult`'s existing refusal coverage that assert `got.Message != ownershipNextAction(...)`)

**Interfaces:**
- Consumes: `gatedrive.IncumbentSnapshot`, `OwnershipError.Incumbent` (Task 1), `gatedrive.ValidDriveID`, existing `GateDriveResult` fields `Stage`/`Locator`/`Message`/`Reason`, existing `legacyInventoryLocator`.
- Produces (Tasks 3–4 rely on these exact names):

```go
const stageWorktreeAdmission = "worktree-admission"

// incumbentRefusalLocator returns the bounded safe locator for an admission
// refusal's incumbent: "incumbent-drive:<id>" / "incumbent-run:<id>", "" when
// no identity validates. (Same convention as app/gate.go incumbentLocator.)
func incumbentRefusalLocator(inc *gatedrive.IncumbentSnapshot) string

// incumbentRemedyMessage returns the credential-free next-action guidance for
// the refusal kind + incumbent, "" never (always a bounded honest sentence).
func incumbentRemedyMessage(kind gatedrive.OwnershipErrorKind, inc *gatedrive.IncumbentSnapshot) string

// quoteOperand renders a path as a safely single-quoted shell operand for
// human guidance ('...' with embedded ' as '\'').
func quoteOperand(path string) string
```

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/gate_drive_test.go`, following the file's existing crafted-ownership-error pattern (a fake engine whose `Start` returns `(gatedrive.DriveDoc{}, err)`; reuse the same fake the `ErrUnresolvedExecution` message test uses). Table-test the mapping helpers directly plus one end-to-end `Start` case:

```go
// TestMapDriveResultWorktreeAdmissionRefusal proves an admission refusal
// carrying an incumbent snapshot surfaces the typed stage, the safe incumbent
// locator, and per-kind credential-free guidance — and that a snapshot-free
// refusal keeps the existing generic next-action fallback.
func TestMapDriveResultWorktreeAdmissionRefusal(t *testing.T) {
	rawInc := &gatedrive.IncumbentSnapshot{Kind: "raw", State: "executing",
		RawRunID: "0123456789abcdef0123456789abcdef", RawRunDir: "/runs/0123456789abcdef0123456789abcdef"}
	drivenInc := &gatedrive.IncumbentSnapshot{Kind: "scoped", State: "executing", DriveID: validDriveIDForTest(t)}
	epochInc := &gatedrive.IncumbentSnapshot{Kind: "scopeless", State: "executing", EpochOwned: true}
	blankInc := &gatedrive.IncumbentSnapshot{State: "reserved"}

	cases := []struct {
		name       string
		err        *gatedrive.OwnershipError
		wantStage  string
		wantLoc    string
		msgHas     []string
		msgLacks   []string
	}{
		{"raw busy", ownershipErrWith(gatedrive.ErrWorktreeBusy, rawInc),
			"worktree-admission", "incumbent-run:0123456789abcdef0123456789abcdef",
			[]string{"gate observe", "gate stop", "'/runs/0123456789abcdef0123456789abcdef'", "completed"},
			[]string{"token"}},
		{"driven busy", ownershipErrWith(gatedrive.ErrWorktreeBusy, drivenInc),
			"worktree-admission", "incumbent-drive:" + drivenInc.DriveID,
			[]string{"drive"}, []string{"gate stop"}},
		{"epoch owned", ownershipErrWith(gatedrive.ErrStaleRunEpoch, epochInc),
			"worktree-admission", "",
			[]string{"run.cancel"}, []string{"gate stop", "epoch-"}},
		{"unknown identity", ownershipErrWith(gatedrive.ErrWorktreeBusy, blankInc),
			"worktree-admission", "",
			[]string{"occupies"}, []string{"gate stop", "gate observe"}},
		{"no snapshot keeps fallback", ownershipErrWith(gatedrive.ErrWorktreeBusy, nil),
			"", "", []string{ownershipNextAction(gatedrive.ErrWorktreeBusy)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDriveResult(OperationGateDriveStart, gatedrive.DriveDoc{}, tc.err)
			if got.Reason != string(tc.err.Kind) {
				t.Fatalf("reason = %q, want %q", got.Reason, tc.err.Kind)
			}
			if got.Stage != tc.wantStage || got.Locator != tc.wantLoc {
				t.Fatalf("stage/locator = %q/%q, want %q/%q", got.Stage, got.Locator, tc.wantStage, tc.wantLoc)
			}
			for _, s := range tc.msgHas {
				if !strings.Contains(got.Message, s) {
					t.Fatalf("message %q lacks %q", got.Message, s)
				}
			}
			for _, s := range tc.msgLacks {
				if strings.Contains(got.Message, s) {
					t.Fatalf("message %q must not contain %q", got.Message, s)
				}
			}
		})
	}
}

func ownershipErrWith(kind gatedrive.OwnershipErrorKind, inc *gatedrive.IncumbentSnapshot) *gatedrive.OwnershipError {
	return &gatedrive.OwnershipError{Kind: kind, Op: "reserve-worktree-execution", Incumbent: inc}
}

// TestIncumbentRefusalLocatorValidatesIDs proves an invalid drive/run id
// collapses to "" rather than rendering arbitrary bytes into the locator.
func TestIncumbentRefusalLocatorValidatesIDs(t *testing.T) {
	bad := &gatedrive.IncumbentSnapshot{DriveID: "../escape"}
	if got := incumbentRefusalLocator(bad); got != "" {
		t.Fatalf("invalid drive id rendered locator %q", got)
	}
	badRun := &gatedrive.IncumbentSnapshot{RawRunID: "NOT-HEX"}
	if got := incumbentRefusalLocator(badRun); got != "" {
		t.Fatalf("invalid run id rendered locator %q", got)
	}
}

// TestQuoteOperand pins the shell-safe quoting of incumbent paths in guidance.
func TestQuoteOperand(t *testing.T) {
	if got := quoteOperand(`/tmp/o'brien`); got != `'/tmp/o'\''brien'` {
		t.Fatalf("quoteOperand = %q", got)
	}
}
```

(`validDriveIDForTest` — reuse however the file already mints/validates a drive id in fixtures; if nothing exists, use a literal that satisfies `gatedrive.ValidDriveID`, e.g. copy the shape an existing test uses.) Also assert the reworded fallback: update the existing test that pins `ownershipNextAction(gatedrive.ErrWorktreeBusy)` if it quotes the old sentence verbatim.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestMapDriveResultWorktreeAdmission|TestIncumbentRefusalLocator|TestQuoteOperand' -count=1`
Expected: FAIL (undefined helpers / empty stage).

- [ ] **Step 3: Implement**

In `internal/app/gate_drive.go`:

```go
// stageWorktreeAdmission is the typed refusal site for a CURRENT worktree
// admission-slot refusal, distinct from the legacy-inventory stage.
const stageWorktreeAdmission = "worktree-admission"

// rawRunIDShape matches the supervisor's run-id shape (32 lowercase hex); the
// canonical pattern lives in internal/process (see paths.go runIDPattern), and
// this diagnostic-only copy accepts exactly the same ids.
var rawRunIDShape = regexp.MustCompile("^[0-9a-f]{32}$")

func incumbentRefusalLocator(inc *gatedrive.IncumbentSnapshot) string {
	if inc == nil {
		return ""
	}
	switch {
	case inc.DriveID != "" && gatedrive.ValidDriveID(inc.DriveID):
		return "incumbent-drive:" + inc.DriveID
	case inc.RawRunID != "" && rawRunIDShape.MatchString(inc.RawRunID):
		return "incumbent-run:" + inc.RawRunID
	default:
		return ""
	}
}

func quoteOperand(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func incumbentRemedyMessage(kind gatedrive.OwnershipErrorKind, inc *gatedrive.IncumbentSnapshot) string {
	switch {
	case kind == gatedrive.ErrStaleRunEpoch || (inc != nil && inc.EpochOwned):
		return "a workflow run epoch owns this worktree's execution slot; continue that run through its own gate-drive continuation, or cancel it with the run.cancel operation using that run's key and epoch — never a raw gate stop and never a stale epoch"
	case inc != nil && inc.Kind == "raw" && inc.RawRunDir != "" && rawRunIDShape.MatchString(inc.RawRunID):
		dir := quoteOperand(inc.RawRunDir)
		return "a raw gate run occupies this worktree's execution slot; the slot stays occupied until explicit teardown, even after the run completes. Inspect it with docket gate observe " + dir +
			", then settle the slot with docket gate stop " + dir + " --reason <why> — stopping a still-running run cancels it; stopping an already-completed run settles its slot (the stop operation itself decides whether teardown is proven)"
	case inc != nil && inc.Kind == "raw":
		return "a raw gate reservation occupies this worktree's execution slot but its run identity is not recorded; do not start a second gate here — resolve the incumbent before retrying"
	case inc != nil && inc.DriveID != "":
		return "a driven gate occupies this worktree's execution slot; advance or recover it through its owning drive's continuation (gate drive advance / the parent workflow), never a raw gate stop"
	case kind == gatedrive.ErrUnresolvedExecution:
		return ownershipNextAction(gatedrive.ErrUnresolvedExecution)
	default:
		return "an execution occupies this worktree's admission slot but its identity could not be established; do not start a second gate here and do not guess a stop target — resolve the incumbent first"
	}
}
```

In `mapDriveResult`, extend the ownership-error branch — legacy-inventory stays FIRST and untouched; the admission family comes next; everything else keeps the existing `ownershipNextAction` path:

```go
		if oe, ok := gatedrive.AsOwnershipError(err); ok {
			if stage, locator, isInventory := legacyInventoryLocator(oe.Op); isInventory {
				// (existing legacy-inventory block, unchanged)
			} else if oe.Incumbent != nil {
				// A CURRENT worktree-admission refusal diagnoses from the exact
				// incumbent snapshot the refusal was decided on (never a re-read).
				result.Stage = stageWorktreeAdmission
				result.Locator = incumbentRefusalLocator(oe.Incumbent)
				result.Message = incumbentRemedyMessage(oe.Kind, oe.Incumbent)
			} else {
				result.Message = ownershipNextAction(oe.Kind)
			}
		}
```

Reword the `ErrWorktreeBusy` fallback in `ownershipNextAction` from "this worktree already runs a gate execution; wait for it or cancel that run — do not start a second in the same worktree" to slot-occupancy wording:

```go
	case gatedrive.ErrWorktreeBusy:
		return "this worktree's gate execution slot is occupied (the occupying process may have already completed); wait for the incumbent or settle its slot through its own stop/cancel route — do not start a second gate in the same worktree"
```

`GateDriveResult.HumanText` already renders `reason:`, `message:`, `stage:`, `locator:` lines — no change needed there; verify by eye.

Note: `Stage`/`Locator` doc comment on `GateDriveResult` currently says "Empty for every other refusal" about the inventory family — update that comment to name both families (`legacy-inventory` and `worktree-admission`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestMapDriveResult|TestIncumbentRefusalLocator|TestQuoteOperand|TestGateDrive' -count=1`
Expected: PASS. If an existing test pinned the old worktree-busy sentence verbatim, update it to the new wording (the property — no blind second start — is unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/app/gate_drive.go internal/app/gate_drive_test.go
git commit -m "feat(app): worktree-admission refusals surface incumbent locator and per-kind remedy (change 0439)"
```

---

### Task 3: Raw `GateLaunch` refusal diagnoses from the refusal's own snapshot (`internal/app/gate.go`)

Today `GateLaunch`'s admission-failure path calls `incumbentLocator(store, worktreeRoot)` — a post-refusal RE-READ of the slot, so a slot that changed between refusal and read would be misreported as the cause. Spec: "Diagnostic facts come from the refusal's incumbent snapshot; a later changed slot must never be represented as its cause." Route the raw path through the same snapshot.

**Files:**
- Modify: `internal/app/gate.go` (the `ReserveRawWorktreeExecution` failure branch in `GateLaunch`; `incumbentLocator` doc comment)
- Test: `internal/app/gate_test.go`

**Interfaces:**
- Consumes: `incumbentRefusalLocator` (Task 2), `OwnershipError.Incumbent` (Task 1).
- Produces: no new exported surface. `GateResult.Cause` keeps the exact `incumbent-run:<id>` / `incumbent-drive:<id>` shapes (the existing test `TestGateLaunchSecondRefusedWhileFirstLives` asserts `Cause` contains the first run's id — that must stay green).

- [ ] **Step 1: Write the failing test**

Append to `internal/app/gate_test.go` a unit test on the branch's new helper (the integration behavior is already pinned by `TestGateLaunchSecondRefusedWhileFirstLives`):

```go
// TestGateLaunchRefusalCauseFromSnapshot proves the admission-refusal cause is
// derived from the refusal's own incumbent snapshot, not a post-refusal re-read:
// a snapshot-bearing error yields its locator; a snapshot-free error yields "".
func TestGateLaunchRefusalCauseFromSnapshot(t *testing.T) {
	withInc := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "reserve-worktree-execution",
		Incumbent: &gatedrive.IncumbentSnapshot{Kind: "raw", RawRunID: "0123456789abcdef0123456789abcdef"}}
	if got := admissionRefusalCause(withInc); got != "incumbent-run:0123456789abcdef0123456789abcdef" {
		t.Fatalf("cause = %q", got)
	}
	bare := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "reserve-worktree-execution"}
	if got := admissionRefusalCause(bare); got != "" {
		t.Fatalf("snapshot-free cause = %q, want empty", got)
	}
	if got := admissionRefusalCause(errors.New("io")); got != "" {
		t.Fatalf("non-ownership cause = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestGateLaunchRefusalCauseFromSnapshot -count=1`
Expected: FAIL — `admissionRefusalCause` undefined.

- [ ] **Step 3: Implement**

In `internal/app/gate.go`:

```go
// admissionRefusalCause derives the refusal's safe incumbent locator from the
// snapshot the ownership error itself carries — the exact record the refusal
// was decided on under the admission lock. It never re-reads the slot: a later
// changed slot must not be represented as this refusal's cause. A snapshot-free
// or non-ownership error yields "".
func admissionRefusalCause(err error) string {
	oe, ok := gatedrive.AsOwnershipError(err)
	if !ok {
		return ""
	}
	return incumbentRefusalLocator(oe.Incumbent)
}
```

and swap the reserve-failure branch in `GateLaunch`:

```go
		t, aerr := store.ReserveRawWorktreeExecution(repoIdentity, worktreeRoot, svc)
		if aerr != nil {
			r, reason := mapAdmissionFailure(aerr)
			return GateResult{Envelope: NewEnvelope(OperationGateLaunch, r), Reason: reason, Cause: admissionRefusalCause(aerr)}
		}
```

Leave `rawStaleEpochRefusal` unchanged: it decides from its own `LoadWorktreeExecution` read and reports that same copy — decide-and-act on one copy is already satisfied there. `incumbentLocator` keeps that one caller; update its doc comment to note the admission-refusal path now uses the error-borne snapshot (anchor prose on `admissionRefusalCause`, no line numbers).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestGateLaunch|TestGateStop' -count=1`
Expected: PASS — including the pre-existing `TestGateLaunchSecondRefusedWhileFirstLives` (cause still locates the incumbent run, now via the snapshot) and `TestGateStopReleasesRawSlot` (release + readmission untouched; this is the spec's acceptance-2 regression, already in the suite).

- [ ] **Step 5: Commit**

```bash
git add internal/app/gate.go internal/app/gate_test.go
git commit -m "fix(app): raw launch refusal cause comes from the refusal's incumbent snapshot (change 0439)"
```

---

### Task 4: Carry the refusal through finalize's local gate into JSON and human output (`internal/app/finalize_rebase.go`)

Today `mapDriveOutcome` collapses every no-drive-document result to `{Halted, unavailable}`, and the composition renders only the generic "the local gate did not reach a decidable pass/fail". Carry the four diagnostic fields through `LocalGateResult` into `GateReport`, on both the initial `Start` and continued `Advance` slices (both flow through `mapDriveOutcome`).

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`LocalGateResult`, `GateReport`, `mapDriveOutcome`, the `FinalizeGateHalted` composition branch, `FinalizeRebaseResult.HumanText`)
- Test: `internal/app/finalize_rebase_test.go` (uses the existing `fakeGate` scripted seam) and the `mapDriveOutcome` unit tests wherever the file already tests `processFinalizeGate` mapping (search for existing `mapDriveOutcome` tests; if none exist at unit level, the new one below stands alone in `finalize_rebase_test.go`)

**Interfaces:**
- Consumes: `GateDriveResult{Reason, Message, Stage, Locator}` populated by Task 2.
- Produces:

```go
// LocalGateResult gains (halt-diagnostic fields, set on Halted only; same
// meanings as GateDriveResult's fields of the same name):
	HaltReason  string
	HaltMessage string
	HaltStage   string
	HaltLocator string

// GateReport gains (optional; populated on a halted gate that carried detail):
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Stage   string `json:"stage,omitempty"`
	Locator string `json:"locator,omitempty"`
```

- [ ] **Step 1: Write the failing tests**

```go
// TestMapDriveOutcomeCarriesRefusalDetail proves a Start/Advance command
// failure that carried a typed refusal (no drive document) maps to a halted
// LocalGateResult that PRESERVES reason/message/stage/locator beside the coarse
// unavailable cause — and that a detail-less failure keeps the exact current
// generic shape.
func TestMapDriveOutcomeCarriesRefusalDetail(t *testing.T) {
	g := &processFinalizeGate{}
	out := GateDriveResult{
		Envelope: NewEnvelope(OperationGateDriveStart, ResultInvalidInput),
		Reason:   "worktree-busy",
		Message:  "a raw gate run occupies this worktree's execution slot; ...",
		Stage:    stageWorktreeAdmission,
		Locator:  "incumbent-run:0123456789abcdef0123456789abcdef",
	}
	got := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, out)
	if got.Outcome != FinalizeGateHalted || got.HaltCause != GateHaltUnavailable {
		t.Fatalf("outcome/cause = %v/%q", got.Outcome, got.HaltCause)
	}
	if got.HaltReason != "worktree-busy" || got.HaltStage != stageWorktreeAdmission ||
		got.HaltLocator != "incumbent-run:0123456789abcdef0123456789abcdef" || got.HaltMessage == "" {
		t.Fatalf("refusal detail dropped: %+v", got)
	}
	if got.RunDir != "" {
		t.Fatalf("run_dir must not carry incumbent facts: %q", got.RunDir)
	}
	bare := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{})
	if bare.HaltReason != "" || bare.HaltMessage != "" || bare.HaltStage != "" || bare.HaltLocator != "" {
		t.Fatalf("detail-less failure grew detail: %+v", bare)
	}
}

// TestFinalizeRebaseGateHaltCarriesAdmissionRefusal proves the composition
// carries the halt detail into GateReport (JSON) and the human line, keeping the
// blocked disposition, rebase-gate-halted reason, and unavailable halt cause.
func TestFinalizeRebaseGateHaltCarriesAdmissionRefusal(t *testing.T) {
	// Fixture: identical setup to the file's existing gate-halted test (find the
	// fakeGate test that scripts Outcome: FinalizeGateHalted; clone its fixture),
	// with the scripted result extended:
	gate := &fakeGate{result: LocalGateResult{
		Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable,
		HaltReason:  "worktree-busy",
		HaltMessage: "a raw gate run occupies this worktree's execution slot; settle it with docket gate stop '/runs/x' --reason <why>",
		HaltStage:   stageWorktreeAdmission,
		HaltLocator: "incumbent-run:0123456789abcdef0123456789abcdef",
	}}
	// ... run FinalizeRebase via the fixture as the sibling test does ...
	// Assertions:
	// res.Result == ResultBlocked, res.Reason == ReasonRebaseGateHalted
	// res.Gate.HaltCause == GateHaltUnavailable
	// res.Gate.Reason == "worktree-busy"
	// res.Gate.Stage == stageWorktreeAdmission
	// res.Gate.Locator == "incumbent-run:0123456789abcdef0123456789abcdef"
	// res.Gate.Message contains "gate stop"
	// res.Gate.RunDir == "" (never the incumbent's path)
	// res.HumanText() contains "worktree-busy" and "incumbent-run:0123456789abcdef0123456789abcdef"
	//   and "gate stop" (reason + locator + remedy reach the human line)
}

// TestFinalizeRebaseGateHaltGenericUnchanged proves a detail-less halt keeps
// today's generic output exactly: Gate.Reason/Message/Stage/Locator all empty,
// result message "the local gate did not reach a decidable pass/fail; retained,
// no red fabricated", and a HumanText without any locator fragment.
```

Write `TestFinalizeRebaseGateHaltGenericUnchanged` in full against the existing halted-path fixture (scripting only `Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable`). These two composition tests must reuse the file's existing `rebaseFixture` + `finalizeDeps(gh, gate)` helpers verbatim — copy the arrangement from the nearest existing halted/blocked gate test in the same file rather than inventing setup.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestMapDriveOutcomeCarriesRefusalDetail|TestFinalizeRebaseGateHalt' -count=1`
Expected: FAIL — fields undefined.

- [ ] **Step 3: Implement**

1. Add the four fields to `LocalGateResult` and four to `GateReport` (shapes in the Interfaces block; document on `GateReport` that they mirror `GateDriveResult`'s fields of the same name and are populated on a halted gate that carried a typed refusal).
2. In `mapDriveOutcome`, replace the bare no-document branch:

```go
	if out.Drive == nil {
		// A command failure produced no drive document. The coarse classification
		// stays unavailable, but a typed refusal's bounded detail (e.g. a
		// worktree-admission refusal's incumbent locator and remedy) is preserved
		// verbatim rather than swallowed — it is diagnosis, never gate evidence,
		// a charged attempt, or permission to retry.
		return LocalGateResult{
			Outcome:     FinalizeGateHalted,
			HaltCause:   GateHaltUnavailable,
			HaltReason:  out.Reason,
			HaltMessage: out.Message,
			HaltStage:   out.Stage,
			HaltLocator: out.Locator,
		}
	}
```

3. In the composition's `default: // FinalizeGateHalted` branch (the switch on `gres.Outcome` after `base.Gate = &GateReport{...}`), stamp the detail and upgrade the result message only when detail exists:

```go
	default: // FinalizeGateHalted
		base.Disposition = RebaseDispBlocked
		base.Gate.HaltCause = gres.HaltCause
		base.Gate.Reason = gres.HaltReason
		base.Gate.Message = gres.HaltMessage
		base.Gate.Stage = gres.HaltStage
		base.Gate.Locator = gres.HaltLocator
		base.Reason = ReasonRebaseGateHalted
		base.Message = "the local gate did not reach a decidable pass/fail; retained, no red fabricated"
		if gres.HaltReason != "" {
			base.Message = "the local gate could not start: " + gres.HaltReason
			if gres.HaltMessage != "" {
				base.Message += " — " + gres.HaltMessage
			}
		}
		out := newRebaseResult(op, ResultBlocked, base)
		clearGateContinuation(ctx, deps, rc, rec, &out)
		return out
```

4. In `FinalizeRebaseResult.HumanText`, after the existing switch, surface reason + locator so the one-line summary is actionable (the remedy already rides `Message` in the composed JSON; for the human line append the bounded pieces):

```go
	if r.Gate != nil && r.Gate.Reason != "" {
		s += " [gate: " + r.Gate.Reason
		if r.Gate.Locator != "" {
			s += " " + r.Gate.Locator
		}
		if r.Gate.Message != "" {
			s += " — " + r.Gate.Message
		}
		s += "]"
	}
```

Do not touch the earlier `gerr != nil` seam-failure branch (a genuinely detail-less unrecoverable seam error keeps today's generic message), the skip/permit path, or `run_dir` handling anywhere.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestFinalizeRebase|TestMapDriveOutcome' -count=1`
Expected: PASS, including every pre-existing rebase test (passed / waiting / failed / halted shapes unchanged where no detail exists).

- [ ] **Step 5: Mutation-check the carry**

Temporarily drop `HaltLocator: out.Locator` from `mapDriveOutcome`; `TestMapDriveOutcomeCarriesRefusalDetail` and `TestFinalizeRebaseGateHaltCarriesAdmissionRefusal` must go RED (`-count=1` to defeat the test cache). Restore by hand and re-run green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_test.go
git commit -m "fix(app): finalize carries the local gate's typed admission refusal into GateReport and human output (change 0439)"
```

---

### Task 5: Maintained docs — slot occupancy, recovery boundaries

**Files:**
- Modify: `docs/concepts/run-gate.md`
- Modify (only if it discusses the local gate's halt/recovery — read it first): `docs/concepts/finalize-sequencer.md`

**Interfaces:** none (prose). Point-in-time records (results files, archived changes, specs, Accepted ADRs) are NOT touched.

- [ ] **Step 1: Read both docs end to end**

Find where gate recovery, stopping, and admission are explained (`docs/concepts/run-gate.md` has a lifecycle diagram mentioning `gate-stop / gate-observe`).

- [ ] **Step 2: Write the clarifications**

In `docs/concepts/run-gate.md`, where the worktree admission slot / stopping is described, add or amend prose covering exactly these three facts (wording may be adapted to the doc's voice; the claims may not be weakened):

1. A busy admission slot means the slot is **occupied**, not that its process is still running: a raw gate run retains its slot until explicit teardown, deliberately, even after the run completes. `docket gate observe <run-dir>` inspects it; `docket gate stop <run-dir> --reason <why>` settles it — stopping a still-running run is cancellation, stopping a completed run settles its slot, and the stop operation itself decides whether teardown is proven.
2. Gate history cleanup assesses **historical drives** only; a zero-blocker cleanup result does not establish that the current worktree admission slot is free.
3. Process recovery (`gate recover`) classifies process records and does not release a current raw admission slot.

Anchor any cross-reference on symbol names (`reserveWorktreeExecution`, `releaseRawSlotForStop`) or verbatim-quoted clauses — never line numbers (ADR-0054).

- [ ] **Step 3: Verify no guard reddens**

Run: `go test ./internal/repoguard/ -count=1`
Expected: PASS (anchor-style and prose guards stay green).

- [ ] **Step 4: Commit**

```bash
git add docs/concepts/run-gate.md docs/concepts/finalize-sequencer.md
git commit -m "docs: clarify occupied gate slots, stop-settles-slot, and cleanup/recovery boundaries (change 0439)"
```

(Drop `finalize-sequencer.md` from the add if it needed no edit.)

---

### Task 6: Whole-suite gate

- [ ] **Step 1: Run the full suite through the configured build gate**

Resolve the command from config (`build.test_command`; currently `go run ./cmd/docket development test`) and run it from source in this worktree. Never a hand-rolled subset, never a second copy of the command.

Expected: SUITE green.

- [ ] **Step 2: Read the budget report**

Even on green: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on (serial-confirm per `tests/README.md` before treating a parallel wall-clock number as real).

- [ ] **Step 3: Thesis audit over the branch's own additions**

This change's thesis is "a typed refusal must not be swallowed into a generic message, and a printed remedy must be valid in the exact state that produced it." Apply it to the diff's own additions before review:

- Does any NEW mapping branch drop `Reason`/`Message`/`Stage`/`Locator` on some input shape (e.g. the `Advance` path, the `gerr != nil` path)?
- Is every printed remedy valid in the state that triggers it (the raw-run stop guidance renders only when a confirmed `RawRunDir` exists; the driven/epoch messages never name a raw stop)?
- Do the two locator id validators (`gatedrive.ValidDriveID`, `rawRunIDShape`) cover every string interpolated into a locator or quoted operand?

Fix anything found, with a focused test, before hand-off to review.

---

## Self-Review (completed by the plan author)

- **Spec coverage:** admission-refusal enrichment with snapshot + `worktree-admission` stage (Tasks 1–2); locator convention + credential exclusion (Tasks 1–3); per-kind observe/stop/continuation/run-cancel guidance with no guessed commands on ambiguity (Task 2); snapshot-not-re-read TOCTOU rule at the raw boundary (Task 3); carry-through to `LocalGateResult`/`GateReport`/JSON/human on initial and continued slices, `run_dir` not overloaded, generic fallback preserved, no charged attempt/retry authorization (Task 4); maintained-docs boundary clarifications (Task 5); acceptance 1–2's supported raw-launch regression rides the existing `TestGateLaunchSecondRefusedWhileFirstLives` + `TestGateStopReleasesRawSlot` pinned green in Task 3 plus the new snapshot/locator asserts; acceptance 4's fail-closed reserved/unresolved + quoting/credential tests are in Tasks 1–2; acceptance 5's legacy-inventory-distinct + generic-failure mapping tests are in Tasks 2 and 4; acceptance 6's full-suite gate is Task 6. Out-of-scope list honored: no new CLI command, config, schema, state, recovery policy, or ADR anywhere in the plan.
- **Type consistency:** `IncumbentSnapshot`/`OwnershipError.Incumbent` (Task 1) are consumed by exactly those names in Tasks 2–3; `stageWorktreeAdmission`, `incumbentRefusalLocator`, `quoteOperand` (Task 2) are consumed in Tasks 3–4; `HaltReason/HaltMessage/HaltStage/HaltLocator` and `GateReport.Reason/Message/Stage/Locator` are used consistently in Task 4's tests and implementation.
- **Placeholder scan:** the two composition tests in Task 4 delegate their *fixture arrangement* to the named sibling tests in the same file (deliberate — the file's `rebaseFixture` helpers are the house idiom and must be reused, not restated); every assertion they must make is enumerated. No unfinished-work markers remain anywhere in this plan.
