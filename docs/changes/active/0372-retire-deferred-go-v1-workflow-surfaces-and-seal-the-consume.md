---
id: 372
slug: 'retire-deferred-go-v1-workflow-surfaces-and-seal-the-consume'
title: 'Retire deferred Go v1 workflow surfaces and seal the consumer cutover'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: [371]
stacked_on:
related: [312, 316, 326, 369, 370]
discovered_from: [369]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/retire-deferred-go-v1-workflow-surfaces-and-seal-the-consume'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-30T13:28:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Several Bash-era workflow legs remain active even though their Go v1 capabilities were explicitly deferred: automatic stub capture, automated learnings-index maintenance, and terminal publishing. Treating them as missing Go verbs would silently expand v1 and keep the Bash facade alive indefinitely.

## What changes

- Retire maintained automatic-capture/minting, automated learning (harvest, index rendering,
  capacity, promotion), terminal-publish, and publication-deferral workflow legs without
  implementing replacements — in canonical skill/agent/reference sources AND their generated
  embedded twins under `internal/assets/embedded/tree/`.
- Preserve legacy configuration keys, stored values, indexes, markers, learning records, and
  historical evidence while preventing any enabled value from falling back to Bash.
- Provide capability-specific fail-closed diagnostics and supported explicit alternatives.
- Add the final shape-derived, mutation-tested seal. The seal's prohibited set is the facade
  op-structure **narrowed to the three retired feature families** (`mint-stub`,
  `render-learnings-index`, `terminal-publish`, `mark-publish-deferred`) plus
  enabled-deferred-key-to-Bash wiring — NOT the still-supported facade operations
  (`preflight`, `env`, `board-refresh`, `docket-status`) that remain maintained callers until
  change 370 deletes the facade. Structural historical/frozen exclusions cover the whole
  `scripts/` tree and the frozen parity/deletion test corpus.
- Individually audit and formally disposition conflicting facade-era ADRs 0014, 0029, 0030, 0033;
  preserve 0036, 0074, 0099 as still-current decisions.
- Housekeeping surfaced during reconcile: remove the stale `docket-status` harvest leg and its
  dangling pointer to a *Harvest learnings* step that no longer exists in the finalize skill; and
  invert/retire `tests/test_skill_facade_wiring.sh`, which currently positively *requires* the
  retired instruction shapes and would otherwise contradict the seal.

## Out of scope

Retained lifecycle-operation migration owned by change 369 (already merged — its ADR-index-render removal and done-path close-out are NOT redone here); native host dispatch cutover owned by change 371 (already merged — the `runner:` delegation facade is already retired, out of this scope); implementation of the deferred capabilities; physical Bash facade deletion and the general repo-wide zero-`docket.sh` seal (both owned by change 370); removal of configuration keys or persisted records; release, rollback, or four-host self-host acceptance. The supported atomic ADR-index render (`internal/render/adrindex.go`) and the Go learning read/validate path must be PRESERVED and must not be caught by the seal.

## Design decisions

Each retired surface becomes either a supported explicit alternative, preserved inactive
configuration, or an explicitly unsupported request rejected before mutation. The final seal derives
prohibited executable shapes and structural historical/frozen exclusions instead of hand-listing
caller files, and every exclusion is protected by negative controls. The run halts rather than grow
if retirement requires a new subsystem or conflicts with a still-current ADR.

## Reconcile log

### 2026-08-30

### 2026-08-30 — reconcile (docket-implement-next)

Reconciled against current `main`/`docket` reality via a whole-repo reconnaissance sweep.

- **Preconditions confirmed:** changes 369 and 371 are both merged (archived). 370 is `proposed` and `depends_on: 372`, so this PR must not delete the frozen Bash tree.
- **Design still valid** — scope-adjustable, not fundamentally invalidated. All three deferred families remain live in maintained instructions (`skills/docket-convention/references/{auto-capture,learnings,terminal-close-out}.md`, `docket-implement-next`, `docket-status`, `docket-convention/SKILL.md`) with byte-identical generated twins under `internal/assets/embedded/tree/`; regenerate twice.
- **Scope shrinks:** the redundant standalone ADR-index render (retired by 369) and the `runner:` dispatch facade (retired by 371) are already gone; do not redo them.
- **Seal scoping (key judgment):** the 0372 seal is narrowed to the three retired op-tokens (`mint-stub`, `render-learnings-index`, `terminal-publish`, `mark-publish-deferred`) + enabled-deferred-key→Bash wiring, and must EXCLUDE the still-supported facade ops (`preflight`, `env`, `board-refresh`, `docket-status`) that stay until 370, plus the supported atomic ADR-index render and the Go learning read/validate path. This is faithful to the spec's "narrowed through the explicit retirement classification" language.
- **Two additions folded in:** (1) the `docket-status` SKILL harvest leg carries a dangling pointer to a *Harvest learnings* finalize step that no longer exists — remove the leg and the pointer; (2) `tests/test_skill_facade_wiring.sh` positively asserts the retired routing shapes and must be inverted/retired so it does not fight the seal.
- **No new caller** of any retired surface has appeared since the spec was authored today.
- Relations unchanged: `depends_on:[371]`, `related:[312,316,326,369,370]`, `adrs:[14,29,30,33,36,74,99]`, `discovered_from:[369]`.
