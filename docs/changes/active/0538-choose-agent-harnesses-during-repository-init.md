---
id: 538
slug: 'choose-agent-harnesses-during-repository-init'
title: 'Choose agent harnesses during repository init'
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

`docket repository init` writes the parent-facing dispatch instructions only when the repository itself declares `agent_harnesses`. A global setting never authorizes repository writes (change 0351). But init never asks for the setting and never warns when it is missing.

The result is the same in both modes: no instructions and no message saying why. A shared repository gets no `CLAUDE.md`/`AGENTS.md` block, and a private one gets no `.git/dckt/AGENTS.md`. In private mode the fix today takes three steps: init, then a hand edit of `.git/dckt/config.yml`, then `docket install`. This was found while hand-checking change 535.

## What changes

- `--harnesses <list>` on `init` writes the selection to the repository config and reconciles the dispatch instructions in the same run.
  - Private: the selection goes into `.git/dckt/config.yml`.
  - Shared: it becomes an unstaged `.docket.yml` edit for the human to commit, the same way `configure-tests` works.
- **Picker.** On a terminal, when there is no flag and no repository-level `agent_harnesses`, `init` shows a multi-select picker.
  - It is pre-checked with the global `agent_harnesses`, falling back to the harnesses detected on the machine.
  - Only the human's selection is written.
  - Choosing none writes `agent_harnesses: []`, a recorded decision rather than a warning.
- **No terminal** (an agent, `--json`, or CI): there is no prompt. Init succeeds and warns, naming the exact line to add and the command to re-run.
- **Already-initialized repository:** `--harnesses` or the picker updates the selection. The installer's existing proof-gated removals retire the surfaces of harnesses that were dropped. `[]` retires everything docket owns there.
- **Known consequence:** changing harnesses through `init` inherits init's precondition that the checkout is clean and at the remote integration tip. `docket install` applies a change without that precondition.

## Out of scope

- Making init refuse to run until harnesses are chosen.
- Copying the global `agent_harnesses` into the repository config without asking.
- Changing 0351's rule that only a repository-level `agent_harnesses` authorizes repository writes.
- Relaxing init's clean-checkout and at-remote-tip precondition.
