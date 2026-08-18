---
slug: groomed-root-cause-is-a-hypothesis
hook: "A groomed change's stated root cause is a hypothesis someone inferred from a symptom — verify it against the code at reconcile, because the fix was scoped to that cause and inherits its error."
topics: [reconcile, diagnostics, planning]
changes: [328]
created: 2026-08-18
updated: 2026-08-18
promotion_state: candidate
promoted_to:
---

## Apply
A change file's `## Why` reads like a finding, but its causal claim is usually an **inference made
from a symptom** — often at grooming time, by someone who saw a failure line and not the code that
produced it. The `## What changes` steps are then scoped to that cause. So a wrong cause does not
merely mislead: it **silently narrows the fix**, and the narrowed fix passes review, passes tests,
and leaves the original defect live.

Reconcile is where this gets caught, and it is nearly free there — you are already re-reading the
change against current code, and the causal claim is usually falsifiable in one or two file reads.
The check is concrete:

1. **Find the code path the stated cause requires.** If the claim is "X wrote a record", locate the
   writer and ask whether it could have run at that moment.
2. **Enumerate the other paths to the same symptom.** A symptom is an *observation*; the code
   usually offers several routes to it. Read the decision function end to end and count them.
3. **Re-scope the fix to the enumeration, not the hypothesis.** If the groomed step guards one of
   four paths, widen it — or record why the other three are impossible.

Then write the correction into the `## Reconcile log` rather than only fixing the plan, so the
change file stops teaching the wrong cause to the next reader ([[dormant-code-live-mid-branch]] is
the sibling for premises about *liveness*; this one is about premises about *causation*).

A corollary worth stating separately: when the diagnosis step then **fails to reproduce** the
trigger, say so in those words. A fix landed on an unreproduced trigger is *diagnostics* — it makes
the next occurrence legible — not a demonstrated repair, and the merge gate deserves to know which
one it is being handed.

## War story
- 2026-08-18 (#328, PR #219 — merged) — `TestRecoverMarksCleanlyAbandonedOwnedRun` was a
  load-sensitive flake failing with `Marked:0`. The groomed `## Why` stated the cause: "the owned
  run wrote its own durable terminal record before the test's recover logic ran", and step 2 of
  `## What changes` was scoped exactly to it — "verify no `terminal.json` exists in the run dir
  before calling `Recover`".

  Reading the code at reconcile disproved it in two hops. The test's child is the helper's `sleep`
  mode, which blocks for `time.Hour` with **default** signal disposition (`main_test.go`), so it
  never exits on its own; and the setup kills the group with **SIGKILL**, which is uncatchable, so
  the supervisor cannot write `terminal.json` or anything else. The stated cause was not merely
  unproven — it was unreachable.

  Reading `classifyRun` (`internal/process/recover.go`) end to end then showed **four** routes to
  `Marked:0`, not one: a durable terminal record; a held or unprovable live lock; a pre-existing
  stopped/abandoned marker; and `recoverGroupProbe` answering `probeLive`/`probeUnknown` — the last
  being the most plausible under full-suite load, where the recorded PGID can be recycled between
  the test's own `groupAlive` wait and recover's re-probe. **The groomed fix would have guarded one
  of four**, shipped green, and left the flake live — the expensive failure mode, because the next
  occurrence would arrive against a test that now *looks* hardened.

  The fix was re-scoped to assert every durable verdict absent *and* the group still `probeAbsent`,
  and the correction was written into the change's `## Reconcile log` so `## Why` stops teaching the
  wrong cause. Cost of the check: two file reads, inside a step that was re-reading the change
  anyway.

  The corollary fired in the same run: the stress diagnosis (8 copies × `-count=5`, escalated to 16
  copies × `-count=2` of the whole package — 32 executions under real self-contention) **never
  reproduced** the trigger. The change landed anyway, but as diagnostics — the precondition helper
  converts a future mystery `Marked:0` into a named setup failure — with the non-reproduction stated
  in the results file's `## Verify (human)` and in the PR body rather than implied away. Note what
  makes that honest framing load-bearing rather than decorative: the next occurrence now prints the
  disposition and reason, which is the exact input that was missing this time.
