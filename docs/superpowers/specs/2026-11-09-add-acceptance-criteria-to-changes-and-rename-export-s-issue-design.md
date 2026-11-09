<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0582 — Add acceptance criteria to changes and rename export's issue kind to ticket](../../changes/archive/2026-11-09-0582-add-acceptance-criteria-to-changes-and-rename-export-s-issue.md)**
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

- **Proposal sections keep a consistent order, also when added later.** The five proposal sections have one order: Why, What changes, Acceptance criteria, Out of scope, Open questions. `change.create` already writes them in that order. When an owned-section edit (`change.groom`, reconcile, or any other caller of the shared splicer) adds one of them to a record that lacks it, it is placed in that order instead of at end of file. This matters because the ticket path adds criteria to existing changes routinely — creating a ticket from a change runs a readiness check whose refinement writes missing criteria through `change.groom` — and export keeps source order, so end-of-file placement would put the criteria after the non-goals and open questions in the ticket body. Every other section added later (Reconcile log, Why on hold, Auto-groom blocked, Run halted, Build evidence, and the rest) is still appended at end of file: those are event records, where chronological order is right.
- **"another issue" stays.** `cleanTitle` in `internal/export/refs.go` rewrites a known change reference in an inserted title to "another issue" / "other issues". That outward wording is unchanged: "another ticket" would be wrong when the referenced change has no ticket, and "issue" reads naturally inside a ticket system.

## Design

### `change.create`

- `ChangeCreateRequest` gains an optional `acceptance_criteria` string (`json:"acceptance_criteria,omitempty"`, not required). Its `schema` descriptor follows from the struct, as for the other fields.
- Omitted or blank (whitespace only) writes **no** section — the section is optional, so blank is absent, not an error.
- Non-blank: `render.NewChangeRecord` gains an `AcceptanceCriteria` body field, and `render.ChangeRecord` emits `## Acceptance criteria` between `## What changes` and `## Out of scope`. Section order of a new record becomes: Artifacts, Why, What changes, [Acceptance criteria], Out of scope, [Why on hold].
- Validation (builder's call made at grooming): the body gets the same structural checks `hold_reason` gets — no column-zero `## ` heading outside fenced code and no unterminated fence (`render.ValidateSectionBody`), and the authored-input size bound (`boundAuthored`) — refusing with a new `invalid-acceptance_criteria` finding code registered beside the existing codes. A heading inside the body would otherwise split the section and escape the owned set. `holdReasonProblem`'s messages name `## Why on hold`, so the new field needs its own messages: generalise that helper over the heading name or add a sibling (the builder's call).
- **Digest compatibility:** `changeCreatePayload` gains `AcceptanceCriteria string \`json:"acceptance_criteria,omitempty"\``, populated with the blank-normalized value (blank digests as absent), so a request that does not use the field digests exactly as before and an older retry still replays — the same pattern `hold`/`hold_reason` used.

### Ordered placement of proposal sections (`internal/render/section.go`)

`render.ApplySectionEdits` gains ordered placement for the proposal sections; every other behavior of the splicer is unchanged.

- The proposal order is one exported list in `internal/render`: `## Why`, `## What changes`, `## Acceptance criteria`, `## Out of scope`, `## Open questions`.
- A `replace` edit naming a proposal heading that is **absent** from the record is inserted:
  1. immediately after the section of the nearest **earlier** proposal heading (in the list) that is present — that is, at the start of the next top-level heading after that section, or at end of file when that section is last;
  2. otherwise, immediately before the nearest **later** proposal heading that is present;
  3. otherwise, at end of file (today's behavior).
- "Present" is judged against the record as edited by the same request, so several proposal sections added in one request land in proposal order relative to each other and to the sections already there.
- Layout matches the existing rules: the inserted section is preceded by exactly one blank line and followed by exactly one blank line before the next heading; at end of file it keeps the single trailing newline. All other bytes are unchanged.
- Absent non-proposal headings are appended at end of file exactly as today. A `replace` of a present section stays where it is. `remove` and `preserve` are unchanged.
- The rule keys on heading names, so callers whose owned set holds no proposal heading (the learnings operations) are unaffected.
- **No migration.** Existing records whose proposal sections are already out of order (for example an Open questions appended after a Reconcile log) are not reordered; only a section being added is placed.

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

- `skills/docket-convention/SKILL.md`, *Change body sections*: the opening sentence names `## Acceptance criteria` as optional at creation, and a new list entry after `## What changes` reads, in substance: "`## Acceptance criteria` — optional; verifiable conditions that belong to this change. Written by `change.create` (between What changes and Out of scope) when given; a later owned-section edit that adds it places it in proposal order. Never a build gate." The section also states the rule once: the proposal sections keep the order Why, What changes, Acceptance criteria, Out of scope, Open questions — an edit that adds one places it in that order — and every other section added later is appended at end of file.
- `skills/docket-new-change/SKILL.md` step 4: the request description names the optional `acceptance_criteria` body field beside `why`/`what_changes`/`out_of_scope`.
- Regenerate the embedded twins with `go generate ./internal/assets/`.
- Docs describe only current behavior: no change numbers or history in the convention or skill text.

## Tests

- **Renderer** (`internal/render`): `ChangeRecord` with criteria places `## Acceptance criteria` between What changes and Out of scope (and before Why on hold when both are set); without criteria the output is byte-identical to today.
- **`change.create`** (`internal/app`): criteria land in place; blank criteria write no section; a body carrying a column-zero `## ` heading or an unterminated fence refuses with `invalid-acceptance_criteria` and writes nothing; a request without the field produces the same digest as before the field existed (replay compatibility).
- **Splicer** (`internal/render`): an absent proposal section is inserted after its nearest present predecessor (Acceptance criteria after What changes, before Out of scope; Open questions after Out of scope, ahead of a later Why on hold or Reconcile log); before its nearest present successor when no predecessor is present; at end of file when no proposal section is present; two proposal sections added in one request land in order; an absent non-proposal section is still appended at end of file; a present section replaced in place does not move; CRLF and fenced-code headings behave as today; all other bytes are identical.
- **`change.groom`**: a `revise` adding `## Acceptance criteria` to a record that lacks it places it between What changes and Out of scope; a `revise` replacing an existing one keeps its position; a `spec` outcome carrying it as a section edit is accepted.
- **Export**: the transform under `KindTicket` keeps `## Acceptance criteria` and drops non-allowlisted sections as before; `FileName` yields `<date>-ticket-<slug>.md`; the corpus test's allowlist includes the new heading.
- Guards are mutation-checked: removing the heading from `ChangeOwnedHeadings`, from the proposal order, or from `ticketSections`, or falling back to end-of-file placement, must redden a test.
- The whole suite runs at the build gate.

## Acceptance criteria

- A change created with acceptance criteria shows `## Acceptance criteria` between What changes and Out of scope.
- `change.groom` adds the section to an existing change between What changes and Out of scope, or replaces it where it already is; any proposal section added later lands in proposal order, while other added sections still go at end of file.
- `docket open change <id> --export` writes `<date>-ticket-<slug>.md` and keeps the section.
- A change without the section builds as before and exports the same sections as before, under the `ticket` filename.

## Out of scope

- Reviewers checking a build against acceptance criteria (a possible follow-up).
- The legacy `issue:` frontmatter field and repair's `"issue"` key (change 583).
- Reordering existing records whose sections are already out of order.
- Ordered placement for non-proposal sections.
- Renaming export's "another issue" reference wording.
- Any board change, and any ticket-system call.
