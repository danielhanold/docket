---
id: 50
slug: global-config-layer
title: Global config layer — full-schema ~/.config/docket/config.yml with a coordination-key fence
status: proposed
priority: medium
created: 2026-07-09
updated: 2026-07-09
depends_on: []
related: [16, 26, 44, 45, 46, 47, 49]
adrs: [2, 8, 15, 16]
spec: docs/superpowers/specs/2026-07-09-global-config-layer-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-09-global-config-layer-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-09-global-config-layer-design.md) |
| ADRs | [ADR-0002](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0002-docket-mode-default-and-bootstrap.md), [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

docket has no discoverable user-level configuration story. `.docket.yml` is per-repo only;
the sole global file (`~/.config/docket/agents.yaml`) covers one concern, uses a different
shape than the `.docket.yml` `agents:` block it mirrors, and its format is documented only
in a YAML comment inside docket-convention. The natural assumption — a full `.docket.yml`
at `~/.config/docket/` applying to every repo — fails **silently**: nothing reads it,
nothing warns. The docket author hit exactly this on 2026-07-09. The confusion is a design
gap, not just a docs gap: there is real demand for cross-repo defaults (workflow skills,
agent models, groom/finalize policy, user-level harness fan-out).

## What changes

One global file, `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`, accepting the full
`.docket.yml` schema, resolved per-key as **per-repo > global > built-in** by
`docket-config.sh --export` (single reader; skills' Step-0 interface unchanged):

- **Coordination-key fence:** keys whose effect writes shared, non-re-derivable state
  (`metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`,
  `github_project`) are per-repo-only — set globally they are loudly warned-and-ignored.
- **Global-able:** `skills:`, `agents:` (wrapper-keyed, same shape as `.docket.yml`),
  `auto_groom`, `finalize.*`, `board_surfaces` minus the `github` token (external objects
  stay repo opt-in), and `agent_harnesses` scoped to `sync-agents.sh`'s **user-level pass
  only** (overriding presence-on-disk detection; the per-repo committed pass is untouched).
- **`agents.yaml` auto-migration:** `sync-agents.sh` idempotently rewrites it under
  `agents:` in `config.yml`, renames the original to `agents.yaml.migrated`, logs loudly;
  no dual-read fallback remains.
- **Fail-loud guards:** `~/.config/docket/.docket.yml` → "did you mean `config.yml`?";
  malformed global file warns and falls back to built-ins without bricking repos.
- **Docs:** README "Global config" section with a full example; convention Configuration +
  Agent layer updated to the three-layer story; script contracts updated. The fence
  classification rule is recorded as a new ADR at build time.

## Out of scope

- New configuration keys, or changes to what per-repo `.docket.yml` supports.
- A per-repo uncommitted override file (`.docket.local.yml`).
- Bootstrap-template semantics (baking global coordination keys into fresh repos' committed
  `.docket.yml`) — deferred until a real need appears.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
