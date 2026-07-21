---
slug: marker-scoped-guard-needs-a-population-floor
hook: "A marker-keyed guard validates only the markers it finds — separately assert that the marker EXISTS, sits where you meant, and covers the case you care about; \"at least one\" pins a population, not coverage."
topics: [testing, sentinels, guards]
changes: [108]
created: 2026-07-21
updated: 2026-07-21
promotion_state: candidate
promoted_to:
---

## Apply
When a guard's scope is chosen by a **marker** in the artifact — an opt-in comment, an annotation, a
`docket:`-style directive — the grammar you design will be about *malformed* markers, because that
is the failure you can see. The failures you cannot see are the marker being **absent**, the marker
being **attached to the wrong element**, and the marker being present but on an element for which
the check is **trivially satisfied**. All three read as green, and none of them is a syntax error.

Three separate assertions, none of which substitutes for another:

1. **Existence** — the marker is present at all. A guard whose scope is selected by a marker
   silently degrades to guarding *nothing* when the marker is deleted. Emit a per-element record
   (`seen <line> <token>`) **before any skip**, and assert an exact count of the elements the
   scanner reached — so deletion and displacement both redden.
2. **Attachment** — the marker binds to the element you meant. Position-sensitive attachment rules
   ("nearest preceding non-blank line") fail open under an edit that inserts a blank line or moves
   the marker one line up. Reconcile whole-file: every marker occurrence must be attached to a real
   element, or the orphan is a failure.
3. **Coverage** — the *specific* case is guarded. This is the one that looks done and is not.
   **"At least one element is marked" pins a population, never coverage**: the property migrates to
   whichever element satisfies it most cheaply. The closure is a **positive control** — mutate a
   throwaway copy of the artifact so the drift you care about is really present, and assert the
   guard *reports* it. That holds no matter which element carries the marker, which is exactly what
   an existence floor cannot promise.

The generalization past markers: whenever a guard's **scope is data** rather than code, the scope
selection is itself untested surface. Ask what happens when the selector matches nothing, matches
the wrong thing, and matches something for which the assertion is vacuous — then write the assert
that distinguishes those from success. See [[guards-are-code]] for the vacuity catalogue this
extends, and [[enumerated-floor]] for why the tempting closure (an explicit allowlist of marked
elements) trades one drift surface for another.

## War story
- 2026-07-21 (#108, PR #116) — The change guarding the README's config fences shipped its own
  fail-open twice, both caught by review with a fully green suite, both in the *guard* rather than
  the prose it guards.
  (1) **No population floor.** Deleting the `values` marker from `README.md` — or displacing it one
  non-blank line earlier — left the suite green **with `reclaim.lease_ttl` actually drifted**. The
  value assert was green for a reason other than the property it claimed; it would have read
  identically had the `values` machinery never been wired up at all. The design had reasoned
  carefully about the *typo* direction (a malformed token hard-fails, because a typo'd marker
  otherwise fails open and silent) and missed deletion and displacement completely — the failure
  modes that produce no token to validate.
  (2) **The first fix pinned *a* fence, not *the* fence.** It asserted "at least one fence is
  values-marked." Relocating the marker to fence 209 — which documents shipped defaults and so
  passes value equality trivially — absorbed the assert harmlessly, and the suite went green again
  with `lease_ttl` drifted 72 → 99. The floor was real and still bought nothing, because it
  constrained the population rather than the coverage.
  Closed by a layered set: a `seen <line> <token>` record emitted for every fence reached before
  any skip, an exact-count floor that all 9 were visited, an at-least-one-marked floor,
  whole-file reconciliation that every marker line attaches to a fence, and a **positive control**
  that mutates a `$tmp` copy and asserts the `reclaim:` drift *is* reported. The last one is what
  actually holds the property; the others narrow how it can be evaded. Each was mutation-tested,
  and the relocation scenario re-verified independently after the fix.
  Worth noting where the same error started: the **stub that proposed this change** enumerated the
  unguarded fences and was already wrong when filed — it omitted the `reclaim:` fence. The same
  blind spot appeared three times in one change's lifetime, at three altitudes (proposal, guard
  design, first fix), which is the tell that it is structural rather than careless.
