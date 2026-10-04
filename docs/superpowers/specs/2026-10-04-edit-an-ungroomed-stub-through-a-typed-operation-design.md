<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0509 — Edit an ungroomed stub through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0509-edit-an-ungroomed-stub-through-a-typed-operation.md)**
<!-- docket:backlink:end -->

# Edit an ungroomed stub through a typed operation — design

## Problem

A needs-grooming stub (`proposed`, no `spec:`, not `trivial: true`) cannot have its title, owned
proposal sections, or relationship fields edited through any typed operation without changing its
groom state. `change.groom` `outcome: revise` refuses it with `not-revisable`; `spec` and `trivial`
groom it; `re-enable` flips `auto_groomable: true` and refuses with `nothing-to-re-enable` when the
flag is already true and no abstain marker exists. The remaining route is a hand-edit in the
`.docket` tree, which bypasses the writer's quoting guarantee (ADR-0071) and leaves `BOARD.md`
stale.

## Investigation and prior decisions

Design baseline: main `5805127ab03a9413c7ced1c2e5a74080d60bafca`, inspected 2026-10-04.

- `internal/app/change_groom.go` `changeGroomOp.Plan` gates `revise` on
  `c.Status() == domain.StatusProposed && (c.Spec().Value != "" || c.Trivial())`. Change 0445's
  spec chose this as "the exact complement of the groom gate" because it was the simplest
  selector; it never decided stubs must be unrevisable — stubs were simply outside its target set.
- Every mechanical piece a stub edit needs already runs under `revise`: owned-section splicing
  (`render.ApplySectionEdits` over `render.ChangeOwnedHeadings`), retitle (change 0461, including
  the `not-retitleable` gate), the complete-desired relationship collections, the `## Artifacts`
  re-render, the inline board re-render, and the exact-blob CAS on the record.
- `revise` never writes `spec:` or `trivial:` (no `ps.SetField` for either under that outcome) and
  never writes `auto_groomable:`. A stub edited by `revise` therefore stays needs-grooming, keeps
  its effective auto-groomable value, and keeps its selection band.
- A `revise` carrying `spec_markdown` on a change with no linked spec already refuses
  `spec-not-linked` before any mutation is assembled.
- `render.ChangeOwnedHeadings` includes `## Auto-groom blocked`. `validateChangeGroomShape`
  refuses a section edit naming it only on `re-enable`. Today a `revise` could therefore replace or
  remove that section without touching `auto_groomable:`, desynchronizing the marker from the flag
  (the board's "auto-groom blocked — needs you" cell keys on the marker). Widening `revise` to stubs
  — the only changes that normally carry the marker — makes that gap reachable in practice.

## Design

Widen the existing `revise` outcome; add no new outcome, operation, or request field.

### 1. Gate

In `changeGroomOp.Plan`, the `revise` gate becomes: refuse `not-revisable` unless
`c.Status() == domain.StatusProposed`. The refusal message names the status only ("change NNNN is
not a proposed change (status %q)"). The `spec` / `trivial` / `abstain` / `re-enable` gate is
unchanged, so a needs-design `proposed` change now satisfies both gates — the "exact complement"
property is retired deliberately, and the doc comment above the `GroomRevise` constant and the
gate comment are updated to say `revise` applies to any `proposed` change.

On a stub, `revise` accepts `sections`, `title`, and the relationship fields (`depends_on`,
`related`, `discovered_from`, `adrs`, `stacked_on`). `spec_markdown` still refuses
`spec-not-linked`; its message is reworded to cover both a trivial change and a needs-grooming
stub ("change NNNN has no linked spec to revise"). The `empty-revise` shape rule is unchanged.

`revise` still never writes `spec:`, `trivial:`, or `auto_groomable:`.

### 2. Auto-groom-blocked guard

`validateChangeGroomShape` refuses a `revise` section edit whose heading is
`## Auto-groom blocked` with `invalid-section-heading`, mirroring the existing `re-enable` check.
This applies to every `revise` target, groomed or not. After the change, only `abstain` (append)
and `re-enable` (remove) write that section, so the marker and `auto_groomable:` move together.
`spec` and `trivial` are left as they are (out of scope).

### 3. Skill and doc wording

Edit the maintained sources under `internal/assets/embedded/tree/skills/`:

- `docket-groom-next/SKILL.md`: the explicit-id path gains one branch — when the human asks to
  *edit* (not groom) a needs-grooming stub, apply `change.groom` `outcome: revise` with the
  requested owned-section / title / relationship edits and no brainstorm; the stub stays
  needs-grooming. The Step 4 revise exit drops "already groomed" from its precondition, names
  `spec-not-linked` as the refusal for `spec_markdown` on a stub, and replaces "The change stays
  build-ready" with "The change keeps its groom state." The contended-retry sentence's "stop only
  if the change is no longer an already-groomed `proposed` change" becomes "no longer `proposed`."
- `docket-new-change/SKILL.md`: its pointer to `revise` for adjusting a just-landed spec stays; no
  wording there claims stubs are unrevisable unless the trace finds one.
- `docs/reference/glossary.md`: the `revise` entry describes it as editing any `proposed` change
  without changing its groom state.

Run the repo's skill-sync/embedded-tree checks the suite already enforces; regenerate any derived
copies through the existing generator, never by hand.

### 4. Tests

In `internal/app/change_groom_test.go`:

- Flip the `not-revisable needs-grooming` table row to a success test: a stub `revise` with a
  `## Why` replace commits; the record has no `spec:`, no `trivial:`, an unchanged
  `auto_groomable:`, a fresh `updated:`, a re-rendered `## Artifacts` block, and the board row
  still reads needs-grooming.
- New: stub `revise` with `spec_markdown` (and `spec_revision`) refuses `spec-not-linked` and
  writes nothing.
- New: stub `revise` with only `title` retitles it.
- New: `revise` with a section edit naming `## Auto-groom blocked` refuses
  `invalid-section-heading`, on both a stub and a groomed change.
- Keep every other `not-revisable` row (blocked, in-progress, deferred, implemented, done, killed).
- Mutation check: restore the old gate and confirm the stub-success test reddens; drop the guard
  and confirm the auto-groom-blocked test reddens.

## Out of scope

- Editing changes past `proposed`.
- Editing `type`, `priority`, or `auto_groomable` (no typed op edits the first two today; the
  third belongs to abstain/re-enable).
- Changing a stub's groom state through `revise`.
- Guarding `## Auto-groom blocked` on the `spec` / `trivial` outcomes.

## Success criteria

- A human asks "edit stub N's Why" and docket lands it through `change.groom` `outcome: revise`
  in one metadata commit with a refreshed board; the stub is still needs-grooming.
- No outcome other than `abstain` / `re-enable` can write `## Auto-groom blocked` via `revise`.
- The full suite is green.
