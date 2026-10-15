<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0550 — Let change.groom set a change's priority](../../changes/archive/2026-10-15-0550-let-change-groom-set-a-change-s-priority.md)**
<!-- docket:backlink:end -->

# Let change.groom set a change's priority — design

## Problem

A change's `priority:` is the first key `docket-implement-next` sorts build-ready changes by, and the board shows it. Only `change.create` can set it. After a change exists, no typed operation edits `priority:`, so re-prioritizing means hand-editing the record on the metadata branch. That leaves `BOARD.md` stale and is blocked for agents running in auto mode.

## Decision

Add an optional `priority` field to the `change.groom` request, modelled exactly on the existing `title` field (the retitle precedent).

### Scope: proposed changes only

`change.groom` already refuses any change that is not `proposed`, and this design keeps that rule. Priority only changes behaviour for proposed changes:

- **Proposed:** priority is the first selection key.
- **Implemented:** finalize uses priority only as a late tie-breaker, after mergeability, changed-file count and diff size.
- **Deferred:** revive the change to `proposed`, then re-prioritize it.
- **In-progress and blocked:** priority has no effect.

A dedicated `change.set-priority` command covering every non-final status was considered and rejected (YAGNI).

## Request

`ChangeGroomRequest` gains `Priority string` with JSON key `priority`, `omitempty`, tagged `docket:"enum=priorities"`. That is the same vocabulary `ChangeCreateRequest.Priority` uses, so the schema descriptor names the allowed values with no further work. Empty means unchanged.

## Validation (shape stage, before any write)

- A non-empty `priority` that `domain.ParsePriority` rejects is refused with the existing `unknown-priority` finding code, using the same message as `change.create`: `unknown priority %q; valid values are critical, high, medium, low`. The match is case-sensitive, as in `change.create`.
- The `abstain` outcome refuses any non-empty `priority`, the same way it refuses `title`. An autonomous abstain cannot rewrite the proposal or re-rank it. Reuse the abstain branch's existing refusal pattern; if it needs its own code, keep it distinct from `unknown-priority`.
- The `revise` outcome's minimum-input rule changes from "non-empty `spec_markdown`, or a replace/remove section edit, or a `title`" to "... or a `title`, or a `priority`". A priority-only revise is the main use case and must not be refused as `empty-revise`. Update the refusal message to name `priority` as well.

## Write

In the first patch pass of the groom transaction, next to the retitle write, a non-empty `priority` is written with `upsertField(&ps, doc1, "priority", document.String(priority))`. `upsertField` tolerates a record that lacks the key, the same way the title write does. `updated:` is bumped as it is for every groom. The candidate snapshot carries the new priority, so the inline board re-rendered in the same commit shows it.

Nothing else changes: groom state (`spec:`, `trivial:`, `auto_groomable:`), slug, record path, spec path and branch all stay put. Re-sending the current value is not an error; like re-sending the current title, it lands as an ordinary groom write.

## Docs

In `skills/docket-groom-next/SKILL.md`, next to the **Retitle** paragraph, add one short paragraph: exits 1, 2, 5 and 6 may also carry `priority` (critical/high/medium/low) to re-prioritize the change in the same transaction; a `priority` alone is a valid revise; abstain refuses it. Then run `go generate ./internal/assets/` so the embedded skill copy matches. Leave the `docket-auto-groom` instructions unchanged: autonomous grooming never re-prioritizes.

## Tests

Add these alongside the existing groom tests, following the retitle tests' structure:

1. The spec, trivial, revise and re-enable outcomes each land a new priority. The record's `priority:` and the rendered board row both show it.
2. A revise carrying only `priority` succeeds and does not change groom state.
3. A groom without `priority` leaves the existing value byte-identical.
4. An unknown value (for example `urgent`, or `High`) is refused with `unknown-priority`, and nothing is written.
5. Abstain with a `priority` is refused, and nothing is written.
6. Mutation check: removing the priority clause from the `empty-revise` rule makes test 2 fail; removing the `ParsePriority` check makes test 4 fail.

Run the whole suite at the build gate.

## Out of scope

- A `change.set-priority` command, or re-prioritizing non-proposed changes.
- Changing the priority values themselves or how selection ranks them.
- Bulk re-prioritization.
