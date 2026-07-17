---
slug: consolidation-flattens-caller-variance
hook: "Restatements across N callers are not pure duplication — diff them against each other before templating, or the shared source silently rewrites the callers that differed."
topics: [refactoring, docs, contracts]
changes: [85]
created: 2026-07-17
updated: 2026-07-17
promotion_state: retained
promoted_to:
---

## Apply
Collapsing prose that N callers each restated is the core move of every slimming round, and it rests
on an assumption worth testing before you make it: that the restatements were **duplication**. Often
one or two carried real per-caller variance — a different posture (must-land vs best-effort,
abort-and-report vs continue), a different gate, a different failure path — and applying one template
literally **rewrites those callers**. It lands looking like a docs edit while being a behavior change.

Before consolidating, diff the restatements **against each other**, not just against your template.
Where they genuinely differ, the shared source must **defer to the caller** ("steps 4–5 follow the
caller") rather than pick one posture and flatten the rest. Where the difference is real, keep both
sentences; the two lines you saved are not worth a silently inverted contract.

Two traps specific to this move:

- **The consolidation you are trusting may already have flattened something.** Check the existing
  shared source against its callers *before* extending it — a prior round's flattening reads as
  settled contract.
- **The sentinel net cannot see this.** Grep anchors pin phrases, not postures; no test fails when a
  best-effort caller starts reading as abort-and-report. This class needs a human diff read of the
  before/after per caller, which is exactly the review step a "purely mechanical" framing invites you
  to skip. See [[guards-are-code]] and [[test-premise-deleted-not-regated]].

## War story
- 2026-07-17 (#85, PR #95) — A behavior-neutral slimming round hit this twice on one file,
  `references/terminal-close-out.md`. (a) The brief's literal single abort-and-report template for
  step 5 would have made `docket-status`'s **merge sweep** read as abort-and-report — a real behavior
  change, caught only because a reviewer traced the sweep's call chain by hand and both posture
  sentences were kept instead. (b) Auditing that fix surfaced a **pre-existing** flattening from the
  earlier 0053 consolidation: step 5 lumped "the two kill callers" into abort-and-report, but
  `docket-implement-next`'s reconcile-kill runs its board pass best-effort per its own skill body —
  contradicting the file's own "Steps 4–5 follow the caller" rule, and shipped undetected because no
  test pinned the wording. The same round's other consolidation (six Board-pass litanies → one
  `--must-land` flag) was safe precisely because the variance moved into a *flag* the callers pass,
  not prose a template overwrites.
