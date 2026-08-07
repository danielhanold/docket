---
slug: yielded-worker-return-closes-every-door
hook: "A worker that backgrounds its work and yields returns its pre-yield text as if it were an outcome — and because the worker may still be running, every cheap recovery door is closed; halt and preserve the worktree."
topics: [subagents, process, worktrees]
changes: [231]
created: 2026-08-07
updated: 2026-08-07
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

## War story
- 2026-08-07 (#231, PR #170) — The change exists because a worker on #223 woke after being presumed
  dead and committed into a worktree its replacement was editing, duplicating an assert group. On
  #231's *own* branch, during the Step 6 fix loop, a fix worker backgrounded the full suite,
  yielded, and returned `Waiting for the suite to finish.` with no schema-valid outcome, no commit,
  and two files left staged in the shared worktree. The prohibition this branch was adding had
  already closed the discard-and-re-dispatch door, and every other door was closed independently.
  The run halted with the worktree intact; a human authorized discarding the abandoned edits before
  the resume. The rule's first real exercise was on the branch that wrote it.
