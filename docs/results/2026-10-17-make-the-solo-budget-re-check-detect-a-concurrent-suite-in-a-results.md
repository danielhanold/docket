<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0528 — Make the solo budget re-check detect a concurrent suite in another worktree](../changes/archive/2026-10-17-0528-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a.md)**
<!-- docket:backlink:end -->

# Make the solo budget re-check detect a concurrent suite in another worktree — Results

**Human action:** None is required to merge. Two optional wording decisions are listed under Known issues and follow-ups.

## Outcome

Before this change, the suite runner's solo budget re-check could report `SERIAL CONFIRMED OVER BUDGET:` from a reading taken while another worktree's suite was loading the machine. That line is the only budget line treated as a real breach, so each false breach turned into follow-up and grooming work.

Now every `docket development test` run puts up a "run sign" while it runs. The sign is a file beside the budget-history store, held under a kernel lock, so a crashed run can never look alive. The re-check looks for other runs' signs just before and just after it times a test:

- No other run seen: behavior is unchanged.
- Another run seen and the test came in under the line: it clears exactly as before.
- Another run seen and the test came in over the line: the report prints `SERIAL CONFIRMATION NOT COUNTED: <test> — <N>s solo while another suite was running (<worktree>); …; recheck stays due`. No breach is recorded, and the test stays due so the next quiet run re-checks it first. If the runner cannot tell whether another run is going, it treats the reading the same way.
- Under the opt-in strict mode (`DOCKET_RUNTESTS_STRICT=1`, which no gate uses), a not-counted reading fails the run with exit 4, the same as a failed confirmation.

Nothing waits and nothing adds time. A sign that cannot be put up prints one warning and never fails a run. `tests/README.md` and the glossary describe the new line. The build follows the agreed design.

## Verification performed

- Full suite through `build.test_command` (`go run ./cmd/docket development test`) passed at the final head. The budget report has no `SERIAL CONFIRMED OVER BUDGET:` and no `OVER BUDGET:` lines. `test_go_race` (row 60 s, which runs the suiterunner tests) took 242 s under `-j11` and is `PARALLEL-SENSITIVE` with a last solo measurement of 55 s, about 18% headroom under its 90 s solo line. It was already parallel-sensitive before this change.
- Every new guard was mutation-tested by its build worker: dead-sign handling, can't-tell handling, self exclusion, the stale-temp sweep, the before and after scans, the not-counted record bookkeeping, strict arming, sign lifetime in `Run`, the warning-not-failure path, the own-sign wiring on both re-check paths, and the lock-before-publish ordering.
- A repo-wide search for `SERIAL CONFIRM` and `OVER BUDGET` found no executable consumer other than the release-candidate workflow. That workflow's line-anchored greps do not match the new prefix.
- Whole-branch review (deep tier): 2 minor findings, both fixed in-branch (a missing lock-before-publish guard, and README wording about strict mode). The full table is in the PR body.

## Known issues and follow-ups

- **The release-candidate workflow comment overstates its vocabulary.** A comment in `.github/workflows/release-candidate.yml` calls its two greps "the runner's complete budget vocabulary". The new `SERIAL CONFIRMATION NOT COUNTED:` line is in neither set, so a release run never shows it in the step summary and never fails on it. That is the intended behavior, but the comment is no longer exactly true. Impact: documentation only, confirmed. Suggested next action: reword the comment. Fits #557: retiring the direct `OVER BUDGET:` path already means editing those greps. #557 needs grooming, so edit it through `docket-groom-next 557` (revise); it stays needs-grooming.
- **AGENTS.md does not mention the new line.** The `## Guards and tests` budget bullet still names only `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:`. An agent seeing `SERIAL CONFIRMATION NOT COUNTED:` has only `tests/README.md` to explain it. Impact: minor, confirmed. AGENTS.md promotion is a human decision, so this change left it alone. Suggested next action: decide whether to add one clause. No existing change fits (checked 20); a new change a human captures.
- **Accepted blind spots, as designed.** Load from a hand-run `go test`, another repository, or a suite that both starts and finishes inside one re-check is not detected. Feature branches cut before this merges put up no signs until they are rebased.
