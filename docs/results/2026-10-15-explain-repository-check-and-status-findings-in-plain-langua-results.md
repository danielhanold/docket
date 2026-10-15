<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0553 — Explain repository check and status findings in plain language](../changes/active/0553-explain-repository-check-and-status-findings-in-plain-langua.md)**
<!-- docket:backlink:end -->

# Explain repository check and status findings in plain language — Results

**Human action:** Assessment pending: the build is green and the review fix pass is in progress.

## Outcome

Build complete at a3ee5f9b7 (10 plan tasks, all green on the first full-suite run). Every record-level finding from `docket repository check` and `docket status` now carries a plain-language message, an "is it harmful" impact line, and a remedy. Review fixes are pending.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at a3ee5f9b7 on the first attempt. Budget report showed BUDGET WATCH and PARALLEL-SENSITIVE screening lines only, with no SERIAL CONFIRMED OVER BUDGET.
- Whole-branch review (deep tier, bumped from standard for a 3019-line diff): 10 findings (0 blocker, 3 important, 7 minor). The fix pass is pending. Findings returned:
  1. important: `docket status` shows document-parse failures in raw form, with no impact and no remedy (`parseFinding`).
  2. important: `findingsToStatus` attaches hand-edit remedies to findings from mutation refusals, which describe records that were never written.
  3. important: record remedies in check name `.docket` and `docket repository repair` even when the setup state is not healthy and repair would refuse.
  4. minor: `HandEditThenRepair` glues "in `<file>`" onto the end of the instruction; the empty-rel case leaves "it" with nothing to refer to.
  5. minor: `PlainDocumentError` stacks two parentheticals and repeats the name.
  6. minor: the type-invalid text describes the slug rule, not the type rule.
  7. minor: placeholder values are quoted as if they were stored (`missing` status, `id: 0`).
  8. minor: the priority-unknown remedy doesn't name the groom revise priority edit for proposed changes.
  9. minor: a family-test fixture comment is misplaced.
  10. minor: the empty-slug catalog fixture shows an unrealistic path, `0263-.md`.
