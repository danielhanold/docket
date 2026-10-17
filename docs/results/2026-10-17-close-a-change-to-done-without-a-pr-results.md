<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0559 — Let a change deliver a results artifact instead of a PR](../changes/active/0559-close-a-change-to-done-without-a-pr.md)**
<!-- docket:backlink:end -->

# Let a change deliver a results artifact instead of a PR — Results

**Human action:** Optional only. The automated suite covers the new operation, cleanup, and run-verify paths. The new implement-next results-artifact ending is skill prose that no test exercises, so the first real run of a results-artifact change (for example #547) is its first proof.

## Outcome

Before this change, a change could reach a final status only through a merged PR (`done`) or through kill (`killed`, which reads as abandoned). Research and measurement work that ends in findings had no honest ending.

Now every change has a `deliverable`: `pr` (the default, and what an absent field means) or `results-artifact`.

- `docket change create` writes the field. `change.groom` can now edit `deliverable`, `type`, and `branch_prefix` on a proposed change. A `results-artifact` change cannot be stacked, and a change cannot stack on one.
- The new `docket change complete` operation moves a build-ready proposed or an in-progress `results-artifact` change to `done` and archives it in one metadata commit, with no PR. It requires a valid final results artifact. A replay returns `already-completed`. `change.mark-implemented` now refuses a `results-artifact` change, so each deliverable has exactly one ending.
- `finalize cleanup` has a no-PR path for a completed change. It removes a clean worktree and deletes the local branch only if the branch has no commits of its own. It never touches a remote ref. It reports `already-clean` when nothing is left on this machine, including a change that was claimed but never had a workspace prepared. The sweep cleans a leftover worktree through the same path.
- `run verify` reports `run-complete` for a completed `results-artifact` change and `run-incomplete` with `not-completed` while it is still in progress.
- A completed change's Plan and Results links point at the metadata branch. The board's archive date column now reads "Closed" instead of "Merged".
- Every no-op replay result now names what already happened: `already-closed-out`, `already-recorded`, `already-retargeted`, and four internal values. A bare `already` no longer appears anywhere.
- implement-next has a results-artifact ending (`skills/docket-implement-next/references/results-artifact.md`). It does the work in the worktree without committing, halts at human steps, attaches the results, has them reviewed, completes the change, and cleans up. `docket-review` gained a results-artifact mode. The convention, the grooming and capture skills, and the user docs describe the new ending.
- ADR-0151 records the decision.

Departures from the spec:

- The groom refusal for an invalid branch prefix keeps the existing code `invalid-branch_prefix` rather than the spec's `invalid-branch-prefix`.
- A refusal from `change complete` carries an empty disposition and reports its reason as the first finding's code, so the four-value disposition list stays closed.

## Human actions and testing

### Optional — complete a throwaway results-artifact change by hand

This exercises the new operation and cleanup end to end on a real repository, beyond the fixtures.

Prerequisites: the binary built from this branch is installed, and you have a scratch repository with docket initialized (`docket repository init`).

1. Create a change with `deliverable: results-artifact` and `trivial: true` via `docket change create --request <file>`.
   Expected: the record shows `deliverable: 'results-artifact'` right after `type:`.
2. Attach a results file with `docket change attach-results` (a `**Human action:**` line and an `## Outcome` section are enough).
3. Run `docket change complete --request <file>` with `change_id`, `path`, and `revision` taken from `docket status --json`.
   Expected: disposition `completed`, the record moves to `archive/`, and BOARD.md lists it under the "Closed" column.
4. Run the same command again.
   Expected: `already-completed`, and no new commit.
5. Run `docket finalize cleanup --id <id>`.
   Expected: `already-clean`, since no worktree was ever prepared.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) through the build gate: green on the build head, and green again on the final head after the review fixes. Build evidence is verified against the final head.
- Each task worker ran focused tests with a red-then-green cycle where behavior changed, and mutation-tested its guards: the deliverable precondition, scoped validation, the readiness check, the own-commits branch check, the results-artifact link rule, run-verify branches, the no-bare-`already` vocabulary check, the feature-dispatch payload, and the never-prepared `already-clean` guard.
- The capability catalog is 15963 of 16384 bytes after the new `change.complete` entry. The cap was not raised.
- Budget report: no serial-confirmed breach. Both gate runs printed `PARALLEL-SENSITIVE` and `BUDGET WATCH` screening lines for several integration shards (privatemode, rebase, recordops, repovisibility*, state, finalizerebaseops, merge, finalize e2e) under `-j11`. These are machine-load screening signals, not failures. The new FinalizeCleanup integration tests add roughly 8 to 10 seconds to that shard.
- Whole-branch review (deep tier, bumped from standard for a diff over 1500 lines): 6 findings (1 important, 5 minor), all fixed in-branch. The full table is in the PR body.
- Backlog check: I compared each follow-up below against all 22 proposed and deferred changes.

## Known issues and follow-ups

- **The sweep does not retry a leftover local branch.** Suppose cleanup removes a completed change's worktree, but the local-branch delete then fails on a transient error. No later sweep will retry the delete, because the sweep looks only for worktrees. `pr` changes already have the same gap. Impact: a stray local branch stays until someone deletes it by hand or reruns `finalize cleanup`. Confirmed by reading the code; the docs were reworded so they no longer promise a retry. Workaround: rerun `docket finalize cleanup --id <id>`. Suggested next action: add a local-ref presence leg to the sweep for done changes. No existing change fits (checked 22). A human captures it as a new change.
- **Groom cannot clear `stacked_on`.** When groom refuses a stacking conflict, the message can only tell the user to remove `stacked_on` by other means, because `change.groom` can set the field but not clear it. Impact: unstacking a change needs a hand edit. Confirmed. Suggested next action: let `change.groom` clear `stacked_on` (three-state, like `branch_prefix`). No existing change fits (checked 22). A human captures it as a new change.
- **Possible overlapping-edit refusal in groom's first patch pass.** On a record missing both `updated:` and `auto_groomable:`, groom's first pass could upsert both keys in one patch set and hit an overlapping-edit error. Suspected, not reproduced; current records carry `updated:`. Suggested next action: confirm with a fixture and route those writes through the per-field upsert this change used. No existing change fits (checked 22). A human captures it as a new change.
