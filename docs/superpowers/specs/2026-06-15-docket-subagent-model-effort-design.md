# Design: docket skills as model/effort-pinned subagents — foundation

**Status:** design (brainstormed 2026-06-15 via `docket-new-change`)
**Change:** 0016
**Related:** change 0017 (composition wiring — the follow-on that rewires nested invocations; `depends_on: 16`), change 0015 (finalize rebase/retest — consumes the abort-and-report + fan-out patterns established here), `docket-convention` (the shared contract gains the `agents:` config block and the agent layer)

## 1. Context / problem

Every docket skill runs **inline in the main conversation** at whatever model and effort the session happens to be on. There is no way to say "run `implement-next` on Opus at maximum effort but render the board on Sonnet at medium." The skills differ enormously in blast radius — `implement-next` and `auto-groom` are fully autonomous with no human backstop until a merge gate, while board rendering and ADR template-fill are near-mechanical — yet they all inherit one undifferentiated session model.

We want per-skill control of **model** and **effort**, configurable without editing skill bodies, with sane built-in defaults. The driving distinction is **autonomy**: the two skills that run to completion with no human catching errors (`implement-next`, `auto-groom`) earn the ceiling (Opus / `xhigh`); everything human-steered has a person in the loop and needs less.

A third axis specific to this codebase breaks the naive "mechanical ⇒ cheapest model" mapping: **git-state-corruption risk**. `finalize-change` and the `status` merge-sweep are cognitively light but run terminal-publish against shared branches — a botched idempotent git procedure corrupts the integration branch. Sonnet is the floor for anything that runs terminal-publish; Haiku is off the table for them.

## 2. Harness capabilities (verified, not assumed)

Confirmed against current Claude Code docs before designing:

- **Subagent frontmatter supports both `model` and `effort`.** `model`: `opus`/`sonnet`/`haiku`/`fable` aliases, full ids, or `inherit` (default). `effort`: `low`/`medium`/`high`/`xhigh`/`max`, validated per model, overrides the session effort. There is **no `auto`** frontmatter effort value — `auto` is a `/effort`-command value meaning "model default"; in frontmatter that is expressed by **omitting `effort`** (inherit).
- **Nested subagents work** (Claude Code ≥ v2.1.172): a foreground subagent can spawn further subagents at any depth. This makes the composition design (§6) real rather than a workaround.
- **Project-level `.claude/agents/` wins over user-level `~/.claude/agents/`** by name, and project-level files are committable. This is the mechanism that makes per-repo overrides reproducible.
- **`skills:` frontmatter injects the full content of the named skills** into the subagent's context at startup (not an allow-list — a content include).
- **Frontmatter is static** — no templating or runtime config reads. Therefore configurability requires a **generator** that writes agent files from resolved config; it cannot be done with live `.docket.yml` reads at invocation time.

## 3. Two mechanisms, by autonomy

| Group | Skills | Form |
|---|---|---|
| **Autonomous / procedural** | `implement-next`, `auto-groom`, `finalize-change`, `status`, `adr` | real subagent (`agents/docket-*.md`), model+effort pinned in frontmatter |
| **Live-dialogue** | `new-change`, `groom-next` | stay inline skills; model/effort is **advisory** |
| **Reference** | `convention` | not an agent — injected via every wrapper's `skills:` list |

`finalize-change` is human-*triggered* but not human-*conversational*: once invoked it runs a deterministic procedure (merge → verify → harvest → archive → publish → clean up → board) with no mid-run questions, so it is a subagent. The cost: a subagent cannot pause to ask, so finalize-as-subagent treats "PR not actually approved / merge conflict / dirty worktree" as **abort-and-report**, never "ask the human." That is a reasonable precondition (you invoke it because you approved) and a named behavior change.

`new-change` and `groom-next` run a brainstorm *with* the human and therefore cannot be fire-and-forget subagents. A skill also **cannot force the session model**. So their config entry is **advisory**: at startup the skill surfaces its recommended model/effort (e.g. "recommended: sonnet/high — `/model`, `/effort` to match") and may emit an `/effort` nudge, but the human owns the session. This is the honest ceiling of control for interactive skills; a human is present to apply it.

## 4. The subagent wrapper

Each autonomous skill gets a **thin wrapper** whose only job is to pin model+effort and load the existing skill — the skill body stays the single source of behavior, never duplicated:

```yaml
---
name: docket-implement-next
description: <copied from the skill, so name-mention / Task subagent_type selection works>
model: opus
effort: xhigh
skills: [docket-implement-next, docket-convention]   # full content injected at startup
---
Execute docket-implement-next to drain the next build-ready change. Follow the skill exactly.
```

### Built-in default table

| Wrapper | model | effort | Primary driver |
|---|---|---|---|
| `docket-implement-next` | opus | xhigh | Autonomous reconcile + escalation judgment, no backstop until merge gate |
| `docket-auto-groom` (designer) | opus | xhigh | Fully autonomous design feeding an autonomous build |
| `docket-auto-groom` (critic) | opus | xhigh | Adversarial gate must be ≥ the designer or it is theater |
| `docket-finalize-change` | sonnet | medium | Procedural, but terminal-publish corrupts shared branches if botched |
| `docket-status` | sonnet | medium | Board/health near-mechanical, but the sweep runs terminal-publish |
| `docket-adr` | sonnet | medium | Allocate/template/regen/publish; Context/Decision/Consequences usually handed down |
| `new-change` (advisory) | sonnet | auto (omit) | Human-steered; wide variance from trivial/scan to full brainstorm |
| `groom-next` (advisory) | sonnet | high | Human-steered; cold-start recap is consistently genuine synthesis |

`auto` ⇒ omit `effort` (inherit/model-default). Any agent without an explicit config entry defaults to `model: inherit`, `effort` omitted.

## 5. Layered config

Frontmatter is static, so "configurable" means a **generator** resolves layers and writes agent files. Precedence **per-repo > global > built-in**:

| Layer | Source | Generates |
|---|---|---|
| Built-in | the §4 table, shipped in docket's `agents/` source | — |
| Global personal | `~/.config/docket/agents.yaml` (optional, XDG) | user-level `~/.claude/agents/docket-*.md` |
| Per-repo | `.docket.yml` `agents:` block (committed) | **project-level** `<repo>/.claude/agents/docket-*.md` |

Schema (in `.docket.yml`, documented in `docket-convention`):

```yaml
agents:
  implement-next: { model: opus,   effort: xhigh }
  auto-groom:     { model: opus,   effort: xhigh }
  status:         { model: sonnet, effort: medium }
  # unlisted -> built-in default; effort: auto -> omit the frontmatter field
```

**Reproducibility resolution.** A per-machine global file alone would let the same autonomous change build on Opus locally and Sonnet on CI — breaking docket's "config identical for every clone" rule for exactly the skills that need it most. Per-repo overrides sidestep this: they generate **committed project-level** agent files, which are clone-identical by construction, and Claude Code's project-over-user precedence applies them for free (no manual layer-merge in the generator for the per-repo case). The global file is convenience only, for repos that pin nothing.

## 6. Composition (designed here, built in 0017)

Nesting is confirmed, so the sub-invocations run at their *own* models rather than inheriting the parent's:

- `implement-next` (opus/xhigh) **spawns** `status` (sonnet/medium) at step 0 and `adr` (sonnet/medium) at step 6 as nested foreground subagents.
- `auto-groom` (opus/xhigh) **spawns a fresh `critic` subagent** (opus/xhigh) — genuine adversarial isolation, both pinned Opus.

**Out of scope (this change and 0017):** the TDD build's model is `superpowers:subagent-driven-development`'s own config, where most token spend lands. Pinning `implement-next` to Opus makes its *reconcile and escalation* Opus; it does **not** make the build Opus. Untouched here.

The actual rewiring is **0017** (`depends_on: 16`): it edits `implement-next` and `auto-groom` skill bodies to dispatch named subagents instead of inline sub-invocations. 0016 only establishes the wrappers, config, and generator so that standalone invocation works; sub-invocations keep running inline (at the parent's model) until 0017 lands.

## 7. `docket-convention` changes

The convention is the single source of the contract, so it gains: the `agents:` config block and its schema; the agent layer and precedence; `auto` ⇒ omit-effort; the generator's role; the abort-and-report semantics for autonomous subagents; and a pointer to the §6 composition (whose detail lands with 0017). No behavior in other skills changes from the doc edit alone.

## 8. The generator / installer — `sync-agents.sh`

A new idempotent script, **separate** from `link-skills.sh` (different artifact — generated copies, not symlinks — and a different idempotency contract — overwrite vs create-if-missing):

1. Read built-in defaults (shipped) ⊕ global `~/.config/docket/agents.yaml` ⊕ (for the per-repo pass) `.docket.yml` `agents:`.
2. For the global/built-in resolution: **write** `~/.claude/agents/docket-*.md` (and any other present harness agent dirs, mirroring `link-skills.sh`'s harness list).
3. For a repo with an `agents:` block: **write** the overridden wrappers as project-level `<repo>/.claude/agents/docket-*.md` (committed).
4. `auto`/unset → omit `effort`; unlisted agent → built-in default.

Idempotent: re-run after editing any layer. Unlike `link-skills.sh`'s symlinks, agent files are **generated copies** (symlinking can't bake per-layer model/effort), so the generator owns them and overwrites on each run.

**`--check` mode (CI gate).** `sync-agents.sh --check` re-resolves config and exits non-zero with a diff if any committed project-level agent file drifts from what the config would generate — guarding the reproducibility guarantee for per-repo overrides. Write mode and `--check` share one resolver so they cannot disagree.

**Generation lifecycle — on-demand, not per-session.** `sync-agents.sh` runs **on demand**: at docket install time and whenever a config layer is edited — the same mental model as `link-skills.sh`. It does **not** hook into session start. Generated agents are copies (they bake resolved model/effort, so they can't symlink-track config the way skills do), but the committed project-level agents are exactly what gives per-repo overrides their reproducibility — silently regenerating them every session would mutate committed files out of band and race the commits that make the override clone-identical. The drift backstop is therefore **CI `--check`**, not a session hook: edit config → run `sync-agents.sh` → commit; CI fails if committed agents fall out of sync. (A per-session refresh scoped to the user-level `~/.claude/agents/` layer only would be safe, but is out of scope — optional polish, not core.)

## 9. Testing strategy

Following `tests/test_link_skills.sh`'s seam (`DOCKET_HARNESS_ROOT` overrides `$HOME`):

- **Generation**: built-in defaults produce the §4 table verbatim; `effort: auto`/unset omits the field; unlisted agent gets `model: inherit`.
- **Precedence**: a `.docket.yml agents:` override produces a project-level file; a global `~/.config/docket/agents.yaml` produces a user-level file; project beats user for the same name.
- **Idempotency**: second run is a no-op (byte-identical output).
- **`--check`**: passes when committed agents match resolved config; fails with a diff after an out-of-band edit to a committed agent file.
- **Wrapper validity**: each generated file has required `name`/`description`, a resolvable `model`, an `effort` in the allowed set or absent, and a `skills:` list including `docket-convention`.
- **Advisory entries**: `new-change`/`groom-next` produce **no** agent file (they stay skills) — the generator must skip them.

## 10. Decisions (resolved at brainstorm)

- **Effort default:** table-authoritative — each wrapper ships its specific table effort baked in; `new-change` and any unlisted agent ship with no `effort` (auto).
- **Generator:** a separate `sync-agents.sh`, not an extension of `link-skills.sh`.
- **Global file:** `~/.config/docket/agents.yaml` (XDG); no broader global docket config file is introduced.
- **`--check` mode:** in scope — CI gate asserting committed project-level agents match resolved config.
- **Generation lifecycle:** on-demand (install / config-change), no session hook; CI `--check` is the drift backstop.

Remaining for plan time:

- Exact harness agent-dir list `sync-agents.sh` writes into (mirror `link-skills.sh`'s `HARNESS_SKILL_DIRS`, swapping `skills` → `agents`).
- Whether `auto-groom`'s critic is a distinct committed wrapper file or a variant the skill spawns — settled with 0017's rewiring, since 0016 doesn't yet spawn it.

## 11. Scope

- **0016 (this change):** wrappers for the 5 autonomous skills, the layered config schema + precedence, the generator, the advisory mechanism for the 2 interactive skills, and the `docket-convention` doc update. Standalone invocation of each subagent works at its pinned model/effort. Sub-invocations still run inline.
- **0017 (`depends_on: 16`):** rewire `implement-next → status/adr` and `auto-groom → critic` to nested subagent dispatch.
