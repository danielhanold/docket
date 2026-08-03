<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0191 — Enforce YAML scalar well-formedness in change-file frontmatter](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-03-0191-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md)**
<!-- docket:backlink:end -->
# Enforce YAML scalar well-formedness in change-file frontmatter — results
Change: #191 · Branch: feat/enforce-yaml-scalar-wellformedness-in-change-frontmatter · PR: <opened at step 7> · Plan: docs/superpowers/plans/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md · ADRs: none

## Verify (human)

No interactive/manual checks required — everything is covered by automated tests. One expected
real-history outcome worth knowing at the merge gate: after this lands, the next board health pass
surfaces change **0121's** unquoted colon-space `title:` as the `scalar-form` check's first finding
— **warn-only by design**, exactly the work the check exists to surface, not a regression.

## Findings

- **Build gate green: 74/74 tests passed** (command `for test in tests/test_*.sh; do
  "$DOCKER_BASH_PATH" "$test"; done`; evidence head_sha `52973a28`). The four pinned `BOARD_CHECK_IDS`
  surfaces (lib array 13→14, `--help` header, `board-checks.md` per-check section, `docket-status.md`
  `check <check-id>` row) are all green, including the both-directions set-compares and the doc-drift
  pins.
- **Corpus verification (spec Assumption 6):** exactly one real change has an unquoted colon-space
  `title:` — 0121 — and no change has a bare-boolean title. Quoted colon-space titles (e.g. 0190's
  own `title: "…"`) stay green, confirming the quote leg keeps the 0190 accept shape silent.
- **Whole-branch review (docket-review-standard, rung from the build record): 0 blockers, 0
  important, 3 minor** — recorded for the human's merge-time judgment, not fixed here:
  1. `scripts/board-checks.sh` — `scalar_form_check(){…}` is defined inside the per-file walk loop
     (redefined per file); the file's other helpers are top-level. Recommend hoisting next to
     `renders_row`. Cosmetic only.
  2. `scripts/board-checks.sh` / `board-checks.md` — undocumented comment-strip asymmetry: `blocked_by`
     is read via `fm_field_raw` (strips a whitespace-preceded `#…` inline comment), `title` via
     `field_raw` (keeps it), so a `title: Fix thing # see: notes` line flags while the same shape on
     `blocked_by` does not. Recommend a sentence in the block comment + `board-checks.md` section (or a
     pinning fixture).
  3. `tests/test_board_checks.sh` — the count-pin provenance comment above the `BOARD_CHECK_IDS`
     assert still reads "13 since change 0117…" while the assert now says 14; reword for the next
     bumper.
- **Notable plan deviation (router reconciliation):** Task 2's worker returned `BLOCKED` because the
  doc-drift pins in `test_board_checks.sh` make the `--help`/array/count change ungreensable in
  isolation — the doc edits (plan Task 4) must land first. No malformed return and no escalation was
  involved; the router reordered Task 4 before Task 2's gate commit. The check itself and all fixtures
  are otherwise exactly as planned. The three minor findings above were the only reviewer output.

## Follow-ups

None automatically minted (auto-capture enabled, `types: all`): all three minor findings stay inside
this change's own surface — each is resolved by a trivial edit within the diff or noted in the PR
body — and nothing crossed the materiality bar for its own change/PR. Per spec Assumption 5 no ADR
was produced (the change codifies AGENTS.md's house rule and ADR-0065's quote leg into a guard), and
the reviewer did not read the colon-space leg as a new decision.
