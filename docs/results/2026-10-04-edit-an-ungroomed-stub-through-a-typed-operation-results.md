<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0509 — Edit an ungroomed stub through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0509-edit-an-ungroomed-stub-through-a-typed-operation.md)**
<!-- docket:backlink:end -->
# Edit an ungroomed stub through a typed operation — results

**Human action:** None required. The change is covered by automated tests and the full suite is green. One optional walkthrough is listed below.

## Outcome

`change.groom` `outcome: revise` now accepts any `proposed` change, so a needs-grooming stub can have its title, owned proposal sections, and relationship fields edited through a typed operation. The stub stays needs-grooming: `revise` never writes `spec:`, `trivial:`, or `auto_groomable:`. A `spec_markdown` revise on a stub refuses `spec-not-linked` ("change NNNN has no linked spec to revise"), and the `not-revisable` refusal now names the status only.

A `revise` section edit naming `## Auto-groom blocked` now refuses `invalid-section-heading`, on stubs and groomed changes alike. Only `abstain` and `re-enable` write that section, so the marker and `auto_groomable:` always move together.

`docket-groom-next` routes a request to *edit* (not groom) a stub straight to the revise exit with no brainstorm. The glossary and `docket change groom` help match. A new prose-contract row, `change_0509_stub_revise`, pins that wording.

Deviation from the spec: the spec said to edit the skill under `internal/assets/embedded/tree/skills/`. That directory is generated, so the edit went to the repo-root `skills/` source and the embedded copy was regenerated. The skill's frontmatter `description:` was also widened so that requests to edit a stub reach the skill. The skill's word budget ceiling went up to 2170; the 78-line ceiling did not change.

## Human actions and testing

- **Optional — edit a real stub's Why.** Prerequisites: an installed binary built from this branch, and a scratch repository with one needs-grooming stub. Steps: write a groom request with `outcome: revise`, the stub's id and current revision, and a `sections` entry replacing `## Why`. Then run `docket change groom --input <file>`. Expected: `result: applied`, the stub's `## Why` replaced, `BOARD.md` still showing the stub as needs-grooming, and no `spec:` or `trivial:` added. Cleanup: discard the scratch repo.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed through the build gate. The run printed three informational `PARALLEL-SENSITIVE` budget lines for unrelated e2e/race tests and no `SERIAL CONFIRMED OVER BUDGET` line.
- Mutation checks: restoring the old gate made the stub-revise tests fail; disabling the new guard made both revise guard tests fail, while the existing re-enable check stayed green on its own; stripping the skill's edit-a-stub sentence made the prose contract fail.
- Whole-branch review ran at the standard tier and found 3 minor wording gaps: a glossary clause, the skill's list of fields revise never writes, and the contended-retry rule for a stub edit that loses a race to a groom. All three were fixed in-branch.

## Known issues and follow-ups

- `spec` and `trivial` groom outcomes can still name `## Auto-groom blocked` in a section edit. The spec deliberately left this out of scope. Impact is low: those outcomes are rarely run on an abstained stub with a section edit like that. If it matters, a human can capture a follow-up change.
