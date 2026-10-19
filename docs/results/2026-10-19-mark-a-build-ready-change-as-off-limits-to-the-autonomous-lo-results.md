<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0560 — Put a groomed change on hold so loops skip it](../changes/active/0560-mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo.md)**
<!-- docket:backlink:end -->

# Put a groomed change on hold so loops skip it — Results

**Human action:** No action is required before merging. One optional walkthrough below lets you see a hold work end to end on a scratch repository.

## Outcome

Before this change, every groomed change looked the same to an autonomous loop. A change that needed a person for some step was claimed, worked until that step, and halted, and the halt ended the whole drain.

Now a change can carry `hold: true` in its frontmatter:

- A held change that would otherwise be build-ready reads as `on-hold`. It is left out of the `status` operation's `ready` queue, out of `context.implementation` with no id, and out of an id-list drain. A run that names that one change still builds it, and a claim or `change.complete` by id accepts it.
- Any earlier readiness outcome wins. A held stub still reads needs-grooming, and a held change waiting on a dependency or an unbuilt stack parent still shows that wait.
- A garbled value (such as `hold: yes`) reads as held and raises the usual malformed-field warning, so a bad hand edit keeps work away from the loop instead of letting it through.
- `change.groom` (spec, trivial, revise and re-enable outcomes) and `change.create` accept `hold` and an optional `hold_reason`. The reason is written to a `## Why on hold` section, and lifting the hold removes it. Abstain refuses `hold` with `invalid-hold`. A reason sent without `hold: true` is refused with `hold-reason-without-hold`, and an empty or malformed reason with `invalid-hold_reason`. No other edit may write that section.
- On the board, the Groomed table has a new Readiness column that reads `build-ready` or `on hold`. A held trivial change stays in Proposed with `on hold`. The In progress table's Readiness column is now called State and reads `building` or `run halted — needs you`, so no cell is left blank.
- The skills (implement-next, groom-next, new-change, auto-groom, convention, status) and the user docs describe the hold. The auto-groomer sets a hold on specs that name a manual step and never lifts one. ADR-0153 records the decision.

## Human actions and testing

### Optional — see a hold in a scratch repository

This shows a held change dropping out of the loop's queue while a named read still accepts it. Automated tests already cover it.

Prerequisites: the `docket` binary built from this branch, and a throwaway docket repository with two build-ready changes, A and B (for example, two trivial changes).

1. Put A on hold: `docket change groom` with a revise request carrying `"hold": true, "hold_reason": "Needs a sign-in."` (run `docket schema` to see the request keys).
   Expected: A's record gains `hold: true` and a `## Why on hold` section, and `BOARD.md` shows A as `on hold`.
2. Run `docket status --json`.
   Expected: the `ready` array lists B but not A. A's row has `readiness: on-hold` and `readiness_reason: on hold`.
3. Run `docket context implementation --id <A> --json`.
   Expected: the bundle is returned with `claim_eligible: true`.
4. Lift the hold with a revise request carrying `"hold": false`.
   Expected: `hold:` is empty, the `## Why on hold` section is gone, and A is back in `ready`.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head, after the review fixes. Build evidence is recorded in the change record.
- Each new guard was mutation-tested by its build worker: when the guarded behavior was removed, the test went red.
- The suite printed BUDGET WATCH lines for six integration test files that ran long under parallel load. These are screening notes, not breaches, and this change does not touch those tests.
- Whole-branch review (deep tier): 6 minor findings, all fixed in-branch; full table in the PR body.

## Known issues and follow-ups

- **The live board changes shape after install.** Once a binary with this change is installed, the next metadata write re-renders `BOARD.md`: the Groomed table gains a Readiness column and the In progress table's column becomes State, reading `building`. The suite cannot see the live board. This is confirmed and expected, with no harm; no action is needed beyond noticing the one-time diff.
