---
id: 502
slug: 'align-the-skills-and-agent-files-with-the-docket-binary'
title: 'Align the skills and agent files with the docket binary'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [464]
stacked_on:
related: [363, 371]
discovered_from: [464]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Split out of #464 at grooming (2026-10-04): #464 aligns the human-facing docs with the docket binary; this change does the same for the agent-facing files — the skills, their references and templates, the agent wrappers, and `AGENTS.md`. Editing these changes what agents actually do, so it is reviewed separately and built after #464 lands.

A read-only audit (main @ `20bc0a36a`) found the skills still describing behaviour the binary removed, refuses, or never provided:

- **Configuration values that never arrive.** Skills read `SKILL_BRAINSTORM` / `SKILL_PLAN` / `SKILL_BUILD` / `SKILL_REVIEW`, `DUMMY_MODE_*`, `AUTO_CAPTURE_*`, `REVIEW_MIN_FIX_SEVERITY`, `REVIEW_MAX_FIX_TASKS`, `GATE_OBSERVATION_BUDGET`, and `BUILD_CHECKPOINT` "from the startup-check export". `repository prepare` exports none of them (its `skills` context is always empty). Every `auto` / custom-binding branch is dead because any explicit `skills.*` value blocks every write, so each `auto-or-halt` collapses to halt.
- **Refused features presented as working:** skill rebinding and the `auto` sentinel, dummy mode (a convention section, the whole `dummy-mode.md` reference, and six consumers), the repo-wide `auto_groom` knob, terminal publish, `build.checkpoint`, `finalize.skip_results_only_delta`, `finalize.gate: ci|both`, and repo-layer `agents:` pins. The convention's `.docket.yml` sample blocks every write if copied.
- **Names that do not exist:** `stack-base.sh`, `disable-worktree-hooks.sh`, `scripts/<name>.md` contracts, `verify-run`, `fm_field`, `reclaim-claims`, `run-halt`, the health checks `publish-deferred`, `adr-unpublished`, `stack-invalid`, `stack-parent-killed`, the status tokens `board off`, `promote-failed`, `stack-carried-failed`, and hand `git worktree add` / `gh pr edit` steps that typed operations now own.
- **A configurable metadata branch:** about 25 sites treat `metadata_branch` / "docket-mode" / "repo-mode" as a variable. There is one layout: the `docket` branch through the `.docket/` worktree.
- **Wrong facts:**
  - The convention says config is read from `origin/HEAD`, that `origin/HEAD` is repaired, and that `auto` falls back to `main`. In fact config comes from the primary worktree's files, and an unresolvable remote HEAD is an error.
  - Learnings are written by `learning.record` / `learning.update`.
  - `attach-plan` accepts only `docs/superpowers/plans`.
  - `adr-template.md` lacks the *Alternatives considered* section that `adr.record` requires.
  - `change-template.md` has drifted from what `change.create` writes.
  - `## Run halted` is written by `change.halt`, and `change.resume-halted` also clears it.
  - Agent wrappers are user-level only, and no per-repository wrapper is generated.
  - `docket install check` is machine-only.
- **Four real bugs caused by the prose:**
  1. **auto-groom dirties `.docket/`.** It drafts its spec under `.docket/docs/superpowers/specs/`, so the next `repository prepare` refuses with `metadata-worktree-dirty`. The draft belongs in the request file, as in groom-next.
  2. **docket-review raises a false blocker.** It demands `result: green` evidence, so under the supported `build.gate: off` it reports `unverified-build-state` against implement-next's accepted `skipped` / `build-gate-off` evidence.
  3. **Finalize contradicts itself.** Finalize step 5, `gate-failure.md`, and the integration-repair wrapper re-gate a repaired head with raw `gate.launch` + `gate.observe` + `evidence.record --run`. `gate-caller-loop.md` and docket-build say no workflow caller composes raw gate verbs.
  4. **Finalize's sign-off advice is wrong.** `docket-finalize-change/references/gate-failure.md` (*Sign-off on auto-authored repairs*) says the human re-runs finalize and the retry clears a `repair-needs-signoff` block. The SKILL.md says the block needs an explicit human `finalize.clear-block` first, and a plain re-run does not clear it.
- **Retired behaviour found while building #464** (not in the original audit):
  - docket-status describes stale-claim and dependency-stall health checks the binary no longer runs.
  - The convention still documents `learnings.cap` and an active-findings cap on the ledger.
  - The convention and `agent-layer.md` describe merging `skills:` across config layers, and a legacy `agents.yaml` auto-migration.
- **About 85 citations of individual changes** (implement-next 10, convention about 45, edge-paths 5, fix-loop 5, docket-brainstorm 5, and others) plus "deferred from Go v1" history narration.

## What changes

Apply #464's alignment rules to `skills/` (all twelve skills, their references and templates), `agents/` wrappers, and `AGENTS.md`:
- describe only current behaviour;
- removed or refused things never appear;
- no citations of individual changes or PRs;
- ADR citations are fine where the decision is still current;
- "loop" is reserved for `/loop`.

- **Roles are fixed defaults.** brainstorm `superpowers:brainstorming`, plan `superpowers:writing-plans`, build `docket-build`, review `docket-review`, finish `superpowers:finishing-a-development-branch`.
  - Replace every `SKILL_*` read with the named default.
  - Delete the `auto` sentinel, custom-binding branches, and the "explicit `auto` authorizes inline" clauses; an undispatchable required dispatch halts.
  - docket-brainstorm is reached only by a human's per-run request.
  - Keep "role binding is a skill invocation", autonomy precedence, and role self-description where they still say something true.
- **Delete dummy mode** everywhere: the convention section, `references/dummy-mode.md`, the six consumers, and finalize's tombstone paragraph.
- **Delete other refused-feature prose.** That covers auto-capture exports and narration, terminal-publish narration and step 3 of `close-out.md`, the `build.checkpoint: true` ledger mode, the `skip_results_only_delta` paragraph, and the repo `auto_groom` knob. Auto-groom selects only stubs carrying `auto_groomable: true`.
- **Rewrite the convention's config sample and config prose from the schema.** Use supported keys only, and state that agent pins are global-only. Read review and gate values from where they actually live: `diagnostic.config`, prepare's `build` context, and the gate driver itself.
- **Replace Bash-era names with the typed operations:**
  - `workspace.prepare`
  - `finalize.retarget-children`
  - `run.verify` / `run.verdict`
  - `change.halt` / `change.resume-halted`
  - `learning.record` / `learning.update`
  - readiness `stack-base-unresolved`
  - the `change-stack-cycle` finding and the effective-base kinds
  - `internal/gitcli` hooks-off via `repository prepare`
- **Metadata branch.** Replace `metadata_branch` / docket-mode / repo-mode wording with the `docket` branch and the `.docket/` worktree.
- **Fix the three bugs** listed in *Why*. Fix `adr-template.md` and `change-template.md` to match what `adr.record` and `change.create` write.
- **Rename "fix loop" to "fix pass".** Rename `docket-implement-next/references/fix-loop.md` to `fix-pass.md`, update its wording, and update every link to it, including the docs link #464 leaves.
- **Agent layer.** Rewrite `references/agent-layer.md` around user-level wrappers, the compiled built-in table, global-only overrides, and `agent_harnesses` as the dispatch-surface opt-in. Delete per-repository generation, the Bash validator, the generic emitter, and the three-leg drift gate. Fix `agents/docket-plan-writer.md` (no `SKILL_PLAN`; plans live under `docs/superpowers/plans`).
- **Citations.** Remove the change citations, including `AGENTS.md`'s "since change 0392" bullet; `AGENTS.md` edits are authorized at grooming by the human.
- **Test guards.** Update every repo test that pins skill wording in the same commit as the prose it guards: `skill_handoff_sites_test`, the retired-vocabulary rows, `TestProseContracts`, `budgets_test`, and the per-skill rows. Regenerate the embedded assets.

Decide at grooming:
- **Finalize re-gate contradiction.** Which re-gate path is correct for a repaired head: raw gate verbs, or a driver path? Fix the losing side.
- **Ad-hoc dummy mode.** Should the in-session "human asks for plain language" behaviour survive in any form once dummy mode is deleted?
- **`gate-execution-evidence.md`.** It is historical probe evidence that is never read before a gate run. Should it leave the skill bundle?

## Out of scope

- The human-facing docs, `.docket.example.yml`, `agents/harness-defaults.yml` comments, and dead `scripts/runners/` — that is #464.
- Changing what the binary does, including exporting the values the skills used to read, or removing the refused/obsolete rows from the config schema.
- Point-in-time records (archived changes, results, Accepted ADRs, specs, plans, frozen fixtures under `testdata/` and `internal/install/legacydata/`).
- Generated `CLAUDE.md` / `AGENTS.md` dispatch blocks, beyond what their generators already produce.
