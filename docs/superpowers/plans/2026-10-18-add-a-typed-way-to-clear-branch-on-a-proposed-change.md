<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0568 — Stop deferring a change that is already being built](../../changes/archive/2026-10-18-0568-add-a-typed-way-to-clear-branch-on-a-proposed-change.md)**
<!-- docket:backlink:end -->

# Stop deferring a change that is already being built — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`: one tier agent per task under the `docket-build-task` contract, then a single full-suite gate. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `change.defer` refuses an `in-progress` change with a dedicated, plainly worded refusal; the four lifecycle commands print refusal messages in terminal output; `change.kill` warns, by name, about the branch it leaves behind.

**Architecture:** The domain (`domain.Defer`) narrows its legal source to `proposed` and returns a new stable reason token for `in-progress`. The app layer turns that token into a message-bearing refusal through the existing `refuseLifecycle` helper (the `not-restackable` precedent: text in `Detail["message"]`, domain stays text-free). Kill records the pre-kill `branch:` in its commit receipt (the engine's sanctioned Plan→result channel) and the result builder appends a warning finding from it; no git probing anywhere.

**Tech Stack:** Go (`internal/domain`, `internal/app`), Markdown skills/docs, `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-18-add-a-typed-way-to-clear-branch-on-a-proposed-change-design.md` (on the `docket` metadata branch; read it beside this plan).

## Global Constraints

- Deferring a `proposed` change is unchanged. Every other defer refusal (deferred, blocked, implemented, stacked-merged, done, killed) keeps `illegal-source-status`.
- `not-restackable` and `not-retitleable` gates and their wording stay exactly as they are; only their explanatory code comments are reworded.
- `findingsToStatus` keeps clearing `Remedy`; no new impact/remedy pipeline for typed-operation refusals.
- JSON output is unchanged apart from the new codes and messages.
- Kill still clears `branch:` and `claimed_at:` from the record, never deletes a branch or worktree, never probes git, and never writes the branch name into the archived record.
- A kill of a record with no `branch:` prints exactly what it prints today.
- Docs describe current behavior only — no change or PR citations in docs/skills (ADR references are fine). Archived changes, plans, results, specs, and Accepted ADRs stay as written.
- Every finding code is minted from a `FindingCode` constant in `internal/app/finding_codes.go` and listed, sorted, in `AllFindingCodes` (`TestNoInlineFindingCodeLiterals`, `TestFindingCodeRegistryIntegrity`, `TestVocabularyConstCompleteness`).
- Cross-references in maintained source anchor on symbol names, never line numbers.
- Mutation-test each new assert (strip the guarded line, watch it redden, restore from a backup copy — not `git checkout --`), using `go test -count=1`.

## Review Focus

1. A hand-edited `branch:` containing `&`, `<`, or `>` (legal in git refs) on a killed record — expected: the kill still applies; `json.Marshal`'s HTML escaping would make the receipt non-canonical and fail the engine's `validateReceipt`. Task 3 marshals the kill receipt with HTML escaping disabled and pins it with a test.
2. A defer refusal reaching the terminal — expected: the person sees the block/kill instructions, not just `change.defer: invalid-state`. Task 2's `HumanText` test.
3. An `in-progress` defer must write nothing — expected: refused plan carries zero files. Task 2's matrix asserts `len(plan.Files) == 0`.
4. A kill of a never-claimed `proposed` change — expected: no `branch-left-behind` finding, HumanText byte-identical to today. Task 3 tests both.
5. The domain token and the app constant drifting apart (renaming one) — expected: the refusal silently loses its message and falls back to the generic policy path. Task 2 pins `string(FCNotDeferrable) == domain.ReasonNotDeferrable`.

---

### Task 1: Domain — `Defer` refuses an in-progress change (tier: economy)

**Files:**
- Modify: `internal/domain/actions.go` (`Defer` and its doc comment; new exported const)
- Test: `internal/domain/actions_test.go`

**Interfaces:**
- Produces: `const ReasonNotDeferrable = "not-deferrable"` (exported, `internal/domain/actions.go`). `Defer(c Change) (ActionResult, *PolicyFailure)` — unchanged signature; `in-progress` returns `newFailure(c, FailInvalidState, ReasonNotDeferrable, map[string]string{"action": "defer", "from": "in-progress"})`.

- [ ] **Step 1: Update the matrix and add the failing test.** In `matrixActions`, change the `Defer` row's `legal` to `map[Status]Status{StatusProposed: StatusDeferred}`. Add:

```go
// TestDeferRefusesInProgressAsNotDeferrable: a change being built cannot be
// deferred; every other non-proposed source keeps illegal-source-status.
func TestDeferRefusesInProgressAsNotDeferrable(t *testing.T) {
	in := actSpec{id: 7, slug: "a-slug", status: StatusInProgress, branch: "feat/a-slug",
		claimedAt: "2026-08-10T00:00:00Z", plan: "docs/plan.md", reconciled: true}.build()
	before := fingerprint(in)
	_, fail := Defer(in)
	if fingerprint(in) != before { t.Fatal("input mutated") }
	if fail == nil || fail.Kind != FailInvalidState || fail.Reason != ReasonNotDeferrable ||
		fail.State != StatusInProgress || fail.Change != 7 {
		t.Fatalf("failure = %+v, want invalid-state/%s from in-progress", fail, ReasonNotDeferrable)
	}
	for _, st := range []Status{StatusBlocked, StatusDeferred, StatusImplemented, StatusStackedMerged, StatusDone, StatusKilled} {
		if _, f := Defer(actSpec{id: 7, slug: "a-slug", status: st}.build()); f == nil || f.Reason != "illegal-source-status" {
			t.Errorf("defer from %q: failure = %+v, want illegal-source-status", st, f)
		}
	}
}
```

- [ ] **Step 2: Run** `go test -count=1 ./internal/domain/ -run 'TestDeferRefusesInProgressAsNotDeferrable|TestActionSourceStateMatrix'` — Expected: FAIL (undefined `ReasonNotDeferrable`; then the matrix's `Defer/in-progress` row succeeds when it must refuse).

- [ ] **Step 3: Implement.** Declare `ReasonNotDeferrable` near `StackParentKilledReason` with a one-line comment. In `Defer`, check `c.Status() == StatusInProgress` first and return the failure above; then `requireStatus(c, "defer", StatusProposed)`. Rewrite the doc comment: Defer parks a `proposed` change and touches status only; a change being built is refused with `ReasonNotDeferrable` because deferring would carry its branch, claim stamp, plan, and results onto a record that revive later returns to `proposed` — it is blocked (to pause, keeping its branch) or killed (to drop) instead.

- [ ] **Step 4: Run** `go test -count=1 ./internal/domain/` — Expected: PASS (includes `TestLifecycleActionCarriesDeliverable`, which defers `proposed` records).

- [ ] **Step 5: Commit**

```bash
git add internal/domain/actions.go internal/domain/actions_test.go
git commit -m "fix(domain): refuse deferring a change that is being built"
```

---

### Task 2: App — the `not-deferrable` refusal and lifecycle refusal text (tier: standard)

**Files:**
- Modify: `internal/app/finding_codes.go` (constant + `AllFindingCodes` entry)
- Modify: `internal/app/change_lifecycle.go` (`changeLifecycleOp.Plan`, new `notDeferrableMessage`, `ChangeLifecycleResult.HumanText`)
- Test: `internal/app/change_lifecycle_test.go`

**Interfaces:**
- Consumes: `domain.ReasonNotDeferrable` (Task 1).
- Produces: `FCNotDeferrable FindingCode = "not-deferrable"`; `func notDeferrableMessage(id int) string`; `ChangeLifecycleResult.HumanText()` prints, on any non-applied result, `"<operation>: <result>"` followed by one line per finding with a non-empty `Message`, each `"\n  " + Message`.

- [ ] **Step 1: Write the failing tests** in `change_lifecycle_test.go`:
  - `TestChangeDeferPlanSourceStatusMatrix`: turn the `{"in-progress", false}` row into a refused row whose expected code is `not-deferrable`; give rows a `code string` field (`illegal-source-status` for blocked/deferred, `not-deferrable` for in-progress) and add `{"implemented", true, "illegal-source-status"}`. For every refused row also assert `len(plan.Files) == 0`.
  - `TestChangeDeferInProgressRefusalMessage`: plan `baseDeferOp` over `lifecycleChange(3, "widget", "in-progress")`; find the `not-deferrable` finding; assert `Detail["message"]` contains `0003`, `being built`, `cannot be deferred`, `change.block`, `change.unblock`, `change.kill`, `branch`, `worktree`, and `delete them`; assert `len(Detail) == 1` (message only — no reason/from noise in the rendered text).
  - `TestNotDeferrableCodeMatchesDomainToken`: `string(FCNotDeferrable) == domain.ReasonNotDeferrable`, and `FCNotDeferrable` is a member of `AllFindingCodes` (copy the `registered` loop from `TestChangeGroomPlanNotRestackable`).
  - `TestChangeLifecycleResultHumanTextPrintsRefusalMessages`: for each of `OperationChangeBlock`, `OperationChangeDefer`, `OperationChangeUnblock`, `OperationChangeRevive`, build `newChangeLifecycleResult(op, ResultInvalidState, ChangeLifecycleResult{Findings: []StatusFinding{{Code: "x", Severity: "error", Message: "first"}, {Code: "y", Severity: "error", Message: "second"}}})` and assert `HumanText() == op + ": invalid-state\n  first\n  second"`; an applied result still renders `"change 0003 → deferred — <rev>"` exactly; a non-applied result with no findings renders exactly `op + ": " + result`.

- [ ] **Step 2: Run** `go test -count=1 ./internal/app/ -run 'TestChangeDefer|TestNotDeferrable|TestChangeLifecycleResultHumanText'` — Expected: FAIL (undefined `FCNotDeferrable`; in-progress defer plans files).

- [ ] **Step 3: Implement.**
  - `finding_codes.go`: add `FCNotDeferrable FindingCode = "not-deferrable"` beside `FCNotRestackable` with a comment (refuses `change.defer` on a change being built; mirrors `domain.ReasonNotDeferrable`); insert `FCNotDeferrable` into `AllFindingCodes` between `FCMissingRationale` and `FCNotFound` (sorted).
  - `changeLifecycleOp.Plan`: when `fail != nil && fail.Reason == domain.ReasonNotDeferrable`, `return refuseLifecycle(FCNotDeferrable, notDeferrableMessage(o.changeID))`; otherwise `refuseLifecyclePolicy(fail)` as today. Extend the legality-gate comment to say so.
  - `notDeferrableMessage(id)` returns exactly this copy (spec wording; `%04d` for the id):

    `change %04d is being built (in progress), and a change being built cannot be deferred. To pause it and keep its branch and work, block it with a reason (the change.block operation); change.unblock resumes it later. To drop it, kill it (the change.kill operation): the change is archived and its branch: cleared, but its git branch and worktree stay in place — delete them yourself if you don't need them. It can be captured again later.`
  - `HumanText`: keep the applied case; the default case writes the existing line then each non-empty finding message on its own `"\n  "`-prefixed line (`strings.Builder`).

- [ ] **Step 4: Run** `go test -count=1 ./internal/app/ -run 'TestChangeDefer|TestChangeBlock|TestChangeUnblock|TestChangeRevive|TestNotDeferrable|TestChangeLifecycle|TestNoInlineFindingCodeLiterals|TestFindingCodeRegistryIntegrity|TestVocabularyConstCompleteness'` — Expected: PASS. Mutation: delete the `ReasonNotDeferrable` branch in `Plan` → `TestChangeDeferInProgressRefusalMessage` reddens (code becomes `not-deferrable` via the generic path but `Detail` gains reason/from and loses the message); restore.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finding_codes.go internal/app/change_lifecycle.go internal/app/change_lifecycle_test.go
git commit -m "fix(lifecycle): explain the not-deferrable refusal and print refusal messages"
```

---

### Task 3: Kill names the branch it leaves behind (tier: standard)

**Files:**
- Modify: `internal/app/finding_codes.go`, `internal/app/change_kill.go`
- Test: `internal/app/change_kill_test.go`

**Interfaces:**
- Produces: `FCBranchLeftBehind FindingCode = "branch-left-behind"`; `changeKillReceipt` gains `Branch string \`json:"branch,omitempty"\`` between `ArchivePath` and `ID` (alphabetical keys stay canonical); `func marshalKillReceipt(r changeKillReceipt) ([]byte, error)` (HTML escaping off, trailing newline trimmed); `func branchLeftBehindFinding(id int, branch string) StatusFinding`.

- [ ] **Step 1: Write the failing tests** in `change_kill_test.go`:
  - `TestChangeKillPlanReceiptRecordsLeftBranch`: plan an `in-progress` record (`branch: feat/widget`) → `decodeChangeKillReceipt(plan.Receipt).Branch == "feat/widget"`; plan a `proposed` record → `Branch == ""` and the raw receipt bytes do not contain `"branch"`. Also keep asserting the archived record does not carry `feat/widget`.
  - `TestKillReceiptCanonicalWithHTMLCharacters`: `marshalKillReceipt` of a receipt with `Branch: "feat/a&b<c>"`; decode into `any`, re-encode with a `json.Encoder` with `SetEscapeHTML(false)`, trim the newline, and assert byte equality with the original (the engine's `validateReceipt` rule) and that the bytes contain `a&b<c>` literally.
  - `TestChangeKillResultWarnsBranchLeftBehind`: `changeKillResultFromOutcome(transaction.Result{Disposition: transaction.DispositionApplied, AppliedCommit: "cafe…40", Receipt: <receipt with Branch "feat/widget">}, nil)` → result applied; exactly one finding with code `branch-left-behind`, severity `warning`, `Entity == "change"`, `Identity == "0003"`, message containing `feat/widget`, `local and remote`, `worktree`, and `delete them by hand`; `HumanText()` equals the old applied line plus `"\n  " + message`.
  - `TestChangeKillResultNoBranchNoWarning`: same with no `Branch` → no `branch-left-behind` finding and `HumanText() == "change 0003 killed (archived " + killArchivePath(3, "widget") + ") — " + rev` exactly.

- [ ] **Step 2: Run** `go test -count=1 ./internal/app/ -run 'TestChangeKill|TestKillReceipt'` — Expected: FAIL (unknown field `Branch`, undefined helpers).

- [ ] **Step 3: Implement.**
  - Register `FCBranchLeftBehind` (comment: warning on an applied kill whose record carried `branch:`) and insert it in `AllFindingCodes` immediately before `FCBranchMalformed`.
  - `changeKillOp.Plan`: build the receipt with `Branch: c.Branch().Value` (the pre-kill record) via `marshalKillReceipt` instead of `json.Marshal`.
  - `changeKillResultFromOutcome`: on `ResultApplied`, when the decoded receipt's `Branch != ""`, append `branchLeftBehindFinding(rec.ID, rec.Branch)` after the engine's findings.
  - `branchLeftBehindFinding` message copy (exact): `change %04d's branch %s is left in place: the kill cleared it from the record, but its local and remote branch and any feature worktree, if they exist, are untouched. Look them over, then delete them by hand if you don't need them.` — `Code: string(FCBranchLeftBehind)`, `Severity: string(domain.SeverityWarning)`, `Entity: string(domain.EntityChange)`, `Identity: fmt.Sprintf("%04d", id)`.
  - `ChangeKillResult.HumanText`: the applied case appends `"\n  " + f.Message` for each finding whose code is `branch-left-behind` only (grandfathered engine findings stay out of the text, so a branchless kill prints exactly today's line). Refusal text unchanged.
  - Update the file-header comment (it still inspects no branch/worktree state; it now reports the recorded branch it clears) and the receipt comment (the branch rides the commit receipt, never the archived record).

- [ ] **Step 4: Run** `go test -count=1 ./internal/app/ -run 'TestChangeKill|TestKillReceipt|TestNoInlineFindingCodeLiterals|TestFindingCodeRegistryIntegrity|TestVocabularyConstCompleteness'` — Expected: PASS. Mutations: drop the warning append → `TestChangeKillResultWarnsBranchLeftBehind` reddens; swap `marshalKillReceipt` back to `json.Marshal` → `TestKillReceiptCanonicalWithHTMLCharacters` reddens; restore each.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finding_codes.go internal/app/change_kill.go internal/app/change_kill_test.go
git commit -m "fix(kill): warn which branch and worktree a kill leaves behind"
```

---

### Task 4: Docs and stale comments describe the narrowed lifecycle (tier: economy)

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (lifecycle diagram, *Rules* sentence)
- Modify: `internal/repoguard/budgets_test.go` (`docket-convention/SKILL.md` row)
- Modify: `docs/reference/glossary.md` ("Block / unblock, defer / revive, kill"), `docs/concepts/change-lifecycle.md` (off-ramps block)
- Modify (comments only): `internal/app/change_groom.go` (retitle-gate and re-stack-gate comments), `internal/app/change_groom_test.go` (`revivedReviseFiles` comment), `internal/app/change_lifecycle_test.go` (`deferredWithClaim` comment)
- Regenerate: `internal/assets/embedded/tree/...` via `go generate ./internal/assets/`

**Interfaces:** none (prose).

- [ ] **Step 1: Convention diagram.** Delete the three-line `deferred` box above `proposed ──claim──▶` and replace the bare `     │` spacer line just above `     └──── killed …` with `     ├──── deferred (shelved; revive → proposed)`, so `deferred` hangs off `proposed`, not `in-progress`.

- [ ] **Step 2: Convention *Rules* sentence.** Replace "`deferred` may be entered from `proposed` or `in-progress` (add `## Why deferred`) and revived to `proposed`;" with "`deferred` is entered only from `proposed` (add `## Why deferred`; a change being built is blocked or killed instead) and revived to `proposed`;".

- [ ] **Step 3: Measure and re-baseline.** `wc -lw skills/docket-convention/SKILL.md` must be at or under 363 lines / 7317 words (expected ≈361/7316). If over, trim within the edited text — never raise the ceiling. Lower the budget row to the measured values with a leading note `0568: deferred is entered only from proposed (363/7317 -> <l>/<w>);`.

- [ ] **Step 4: Glossary and concepts.** Glossary: "**defer** shelves a change with `## Why deferred`" → "**defer** shelves a proposed change with `## Why deferred` (a change being built is blocked or killed instead)"; "**kill** archives it as obsolete with `## Why killed`" → append "; its git branch and worktree are left for you to delete". Concepts off-ramp: "deferred  — consciously shelved; may be revived to proposed" → "deferred  — a proposed change consciously shelved; may be revived to proposed".

- [ ] **Step 5: Comments.** Reword so none describes deferral out of in-progress as a live path; gates' code and refusal strings untouched:
  - retitle gate: a proposed change carrying `branch:`/`plan:`/`results:` is a hand-edited or legacy record (no lifecycle transition leaves them on a proposed change, since a change being built cannot be deferred); keep the backlink/identity-check rationale.
  - re-stack gate: same framing for `branch:`; keep the "next claim reuses that branch / old base's commits" rationale.
  - `revivedReviseFiles`: a proposed change that still carries feature-branch artifacts (hand-edited or legacy).
  - `deferredWithClaim`: a legacy or hand-edited deferred record still carrying a branch and claim stamp; revive must leave them intact.
  Then whole-repo grep for leftovers: `grep -rnE "deferr?(al|ed)? (out of|from) (an )?in-progress|in-progress (claim|change)[^.]{0,40}defer" --exclude-dir=.git . | grep -v '^./docs/superpowers/\|^./docs/changes/\|^./docs/results/\|^./docs/adrs/\|embedded/'` — Expected: no output.

- [ ] **Step 6: Regenerate and verify.** `go generate ./internal/assets/` then `go test -count=1 ./internal/assets/ ./internal/repoguard/ ./internal/app/ -run 'TestSkillSizeBudgets|Embedded|Manifest|Prose|TestChangeGroom'` — Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md internal/assets/embedded internal/repoguard/budgets_test.go docs/reference/glossary.md docs/concepts/change-lifecycle.md internal/app/change_groom.go internal/app/change_groom_test.go internal/app/change_lifecycle_test.go
git commit -m "docs: deferred is entered only from proposed; kill leaves the branch for you"
```

---

### Task 5: Full-suite gate (run by docket-build)

- [ ] Run the configured `build.test_command` from the feature worktree: `go run ./cmd/docket development test`. Expected: green. Read the budget report: act on any `SERIAL CONFIRMED OVER BUDGET:` line and note any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line in the results.
