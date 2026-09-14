---
id: 423
slug: 'certify-native-multi-agent-v2-orchestration-through-docket-i'
title: 'Certify native Multi-Agent V2 orchestration through Docket ImplementNext'
status: 'done'
priority: 'critical'
type: 'chore'
created: '2026-09-11'
updated: '2026-09-14'
depends_on: []
stacked_on:
related: [323, 384, 393, 412, 424, 425, 426]
discovered_from: [323, 412]
adrs: [59, 60, 94, 114]
spec: 'docs/superpowers/specs/2026-09-13-certify-native-multi-agent-v2-orchestration-through-docket-i-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/certify-native-multi-agent-v2-orchestration-through-docket-i'
pr:
blocked_by:
reconciled: false
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-13-certify-native-multi-agent-v2-orchestration-through-docket-i-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-13-certify-native-multi-agent-v2-orchestration-through-docket-i-design.md) |
| ADRs | [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md), [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md) |
<!-- docket:artifacts:end -->

## Why

Docket adopted Codex app-server root entry after real ImplementNext runs could not launch docket-plan-writer as a nested child. Subsequent controlled tests found a more specific cause: the affected coordinator was pinned to a Multi-Agent V1 model, while an otherwise equivalent Multi-Agent V2 model received collaboration tools and successfully launched the same registered child. Before Docket reverses a large body of launch architecture, it needs durable, production-shaped evidence that the distinction is model capability rather than prompt wording, reasoning effort, role complexity, or an artifact of the earlier minimal probes.

## What changes

Deliver a repeatable manually operated native Codex POC that runs the actual ImplementNext coordinator, actual plan writer and corrected actual standard worker continuously from a fresh primary checkout. Under user-approved option 2, feature children validate their assigned worktree and explicitly target all feature operations there even when their startup cwd is primary. Carry complete planner and child-capability payloads, the unchanged outer gate context, real TDD/commit/acknowledgement, configured exact-commit gates, a results checkpoint and terminal bounded halt. Package setup, deterministic validation and sanitized evidence. No additional Luna control, production routing change or Docket build workflow is part of 423.

## Out of scope

Production dispatch or model-policy implementation (425 and 424), agent.enter calls, native startup-placement workarounds, hard isolation or parallel-safety claims, further Luna/control probes, review/PR/merge of the disposable candidate, and fabricated production merge/build evidence. Manual closure follows accepted POC evidence and an explicitly documented metadata exception.

## Reconcile log

### 2026-09-14

User-directed 2026-09-14 scope reconciliation: option2 replaces native startup equality, Luna/control retests end, continuous corrected positive path remains required, reusable POC package is delivered manually, and done requires explicit evidence acceptance plus truthful manual metadata close-out rather than Docket build/finalize fiction. Production work feeds425; historical evidence and accepted ADRs remain unchanged.

Manual POC completion

Accepted and manually completed on 2026-09-14 under the user instruction: “Continue with final evidence packaging and manual closeout.” This is the approved POC lifecycle exception; no production PR, merge or Docket build/finalize run is asserted.

The continuous native ImplementNext → planner → standard worker run passed the agreed option 2 contract, including real TDD, the committed and locally published results checkpoint, the configured full suite at that checkpoint, and an unchanged primary audit. The fixture then halted deliberately under its typed boundary. Acceptance is `continuous-functional-passed`, with `evidence_audit_complete: false` and the documented observation limitations accepted.

The [final report and reusable evidence package](https://github.com/danielhanold/docket/blob/docket/docs/codex/fixtures/native-implement-next/FINAL-RESULTS.md) preserve the evidence, independent review, Git history, reproducible fixture and checksums. This metadata-resident POC report is intentionally linked here rather than through a main-branch lifecycle results field. The original fixtures are retained; the historical branch label remains only as provenance, with the active claim and reconciliation bookkeeping cleared.

Change 424 owns production model capability policy. Change 425 consumes this evidence for Codex-only native dispatch and explicit feature-worktree binding; it still depends on 424. This POC does not certify hard isolation, parallel safety or production readiness.
