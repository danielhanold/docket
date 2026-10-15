<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0550 — Let change.groom set a change's priority](../changes/archive/2026-10-15-0550-let-change-groom-set-a-change-s-priority.md)**
<!-- docket:backlink:end -->

# Results: Let change.groom set a change's priority

**Human action:** None required. The change is fully covered by automated tests; an optional walkthrough is below.

## Outcome

`change.groom` now accepts an optional `priority` field (`critical`, `high`, `medium`, `low`). A valid value is written to the record's `priority:` line in the same metadata commit as the rest of the groom, so the board shows the new priority right away. The field works with the spec, trivial, revise and re-enable outcomes. A revise that carries only a priority is accepted. An unknown value is refused with `unknown-priority` before anything is written, and an abstain that carries a priority is refused with the new `invalid-priority` code. The `docket-groom-next` skill documents the field in a new **Reprioritize** paragraph, and the glossary is updated.

## Human actions and testing

- **Optional — reprioritize a proposed change by hand.** Prerequisites: an installed binary built from this branch and a scratch docket repository with a proposed change (for example #1 at `medium`). Steps: write a request file with `id`, the record's current blob `revision`, `outcome: revise` and `priority: high`, then run `docket change groom --input <file> --json`. Expected: `result: applied`; the record shows `priority: 'high'` and `BOARD.md` shows the change at high priority in the same commit. Repeat with `priority: urgent` and expect `unknown-priority` with nothing written. Cleanup: delete the scratch repository.

## Verification performed

- Full build gate (`go run ./cmd/docket development test`) passed at the branch head. The budget report showed only screening lines (`BUDGET WATCH`, `PARALLEL-SENSITIVE`) for integration scripts this change does not touch, and no `SERIAL CONFIRMED OVER BUDGET` line.
- Each task's workers mutation-tested their guards: removing the `ParsePriority` check, the priority clause of the empty-revise rule, the `upsertField` write, or the skill paragraph each turned the matching tests red.
- Review: `docket-review-standard` reviewed the whole branch and returned no findings.
