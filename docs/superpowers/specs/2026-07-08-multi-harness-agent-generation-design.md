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
- Read `agent_harnesses` (default `[claude]`, unknown-token warn-and-drop) via a **direct parse in
  `sync-agents.sh`**, a small top-level list reader beside `block_names`. `sync-agents.sh` does not
  use `docket-config.sh` (it is a self-contained `.docket.yml` parser); generation and `--check`
  are the same script, so the reader is shared between them with no cross-script dependency, and it
  reads the local working-tree `.docket.yml` — the correct file, since sync generates committed
  files in that same tree. (RESOLVED — see Open questions.)

### User-level pass — unchanged (deliberate)

The user-level pass (`~/.config/docket/agents.yaml` → every present `HARNESS_AGENT_DIRS`) keeps
"every present harness." `agent_harnesses` is a **per-repo** knob about **committed** files;
applying it to the user-level pass would be a behavior change that could stop writing
`~/.cursor/agents/` for existing global-config users (e.g. the setup that first worked). So
`agent_harnesses` governs the **project-level pass only**. Revisit only if a global default is
wanted — and then the default must preserve today's user-level fan-out.

## What the implementer edits

- **`sync-agents.sh`** — a small top-level `agent_harnesses` list parser (default `[claude]`,
  unknown-token warn-and-drop); `project_level_pass` loop over it; `check_project_level` over the
  same set; the token→dir map. No `docket-config.sh` change — sync stays a self-contained parser.
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

## Resolved decisions

1. **`agent_harnesses` is read by a direct parse in `sync-agents.sh`** — not `docket-config.sh`.
   sync is a self-contained `.docket.yml` parser; generation and `--check` are the same script (so
   the reader is shared), and it must read the *local working-tree* `.docket.yml` it is syncing —
   not `docket-config.sh`'s authoritative `origin/HEAD` copy. `docket-config.sh` is untouched.
2. **Dir creation** — sync's existing `mkdir -p` per listed harness (as for `.claude/agents`); no
   `install.sh` seeding.

## Open questions (resolve at build)

1. **Live verification (NOT an automated test): the generated wrapper functions under Cursor.**
   The hermetic suite (`test_sync_agents.sh`) can only assert the *bytes generated*; whether Cursor
   *honors* the file is live behavior, verified once at build (per the repo's metadata-branch
   testing convention). The generated `.cursor/agents/docket-*.md` is richer than the hand-made
   probe — it carries `effort:` and, load-bearingly, `skills: [docket-<skill>, docket-convention]`.
   Confirm on the real generated file that Cursor (a) still **honors `model:`** despite the extra
   frontmatter, and (b) still **loads the skill via `skills:`** so the agent actually *is* the
   docket agent (not a bare model on an empty prompt). If (b) fails, Cursor may need the skill body
   inlined rather than referenced — a possible follow-up beyond this change.

## ADR

ADR-[[0015]] (Accepted) records the decision — harness-neutral direct model IDs + explicit
per-repo harness fan-out; this change implements the fan-out half.
