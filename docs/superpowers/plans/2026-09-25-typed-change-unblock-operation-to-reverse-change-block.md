<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0450 — Typed change.unblock operation to reverse change.block](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0450-typed-change-unblock-operation-to-reverse-change-block.md)**
<!-- docket:backlink:end -->
# Typed `change.unblock` and `change.revive` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the existing `domain.Unblock` (`blocked` → `in-progress`, clearing `blocked_by`) and `domain.Revive` (`deferred` → `proposed`) transitions as typed operations `change.unblock` / `change.revive` through the existing `executeChangeLifecycle` driver, replacing the hand-edit workflow.

**Architecture:** No new mechanism. Two new request types and entry points in `internal/app/change_lifecycle.go` compose the same shared driver `change.block` / `change.defer` use (exact-blob version pin, domain legality gate, `updated:` refresh, artifact-block and inline-board re-render, one metadata commit). Wiring mirrors `change.defer` at every enumeration site: CLI subcommands, schema registry, shadow/schema-tag/CLI/asset-independence tests, and the docket-convention lifecycle prose.

**Tech Stack:** Go (stdlib + repo-internal packages only), Cobra CLI, the repo's transaction engine, `go generate ./internal/assets/` for the embedded skill bundle.

**Spec:** `docs/superpowers/specs/2026-09-24-typed-change-unblock-operation-to-reverse-change-block-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary checkout).

## Global Constraints

- Change id: 0450. Feature branch: `chore/typed-change-unblock-operation-to-reverse-change-block`. Work only in this feature worktree; never write docket metadata.
- Operation ids are exactly `change.unblock` and `change.revive`; CLI subcommands are exactly `docket change unblock` / `docket change revive`.
- No `reason` field on either request (spec: YAGNI). Requests carry only `change_id`, `path`, `version`.
- Result type is the existing `ChangeLifecycleResult`, unchanged.
- Wrong source status is refused via the domain's `requireStatus` failure mapped to `invalid-state`; nothing is written.
- Revive leaves the body alone: `## Why deferred` kept, `branch:` and `claimed_at:` byte-intact. Do not change `change.block` or `domain.Revive` semantics.
- Out of scope: automatic unblocking, `domain.KillStackParent` recovery, `finalize.block`/`finalize.clear-block`.
- The final build gate is the whole suite via `go run ./cmd/docket development test` (run once by the build skill's gate, not per task). Per-task verification uses focused `go test` commands as written in each task; note `internal/app/change_integration_test.go` is behind `//go:build integration`, so those runs need `-tags integration`.
- `git add` only the exact files each task names — never `git add -A` (shared-loop discipline).

## Review Focus

Spec-implied conditions no single task's happy path covers; each has its pinning test added to the owning task:

1. **Unblock/revive on a github board surface** must refuse before any engine call (users on the github surface would otherwise half-write) — Task 1 fence tests.
2. **Version drift between read and submit** must map to `contended`, not a write over a moved record — Task 2 recording-engine drift assertions.
3. **A halted-then-blocked change (the 0444 path)** must come all the way back: halt → block → unblock → resume-halted removes `## Run halted` — Task 2 resume regression.
4. **A Bash-era record lacking `updated:`** must not internal-error on unblock (the driver upserts) — Task 1 missing-updated test for unblock.
5. **Revive of a record whose `## Why deferred` is absent** (deferred by an old tool or hand edit) must still apply — revive passes no section edits, so absence is legal — Task 1 plan test uses a record without the section.

---

### Task 1: `change.unblock` / `change.revive` app operations

**Files:**
- Modify: `internal/app/change_lifecycle.go`
- Test: `internal/app/change_lifecycle_test.go`

**Interfaces:**
- Consumes: `executeChangeLifecycle`, `validateLifecycleShape`, `newChangeLifecycleResult`, `domain.Unblock`, `domain.Revive` — all already present.
- Produces: `OperationChangeUnblock = "change.unblock"`, `OperationChangeRevive = "change.revive"`, `type ChangeUnblockRequest struct { ChangeID int; Path string; Version string }` (json/docket tags as below), `type ChangeReviveRequest` (identical fields), `func ChangeUnblock(ctx, deps PlanningDeps, repoDir string, req ChangeUnblockRequest) ChangeLifecycleResult`, `func ChangeRevive(…, req ChangeReviveRequest) ChangeLifecycleResult`. Tasks 2–3 use these names exactly.

- [ ] **Step 1: Write the failing tests**

In `internal/app/change_lifecycle_test.go`, mirror the existing block/defer tests (same file, same helpers). Add:

```go
func TestChangeUnblockRejectsBadShapeWithoutEngineCall(t *testing.T) {
	// Clone TestChangeBlockRejectsBadShapeWithoutEngineCall's cases minus the
	// empty-reason case: non-positive id, empty path, empty version each yield
	// ResultInvalidInput with the same finding codes (invalidIDCode("change_id"),
	// FCEmptyPath, FCEmptyVersion) and zero engine calls.
}

func TestChangeReviveRejectsBadShapeWithoutEngineCall(t *testing.T) { /* same */ }

func TestChangeUnblockFencesGithubBoardSurface(t *testing.T) {
	// Clone TestChangeBlockFencesGithubBoardSurface for ChangeUnblock.
}

func TestChangeReviveFencesGithubBoardSurface(t *testing.T) { /* same */ }
```

Add op helpers beside `blockOp`/`deferOp` (follow their exact shape at the `baseLifecycleOp` call sites):

```go
func unblockOp(surfaces []string, id int, recPath string) changeLifecycleOp {
	return baseLifecycleOp(OperationChangeUnblock, surfaces, id, recPath,
		func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) { return domain.Unblock(c) }, nil)
}

func reviveOp(surfaces []string, id int, recPath string) changeLifecycleOp {
	return baseLifecycleOp(OperationChangeRevive, surfaces, id, recPath,
		func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) { return domain.Revive(c) }, nil)
}
```

Plan-behaviour tests, following `TestChangeBlockPlanFileSet` / `TestChangeBlockPlanSourceStatusMatrix` byte-for-byte in structure:

```go
func TestChangeUnblockPlanFileSet(t *testing.T) {
	// Fixture: a record with status: 'blocked' and blocked_by: 'waiting on 0446'.
	// Assert the mutated record has status: 'in-progress', blocked_by: (bare
	// null form — lifecycleFieldValue("") renders document.Null()), updated:
	// refreshed to the test clock date, the docket:artifacts block re-rendered,
	// and — inline surface — BOARD.md in the file set. Commit subject is
	// "change 0003 → in-progress" (fmt "change %04d → %s"); receipt decodes to
	// changeLifecycleReceipt{ID: 3, Op: "change.unblock", Status: "in-progress"}.
}

func TestChangeRevivePlanFileSet(t *testing.T) {
	// Fixture: status: 'deferred' WITHOUT a ## Why deferred section (Review
	// Focus 5), carrying branch: and claimed_at:. Assert status: 'proposed',
	// updated: refreshed, branch:/claimed_at: byte-intact, no section added.
}

func TestChangeRevivePlanPreservesWhyDeferredAndClaim(t *testing.T) {
	// Fixture: status: 'deferred' WITH "## Why deferred\n\nParked for X." plus
	// branch: 'feat/widget' and claimed_at:. Assert the section body, branch,
	// and claimed_at survive byte-identical in the mutated record.
}

func TestChangeUnblockPlanSourceStatusMatrix(t *testing.T) {
	// Clone TestChangeBlockPlanSourceStatusMatrix: every non-'blocked' status
	// refuses with the domain's requireStatus reason token as the finding code,
	// Refused: true, no files planned.
}

func TestChangeRevivePlanSourceStatusMatrix(t *testing.T) {
	// Same for every non-'deferred' status.
}

func TestChangeUnblockPlanToleratesMissingUpdatedField(t *testing.T) {
	// Clone TestChangeBlockPlanToleratesMissingUpdatedField for unblock
	// (Review Focus 4): a record without updated: gains the field, no error.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestChangeUnblock|TestChangeRevive' -count=1`
Expected: FAIL to compile — `OperationChangeUnblock`, `ChangeUnblockRequest`, `ChangeUnblock`, etc. undefined.

- [ ] **Step 3: Implement the operations**

In `internal/app/change_lifecycle.go`:

```go
const (
	OperationChangeBlock   = "change.block"
	OperationChangeDefer   = "change.defer"
	OperationChangeUnblock = "change.unblock"
	OperationChangeRevive  = "change.revive"
)

// ChangeUnblockRequest is the closed, caller-supplied request for one unblock.
// Path and Version pin the exact submitted record. No reason field: the commit
// subject records the transition and the cleared blocked_by text stays in git
// history.
type ChangeUnblockRequest struct {
	ChangeID int    `json:"change_id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Version  string `json:"version" docket:"required"`
}

// ChangeReviveRequest is the closed, caller-supplied request for one revive.
type ChangeReviveRequest struct {
	ChangeID int    `json:"change_id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Version  string `json:"version" docket:"required"`
}

// ChangeUnblock validates the request, pins authoritative context, and drives
// one atomic transaction that unblocks the change (blocked → in-progress,
// clearing blocked_by) and — when inline is enabled — re-renders the board.
func ChangeUnblock(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeUnblockRequest) ChangeLifecycleResult {
	findings := validateLifecycleShape("change_id", req.ChangeID, req.Path, req.Version)
	if len(findings) > 0 {
		return newChangeLifecycleResult(OperationChangeUnblock, ResultInvalidInput, ChangeLifecycleResult{Findings: findings})
	}
	action := func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) {
		return domain.Unblock(c)
	}
	return executeChangeLifecycle(ctx, deps, repoDir, OperationChangeUnblock, req.ChangeID, req.Path, req.Version, action, nil)
}

// ChangeRevive validates the request, pins authoritative context, and drives
// one atomic transaction that revives the change (deferred → proposed). The
// ## Why deferred section, branch, and claim stamp are left untouched, per
// domain.Revive's contract.
func ChangeRevive(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeReviveRequest) ChangeLifecycleResult {
	findings := validateLifecycleShape("change_id", req.ChangeID, req.Path, req.Version)
	if len(findings) > 0 {
		return newChangeLifecycleResult(OperationChangeRevive, ResultInvalidInput, ChangeLifecycleResult{Findings: findings})
	}
	action := func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) {
		return domain.Revive(c)
	}
	return executeChangeLifecycle(ctx, deps, repoDir, OperationChangeRevive, req.ChangeID, req.Path, req.Version, action, nil)
}
```

Update the two comments the spec names so they describe all four transitions:

- The file-header comment (`// This file is the `change block` and `change defer` planning operations…`) → "the `change block`, `change defer`, `change unblock`, and `change revive` planning operations", keeping the rest of its claims accurate (defer is still the only one with an authored section; unblock/revive edit no sections).
- The `lifecycleFieldValue` trailing sentence ("Block and defer only ever set string-valued owned fields (status, blocked_by).") → "The lifecycle transitions only ever set string-valued owned fields (status, blocked_by); unblock clears blocked_by via the empty target."

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestChange(Block|Defer|Unblock|Revive)|TestLifecycle' -count=1`
Expected: PASS (new and pre-existing lifecycle tests).

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_lifecycle.go internal/app/change_lifecycle_test.go
git commit -m "feat(app): typed change.unblock and change.revive operations (change 0450)"
```

---

### Task 2: Integration coverage — applied results, drift, and the 0444 resume regression

**Files:**
- Test: `internal/app/change_integration_test.go`

**Interfaces:**
- Consumes: `ChangeUnblock`, `ChangeRevive`, `ChangeUnblockRequest`, `ChangeReviveRequest` (Task 1); existing helpers `recordingEngine`, `fakeChangeReader`, `mainModePin`, `mustMarshal`, `changeLifecycleReceipt`, `newGitClient`, `testClock`, `setupHaltedFixture`, `planRepoModes`, `originFile`, `groomPath`, and the existing `validBlockRequest()` pattern.
- Produces: nothing consumed later; pure test coverage.

- [ ] **Step 1: Write the failing tests**

All in `internal/app/change_integration_test.go` (note `//go:build integration`). Mirror `TestIntegrationChangeAuthoringBlockAppliedResult` / `…DeferAppliedResultCarriesDeferStatus`:

```go
func validUnblockRequest() ChangeUnblockRequest {
	// Same id/path/version literals validBlockRequest uses, minus Reason.
}
func validReviveRequest() ChangeReviveRequest { /* same */ }

func TestIntegrationChangeAuthoringUnblockAppliedResult(t *testing.T) {
	// recordingEngine returns DispositionApplied with receipt
	// changeLifecycleReceipt{ID: 3, Op: OperationChangeUnblock, Status: "in-progress"}.
	// Assert Result applied; ID 3; Status "in-progress"; Revision equals the
	// engine's AppliedCommit; Operation == OperationChangeUnblock; exactly one
	// engine call whose single entity expectation pins the request's path at
	// the exact blob version (clone the Expected assertions from the block test).
}

func TestIntegrationChangeAuthoringReviveAppliedResultCarriesProposedStatus(t *testing.T) {
	// Same shape; receipt Status "proposed"; Operation == OperationChangeRevive.
}

func TestIntegrationChangeAuthoringUnblockContendedOnVersionDrift(t *testing.T) {
	// recordingEngine returns transaction.Result{Disposition: transaction.DispositionContended};
	// assert res.Result == ResultContended for ChangeUnblock (Review Focus 2).
}

func TestIntegrationChangeAuthoringReviveContendedOnVersionDrift(t *testing.T) { /* same */ }
```

The 0444 regression (Review Focus 3), beside `TestIntegrationChangeRuntimeResumeHalted` and reusing its fixture:

```go
// TestIntegrationChangeRuntimeUnblockThenResumeHalted proves the 0444 path as
// typed operations end to end: a halted in-progress change is blocked, then
// unblocked (status back to in-progress, blocked_by cleared, ## Run halted
// preserved), then resume-halted succeeds and removes exactly the marker.
func TestIntegrationChangeRuntimeUnblockThenResumeHalted(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			f := setupHaltedFixture(t, m)
			// 1. Block through the real engine. Re-read the record's current blob
			//    version from origin before each step (originFile gives the bytes;
			//    hash or re-list for the blob id the same way the fixture derives
			//    f.version — follow whatever helper setupHaltedFixture uses).
			// 2. Assert origin record: status: 'blocked', blocked_by recorded,
			//    "## Run halted" still present.
			// 3. Unblock through the real engine with the post-block version.
			//    Assert applied; origin record: status: 'in-progress',
			//    blocked_by: cleared (bare null form), "## Run halted" preserved.
			// 4. ChangeResumeHalted with a quiescent fake workspace (clone the
			//    "quiescent-resumes" subtest's call) and the post-unblock version.
			//    Assert applied and "## Run halted" gone from the origin record.
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationChangeAuthoringUnblock|TestIntegrationChangeAuthoringRevive|TestIntegrationChangeRuntimeUnblockThenResumeHalted' -count=1`
Expected: FAIL only if Task 1 is absent (compile) — with Task 1 in place these should PASS if the driver truly needs no new code. A failure here is a real finding about the driver, not a test to weaken; debug it (superpowers:systematic-debugging) before touching non-test code.

- [ ] **Step 3: Make them pass**

Expected implementation delta: none (the operations exist from Task 1). Fix only genuine defects the tests expose.

- [ ] **Step 4: Run the package's integration tests**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationChange' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_integration_test.go
git commit -m "test(app): unblock/revive integration and 0444 resume regression (change 0450)"
```

---

### Task 3: Wiring — CLI subcommands, schema registry, shadow/schema-tag/asset enumerations

**Files:**
- Modify: `internal/cli/change.go`
- Modify: `internal/cli/install.go`
- Modify: `internal/app/schema_registry.go`
- Test: `internal/cli/change_test.go`, `internal/app/schema_tags_test.go`, `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: `app.ChangeUnblock` / `app.ChangeRevive` and their request types (Task 1); `changeSubcommand`, `decodeRequestFlag`, `setResult`, `EffectMetadataWrite` (existing CLI plumbing).
- Produces: catalog-visible operations `change unblock` / `change revive`. `TestAssetIndependentSetExact` (`internal/cli/root_test.go`) enforces the `assetIndependent` ↔ Cobra-tree correspondence both ways — it needs no edit, only the map entries.

- [ ] **Step 1: Extend the enumerating tests first**

`internal/cli/change_test.go`:
- Subcommand list (`for _, sub := range []string{"create", "groom", "block", "defer", "kill"}`): add `"unblock", "revive"`.
- Operation-id pairs table (`{"block", "change.block"}, {"defer", "change.defer"}`): add `{"unblock", "change.unblock"}, {"revive", "change.revive"}`.
- Catalog-key list (`"change block", "change defer", …`): add `"change unblock", "change revive"`.

`internal/app/schema_tags_test.go` — two new rows following the block row exactly:

```go
{"change.unblock", ChangeUnblockRequest{}, func() []StatusFinding {
	return ChangeUnblock(context.Background(), PlanningDeps{}, "", ChangeUnblockRequest{}).Findings
}},
{"change.revive", ChangeReviveRequest{}, func() []StatusFinding {
	return ChangeRevive(context.Background(), PlanningDeps{}, "", ChangeReviveRequest{}).Findings
}},
```

`internal/app/shadow_test.go` — two rows beside the block/defer ones:

```go
{"change.unblock", newChangeLifecycleResult(OperationChangeUnblock, ResultApplied, ChangeLifecycleResult{})},
{"change.revive", newChangeLifecycleResult(OperationChangeRevive, ResultApplied, ChangeLifecycleResult{})},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ ./internal/app/ -run 'TestChangeSubcommands|TestChange.*Operation|TestAssetIndependentSetExact|TestSchemaTags|TestShadow' -count=1`
(Adjust `-run` to the actual enclosing test names at the edited tables; run the two packages without `-run` if in doubt — they are fast.)
Expected: FAIL — unknown subcommands, unregistered schema ids, missing asset-independent entries.

- [ ] **Step 3: Wire the operations**

`internal/cli/change.go` — two subcommands cloned from `block`, registered in the `changeCmd.AddCommand(…)` list after `deferCmd`:

```go
unblock := changeSubcommand("change", "unblock",
	"Unblock a blocked change back to in-progress, clearing blocked_by, from a JSON request",
	func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
		var req app.ChangeUnblockRequest
		if err := decodeRequestFlag(c, &req); err != nil {
			return err
		}
		setResult(app.ChangeUnblock(c.Context(), deps, repoDir, req))
		return nil
	}, EffectMetadataWrite)

revive := changeSubcommand("change", "revive",
	"Revive a deferred change back to proposed, from a JSON request",
	func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
		var req app.ChangeReviveRequest
		if err := decodeRequestFlag(c, &req); err != nil {
			return err
		}
		setResult(app.ChangeRevive(c.Context(), deps, repoDir, req))
		return nil
	}, EffectMetadataWrite)
```

`internal/app/schema_registry.go` — two rows beside the block/defer rows, same comment style:

```go
{ID: "change.unblock", Request: ChangeUnblockRequest{}, Result: ChangeLifecycleResult{}}, // ChangeUnblock
{ID: "change.revive", Request: ChangeReviveRequest{}, Result: ChangeLifecycleResult{}},   // ChangeRevive
```

`internal/cli/install.go` — in `assetIndependent`, beside `"change block"` / `"change defer"`:

```go
"change unblock":          true,
"change revive":           true,
```

(match the file's existing alignment).

- [ ] **Step 4: Run both packages' unit tests**

Run: `go test ./internal/cli/ ./internal/app/ -count=1`
Expected: PASS, including `TestAssetIndependentSetExact` (both directions of the correspondence) and the schema round-trip tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/change.go internal/cli/install.go internal/cli/change_test.go internal/app/schema_registry.go internal/app/schema_tags_test.go internal/app/shadow_test.go
git commit -m "feat(cli): wire change unblock and change revive into the catalog (change 0450)"
```

---

### Task 4: Convention prose and embedded-asset regeneration

**Files:**
- Modify: `skills/docket-convention/SKILL.md`
- Regenerate: `internal/assets/embedded/` (via `go generate ./internal/assets/` — commit whatever it rewrites, including the manifest)

**Interfaces:**
- Consumes: nothing from earlier tasks (prose only).
- Produces: the shipped convention text agents follow; `TestEmbeddedMatchesAuthored` (`internal/assets/embedded_test.go`) reds on any authored-vs-embedded drift.

- [ ] **Step 1: Sweep for every hand-edit instruction**

Per the spec, grep the shipped skill/agent text and README before editing (never hand-list sites):

Run: `PAT='one-line frontmatter edit'; OUT=$(grep -rn -F -e "$PAT" skills/ agents/ cursor-rules/ README.md docs/ internal/assets/embedded/ 2>/dev/null); printf '%s\n' "$OUT"`

Expected hits: `skills/docket-convention/SKILL.md` (the Rules sentence), its mirror under `internal/assets/embedded/tree/…` (regenerated, never hand-edited), and point-in-time records under `docs/superpowers/` (specs/plans — historical, leave untouched). Also probe for other phrasings of the same instruction, e.g. `grep -rniE -e 'unblock|reviv' skills/ agents/ cursor-rules/ README.md | grep -viE 'change (unblock|revive)|docket-'` and read the survivors. Only maintained instructional text gets edited; anything unexpected gets fixed the same way as the Rules sentence.

- [ ] **Step 2: Edit the Rules sentence**

In `skills/docket-convention/SKILL.md`, in the `**Rules.**` paragraph (currently containing "…and revived to `proposed`; clearing a blocker or reviving is a one-line frontmatter edit, no move."), replace that clause so it names the operations:

```
`deferred` may be entered from `proposed` or `in-progress` (add `## Why deferred`) and revived to `proposed`; clear a blocker with `change.unblock` (`blocked` → `in-progress`, clearing `blocked_by`) and revive with `change.revive` (`deferred` → `proposed`) — typed operations, no file move.
```

Leave the lifecycle diagram's edges untouched (they already show `blocked ──clears──▶ in-progress` and revive → `proposed`).

- [ ] **Step 3: Regenerate the embedded bundle**

Run: `go generate ./internal/assets/`
Then: `git status --porcelain` — expect the embedded mirror of `SKILL.md` and the asset manifest changed; nothing else.

- [ ] **Step 4: Run the guards**

Run: `go test ./internal/assets/ -count=1`
Expected: PASS (`TestEmbeddedMatchesAuthored` proves authored and embedded agree). Also run `go test ./internal/repoguard/ -count=1` (prose/anchor guards over maintained text).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md internal/assets/embedded/
git commit -m "docs(convention): clearing a blocker / reviving are typed operations (change 0450)"
```

---

## Self-review notes

- Spec coverage: operations (Task 1), request shape + refusals + preservation (Task 1), applied/contended integration + receipt/commit-subject + 0444 resume regression (Task 2), all five wiring sites and enumeration tests (Task 3), documentation sweep + convention edit (Task 4). Board-row assertion rides the inline file-set tests in Task 1 (`BOARD.md` in the planned file set), matching how the existing block/defer tests pin it.
- The spec's "one applied unblock and one applied revive through the real engine": the existing `TestIntegrationChangeAuthoring*Applied*` siblings use the recording engine for receipt/result assertions, while the real-engine end-to-end coverage (commit subject on origin, record bytes, marker survival) lands in `TestIntegrationChangeRuntimeUnblockThenResumeHalted`, which drives block → unblock → resume through `setupHaltedFixture`'s real repositories. Together they cover the spec's intent in this suite's own idiom.
- Types: `ChangeUnblockRequest`/`ChangeReviveRequest` fields and tags are identical across Tasks 1–3.
- No placeholders: every step names its exact file, code, command, and expected outcome.
