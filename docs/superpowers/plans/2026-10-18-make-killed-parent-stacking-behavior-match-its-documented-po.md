<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0565 — Refuse a stack-parent kill that would strand built children](../../changes/archive/2026-10-18-0565-make-killed-parent-stacking-behavior-match-its-documented-po.md)**
<!-- docket:backlink:end -->

# Refuse a stack-parent kill that would strand built children — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `change.kill` refuses to kill a change that still has built stacked descendants, kills never touch descendants' status, the board names a killed stack parent plainly, and the docs describe only that behavior.

**Architecture:** A snapshot-aware kill in `internal/domain` (`KillInSnapshot`) wraps `Kill` and adds a pruned walk over stack children. It replaces the unused `KillStackParent` blocking machinery, which is deleted. `changeKillOp.Plan` calls it inside the transaction attempt and turns its refusal into a registered finding with a plain-text message, which `ChangeKillResult.HumanText` now prints the same way #568 made the lifecycle commands print theirs. The board's `ReadyStackBaseUnresolved` arm gets a killed-parent variant keyed on `r.StackBase.Kind`.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), markdown skill/docs files, the embedded skill tree (`go generate ./internal/assets/`).

**Spec:** `docs/superpowers/specs/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-design.md` (on the metadata branch; a copy rides the feature branch).

## Global Constraints

- The domain owns legality: the refusal rule lives in `internal/domain`, not `internal/app`.
- The source-status check (`illegal-source-status`) runs first, so killing an `implemented` change still reports what it reports today.
- "Built" descendant = status `implemented` or `stacked-merged`, reached from the killed change only through non-final (`!Status().Final()`) links. A final child (`done`/`killed`) is skipped and its subtree is never entered. `in-progress`, `proposed`, `blocked`, `deferred` descendants never trigger the refusal.
- A kill never changes any descendant record. Never auto-block, auto-rebase, auto-re-parent, or auto-promote.
- `changeKillResultFromOutcome` keeps mapping every kill refusal to `invalid-state`; no envelope result changes.
- Every finding code is a constant in `internal/app/finding_codes.go` and listed in `AllFindingCodes` (`TestNoInlineFindingCodeLiterals`).
- The refusal message names every built descendant id with its status, and its remedy never names a command that would refuse in that state: an `in-progress` parent's message names `change.block` (and `change.unblock`) and never `change.defer`; any other parent status (a `proposed` legacy record carrying `branch:`) names leaving it or `change.defer`, never `change.block`.
- Board text, verbatim: `stack parent #%04d killed — re-parent, un-stack, or kill` (no `⏳`, no `— needs you` suffix), naming `r.StackBase.Cause`. Every other unresolved kind keeps `⏳ waiting on #%04d — stack base not built` naming `c.StackedOn().Value`.
- Docs describe current behavior only; no change or PR citations in maintained docs (ADR citations are fine). Cross-references anchor on symbol names, never line numbers.
- Go tests that observe a mutation or re-verify run with `-count=1`. Mutation restores use a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --`.
- Whole-suite gate is `build.test_command` (`go run ./cmd/docket development test`), run by docket-build at the end.

## Review Focus

1. **Grandchild below a `done` or `killed` link.** A built grandchild under a final child must not refuse the kill, or the parent becomes permanently unkillable. Task 1 pins both cases.
2. **Cycle in `stacked_on`.** A cycle that loops back through the change being killed must end, not hang. Task 1 adds a cyclic fixture.
3. **Kill of an illegal-status change now prints its message.** Once refused kill output prints `Detail["message"]`, `illegal-source-status` for kill must read "cannot be killed", not "cannot be moved by kill". Task 2 adds `"kill"` to `illegalSourceVerb` and tests it.
4. **Legacy `proposed` parent with `branch:`.** The remedy must not name `change.block` (which refuses from `proposed`). Task 2 tests the message branch.
5. **Killed grandparent behind a branchless `stacked-merged` parent.** The board must name the grandparent (`StackBase.Cause`), not `stacked_on`. Task 3 tests it.

---

### Task 1: Snapshot-aware kill in the domain; delete `KillStackParent`

**Files:**
- Modify: `internal/domain/actions.go` (delete `StackParentKilledReason`, `DescendantOutcome`, `StackKillResult`, `KillStackParent`; add `ReasonBuiltStackDescendants`, `BuiltStackDescendants`, `KillInSnapshot`; reword the `blockedResult` and `killedResult` doc comments so neither mentions `KillStackParent`. Keep both helpers: `Block` uses `blockedResult`, `Kill` uses `killedResult`.)
- Test: `internal/domain/actions_test.go` (delete `TestKillStackParentBlocksLiveDescendants`, `TestKillStackParentLookupFailures`, `TestKillStackParentRejectsIllegalParentState`; repoint `TestActionsNeverMutateSnapshotChanges` at `KillInSnapshot`; add the new tests below)

**Interfaces:**
- Produces:
  - `const ReasonBuiltStackDescendants = "built-stack-descendants"`
  - `func BuiltStackDescendants(s Snapshot, id ChangeID) []ChangeID`: built descendants of `id`, ascending id order, empty/nil when none.
  - `func KillInSnapshot(s Snapshot, c Change) (ActionResult, *PolicyFailure)`: runs `Kill(c)` first and returns its failure unchanged. On a hit it returns `newFailure(c, FailBlocked, ReasonBuiltStackDescendants, map[string]string{"action": "kill", "descendants": "<ids comma-joined, decimal, ascending>"})`, for example `"13,14"`. Otherwise it returns `Kill`'s result.

- [ ] **Step 1: Write the failing tests** in `internal/domain/actions_test.go`, using `actSnapshot`/`actSpec` and a local `edge(id)` helper that returns `OptionalInt{State: FieldPresent, Value: id, Raw: strconv.Itoa(id)}`.

  - `TestKillInSnapshotRefusesOnlyBuiltDescendants`: a table over `allStatuses`. Parent 1 is `in-progress` with `branch: "feat/root"`, and child 2 is stacked on 1 with the row's status. Expect a refusal exactly for `StatusImplemented` and `StatusStackedMerged`, with `fail.Kind == FailBlocked`, `fail.Reason == ReasonBuiltStackDescendants`, and `fail.Detail["descendants"] == "2"`. Every other row returns no failure and `result.Change.Status() == StatusKilled`.
  - `TestKillInSnapshotRefusesBuiltGrandchild`: parent 1 `in-progress`, child 2 `proposed` on 1, grandchild 3 `implemented` on 2. Expect a refusal with `Detail["descendants"] == "3"`.
  - `TestKillInSnapshotPrunesFinalLinks`: two sub-cases. Child 2 `done` on 1 with grandchild 3 `implemented` on 2 must not refuse. Child 2 `killed` on 1 with grandchild 3 `stacked-merged` on 2 must not refuse.
  - `TestKillInSnapshotNamesEveryBuiltDescendant`: parent 1 `in-progress`. Children 4 `stacked-merged` and 2 `implemented` on 1, child 5 `proposed` on 1, and grandchild 6 `implemented` on 5. Expect `Detail["descendants"] == "2,4,6"` and `BuiltStackDescendants(s, 1)` deep-equal to `[]ChangeID{2, 4, 6}`.
  - `TestKillInSnapshotProposedParentWithBranch`: parent 1 `proposed` with `branch: "feat/legacy"` and child 2 `implemented` on 1. Expect a refusal with `fail.State == StatusProposed`.
  - `TestKillInSnapshotSourceStatusFirst`: parent 1 `implemented` with child 2 `implemented` on 1. Expect `fail.Reason == "illegal-source-status"`.
  - `TestKillInSnapshotTerminatesOnCycle`: change 1 `in-progress` stacked on 2, and 2 `proposed` stacked on 1. Expect no failure (no built descendants), and the call returns.
  - `TestKillInSnapshotLeavesDescendantsUntouched`: parent 1 `in-progress`, child 2 `proposed` on 1. After the call, each snapshot change's `fingerprint` is unchanged, and `result.Changed` names only the parent's own fields (`status`, `claimed_at`, `branch`).

- [ ] **Step 2: Run to verify failure**

  Run: `go test -count=1 ./internal/domain/ -run 'TestKillInSnapshot'`
  Expected: build failure, `undefined: KillInSnapshot`.

- [ ] **Step 3: Implement** `BuiltStackDescendants` and `KillInSnapshot` in `internal/domain/actions.go`, next to `Kill`, and delete the `KillStackParent` family. The walk is an iterative preorder over `StackChildren` with a visited set seeded with `id`. A child whose `Status().Final()` is true is skipped and not expanded. Any other child is collected when its status is `StatusImplemented` or `StatusStackedMerged`, and expanded either way. Sort the collected ids ascending. Do not reuse `StackDescendantsParentFirst`, because it walks straight through final links. Write a doc comment on `KillInSnapshot` that states the rule and why final links are pruned (their subtree already resolves to the integration branch or `parent-killed`, and the kill changes nothing for it). Reword the `blockedResult` and `killedResult` comments, for example "the shared block transition: blocked status plus the recorded reason" and "the shared kill transition".

- [ ] **Step 4: Run to verify pass**

  Run: `go test -count=1 ./internal/domain/`
  Expected: PASS. Also run `grep -rn -e KillStackParent -e StackParentKilledReason -e StackKillResult -e DescendantOutcome internal/ cmd/` and expect no output.

- [ ] **Step 5: Mutation-test** with a backup copy (`bak=$(mktemp "${TMPDIR:-/tmp}/actions.go.XXXXXX") && cp internal/domain/actions.go "$bak"`; restore with `mv -f "$bak" internal/domain/actions.go`). Before and after each mutation, confirm with `grep -c` that it landed. Run `go test -count=1 ./internal/domain/ -run TestKillInSnapshot` once per mutation:
  (a) drop the `StatusStackedMerged` arm, and the table row for `stacked-merged` goes red;
  (b) collect direct children only (no expansion), and `RefusesBuiltGrandchild` goes red;
  (c) remove the final-link prune, and `PrunesFinalLinks` goes red.
  Restore the file and re-run the domain package green.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/domain/actions.go internal/domain/actions_test.go
  git commit -m "feat(domain): refuse a kill that would strand built stacked descendants"
  ```

### Task 2: `change.kill` gates on the snapshot-aware kill and prints its refusal

**Files:**
- Modify: `internal/app/change_kill.go` (`changeKillOp.Plan`, `changeKillResultFromOutcome`, `ChangeKillResult.HumanText`, the file-header and `Plan` doc comments that name `domain.Kill`)
- Modify: `internal/app/change_lifecycle.go` (`illegalSourceVerb`: add `case "kill": return "killed"`)
- Modify: `internal/app/finding_codes.go` (add `FCBuiltStackDescendants FindingCode = "built-stack-descendants"` beside `FCNotDeferrable`, with a comment saying it mirrors `domain.ReasonBuiltStackDescendants`, the token Plan matches on; add it to `AllFindingCodes` in its sorted position)
- Test: `internal/app/change_kill_test.go`

**Interfaces:**
- Consumes: `domain.KillInSnapshot`, `domain.BuiltStackDescendants`, `domain.ReasonBuiltStackDescendants` (Task 1).
- Produces: `func builtStackDescendantsMessage(parent domain.Change, built []domain.Change) string` in `change_kill.go`.

- [ ] **Step 1: Write the failing tests** in `internal/app/change_kill_test.go`. Add a helper `stackedOn(rec string, parent int) string` that replaces `"stacked_on:\n"` with `"stacked_on: <parent>\n"` in a `lifecycleChange` record. Fixtures: parent `groomPath(3, "widget")`, child `groomPath(4, "child")`. If the loader rejects a bare `implemented` record, add the `pr:`/`plan:` lines the way `finalize_merge_test.go` does.

  - `TestChangeKillPlanRefusesBuiltStackedChild`: parent `in-progress`, child `implemented` stacked on 3. Expect `opRes.Refused`, `len(plan.Files) == 0`, and `hasDomainFindingCode(opRes.Findings, "built-stack-descendants")`. The finding's `Detail["message"]` contains `"#0004 implemented"`, `"change.block"`, and `"change.unblock"`, and does **not** contain `"change.defer"`.
  - `TestChangeKillPlanRefusesStackedMergedChild`: same setup with child `stacked-merged`. The message contains `"#0004 stacked-merged"`.
  - `TestChangeKillPlanLegacyProposedParentRemedy`: parent `proposed` with `branch: feat/widget` inserted, child `implemented` on 3. Refused. The message contains `"change.defer"` and does not contain `"change.block"`.
  - `TestChangeKillPlanLeavesProposedChildUntouched`: parent `in-progress`, child `proposed` on 3, inline board on. Not refused, and no planned `FileMutation` has `Path == groomPath(4, "child")`.
  - `TestChangeKillBuiltDescendantRefusalPrintsPlainText`: run the refused `opRes.Findings` from the first test through `changeKillResultFromOutcome(transaction.Result{Disposition: transaction.DispositionRefused, Findings: opRes.Findings}, nil)`. Expect `Result == ResultInvalidState`, and `HumanText()` split on `"\n"` is exactly 2 lines: `"change kill: invalid-state"`, then a line whose trimmed text begins `"change 0003 has built stacked changes"`. `HumanText` must not contain `"message="`.
  - `TestChangeKillIllegalSourceRefusalPrintsPlainText`: record 3 `blocked`. `HumanText()` contains `"change 0003 is blocked, so it cannot be killed."`.
  - Leave `TestChangeKillPlanSourceStatusMatrix` as it is: it must stay green.

- [ ] **Step 2: Run to verify failure**

  Run: `go test -count=1 ./internal/app/ -run 'TestChangeKill'`
  Expected: FAIL. The new plan tests see no refusal, and the print tests see a 1-line `HumanText`.

- [ ] **Step 3: Implement.**
  - In `Plan`, replace `domain.Kill(c)` with `domain.KillInSnapshot(snap, c)`. When `fail.Reason == domain.ReasonBuiltStackDescendants`, resolve each id from `domain.BuiltStackDescendants(snap, c.ID())` through `snap.Change` and return a refusal with code `FCBuiltStackDescendants`, `Detail{"message": builtStackDescendantsMessage(c, built), "descendants": fail.Detail["descendants"]}`, severity error, and entity change. `refuseLifecycle` takes only a message, so build the `OperationResult` directly in the same shape. Every other failure still goes to `refuseLifecyclePolicy(fail)`.
  - `builtStackDescendantsMessage` lists descendants as `#%04d <status>`, comma-separated. The in-progress text reads, in substance: `change 0003 has built stacked changes (#0004 implemented, #0005 stacked-merged) whose code exists only on its branch; killing it would strand them with no way to finish or kill them. Finish 0003, or pause it with a reason (the change.block operation): its branch stays for those changes, and change.unblock resumes it.` For any other parent status, the remedy sentence reads instead: `Leave 0003 as it is, or defer it (the change.defer operation): its branch stays for those changes.`
  - In `changeKillResultFromOutcome`, copy `lifecycleResultFromOutcome`'s loop that overwrites `out.Findings[i].Message` with `res.Findings[i].Detail["message"]` when it is non-empty.
  - Change `HumanText`'s default arm to `lifecycle`'s layout: `change kill: <result>`, then each finding with a non-empty `Message` on its own `"\n  "`-indented line. Update its doc comment.
  - Update the file-header comment ("The domain owns legality: domain.KillInSnapshot decides …, including refusing while built stacked descendants depend on the change's branch") and `Plan`'s comment to match.

- [ ] **Step 4: Run to verify pass**

  Run: `go test -count=1 ./internal/app/ -run 'TestChangeKill|TestChangeLifecycle|TestChangeBlock|TestChangeDefer|TestNoInlineFindingCodeLiterals|TestAllFindingCodes'`
  Expected: PASS. Then run `go test -count=1 ./internal/app/` and expect PASS.

- [ ] **Step 5: Mutation-test** with a backup copy. Remove the `Detail["message"]` overwrite loop in `changeKillResultFromOutcome`, and `TestChangeKillBuiltDescendantRefusalPrintsPlainText` must go red. Swap the remedy branch so the in-progress parent gets the defer text, and `TestChangeKillPlanRefusesBuiltStackedChild` must go red. Restore both and re-run green.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/app/change_kill.go internal/app/change_kill_test.go internal/app/change_lifecycle.go internal/app/finding_codes.go
  git commit -m "feat(kill): refuse killing a change whose built stacked changes need its branch"
  ```

### Task 3: Board names a killed stack parent

**Files:**
- Modify: `internal/render/board.go` (`boardReadinessCell`'s `ReadyStackBaseUnresolved` arm, and the cell-vocabulary comment that lists `stack-base-unresolved → "⏳ waiting on #<parent> — stack base not built"`)
- Modify: `internal/render/testdata/board/PROVENANCE.md` (the "deliberately does not cover — unit-tested directly" list gains `proposed \`stack parent #NNNN killed — re-parent, un-stack, or kill\``)
- Test: `internal/render/board_test.go`

**Interfaces:**
- Consumes: `domain.Readiness.StackBase` (`Kind`, `Cause`) and `domain.BaseParentKilled`, both of which already exist.

- [ ] **Step 1: Write the failing tests** next to `TestBoardReadinessStackBaseUnresolved`. Put the killed parent at `Location: domain.LocationArchive`, `Path: "docs/changes/archive/2026-10-01-0008-parent.md"`, `Status: domain.StatusKilled`.
  - `TestBoardReadinessKilledStackParent`: subtests `spec` (child 9 with `Spec` set) and `trivial` (child 9 with `Trivial: true`, no spec), each stacked on 8. Render with `boardFromFacts(t, domain.BranchFacts{}, …)`. The output contains `"stack parent #0008 killed — re-parent, un-stack, or kill"` and does **not** contain `"waiting on #0008"`.
  - `TestBoardReadinessKilledStackGrandparent`: killed 7 (archive), 8 `stacked-merged` stacked on 7 with no branch in the facts, and proposed spec'd child 9 stacked on 8. The output contains `"stack parent #0007 killed — re-parent, un-stack, or kill"`.
  - `TestBoardReadinessStackBaseUnresolved` stays as it is: a live unbuilt parent still renders `⏳ waiting on #0008 — stack base not built`.

- [ ] **Step 2: Run to verify failure**

  Run: `go test -count=1 ./internal/render/ -run 'TestBoardReadiness'`
  Expected: FAIL. The cell still reads `⏳ waiting on #0008 — stack base not built`.

- [ ] **Step 3: Implement.** In the `ReadyStackBaseUnresolved` arm, when `r.StackBase.Kind == domain.BaseParentKilled`, return `fmt.Sprintf("stack parent #%04d killed — re-parent, un-stack, or kill", int(r.StackBase.Cause))`. Otherwise keep the existing waiting cell. Add a `parent-killed` line to the vocabulary comment, and note that it names the killed ancestor (`StackBase.Cause`) and carries no "needs you" suffix because it is not driven by a marker section.

- [ ] **Step 4: Run to verify pass**

  Run: `go test -count=1 ./internal/render/`
  Expected: PASS. The golden `board.golden` is unchanged, because the corpus has no killed stack parent.

- [ ] **Step 5: Mutation-test.** Remove the `BaseParentKilled` branch from a backup copy, and both new tests must go red. Restore it.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/render/board.go internal/render/board_test.go internal/render/testdata/board/PROVENANCE.md
  git commit -m "feat(board): say plainly when a proposed change's stack parent was killed"
  ```

### Task 4: Docs describe only the current behavior

**Files:**
- Modify: `skills/docket-convention/references/stacked-changes.md` (the *When a parent is killed* section, and the readiness sentence that starts "A change whose base does not resolve is **not build-ready**")
- Modify: `skills/docket-convention/SKILL.md` (*Build-readiness & selection*: the "the board reads *waiting on #A — stack base not built*" sentence)
- Modify: `docs/reference/glossary.md` (*Stacked change / effective base*, *Readiness reason / "waiting on #N" / "needs you" cells*, *Block / unblock, defer / revive, kill*)
- Modify: `internal/assets/embedded/tree/**` (regenerated, not hand-edited)
- Modify: `internal/repoguard/budgets_test.go` (only if a ceiling is exceeded)
- Test: `internal/repoguard/prose_contracts_test.go` (`alignmentContracts`, which collapses whitespace)

- [ ] **Step 1: Write the failing guard.** Append rows to `alignmentContracts` under sentinel `change_0565_kill_refuses_built_descendants`:
  - `skills/docket-convention/references/stacked-changes.md`: present `"stack parent #A killed — re-parent, un-stack, or kill"` and `"`change.kill` refuses while the change has built stacked descendants"`. Absent `"re-scope, re-parent, or kill"`, `"human-directed edit"`, and `"flip to `blocked`"`.
  - `skills/docket-convention/SKILL.md`: present `"stack parent #A killed — re-parent, un-stack, or kill"`.
  - `docs/reference/glossary.md`: present `"stack parent #A killed — re-parent, un-stack, or kill"` and `"Kill refuses while the change has built stacked descendants"`.

  Run `go test -count=1 ./internal/repoguard/ -run TestAlignmentContracts`. Expected: FAIL on every row. The absent phrases are present today, which proves the negative asserts can redden.

- [ ] **Step 2: Rewrite *When a parent is killed*** in `stacked-changes.md`. Keep the opening rationale, with "nothing is auto-rebased, auto-promoted, or blocked", and state that a kill never changes a descendant's status. Add a paragraph beginning **`change.kill` refuses while the change has built stacked descendants**: an `implemented` or `stacked-merged` descendant reached only through live changes. Its code exists only on the parent's branch, so finish the parent, or pause it with `change.block` (its branch stays, and `change.unblock` resumes it). A descendant below a `done` or `killed` link does not count. Then say that a surviving descendant surfaces as `effective_base.kind` `parent-killed`, and that on the board a proposed one reads *stack parent #A killed — re-parent, un-stack, or kill*. Finish with the spec's exits table, verbatim:

  | Descendant status | Exits |
  |---|---|
  | `proposed` | re-parent or un-stack through `change.groom` (`stacked_on: <id>` / `0`, while it has no `branch:`), or `change.kill` |
  | `in-progress` | `change.kill` |
  | `blocked` | `change.unblock`, then `change.kill` |
  | `deferred` | `change.revive`, then the `proposed` exits |

  Delete both `blocked_by:` bullets and the "human-directed edit, never an automatic sweep" sentence. In the readiness sentence, add the killed variant: "the board reads *waiting on #A — stack base not built* (*stack parent #A killed — re-parent, un-stack, or kill* when the chain reaches a killed parent)".

- [ ] **Step 3: SKILL.md and glossary.** In SKILL.md's "the board reads …" sentence, add a short clause: `(*stack parent #A killed — re-parent, un-stack, or kill* when an ancestor was killed)`. Keep the `` `stack-base-unresolved` `` token, which another sentinel requires. In the glossary:
  - *Stacked change*: "On the board an unresolved base reads *waiting on #A — stack base not built*, or *stack parent #A killed — re-parent, un-stack, or kill* when an ancestor was killed."
  - *Readiness reason … cells*: add **stack parent #A killed — re-parent, un-stack, or kill** to the listed cells, outside the "needs you" group. Its "A 'needs you' cell always comes from a marker section" sentence stays true.
  - *Block / unblock, defer / revive, kill*: add "Kill refuses while the change has built stacked descendants (`implemented` or `stacked-merged` on its branch): finish it, or block it."

- [ ] **Step 4: Regenerate and check budgets.**

  Run: `go generate ./internal/assets/`, then `go test -count=1 ./internal/assets/ ./internal/repoguard/`.
  Expected: `TestEmbeddedMatchesAuthored` and `TestAlignmentContracts` PASS. If `TestSkillSizeBudgets` reports `stacked-changes.md` (214/2149) or `SKILL.md` (360/7317) over its ceiling, first tighten wording. Only then re-baseline the row to the exact measured `wc -lw` numbers, prefixing the row comment with `0565: a kill refuses on built stacked descendants; the killed-parent board text and exits table (old -> new);` in the existing house style.

- [ ] **Step 5: Commit**

  ```bash
  git add skills/docket-convention/references/stacked-changes.md skills/docket-convention/SKILL.md docs/reference/glossary.md internal/assets/embedded internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
  git commit -m "docs: a kill refuses on built stacked changes and never blocks descendants"
  ```

## Notes for the builder

- No new skill prose beyond the convention files above. The reconcile-kill and proposed-kill callers already surface a kill refusal.
- Whether to record an ADR for "a kill refuses on built descendants rather than blocking them" is left open by the spec. If one is recorded, it goes through the docket-adr workflow, outside these tasks.
