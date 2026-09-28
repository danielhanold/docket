<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0467 — Scoped gate starts inherit the run epoch; thread it through the build chain](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0467-document-run-epoch-in-the-docket-build-task-gate-drive-start.md)**
<!-- docket:backlink:end -->
# Scoped gate starts inherit the run epoch; thread it through the build chain — Results

**Human action:** None needed to merge. After merging, rebuild the installed `docket` binary (the usual post-merge reinstall), because the installed binary still has the old driver behavior until then.

## Outcome

During change 0461's run, a build worker's test-gate start was refused with `stale-run-epoch`. The worker only got through by guessing that it should pass `--run-epoch`. The cause had two parts. The "run epoch" (the id the parent's run gate prints when it arms a run) never reached the child agents, and the gate driver ignored the epoch pinned on a worker's recovery scope, looking only at the value the caller passed in.

This change fixes both:

- **Driver.** A scoped gate start now inherits the run epoch its scope was prepared with, so a worker that passes no epoch is admitted. A start that presents a *different* epoch is still refused. The build-owner fast-path check in the app layer (`startBudgetedBuild`) now uses the same effective epoch. Before this, it could refuse a start that the real admission check would have allowed.
- **Skill prose.** `docket-implement-next`, `docket-build`, and `docket-build-task` now say where the epoch goes: it is passed as `--run-epoch` to every `prepare-scope` and to every build-owned suite start. Ordinary task workers never handle it. The one exception is the repair worker, which is handed the epoch for its build-owned re-run after a fix.
- **Parent instructions.** The managed run-gate block in `AGENTS.md` (and `cursor-rules/run-gate.md`) now tells the parent to copy the `<epoch>` into the implement-next dispatch prompt. On Codex, the dispatch prompt is the `agent.enter` request file, and that route now carries the epoch too.
- **Guards.** New repoguard tests pin each of these threads, and each one was mutation-tested. The `AGENTS.md` dispatch-block word budget was re-baselined from 1137 to 1153, which is still below its 1156 ceiling.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed through the build-owned gate at the Task 3 head: 66/66 files. A final certification run at the PR head is recorded in the PR's build-evidence block.
- Whole-branch deep review returned 1 blocker, 2 important, and 1 minor finding. All four were fixed on the branch (commits `7fb799d08`, `6c9226f54`, `18db4ba15`, `c7a4c8e29`), and each fix's worker ran focused tests. No second review round was run, per policy.
- The budget report printed `PARALLEL-SENSITIVE` / `BUDGET WATCH` screening lines for existing slow integration tests. None of those tests is touched by this change, and no serial-confirmed breach was reported.

## Known issues and follow-ups

### Dispatch-block word budget is nearly full

The `AGENTS.md` dispatch block is now 1153 words against a hard ceiling of 1156. The next change that adds wording to that block will have to trim something first. This is confirmed, has no runtime impact, and only affects authors. Suggested next step: trim the block the next time it is edited.

### Installed binary lags until reinstall

Until the post-merge reinstall, the installed `docket` binary still refuses epoch-less scoped starts. Any run that happens before then needs `--run-epoch` passed explicitly, as this run did. Rebuilding per the repo's post-merge rule removes the issue.
