---
slug: backstop-must-compute-not-reenumerate
hook: "A backstop that re-enumerates the causes it backs up is a fourth restatement wearing the word invariant — derive its predicate from the real consumer, and mutation-test its POPULATION, not only its suppression."
topics: [testing, guards, invariants]
changes: [104]
created: 2026-07-20
updated: 2026-07-20
promotion_state: candidate
promoted_to:
---

## Apply
A **backstop** exists to catch the cases the specific checks did not think of. That purpose is
destroyed the moment it is populated from a hand-written list of conditions, because the list an
author can write is exactly the list the specific checks already enumerate — the backstop then
reports nothing its siblings would not, while *reading* like a safety net. The word "invariant" in
the spec does not make it one.

**Derive the predicate from the consuming code's real behavior**, not from the causes you can name.
The question is never "which bad states can I list?" but "what does the consumer actually do, and
what does that imply must hold?" — count-vs-rendered-rows, bytes-in vs bytes-out, registered vs
reachable. A predicate derived that way catches states nobody enumerated, which is the entire
reason the backstop is in the file.

**Mutation-test it by deleting its POPULATION, not only its suppression.** A backstop has two
halves — the code that *creates* an entry and the code that *suppresses* an already-explained one —
and a suppression assert passes **vacuously** when the invariant never computes at all. "Yields
exactly one finding, so suppression works" is a self-cancelling pair, not a suppression decision:
if the populating block also sets the explained flag unconditionally, every entry it creates is
guaranteed suppressed and deleting the whole block leaves the suite green.

**Set the suppression marker only from the arms that genuinely explain the condition.** Marking
defensively from every arm of a sibling check is forward-defensive code that *causes* the failure it
defends against: an arm describing a state that does not trigger the invariant will silence a real
future detection. Suppression is a claim that *this* finding explains *that* drop — restrict it to
the arms where the claim is true.

See [[guards-are-code]] (mutation-testing generally) and [[sole-channel]] (what a backstop owes once
it is the last line).

## War story
- 2026-07-20 (#104, PR #113) — `board-row-dropped` was specified as a computed count-vs-rows
  invariant and implemented as two hand-written conditions — *the same two* `malformed-id` and
  `field-domain` already enumerate. Consequence: an `active/` file carrying a **terminal** status is
  counted in `render-board.sh`'s `total` but rendered in no section (`print_section` runs only for
  the five active statuses; the count line's `done|killed` arm reads the archive-only `ARC_COUNT`).
  `field-domain` was silent (`done` IS in the vocabulary), `malformed-id` was silent (the id is
  valid), nothing set `DROPPED` — the board rendered `**2 changes**` above a single row with every
  check green, the exact symptom the change existed to eliminate, on a state the toolchain documents
  as reachable (`sweep-failed <id> archive <reason>`). Fixed by deriving a `renders_row` predicate
  from the renderer's real bucketing. Recorded as **ADR-0050**.
- Same change, same check — the status path set `EXPLAINED` unconditionally inside the block that
  set `DROPPED`, so half the population was dead code: deleting it left the suite green, because the
  only assert over it was the self-cancelling "exactly ONE finding" pair. Separately, `EXPLAINED`
  was marked from all four `field-domain` arms, but a bad `priority` or a piped `title` does not
  drop a row — so a future drop path would have been silenced by an unrelated pipe in some change's
  title. Both found by the **whole-branch review, after five per-task reviews had passed with a
  green suite**.
