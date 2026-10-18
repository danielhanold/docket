<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0558 — Point the budget-report rule in AGENTS.md at tests/README.md instead of listing the lines](../changes/archive/2026-10-18-0558-name-the-serial-confirmation-not-counted-budget-line-in-agen.md)**
<!-- docket:backlink:end -->

# Point the budget-report rule in AGENTS.md at tests/README.md — results

**Human action:** None needed. This is a docs-only change, the full suite passed, and review found nothing.

## Outcome

- `AGENTS.md`: the budget-report bullet now gives only the reminder (read the report even on a green run, because no budget line fails the run) and points to `tests/README.md`. It names no line prefix. `CLAUDE.md` is a symlink and changes with it.
- `tests/README.md`: the budget paragraph now sorts all five lines. `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` are screening findings. `SERIAL CONFIRMED OVER BUDGET:` and `OVER BUDGET:` are authoritative breaches to act on, and the paragraph points to the "Never grow a file past its budget" guidance for what to do. `SERIAL CONFIRMATION NOT COUNTED:` needs no action.
- `tests/runtime-budgets.tsv`: the header comment's copy of the line list is now a pointer to `tests/README.md`. Only comment lines changed.
- `docs/reference/glossary.md`: the budget entry now names `OVER BUDGET:` as an authoritative breach. The heading is unchanged.

## Verification performed

- A grep for the budget prefixes finds nothing in `AGENTS.md` or `tests/runtime-budgets.tsv`. `TestRuntimeBudgetsCorrespondence` passes.
- Full suite (`go run ./cmd/docket development test`) passed at d2de9768993eb1864280c0ae307fed170e322ee8. The budget report had only `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` screening lines. There was no `OVER BUDGET:`, `SERIAL CONFIRMED OVER BUDGET:`, or NOT COUNTED line.
- Review (docket-review-standard) found nothing.
