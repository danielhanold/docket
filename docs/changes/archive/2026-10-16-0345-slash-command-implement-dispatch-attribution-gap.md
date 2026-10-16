---
id: 345
slug: slash-command-implement-dispatch-attribution-gap
title: "Slash-command implement dispatch isn't agent-owned — attribution gap forces human-in-the-loop"
status: 'killed'
priority: high
type: feat
created: 2026-08-25
updated: '2026-10-16'
depends_on: [393, 407]
related: [334, 359, 371, 405]
discovered_from: [342]
adrs: [24, 26, 60, 84, 100, 103, 111]
spec: 'docs/superpowers/specs/2026-09-07-slash-command-implement-dispatch-attribution-gap-design.md'
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-slash-command-implement-dispatch-attribution-gap-design.md](../../superpowers/specs/2026-09-07-slash-command-implement-dispatch-attribution-gap-design.md) |
| ADRs | [ADR-0024](../../adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0026](../../adrs/0026-fork-dispatch-opacity-two-invocation-paths.md), [ADR-0060](../../adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0084](../../adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md), [ADR-0100](../../adrs/0100-native-host-dispatch-is-authoritative-for-registered-docket.md), [ADR-0103](../../adrs/0103-enter-codex-coordinator-roles-through-app-server-root-thread.md), [ADR-0111](../../adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md) |
<!-- docket:artifacts:end -->

## Why

User-invoked implement commands can launch the worker before the parent has armed the run gate. The original observation was a Claude slash-command fork during change 0342: an overloaded run left a claimed-but-unbuilt change that the parent could only report. Command entry should reach the same attribution and safe recovery path as an ordinary assistant-owned dispatch across every supported harness.

Change 0407 has since replaced snapshot/timestamp inference with durable proof binding a dispatch context to its successful claim. The remaining gap is creating that context before command-launched work, retaining the parent's gate key, passing the context into the worker, and reaching the post-run verdict loop. Timestamps, launch shape, and child prose cannot supply missing ownership authority.

Claude, Codex, Cursor, and OpenCode are all required delivery targets. Codex's compositional implementer also requires change 0393's native root-coordinator entry, which is implemented but awaiting merge at grooming time.

## What changes

- Keep the existing public implement workflow and ergonomic native command/skill invocation. Run a short coordinator in the current parent session to arm the gate, enter the named implementer with the exact context, and follow the gate's return decision.
- Separate the public entry procedure from the canonical autonomous worker procedure. Assigned workers execute their own charter without self-dispatch or a second gate; preserve native role identity, model/effort settings, permissions, selection scope, and existing implementation depth.
- Deliver the entry contract for Claude, Codex, Cursor, and OpenCode through their native adapters. Use change 0393's required root-coordinator entry for Codex and preserve its working directory and permission context.
- Reuse change 0407's claim receipts and existing retry/continuation accounting. Permit only the gate-authorized bounded retry or exact continuation; retain observe-only behavior when attribution is unavailable and stop on unsafe ownership.
- Update maintained skills, agent wrappers, dispatch surfaces, installer-owned command assets where needed, embedded assets, and documentation together. Require regression coverage and fresh native-session acceptance for all four harnesses, including Cursor IDE.

The linked spec contains the selected architecture, failure boundaries, implementation surfaces, alternatives, and acceptance criteria. Change 0345 remains proposed and waits for dependency 0393 to reach done before implementation.

## Out of scope

- Relaxing the run gate's ownership invariant or authorizing work on a claim another live agent may hold.
- New claim/token formats, a parallel attribution mechanism, or new retry accounting.
- Vendor 529/overload and pre-claim transport-retry behavior.
- Change 0405's test-drive prepare-scope/start handshake investigation.
- Cross-machine recovery, recovery after loss of the parent's gate key, additional harnesses, new supervisor agents, or cross-harness runner fallbacks.
- Changing human merge approval, provider permissions, or implementing/build-planning this change during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Why killed

Outdated; replaced by [change 0556](0556-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md), which keeps only the Claude Code part. Killed by human decision on 2026-10-16 after a spec review.

- **The premise is real but narrow.** A live check on 2026-10-16 (Claude Code 2.1.301, throwaway repositories) showed that only a typed `/docket-implement-next` starts an untracked run: Claude Code forks the skill straight into the agent, so the main session never runs `run.start`, and the claim records an empty `gate_context_hash`. `/loop docket-implement-next` and prose prompts ran `run.start`, passed the run context, and produced tracked claims.
- **The four-harness scope no longer holds.** Codex is unsupported, and the 0393 root-coordinator route this spec relied on was abandoned (0424, 0425, 0426, 0431 and 0432 killed; 0433 deferred with a different topology). OpenCode ships untested, with full support after v1.0.0 (0513). Cursor never forks a skill, so its gap, if any, works differently.
- **The vocabulary and machinery changed.** Every run-gate name in the spec was renamed (ADR-0129: `run.start`, `run.verdict`, `run.continue`, `run-*` verdicts, run key, run context). The run tracker gained cancel and resume rules, the key as its only handle, no automatic relaunch, run-context identity in the claim request, and closeout on `run-done` (ADR-0128, 0133, 0135, 0143, 0148). Both dependencies (0393, 0407) are done.
- **Overlap.** A child dropping the run context at claim time is change 0541's.
