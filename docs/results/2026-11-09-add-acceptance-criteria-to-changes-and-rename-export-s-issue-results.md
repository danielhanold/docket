<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0582 — Add acceptance criteria to changes and rename export's issue kind to ticket](../changes/archive/2026-11-09-0582-add-acceptance-criteria-to-changes-and-rename-export-s-issue.md)**
<!-- docket:backlink:end -->

# Results: Add acceptance criteria to changes and rename export's issue kind to ticket

**Human action:** None required to merge. One optional check is listed below.

## Outcome

A change record can now carry an optional `## Acceptance criteria` section, and the change-record export is called `ticket` instead of `issue`.

- `change.create` accepts an optional `acceptance_criteria` body. When it is given, the section is written between `## What changes` and `## Out of scope`. A blank value writes nothing, and a request without the field produces the same digest as before, so older retries still replay. A body that contains a column-zero `## ` heading or an unterminated code fence is refused with `invalid-acceptance_criteria`.
- `## Acceptance criteria` is an owned change section, so `change.groom` and the other owned-section callers can add it or edit it.
- When an owned-section edit adds a missing proposal section (Why, What changes, Acceptance criteria, Out of scope, Open questions), it is placed in that order instead of at the end of the file. Every other added section is still appended at the end. Existing records are not reordered.
- Export's `issue` kind is now `ticket`, with no alias for the old name. `docket open change <id> --export` writes `<date>-ticket-<slug>.md`, and the export keeps the Acceptance criteria section.
- The convention and `docket-new-change` skill describe the section and the ordering rule. The embedded copies were regenerated.

## Human actions and testing

- **Optional — export a change that has acceptance criteria.** Prerequisite: the `docket` binary from this branch installed (or run with `go run ./cmd/docket` inside the feature worktree). Steps: create a scratch change with `docket change create` and an `acceptance_criteria` body, then run `docket open change <id> --export`. Expected: a file named `<date>-ticket-<slug>.md` whose sections run Why, What changes, Acceptance criteria, Out of scope. Cleanup: kill the scratch change with `docket change kill` and delete the exported file.

## Verification performed

- Each build task ran focused tests with a RED/GREEN cycle. Mutation probes turned the tests red when the heading was removed from the owned headings, when it was removed from the proposal order, when it was removed from the ticket allowlist, when placement fell back to end of file, and when the digest used the raw value instead of the cleaned-up one.
- The full suite (`go run ./cmd/docket development test`) passed at the final head. The budget report had no breach lines. The first run also showed screening notes for the repository-visibility and state integration shards running long under parallel load (streak 1/5); this change does not touch those tests.
- Review: the standard tier reviewed the whole branch and found 3 minor wording issues and nothing else. All three were fixed in one commit before the final suite run.
- Two skill size budgets were raised to fit the new documentation. The reviewer confirmed that this was acceptable.
