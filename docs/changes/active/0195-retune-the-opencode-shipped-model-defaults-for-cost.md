---
id: 195
slug: retune-the-opencode-shipped-model-defaults-for-cost
title: Retune the opencode shipped model defaults for cost
status: proposed
priority: low
type: chore
created: 2026-08-02
updated: 2026-08-09
depends_on: [192]
related: [164, 166, 181]
discovered_from: [192]
adrs: [15, 16]
spec: docs/superpowers/specs/2026-08-09-retune-the-opencode-shipped-model-defaults-for-cost-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-retune-the-opencode-shipped-model-defaults-for-cost-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-retune-the-opencode-shipped-model-defaults-for-cost-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

Change 0192 shipped the `opencode:` block in `agents/harness-defaults.yml` with Kimi K3 on seven of
sixteen rows — every judgment row plus both ladder-top rungs. The block works: it was live-certified
during 0192's merge gate, including a real end-to-end dispatch and by-name agent selection. This is
purely a cost retune of working defaults, not a defect.

The trigger is a measurement. A single live **review** run on Kimi K3 cost **~$0.80** (observed
2026-08-02, recorded in 0192's results file). That matches the published per-task figure for the
model almost exactly, so the cost is the model choice, not a runaway loop or a config fault.

The load-bearing observation is that Kimi is not badly *priced* — it is the best value in the 55–59
intelligence band by a wide margin — it is simply applied to far more rows than need it. Published
per-task figures for the relevant candidates:

| Model | Intelligence index | Cost per task | Δ pts vs Flash | $ per marginal pt |
|---|---|---|---|---|
| DeepSeek V4 Flash 0731 | 49.9 | $0.027 | — | — |
| GPT-5.6 Luna | 51.3 | $0.066 | +1.4 | $0.028 |
| Muse Spark 1.1 (xhigh) | 50.7 | $0.290 | +0.8 | $0.329 |
| Grok 4.5 (high) | 53.8 | $0.450 | +3.9 | $0.108 |
| GPT-5.6 Terra | 54.8 | $0.710 | +4.9 | $0.098 |
| Kimi K3 | 56.9 | $0.860 | +7.0 | $0.119 |
| GPT-5.6 Sol | 58.8 | $1.800 | +8.9 | $0.199 |
| Claude Opus 5 | 60.7 | $2.400 | +10.8 | $0.220 |

Source: the human's `model_intelligence_cost.csv` benchmark, 2026-08-02. Each figure is one model at
one effort — the CSV has no Grok 4.5 *medium* row, so nothing here measures effort as a cost dial.

Two conclusions fall out, and the second reverses an obvious-looking first instinct:

1. **Luna is the outlier.** At 51.3 for 6.6¢ it dominates Muse Spark, GLM-5.2, Gemini 3.6 Flash,
   Nemotron, Haiku, and MiniMax outright. Anywhere the block currently pays Kimi $0.86 for judgment
   that is not irreversible, Luna gets most of the way for 8% of the price.
2. **Kimi should stay at the cap rungs.** The tempting swap — Grok, or GPT-5.6 Terra — does not
   survive the arithmetic. Grok at the top compresses the whole ladder into 49.9 → 53.8, under four
   points from economy to max. Terra widens that only to 4.9 for +58% cost over Grok, which is
   paying real money for noise. The next genuine step up from Kimi is Sol at 2.1× the cost for +1.9
   points. So the ladder top is exactly where Kimi already earns its price, and `build-max` is the
   rarest profile by design — 0192 waived certifying it at all.

## What changes

Data-only edits to the shipped `opencode:` block and its mirrored surfaces. No substrate work: 0192
built the harness registration, the native emitter, and the dispatch wiring, and none of it moves.

Six of sixteen rows change; all sixteen efforts are preserved exactly as shipped. Kimi drops from
seven rows to two:

```yaml
  opencode:
    adr:                   { model: openrouter/openai/gpt-5.6-luna, effort: medium }        # ← was kimi-k3
    auto-groom:            { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    auto-groom-critic:     { model: openrouter/openai/gpt-5.6-luna, effort: high }
    brainstorm-consultant: { model: openrouter/openai/gpt-5.6-luna, effort: medium }        # ← was kimi-k3
    build-economy:         { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    build-standard:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    build-premium:         { model: openrouter/x-ai/grok-4.5, effort: medium }              # ← was kimi-k3
    build-max:             { model: openrouter/moonshotai/kimi-k3, effort: high }
    finalize-change:       { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    implement-next:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    integration-repair:    { model: openrouter/x-ai/grok-4.5, effort: high }                # ← was kimi-k3
    rebase-resolver:       { model: openrouter/x-ai/grok-4.5, effort: high }                # ← was kimi-k3
    review-lean:           { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    review-standard:       { model: openrouter/openai/gpt-5.6-luna, effort: medium }        # ← was kimi-k3
    review-deep:           { model: openrouter/moonshotai/kimi-k3, effort: high }
    status:                { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: low }
```

Resulting spread: DeepSeek Flash ×7, Luna ×4, Grok ×3, Kimi ×2 — four families, up from three.

Expected effect, by published per-task figure:

| Rows | Was | Becomes | Per-task |
|---|---|---|---|
| `adr`, `brainstorm-consultant`, `review-standard` | Kimi $0.86 | Luna | $0.066 (−92%) |
| `build-premium`, `integration-repair`, `rebase-resolver` | Kimi $0.86 | Grok | $0.450 (−48%) |
| `build-max`, `review-deep` | Kimi $0.86 | unchanged | $0.860 |

Since most changes route standard or premium, the review rung actually hit most often is
`review-standard` — so the observed ~$0.80 review should land near a nickel.

Beyond the sidecar itself, the change carries the surfaces that must move with it: the singly
commented `opencode:` mirror block in `.docket.example.yml` (with round-trip evidence), the block's
explanatory comment — which currently states the 0192 rationale "three models, chosen for cheap high
intelligence" and would become false at four — and any guard or test that keys on the shipped
values rather than on shape.

**Invariants that must survive.** `review-deep == build-max` (the cap-rung invariant) holds
unchanged, and holds *without* a family break — unlike the cursor block, which pins
`claude-opus-5-high` at both rungs. That is deliberate: it means this change should need **no ADR**.
Model IDs remain opaque passthrough values (ADR-0015) written as bare, unquoted, space-free scalars
(change 0181); the `opencode:` block stays complete at sixteen rows, as `HD_SHIPPED_HARNESSES`
requires in both directions.

## Out of scope

- **The claude, cursor, and codex blocks.** Untouched. This is an opencode-only retune; 0164 already
  did the cross-harness pass.
- **Any substrate change.** No registration, emitter, dispatch, routing, or `REGISTERED_RUNNERS`
  edits. If the retune appears to need one, that is a signal the design is wrong.
- **Effort retuning.** All sixteen efforts stay as shipped. The benchmark measures one effort per
  model and cannot price effort, so moving efforts here would be unmeasured guessing.
- **Reopening 0192's merge-gate items.** They were cleared live on 2026-08-02 and this change is why
  they were not amended into that branch.
- **A user-layer override recipe.** Anyone wanting a cheaper or richer block already overrides
  field-by-field through the normal config layers; that is documentation work, not this.

## Open questions

All resolved by the 2026-08-09 auto-groom (design + adversarial critic pass; full audit trail in
the linked spec's `## Assumptions`):

1. **`review-standard`: Luna** — committed as the conservative default consistent with the
   change's cost purpose and this stub's own savings table; risk bounded by `review-deep` (Kimi)
   and the human merge gate; reversal is a one-line data edit. Rationale audited in the spec.
2. **`openrouter/x-ai/grok-4.5` is a valid ID** — verified 2026-08-09 against the local
   `opencode models` listing (independently reproduced by the critic); re-verified at build time
   since model catalogs drift.
3. **Effort is never ID-encoded on opencode** — the sync-agents native emitter writes `effort:`
   as `reasoningEffort:` agent frontmatter (the path shipped rows feed; 0192 live-certified it),
   and the listing carries no effort-suffixed Grok IDs. The three Grok rows' `effort:` values
   are the right shape as drafted.
4. **Grok medium cost** — unmeasured but bounded above by the $0.450 high figure, so the savings
   direction holds; the exact −48% applies only to the two `high` rows.
5. **Live re-certification** — yes: carried as a build verify item, as a live dispatch of 0192's
   certification class (not merely `opencode debug agent`, which observes emission, not provider
   acceptance).
- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: the data retune stands; the block is now 17 rows (a `plan-writer` row was added) and the `HD_SHIPPED_HARNESSES` Bash guard is deleted. Regroom the mirrored surfaces (`.docket.example.yml` round-trip, the Go test pinning the block's completeness, the row count).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
