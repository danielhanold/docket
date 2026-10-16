<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0554 — Refresh the shipped Claude and Cursor model defaults; drop the interactive-skill model advice](../changes/archive/2026-10-16-0554-update-cursor-review-tier-defaults-to-grok-4-7.md)**
<!-- docket:backlink:end -->

# Refresh the shipped Claude and Cursor model defaults; drop the interactive-skill model advice — Results

**Human action:** Yes, one important check: confirm the new model IDs resolve in Claude Code and Cursor after installing. No repository test can check vendor model IDs.

## Outcome

Docket's built-in model defaults now match the current model roster. On Claude, every agent runs `claude-opus-5-5` (effort sets the tier) except `docket-status`, which runs `claude-haiku-5-5`; `implement-next` keeps its 1-hour cache pin. On Cursor, most agents run Grok 4.6 at the level in the ID, the three review tiers run `grok-4.7-low/medium/high`, and `status`, `plan-writer` and `build-max` run Claude models through Cursor. Every Cursor effort stays `auto`. The same values are in `agents/harness-defaults.yml`, the Go built-in table, the commented example in `.docket.example.yml`, and the regenerated embedded copies. Comments that the new values made untrue were rewritten, and the "review-deep equals build-max" rule is gone. The validation doc's list of Cursor wrappers allowed to carry a Claude model ID now names build-max, plan-writer and status.

The two interactive skills (`docket-new-change`, `docket-groom-next`) no longer recommend a session model. The convention and glossary no longer mention that advice, and `docs/install/models-and-effort.md` now says, without naming any model, to pick the session model by preference and by how complex the change looks. ADR-0008 got a dated update note recording this.

One fix outside the spec's scope: the first full-suite run failed on `TestObserveRunningThenTerminal` in `internal/process`, a timing-sensitive test unrelated to this change. The test assumed one group-wide kill ends the supervisor before its command; on macOS under load the command can die first, so the supervisor records a result and the test waits forever. The test now kills the supervisor first, waits for it, then kills the group. Only the test changed.

## Human actions and testing

### Important — confirm the new model IDs resolve

Model IDs are vendor facts; docket treats them as opaque, so no repository test can tell whether `claude-opus-5-5`, `claude-haiku-5-5`, `grok-4.7-*` or `claude-haiku-5-5-thinking-medium` actually exist in your harness. If one is wrong, that agent fails to start or silently runs a different model. Check after this change merges and the binary is rebuilt.

Prerequisites: the merged binary installed (`docket development install --source /Users/homer/dev/docket`), and no global-config agent pins that override these agents.

1. Run `docket install`, then start a fresh Claude Code session and dispatch `docket-status`.
   Expected: it runs on `claude-haiku-5-5`; other docket agents (for example `docket-adr`) run on `claude-opus-5-5`.
2. With `cursor` in `agent_harnesses`, run `docket install` and open Cursor's agent picker.
   Expected: each docket agent shows its new model, in particular `docket-review-lean/standard/deep` on `grok-4.7-low/medium/high` and `docket-status` on `claude-haiku-5-5-thinking-medium`.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head after the review fixes. The first build run failed on the unrelated `internal/process` timing test described above; it went green after the repair. One later run was interrupted by the gate driver (owner superseded while another worktree's suite ran) with the suite itself exiting 0; it was re-run to get clean evidence.
- The YAML-to-Go parity test and the `.docket.example.yml` mirror guards pass. The example's mutation probe now targets `{ model: claude-haiku-5-5,` and still fails when that value is changed.
- The repair was checked under CPU load: the old test failed in 2 of 4 batches of 300 runs, the fixed one passed 1,800 runs.
- Repo-wide grep over maintained source finds the old IDs only in frozen data, tests' own input tables, archived records and the allowed override sample in the agent-layer reference.
- Whole-branch review (deep tier): 2 minor findings, both fixed in-branch; full table in the PR body.

## Known issues and follow-ups

- **A sibling test uses the same racy kill.** `TestRecoverMarksCleanlyAbandonedOwnedRun` in `internal/process/recover_test.go` kills the whole process group in one step, like the test repaired here. Under heavy load on macOS it could hit the same supervisor-first ordering; today its setup retries three times, so it has not been seen failing. Suspected, not confirmed. Suggested next action: apply the same supervisor-first kill. Fits #527 (its test-suite hygiene list already covers tests that race process timing) — edit it through `docket-groom-next 527` (revise).
- **`test_go_integration_app_reposetup.sh` over its runtime budget.** One gate run reported `SERIAL CONFIRMED OVER BUDGET` (59s solo vs a 52.5s threshold) while another worktree's suite was running, so the reading may be contended. Not caused by this change, and it does not fail the suite. Suggested next action: Fits #527 (it already moves reposetup's private-mode tests into their own shard and re-measures the row) — edit it through `docket-groom-next 527` (revise) if the new number matters; related to #528 (contended solo re-checks).
- **#166 is obsolete.** It would have retuned the advisory recommendation this change removes; its record is already archived, so nothing is needed.
