---
id: 16
slug: harness-first-agent-config
title: Harness-first `agents:` config — per-harness model/effort with field-level default fallback
status: Accepted
date: 2026-07-08
supersedes: []
reverses: []
relates_to: [15, 8]
change: 46
---

## Context

ADR-[[0015]] (change 0045) made per-repo agent-wrapper generation fan out to an explicit
`agent_harnesses:` list, so a repo can commit `<repo>/.claude/agents/…` **and**
`<repo>/.cursor/agents/…` in the same pass. But every generated file still resolved from
**one** agent-keyed `agents:` block — every listed harness got the **same** `model` string.
That is wrong the moment the harnesses run different rosters: a Claude model ID like
`claude-opus-4-8` written into the Cursor file is not an error Cursor surfaces — Cursor
silently ignores an unrecognized `model:` and runs its own house default (0015's "silent
failure surface" consequence, now realized). 0015's reproducibility guarantee held for the
generated **files** (clone-identical, committed) but not for their **values** — the config
could not express "this model on Claude, that model on Cursor." Running docket across
Cursor, Claude, and Codex in the same org — each with its own model roster — needs
per-harness model/effort, not one value fanned out to every harness.

## Decision

Make the `agents:` block **harness-first**. Top-level keys are a reserved **`default:`**
(neutral fallback) plus **harness names** (`cursor`, `claude`, …); each holds the familiar
agent → `{model, effort}` map:

```yaml
agents:
  default:
    implement-next: { model: sonnet, effort: high }
  claude:
    implement-next: { model: opus, effort: xhigh }
  cursor:
    implement-next: { model: gpt-5.5-medium-fast }
```

Resolution is **field-by-field, independent per field, first non-empty wins**:
`agents.<harness>.<agent>.<field>` → `agents.default.<agent>.<field>` → the shipped built-in
in `agents/docket-*.md`. `model` and `effort` fall through independently — a harness block
can pin `model` and inherit `effort` from `default`, or vice versa.

The same harness-first map governs **both** config layers ADR-[[0008]] established, never
hand-merged (Claude Code's native project-over-user precedence still does the layering): the
per-repo `.docket.yml` nests it under `agents:`; the global `~/.config/docket/agents.yaml`
**is** the map at the file's top level (no `agents:` wrapper — a shape asymmetry the reader
carries via an `under_agents` flag, not two parsers).

Four sub-decisions, each consciously chosen over an alternative:

1. **Layer symmetry over a per-harness catch-all default.** No "one model for every agent
   under harness X" knob. Overriding is per-agent, inheriting via `default:` — the same
   granularity ADR-[[0008]] already established, now crossed with harness instead of
   replacing it.
2. **Clean break, not a compatibility shim.** The pre-0046 flat agent-keyed shape (no
   harness layer) is detected, warned, and ignored rather than dual-read. Safe to do: no live
   agent-keyed config exists anywhere — docket's own `.docket.yml` `agents:` block is
   commented out, and no global `agents.yaml` is in use.
3. **No model-ID validation** — ADR-[[0015]]'s unvalidated passthrough is preserved
   verbatim; docket still never allowlists or checks a `model:` string. Instead a
   **non-fatal, allowlist-free footgun warning** fires only when a **non-`claude`** harness's
   `model` field fell through all the way to `default:`/the built-in (a value shaped for
   Claude, likely wrong for that harness). The warning never blocks generation.
4. **`agent_harnesses` (which harness directories get files) stays orthogonal to
   `agents.<harness>` (which values apply).** A harness block present in `agents:` but absent
   from `agent_harnesses:` is dead config — warned and dropped, never silently promoted into
   a generation target.

A dated `## Update` note on ADR-0015 was considered instead of a new ADR — the surrounding
context (multi-harness generation) is the same. Rejected: the harness-first schema, the
field-level fallback order, and the footgun heuristic are a distinct, citable mechanism, not
added color on 0015's decision. 0015's own decision — direct, unvalidated model IDs and
explicit per-repo harness fan-out — is **refined** here (crossed with a harness axis), not
reversed; nothing about 0015 changes status.

## Consequences

- True per-harness model/effort now composes with ADR-[[0008]]'s reproducibility guarantee:
  committed, clone-identical per-harness files, each carrying values correct for its own
  harness.
- `sync-agents.sh`'s readers, all three generation passes, and `--check` resolve per
  `(harness, agent)` pair instead of per `agent` alone; `--check` also flags the legacy
  flat-agent-keyed shape as drift the moment it's seen.
- The footgun warning is heuristic only — it never blocks a build. A zero-config non-Claude
  harness (no `agents:` block at all) sees a per-agent fallback warning for every agent; that
  is the intended signal, not noise to suppress.
- `agent_keys` (the set of agents a pass generates files for) is now a **union** across
  `default:` plus every harness sub-block, so overriding one harness's single agent still
  emits the built-in-pinned file for that same agent on every other listed harness. Harmless,
  and consistent with the reproducibility goal — every listed harness always gets a full set.
- **Open follow-up (not decided here):** whether each non-Claude harness applies
  project-over-user precedence the way Claude Code does natively. If a harness does not,
  docket's generator must merge the global layer into that harness's project-level file
  itself rather than relying on the harness to layer them — a build-time, live-verification
  item for change 0046.

## Update

**2026-07-09 (change 0051, [[0020]]).** [[0020]] retires the premise this open follow-up was
asking about: generated agent artifacts are no longer committed project-level files at all
(they are gitignored and machine-local), so per-harness project-over-user precedence for
*committed* files is moot. The question of whether a non-Claude harness natively layers
local-over-global-over-built-in the way Claude Code does remains open, now scoped to
[[0020]]'s four local/global/committed/built-in layers instead of two. The harness-first
field-by-field resolution this ADR establishes is unchanged and is exactly what [[0020]]
extends with two more layers.
