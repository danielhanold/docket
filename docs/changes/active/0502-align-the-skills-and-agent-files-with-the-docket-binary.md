---
id: 502
slug: 'align-the-skills-and-agent-files-with-the-docket-binary'
title: 'Align the skills and agent files with the docket binary'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [464]
stacked_on:
related: [363, 371, 505, 248, 360, 366, 412]
discovered_from: [464]
adrs: [18, 22, 81, 99, 130]
spec: 'docs/superpowers/specs/2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary-design.md'
plan: 'docs/superpowers/plans/2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/align-the-skills-and-agent-files-with-the-docket-binary'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T12:11:05Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary-design.md) |
| Plan | [2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary.md](https://github.com/danielhanold/docket/blob/fix/align-the-skills-and-agent-files-with-the-docket-binary/docs/superpowers/plans/2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary.md) |
| ADRs | [ADR-0018](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0018-pluggable-skills-passthrough-degrade.md), [ADR-0022](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0022-consultant-authored-brainstorm.md), [ADR-0081](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0081-gate-run-contract-narrowed-per-platform-process-group-where-no-session-primitive-exists.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md), [ADR-0130](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0130-build-task-workers-run-focused-tests-directly-under-a-fixed.md) |
<!-- docket:artifacts:end -->

## Why

Split out of #464 at grooming (2026-10-04). #464 aligned the human-facing docs with the docket binary; this change does the same for the agent-facing files: the skills, their references, the agent wrappers, and `AGENTS.md`. Agents follow this text literally, so changing it changes what agents do. That is why it is reviewed on its own and built after #464.

A read-only audit (main @ `20bc0a36a`, re-verified at main @ `ecbb32f19`) found the skills still describing behaviour the binary removed, refuses, or never provided:

- **Configuration values that never arrive.** Skills read `SKILL_*`, `DUMMY_MODE_*`, `AUTO_CAPTURE_*`, `REVIEW_MIN_FIX_SEVERITY`, `REVIEW_MAX_FIX_TASKS`, `GATE_OBSERVATION_BUDGET`, and `BUILD_CHECKPOINT` "from the startup-check export". `repository prepare` exports none of them; its `skills` object is always empty. Any explicit `skills.*` value blocks every write, so every `auto` / custom-binding branch is dead.
- **Refused features presented as working:** skill rebinding and the `auto` sentinel, dummy mode, the repo-wide `auto_groom` setting, terminal publish, `build.checkpoint`, `finalize.skip_results_only_delta`, `finalize.gate: ci|both`, repo-layer `agents:` pins, and the learnings cap. The convention's `.docket.yml` sample blocks every write if copied.
- **Names that do not exist:** `stack-base.sh`, `disable-worktree-hooks.sh`, `scripts/<name>.md` contracts, `verify-run`, `fm_field`, `reclaim-claims`, several health checks and status tokens, and hand `git worktree add` / `gh pr edit` steps that typed operations now own.
- **A configurable metadata branch:** about 40 sites treat `metadata_branch` / "docket-mode" as a variable. There is one layout: the `docket` branch through the `.docket/` worktree.
- **Wrong facts:**
  - where configuration is read from;
  - who writes learnings;
  - where plans must live;
  - who writes and clears `## Run halted`;
  - per-repository agent wrappers (they are user-level only);
  - what `docket install check` covers;
  - health checks docket-status no longer runs.
- **Four real bugs caused by the prose:**
  1. auto-groom drafts its spec inside `.docket/`, so the next `repository prepare` refuses with `metadata-worktree-dirty`.
  2. docket-review raises a false `unverified-build-state` blocker under the supported `build.gate: off`.
  3. Finalize describes two different ways to re-test a repaired branch, and one of them breaks the gate rules.
  4. Finalize tells the human that re-running it clears a repair sign-off block. It does not: the human must run `finalize clear-block` first.
- **About 55 change citations and 34 "Go v1" history narrations** across the skills, plus one in `AGENTS.md`.

## What changes

Bring `skills/` (all twelve skills and their references), `agents/*.md`, and `AGENTS.md` in line with the binary under #464's rules:
- describe only current behaviour;
- removed or refused things never appear;
- no change or PR citations;
- ADR citations only where the decision is still current;
- "loop" means `/loop` only.

The linked spec carries the verified facts and the full worklist.

- **Roles are fixed defaults.** brainstorm `superpowers:brainstorming`, plan `superpowers:writing-plans`, build `docket-build`, review `docket-review`, finish `superpowers:finishing-a-development-branch`. The `$SKILL_*` reads, the `auto` sentinel, and custom bindings go. An undispatchable required dispatch halts. A skill that is not installed is done inline with a prominent warning. docket-brainstorm runs only when a human asks for it.
- **Deleted:**
  - dummy mode, with no replacement for in-session plain-language requests;
  - the other refused features;
  - the convention's unsupported config-sample keys;
  - `adr-template.md` and `change-template.md` (the Go renderers are the only writers; the convention's record and ADR blocks are corrected to match them);
  - `gate-execution.md`, `gate-execution-evidence.md`, and docket-build's "read it now" pointer to them. The run-boundary probe scenarios move to `docs/release/four-harness-acceptance.md`.
- **Replaced:**
  - Bash-era names become the typed operations;
  - `metadata_branch` wording becomes the `docket` branch and the `.docket/` worktree;
  - `fix-loop.md` becomes `fix-pass.md`, and `gate-caller-loop.md` becomes `gate-driver.md`;
  - the wrong facts are corrected;
  - `references/agent-layer.md` is rewritten around user-level wrappers and global-only overrides.
- **The four bugs:**
  1. auto-groom keeps its draft in the request file, never in `.docket/`.
  2. docket-review accepts `skipped` / `build-gate-off` evidence when `build.gate` is `off`.
  3. Finalize re-tests a repaired head through the gate driver (`gate drive start --owner finalize` → advance → `evidence record`), never raw gate verbs.
  4. Repair sign-off is the human running `finalize clear-block`, then re-running finalize. This is fixed in both SKILL.md step 6 and `gate-failure.md`.
- **Guards:**
  - #464's `TestLivingDocsAlignment` is extended to `skills/`, `agents/`, and `AGENTS.md`. The key check skips YAML frontmatter. Mutation-tested.
  - `TestSkillHandoffSites` is re-keyed from `$SKILL_*` to the fixed role-skill names.
  - Every pinned prose and budget row is updated in the same commit as its text, and the embedded assets are regenerated.
- `AGENTS.md` edits were authorized by the human at grooming.

## Out of scope

- The human-facing docs, `.docket.example.yml`, `agents/harness-defaults.yml`, and `scripts/runners/`. #464 handled those. The one exception is `docs/release/four-harness-acceptance.md`, which receives the run-boundary probe scenarios.
- Changing what the binary does. That includes exporting the values the skills used to read, and removing the obsolete or refused rows from the config schema.
- Point-in-time records: archived changes, results, Accepted ADRs, specs, plans, and frozen fixtures under `testdata/` and `internal/install/legacydata/`.
- Generated `CLAUDE.md` / `AGENTS.md` dispatch blocks, beyond what their generators already produce.
- Adding new retired-vocabulary rows. The extended living-docs guard is the drift guard here.
- Sharing the unsupported-key matcher between the two guards. That is #505.

## Reconcile log

### 2026-10-04

2026-10-04 — Reconciled at main @ fc0e2e440. #464 is done (dependency satisfied). #505 is done: the living-docs guard's key matcher now lives in internal/config (UnsupportedKeyShapes over SettingPaths), so the guard extension builds on that shared matcher. No scope change; the spec's worklist stays a set of hypotheses verified at build time.
