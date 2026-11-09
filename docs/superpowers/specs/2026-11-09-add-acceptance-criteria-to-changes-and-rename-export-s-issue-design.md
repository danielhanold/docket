<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0582 — Add acceptance criteria to changes and rename export's issue kind to ticket](../../changes/active/0582-add-acceptance-criteria-to-changes-and-rename-export-s-issue.md)**
<!-- docket:backlink:end -->

# Acceptance criteria on changes, and export's `issue` kind renamed to `ticket`

Change #582, groomed interactively on 2026-11-09.

## Why

The ticket integration (designed in change 581's spec) needs two small foundations before any ticket capability can be built:

1. A ticket needs **acceptance criteria that belong to the change**. A change body has no place for them today.
2. The export that will become a ticket body is the change-record export, whose kind is named after GitHub issues: `docket open change <id> --export` writes `<date>-issue-<slug>.md`. The ticket work uses one term, "ticket", throughout, and no name should contradict its value.

This change carries both. It builds without reading change 581: every decision it needs is restated here.

## Settled decisions (fixed by change 581; not reopened)

- `## Acceptance criteria` is a new, **optional** change-body section, placed **after `## What changes` and before `## Out of scope`**.
- It is settable at creation (`change.create`) and editable through `change.groom`'s owned sections (outcomes `spec` and `revise`), like the other proposal sections.
- It is **never a build gate**: build-readiness is unchanged, and a change without it is as complete as before. Changes that never become tickets need none.
- The convention's body-section list documents it.
- Export carries it: the change-record export's section allowlist gains `Acceptance criteria`.
- Export's `issue` kind becomes `ticket`, its internal identifiers renamed with it, and the exported change file becomes `<date>-ticket-<slug>.md`. The rename is internal apart from that filename: no JSON field carries the kind (`open`'s `target_kind` is `file`, a different thing), and no skill or guide names the `-issue-` filename.
- **No compatibility alias** for the old kind or filename.

## Decisions made at grooming

- **Placement when added later: end of file.** `change.create` controls a new record's section order, so a created change gets the section in place. When `change.groom` (or any other owned-section edit, such as reconcile) adds the section to a record that lacks it, the shared splicer `render.ApplySectionEdits` appends it at end of file — its existing behavior for every absent section (Open questions and Why on hold land the same way). No ordered-insertion logic is added. Replacing a section that is already present keeps it where it is. Consequence, accepted: export keeps source order, so criteria added after creation also appear last in the ticket body.
- **"another issue" stays.** `cleanTitle` in `internal/export/refs.go` rewrites a known change reference in an inserted title to "another issue" / "other issues". That outward wording is unchanged: "another ticket" would be wrong when the referenced change has no ticket, and "issue" reads naturally inside a ticket system.

## Design

### `change.create`

- `ChangeCreateRequest` gains an optional `acceptance_criteria` string (`json:"acceptance_criteria,omitempty"`, not required). Its `schema` descriptor follows from the struct, as for the other fields.
- Omitted or blank (whitespace only) writes **no** section — the section is optional, so blank is absent, not an error.
- Non-blank: `render.NewChangeRecord` gains an `AcceptanceCriteria` body field, and `render.ChangeRecord` emits `## Acceptance criteria` between `## What changes` and `## Out of scope`. Section order of a new record becomes: Artifacts, Why, What changes, [Acceptance criteria], Out of scope, [Why on hold].
- Validation (builder's call made at grooming): the body gets the same structural checks `hold_reason` gets — no column-zero `## ` heading outside fenced code and no unterminated fence (`render.ValidateSectionBody`), and the authored-input size bound (`boundAuthored`) — refusing with a new `invalid-acceptance_criteria` finding code registered beside the existing codes. A heading inside the body would otherwise split the section and escape the owned set. `holdReasonProblem`'s messages name `## Why on hold`, so the new field needs its own messages: generalise that helper over the heading name or add a sibling (the builder's call).
- **Digest compatibility:** `changeCreatePayload` gains `AcceptanceCriteria string \`json:"acceptance_criteria,omitempty"\``, populated with the blank-normalized value (blank digests as absent), so a request that does not use the field digests exactly as before and an older retry still replays — the same pattern `hold`/`hold_reason` used.

### Owned headings

- `"## Acceptance criteria"` joins `render.ChangeOwnedHeadings`. Every operation that splices owned change sections picks it up: `change.groom` (`spec`, `revise`, and `re-enable` section edits), reconcile, and the lifecycle/kill/claim callers that pass the shared list. No per-operation code change is needed beyond the list itself; `change.groom`'s `invalid-section-heading` check already derives from the list.
- It is not a build-readiness input anywhere (`status` readiness, the selector, the board are untouched).

### Export rename

| Today | Becomes |
|---|---|
| `KindIssue Kind = "issue"` (`internal/export/export.go`) | `KindTicket Kind = "ticket"` |
| `issueSections` (allowlist: Why, What changes, Out of scope, Open questions) | `ticketSections`, also allowing `Acceptance criteria` |
| `stepIssueSections`, `keepIssueSections` | `stepTicketSections`, `keepTicketSections` |
| `FileName`'s `kind != KindIssue` branch and doc comment (`internal/export/filename.go`) | `KindTicket` |
| `"change": export.KindIssue` in `exportKinds` (`internal/app/open_export.go`) | `export.KindTicket` |
| tests: `issueFixture`, `issueWant`, `TestTransformIssue` (`export_test.go`); `issueH2` and its uses (`corpus_test.go`); the `issue-…` expectations (`filename_test.go`) | `ticket` equivalents; `ticketH2` includes `## Acceptance criteria` |

Comments that call the change-record export "the issue" (for example `Title`'s "the issue H1") follow the rename.

**Not renamed:** `cleanTitle`'s "another issue" / "other issues" wording (decided above); the `"issue"` key in `internal/reposetup/repair.go`, which is the legacy frontmatter field retired by change 583; the archived spec of change 561 that mentions `-issue-` (a point-in-time record).

### Documentation

- `skills/docket-convention/SKILL.md`, *Change body sections*: the opening sentence names `## Acceptance criteria` as optional at creation, and a new list entry after `## What changes` reads, in substance: "`## Acceptance criteria` — optional; verifiable conditions that belong to this change. Written by `change.create` (between What changes and Out of scope) when given; a later owned-section edit that adds it appends it at end of file. Never a build gate."
- `skills/docket-new-change/SKILL.md` step 4: the request description names the optional `acceptance_criteria` body field beside `why`/`what_changes`/`out_of_scope`.
- Regenerate the embedded twins with `go generate ./internal/assets/`.
- Docs describe only current behavior: no change numbers or history in the convention or skill text.

## Tests

- **Renderer** (`internal/render`): `ChangeRecord` with criteria places `## Acceptance criteria` between What changes and Out of scope (and before Why on hold when both are set); without criteria the output is byte-identical to today.
- **`change.create`** (`internal/app`): criteria land in place; blank criteria write no section; a body carrying a column-zero `## ` heading or an unterminated fence refuses with `invalid-acceptance_criteria` and writes nothing; a request without the field produces the same digest as before the field existed (replay compatibility).
- **`change.groom`**: a `revise` adding `## Acceptance criteria` to a record that lacks it appends it at end of file; a `revise` replacing an existing one keeps its position; a `spec` outcome carrying it as a section edit is accepted.
- **Export**: the transform under `KindTicket` keeps `## Acceptance criteria` and drops non-allowlisted sections as before; `FileName` yields `<date>-ticket-<slug>.md`; the corpus test's allowlist includes the new heading.
- Guards are mutation-checked: removing the heading from `ChangeOwnedHeadings`, or from `ticketSections`, must redden a test.
- The whole suite runs at the build gate.

## Acceptance criteria

- A change created with acceptance criteria shows `## Acceptance criteria` between What changes and Out of scope.
- `change.groom` adds the section to an existing change at end of file, or replaces it where it already is.
- `docket open change <id> --export` writes `<date>-ticket-<slug>.md` and keeps the section.
- A change without the section builds as before and exports the same sections as before, under the `ticket` filename.

## Out of scope

- Reviewers checking a build against acceptance criteria (a possible follow-up).
- The legacy `issue:` frontmatter field and repair's `"issue"` key (change 583).
- Ordered insertion of sections added after creation.
- Renaming export's "another issue" reference wording.
- Any board change, and any ticket-system call.
