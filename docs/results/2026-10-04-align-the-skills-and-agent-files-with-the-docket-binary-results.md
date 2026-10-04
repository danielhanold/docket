<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0502 — Align the skills and agent files with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0502-align-the-skills-and-agent-files-with-the-docket-binary.md)**
<!-- docket:backlink:end -->
# Align the skills and agent files with the docket binary — Results

**Human action:** Recommended: read the finalize sign-off and configuration wording before merging, because agents follow these files literally. Two binary-level gaps found during the work are listed under Known issues for triage.

## Outcome

The agent-facing files (`skills/`, `agents/*.md`, `AGENTS.md`) now describe only what the docket binary does today. Agents follow this text literally, so the edits change agent behaviour:

- **Fixed role skills.** Skills no longer read `SKILL_*` values that `repository prepare` never exports. Each role names its skill directly (plan `superpowers:writing-plans`, build `docket-build`, review `docket-review`, brainstorm `superpowers:brainstorming`). An undispatchable required dispatch now halts; there is no `auto` fallback. Nothing invokes the finish role, so its row was dropped from the role table.
- **Deleted:** dummy mode, auto-capture narration, terminal publish, the `build.checkpoint` ledger, `finalize.skip_results_only_delta`, `finalize.gate: ci|both`, repository-layer agent pins, the learnings cap, the two record templates, and the two gate-execution references. The run-boundary probe scenarios moved to `docs/release/four-harness-acceptance.md`.
- **Renamed:** `fix-loop.md` is now `fix-pass.md`; `gate-caller-loop.md` is now `gate-driver.md`. "Loop" now means `/loop` only, and `metadata_branch` / "docket-mode" wording is now the `docket` branch and the `.docket/` worktree.
- **Four prose-caused bugs fixed:**
  1. auto-groom drafts its spec in scratch space and sends it as `spec_markdown`, so it no longer dirties `.docket/`.
  2. docket-review accepts `skipped` / `build-gate-off` evidence when `build.gate` is `off`.
  3. Finalize re-gates a repaired head through the gate driver (`gate.drive.start --owner finalize`), never raw gate verbs.
  4. Repair sign-off is the human running `finalize.clear-block`, then re-running finalize; the autonomous path publishes the repaired head before recording the block.
- **Configuration read paths are now stated accurately.** The committed `.docket.yml` is read from the fetched tip of origin's default branch by every operation outside the repository setup family; `.docket.local.yml` and the global file are read from disk; only the setup operations (`repository.prepare` and its siblings) read the primary worktree's files. An unresolvable remote HEAD under `integration_branch: auto` is an error.
- **Autonomous repair sign-off records the block before publishing.** The autonomous finalize path now runs `finalize.block` first and only then publishes the repaired head, so a failed block never leaves a green, unmarked repair on the PR. The human re-reads the record revision after the block lands before running `finalize.clear-block`.
- **Wrong facts corrected along the way:** the claim operation does remove a `## Run halted` section (Step 2 said it did not); a `stacked-merged` change does not satisfy `run.verify` (only `implemented` does); closeout does strip `## Finalize blocked`; an absent `agent_harnesses` writes no repository dispatch files (it is not a Claude-only default).
- **Guards:** `TestLivingDocsAlignment` now scans `skills/`, `agents/`, and `AGENTS.md` (frontmatter skipped for the key check only); `TestSkillHandoffSites` is keyed on the fixed role-skill names; a new whitespace-collapsed `TestAlignmentContracts` table pins every new prose claim.

## Verification performed

- Each of the 20 plan tasks ran its focused check (`./internal/repoguard/ ./internal/assets/ ./internal/harness/... ./internal/render/`) green, and each new or extended guard was mutation-tested: the planted defect reddened it and the restore turned it green.
- Acceptance sweep, whitespace-collapsed over `skills/**`, `agents/*.md`, `AGENTS.md`: no `SKILL_*`, `DUMMY_MODE_*`, `AUTO_CAPTURE_*`, `REVIEW_MIN_FIX_SEVERITY`, `REVIEW_MAX_FIX_TASKS`, `GATE_OBSERVATION_BUDGET`, `BUILD_CHECKPOINT`, `metadata_branch`, "docket-mode", "repo-mode", Bash-era name, or non-`/loop` "loop".
- Relative links in the in-scope files and `docs/release/four-harness-acceptance.md` all resolve. The only remaining references to deleted files are inside frozen top-level `testdata/` fixtures, which are point-in-time records.
- Whole-branch review (deep tier): 8 findings (1 blocker, 4 important, 3 minor), all fixed in-branch; full table in the PR body. The review also caught a guard gap: the living-docs guard blanked whole frontmatter, so it now blanks only the `skills` list and scans descriptions.
- Worklist items found not to be defects: `skills/docket-review` never mentioned a `skills.review` binding; the fix-pass task cap is the supported `review.max_fix_tasks`; `build_gate` / `build_test_command` / `build_max_attempts` in docket-build are implementation-context JSON fields, not config keys.

## Known issues and follow-ups

### Evidence for a finalize-owned re-gate is keyed on build configuration

When finalize re-gates a repaired head, `evidence.record` reads the build gate settings, not the finalize ones. Under `build.gate: off` it mints `skipped` evidence even after a passing finalize drive; a green record names `build.test_command` instead of the `finalize.test_command` that ran; and with `build.gate: local` but no build command it refuses even though the finalize drive passed. Confirmed by code reading, not observed in a run. It predates this change. Suggested next action: a binary change so `evidence.record` honours the drive's owner.

### Agents editing `.docket.yml` locally will not see the change until it lands on origin

Operations read the committed `.docket.yml` from origin's default branch, not the working copy. An agent or human who edits it locally and immediately runs a gate will get the old settings. This is existing binary behaviour, now documented. Workaround: commit and push the change to the default branch first, or use `.docket.local.yml` for machine-only settings.

### A named finalize id bypasses the repair sign-off marker in the binary

`finalize.merge` lets an explicit change id through even while a repair sign-off block is present. The rule that a named id never overrides sign-off is enforced only by the skill text. Confirmed by code reading. Suggested next action: decide whether the binary should refuse it.

### Stale code comments about a repository `auto_groom` default

Comments in `internal/domain/entities.go` and `internal/app/change_create.go` still say an unset `auto_groomable` inherits the repository's `auto_groom`. Comments only; behaviour is per-change. Suggested next action: reword in a later code change.

### `TestSkillHandoffSites` treats "cannot be invoked" as an invocation

The handoff guard's negation pattern has no word boundary inside "cannot", so a line naming a role skill next to "cannot be invoked" needs a `DIRECTED to:` marker. The skill text was worded around it. Suggested next action: tighten the regex if it recurs.
