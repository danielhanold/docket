<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0563 — change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely](../../changes/active/0563-let-change-groom-clear-stacked-on.md)**
<!-- docket:backlink:end -->

# change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely — Implementation Plan

> **For agentic workers:** Executed by `docket-build`, task by task under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `change.groom` can clear `stacked_on` (`stacked_on: 0`), refuses re-stacking a change that already has a branch (`not-restackable`), accepts a revise that carries only relationship fields, and writes every frontmatter field it owns whether or not the record already has the key.

**Architecture:** All work is in `internal/app/change_groom.go`. The Plan closure's shared first `document.PatchSet` (where two absent keys collide as `overlapping-edit`) is replaced by one ordered list of field writes, each applied in its own parse/apply cycle through the existing `upsertFieldBytes` (`internal/app/change_claim.go`). The shape validator gains the `stacked_on` sign rules and the relationship-aware empty-revise rule. A new Plan gate, modeled on the `not-retitleable` gate, refuses changing `stacked_on` once `branch:` is set. Skill docs and the stacked-changes reference name the typed un-stack path.

**Tech Stack:** Go; packages `internal/app`, `internal/document`, `internal/domain`; Markdown skills mirrored into `internal/assets/embedded` by `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-17-let-change-groom-clear-stacked-on-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-17-let-change-groom-clear-stacked-on-design.md`).

## Global Constraints

- `stacked_on: 0` clears the key to the valueless form `stacked_on:` (`document.Null()`, the same shape a cleared `branch_prefix` writes). Omitted / `null` stays "unchanged" and is **not** repurposed.
- A negative `stacked_on` refuses at the shape stage with `invalid-stacked_on` and the message, verbatim: `stacked_on must be a positive change id, or 0 to clear`.
- Abstain keeps refusing any `stacked_on`, `0` included (existing message unchanged).
- The shape-stage results-artifact check refuses only a **non-zero** `stacked_on`; `deliverable: results-artifact` + `stacked_on: 0` must succeed. `groomStackingProblem`'s resulting-state logic is unchanged (only its remedy text changes).
- New finding code `not-restackable` (`FCNotRestackable`), registered in `internal/app/finding_codes.go` like `FCNotRetitleable`. It fires only when the request would **change** the record's `stacked_on` AND `branch:` is non-empty. `plan:` / `results:` alone never block it.
- No single `document.PatchSet` may carry more than one `InsertField`. The `document` package's refusal of same-point inserts stays as it is.
- Out of scope, do not touch: `change.reconcile` (`internal/app/change_reconcile.go`), `change.create` (still refuses `stacked_on: 0`), `validation_text.go` remedies, `domain.ResolveEffectiveBase`, the `document` overlap rule, and the *When a parent is killed* section beyond the one added line (#565 owns its mismatch).
- Commit style: conventional commits (`fix(groom): …`, `docs(skills): …`). This repository is shared visibility.
- Cross-references in code comments anchor on symbol names, never line numbers (AGENTS.md).
- Tests run without cache: always pass `-count=1`. The final whole-suite gate is `docket-build`'s, with the configured `build.test_command`.

## Review Focus

1. **A `stacked_on` value equal to the current one on a branched change** (re-sending what the record already says, or `0` on an unstacked record) must pass, not refuse `not-restackable`. Pinned in Task 3 (`unchanged parent with branch passes`, `clear of unstacked with branch passes`).
2. **An explicit empty list on a missing key** (`discovered_from: []` on a record without `discovered_from:`) must insert `discovered_from: []`, never skip it or write `null`. Pinned in Task 1 (`TestChangeGroomPlanMissingKeyExplicitEmptyListInserted`).
3. **Many absent keys in one request** (a spec outcome on a 0007-shaped record that also sets `depends_on`, `stacked_on`, `discovered_from`) must apply, every key written exactly once. Pinned in Task 1 (`TestChangeGroomPlanSpecOutcomeOnBareRecordInsertsEveryField`).
4. **Setting a first parent on an unstacked change that already has a branch** is a change of base too and must refuse `not-restackable`. Pinned in Task 3 (`first stack with branch refuses`).
5. **A `stacked_on`-only revise reaching the real entry point** must pass shape validation and reach the engine (not stop at `empty-revise`); a negative one must stop before the engine. Pinned in Task 2 (`TestChangeGroomNegativeStackedOnRefusedWithoutEngineCall`, relationship-only table).

---

## File map

- `internal/app/change_groom.go` — every code change (Tasks 1–4).
- `internal/app/finding_codes.go` — register `FCNotRestackable` (Task 3).
- `internal/app/change_groom_test.go` — all new and updated tests (Tasks 1–4).
- `skills/docket-groom-next/SKILL.md` + regenerated `internal/assets/embedded/tree/skills/docket-groom-next/SKILL.md` (Task 4).
- `skills/docket-convention/references/stacked-changes.md` + regenerated embedded copy (Task 4).

Existing test helpers to reuse (all in `change_groom_test.go`): `groomableChange`, `groomPath`, `stackedChange`, `fixtureChange`, `fieldOnlyReviseRequest`, `stubSectionReviseRequest`, `validGroomSpecRequest`, `abstainRequest`, `reEnableRequest`, `groomPlanFor`, `baseGroomOp`, `groomedRecordBytes`, `planPaths`, `hasFindingCode`, `intPtrTest`, `testClock` (clock date is `2026-08-16`), `recordingEngine`, `fakeChangeReader`, `mainModePin`, `newPlanningLoader`, `planningTestConfig`, `newFakeTree`.

---

### Task 1: Upsert every groom-written field in its own patch cycle; `stacked_on: 0` writes the valueless key

**Files:**
- Modify: `internal/app/change_groom.go` — the Plan closure's "First patch pass" block through the `reqFieldEdits` loop; `reqFieldEdits`
- Test: `internal/app/change_groom_test.go` (new section `// --- stacked_on clear and missing keys (change 0563) ---` at the end)

**Interfaces:**
- Consumes: `upsertFieldBytes(src []byte, name string, v document.Value) ([]byte, error)` (`change_claim.go`), `groomFieldEdit{name string; value document.Value}` (existing type).
- Produces: `func (o changeGroomOp) fieldEdits(specPath string) []groomFieldEdit` (replaces `reqFieldEdits`); `func stackedOnValue(id int) document.Value` (`document.Null()` for 0, `document.Int(int64(id))` otherwise). Task 3 does not depend on these.

- [ ] **Step 1: Write the failing tests**

Add two fixture helpers and the tests below. Plan-level tests call `groomPlanFor` directly, so they bypass the shape stage (Task 2 owns shape).

```go
// bareKeysChange is shaped like active record 0007: no auto_groomable:,
// stacked_on:, or discovered_from: key (it keeps the artifacts block, which
// groom's ReplaceBlock requires).
func bareKeysChange(id int, slug string) string {
	s := strings.Replace(groomableChange(id, slug), "stacked_on:\n", "", 1)
	return strings.Replace(s, "discovered_from: []\n", "", 1)
}

// noUpdatedChange lacks both updated: and auto_groomable:.
func noUpdatedChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "updated: 2026-08-02\n", "", 1)
}
```

Each test first asserts its fixture really lacks the keys (e.g. `strings.Contains(src, "stacked_on:")` is false) so a fixture drift cannot make it vacuous.

- `TestChangeGroomPlanClearStackedOn` — files: `stackedChange(2,"add-a-widget",3)`, `groomableChange(3,"base-thing")`, `"docs/changes/BOARD.md": "# Backlog\n\nold\n"`; op `baseGroomOp([]string{"inline"}, fieldOnlyReviseRequest(func(r *ChangeGroomRequest){ r.StackedOn = intPtrTest(0) }))`. Assert: not refused; record contains `"\nstacked_on:\n"` and not `"stacked_on: 3"`; `docs/changes/BOARD.md` is in `planPaths(plan)`; the record still has exactly one `docket:artifacts:start`. Then reload: put the groomed bytes back into `files`, `newPlanningLoader(planningTestConfig(nil)).Load(ctx, newFakeTree(files))`, look up change 2, and assert `domain.ResolveEffectiveBase(after.Snapshot, ch, domain.NewBranchFacts(map[string]bool{}))` equals `domain.EffectiveBase{Kind: domain.BaseResolved, Branch: after.Snapshot.Policy().IntegrationBranch}`.
- `TestChangeGroomPlanClearStackedOnInsertsMissingKey` — record `bareKeysChange(2,"add-a-widget")`, same request. Assert not refused (today: `Plan` returns a missing-patch-target error, so `groomPlanFor` fatals) and record contains `"\nstacked_on:\n"` exactly once (`strings.Count(rec, "stacked_on:") == 1`).
- `TestChangeGroomPlanMissingKeysReviseInsertsRelationships` — files: `bareKeysChange(2,…)`, `groomableChange(3,"base-thing")`, `"docs/changes/active/0001-first.md": fixtureChange(1,"first")`; request `fieldOnlyReviseRequest` with `StackedOn = intPtrTest(3)`, `DiscoveredFrom = []int{1}`. Assert record contains `"\nstacked_on: 3\n"` and `"\ndiscovered_from: [1]\n"`, each key counted once.
- `TestChangeGroomPlanMissingKeyExplicitEmptyListInserted` — `bareKeysChange`, request with `DiscoveredFrom = []int{}` only. Assert record contains `"\ndiscovered_from: []\n"`.
- `TestChangeGroomPlanMissingKeysAbstainAndReEnable` — table over records `{"0007-shaped": bareKeysChange, "no updated": noUpdatedChange}` × requests `{abstainRequest(), reEnableRequest()}`. Assert not refused, record contains `"auto_groomable: false"` (abstain) / `"auto_groomable: true"` (re-enable) and `"updated: '2026-08-16'"`. (The `noUpdated` cases fail today with `overlapping-edit`.)
- `TestChangeGroomPlanSpecOutcomeOnBareRecordInsertsEveryField` — files: `bareKeysChange(2,…)`, `groomableChange(3,"base-thing")`, `fixtureChange(1,"first")`; `validGroomSpecRequest()` plus `DependsOn = []int{1}`, `StackedOn = intPtrTest(3)`, `DiscoveredFrom = []int{1}`. Assert not refused and record contains `"spec: 'docs/superpowers/specs/2026-08-16-add-a-widget-design.md'"`, `"\ndepends_on: [1]\n"`, `"\nstacked_on: 3\n"`, `"\ndiscovered_from: [1]\n"`, with `strings.Count` of each key == 1.
- `TestChangeGroomPlanResultsArtifactWithStackClear` — files: `stackedChange(2,…,3)`, `groomableChange(3,"base-thing")`; request `fieldOnlyReviseRequest` with `Deliverable = "results-artifact"`, `StackedOn = intPtrTest(0)`. Assert not refused, record contains `"\ndeliverable: 'results-artifact'\n"` and `"\nstacked_on:\n"`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestChangeGroomPlan(ClearStackedOn|MissingKey|SpecOutcomeOnBareRecord|ResultsArtifactWithStackClear)' -count=1`
Expected: FAIL — clear writes `stacked_on: 0`; missing-key cases fatal with a missing-patch-target or `overlapping-edit` error from `Plan`.

- [ ] **Step 3: Implement**

In `change_groom.go`:
- Replace `reqFieldEdits` with `func (o changeGroomOp) fieldEdits(specPath string) []groomFieldEdit`, returning, in this order and only when the request/outcome asks: `spec` (`document.String(specPath)`, spec outcome), `trivial` (`document.Bool(true)`, trivial outcome), `auto_groomable` (`false` abstain / `true` re-enable), `updated` (always, clock date `2006-01-02`), `title`, `priority`, `depends_on`, `stacked_on` (`stackedOnValue(*o.req.StackedOn)`), `related`, `discovered_from`, `adrs` (via existing `intSeqValue`), then `type`, `deliverable`, `branch_prefix` (existing logic moved verbatim).
- In Plan, keep the `spec-path-taken` probe where it is (before any write), drop the shared `ps`/`doc1` first pass, and apply `fieldEdits(specPath)` to `edited` one entry at a time with `upsertFieldBytes`, wrapping errors as `"change groom: patching %s: %w"`. The rest of Plan (candidate, stacking check, artifact block pass) consumes the result exactly as it consumed `intermediate`.
- Update the comments that described the two-pass split (the "First patch pass" comment, the 0559 per-field comment, and the `upsertField`-specific comments) into one comment stating the invariant: every groom-written field is upserted, one insert per parse/apply cycle, because two inserts in one `PatchSet` collide at the pre-fence point.

- [ ] **Step 4: Run the new tests and the whole groom file**

Run: `go test ./internal/app/ -run 'TestChangeGroom' -count=1`
Expected: PASS (existing tests' expected bytes are unchanged: `SetField` on a present key writes the same bytes whether batched or not).

- [ ] **Step 5: Mutation check**

Temporarily route `updated` and `auto_groomable` back through one shared `PatchSet` (or make `stackedOnValue(0)` return `document.Int(0)`); confirm `TestChangeGroomPlanMissingKeysAbstainAndReEnable` (respectively `TestChangeGroomPlanClearStackedOn`) goes red with `-count=1`; restore from a backup copy of the file, not `git checkout`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_groom.go internal/app/change_groom_test.go
git commit -m "fix(groom): upsert every groom-written field one insert at a time; stacked_on 0 clears"
```

---

### Task 2: Shape rules — negative `stacked_on`, results-artifact with `0`, relationship-only revise

**Files:**
- Modify: `internal/app/change_groom.go` — `validateChangeGroomShape`, `hasEffectiveSectionEdit` doc comment
- Test: `internal/app/change_groom_test.go`

**Interfaces:**
- Produces: `func hasRelationshipEdit(req ChangeGroomRequest) bool` — true when any of `DependsOn`, `Related`, `DiscoveredFrom`, `ADRs` is non-nil (an explicit `[]` counts) or `StackedOn` is non-nil (`0` counts).

- [ ] **Step 1: Write / update the failing tests**

- In `TestChangeGroomReviseShapeValidation`, rename the case `"relationships-only revise refused"` to `"relationships-only revise passes"` and change its expected code from `"empty-revise"` to `""`.
- `TestChangeGroomEachRelationshipFieldAloneIsAValidRevise` — table, each run through `validateChangeGroomShape(fieldOnlyReviseRequest(mut))` expecting zero findings: `depends_on [1]`, `depends_on []`, `related [1]`, `discovered_from [1]`, `adrs [1]`, `stacked_on 3`, `stacked_on 0`. Then the all-absent `fieldOnlyReviseRequest(func(*ChangeGroomRequest){})` must carry `empty-revise` whose message contains each of `"depends_on"`, `"related"`, `"discovered_from"`, `"adrs"`, `"stacked_on"` (and still `"deliverable"`, `"type"`, `"branch_prefix"`, so `TestChangeGroomEachNewFieldAloneIsAValidRevise` stays green).
- `TestChangeGroomStackedOnShape`:
  - for each of `validGroomSpecRequest()`, a trivial request with a `## Why` replace rationale, `fieldOnlyReviseRequest(nil-op)`, `reEnableRequest()` with `StackedOn = intPtrTest(-1)`: findings contain `invalid-stacked_on` with message exactly `stacked_on must be a positive change id, or 0 to clear`;
  - `abstainRequest()` with `StackedOn = intPtrTest(0)`: contains `invalid-stacked_on`;
  - `fieldOnlyReviseRequest` with `Deliverable = "results-artifact"`, `StackedOn = intPtrTest(0)`: zero findings;
  - same with `StackedOn = intPtrTest(3)`: contains `results-artifact-not-stackable` (already pinned by `TestChangeGroomStackingRefusals` (a); keep that test untouched).
- `TestChangeGroomNegativeStackedOnRefusedWithoutEngineCall` — `ChangeGroom` with `fieldOnlyReviseRequest(StackedOn=-1)` returns `ResultInvalidInput`, `invalid-stacked_on`, and `len(engine.calls) == 0`; with `StackedOn = 0` it does **not** return `invalid-input` for `empty-revise` (assert `!hasFindingCode(res.Findings, "empty-revise")`).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestChangeGroom(ReviseShapeValidation|EachRelationshipFieldAlone|StackedOnShape|NegativeStackedOn)' -count=1`
Expected: FAIL — relationship-only revise refuses `empty-revise`; negative accepted; results-artifact + `0` refused.

- [ ] **Step 3: Implement in `validateChangeGroomShape`**

- Revise branch: add `&& !hasRelationshipEdit(req)` to the empty-revise condition; new message, verbatim: `the revise outcome requires a non-empty spec_markdown, at least one replace/remove section edit, a title, a priority, a deliverable, a type, a branch_prefix, or a relationship field (depends_on, related, discovered_from, adrs, stacked_on)`.
- In the existing `req.Outcome != GroomAbstain` block: add `if req.StackedOn != nil && *req.StackedOn < 0 { addShape(FCInvalidStackedOn, "stacked_on must be a positive change id, or 0 to clear") }`, and change the results-artifact condition to `req.StackedOn != nil && *req.StackedOn != 0`.
- Rewrite `hasEffectiveSectionEdit`'s last doc sentence (it claims relationship patches alone do not qualify) to point at `hasRelationshipEdit`.

- [ ] **Step 4: Run**

Run: `go test ./internal/app/ -run 'TestChangeGroom' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_groom.go internal/app/change_groom_test.go
git commit -m "fix(groom): relationship fields make a revise non-empty; refuse negative stacked_on"
```

---

### Task 3: `not-restackable` — no base change once a branch exists

**Files:**
- Modify: `internal/app/finding_codes.go` — const block next to `FCNotRetitleable`; `AllFindingCodes`
- Modify: `internal/app/change_groom.go` — Plan, immediately after the retitle gate
- Test: `internal/app/change_groom_test.go`

**Interfaces:**
- Produces: `FCNotRestackable FindingCode = "not-restackable"`; `func stackedOnChanges(cur domain.OptionalInt, want int) bool` — for `want == 0`: true when `cur.State` is neither `domain.FieldAbsent` nor `domain.FieldEmpty`; for `want > 0`: true unless `cur.State == domain.FieldPresent && cur.Value == want`.

- [ ] **Step 1: Write the failing test**

`TestChangeGroomPlanNotRestackable` — table. Base record: `groomableChange(2,"add-a-widget")` with `"status: proposed\n"` → `"status: proposed\nbranch: 'feat/add-a-widget'\n"` (as `revivedReviseFiles` does), optionally `stacked_on: 3`; files always include `groomableChange(3,"base-thing")` and `groomableChange(4,"other-base")`. Request: `fieldOnlyReviseRequest` with `StackedOn` set.

| case | record | stacked_on | want |
|---|---|---|---|
| re-parent with branch refuses | branch, stacked 3 | 4 | refused `not-restackable`, `len(plan.Files)==0` |
| clear with branch refuses | branch, stacked 3 | 0 | refused `not-restackable` |
| first stack with branch refuses | branch, unstacked | 3 | refused `not-restackable` |
| unchanged parent with branch passes | branch, stacked 3 | 3 | not refused |
| clear of unstacked with branch passes | branch, unstacked | 0 | not refused |
| plan-only re-parent passes | no branch, `plan: 'docs/superpowers/plans/2026-08-10-add-a-widget.md'` (plan file present in `files`), stacked 3 | 4 | not refused |

Also assert the refusal's `Detail["message"]` names the branch (`feat/add-a-widget`).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestChangeGroomPlanNotRestackable|TestFindingCode' -count=1`
Expected: FAIL — compile error on `FCNotRestackable` / refusal cases apply.

- [ ] **Step 3: Implement**

- `finding_codes.go`: add, after `FCNotRetitleable`, a doc comment ("refuses a change-groom stacked_on edit that would change the base of a change whose branch: is already cut (change 0563); mirrors not-retitleable") and the const. Insert `FCNotRestackable,` into `AllFindingCodes` between `FindingCode(ReasonImplementedNotPRDeliverable),` (`"not-pr-deliverable"`) and `FindingCode("not-results-artifact"),` — the list is hand-sorted and `TestFindingCodeRegistryIntegrity` checks it.
- Plan gate: `if o.req.StackedOn != nil && c.Branch().Value != "" && stackedOnChanges(c.StackedOn(), *o.req.StackedOn)` → `refuseGroom(string(FCNotRestackable), msg)` with message: `change %04d already has branch %q cut from its current base; changing stacked_on would leave that branch carrying the old base's commits — delete or rebuild the branch before re-stacking or un-stacking it` (id, branch). Comment states the reason (a deferred-then-revived claim reuses its branch; a new base would carry the old parent's commits into the PR) and that an unchanged value is not a change.

- [ ] **Step 4: Run**

Run: `go test ./internal/app/ -run 'TestChangeGroom|TestFindingCode|TestAllFindingCodes' -count=1`
Expected: PASS (includes the registry integrity and AST completeness guards in `finding_codes_test.go`).

- [ ] **Step 5: Commit**

```bash
git add internal/app/finding_codes.go internal/app/change_groom.go internal/app/change_groom_test.go
git commit -m "fix(groom): refuse not-restackable when a branched change's stacked_on would change"
```

---

### Task 4: Name the typed un-stack path in remedies and docs

**Files:**
- Modify: `internal/app/change_groom.go` — `groomStackingProblem` messages (first and third returns only)
- Modify: `skills/docket-groom-next/SKILL.md`, `skills/docket-convention/references/stacked-changes.md`
- Regenerate: `internal/assets/embedded/tree/skills/...` via `go generate ./internal/assets/`
- Test: `internal/app/change_groom_test.go`

- [ ] **Step 1: Write the failing test**

`TestChangeGroomStackingRemediesNameTypedPath` — reuse the `TestChangeGroomStackingRefusals` setups (b) "record already stacked" and (d) "active child stacks on it" with the `toResults` request. For each, the `results-artifact-not-stackable` finding's `Detail["message"]` contains `"stacked_on: 0"` and does not contain `"ask a human"`; for (d) it also contains `"0005"` and `"proposed"`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run TestChangeGroomStackingRemediesNameTypedPath -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement**

- `groomStackingProblem`, self-stacked message → `change %04d is stacked on %04d; a results-artifact change takes no part in stacking, so it cannot be stacked; keep it a pr change, or un-stack it by grooming it with stacked_on: 0, and order it with depends_on`.
- Other-child message → `change %04d stacks on change %04d; a results-artifact change takes no part in stacking, so keep it a pr change, or un-stack change %04d by grooming it with stacked_on: 0 (it must still be proposed to be groomed)`. The results-artifact-parent message is unchanged.
- `skills/docket-groom-next/SKILL.md`: after the **Deliverable, type, and branch prefix.** paragraph add a **Stacking.** paragraph: exits 1, 2, 5, 6 may carry `stacked_on` — a change id stacks or re-parents it, `0` clears it (un-stacks it; the usual fix when its stack parent was killed), omitted or `null` leaves it; any relationship field alone (`depends_on`, `related`, `discovered_from`, `adrs`, `stacked_on`) is a valid revise; a negative value refuses with `invalid-stacked_on`, and changing `stacked_on` on a change that already has a `branch:` refuses with `not-restackable` (deal with the branch first); each writes nothing — surface it. Keep it skill-generic (no change numbers; this body ships into other repos).
- `skills/docket-convention/references/stacked-changes.md`, *When a parent is killed*: add exactly one line after the `effective_base.kind` paragraph: `While a descendant is still \`proposed\`, re-parent or un-stack it through \`change.groom\` (\`stacked_on: <new parent>\` or \`stacked_on: 0\`).` Edit nothing else in that section.
- Run `go generate ./internal/assets/` to mirror both skill files.

- [ ] **Step 4: Run**

Run: `go test ./internal/app/ ./internal/assets/ -count=1`
Expected: PASS (the assets tests catch an unregenerated embedded copy; `git status` shows both embedded mirrors modified).

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_groom.go internal/app/change_groom_test.go skills/docket-groom-next/SKILL.md skills/docket-convention/references/stacked-changes.md internal/assets/embedded
git commit -m "docs(skills): groom names stacked_on 0 as the typed un-stack path"
```
