---
slug: yielded-worker-return-closes-every-door
hook: "A worker that backgrounds its work and yields returns its pre-yield text as if it were an outcome — and because the worker may still be running, every cheap recovery door is closed; halt and preserve the worktree."
topics: [subagents, process, worktrees]
changes: [231, 309]
created: 2026-08-07
updated: 2026-08-16
promotion_state: candidate
promoted_to:
---

## Apply
A dispatched worker that backgrounds a long step (a full suite, a build) and yields to await a
completion event never receives one — a subagent has no channel for a task notification. What the
controller gets back is the worker's **pre-yield prose** — "Waiting for the suite to finish." —
delivered through the same channel a finished worker uses. It is a malformed return, not a result,
and the controller must classify it as one before deciding anything.

The trap is what that classification rules out. The worker is **not known to be stopped**, so:

- **Re-dispatching it to repair its own return** races an agent that may still be writing.
- **Discarding its uncommitted files and re-dispatching fresh** is the double-write: the original
  wakes into a worktree a replacement is editing.
- **Escalating** is not free — a malformed return carries no evidence the task needed a stronger
  tier, so escalation would be spending budget on an unread signal.
- **Adopting the child's uncommitted files** is forbidden outright; they are another agent's work
  in an unknown state.

Every door being closed is the correct outcome, not a design gap: **halt, preserve the worktree
exactly as it stands, and report what was left staged.** Clearing a dead worker's uncommitted
edits takes a human's authorization, because only a human can establish the worker is actually
gone. Accept that a halted run cannot self-heal — that is the price of never racing.
Related: [[capability-absence-needs-a-failed-attempt]], which is the same never-yield rule reached
from the opposite direction (a yielded child reporting a capability gap it never probed).

**The pressure to break this rule peaks on the LAST task, and it arrives disguised as tidiness.**
When the yield happens on a final task whose files are already written and whose suite is already
green, adopting them costs one commit while closing the run costs a human round-trip — so the
forbidden move is also the cheap, plausible, apparently harmless one. It is still forbidden, and
green evidence is not the missing ingredient: the prohibition is about the *worker's* liveness,
which a passing suite says nothing about. A run that adopts here ships a branch whose final commit
no worker authored and no worker self-reviewed, and the only receipt for that gap is a prose note in
a results file that no gate reads.

## War story
- 2026-08-07 (#231, PR #170) — The change exists because a worker on #223 woke after being presumed
  dead and committed into a worktree its replacement was editing, duplicating an assert group. On
  #231's *own* branch, during the Step 6 fix loop, a fix worker backgrounded the full suite,
  yielded, and returned `Waiting for the suite to finish.` with no schema-valid outcome, no commit,
  and two files left staged in the shared worktree. The prohibition this branch was adding had
  already closed the discard-and-re-dispatch door, and every other door was closed independently.
  The run halted with the worktree intact; a human authorized discarding the abandoned edits before
  the resume. The rule's first real exercise was on the branch that wrote it.
- 2026-08-16 (#309, PR #211 — merged) — **The rule was crossed, on the last task, and the branch
  merged anyway.** Change 0309's final build task (Task 10 — the race-test shard) hit this exact
  shape: the dispatched worker backgrounded a full-suite run and yielded on it, unresumable. But
  unlike #231, the controller did **not** halt. It certified the suite green itself and committed
  the worker's files directly — the adoption this finding forbids outright — and recorded it as a
  one-line "process note for the maintainer" in the results file. Nothing downstream caught it:
  the merge gate validates the *branch*, not who authored the commits on it, so a clean rebase, a
  green suite, and a docs-only skip permit all passed exactly as they would have on a fully-worked
  branch. What this entry adds to the family is the failure mode of the rule rather than of the
  worker: the earlier entry establishes that every door is closed, and this one shows that when the
  yield lands on the *last* task, the closed doors stop feeling like a constraint and start feeling
  like pedantry — the work looks done, the tests are green, and halting reads as ceremony. That is
  precisely the moment the liveness question is unanswered. Also note the reporting asymmetry: the
  adoption was disclosed honestly and in the right place, and it still reached `done` unchallenged,
  because a prose note in `## Follow-ups` is not a gate. If this class is to be caught rather than
  merely confessed, the signal has to live somewhere a close-out actually reads.
