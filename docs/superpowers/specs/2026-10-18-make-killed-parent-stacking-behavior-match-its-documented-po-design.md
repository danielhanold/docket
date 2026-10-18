<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0565 — Refuse a stack-parent kill that would strand built children](../../changes/archive/2026-10-18-0565-make-killed-parent-stacking-behavior-match-its-documented-po.md)**
<!-- docket:backlink:end -->

# Refuse a stack-parent kill that would strand built children — design

Change #565, groomed 2026-10-18 with Daniel.

## Why

The stacked-changes reference (`skills/docket-convention/references/stacked-changes.md`, section *When a parent is killed*) says killing a stack parent flips every open descendant to `blocked` with `blocked_by: stack parent #A killed — re-scope, re-parent, or kill`. The code does not do this. `change.kill` calls `domain.Kill`, and `domain.KillStackParent`, which implements that blocking, is called from nowhere but its own tests. The same paragraph then says the flip is "a human-directed edit, never an automatic sweep", which contradicts the bullets above it.

Tracing the code showed that the documented policy would make things worse, and that the real gap is somewhere else.

**Blocking a descendant removes its exits.** A `blocked` proposed child can no longer be re-parented or un-stacked (`change.groom` takes `proposed` only) or killed (`domain.Kill` takes `proposed`/`in-progress` only). The only way out would be `change.unblock`, which moves a never-claimed change to `in-progress`. #563 just gave a proposed child a clean typed exit (`stacked_on: <new parent>` or `0`), and blocking would take it away.

**Today's board misleads.** A proposed child of a killed parent renders `⏳ waiting on #A — stack base not built`, which reads as if it will resolve by itself. Only `docket status --json` says `stack base unresolved (parent-killed)`.

**The real dead end is built descendants.** A parent can only be killed while it is `proposed` or `in-progress`. A `proposed` change normally has no branch, so nothing can be built on it. Change #568 (being built, and this change depends on it) closes the one normal path to a proposed change with a `branch:` (deferring an in-progress change, then reviving it), so after #568 only a hand-edited or legacy record can be in that state. An `in-progress` parent has its branch cut, and children may already be built on it. If such a parent has a child that is `implemented` (PR open against the parent's branch) or `stacked-merged` (merged into the parent's branch) when it is killed:

- the `implemented` child: finalize refuses to merge it (its effective base no longer resolves), the sweep skips it, `change.kill` refuses it (illegal source status), and `change.groom` won't take it. It sits as `implemented` forever unless someone hand-edits the record.
- the `stacked-merged` child: its code is on a branch that will never reach the integration branch. Stack close-out never carries it (`carryKilledAncestor`), and no operation can move or kill it. It is stuck permanently.

This has never happened here: the repo has had 4 stacked changes, and none had a killed parent. But nothing prevents it, and recovering means hand-editing a record.

## Decision

1. **Killing a change never touches its descendants' status.** The doc is rewritten to match the code. The unused blocking machinery is deleted.
2. **`change.kill` refuses to kill a change that has built stack descendants.** "Built" means a descendant whose status is `implemented` or `stacked-merged` and whose effective base the kill would turn into `parent-killed`. That is, every link between it and the change being killed is a live (non-final) change. The refusal names those descendants and the way forward: finish the change, or pause it with `change.block` and a reason (its branch stays alive for its children, and `change.unblock` resumes it). Deferring is no remedy: after #568, `change.defer` refuses an `in-progress` change. One refusal closes the only path into the stranded state. No exits get built for state that can no longer arise.
3. **The board says plainly when the parent was killed.** A proposed row whose effective base is `parent-killed` renders `stack parent #A killed — re-parent, un-stack, or kill`, not `⏳ waiting on #A — stack base not built`.

Under the new rule, a killed change's surviving descendants can only be `proposed`, `in-progress`, `blocked`, or `deferred`, and each has a typed exit:

| Descendant status | Exits |
|---|---|
| `proposed` | re-parent or un-stack through `change.groom` (`stacked_on: <id>` / `0`, while it has no `branch:`), or `change.kill` |
| `in-progress` | `change.kill` |
| `blocked` | `change.unblock`, then `change.kill` |
| `deferred` | `change.revive`, then the `proposed` exits |

## Goals

- `change.kill` on a change with an `implemented` or `stacked-merged` descendant reached through live links only (direct child or deeper) refuses, writes nothing, and the finding names every such descendant id and the remedy.
- `change.kill` on a change whose descendants are all `proposed`/`in-progress`/`blocked`/`deferred` (or final, or none) behaves exactly as today. Descendant records are untouched.
- The board's proposed-section readiness cell distinguishes a killed parent from a not-yet-built base.
- The stacked-changes reference, the convention's build-readiness sentence, and the glossary describe only the current behavior.
- `KillStackParent`, `StackParentKilledReason`, `StackKillResult`, `DescendantOutcome`, and their tests are removed, along with the comments on `blockedResult` and `killedResult` that mention `KillStackParent`. Both helpers stay: `Block` uses `blockedResult` and `Kill` uses `killedResult`.

## Non-goals

- Exits for `implemented` / `stacked-merged` children of an already-killed parent. Under the new refusal that state can't be created through docket; a pre-existing case (none exist) would be a hand repair.
- Widening `change.groom`, `change.kill`, or `change.unblock` to new source statuses.
- Changing how the effective base resolves, or the readiness evaluation order. A needs-grooming child of a killed parent still reads `needs-grooming`, because readiness checks design before the stack (`EvaluateReadiness`, rule 5), and grooming is where a human sees its `stacked_on:`.
- Board cells for non-proposed rows. A resumed `in-progress` child of a killed parent already stops on the unresolved base and surfaces through the existing halt path.
- New skill prose. The reconcile-kill and proposed-kill callers already treat a kill refusal as "aborts the kill and is surfaced" (`docket-implement-next/references/edge-paths.md` *Reconcile-kill*, `docket-new-change` *Proposed-kill sub-path*), so a reconcile pass that hits this refusal halts for a human under its existing posture.

## Anti-goals

- Never auto-block, auto-rebase, auto-re-parent, or auto-promote a descendant. Re-scoping off a killed parent stays a human decision.
- The refusal must not trigger on final descendants (`done`, `killed`), on anything below one, or on `in-progress` descendants. An `in-progress` child is killable on its own, so refusing on it would add friction with no stranded state to prevent.
- Never refuse in a way that can't be cleared. A built descendant below a `done` link already resolves to the integration branch (`ResolveEffectiveBase` rule 3), and one below a `killed` link is already `parent-killed`. In neither case does this kill change anything, and neither descendant can be moved, so counting it would make the parent permanently unkillable.

## Design

### Domain rule (owns legality)

The rule lives in `internal/domain`: the app layer's kill comment says "The domain owns legality" and it must stay true. Kill legality now needs the snapshot, not just the change. The builder picks the shape. One natural option reshapes the deleted `KillStackParent` slot into a snapshot-aware kill. It calls `Kill`, then walks the stack children from the killed change. A final child (`done`/`killed`) is skipped and its subtree is never entered (my call, from the anti-goal above). A non-final child is collected if its status is `StatusImplemented` or `StatusStackedMerged`, and its own children are walked either way. `StackDescendantsParentFirst` walks straight through final links, so the walk needs that pruning, whether through a new helper or a filtered `StackChildren` recursion. Any hit returns a `*PolicyFailure` with a stable reason, and its detail carries the descendant ids. **Builder's call:** the reason token (e.g. `built-stack-descendants`) and the `PolicyFailureKind`. `changeKillResultFromOutcome` maps every kill refusal to `invalid-state` and stays that way: this refusal is a state-shaped kill refusal like the rest, and no envelope result changes.

Order of checks: the source-status check (`illegal-source-status`) stays first, so killing an `implemented` change still reports what it reports today.

`changeKillOp.Plan` (`internal/app/change_kill.go`) calls the snapshot-aware domain function in place of `domain.Kill(c)`, against the attempt's snapshot, so the check runs inside the transaction's exact-revision attempt like every other legality gate.

### Refusal finding

The finding follows the house refusal pattern: the code is registered in `internal/app/finding_codes.go` (`TestNoInlineFindingCodeLiterals`), and the message says what was refused, why it matters, and what to do, in plain words. Message, in substance:

> change 0012 has built stacked changes (#0013 implemented, #0014 stacked-merged) whose code exists only on its branch; killing it would strand them with no way to finish or kill them. Finish 0012, or pause it with change block (its branch stays for those changes).

The check runs regardless of the parent's status. For a `proposed` parent it fires only on a hand-edited or legacy record that still carries a `branch:`. In that case `change.block` is not available (it takes `in-progress` only), so the message's remedy there is to leave the change as it is or defer it, which is still allowed from `proposed`. **Builder's call:** whether the message varies by the parent's status, as long as it never names a command that would refuse.

**Builder's call:** the exact wording, and whether it goes through `refuseLifecyclePolicy` (extended to carry a message) or a dedicated refusal helper. The finding must carry the code, every built descendant id, and the remedy.

**Terminal output.** `ChangeKillResult.HumanText` prints only `change kill: <result>` on a refusal today. #568 makes the four lifecycle commands (block, defer, unblock, revive) print each refusal finding's message, and gives kill's *applied* output a `branch-left-behind` warning line. Kill's *refused* output gets the same treatment here: a refused `docket change kill` prints each finding's message under the result line, using the same layout #568 settled on. Build on #568's code (`Detail["message"]` on the refusal, its `HumanText` layout) rather than inventing a second one.

### Board cell

`boardReadinessCell` (`internal/render/board.go`), in the `ReadyStackBaseUnresolved` arm: when `r.StackBase.Kind == domain.BaseParentKilled`, render `stack parent #%04d killed — re-parent, un-stack, or kill`, naming `r.StackBase.Cause` (the killed ancestor, which for a grandchild can be the grandparent rather than `stacked_on`). Every other unresolved kind keeps `⏳ waiting on #<stacked_on> — stack base not built`. The cell deliberately does **not** use the `— needs you` suffix: the glossary's *Readiness reason / "waiting on #N" / "needs you" cells* entry says a "needs you" cell always comes from a marker section, and this one comes from the parent's status (my call, to keep that rule true). Naming the three exits does the same job. Update `render/board.go`'s cell-vocabulary comment and `internal/render/testdata/board/PROVENANCE.md` with it.

`docket status` already reports `stack base unresolved (parent-killed)` for this row, so it needs no change.

### Docs (current behavior only)

- `skills/docket-convention/references/stacked-changes.md`:
  - Rewrite *When a parent is killed*: nothing is auto-rebased, auto-promoted, or blocked; a kill leaves descendants' status untouched; `change.kill` refuses while any descendant is `implemented` or `stacked-merged` (finish the parent, or pause it with `change.block`); the `parent-killed` effective base and the board's `stack parent #A killed — re-parent, un-stack, or kill` cell are how a surviving descendant is surfaced; include the per-status exits table above. Drop both `blocked_by:` bullets and the "human-directed edit, never an automatic sweep" sentence.
  - The *Readiness* lines ("the board reads *waiting on #A — stack base not built*") gain the killed-parent variant.
- `skills/docket-convention/SKILL.md` *Build-readiness & selection*: add a short clause for the killed-parent board text to the "the board reads …" sentence. This file is always loaded, so keep the addition to a clause.
- `docs/reference/glossary.md`: the *Stacked change / effective base* entry names both board readings, and the *Readiness reason / "waiting on #N" / "needs you" cells* entry lists the new cell if that entry enumerates the readiness cells. In the *Block / unblock, defer / revive, kill* entry, add one sentence: kill refuses while the change has built stacked descendants.
- Regenerate the embedded skill tree: `go generate ./internal/assets/`.

No change or PR citations in maintained docs. ADR citations are fine.

## Sequencing with #568

`depends_on: [568]`. Both changes edit `internal/domain/actions.go` and `internal/app/change_kill.go`. #568 also settles the refusal-message delivery and the kill `HumanText` layout that this change extends, and it removes the deferral path that this spec's remedy text relies on being gone. Build 565 after 568 lands, never alongside it.

## Testing

- Domain: table test of the kill legality over a snapshot. A parent with each descendant status (`proposed`, `in-progress`, `blocked`, `deferred`, `implemented`, `stacked-merged`, `done`, `killed`) is refused exactly for `implemented` and `stacked-merged`. A grandchild-only `implemented` descendant (direct child `proposed`) is refused. An `implemented` grandchild below a `done` child, or below a `killed` child, is **not** refused. Both an `in-progress` parent and a `proposed` parent that carries a `branch:` (a legacy or hand-edited record) are covered. A refusal names all built descendants, and its message never names `change.defer` for an `in-progress` parent. An `implemented` parent still gets `illegal-source-status`. Mutation-test it: drop the `stacked-merged` arm, limit the walk to direct children, or remove the final-link pruning, and watch a case go red.
- App: `change.kill` on a stacked parent with an `implemented` child refuses, writes nothing (no commit, both records byte-identical), and returns the registered code. Its `HumanText` shows the message. With only a `proposed` child, it applies and leaves the child record byte-identical.
- Render: a proposed, trivial or spec'd child of a killed parent renders `stack parent #NNNN killed — re-parent, un-stack, or kill`. A child whose killed ancestor is the grandparent names the grandparent. A child of an unbuilt-but-live parent still renders `⏳ waiting on #NNNN — stack base not built`.
- The whole suite at the build gate (`build.test_command`).

## Left to the builder

- The shape of the snapshot-aware kill function, its pruned walk, the refusal reason token, and its `PolicyFailureKind`.
- Finding wording, and the plumbing for the message (within the house pattern above).
- Whether to record an ADR for "a kill refuses on built descendants rather than blocking them". It reverses the policy the stacked-changes reference has stated since stacking shipped, so it is a reasonable candidate.
