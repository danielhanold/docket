<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0477 — Finish the run-tracker rename (--gate-context, DOCKET_AGENT_GUARDIAN_GATE_KEY, dispatch_context)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-30-0477-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex.md)**
<!-- docket:backlink:end -->
# Finish the run-tracker rename (--gate-context, DOCKET_AGENT_GUARDIAN_GATE_KEY, dispatch_context) — Results

**Human action:** Yes, at landing. Merge only while no dispatched implement-next run is in flight, rebuild the installed `docket` binary right away, restart coordinator sessions, and re-run `docket install` in consumer repositories. No other action is required.

## Outcome

Change 0471 renamed the run gate to the run tracker but left three old spellings behind. This change renames them as a hard cut, with no aliases (ADR-0129 rows 38e-38h):

- `docket gate drive start` and `docket gate drive prepare-scope` now take `--run-context` instead of `--gate-context`. This is the same flag name `change claim` already uses, so the run-context token printed by `run start` goes to both commands under one name. The old flag is now simply an unknown flag.
- The guardian process environment variable is now `DOCKET_AGENT_GUARDIAN_RUN_KEY`. Only the docket binary sets and reads it.
- The `run start` JSON result now reports the token under `run_context` instead of `dispatch_context`. The text line `run-started <key> <run-id> <run-context>` is unchanged.
- Go comments and test text that said "outer gate" or "gate context" when they meant the run tracker now use run-tracker wording. The checkpoint meaning of "gate" is unchanged.
- The skills (docket-build, its gate-caller-loop reference, docket-build-task, docket-implement-next), their embedded copies, the generated Codex dispatch sentence and the AGENTS.md managed block all now pass `--run-context`.
- The retired-vocabulary seal no longer has a special case for keeping the gate drive's `--gate-context`. Row 12 is now a plain row that also covers 38e, and rows 38f and 38g are new. About 200 lines of special-case scanner code were deleted. Row 38h (`rungate store`) was already covered by row 38 and is recorded only.

Nothing persisted used the old names, so unlike 0471 there is no storage reset. The committed claim-receipt key `gate_context_hash` is deliberately unchanged and still legal under the seal.

One departure from the plan: the new dispatch sentence is one word longer, so the AGENTS.md dispatch-block word budget in `internal/repoguard/budgets_test.go` was raised from 1153 to 1154. That is still below the prior ceiling.

## Human actions and testing

### Important — land with no run in flight, then rebuild and restart

A dispatched run whose loaded skill text still passes `--gate-context` fails against the new binary with "unknown flag". A coordinator session that loaded the old AGENTS.md dispatch block will also keep writing the old label into dispatch requests until it restarts.

1. Before merging, confirm no `docket-implement-next` run is active. Run `docket status` and, for each change it lists as `in-progress`, run `docket run verify --id <id>`. As a secondary check, `ps aux | grep "docket gate drive" | grep -v grep` should print nothing. A run can be active without a gate-drive process (while it plans or reviews), so the status check is the one that counts.
   Expected: no change is `in-progress` under a live run, and no gate-drive processes.
2. Merge the PR, then follow the repository's "Rebuild the binary after a merge to main" procedure (`repository.sync-integration`, then `development.install --source /Users/homer/dev/docket`).
   Expected: `docket version` reports the merged `main` HEAD.
3. Run `docket gate drive start --help`.
   Expected: it lists `--run-context` and does not list `--gate-context`.
4. Restart any coordinator sessions, then run `docket install` in each consumer repository.
   Expected: the regenerated dispatch block reads "labeled for `--run-context` on `change.claim` and the gate drive".

## Verification performed

- Each of the five plan tasks ran focused tests test-first through the gate driver. The seal was mutation-tested: restoring the old spelling in a skill argv line, the CLI flag literal, the env constant, the struct tag, and the generator's dispatch sentence each turned the seal red and named the replacement. Weakening the token boundary turned the `gate_context_hash` negative controls red, which proves they work.
- A residual grep finds the old spellings only in test lines that assert they are refused or retired.
- The whole suite (`build.test_command`) passed at the build gate: 74 of 74 files. The certifying run's evidence is in the PR body. The budget report showed only parallel-screening lines (`BUDGET WATCH` / `PARALLEL-SENSITIVE`) in unrelated integration and toolchain tests, with no serially confirmed breach.
- A whole-branch review returned 5 findings: 1 important and 4 minor. All five were fixed in-branch.
  - The run-context wiring test now requires the specific `scope-identity-mismatch` refusal, so a busy worktree can no longer satisfy it. This was mutation-checked.
  - The unknown-run-id hint now names where the run context goes.
  - The seal also catches reads of `dispatch_context` by consumers such as `jq`, not just struct tags. This was mutation-checked.
  - An overlong comment was reflowed.
  - This file's landing check now uses the status surface.
