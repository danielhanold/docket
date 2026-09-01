---
id: 393
slug: 'enter-compositional-docket-roles-as-coordinator-capable-code'
title: 'Enter compositional Docket roles as coordinator-capable Codex root threads'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-01'
updated: '2026-09-01'
depends_on: []
stacked_on:
related: [364, 365, 384]
discovered_from: [384]
adrs: [36, 59, 60, 94]
spec: 'docs/superpowers/specs/2026-09-01-enter-compositional-docket-roles-as-coordinator-capable-code-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/enter-compositional-docket-roles-as-coordinator-capable-code'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-01T13:28:27Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-01-enter-compositional-docket-roles-as-coordinator-capable-code-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-01-enter-compositional-docket-roles-as-coordinator-capable-code-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

Change 0384 was intended to restore nested Docket composition in Codex, but its generated-wrapper change did not alter the launch topology. The actual registered docket-implement-next path still starts as a depth-1 agent without top-level collaboration controls, so it cannot dispatch docket-plan-writer and change 0364 has halted at Step 4 for the second time. Sequential live spikes isolated the boundary: a generic child can spawn a grandchild, the exact registered docket-implement-next child still cannot, and the same role contract succeeds when entered as a coordinator-capable root thread. This follow-up applies that empirically selected root-entry design to the production Codex path.

## What changes

Add a Codex harness-native coordinator root-entry path for Docket roles that own nested dispatch. Seed the new root thread from the same installed role definition used by registered dispatch, preserving developer instructions, model, reasoning effort, skill preload, recursion guard, request payload, working directory, permissions, foreground completion, and return contract without hand-duplicating wrapper prose. Route the VS Code-backed docket-implement-next entry through that path and prove the real docket-implement-next to docket-plan-writer edge in a fresh process, alongside focused adapter tests, a mutation that restores the old depth-1 launch and fails, and the repository's full build gate.

## Out of scope

A generic process requirement for verification that could not have run before merge; granting broad collaboration controls to every spawned agent; a typed parent-relay protocol or approved-relay fallback; generic-agent, shell-runner, subprocess-session, or cross-harness substitutes; changing Docket's role topology, child payloads or receipts, Tier-C authorization, model or effort pins, skill bindings, worktree scopes, or the separate run-gate continuation behavior tracked by change 0359.

## Run halted

### 2026-09-01

Halted at Step 3 (reconcile) — 2026-09-01 — by an autonomous docket-implement-next run executing in the Claude Code harness.

## Disposition

HALTED. Not built. The change was claimed, reconciled against current code, and found to require a live execution host this autonomous implementer cannot provide. The claim lease will self-heal; no feature branch, workspace, or code was created.

## Why this run cannot faithfully build 393

393's load-bearing deliverable is a RUNTIME Codex root-entry adapter, and its central acceptance is a LIVE behavioral proof. Both are bound to a VS Code-backed Codex app-server host that is not this execution environment.

1. Docket has no runtime launch layer to extend. The `harness.Adapter` interface (`internal/harness/harness.go`) exposes only `Detect` and `Plan(PlanInput) []install.Target`. Every adapter — codex included (`internal/harness/codex/codex.go`) — is a pure, stateless STATIC GENERATOR that emits install targets (for Codex, one TOML per role under `~/.codex/agents/`). There is no app-server client, no thread-creation code, no VS Code host connection, and no `codex exec` path anywhere in the Go tree (confirmed by a whole-repo search; the only occurrences are a banned-substring guard in `internal/harness/cross_harness_test.go` and a skill doc). Agent launching at runtime is performed natively by the Codex host reading those static files — never by Docket code.

2. Spec section 3 mandates exactly the runtime surface Docket does not have. It requires an adapter that "create[s] a root thread in the caller's repository and permission context ... apply[ies] the role's developer instructions and configured model at thread start ... wait[s] in the foreground for the turn-completed or terminal-error event ... and return[s] the role's final output," using "the VS Code-backed host connection or another supported in-process/native app-server entry surface," explicitly not shelling to `codex exec`. The spike that selected this design (spec lines 21-29) drove a live Codex app-server root thread by hand; there is no in-repo protocol to code against, and any Go adapter written blind could not be exercised here.

3. The acceptance proof is non-severable and unreachable from Claude Code. The Behavioral regression (spec lines 96-105) and Success criteria (lines 131-137) require running the ACTUAL installed `docket-implement-next` through root entry with a REAL `docket-plan-writer` launch, with evidence showing the coordinator's root status and a persisted `vscode` session source. This autonomous run executes in Claude Code, and its dispatched build/plan/review workers are Claude Code subagents; none can create or observe a live `vscode`-sourced Codex app-server session. The proof is the crux of the change, not an optional extra.

4. Delivering only the in-repo Go parts would reproduce the exact anti-pattern 393 exists to correct. 393's own "Why" and the spec's "Why change 0384 did not fix the production path" (lines 15-19) state that predecessor 384 is a FAILURE precisely because it added inventory posture, evidence, and documentation WITHOUT changing or behaviorally proving the launch topology. Shipping sections 1-2 plus contract tests and goldens while leaving section 3's runtime adapter and its live proof unperformed is that same non-fix. Spec line 65 is explicit: if the VS Code integration cannot expose the demonstrated root-thread operation, "stop with a precise unsupported-host error. Do not silently fall back." This halt is that stop.

This is not "the build is hard." It is a hard environmental blocker on a non-severable, live-host-bound deliverable — a change whose correctness is defined by live Codex host behavior cannot be honestly implemented and certified by a Claude Code build loop producing Go plus unit-tests-against-a-fake with the mandatory live proof skipped. That is the third consecutive time this class of certification would go unchecked (365 left it unchecked -> 384 still unproven -> 393).

## Reconcile findings (current, verified)

- Change 393 is build-ready per selection policy: `proposed`, `spec:` set, no `depends_on`, base resolves to `main`. The design itself is sound and well-specified; the blocker is the executor/environment, not the design.
- Related predecessors: 384 is `done` (PR #260) but was the non-fix described above; 365 is archived; 364 currently in active is an unrelated `migrate-primary-clean-fast-forward` record (the spec's "change 0364" reference is to a prior implement-next run of a differently-numbered change and is point-in-time context, not a current dependency).
- Cited ADRs 36, 59, 60, 94 are all in force and unmodified. ADR-0036 (committed machine-neutral parent routing) would be the boundary any production root-entry change must not silently overload (spec line 117).
- `codex-cli 0.152.0` is installed on this machine and `codex` is on PATH, but a CLI binary is not the VS Code-backed app-server root-thread surface the spec requires, and this Claude Code run has no channel to drive or observe a `vscode`-sourced Codex session.

## Recommended human action

Run 393 in the environment where its deliverable lives and can be proven: a live VS Code-backed Codex host. Either implement it by hand there against the actual app-server protocol, or dispatch a Codex-hosted `docket-implement-next` run (once the very root-entry capability 393 delivers exists, or via the manual spike path used to select the design). Then resume this halted change by id through the resume path (`docket change resume-halted --acknowledge-quiescent`). No follow-up change needs minting; the design and spec stand as written.

## Reconcile log

### 2026-09-01

Reconciled against main at 4e510f735d5bf249a17e7f91f58455889791271c and Codex CLI 0.152.0. The Go harness adapter is currently installation-only, so this change must add an explicit runtime entry boundary rather than overload wrapper rendering. The managed app-server control-socket proxy rejected a live initialize attempt with Broken pipe, while the already-spiked direct app-server protocol remains the supported root-thread route; implement the adapter over that native protocol and retain fail-closed rejection of codex exec, ordinary-child fallback, and typed relay. The pre-workspace resume command also misclassified an absent manifest/path/ref as a foreign active writer; the verified empty state is safe for this explicitly authorized inline continuation and is reported as separate follow-up work.
