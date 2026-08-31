<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0367 — Configurable rendered-board sections and sorting](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-31-0367-configurable-rendered-board-sections-and-sorting.md)**
<!-- docket:backlink:end -->

# Configurable rendered-board sections and sorting

**Change:** 0367 · **Type:** feat · **Priority:** medium · **Date:** 2026-08-29
**Depends on:** 0370 · **Related:** 0022, 0261, 0318, 0369, 0371, 0372, 0370 · **ADRs:** 0012, 0052

## Purpose and boundary

The inline `BOARD.md` surface currently projects lifecycle statuses directly: build-ready proposals
remain mixed with ideas that still need grooming, `implemented` and `stacked-merged` render as
separate sections, and active rows sort by ascending change ID. This makes the board reflect storage
states more closely than the way a human scans work.

This change makes the **rendered inline board only** a configurable human-facing projection. It does
not add lifecycle statuses, alter readiness, change autonomous selection, reorder the digest, or
change the GitHub mirror. The change depends on 0370 and targets only the final post-cutover Go
product; the Bash renderer is retired by the 0318 → 0369 → 0371 → 0372 → 0370 sequence and is not
extended here.

## Configuration contract

Add a supported, global-able `board` block to all ordinary Docket configuration layers: global
`config.yml`, committed `.docket.yml`, and repository-local `.docket.local.yml`.

```yaml
board:
  section_order: [in-progress, built, blocked, groomed, proposed, deferred]
  sorting:
    in-progress: {by: updated, direction: desc}
    built: {by: updated, direction: desc}
    blocked: {by: updated, direction: desc}
    groomed: {by: updated, direction: desc}
    proposed: {by: updated, direction: desc}
    deferred: {by: updated, direction: desc}
```

The six section tokens are a closed rendered-view vocabulary, deliberately separate from the
lifecycle-status vocabulary. `board.section_order` is a whole-list replacement: the highest valid
configured list wins. A valid list contains every token exactly once. A list with a missing,
unknown, or duplicate token is warned about and ignored as one value, allowing the next valid lower
layer or the built-in order to win. Docket never appends forgotten sections automatically.

Each `board.sorting.<section>.by` and `.direction` is an independent scalar leaf. Higher layers may
override one leaf while inheriting the other leaf and every sibling section. Allowed `by` values are
`id`, `updated`, and `created`; allowed directions are `asc` and `desc`. An invalid sort leaf is
warned about and ignored, so that leaf inherits from the next valid lower layer or the built-in
default. The built-in sort for all six sections is `updated desc`.

These settings are global-able because the requirement explicitly makes presentation personalizable
at every common config layer. They are not coordination-fenced: they affect a derived presentation,
not authoritative change state. Existing `board_surfaces` semantics remain separate and unchanged.

The typed Go config schema, resolver, inspection output, canonical example, and active configuration
documentation all expose the new leaves through the same single-owner registry and provenance model
as existing supported config. No renderer re-parses YAML.

## Rendered section classification

The renderer classifies every active change into exactly one displayed section, in this precedence
order:

1. **Blocked** — a lifecycle-`blocked` change, or an `implemented` change carrying the exact
   `## Finalize blocked` section. Moving the latter is presentational only; its stored status stays
   `implemented`.
2. **In progress** — remaining lifecycle-`in-progress` changes.
3. **Built** — remaining lifecycle-`implemented` changes plus lifecycle-`stacked-merged` changes.
4. **Groomed** — lifecycle-`proposed` changes with a non-empty `spec:` that satisfy every existing
   build-readiness conjunct, including dependencies and effective stack-base resolution.
5. **Proposed** — every other lifecycle-`proposed` change. This includes needs-brainstorm,
   auto-groom-blocked, dependency-waiting, unresolved-stack-base, spec-bearing but not-yet-ready,
   and `trivial: true` build-ready changes. Trivial work remains proposed because this board's
   `groomed` label specifically means build-ready **and spec-backed**.
6. **Deferred** — lifecycle-`deferred` changes.

Classification is a pure function over the already-loaded snapshot, branch facts used by stack
readiness, and exact marker-section presence. The existing domain readiness functions remain the
owners of readiness; the renderer composes them rather than restating their predicates.

The count summary uses these six rendered groups in configured order, followed by unchanged
terminal `done` and `killed` counts. Zero-count groups remain omitted from both the summary and body.

## Section tables

Existing single-shape tables remain recognizable:

- **In progress:** ID, title, priority, type, spec, branch.
- **Groomed:** ID, title, priority, type, spec.
- **Proposed:** ID, title, priority, type, readiness. A trivial build-ready row says
  `build-ready (trivial)` so its placement is self-explanatory.
- **Deferred:** ID, title, priority, type.

The two mixed-source sections use unified layouts:

- **Built:** ID, title, priority, type, PR, state. An ordinary implemented row says
  `awaiting merge`; a stacked-merged row says `merged into #NNNN`.
- **Blocked:** ID, title, priority, type, PR, reason. A lifecycle-blocked row renders its
  `blocked_by:` text and an empty PR when none exists; a finalize-blocked row renders its PR and
  `finalize blocked — needs you`.

Section order controls only these six active tables. The Mermaid graph keeps its deterministic
ID-based node and edge order and remains after the active tables.

## Row sorting

Sorting runs after classification, independently inside each displayed section.

- `id` compares numeric change IDs.
- `updated` compares the parsed `updated:` calendar date.
- `created` compares the parsed `created:` calendar date.
- `asc` or `desc` controls the primary key.
- Equal date values use numeric ID in the same direction as the primary sort: descending date ties
  use descending ID; ascending date ties use ascending ID.
- Missing, empty, or malformed date fields sort after all valid dates regardless of direction; ties
  within the unknown-date group use ID in the configured direction.

The comparator is total and deterministic. Filesystem walk order and stable-sort arrival order can
never decide the final row order.

## Archive footer

Archive remains a fixed footer outside `board.section_order` and `board.sorting`. Its existing
contract remains date descending, then numeric ID descending within each date. The recent-done
window, killed-row retention, older-done monthly collapse, labels, and displayed `YYYY-MM-DD` value
remain unchanged.

The implementation adds an explicit same-day regression fixture because that secondary ordering was
not obvious from ordinary fixtures. It does not add a merge timestamp field, inspect commit history,
or query GitHub.

## Projection isolation

Only the persisted inline Markdown projection changes.

- The machine-readable backlog digest keeps lifecycle-status counts, active change lines, readiness
  tokens, and the autonomous ready queue exactly as they are.
- Autonomous selection remains priority, creation age, then lowest ID.
- The GitHub Issues/Projects mirror keeps its lifecycle/readiness mapping and ordering.
- The Mermaid dependency graph keeps its existing deterministic ordering.
- Change frontmatter and terminal-closeout writes are unchanged.

This isolation is mechanically guarded: tests compare digest and selection results before and after
non-default board presentation settings, and no config leaf is passed into those projections.

## Go implementation shape

After 0370, the Go renderer is the sole inline-board authority. Extend its input with typed board
presentation options produced by the resolved config. Keep three responsibilities distinct:

1. config owns validation, defaults, layered leaf resolution, provenance, and diagnostics;
2. board classification maps domain changes to rendered section tokens without mutating them; and
3. row comparators order an already-classified section from typed sort settings.

Application paths that plan an inline-board mutation pass the same resolved presentation options to
the renderer. There is one renderer entry point and one option-building path; transaction callers do
not reconstruct defaults. The Bash renderer, Bash config resolver, and their tests receive no new
feature code.

## Failure behavior

Invalid board presentation values are non-fatal warnings with inheritance, as specified above. Once
resolution succeeds, rendering remains fail-closed on malformed authoritative change data under the
existing snapshot rules. A config warning must not suppress unrelated valid board leaves or disable
the inline surface.

All rendered Markdown continues to escape or preserve values according to the existing renderer
contract. This change introduces no shell evaluation, network call, Git write, or user-authored
template execution inside rendering.

## Verification

The build must cover:

- built-in defaults and the complete default order;
- each common config layer, whole-list replacement, and independent per-leaf inheritance;
- missing, duplicate, and unknown section-order tokens warning and falling back to a lower valid
  value;
- invalid sort field/direction warning and inheriting only the invalid leaf;
- all three sort fields in both directions, same-date ties, and unknown-date-last behavior;
- every classification bucket, including spec-backed readiness, trivial build-ready placement,
  unresolved dependencies and stack bases, stacked-merged rows, and finalize-blocked precedence;
- count-summary parity with rendered group membership;
- unified Built and Blocked table cells;
- configured section ordering with empty sections omitted;
- same-day archive rows remaining ID-descending and archive staying a fixed footer;
- byte-stable repeat rendering;
- unchanged digest, ready queue, Mermaid graph, and GitHub-mirror semantics; and
- config inspection/example/documentation drift guards.

Mutation tests remove or invert the spec-backed groomed conjunct, finalize-blocked precedence, one
section-order completeness check, one per-section sort leaf, the date tie-breaker, and the
projection-isolation boundary; each mutation must redden a focused guard.

## Out of scope

New lifecycle statuses; changing build-readiness; moving dependency-waiting or auto-groom-blocked
proposals into the Blocked section; moving a future `## Run halted` surface (change 0261 owns that
behavior); configurable Archive placement or sorting; merge timestamps; Git-history or GitHub
lookups during rendering; GitHub mirror presentation; digest or selection ordering; priority as a
board sort key; filtering or hiding sections; per-row custom layouts; and any extension of the Bash
renderer or Bash config stack.
