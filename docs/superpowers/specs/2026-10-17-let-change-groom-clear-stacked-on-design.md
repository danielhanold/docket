<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0563 — change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely](../../changes/archive/2026-10-18-0563-let-change-groom-clear-stacked-on.md)**
<!-- docket:backlink:end -->

# change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely

Change #563, groomed 2026-10-17. Folds in #564 (killed as consolidated into this change).

## Why

Three gaps in `change.groom` (`internal/app/change_groom.go`), all in its request validation and
its frontmatter patching:

1. **No way to clear `stacked_on`.** The request field is `*int`: omitted or `null` means
   "unchanged", any value sets it. Un-stacking a proposed change — typically one whose stack
   parent was killed, so its base resolves `parent-killed` and it can never build — needs a hand
   edit of the record. The results-artifact stacking refusal even says "ask a human to remove
   stacked_on".
2. **A relationship-only revise is refused.** The `empty-revise` check counts spec, sections,
   title, priority, deliverable, type, and branch_prefix, but none of `depends_on`, `related`,
   `discovered_from`, `adrs`, `stacked_on`. So `{outcome: revise, depends_on: [12]}` refuses,
   though the groom skill already describes revise as carrying "any relationship-field updates".
3. **Missing frontmatter keys break groom (was #564).** Two failure modes, both live:
   - The relationship fields (`depends_on`, `related`, `discovered_from`, `adrs`, `stacked_on`)
     and `trivial` are written with plain `PatchSet.SetField`, which fails with a
     missing-patch-target internal error when the record lacks the key. Ten active records lack
     `stacked_on:` (0007–0010, 0195, 0248, 0257, 0263, 0273, 0302) and four also lack
     `discovered_from:` (0007–0010), so grooming any of them with those fields fails today.
   - The fields written through `upsertField` (`auto_groomable`, `updated`, `title`,
     `priority`) share one `PatchSet`; when two are absent, both become `InsertField` at the
     same zero-width point and `Apply` fails with `overlapping-edit` (reproduced at the document
     layer). 0007–0010 lack `auto_groomable:`; a record also lacking `updated:` would trip it.

   #559 already hit the collision for `deliverable`/`branch_prefix` and solved it with one
   parse/apply cycle per insert (`upsertFieldBytes`, driven by `reqFieldEdits`).

## Design

### 1. `stacked_on: 0` clears

- On spec, trivial, revise, and re-enable: `stacked_on: 0` writes the valueless key
  (`stacked_on:`), the shape the template and a cleared `branch_prefix` use; `domain` stack
  resolution already reads an empty edge as unstacked. On a record without the key, the
  valueless key is inserted (see §3 — every groom-written field is upserted in template shape).
- A positive value sets it, as today. Omitted / `null` stays "unchanged" — deliberately **not**
  repurposed, since a caller that always sends `null` must not start un-stacking.
- A negative value refuses at the shape stage with `invalid-stacked_on` ("stacked_on must be a
  positive change id, or 0 to clear").
- Abstain keeps refusing any `stacked_on`, `0` included.
- The shape-stage results-artifact check (`deliverable: results-artifact` together with
  `stacked_on`) refuses only a **non-zero** `stacked_on`: `deliverable: results-artifact` plus
  `stacked_on: 0` is the legitimate conversion and must succeed. The resulting-state check
  (`groomStackingProblem`) is unchanged.
- `groomStackingProblem`'s remedies that say "ask a human to remove stacked_on" name the typed
  path instead: groom the change with `stacked_on: 0` (for the other-child case, note the child
  must still be `proposed` to be groomed).

### 2. No re-stacking once a branch exists (new refusal `not-restackable`)

A request whose `stacked_on` would **change** the record's value (set to a different parent, or
clear a present edge) refuses with a new finding code `not-restackable` when the change's
`branch:` is non-empty — mirroring the `not-retitleable` gate. Reason: a change that was claimed
and then deferred keeps a branch cut from the old base; if the next claim reuses it under a new
base, the PR targets the new base while carrying the old parent's commits (a killed parent's
work would reach the integration branch). The message tells the human to deal with the branch
first. `plan:` / `results:` alone do not block. A request that leaves the value unchanged (same
id, or `0` on an already-unstacked record) is not a change and passes. Register the code in
`finding_codes.go` like its siblings.

### 3. Every groom-written field tolerates a missing key, one insert per pass

Every frontmatter field groom writes is upserted: set when present, inserted in the template's
shape when absent (`discovered_from: []`, `stacked_on:` for a clear, etc. — an explicit `[]`
on a missing list key inserts the key with `[]`). No single `document.PatchSet` may carry more
than one `InsertField`. Extend the existing per-field cycle (`upsertFieldBytes`, as
`reqFieldEdits` does for #559's fields) to cover `auto_groomable`, `updated`, `title`,
`priority`, `trivial`, and the five relationship fields. Exactly where to draw the line between
the first `PatchSet` and the per-field cycle is the builder's call, as long as the invariant
holds. The `document` package's refusal of competing same-point inserts is intentional and
stays. `change.reconcile` has the same `SetField` pattern for relationships; leave it (see Out
of scope).

### 4. Relationship fields count toward a non-empty revise

Any of `depends_on`, `related`, `discovered_from`, `adrs` (non-nil, including an explicit `[]`)
or `stacked_on` (non-nil, including `0`) makes a revise non-empty. Update the `empty-revise`
message to list them.

### Docs

- `skills/docket-groom-next/SKILL.md`: `stacked_on: 0` clears it; any relationship field alone
  is a valid revise; `not-restackable` joins the surfaced refusals. Mirror into the embedded copy
  (`go generate ./internal/assets/`).
- `skills/docket-convention/references/stacked-changes.md`: one line under *When a parent is
  killed* — "while a descendant is still `proposed`, re-parent or un-stack it through
  `change.groom` (`stacked_on: <new parent>` or `stacked_on: 0`)". Do not otherwise edit that
  section: its mismatch with the code (descendants are never auto-blocked) is #565.

## Tests

- clear on a stacked record → valueless `stacked_on:`; board/artifacts re-render; change
  resolves to the integration branch.
- clear on a record without the key → applied, valueless key inserted.
- negative `stacked_on` → `invalid-stacked_on`; abstain with `stacked_on: 0` → refused.
- `not-restackable`: re-parent and clear each refuse when `branch:` is set; an unchanged value
  with `branch:` set passes; `plan:`-only passes.
- each relationship field alone passes the revise shape check (table test), and an all-absent
  revise still refuses `empty-revise`.
- `deliverable: results-artifact` + `stacked_on: 0` on a stacked change → applied.
- missing-key regressions using the real record shapes: a record shaped like 0007 (no
  `auto_groomable:`, `stacked_on:`, `discovered_from:`) takes a revise setting `stacked_on` and
  `discovered_from`, and takes abstain and re-enable; a record lacking both `updated:` and
  `auto_groomable:` takes abstain and re-enable.

## Out of scope

- `change.reconcile`'s identical `stacked_on` / missing-key gaps: un-stacking is a human scoping
  decision, and reconcile is the autonomous pass.
- `change.create`: still refuses `stacked_on: 0` (nothing to clear).
- Killed-parent descendant handling and its doc/code mismatch (#565).
- Cycle / killed-parent repair findings in `validation_text.go` keep their hand-edit remedies.
- Changing how stacks resolve their base, or the `document` package's overlap rule.
