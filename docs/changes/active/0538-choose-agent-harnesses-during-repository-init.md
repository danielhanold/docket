---
id: 538
slug: 'choose-agent-harnesses-during-repository-init'
title: 'Choose agent harnesses during init and with configure-harnesses'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-07'
updated: '2026-10-07'
depends_on: []
stacked_on:
related: [531, 533, 351]
discovered_from: [535]
adrs: []
spec: 'docs/superpowers/specs/2026-10-07-choose-agent-harnesses-during-repository-init-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-07-choose-agent-harnesses-during-repository-init-design.md](../../superpowers/specs/2026-10-07-choose-agent-harnesses-during-repository-init-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository init` writes the parent-facing dispatch instructions only when the repository itself declares `agent_harnesses`. A global setting never authorizes repository writes (change 0351). But nothing ever asks for the setting, and nothing says when it is missing.

The result is the same in both modes: no instructions and no message saying why. A shared repository gets no `CLAUDE.md`/`AGENTS.md` block, and a private one gets no `.git/dckt/AGENTS.md`. Agents in that repository then run docket workflows inline and untracked. Fixing it today means hand-editing the config (`.docket.yml`, or `.git/dckt/config.yml` in private mode) and re-running `init` or `docket install`, and changing the selection later has no command at all. This was found while hand-checking change 535.

## What changes

- **One shared "choose harnesses" step** writes `agent_harnesses` to the repository config and refreshes the dispatch instructions in the same run, through the machinery `docket install` already uses (dropped harnesses lose their surfaces through the existing proof-gated removals).
  - Shared: an uncommitted `.docket.yml` edit for the human to commit, like `configure-tests`.
  - Private: `.git/dckt/config.yml`.
- **`docket repository init`** runs it on first setup.
  - `--harnesses claude,cursor` (or `none` for `[]`).
  - On a terminal with no flag and no repository-level value: an arrow-key checkbox picker (built on `charmbracelet/huh`), pre-checked with the global `agent_harnesses`, falling back to the harnesses detected on the machine. Nothing checked writes `[]`.
  - No terminal (an agent, `--json`, CI): init succeeds and warns, naming the line to add and the command to run.
  - A repository that already declares the key is left as it is.
- **New `docket repository configure-harnesses`** runs the same step any time later, on a set-up repository, without init's clean-checkout and at-tip precondition. With no flag on a terminal it shows the picker pre-checked with the current value; with no flag and no terminal it refuses.
- **`docket repository check`** warns (`harnesses-unset`, never blocking) while no repository-level layer declares the key. `[]` or any list silences it.
- One new ADR records adopting `huh` as docket's interactive UI dependency.

## Out of scope

- Making init refuse to run until harnesses are chosen.
- Copying the global `agent_harnesses` into the repository config without asking.
- Changing 0351's rule that only a repository-level `agent_harnesses` authorizes repository writes.
- Relaxing init's clean-checkout and at-remote-tip precondition.
- Converting the existing y/N prompts or other commands to `huh`.
