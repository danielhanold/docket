<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0231 — A presumed-dead build worker can wake and race its own replacement in one worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0231-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla.md)**
<!-- docket:backlink:end -->

# A presumed-dead build worker can wake and race its own replacement in one worktree — results

Change: #0231 · Branch: feat/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-plan.md · ADRs: none

## Verify (human)

- [ ] Decide whether the size-budget row raises are the right call, or whether the prose should have been compressed instead. Three rows moved in one change — `docket-build/SKILL.md` 320/2950 → 325/3000, `docket-build-task/SKILL.md` 125/1100 → 130/1150, `fix-loop.md` 180/1850 → 185/1900 — each with its rationale comment naming the `references/` home it was considered for. Only a human can judge whether three raises in one change is regrowth or honest accounting.
- [ ] Confirm the escalated-worker carve-out still reads correctly after the amend ban widened. `docket-build-task` § *Scope* now forbids amending **any** commit, and separately still permits an escalated worker to "revise or replace" the weaker worker's **uncommitted** changes. The guard `worker: the escalated-worker allowance for uncommitted work survives` pins that they did not collide, but whether the two read coherently to a worker is a prose judgment.

## Findings

**This run hit the exact failure mode this change exists to prevent.** During the Step 6 fix loop, the first dispatched fix worker backgrounded the full test suite, yielded to await a completion event that cannot reach a subagent, and returned the pre-yield text `Waiting for the suite to finish.` — no schema-valid outcome, no commit, two files left staged in the shared worktree. Every recovery door was already closed by contract: re-dispatch to repair its own return, discard-and-re-dispatch (the rule *this branch adds*), escalation (a malformed return is never a free escalation), and adopting a child's uncommitted files. The run halted with the worktree preserved, and a human authorized discarding the abandoned edits before the resume. The prohibition worked. It is worth recording that the rule's first real exercise was on its own branch.

**Two contract gaps that halt exposed, neither in this change's scope:**

- `references/fix-loop.md` names dispositions for Tier C dispatch unavailability, escalation exhaustion, and a red suite gate — but **not for a malformed fix-worker return**. The fix loop inherits `docket-build`'s halting rule by way of the `docket-build-task` contract, which is arguably enough, but it is not stated where a Step 6 controller reads.
- After such a halt the worktree holds a dead worker's uncommitted files, and **nothing sanctions a later run clearing them**. The resume here required explicit human authorization. That is probably correct — but it means a halted run cannot self-heal even when the abandoned work is provably worthless.

Both were deliberately not minted as stubs: they are contract-design questions about the very rules this change edits, so their boundaries are not settled enough to clear auto-capture's boundary gate. They belong in a human's hands.

**Review found six findings** (0 blocker, 2 important, 4 minor) — dispositions in the PR body. The one declined in-branch (finding 2, staging discipline) became change **#0238**. Notable: the reviewer caught that the new fix-loop guards had been added to the one test file that explicitly disclaims owning them, and change 0234 merged mid-run and made that finding stronger by expanding `tests/test_docket_review.sh`'s fix-loop guard section.

**Three plan deviations, all corrections to the plan's own commands rather than to its design:**

1. The plan's mutation blocks used `git checkout -- <file>` to restore after each probe, which restores **HEAD** — silently wiping the unstaged prose edit under test and leaving later mutations probing the pre-change file. Every task after the first staged its edit with `git add` before mutating. Worth carrying forward as a house rule.
2. `grep -c` is line-based, so a guarded phrase that wraps across a line break reports 0 both before and after a mutation — a mutation that never landed reads exactly like a robust guard. Counts were taken through a whitespace-flattened copy instead.
3. The plan estimated `docket-build-task/SKILL.md`'s line row could stay at 125 and only its word row move; the measured actual left 3 lines of headroom, which is the near-zero margin the budget block explicitly forbids, so both axes moved. Two of three rows landed on the "raise both" reading.

## Follow-ups

- **#0238** — "A build worker may stage paths its task never touched" (minted from review finding 2). The worker contract closes the *amend* half of the 0223 double-write and leaves the *staging* half open: nothing forbids `git add -A`, so a worker can sweep in dirty paths it never wrote. Needs its own design pass for the escalated-worker carve-out.
- The spec's assumption A9 declined a controller-side check that the branch tip is still the SHA it accepted before starting the next task, on the grounds that A1's prohibition makes the state unreachable. That reasoning held up under review and is left as the spec recorded it — a human wanting belt-and-braces would file it separately.
- The suite reports a standing advisory `OVER BUDGET:` list of roughly ten pre-existing heavy fixture-building files under parallel load. None is a file this change touched, and raising `tests/runtime-budgets.tsv` was out of scope throughout.
