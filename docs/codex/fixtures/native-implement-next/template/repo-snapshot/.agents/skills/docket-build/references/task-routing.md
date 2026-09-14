# task-routing — the character→profile rubric

The shared classification rubric behind docket's profile-routed work. **Two consumers read this
file**, and it is written for both:

- **`docket-build`** routes each plan task to a profile agent (`## Routing` in its `SKILL.md`).
- **`docket-implement-next`** routes each review finding in its Step 6 fix loop
  (`references/fix-loop.md`).

Neither restates it. What is classified differs — a plan task, a review finding — but the question
is identical: *how much reasoning investment does this piece of work need, and what happens if it
is got wrong?* Consumer-specific rules (a plan's `**Build profile:**` override, the fix loop's
`premium` ceiling, escalation ladders) belong to each consumer, not here.

## The rubric

Classify with a deliberate asymmetry — `economy` must be *positively* established, named risk
selects upward, and uncertainty defaults to `standard`.

The `max`/`premium` boundary has an organizing principle, not just a list: **`max` is for mistakes
the correction machinery cannot walk back.** Destroyed data cannot be un-destroyed by a retry, and
a wrong architectural call shapes every task after it; a patch-correctable bug is caught at the
suite gate or in review. Resolve edge cases by applying that test, not by extending the lists
below.

- **`max`** — **unresolved architecture** or an **irreversible data change** (a destructive
  migration, a backfill, anything that cannot be rolled back). Nothing else classifies here.
  Irreversibility is the test: a reversible or purely additive migration is *not* `max` — it is
  `premium`, or `standard` if it carries no consequential risk at all.
- **`premium`** — authentication or security boundaries, concurrency or locking, release
  infrastructure, or any consequential risk **explicitly named in the plan or spec text**. That last
  door is honored, not inferred: never articulate a new risk on your own — your classification is
  this closed list, so uncertainty still sinks to `standard`.
- **`standard`** — everything remaining; the default and the uncertainty sink. Deliberately includes
  hard-but-safe work: difficulty known ahead of time is handled by the consumer's own override
  **where one exists** (docket-build's `**Build profile:**`; the fix loop has no override at all, so
  a known-hard finding simply routes here), and difficulty discovered while working is handled by
  the `standard -> premium` escalation.
- **`economy`** — *only when* the work is fully specified, follows an established pattern, carries no
  consequential risk, and requires **no cross-file reasoning** — either localized to a couple of
  implementation files (tests do not count against locality), or a mechanical, pattern-identical
  edit repeated across many files whose instances do not interact and where a missed instance fails
  loudly (a grep, a validator) rather than silently. All four conditions must hold; doubt about any
  one of them means `standard`.

`max` is rare by construction. Its doors are narrow and each consumer states its own: docket-build
admits the two-item rubric above, an explicit plan override, and a `premium` escalation; the fix
loop admits **none of them** — it never dispatches `max` at all.
