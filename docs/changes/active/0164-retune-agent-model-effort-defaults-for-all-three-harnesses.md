---
id: 164
slug: retune-agent-model-effort-defaults-for-all-three-harnesses
title: Retune agent model/effort defaults for all three supported harnesses
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: []
adrs: [39]
spec:
plan:
results:
trivial: true
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
| ADRs | [ADR-0039](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0039-config-example-mirrors-wrapper-defaults.md) |
<!-- docket:artifacts:end -->

## Why

The shipped per-skill model/effort defaults have drifted out of date. The nine
`agents/docket-*.md` wrappers — the single source of truth for the built-in `claude` tier per
ADR-0039 — still pin `claude-opus-4-8` / `claude-sonnet-5` at `xhigh` / `medium`, while the
`codex` and `cursor` example blocks in `.docket.example.yml` carry unvalidated IDs from earlier
retunes. The config-layer mirrors that advertise themselves as matching the built-ins therefore
lie: a user reading `.docket.example.yml`'s `claude:` block sees values the wrappers do not
actually ship.

This is a values-only retune (precedent: change 0042), not a behavior change to the agent layer.

## What changes

Apply this configuration, verbatim, as the shipped defaults for all three harnesses:

```yaml
agents:
  claude:
    status:                { model: claude-haiku-4-5-20251001, effort: medium }
    adr:                   { model: claude-opus-5,             effort: low }
    brainstorm-consultant: { model: claude-opus-5,             effort: medium }
    auto-groom:            { model: claude-opus-5,             effort: low }
    auto-groom-critic:     { model: claude-opus-5,             effort: medium }
    implement-next:        { model: claude-opus-5,             effort: medium }
    rebase-resolver:       { model: claude-opus-5,             effort: medium }
    integration-repair:    { model: claude-opus-5,             effort: medium }
    finalize-change:       { model: claude-opus-5,             effort: low }
  codex:
    status:                { model: gpt-5.6-luna,  effort: xhigh }
    adr:                   { model: gpt-5.6-terra, effort: xhigh }
    brainstorm-consultant: { model: gpt-5.6-sol,   effort: medium }
    auto-groom:            { model: gpt-5.6-sol,   effort: low }
    auto-groom-critic:     { model: gpt-5.6-sol,   effort: medium }
    implement-next:        { model: gpt-5.6-sol,   effort: medium }
    rebase-resolver:       { model: gpt-5.6-sol,   effort: high }
    integration-repair:    { model: gpt-5.6-sol,   effort: high }
    finalize-change:       { model: gpt-5.6-terra, effort: high }
  cursor:
    status:                { model: cursor-grok-4.5-low-fast,  effort: auto }
    adr:                   { model: cursor-grok-4.5-high,      effort: auto }
    brainstorm-consultant: { model: cursor-grok-4.5-high,      effort: auto }
    auto-groom:            { model: cursor-grok-4.5-medium,    effort: auto }
    auto-groom-critic:     { model: cursor-grok-4.5-high,      effort: auto }
    implement-next:        { model: cursor-grok-4.5-high,      effort: auto }
    rebase-resolver:       { model: cursor-grok-4.5-high,      effort: auto }
    integration-repair:    { model: cursor-grok-4.5-high,      effort: auto }
    finalize-change:       { model: cursor-grok-4.5-high-fast, effort: auto }
```

Sites to update:

- The nine `agents/docket-*.md` wrappers — `model:` + `effort:` frontmatter, the built-in `claude`
  tier (ADR-0039).
- `.docket.example.yml`'s commented `agents:` reference — all three harness blocks, including the
  `cursor:` block, which currently uses bare `grok-4.5-*` IDs where the new values are
  `cursor-grok-4.5-*`.
- `skills/docket-convention/references/agent-layer.md` — the illustrative `status:` line and any
  other model literal in its examples.
- The literal assertions in `tests/test_sync_agents.sh`, `tests/test_sync_agents_codex.sh`,
  `tests/test_sync_agents_cursor.sh`, and `tests/test_docket_example_yml.sh` — these pin the
  built-in and example values by string, so they move with the retune rather than being weakened.

Verification: `sync-agents.sh --check` clean, plus the four touched test files green.

## Out of scope

- Any change to how the agent layer resolves, generates, or dispatches wrappers — values only.
- Live validation that each `codex` / `cursor` model ID exists in that harness. The `claude` IDs
  are real; the other two follow the file's existing "unvalidated examples" posture unless the
  build can cheaply confirm them.
- Updating this machine's `~/.config/docket/config.yml` or `.docket.local.yml` mirrors. Those are
  per-machine files outside the repo's change surface; note in the results that they now differ
  from the built-ins.
