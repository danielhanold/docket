# Design — Per-harness agent model overrides (harness-first `agents:`)

Change: #0046 · slug `per-harness-agent-models` · spec drafted 2026-07-08 (owner)
Depends on: [45]. Related: [16, 42, 43, 44, 45]. ADR: [15] (+ likely new/update at build).

## Problem

Change #0045 (per ADR-[[0015]]) makes `sync-agents.sh`'s project-level pass fan out over
`agent_harnesses: [claude, cursor]`, generating committed `<repo>/.claude/agents/docket-*.md`
**and** `<repo>/.cursor/agents/docket-*.md`. But every generated file is resolved from the **one**
agent-keyed `agents:` block. So `implement-next: { model: claude-opus-4-8 }` writes that exact
string into the **Cursor** file too — and Cursor has no `claude-opus-4-8`. Per ADR-0015's own
consequence, Cursor **silently ignores an unknown `model:` and runs its house default**. The
per-repo config is present but ineffective on the non-Claude harness: exactly the reproducibility
failure ADR-0015 set out to fix (ADR-[[0008]]), relocated one layer down.

The operator genuinely needs **different model IDs per harness** — the whole point of running docket
across Cursor / Claude / Codex is that each exposes a different model roster (`gpt-5.5-medium-fast`
on Cursor, `claude-opus-4-8` on Claude, …). #0045 fans out the *files*; it cannot vary the
*values*. This change closes that.

## Decision

Make the `agents:` block **harness-first**. Its top-level keys are a reserved **`default:`** (the
neutral fallback) plus **harness names**; each sub-block is the familiar agent → `{model, effort}`
map. The identical shape lives at both config layers — user/global
(`~/.config/docket/agents.yaml`) and per-repo (`<repo>/.docket.yml`) — so each harness has a global
default that per-repo config overrides (the "layer symmetry" the owner chose over a per-harness
catch-all default model).

### Config shape

```yaml
# .docket.yml (or ~/.config/docket/agents.yaml — same shape)
agent_harnesses: [claude, cursor]     # (#0045) which harnesses to GENERATE files for

agents:
  default:                            # neutral fallback — RESERVED key
    implement-next: { model: claude-opus-4-8, effort: xhigh }
    status:         { model: claude-haiku-4-5-20251001 }
  cursor:                             # per-harness override
    implement-next: { model: gpt-5.1, effort: high }
    status:         { model: gpt-5.5-medium-fast }
  # claude not listed -> uses `default` above
```

### Resolution — per (harness `H`, agent `A`), field by field

`model` and `effort` resolve **independently** down this chain, first non-empty wins:

1. `agents.<H>.<A>` — per-harness override
2. `agents.default.<A>` — neutral
3. built-in default in the shipped `agents/docket-<A>.md`

So `agents.cursor.implement-next: { model: gpt-5.1 }` (no `effort`) takes `model` from the Cursor
block and `effort` from `agents.default.implement-next` (then the built-in). You override only what
differs. **No `agents:` block at all ⇒ every field falls to the built-in default — which are Claude
IDs** (`claude-opus-4-8`, `claude-haiku-4-5-…`): correct for the `claude` harness, silently wrong on
any non-Claude harness (see *Footgun warning*).

### Layer precedence — unchanged, native

As today (ADR-0008), the generator writes **two independent layers** and never hand-merges them:
user-level = built-in ⊕ global; project-level = built-in ⊕ per-repo. The harness applies
**project-over-user precedence natively** (Claude Code does for `.claude/agents/`). Whether each
**non-Claude** harness does the same for its own `.<H>/agents/` dir is a **build-time
live-verification item** (see Open questions) — if a harness does not, docket may need to merge
global into that harness's project-level file, a scoped follow-up.

### `agent_harnesses` vs `agents.<harness>` — orthogonal

`agent_harnesses` (#0045) stays the **authoritative fan-out list**: which harness *dirs* get files
(explicit, per ADR-0015 — never filesystem auto-detected). `agents.<harness>` supplies *values*.
They are decoupled:

- Harness in `agent_harnesses` with **no** `agents.<H>` block → uses `default` (+ warning if
  non-Claude, per below).
- Harness block in `agents:` **not** in `agent_harnesses` → **dead config → warned + ignored**
  (mirrors the unknown-token handling of `agent_harnesses` / `board_surfaces`).

### Legacy shape — clean break

`agents:` is harness-first only. A **bare agent key** at the top level of `agents:` (today's
pre-0046 shape, e.g. `agents:\n  implement-next: …`) is neither `default` nor a known harness →
**warned + ignored**, and `--check` reports it as drift. Safe because docket's own `agents:` block
is commented (no live entries) and #0045 is unbuilt, so nothing consumes the old shape yet.

### Footgun warning (heuristic, allowlist-free)

docket does not (and per ADR-0015 must not) validate model IDs against a roster. But it can catch
the precise silent failure: when generating a **non-`claude`** harness file whose **`model`**
resolved from `default`/built-in (i.e. no `agents.<H>` override supplied it), emit a non-fatal
stderr line:

```
sync-agents: WARN cursor/docket-implement-next: model 'claude-opus-4-8' came from
  default/built-in; may not be a valid model ID for harness 'cursor'.
```

Scoped to non-`claude` harnesses (the `claude` harness's built-ins/`default` are Claude IDs, so no
warning). Never an error; sync still succeeds.

## What the implementer edits

- **`sync-agents.sh`** — the config readers (`entry_line`, `block_names`, `resolve_from`) gain a
  **harness scope**: read under `agents.<H>` then fall back to `agents.default` (field-level),
  replacing the flat `agents:` reader. `user_level_pass` and `project_level_pass` resolve per
  (harness, agent) — the user-level pass over each present harness, the project-level pass over
  `agent_harnesses`. Bare-agent-key legacy blocks warned + ignored. Add the non-Claude fallback
  warning. `check_project_level` extends the drift diff to every per-harness file and flags the
  legacy shape. Still a self-contained `.docket.yml` parser (no `docket-config.sh` dependency).
- **`docket-convention`** — update the `.docket.yml` config schema and the *Agent layer* prose to
  the harness-first `agents:` shape (`default:` + harness keys), the field-level (H → default →
  built-in) resolution, layer symmetry, and the `agent_harnesses`/`agents.<harness>` orthogonality.
  This supersedes the agent-keyed `agents:` shape currently documented there.
- **`.docket.yml`** (this repo) — replace the commented agent-keyed example with a commented
  harness-first one. No functional change (docket dogfoods Claude Code; `default`/`claude` keep it
  identical).
- **Tests** (`tests/test_sync_agents.sh`) — (a) harness-first resolution: `agents.cursor` override
  wins, `agents.default` fallback, built-in floor; (b) **field-level merge** — override `model`,
  inherit `effort` from `default`; (c) arbitrary non-Claude model ID passes through verbatim into
  `.cursor/agents/`; (d) `default`-only (no harness block) reproduces today's `.claude/agents/`
  output byte-for-byte; (e) dead-config harness (in `agents:` but not `agent_harnesses`) warned +
  dropped; (f) legacy bare-agent-key block warned + ignored; (g) `--check` catches per-harness drift
  and the legacy shape; (h) the non-Claude fallback warning fires (and is suppressed for `claude`).

## Relationship to #0045

`depends_on: [45]`. #0045 lands the `agent_harnesses` fan-out plumbing (the `project_level_pass`
loop over harness dirs) with a **shared** model across harnesses; #0046 **rewrites that resolution**
to harness-first per-harness values. This knowingly replaces #0045's resolution semantics and its
test (b) ("both harnesses generated with the *same* resolved model") — accepted rework, the price
of layering rather than folding. The implementer's reconcile pass should confirm #0045's fan-out
loop shape before rewriting the reader.

## Out of scope

- Re-introducing any **tier** abstraction — killed with #0043; values stay direct model IDs
  (ADR-0015).
- A **per-harness catch-all default model** (one model for all of a harness's agents) — considered
  and rejected in favor of layer symmetry + explicit per-agent entries.
- **Validating** model IDs against a harness roster — docket stays passthrough (ADR-0008/0015); the
  heuristic warning above is the only signal, and it never blocks.
- The **`build:`** SDD-dispatch surface (#0044) — separate change; shares only the direct-model-ID
  vocabulary.
- Narrowing the **user-level** fan-out to `agent_harnesses` — that is #0045's concern; #0046 keeps
  the user-level pass over every present harness and only reshapes how each file's *values* resolve.

## Open questions (resolve at build)

1. **Live verification (NOT an automated test): non-Claude project-over-user precedence.** The
   two-layer model assumes each harness prefers its project-level `.<H>/agents/` over the user-level
   `~/.<H>/agents/`, as Claude Code does. Confirm on the real harness (Cursor) that a project-level
   per-harness file wins over a user-level one; if not, docket must merge global into the
   project-level file for that harness (scoped follow-up).
2. **ADR.** The harness-first `agents:` shape is a new architectural decision extending ADR-0015.
   Decide at build: a **new ADR** vs a dated **`## Update`** note on ADR-0015 (immutable-except-status
   allows appended updates). ADR-0015's decision (direct model IDs, explicit per-repo fan-out) is not
   reversed — only refined.
3. **Warning trigger precision.** Confirm the warning keys on the resolved **`model`** provenance
   (default/built-in) for non-`claude` harnesses only, and does not misfire when the operator
   deliberately relies on `default` for a Claude-lineage harness alias.

## ADR

ADR-[[0015]] (Accepted) records the governing direction — harness-neutral direct model IDs +
explicit per-repo harness fan-out. #0046 refines the *value* resolution to be per-harness; the
harness-first shape decision is captured at build per Open question 2.
