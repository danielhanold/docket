# Design — Per-repo multi-harness agent generation (`agent_harnesses`)

Change: #0045 · slug `multi-harness-agent-generation` · spec drafted 2026-07-08 (owner)
Standalone. Related: [16, 42, 43, 44]. ADR: [15].

## Problem

docket's per-agent model config (#0016, ADR-[[0008]]) generates committed
`<repo>/.claude/agents/docket-*.md` from the `.docket.yml` `agents:` block. But the
**project-level pass writes `.claude/agents/` only** — the multi-harness fan-out
(`HARNESS_AGENT_DIRS`, which includes `~/.cursor/agents`) lives solely in the *user-level*
pass. So when docket runs through a non-Claude harness — the motivating case is **Cursor at
work** — the committed per-repo `agents:` config reaches nothing the harness reads, and
ADR-0008's reproducibility guarantee silently fails there.

Verified 2026-07-08 (probe subagent): Cursor honors an arbitrary project-level `model:`
(`gpt-5.5-medium-fast`) from `<repo>/.cursor/agents/…`. So the only missing piece is *emitting*
the committed wrappers where Cursor looks; the passthrough (ADR-0008) and the honoring already
work.

## Decision (per ADR-[[0015]])

Add `.docket.yml` **`agent_harnesses:`** — an explicit list of harnesses the per-repo pass
generates committed agent files for. **Global default `[claude]`** (byte-identical to today). A
Cursor repo sets `[claude, cursor]` (or `[cursor]`). Explicit over present-directory
auto-detection, so targets are predictable and a stray `.cursor/` never silently mints files.

### Config shape

```yaml
# .docket.yml
agent_harnesses: [claude, cursor]   # default [claude]
agents:
  status:         { model: gpt-5.5-medium-fast }
  implement-next: { model: gpt-5.1, effort: high }
```

Each harness `H` in the list → committed `<repo>/.<H>/agents/docket-*.md`, resolved from the
same built-in ⊕ per-repo config. Model IDs pass through **verbatim** — no tiers, no allowlist.
Token→dir mapping reuses the existing `HARNESS_AGENT_DIRS` vocabulary (`claude`→`.claude/agents`,
`cursor`→`.cursor/agents`, `codex`→`.codex/agents`, …). An unknown token is **warned-and-ignored**
(a typo must never abort a sync), mirroring `board_surfaces`.

### `sync-agents.sh` changes

- **`project_level_pass`** — replace the single hardcoded `PROJECT_AGENT_DIR=$REPO/.claude/agents`
  with a loop over the resolved `agent_harnesses`, writing `<repo>/.<H>/agents/docket-*.md` for
  each. Default `[claude]` reproduces current behavior exactly.
- **`check_project_level` (`--check`)** — extend the drift diff to every generated per-harness file
  across `agent_harnesses`. A missing or stale file for any listed harness fails CI.
- Read `agent_harnesses` (default `[claude]`, unknown-token warn-and-drop) — lean on
  `docket-config.sh --export` so the skill and the CI gate share one resolver.

### User-level pass — unchanged (deliberate)

The user-level pass (`~/.config/docket/agents.yaml` → every present `HARNESS_AGENT_DIRS`) keeps
"every present harness." `agent_harnesses` is a **per-repo** knob about **committed** files;
applying it to the user-level pass would be a behavior change that could stop writing
`~/.cursor/agents/` for existing global-config users (e.g. the setup that first worked). So
`agent_harnesses` governs the **project-level pass only**. Revisit only if a global default is
wanted — and then the default must preserve today's user-level fan-out.

## What the implementer edits

- **`sync-agents.sh`** — `project_level_pass` loop over `agent_harnesses`; `check_project_level`
  over the same set; the token→dir map + unknown-token warn.
- **`scripts/docket-config.sh`** (+ `.md`) — parse and export `agent_harnesses` (default
  `[claude]`, unknown-token warn-and-drop).
- **`docket-convention`** — document `agent_harnesses` in the config schema + the Agent-layer
  prose; state the **direct-model-ID (harness-neutral) contract** and that project generation now
  targets the listed harnesses. Note the passthrough is what makes non-Claude harnesses work.
- **`.docket.yml`** (this repo) — no functional change (docket dogfoods Claude Code; default
  `[claude]` keeps it identical); optionally add a commented `agent_harnesses` example.
- **Tests** (`test_sync_agents.sh`) — (a) default `[claude]` → byte-identical to today; (b)
  `[claude, cursor]` → both `.claude/agents/` and `.cursor/agents/` generated with the same
  resolved model, incl. an arbitrary non-Claude ID passing through; (c) `--check` detects drift in
  a `.cursor/agents/` file; (d) unknown harness token warned + dropped, not fatal.

## Out of scope

- The reshaped `build:` SDD-dispatch surface (#0044) — separate change; shares only the
  direct-model-ID vocabulary (ADR-0015).
- Re-introducing any tier abstraction — killed with #0043.
- Changing the user-level pass semantics (see above).
- Validating model IDs against a harness roster — docket stays passthrough (ADR-0008/0015); a
  wrong ID is a documented footgun, not a docket error.

## Open questions (resolve at build)

1. Where `agent_harnesses` is read — `docket-config.sh --export` (preferred, shared with CI) vs a
   direct parse in `sync-agents.sh`.
2. Dir creation — lean on sync's existing `mkdir -p` per listed harness (as it already does for
   `.claude/agents`); no install.sh seeding needed.
3. Confirm the **generated** `.cursor/agents/docket-*.md` is read by Cursor identically to the
   hand-made probe file (filename/format parity).

## ADR

ADR-[[0015]] (Accepted) records the decision — harness-neutral direct model IDs + explicit
per-repo harness fan-out; this change implements the fan-out half.
