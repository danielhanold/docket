---
id: 164
slug: retune-agent-model-effort-defaults-for-all-three-harnesses
title: Retune agent model/effort defaults for all three supported harnesses
status: done
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-29
depends_on: []
related: []
discovered_from: []
adrs: [39]
spec:
plan: docs/superpowers/plans/2026-07-28-retune-agent-model-effort-defaults.md
results: docs/results/2026-07-28-retune-agent-model-effort-defaults-for-all-three-harnesses-results.md
trivial: true
auto_groomable:
branch: feat/retune-agent-model-effort-defaults-for-all-three-harnesses
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/138
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-07-28-retune-agent-model-effort-defaults.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-retune-agent-model-effort-defaults.md) |
| Results | [2026-07-28-retune-agent-model-effort-defaults-for-all-three-harnesses-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-28-retune-agent-model-effort-defaults-for-all-three-harnesses-results.md) |
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
- (added at reconcile) `README.md`'s two illustrative `agents:` example blocks and
  `sync-agents.sh`'s effort-rendering comment table — same class of illustrative model literal as
  `agent-layer.md`, and stale for the same reason.

Verification: `sync-agents.sh --check` clean, plus the four touched test files green.

## Out of scope

- Any change to how the agent layer resolves, generates, or dispatches wrappers — values only.
- Live validation that each `codex` / `cursor` model ID exists in that harness. The `claude` IDs
  are real; the other two follow the file's existing "unvalidated examples" posture unless the
  build can cheaply confirm them.
- Updating this machine's `~/.config/docket/config.yml` or `.docket.local.yml` mirrors. Those are
  per-machine files outside the repo's change surface; note in the results that they now differ
  from the built-ins.
- The `claude-sonnet-5` advisory model pinned in the two interactive skills
  (`docket-new-change`, `docket-groom-next`) and asserted at `tests/test_sync_agents.sh:494-495`.
  Those are advisory session-model recommendations, not wrapper defaults; retuning them is a
  separate judgment call.

## Reconcile log

### 2026-07-28

Re-read against current code on `origin/main`. Findings:

- **Claude tier is stale as described.** Eight of nine wrappers move; `status` alone is already at
  the target (`claude-haiku-4-5-20251001` / `medium`). `adr` and `finalize-change` move off
  `claude-sonnet-5`/`medium`; the other six move off `claude-opus-4-8`/`xhigh`. All nine land on
  the values in `## What changes`.
- **The `codex:` example block is ALREADY byte-equivalent to the target** (`gpt-5.6-luna` /
  `-terra` / `-sol` at the same efforts). It needs no value edit — only whatever whitespace
  alignment the retune's formatting implies. Scope narrows accordingly.
- **The `cursor:` example block is the real second edit**: every ID moves from bare `grok-4.5-*`
  to the `cursor-grok-4.5-*` namespace, and three of the nine change tier as well
  (`status` fast-medium→low-fast, `auto-groom` high→medium, `finalize-change` fast-high→high-fast).
- **Mirror equality is machine-enforced**, so this is not a "remember to update both" risk:
  `tests/test_docket_example_yml.sh` §(4) asserts the commented `agents.claude` block matches each
  wrapper's frontmatter value-for-value (relocated ADR-0039). The wrappers lead; the example
  follows; a half-done retune fails the suite.
- **Two extra live surfaces carry the same stale literals** and are folded into scope:
  `README.md:397` and `README.md:425` (the `config.yml` and `.docket.local.yml` illustrative
  blocks), plus `sync-agents.sh:435-436`'s comment table showing effort rendering. Archived
  changes, plans, and specs under `docs/` keep their literals — they are historical records.
- **Two cross-harness tests pin the CLAUDE `status` id deliberately**
  (`test_sync_agents_codex.sh:35`, `test_sync_agents_cursor.sh:47`) as the built-in a non-claude
  harness inherits. Since `status` does not move, both stay green untouched — no weakening needed.
- **One cursor value assertion does move**: `test_docket_example_yml.sh:907` pins
  `grok-4.5-fast-medium` for the generated cursor `status` wrapper and follows the example block
  to `cursor-grok-4.5-low-fast`.
- Design intact; no invalidation. Values-only, exactly as drafted.
