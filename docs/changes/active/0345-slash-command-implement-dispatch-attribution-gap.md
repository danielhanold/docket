---
id: 345
slug: slash-command-implement-dispatch-attribution-gap
title: "Slash-command implement dispatch isn't agent-owned — attribution gap forces human-in-the-loop"
status: proposed
priority: high
type: feat
created: 2026-08-25
updated: 2026-08-26
depends_on: []
related: []
discovered_from: [342]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

When a human launches an implement run with the `/docket-implement-next <id>` slash command, the
harness forks the subagent directly from that command. There is no assistant turn between the human
typing the command and the fork launching, so the parent session never gets to run the pre-dispatch
half of the run gate: it cannot re-sync, snapshot the in-progress before-set, or stamp a dispatch
epoch (`date -u +%s`) *before* launch.

The run gate already classifies this as **unattributed mode** — "a slash-command launch, a
notification-first session, or any dispatch you did not snapshot." In that mode the parent can only
**verify-and-report**: it may run `verify-run <id>` and relay the verdict, but it must NEVER
auto-re-dispatch, because a timestamp alone cannot prove an in-progress id is *this* run's dead claim
versus a concurrent live agent's claim (`claimed_at` re-stamps at every phase boundary).

The consequence is a reliability asymmetry the human feels directly: an implement run launched by
slash command that dies early or returns `run-incomplete` cannot be autonomously recovered — the
human must confirm each re-dispatch. A run the agent dispatches from a normal turn (able to capture
before-set + epoch first) recovers on its own. Observed live on change 342: the first dispatch died
on a 529 and left a claimed-but-unbuilt change that the agent could only surface, not re-drive.

The slash command is the *natural, ergonomic* way a human kicks off a run — pushing humans toward
prose invocation to regain attributability is a workaround, not a fix. The dispatch mechanism should
not silently downgrade the run gate.

## What changes

Make a slash-command-launched implement dispatch behave — for run-gate purposes — the same as an
agent-owned one, so the parent can attribute and (where safe) autonomously recover it. Candidate
directions to evaluate at brainstorm time (not yet decided):

- **Catch/intercept** the slash-command launch so an attribution anchor (before-set + dispatch
  epoch, or a dispatch nonce written into the claim) is captured at or before fork time.
- **Stamp a dispatch token** into the claim itself (e.g. a run/dispatch id the child writes on
  claim) so attribution no longer depends on a wall-clock epoch that re-stamps.
- **Document** the limitation crisply and, if no reliable capture exists, codify the
  verify-and-report-only posture as the intended contract for slash launches — with clear human
  guidance on when prose invocation is preferable.

## Out of scope

- Changing the run gate's core safety invariant (never re-dispatch onto a change a live agent may
  hold). Any fix must preserve it, not relax it.
- The 529/overload retry behavior itself — that's an orthogonal transient-failure concern.

## Open questions

- Can the harness expose a pre-fork hook (or a synchronous parent turn) for slash-command launches
  at all, or is the fork-from-command genuinely atomic with no interposition point?
- Is a claim-embedded dispatch nonce a cleaner attribution primitive than the before-set + epoch
  pair, and would it also improve attributability for the notification-first and detached cases?
- Does this belong in docket (the skill/agent contract) or in harness configuration, and how
  portable is any fix across the shipped harnesses?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
