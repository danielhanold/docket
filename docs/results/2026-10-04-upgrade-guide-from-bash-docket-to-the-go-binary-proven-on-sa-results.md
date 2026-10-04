<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0511 — Upgrade guide from Bash docket to the Go binary, proven on saved v0.9.2 and v0.9.3 installs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0511-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa.md)**
<!-- docket:backlink:end -->
# Upgrade guide from Bash docket to the Go binary, proven on saved v0.9.2 and v0.9.3 installs — Results

**Human action:** Read the guide once as a Bash user would before alpha.1 ships, and note one wording change for the release work: the proof test is named `TestIntegrationBashUpgrade`, not `TestBashUpgrade`. Merging needs nothing else.

## Outcome

Bash docket users on v0.9.2 or v0.9.3 whose repositories already use the `docket` branch now have a written upgrade path. The guide is `docs/release/upgrading-from-bash.md`, and `README.md` and `docs/install/install.md` both link to it. It covers:

- installing the Go binary;
- taking over the old Claude Code install;
- upgrading each repository;
- a table of `.docket.yml` settings that changed;
- leftovers to delete;
- restarting Claude Code.

A test proves the guide. Saved copies of real Bash state live in `testdata/bash-upgrade/v0.9.2/` (628 KB) and `testdata/bash-upgrade/v0.9.3/` (724 KB). They were built from the actual tags in a sandbox. Each holds a git bundle of the repository, the home folder's harness files, and a `PROVENANCE.md` saying how it was made. The v0.9.2 case was bootstrapped with an empty orphan `docket` branch. The v0.9.3 case was bootstrapped through `migrate-to-docket.sh` and seeded from main. Both pass the binary's ownership check for a `docket` branch that has no ownership receipt, so the riskiest unknown in the spec did not turn out to be a defect.

The test lives in a new package, `internal/bashupgrade`, with its own shard `tests/test_go_integration_bashupgrade.sh` and a 45s budget. It reads the guide's `<!-- upgrade-step: … -->` markers and runs or mirrors each marked step against both cases. It fails if any of these happen:

- a marked step has no test action;
- an unmarked block runs `docket`;
- the guide's finding or settings tables list something no case reported, or leave out something a case did report.

It ends each case with a clean `install check` and a clean `repository check`, no errors from `status`, every original record intact, and one successful `change create`.

Where the build departed from the spec:

- **Test name prefix.** The tests are named `TestIntegrationBashUpgrade*`, because the integration-test contract rejects any other prefix for integration-tagged tests. Change 0366's release gate must use this name.
- **Install conflict on both tags.** Taking over the install hits an ownership conflict on both tags, not only v0.9.3. Every Bash skill symlink into the checkout conflicts. v0.9.3 also conflicts on `docket-plan-writer.md`. The guide's remedy deletes those links and the file, then runs the download command again. That rerun is needed because a failed install leaves no `docket` on `PATH`.
- **Steps added to the guide.** The guide gained these steps:
  - remove the `runtime.bash` block from the global config;
  - rewrite the `.gitignore` block, then commit and push;
  - run `repository prepare` and `check` again after `repair`;
  - remove the Bash dispatch block from the repository's `CLAUDE.md`;
  - delete the per-repository `.claude/agents/docket-*.md` files.
- **Facts the guide spells out.** `docket status` reads `.docket.yml` from the pushed default branch, not from your working copy. A clean `repository prepare` does not mean the repository is healthy.

## Human actions and testing

### Important — Read the guide as a Bash user would

The test proves every command in the guide, but not whether a person can follow it. The guide is the only upgrade help these users get.

Prerequisites: a checkout of this branch.

1. Open `docs/release/upgrading-from-bash.md` and read it top to bottom, as a v0.9.3 user would.
   Expected: every step says where to run it (your home folder or one repository) and what you should see afterward. No step asks you to hand-edit the `docket` branch.
2. Check the settings table in section 6 against what you remember of old configs.
   Expected: each old setting is marked as an error, a warning or a notice, and has a fix.

### Optional — Run the proof yourself

Prerequisites: a Go toolchain and this branch checked out.

1. From the repository root, run `bash tests/test_go_integration_bashupgrade.sh`.
   Expected: it exits 0 in about 20 seconds, and both the v0.9.2 and v0.9.3 cases pass.

## Verification performed

- Whole suite, using the build test command (`go run ./cmd/docket development test`): green before review. It was rerun after the review fixes, and the PR's evidence block records the result. Budget report: there were no `SERIAL CONFIRMED OVER BUDGET` or `BUDGET WATCH` lines. Three `PARALLEL-SENSITIVE` lines were for unrelated shards (`test_go_finalize_e2e.sh`, `test_go_integration_app_merge.sh`, `test_go_race.sh`).
- The new shard was timed twice on its own: 20.62s and 20.35s. The budget is set to 45s.
- Mutation check 1 (spec): removing the `configure-tests` step from a copy of the guide turned the test red. It was red both with the registry entry kept (the shape guard caught it) and with the entry removed (both cases failed on `test-config-missing`).
- Mutation check 2 (spec): removing the takeover remedy turned the v0.9.3 case red. It also turned the v0.9.2 case red, because that case conflicts too. The run fails at `confirm-install` with exit 127, since no binary is installed.
- More mutation checks during the build and the fixes also turned the test red:
  - the ownership check refusing a foreign `docket` root (both bootstrap kinds);
  - removing rows from the finding and settings tables;
  - removing the step that deletes the dispatch block or the agent files;
  - removing each command shape from the guard that catches unmarked blocks running `docket`.
- What each case reported before the upgrade:
  - `repository check` on v0.9.2: `postconditions-unmet`, `committed-ignore-invalid`, `board-stale`, `artifact-links-stale` (twice) and `test-config-missing`.
  - v0.9.3: the same, plus `legacy-config-key-present`.
- Whole-branch review (deep tier): 7 findings (3 important, 4 minor), all fixed in-branch. The full table is in the PR body.
- Backlog check: 23 proposed or deferred changes were checked.

## Known issues and follow-ups

### Parts of the guide the test does not prove

- The real `curl` download and the `shasum` check are only checked for shape. Change 0366's public-install check proves them against the real release.
- Restarting Claude Code cannot be observed by the test.
- `finalize.gate: both` is listed as an error, but no saved case sets it. Only `ci` was observed.
- Whether `.claude/settings.local.json` is safe to delete is not covered. The guide says so.
- The two v0.9.3 per-repository files (the agent wrappers and `settings.local.json`) were regenerated from the tag rather than copied from the original sandbox. `PROVENANCE.md` records this.

Suggested next action: none for this change. Change 0366 carries the release-side checks.

### `repository check` calls a branch "diverged" when it is only behind

After `repository repair --yes`, the local `.docket` copy is one commit behind `origin/docket`. Until you run `repository prepare` again, `repository check` reports `local-metadata-diverged` and tells you to "reconcile with a human", even though nothing has diverged. The guide avoids this by running `prepare` again right after `repair`, so users following it are not affected. This is confirmed, and the impact is a misleading message. Suggested next action: a human could capture a small fix so `check` reports a branch that is only behind as behind. No existing change fits (checked 23); capture a new change.

### Retire the saved cases when stable v1.0.0 ships

`testdata/bash-upgrade/`, `internal/bashupgrade/`, the shard and its budget row exist only until stable v1.0.0. Retiring them must use the real test name, `TestIntegrationBashUpgrade`. Fits #514 (deferred): revive it when v1.0.0 ships, and update its "What changes" to the real test name and package.
