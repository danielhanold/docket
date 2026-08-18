---
id: 315
slug: claim-to-implemented-workflow
title: 'Claim-to-implemented agent workflow'
status: implemented
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-18
depends_on: [312, 313, 314]
stacked_on:
related: [324]
discovered_from: [303]
adrs: [59, 66, 83, 92, 94, 95]
spec: docs/superpowers/specs/2026-08-17-claim-to-implemented-workflow-design.md
plan: docs/superpowers/plans/2026-08-17-claim-to-implemented-workflow-plan.md
results: docs/results/2026-08-18-claim-to-implemented-workflow-results.md
trivial: false
auto_groomable:
branch: feat/claim-to-implemented-workflow
pr: https://github.com/danielhanold/docket/pull/216
blocked_by:
reconciled: true
claimed_at: 2026-08-18T06:33:00Z
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-17-claim-to-implemented-workflow-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-17-claim-to-implemented-workflow-design.md) |
| Plan | [2026-08-17-claim-to-implemented-workflow-plan.md](https://github.com/danielhanold/docket/blob/feat/claim-to-implemented-workflow/docs/superpowers/plans/2026-08-17-claim-to-implemented-workflow-plan.md) |
| Results | [2026-08-18-claim-to-implemented-workflow-results.md](https://github.com/danielhanold/docket/blob/feat/claim-to-implemented-workflow/docs/results/2026-08-18-claim-to-implemented-workflow-results.md) |
| PR | [#216](https://github.com/danielhanold/docket/pull/216) |
| ADRs | [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0083](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

The essential implementation lifecycle must work end to end through Claude Code without direct
metadata edits or moving model judgment into the binary.

## What changes

Add authoritative implementation context and typed claim, reconciliation, lease, artifact, and
implemented-state operations; expose the landed workspace, gate, evidence, and PR mechanics through
the application and CLI layers; and revise the Go-v1 agent skills to sequence them while Claude
retains reconciliation, planning, build, review, and PR-authorship judgment.

## Out of scope

Behavior owned by changes 0305 through 0314; finalize, rebase, retest, merge, archive, reclaim,
cleanup, persistent halted recovery, and stack closeout from 0316; release and live four-harness
acceptance from 0317; configuration contraction, self-hosting, Bash removal, and hard cutover from
0318; and deferred auto-groom, capture/harvest, terminal publishing, CI-gate, results-skip,
cross-harness, role-rebinding, or Bash-fallback capabilities.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-17

Reconciled against current `main`. Verified against reality:

- Dependencies 0312, 0313, 0314 and related 0324 are all archived `done`; the spec's "landed
  foundation" holds. `internal/domain`, `internal/repository/transaction`, `internal/document`,
  `internal/workspace`, `internal/githubcli`, `internal/evidence`, `internal/process`,
  `internal/app`, and `internal/cli` all exist and expose the named primitives (`EvaluateReadiness`,
  `ClaimEligibility`, `Claim`, `RefreshClaim`, `ResolveEffectiveBase`, `MarkImplemented`, workspace
  prepare/inspect/publish, evidence record/verify, githubcli PR adapter, process gate
  launch/observe/stop).
- Every public operation this change adds is genuinely absent at the CLI/app layer today: `context
  implementation`, `change claim`/`refresh-claim`/`reconcile`/`attach-plan`/`attach-results`/
  `mark-implemented`, `artifact backlink`, `workspace prepare`/`inspect`/`publish`, `evidence
  record`/`verify`, `pr publish`, and `run verify`. `internal/app` does not yet import
  workspace/evidence/githubcli — those services are built but unwired. No scope has been delivered
  elsewhere; no drop needed.
- Asset/embed mechanism confirmed: authored sources at `skills/docket-implement-next/`,
  `skills/docket-build/`, `agents/docket-plan-writer.md`; embedded copies under
  `internal/assets/embedded/tree/`; regenerate via `go generate ./internal/assets/`
  (`cmd/genassets`). Source and embedded assets must be regenerated together.
- No new adjacent follow-up work surfaced that meets the auto-capture materiality bar (the
  status-sweep health items on #189/#44 are pre-existing and separately tracked). Scope, spec, and
  acceptance criteria stand as written.
