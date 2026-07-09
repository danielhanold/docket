# Global config layer — `~/.config/docket/config.yml` — design

**Date:** 2026-07-09
**Change:** #0050
**Status:** approved (brainstormed with Daniel 2026-07-09)

## Problem

docket has no user-level configuration story a user can discover or predict. `.docket.yml`
is per-repo only, and the sole global file — `~/.config/docket/agents.yaml` — covers one
concern (agent model/effort), uses a *different shape* than the `.docket.yml` `agents:`
block it mirrors (no `agents:` wrapper key), and its format is documented only in a YAML
comment inside docket-convention's Agent layer example. The natural assumption — drop a
full `.docket.yml` at `~/.config/docket/` and have it apply everywhere — fails **silently**:
nothing reads it, nothing warns. The docket author hit exactly this (2026-07-09).

## Decision summary

One global file, **`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`**, accepting the
**full `.docket.yml` schema**. Three-layer, per-key resolution: **per-repo > global >
built-in**. Coordination keys are fenced (warned-and-ignored globally). `agents.yaml` is
auto-migrated into the new file. Misplacement fails loud.

## 1. File and precedence

- Canonical path: `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` (visible,
  XDG-conventional — cf. `~/.config/git/config`; matches `sync-agents.sh`'s existing
  `XDG_CONFIG_HOME` handling).
- Schema: identical to `.docket.yml` — same keys, same shapes. The `agents:` block keeps
  its wrapper key (unlike old `agents.yaml`, whose top-level map was the source of the
  shape confusion).
- Resolution is **per-key**: a key set in the repo's committed `.docket.yml` wins; else the
  global value (if the key is global-able); else the built-in default. Map-valued keys
  (`skills:`, `agents:`) merge **field-by-field**, the same first-non-empty-wins rule the
  Agent layer already uses.
- **Single reader:** `docket-config.sh --export` implements the layer once; every skill
  receives resolved values at Step 0 unchanged in interface. No other consumer parses the
  global file — except `sync-agents.sh`, which reads only its `agents:` block (see §3) and
  its `agent_harnesses:` key (user-level-pass scope, see §2).
- The global file is read from the local filesystem (it is per-machine by definition —
  there is no authoritative-ref concern as with `.docket.yml`'s `origin/HEAD` read).

## 2. The coordination-key fence

**Classification rule** (ADR-worthy, to be recorded at build time): *a key is
**per-repo-only** when its effect is written to shared state — commits on shared branches
or GitHub objects; it is **global-able** when its effect is confined to the local run.*

| Class | Keys | Global treatment |
|---|---|---|
| Per-repo-only | `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project` | **Warned-and-ignored**: loud stderr warning from `docket-config.sh` naming the key and the reason ("per-repo-only — set it in the repo's committed .docket.yml") |
| Global-able | `skills:` (all five roles), `agents:` (full harness-first block), `auto_groom`, `finalize.gate`, `finalize.test_command`, `board_surfaces` (minus the `github` token), `agent_harnesses` (user-level-pass scope only) | Honored as the middle layer |

Rationale for the borderline calls:

- `board_surfaces` is global-able **except the `github` token**: `BOARD.md` is a derived
  view regenerated deterministically from the change files, so per-machine divergence is
  self-healing staleness, not corruption. The `github` token from the global layer is
  warned-and-ignored — it mints issues and a Projects board (external objects, not
  self-healing) and must stay a repo opt-in. Global `[]` and `[inline]` work.
- `agent_harnesses` is global-able with a **scope split**: the global value overrides the
  user-level pass's presence-on-disk selector in `sync-agents.sh` — controlling which
  `~/.<H>/agents/` dirs get user-level wrappers (and the user-level Cursor dispatch rule).
  It never influences the per-repo committed pass, where only the repo's own
  `agent_harnesses` governs — a global value shaping *committed* files would fail
  `sync-agents.sh --check` on every other machine. The two scopes do not cross; the
  per-repo key keeps its exact current meaning.
- `auto_groom` is global-able: it gates loops the user runs on their own machine; its
  writes (specs) are legitimate docket commits either way. Policy divergence, not state
  corruption.
- `finalize.test_command` is global-able but usually wrong globally; a wrong command makes
  the finalize gate fail closed (abort-and-report) — a safe failure, not corruption.

The refined rule, precisely: *fenced is any global value that would change **shared
state** — commits on shared branches whose content is not deterministically re-derivable,
committed generated files, or external (GitHub) objects. Self-healing derived views and
per-machine uncommitted files are global-able.*

Warn-and-ignore (never abort) is docket's established posture for config noise
(`board_surfaces` unknown tokens, unknown skill role keys, pre-0046 bare agent keys).
The warning repeats at every skill's Step 0 until fixed — loud by construction.

## 3. `agents.yaml` auto-migration

Owned by `sync-agents.sh` (so `install.sh` inherits it). Idempotent:

1. If `agents.yaml` exists and `config.yml` has **no** `agents:` block → rewrite the old
   file's top-level harness-first map under an `agents:` key in `config.yml` (creating the
   file if needed), rename the old file to `agents.yaml.migrated` (backup; git-less users
   keep a copy), and log loudly what moved where.
2. If `config.yml` already has an `agents:` block and a live `agents.yaml` is also
   present → warn that `agents.yaml` is stale and unread; do not read it. (The `.migrated`
   rename in step 1 makes this state unreachable via the migration itself.)
3. No dual-read fallback: after this change, `sync-agents.sh` reads global agent config
   **only** from `config.yml` (`under_agents=1`, same as the per-repo path — the existing
   parameterization already supports this).

Comment loss during the rewrite is accepted: the map is small, and the original file
survives as `.migrated`.

## 4. Fail-loud guards

- `~/.config/docket/.docket.yml` present → `docket-config.sh` warns: "global config is
  `config.yml`, not `.docket.yml` — did you mean `~/.config/docket/config.yml`?". The file
  is not read.
- Unknown keys in `config.yml` → same warn-and-ignore as `.docket.yml`.
- A malformed/unreadable `config.yml` warns and falls back to built-ins for the global
  layer (per-repo still honored) — a broken personal file must not brick every repo.

## 5. Documentation

- README: new "Global config" section — complete copy-pasteable `config.yml` example, the
  three-layer precedence, and the explicit statement that coordination keys are
  per-repo-only (with the one-line why). The "Tuning an agent's model & effort" section is
  updated to point at `config.yml`, and gains the clarification that `sync-agents.sh`
  always writes BOTH layers — user-level wrappers into present harness dirs AND (for
  opted-in repos) committed project-level wrappers, with project winning at runtime — the
  both-passes behavior that reads as "it wrote into my repo instead of globally."
- docket-convention: Configuration section gains the three-layer story; the Agent layer
  section drops the `agents.yaml`-shape comment in favor of the unified shape.
- `scripts/docket-config.md` and the `sync-agents.sh` header document the mechanics
  (read order, fence list, migration).

## 6. Testing

- `docket-config.sh` fixtures: global-only key honored; per-repo overrides global;
  field-by-field `skills:`/`agents:` merge across layers; fenced key warned-and-ignored;
  global `board_surfaces` honored minus a warned-and-ignored `github` token;
  `.docket.yml`-misplacement warning; malformed global file falls back with warning;
  `XDG_CONFIG_HOME` honored.
- `sync-agents.sh` fixtures: migration happy path; idempotency (second run no-ops); stale
  `agents.yaml` warning; global agents read from `config.yml` only; global
  `agent_harnesses` narrows/extends the user-level pass while leaving the per-repo
  committed pass untouched.

## Out of scope

- Any new configuration keys.
- Changing what the per-repo `.docket.yml` supports or where it lives.
- A per-repo *uncommitted* local override file (e.g. `.docket.local.yml`) — no demonstrated
  need; would reopen the clone-identical question.

## Approaches considered and rejected

- **Honor everything globally** — a global `metadata_branch`/`changes_dir` silently splits
  the backlog across machines; the exact failure the committed-file rule exists to prevent.
- **Bootstrap-template semantics for coordination keys** (bake global values into a fresh
  repo's committed `.docket.yml`) — more machinery; deferred until a real need appears.
- **Per-consumer global reads** — N merge implementations that drift; the single-reader
  contract (`docket-config.sh`) is the whole point of change 0026's config-resolution
  script.
- **A materializing generator** (write resolved config per repo, like agent wrappers) —
  wrappers must be static files for harness pinning; runtime config has a single runtime
  reader, so generation buys nothing.
- **Keeping `agents.yaml` as a peer file** — permanently two global files with two shapes;
  the confusion this change exists to kill.
