# Design — Model-tier indirection for docket agent model selection + config-driven advisories

Change: #0043 · slug `agent-model-tiers` · spec drafted 2026-07-07 (interactive brainstorm, owner)
Depends on #0042 (`done`). Related: [16, 17, 42].

## Problem

After #0042, a concrete model ID (`claude-sonnet-5`, `claude-opus-4-8`,
`claude-haiku-4-5-20251001`) is stamped into eight `agents/docket-*.md` frontmatters and two
advisory lines in the interactive skills. The assignments cluster into exactly three groups —
Opus 4.8/`xhigh`, Sonnet 5/`medium`, Haiku 4.5/`medium` — but each concrete value is repeated
across files. The **next** model sunset therefore repeats #0042's ~10-file churn, and a
cost-conscious repo that wants "run everything one tier cheaper" has to override every agent
individually in its `.docket.yml`.

The fix is a level of indirection: name the three clusters as **tiers**, define each tier's
`{model, effort}` in **one place**, and have everything reference a tier. Then a sunset — or a
whole-repo cost policy — is a three-line edit to the tier→model map.

## Decision

Introduce a **tier layer** as the source of truth for every agent's model/effort, generated into
concrete frontmatter by the existing `sync-agents.sh`. This is the *full* reach chosen in the
brainstorm: the shipped `agents/docket-*.md` frontmatter becomes a **generated artifact**, not a
hand-authored one.

### 1. Built-in tier map + agent→tier manifest (new source of truth)

A docket-shipped manifest (format/location an implementer call — e.g. `agents/tiers.yaml`, or a
data block `sync-agents.sh` reads) carries two tables:

```yaml
# tiers: name -> {model, effort}   (the three clusters #0042 pinned)
tiers:
  critical: { model: claude-opus-4-8,             effort: xhigh }
  standard: { model: claude-sonnet-5,             effort: medium }
  economy:  { model: claude-haiku-4-5-20251001,   effort: medium }

# defaults: agent/skill -> tier   (which cluster each belongs to)
defaults:
  implement-next:    critical
  auto-groom:        critical
  auto-groom-critic: critical
  integration-repair: critical
  rebase-resolver:   critical
  adr:               standard
  finalize-change:   standard
  status:            economy      # #0042's demotion, now a tier assignment
  new-change:        standard     # advisory-only (no agent file)
  groom-next:        standard     # advisory-only
```

Tier names `critical` / `standard` / `economy` are the proposal; final names are a build-time
call, but they must be role/cost-semantic (not model names), because the whole point is that the
model behind a tier can change.

### 2. `sync-agents.sh` resolves tier → concrete frontmatter, and regenerates the shipped files

`sync-agents.sh` gains tier resolution and, per the *full* choice, treats `agents/docket-*.md` as
output it regenerates from the manifest (the same way it already regenerates the user-level and
project-level copies). Resolution order for one agent:

1. Start from its built-in **tier** (`defaults`) → the tier's `{model, effort}`.
2. Apply config overrides (below), highest-precedence wins.
3. `emit()` bakes the resolved concrete `model:`/`effort:` into the frontmatter.

The shipped `agents/docket-*.md` remain **committed** (they must stay directly usable by a harness
that never ran the generator), but they are now generated — so editing the manifest and running
`sync-agents.sh` rewrites all eight at once. That is the "one place" win.

### 3. Config layers — two override shapes, precedence unchanged (per-repo > global > built-in)

The existing `agents:` block keeps working and gains a tier-aware form; a new sibling `tiers:`
block remaps a tier's model:

```yaml
# .docket.yml (per-repo) or ~/.config/docket/agents.yaml (global)
tiers:
  critical: { model: claude-sonnet-5 }     # a cost-conscious repo runs "critical" on Sonnet 5
agents:
  status:        { tier: standard }        # reassign one agent to another tier
  finalize-change: { model: claude-opus-4-8, effort: xhigh }  # explicit model/effort STILL wins over tier
```

Resolution for an agent entry: an explicit `model:`/`effort:` on the entry wins (backward-compatible
with #0016); else a `tier:` on the entry selects a tier; else the built-in `defaults` tier applies.
A `tiers:` override changes what a tier resolves to, at its layer's precedence. This reuses
`sync-agents.sh`'s existing per-repo>global>built-in layering — no new precedence rule.

### 4. Config-driven advisories (the two interactive skills)

`docket-new-change` and `docket-groom-next` have no agent file (they're inline skills), so
`sync-agents.sh` cannot bake their advisory. Instead they **resolve their advisory at startup**:
`docket-config.sh` gains a lookup (e.g. `docket-config.sh --advisory <skill>`) that resolves the
skill's tier through the same manifest ⊕ config layers and prints the concrete model. The skill's
"Recommended model/effort (advisory)" line becomes computed, not hardcoded — so it tracks the tier
map like everything else, and #0042's hand-edited advisory strings stop being a maintenance point.

### 5. Drift gate covers the now-generated shipped files

`sync-agents.sh --check` currently guards only the project-level committed copies. Extend it so the
committed `agents/docket-*.md` are also checked against a fresh resolve of the built-in manifest —
otherwise a hand-edit to a generated agent file (or a manifest change without a re-run) drifts
silently. This keeps the CI drift backstop honest for the new source-of-truth.

## Backward compatibility

- Existing `.docket.yml`/global `agents: { x: { model, effort } }` overrides resolve identically
  (explicit model/effort still wins). No consumer repo config breaks.
- Short-alias config **input** (`{ model: haiku }`) stays valid; only the *shipped defaults* are
  tier-resolved to full IDs.
- A repo that never touches tiers gets byte-identical agent files to what #0042 shipped (the
  built-in manifest resolves to exactly #0042's table).

## What the implementer edits

- **New manifest** (`agents/tiers.yaml` or equivalent) — the tier map + agent→tier defaults.
- **`sync-agents.sh`** — tier resolution; regenerate `agents/docket-*.md` from the manifest;
  `agents:`-entry `tier:` support; `tiers:` block parsing at both config layers; extend `--check`
  to the shipped files.
- **`scripts/docket-config.sh`** (+ contract `docket-config.md`) — the `--advisory <skill>` (or
  equivalent) tier-resolving lookup.
- **`skills/docket-new-change/SKILL.md`**, **`skills/docket-groom-next/SKILL.md`** — advisory line
  becomes computed via the lookup instead of a literal.
- **`docket-convention`** — the "Agent layer" section documents the tier map, the two override
  shapes, and the advisory resolution; note the shipped agent files are now generated.
- **Tests** — `tests/test_sync_agents.sh` (tier resolution, override precedence, `--check` on
  shipped files), plus coverage for the advisory lookup. Keep the #0042 built-in-value assertions
  (they now assert the *resolved* defaults).
- **ADR** — likely warranted: "the built-in agent defaults are tier-generated, and the shipped
  `agents/docket-*.md` are generated artifacts." Decide at build via `docket-adr`.

## Open questions (resolve at build)

1. **Manifest format/location** — standalone `agents/tiers.yaml` vs a block `sync-agents.sh` reads
   vs extending `.docket.yml`'s schema for the built-in defaults. Lean: a docket-shipped
   `agents/tiers.yaml` (built-in), overridable by `.docket.yml`/global `tiers:`.
2. **Final tier names** — `critical/standard/economy` vs another role/cost vocabulary.
3. **Advisory lookup surface** — a new `docket-config.sh` flag vs a tiny dedicated helper; and how
   the skill renders "no harness resolution available" gracefully offline.
4. **Should a repo be able to define *new* tiers**, or only remap the three built-ins? Lean: remap
   only in the first cut (YAGNI); revisit if asked.

## Non-goals

- The TDD build model (`build.implementer`/`build.reviewer`) — that is #0044, which consumes this
  tier map.
- Re-tuning which agent belongs to which tier beyond #0042's assignments (this change only makes
  the assignments indirect; it does not re-decide them).
- Per-change or per-run model selection — tiers are repo/global config, not change frontmatter.
