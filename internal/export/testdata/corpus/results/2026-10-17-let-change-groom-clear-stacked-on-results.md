<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0563 — change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely](../changes/archive/2026-10-18-0563-let-change-groom-clear-stacked-on.md)**
<!-- docket:backlink:end -->

# change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely — Results

**Human action:** No action is required to merge. Two follow-ups below are for backlog triage: three old deferred records still can't be revived or groomed, and there is no typed way to clear `branch:` on a proposed change.

## Outcome

`change.groom` can now un-stack a proposed change. Send `stacked_on: 0` and the record gets the empty `stacked_on:` key, so the change builds from the integration branch again. Omitting the field or sending `null` still leaves it unchanged. A negative value is refused with `invalid-stacked_on`. `deliverable: results-artifact` together with `stacked_on: 0` is accepted, which turns a stacked change into a results-artifact change in one step.

Changing `stacked_on` on a change that already has a `branch:` is now refused with the new code `not-restackable`. That covers setting a first parent, re-parenting, and clearing. Sending the value the record already has still passes. The refusal says what actually lifts it: decide what to keep from the branch, then clear the record's `branch:` by hand, or kill the change and capture a new one. Deleting the git branch alone does not lift it.

A revise that changes only relationship fields (`depends_on`, `related`, `discovered_from`, `adrs`, `stacked_on`) is no longer refused as `empty-revise`.

Groom now writes every frontmatter field one at a time, adding the key when the record lacks it. Records missing `stacked_on:`, `discovered_from:`, `auto_groomable:` or `updated:` used to fail with a missing-target or `overlapping-edit` error. They now groom normally.

The groom-next skill and the stacked-changes reference now describe the typed un-stack path. Both files had to be trimmed to stay within their size budgets, and a prose-contract sentinel now protects the new Stacking guidance.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head. The first run failed only `TestSkillSizeBudgets`, because the new skill text went over budget. A repair commit trimmed both files without raising the ceilings.
- Budget screening reported `BUDGET WATCH` and `PARALLEL-SENSITIVE` lines for several integration test files under `-j11`, with no `SERIAL CONFIRMED OVER BUDGET` line. These come from parallel contention, and this change touched none of those tests.
- Workers mutation-tested the new tests. Routing two inserts back through one patch set, writing `0` as a parent id, making `stackedOnChanges` always return true, and dropping the finding-code registration each turned the relevant tests red.
- Whole-branch review (deep tier) returned 3 findings. 1 important and 1 minor were fixed in-branch. The other minor is reported as a follow-up below. The full table is in the PR body.
- Backlog check: the 24 proposed and deferred changes were checked by title and scope.

## Known issues and follow-ups

### Records 0007, 0009 and 0010 still can't be revived or groomed

These three deferred records have no generated `## Artifacts` block. Revive and groom both rewrite that block, so both still fail with a missing-patch-target error on them. This is confirmed by code reading, not by running it. The other seven records the spec names (0008, 0195, 0248, 0257, 0263, 0273, 0302) do have the block and now groom correctly. The "0007-shaped" test fixture keeps an artifacts block, so it covers only the missing frontmatter keys. Workaround: run `repository.repair`, or add the block by hand before reviving. Suggested next action: insert the block on revive and groom when it's missing, or repair the three records. No existing change fits (checked 24). Capture a new change.

### No typed way to clear `branch:` on a proposed change

`not-restackable`'s remedy requires clearing `branch:` by hand, or killing the change and capturing a new one. Nothing else clears that field on a proposed record. This is confirmed. It only affects changes that were claimed and later deferred or reclaimed while they had a branch. Suggested next action: add an explicit typed operation if this comes up in practice. Related to #565 (killed-parent stacking, the main reason to un-stack). Capture a new change that links #565 under `related:`.
