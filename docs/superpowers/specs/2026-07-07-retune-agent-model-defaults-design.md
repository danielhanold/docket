# Design — Re-tune default agent models for the Claude 5 lineup (pin explicit versions)

Change: #0042 · slug `retune-agent-model-defaults` · spec drafted 2026-07-07 (interactive brainstorm, owner)

## Problem

docket ships eight model/effort-pinned subagent wrappers (`agents/docket-*.md`) plus two
advisory recommendations in the interactive skills (`docket-new-change`, `docket-groom-next`).
Every one selects its model with a **bare alias** — `opus` or `sonnet`. Those aliases were
chosen when Sonnet 4.6 (and the prior Opus) were the current models.

Sonnet 4.6 is being retired. Because the wrappers use bare aliases, nothing *breaks* — `sonnet`
will silently resolve to **Sonnet 5** and `opus` now resolves to **Opus 4.8** — but that silence
is exactly the problem for a system whose core promise is *clone-identical reproducibility*: the
model each agent runs on can change underneath a pinned change without any commit recording it,
and the current tier assignments have never been re-examined against the new lineup (Opus 4.8,
Sonnet 5, Haiku 4.5, Fable 5), where Sonnet 5 is materially more capable than 4.6 and a cheap
Haiku 4.5 now exists.

This change is the **urgent, standalone re-tune** on today's mechanism (no new machinery). It is
deliberately the simple, fast fix so it can land before the 4.6 sunset; change #0043 later folds
the concrete versions this change pins into a single tier map so the *next* sunset is a one-line
edit. See `related: [43]`.

## Decision

Pin **explicit model IDs** in every built-in default and re-evaluate the tier assignments. Two
substantive judgments, both made by the owner in the brainstorm:

1. **Explicit versions, not bare aliases.** The built-in defaults become full model IDs so a
   clone/agent/device runs the exact model the commit records, and the 4.6→5 jump is a visible,
   reviewed diff rather than a silent alias resolution. The cost — every future sunset edits ~8
   files — is precisely what #0043's tier indirection removes; accepting it here keeps this change
   small and shippable now.

2. **`status` demotes to Haiku 4.5.** `docket-status`'s dominant work — rendering `BOARD.md` — is
   mechanical, and Haiku 4.5 is cheap/fast on the most-frequently-run agent. Its sweep does mutate
   git state (archive + terminal-publish) and run health checks; that residual risk is accepted as
   the deliberate cost/quality trade (revisit if the sweep proves shaky on Haiku).

### The re-tuned built-in table

| Agent wrapper (`agents/docket-*.md`) | model | effort | change |
|---|---|---|---|
| `docket-implement-next` | `claude-opus-4-8` | `xhigh` | pin only (was `opus/xhigh`) |
| `docket-auto-groom` | `claude-opus-4-8` | `xhigh` | pin only |
| `docket-auto-groom-critic` | `claude-opus-4-8` | `xhigh` | pin only |
| `docket-integration-repair` | `claude-opus-4-8` | `xhigh` | pin only |
| `docket-rebase-resolver` | `claude-opus-4-8` | `xhigh` | pin only |
| `docket-adr` | `claude-sonnet-5` | `medium` | pin only (was `sonnet/medium`) |
| `docket-finalize-change` | `claude-sonnet-5` | `medium` | pin only |
| `docket-status` | `claude-haiku-4-5-20251001` | `medium` | **demoted** + pinned (was `sonnet/medium`) |

Advisory recommendations (prose in the interactive skill bodies, not agent files):

| Interactive skill | advisory model |
|---|---|
| `docket-new-change` | `claude-sonnet-5` (was "`sonnet`, effort: model default") |
| `docket-groom-next` | `claude-sonnet-5` (was "`sonnet` / `high`") |

The two autonomy tiers are unchanged in *shape* — the five no-backstop / adversarial / conflict-&-
repair agents stay on the flagship (now Opus 4.8, `xhigh`); the judgment/writing and merge-
orchestration agents stay one tier down (now Sonnet 5, `medium`). Only `status` moves tier.

## Build-time verification (load-bearing)

The Claude Code agent `model:` frontmatter field must accept a **full model ID**
(`claude-sonnet-5`, `claude-opus-4-8`, `claude-haiku-4-5-20251001`), not only the short aliases
`opus|sonnet|haiku|fable`. The environment advertises these exact IDs, so this is expected to work
— but the implementer MUST confirm a wrapper with a full-ID `model:` actually dispatches on that
model before relying on it. If the harness rejects full IDs in agent frontmatter, explicit pinning
is not achievable this way: **surface that and stop** (abort-and-report) rather than silently
falling back to bare aliases, since falling back would defeat the whole change.

## What the implementer edits

- **`agents/docket-*.md` (8 files)** — rewrite the `model:` / `effort:` frontmatter to the table
  above. `effort` values are unchanged everywhere except that `status` stays `medium`.
- **`skills/docket-new-change/SKILL.md`** and **`skills/docket-groom-next/SKILL.md`** — update the
  "Recommended model/effort (advisory)" line to `claude-sonnet-5`. Keep the advisory framing (these
  skills can't force the session model; it stays a recommendation the human owns).
- **`tests/test_sync_agents.sh`** — the load-bearing test changes:
  - The `"$w: model is a known alias"` assertion (currently
    `[[ "$(fm "$f" model)" =~ ^(opus|sonnet|haiku|fable)$ ]]`) must be **relaxed to also accept
    explicit `claude-*` model IDs** — the built-ins no longer carry bare aliases.
  - The five hardcoded built-in assertions (`implement-next`/`auto-groom` = `opus/xhigh`,
    `finalize-change`/`status`/`adr` = `sonnet/medium`) must be updated to the new pinned values —
    in particular **`status` becomes `claude-haiku-4-5-20251001`/`medium`**.
  - The override tests that feed short aliases through config (`{ model: haiku }`, `{ model: fable }`,
    etc.) exercise the *override* path and stay valid — bare aliases remain legal **config input**;
    only the shipped *defaults* pin full IDs.
- **`tests/test_composition_wiring.sh`** — verify only: its regex matches `alias/effort` prose pairs
  (`opus/xhigh`, …) to enforce "tiers are never restated in prose." Full IDs don't match that
  regex, so no change is expected — but confirm the run stays green.

## Non-goals

- **No new mechanism.** No tier map, no manifest, no `sync-agents.sh` change — that is #0043. This
  change only edits the concrete values in the existing files.
- **The `.docket.yml` `agents:` block example in `docket-convention`** demonstrates *user-facing
  config syntax* with short aliases (`{ model: opus }`). Short aliases remain valid config input, so
  the example needs no change; do not "pin" the doc example.
- **The TDD build model** (SDD's implementer/reviewer dispatches) is untouched — that is #0044.
- **No re-tuning of effort tiers** beyond keeping `status` at `medium`; effort re-evaluation is out
  of scope.

## Testing approach (implementer's TDD call)

`tests/test_sync_agents.sh` is the home for built-in-default assertions (shown above). Floor:

1. Each built-in wrapper's `model:` is a recognized full model ID (post-relaxation regex).
2. The eight per-agent (model, effort) pairs match the re-tuned table exactly — including
   `status = claude-haiku-4-5-20251001/medium`.
3. The existing global/per-repo override tests still pass (short-alias config input still resolves).
4. `sync-agents.sh --check` stays green on the repo's own committed project-level files (if any).

No ADR: this applies the existing agent-layer decisions (ADR-0008 lineage / change #0016); it
records no new architectural decision. `adrs: []`.
