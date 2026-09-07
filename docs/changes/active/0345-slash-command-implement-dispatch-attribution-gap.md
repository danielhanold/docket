---
id: 345
slug: slash-command-implement-dispatch-attribution-gap
title: "Slash-command implement dispatch isn't agent-owned — attribution gap forces human-in-the-loop"
status: proposed
priority: high
type: feat
created: 2026-08-25
updated: '2026-09-07'
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
| Spec | [2026-09-07-slash-command-implement-dispatch-attribution-gap-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-slash-command-implement-dispatch-attribution-gap-design.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0026](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0026-fork-dispatch-opacity-two-invocation-paths.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0084](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md), [ADR-0100](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0100-native-host-dispatch-is-authoritative-for-registered-docket.md), [ADR-0103](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0103-enter-codex-coordinator-roles-through-app-server-root-thread.md), [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md) |
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
