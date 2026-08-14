---
id: 19
slug: global-config-fence-classification
title: Global config layer — the coordination-key fence classification rule
status: Accepted
date: 2026-07-09
supersedes: []
reverses: []
relates_to: [8, 15, 16]
change: 50
---

## Context

Change 0050 introduced a global config file (`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`)
accepting the full `.docket.yml` schema, resolved per-key as per-repo > global > built-in by
`docket-config.sh --export` (single reader). The previous lone global file (`agents.yaml`)
covered one concern with a different shape and failed silently when users assumed a full global
`.docket.yml` worked (hit by the docket author 2026-07-09).

The non-obvious decision is WHICH keys a global file may set. Honor-everything-globally was
rejected because a global `metadata_branch`/`changes_dir` silently splits the backlog across
machines — the exact failure the committed-file rule exists to prevent.

## Decision

A key is **per-repo-only (fenced)** when its effect writes **shared state** — commits on shared
branches whose content is not deterministically re-derivable, committed generated files, or
external (GitHub) objects. A key is **global-able** when its effect is confined to the local run —
self-healing derived views and per-machine uncommitted files. Fenced keys set globally are loudly
warned-and-ignored (never honored, never fatal — the warn-and-ignore posture docket already uses
for config noise).

**Resulting classification:**

- **Fenced:** `metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`,
  `github_project`, and the `github` token of `board_surfaces` when it arrives from the global
  layer (it mints issues + a Projects board — external, not self-healing).
- **Global-able:** `skills:`, `agents:` (harness-first block), `auto_groom`,
  `finalize.gate`/`finalize.test_command`, `board_surfaces` minus `github` (BOARD.md is
  deterministically re-derivable, so per-machine divergence is self-healing staleness), and
  `agent_harnesses` with a scope split — the global value governs only `sync-agents.sh`'s
  user-level pass (which harness roots get uncommitted per-machine wrappers, including creating
  listed dirs, skipping unlisted, pruning de-listed); the per-repo committed pass is governed
  solely by the repo's own key, because a global value shaping committed files would fail
  `sync-agents.sh --check` on every other machine.

## Consequences

Cross-repo defaults work for workflow/policy knobs with zero per-repo config; clone-identical
coordination is preserved by construction (fenced keys can never diverge per machine); the fence
list must be re-evaluated whenever a new `.docket.yml` key is added (classify by the rule, not by
enumeration); the legacy `agents.yaml` is auto-migrated into `config.yml`'s `agents:` block with
no dual-read.
