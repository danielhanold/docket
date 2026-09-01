---
id: 384
slug: 'launch-compositional-docket-agents-in-coordinator-capable-ha'
title: 'Launch compositional Docket agents in coordinator-capable harness contexts'
status: 'implemented'
priority: 'critical'
type: 'fix'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [359, 364]
discovered_from: [365]
adrs: [36, 59, 60, 94]
spec: 'docs/superpowers/specs/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md'
plan: 'docs/superpowers/plans/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha.md'
results: 'docs/results/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/launch-compositional-docket-agents-in-coordinator-capable-ha'
pr: 'https://github.com/danielhanold/docket/pull/260'
blocked_by:
reconciled: true
claimed_at: '2026-08-31T16:27:17Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md) |
| Plan | [2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha.md) |
| Results | [2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-results.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

Change 0365 taught generated Codex wrappers to ask for nested dispatch, but its mandatory fresh-process certification was left unchecked. The first real test was the implement-next run for change 0364: the root launched the registered docket-implement-next agent, but that session had no collaboration control with which to launch docket-plan-writer, so it halted before planning. Codex documents nested agents and the root exposes the registered Docket roles, making Docket's launch method or context the defect to resolve.

## What changes

Separate workflow semantics from harness launch mechanics. Prototype the supported Codex entry paths, identify a native coordinator-capable launch that preserves nested named-agent controls, encode entry and child invocation through the harness adapter and generated surfaces, and certify root to Docket coordinator to registered child in a fresh process. Keep caller payload and return contracts and child role bodies harness-neutral; use generated per-agent metadata only where a harness must distinguish coordinator and leaf launches.

## Out of scope

Changing existing agent topology, payload or return protocols, model or effort pins, worktree scopes, Tier-C authorization, or skill bindings; adding a parent-relay, generic-agent, shell-runner, or cross-harness fallback unless every accessible native coordinator launch is conclusively unavailable; and the separate run-gate continuation work tracked by change 0359.

## Reconcile log

### 2026-08-31

### 2026-08-31 — reconcile (docket-implement-next)

Reconciled against current reality before planning.

- **Environment premise holds.** Codex CLI 0.151.0 is installed on this machine, `~/.codex/config.toml` sets `multi_agent = true`, a shared local app-server daemon backs `codex agents`, and every Docket agent wrapper is installed as a native TOML under `~/.codex/agents/` (docket-implement-next, docket-plan-writer, docket-build-*, etc.). The spec's premise — a registered-agent root that must reach a coordinator-capable launch — matches the live environment exactly, so no scope adjustment is warranted on those grounds.
- **Cited code is current.** `internal/harness/codex/codex.go` still emits the change-0365 `codexDispatchBoundary` into every generated agent unconditionally (it is not an allowlist), and `renderAgent` places the recursion guard first and the dispatch boundary second. ADR-0036 (repository-owned parent routing, machine-local Codex wrappers), ADR-0059 (capability resolved, not inferred from a tool name), ADR-0060 (wrapper conforms to target-harness contract), and ADR-0094 (pinned plan-writer) are all still in force and unmodified. The harness adapter layer (`internal/harness/`, four adapters) and the `agents/docket-*.md` common inventory are the boundaries the design names.
- **Related work unchanged.** Change 0359 (run-gate waiting/continuation) is untouched by this change. Change 0364 remains the failed live transcript with its durable `## Run halted` marker; it is not modified here and is resumed only after this fix merges and installs.
- **Crux confirmed, not softened.** The acceptance evidence is a live, fresh-process Codex certification (root -> coordinator -> named leaf -> unique sentinel) across both supported entry paths; automated generator/adapter tests cannot substitute for it, and no production launch mechanism may be encoded until the disposable fixture proves it (Design section 1). This is reachable on this machine but is genuine live multi-process investigation of the Codex 0.151.0 app-server surface. Per the spec, a PR may be opened for review while any external limitation is clearly reported, but the change is not `done` without the successful nested sentinel or an approved redesign. No spec sections are still-mutable in a way that requires rewriting; relations (`related: [359, 364]`, `discovered_from: [365]`, `adrs: [36, 59, 60, 94]`) are accurate and left unchanged.

## Finalize blocked

### 2026-09-01 — attempt 20260901T004319Z-0c3b7ae33eae

<!-- attempt:20260901T004319Z-0c3b7ae33eae -->

- Reason: repair-needs-signoff
- Head: af88b93f4717a499b38b528638e96205bd76d08c
- PR: #260
- Comment: https://github.com/danielhanold/docket/pull/260#issuecomment-5486997553

Remedy: Review the pushed repair commit af88b93f on PR #260 (git show af88b93f). If it looks correct, re-run docket-finalize-change naming id 384 — the retry clears this block and merges. If not, amend or reject the repair on the branch.
