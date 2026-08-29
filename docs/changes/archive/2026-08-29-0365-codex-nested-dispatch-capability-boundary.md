---
id: 365
slug: codex-nested-dispatch-capability-boundary
title: 'Make nested Docket dispatch reliable for every Codex agent invocation'
status: 'done'
priority: critical
type: fix
created: '2026-08-29'
updated: '2026-08-29'
depends_on: []
stacked_on:
related: [353, 359]
discovered_from: [361]
adrs: [36, 59, 60, 94]
spec: docs/superpowers/specs/2026-08-29-codex-nested-dispatch-capability-boundary-design.md
plan: 'docs/superpowers/plans/2026-08-29-codex-nested-dispatch-capability-boundary.md'
results: 'docs/results/2026-08-29-codex-nested-dispatch-capability-boundary-results.md'
trivial: false
auto_groomable:
branch: 'fix/codex-nested-dispatch-capability-boundary'
pr: 'https://github.com/danielhanold/docket/pull/251'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-codex-nested-dispatch-capability-boundary-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-codex-nested-dispatch-capability-boundary-design.md) |
| Plan | [2026-08-29-codex-nested-dispatch-capability-boundary.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-29-codex-nested-dispatch-capability-boundary.md) |
| Results | [2026-08-29-codex-nested-dispatch-capability-boundary-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-29-codex-nested-dispatch-capability-boundary-results.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

The Go installer emits and registers `docket-plan-writer`, but a live Codex implement-next run
halted before planning because the parent inspected a nested JavaScript tool inventory, did not see
Codex's top-level collaboration controls there, and falsely declared native dispatch unavailable.
No plan-writer dispatch was attempted or rejected. The same false capability inference can affect
every Docket skill that composes another agent.

This blocks both user-facing entry paths Docket currently supports: prose routed through the
managed repository dispatch block and direct `@docket-…` agent invocation. The earlier change 0353
treated raw named-agent invocation as operator error, but current Docket routing deliberately uses
that path, so all registered Codex agents must carry a correct nested-dispatch contract.

## What changes

Teach every generated Codex agent to use direct named-agent dispatch from its active top-level tool
surface and to reject nested orchestration inventories as evidence of unavailability. Strengthen
the shared capability-resolution rule by shape, preserve the existing tiered postures for genuine
dispatch rejection, and cover every current composition family through inventory-derived tests and
live fresh-process validation of both prose and direct `@agent` invocation. Update Codex setup and
validation documentation, including the requirement to restart the Codex process after wrapper
installation.

## Out of scope

Runner/subprocess fallbacks for clients that genuinely reject nested dispatch; changing any
model/effort pin, agent topology, return protocol, tier assignment, or worktree scope; authorizing
implicit inline execution; adding skill wrappers for agent-only workers; and the separate run-gate
continuation work tracked by change 0359.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-29

Reconciled against current `main` (integration branch) and the cited context; scope holds unchanged, no obsolescence or fundamental invalidation.

Verified current reality:

- **Prototype precondition already satisfied.** The spec's "Prototype disposition" requires the diagnostic prototype (a Codex renderer paragraph, one Go test, regenerated goldens) to be removed from `main` before implementation. It is already absent: no "direct named-agent dispatch", "nested orchestration inventory", "top-level tool surface", or "collaboration controls" text exists in `internal/harness/codex/` or in any golden on `main`. The implementer starts from a clean base and writes the failing tests first, as the spec directs.
- **Renderer producer confirmed.** The Codex-specific emitter is `internal/harness/codex/codex.go` (`renderAgent`); the shared agent bodies live in `agents/docket-*.md` and stay harness-neutral, so the new developer-instruction paragraph is prepended in the Codex renderer only. Goldens: `internal/harness/codex/testdata/golden/` (17 `docket-*.toml`); comparison test `internal/harness/codex/codex_test.go` (`TestCodexGoldenAgents`, regenerated with `-update`).
- **Non-Codex goldens must stay byte-unchanged.** Claude/Cursor/OpenCode goldens under `internal/harness/<h>/testdata/golden/` (17 `docket-*.md` each) — no diff expected.
- **Convention producer confirmed.** The "Dispatch-capability resolution (change 0137)" section is canonical in `skills/docket-convention/SKILL.md`, with a byte-identical embedded mirror at `internal/assets/embedded/tree/skills/docket-convention/SKILL.md` that MUST be regenerated in lockstep (the embedded tree is generated from the canonical skill). Existing guard: `tests/test_dispatch_capability.sh` — extend it for the active/top-level-surface vs nested-inventory distinction; sibling phrase guards live in `tests/test_docket_build.sh`, `tests/test_docket_review.sh`, `tests/test_skill_size_budgets.sh`.
- **Codex docs confirmed present and lacking the target language.** `docs/codex/setup.md` and `docs/codex/validation-runbook.md` exist; neither currently states the two supported invocation paths (prose routing + direct `@agent`), the nested-dispatch-uses-top-level-surface rule, or the fresh-process requirement. Runbook structure guard: `tests/test_codex_runbook.sh`.
- **Relations unchanged.** related=[353 (killed), 359 (proposed)] still accurate; discovered_from=[361]; adrs=[36,59,60,94] are CITED (retained authoritative), not produced. Spec expects no new ADR unless a decision that changes one of those rules surfaces — none did, so no ADR is planned. depends_on empty; not stacked.
- Suite runner is `scripts/run-tests.sh` (parallel shell suite) plus Go `*_test.go`; treat any `SERIAL CONFIRMED OVER BUDGET` line as a failure to address, `BUDGET WATCH` as screening only.
