<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0461 — Allow editing an existing change's title](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0461-allow-editing-an-existing-change-s-title.md)**
<!-- docket:backlink:end -->

# Retitle an existing change — design

## Goal

Let a human retitle a `proposed` change through a typed operation, so `title:` stays
writer-quoted (ADR-0071) and the board, the change's `## Artifacts` block, and the spec's
`docket:backlink` block move with it in one metadata transaction. The slug, filename,
`branch:`, and spec path never change: a title edit renames nothing.

## Background (traced 2026-09-28)

- Title and slug are decoupled after creation. `slugifyTitle` runs only in `change.create`
  and the ADR create/supersede ops; every title reader (`render/board.go` rows,
  `render.BacklinkContent`, `status`, `implementation_context`) reads `title:` directly, and
  nothing compares it to `slug:`.
- `changeGroomOp.Plan` (`internal/app/change_groom.go`) already patches frontmatter
  generically (`upsertField` / `SetField`), then renders the artifact block and board from
  the groomed candidate snapshot. A title patch slots into that first patch pass.
- The only title-bearing backlink for a `proposed` change is the linked spec's
  (`Change NNNN — <title>`); plan, results, and PR body do not exist before claim. Today
  revise re-stamps the spec backlink only on a whole-body `spec_markdown` replace.
- Revise refuses a request with neither `spec_markdown` nor a replace/remove section edit
  (`empty-revise`), and its gate admits only already-groomed changes.
- The board writes `c.Title()` into table cells unescaped, and no validator rejects a
  newline, so a `|` or line break in any title (from create or a retitle) corrupts the table.

## Design

### Request

`ChangeGroomRequest` gains `Title string \`json:"title,omitempty"\``. Empty/absent means
"leave the title unchanged". The `schema` descriptor picks the field up from the struct as it
does for every other request field.

### Outcome acceptance (`validateChangeGroomShape`)

| Outcome | `title` |
|---|---|
| `spec`, `trivial`, `rearm` | accepted |
| `revise` | accepted; a non-empty `title` alone satisfies the effective-edit minimum |
| `abstain` | refused with a new `invalid-title` finding code (abstain cannot rewrite the proposal, same rule as its relationship-field refusals) |

The `empty-revise` condition becomes: no `spec_markdown`, no replace/remove section edit,
**and** no `title`. Update its diagnostic text to name all three.

No status change: the existing groom gate and its exact complement, the revise gate, keep
every title edit on `proposed` changes.

### Validation (shared)

A new helper `validateTitle(title string) (FindingCode, string)` (or equivalent returning a
finding) requires the trimmed title to be non-empty, a single line (no `\n` / `\r`), and free of
other control characters (`unicode.IsControl`). It is called:

- by `change.create`, alongside its existing `empty-title` and valid-slug-token checks (both
  unchanged; the slug check still governs the minted slug);
- by `change.groom` whenever `title` is non-empty, reusing `FCEmptyTitle` for whitespace-only
  and a new `invalid-title` code for multi-line / control characters.

Both run as request-shape validation, so a bad title never reaches the engine.

### Plan (`changeGroomOp.Plan`)

1. **Field patch.** In the first patch pass, when `title` is non-empty,
   `ps.SetField("title", document.String(title))` (every canonical record carries `title:`; use
   `upsertField` if the implementer finds a producer that omits it). The writer quotes the
   scalar, so ADR-0071 holds by construction. `updated:` is already upserted.
2. **Derived views.** Unchanged code: the candidate snapshot built from the patched bytes
   drives `render.ArtifactBlockContent` and `includeBoard`, so the board row and archive rows
   pick up the new title.
3. **Spec backlink re-stamp.** When the groomed title differs from the current one, the change
   links a spec (`c.Spec().Value != ""`), and the request carries no `spec_markdown`
   (the spec outcome and a spec-body revise already render the backlink from `gc`): read the
   spec's current bytes with `treeBlob`, refuse with the existing `spec-file-missing` when
   absent, replace only the `docket:backlink` block with `render.BacklinkContent(gc, o.link)`
   via the document block-replace path, and declare a `MutationReplace` only if the bytes
   changed. No `spec_version` is required: the re-stamp reads from the attempt's own fresh
   base, so a concurrent spec edit surfaces as a contended push rather than being clobbered —
   unlike a whole-body replace, which carries caller-authored bytes. A spec missing its
   backlink block is refused (new code or reuse of the existing malformed-marker refusal),
   never silently inserted.
4. **No-op.** A title equal to the current one yields byte-identical outputs; the existing
   "declare only changed paths" rule makes that a clean no-op.

Receipt shape and commit subject (`change NNNN groomed (<outcome>)`) are unchanged.

### Board escaping (`internal/render/board.go`)

Escape `|` as `\|` in title cells for every active-section row (`boardSectionRow`) and archive
row, reusing the replacer shape `boardRepairCell` already uses (a shared `boardTitleCell`
helper). Newline flattening is harmless defense for legacy records that predate the validator.

### Skill prose

`docket-groom-next` Step 4: exits 1 (spec), 2 (trivial), 5 (revise), and 6 (re-arm) may carry
`title` when the settled design renamed the change; note that the slug and paths never change.
No convention change is needed beyond that pointer.

## Testing

- Title-only revise on a spec'd change: one commit updates `title:`, `updated:`, the board
  row, and the spec's backlink line; spec body bytes outside the block are untouched.
- Title on the `spec` outcome (new spec's backlink carries the new title) and on the `trivial`
  outcome (no spec file touched).
- `abstain` with `title` → `invalid-title`, nothing written.
- Revise with the current title → applied no-op, no files declared.
- Multi-line / control-character title refused by both `change.create` and `change.groom`;
  whitespace-only → `empty-title`.
- A title with `|` renders escaped in the board; a title with `:` / `#` / leading quote
  round-trips through the writer as a string.
- A linked spec lacking a backlink block is refused on a title-only revise.
- Mutation-test the re-stamp and the escaping: remove each and watch its test redden.

## Out of scope

- Retitling any non-`proposed` change (would need feature-branch plan/results backlinks and the
  PR title).
- Renaming the slug, record file, spec path, or branch.
- Editing other scalars (priority, type).
- Retitling terminal (archived) records.
