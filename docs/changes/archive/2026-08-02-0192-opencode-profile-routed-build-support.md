---
id: 192
slug: opencode-profile-routed-build-support
title: opencode support for profile-routed Docket builds
status: done
priority: medium
type: feat
created: 2026-08-02
updated: 2026-08-02
depends_on: []
related: [77, 167, 168, 169]
discovered_from: []
adrs: [15, 36, 63, 64]
spec: docs/superpowers/specs/2026-08-02-opencode-profile-routed-build-support-design.md
plan: docs/superpowers/plans/2026-08-02-opencode-profile-routed-build-support.md
results: docs/results/2026-08-02-opencode-profile-routed-build-support-results.md
trivial: false
auto_groomable:
branch: feat/opencode-profile-routed-build-support
pr: https://github.com/danielhanold/docket/pull/150
blocked_by:
claimed_at: 
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-02-opencode-profile-routed-build-support-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-02-opencode-profile-routed-build-support-design.md) |
| Plan | [2026-08-02-opencode-profile-routed-build-support.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-02-opencode-profile-routed-build-support.md) |
| Results | [2026-08-02-opencode-profile-routed-build-support-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-02-opencode-profile-routed-build-support-results.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md) |
<!-- docket:artifacts:end -->

## Why

Docket ships three complete agent harnesses today — Claude, Cursor, and Codex (change 0169) — and
the harness-indexed sidecar (ADR-0064) makes adding a fourth a well-worn path. opencode is a
terminal harness with native markdown-defined subagents, per-project `.opencode/agents/`
generation targets, and the same committed project-root `AGENTS.md` dispatch surface Codex already
uses — and through OpenRouter it unlocks radically cheaper defaults: the selected table anchors on
DeepSeek V4 Flash (~82% of frontier intelligence at ~1% of frontier cost) and prices a full change
lifecycle at roughly $1–2.

Unlike 0169, which shipped defaults into an existing substrate, opencode needs the substrate too:
harness registration, a native emitter, and dispatch wiring, plus the complete sixteen-agent
shipped default block and live certification.

## What changes

- Register `opencode` as a known and shipped harness; the sidecar validator then requires a
  complete sixteen-agent `opencode:` block in both directions.
- Add an opencode emitter to the `emit_for_harness` registry writing `.opencode/agents/`
  markdown definitions: `mode: subagent`, resolved model verbatim, resolved effort as a
  reasoning-effort passthrough option whose exact spelling is verified at build time against a
  real opencode installation.
- Ship the selected three-model default table: DeepSeek V4 Flash on every volume row (status,
  grooming, orchestration, economy/standard builds, lean review), Kimi K3 on the judgment rows
  and the ladder top at two efforts (premium/review-standard at medium, max/review-deep at high),
  and GPT-5.6 Luna as the family-diverse auto-groom critic. All IDs OpenRouter-prefixed.
- Add opencode to the AGENTS.md dispatch harnesses; the managed committed block becomes
  harness-neutral, serving Codex and opencode from one block.
- Mirror the block in `.docket.example.yml`, extend the mirror/round-trip/leak guards with
  mutation evidence, add `docs/opencode/setup.md`, and update maintained docs.
- Gitignore the generated `.opencode/agents/docket-*.md` definitions alongside the existing
  per-harness entries (added at reconcile).
- Certify economy, standard, and premium named dispatches in a live opencode session; record
  explicit waivers for the max rung, review rungs, classification, and escalation.

## Out of scope

- A Claude-to-opencode whole-run runner (`REGISTERED_RUNNERS` unchanged); possible follow-up.
- Changes to the shared task-worker contract, the `docket-build` controller's routing, or the
  Claude/Cursor/Codex mappings.
- A vendor model registry, runtime allowlist, or automatic fallback for unavailable model IDs.

## Open questions

- Exact OpenRouter model-ID spellings and the reasoning-effort passthrough option key must be
  verified against the installed `opencode models openrouter` catalog at reconcile and again
  before certification; any drift stops for a human, never a silent substitution.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-02 — reconcile at claim

Scope holds; no drift that changes the design. Verified against current `origin/main` and a live
opencode installation.

**Substrate confirmed exactly as the spec describes it.** `scripts/lib/harness-defaults.sh` carries
`HD_KNOWN_HARNESSES="claude cursor codex"` (line 20) and `HD_SHIPPED_HARNESSES="claude cursor codex"`
(line 27); `sync-agents.sh` carries `is_valid_harness`, `AGENTS_MD_DISPATCH_HARNESSES="codex"` with
its `harness_gets_agents_md` reader, the `emit_for_harness` registry dispatching to
`emit_codex_toml` / `emit_cursor_md` over a Claude-shaped default, `harness_ext` (codex⇒toml, else
md), and `sync_codex_agents_md_dispatch`. `REGISTERED_RUNNERS="codex cursor"` is untouched by this
change, per the non-goal. All four insertion points named in the design are real and unchanged.

**Model IDs verified live (blocking item from `## Open questions`).** opencode **1.18.11** installed
at `/opt/homebrew/bin/opencode`. All three selected OpenRouter IDs are present verbatim in the
installed catalog (`opencode models`, 360 entries): `openrouter/deepseek/deepseek-v4-flash-0731`,
`openrouter/moonshotai/kimi-k3`, `openrouter/openai/gpt-5.6-luna`. No substitution needed; the
shipped table stands as designed.

**Effort passthrough remains the one open build-time gate.** The exact frontmatter option key that
reaches the OpenRouter provider (docs document `reasoningEffort`) was NOT settled at reconcile — it
stays a plan task requiring proof against the real installation before the emitter hardcodes a
spelling, and a non-functional passthrough stops for a human rather than degrading to unpinned
effort, exactly as `## Failure behavior` requires.

**One addition the spec did not name.** Generated wrappers are machine-local and gitignored per
harness — `.gitignore` lines 10/11/15/16 cover `.codex/agents/docket-*.{md,toml}`,
`.cursor/agents/docket-*.md`, and `.cursor/rules/docket-dispatch.mdc`. `.opencode/agents/docket-*.md`
must join them, or every generated definition shows up as untracked repo noise. Folded into scope as
part of the emitter work; too small to be its own change.
