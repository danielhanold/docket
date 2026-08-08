---
slug: marker-scoped-guard-needs-a-population-floor
hook: "A marker-keyed guard validates only the markers it finds — separately assert that the marker EXISTS, sits where you meant, and covers the case you care about; \"at least one\" pins a population, not coverage."
topics: [testing, sentinels, guards]
changes: [108, 120, 140, 145, 164, 250]
created: 2026-07-21
updated: 2026-08-08
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
- 2026-07-28 (#120, PR #130 — merged) — **Moving the marker's attachment rule moved the hole; only
  counting closed it.** The guard admitted an occurrence if its *line* carried a class marker, and
  these skill files run one paragraph per line — so a later edit could add a new violating
  occurrence into an already-marked paragraph and stay green (the reviewer reproduced it). The
  same-line design had itself been chosen to close a *different* fail-open: a "nearest preceding
  non-blank line" rule reads green the moment an edit inserts a blank line. Generalize past the
  incident: **a marker-scoped guard fails open wherever its unit of admission is coarser than its
  unit of violation**, and changing the unit relocates the gap rather than removing it. The fix was
  to require the marker count on a line to EQUAL the occurrence count, with a non-vacuous pair of
  fixtures (2-occurrence/1-marker rejected, 2-occurrence/2-marker accepted).
- 2026-07-28 (#145, PR #135 — merged) — Same shape, section scope. A guard pinning that
  `### Health checks` carries no restated check-id list is escaped by re-adding the list under a
  **new heading elsewhere in the file** — the non-vacuity anchor catches a *rename of* the guarded
  heading, not a fresh section beside it. Mutation-confirmed at the whole-branch review. The
  file-wide alternative was rejected on merit (it reddens `### Merge sweep`'s legitimate
  `publish-deferred` prose), so the limitation was shipped **named in the guard's own header
  comment** instead — the right move when the hole is a priced trade-off rather than a defect.
- 2026-07-29 (#164, PR #138 — merged) — Same shape, `sed` range scope. A test isolating the
  commented `agents:` region of `.docket.example.yml` selected it with
  `sed -n '/^# agents:$/,/<cursor model literal>/p'` — an **end address keyed on a value the same
  change was editing**. The build correctly moved the anchor in lockstep, so nothing was broken;
  the reviewer asked the question the build had not, and mutated the two apart. The slice grew from
  33 lines to 63 — running to EOF and swallowing the `runners:` block and surrounding prose — and
  the file still exited **green**, because every existing assert (exit 0, no unknown-harness
  warning, wrapper exists, status model) survives an over-wide slice. The plan had predicted "the
  failure is loud but its cause is not"; the failure was not loud at all. **A range end address is
  a scope marker with the same fail-open behavior as any other, and an over-wide slice is the
  vacuity case that looks most like coverage** — every assert still runs, just against more text
  than intended. Fixed by pinning the slice's last line, mutation-verified in both directions
  (unmutated 182 ok / 0 red; mutated 181 ok / 1 red, the red being exactly the new assert).
- 2026-08-07 (#250, PR #175 — merged) — Same shape, **consumption** scope. A new correspondence
  guard over two by-value duplicated constants (`ORPHAN_PR_IDLE_SECS` / `ABORTED_RUN_IDLE_SECS`,
  the cost ADR-0072 accepted) asserted exactly-one assignment line per file and that the two
  evaluate equal. Both asserts are satisfied by a state where the constants are assigned, agree,
  and **neither is read** — inlining a literal at the predicate site keeps the guard green while
  deleting the property it exists to protect. The whole-branch review caught it; the fix adds a
  per-file consumption floor (`>= 2` occurrences: the assignment plus at least one use).
  Generalize: **an existence-and-agreement guard over a constant pins the declaration, not the
  dependency** — pair it with a floor on the constant's *uses*, or the guarded code can stop
  depending on the guarded value without ever reddening.
- 2026-08-08 (#140, PR #183 — merged) — Same shape, **negated-assert** scope, and the cheapest
  instance of it yet. A test proving that `runner-dispatch.sh` normalizes docket's `inherit`
  sentinel away routed a dispatch through a throwaway probe adapter that captures its argv to a
  file, then asserted absence twice: `! grep -qxF -- "--model"` and `! grep -qxF -- "inherit"`.
  Both are satisfied by an **empty argv file** — if the probe never ran, was never dispatched to,
  or wrote nothing, the group reads exactly as it reads on success. The whole-branch review caught
  it and the mutation arm reproduced it live: with the probe helper neutered to write an empty
  argv file, both negated asserts **stayed green**. Generalize past markers and scopes to the
  assertion *form* itself: **a group of purely negative asserts carries no evidence that its
  subject exists at all**, so every absence-shaped assert needs a positive control beside it — one
  assert that reddens when the fixture produced nothing. The floor is cheap (assert some flag the
  adapter *does* receive) and it is the only thing separating "the sentinel was normalized away"
  from "nothing was ever observed."
