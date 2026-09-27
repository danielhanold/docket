<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0382 — ChangeCreateRequest should accept typed auto_groomable / branch_prefix scalars](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-27-0382-changecreaterequest-typed-auto-groomable-branch-prefix-scalars.md)**
<!-- docket:backlink:end -->
# ChangeCreateRequest should accept typed auto_groomable / branch_prefix scalars — Results

**Human action:** No action is required before merge beyond the normal PR review. One behavior change is worth a glance: presence markers such as `## Auto-groom blocked` inside fenced code blocks no longer count on the board (see Known issues).

## Outcome

Before this change, three skill steps edited a change's `auto_groomable` and `branch_prefix` fields with plain git: `docket-new-change` after creating a change, `docket-auto-groom` when it abstained, and a human re-arming an abstained stub. The two auto-groom edits left `BOARD.md` stale, and a malformed branch prefix only failed later, inside an autonomous implement run.

Now:

- `docket change create` takes optional `auto_groomable` (true, false, or unset) and `branch_prefix` fields. The prefix is trimmed, has one trailing `/` removed, and is lowercased. An invalid prefix is refused up front with `invalid-branch_prefix`.
- `docket change groom` has two new outcomes. `abstain` sets `auto_groomable: false` and appends a dated `## Auto-groom blocked` entry. `rearm` sets it back to `true` and removes that section. Both re-render the board in the same commit.
- The four skills (new-change, auto-groom, groom-next, convention) use these typed operations instead of hand edits.

Review fixes applied in-branch:

- The board and `rearm` now agree on what counts as a blocked marker. Heading-shaped lines inside fenced code are ignored by both.
- A hand-typed non-boolean `auto_groomable` (for example `yes`) is reported as a warning. It is not an error, so it cannot block PR publication or finalize on a record that used to be valid.
- New tests cover abstain/re-arm when another section follows the blocked section. The `docket-groom-next` skill now documents the re-arm exit.

The `docket-groom-next/SKILL.md` word ceiling was raised from 1889 to 1996 with human approval.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed on the pre-review head f1c117c6 (54/54 files). The suite was run again on the final head and its result is in the PR's build-evidence block.
- A deep-rung whole-branch review returned 4 minor findings and no blockers or important findings. All four were fixed (commits 529d7f32, 6b24de39), and focused package tests passed after each fix.
- The new tests for a section following the blocked section were not mutation-checked against the section-boundary logic.

## Known issues and follow-ups

### Fenced presence markers no longer show on the board

This affects any change file that has `## Run halted`, `## Auto-groom blocked`, `## Finalize blocked`, or `## Publish deferred` only inside a fenced code block (for example, a change that discusses those headings). Before this change the board treated such a record as halted or blocked. Now it does not. This matches how the typed operations that write and remove those sections already behaved, so it is an intended, confirmed change. Nothing needs to be done unless a real marker was ever written inside a fence, which the writers never do.

### Suite budget watch lines

The pre-review full suite printed several `BUDGET WATCH` lines, for example `test_go_race` at 407s and `test_go_toolchain` at 390s under -j11. These are screening findings, and none of them is a serially confirmed breach. No action is needed unless they repeat.
