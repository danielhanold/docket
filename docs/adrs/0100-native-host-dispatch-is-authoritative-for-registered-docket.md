---
id: 100
slug: 'native-host-dispatch-is-authoritative-for-registered-docket'
title: 'Native host dispatch is authoritative for registered docket agents'
status: 'Accepted'
date: '2026-08-30'
supersedes: [37]
reverses: []
relates_to: [36, 74]
change: 371
---

## Context

docket's agent layer generates one wrapper file per registered `docket-*` agent, and a parent invokes a workflow by dispatching that agent. Historically two invocation paths coexisted on the maintained surface. The first is the host harness's own native named-agent dispatch, contracted by the committed, machine-neutral `docket:dispatch` block (ADR-0036) that each harness's parent-facing instructions file carries. The second was cross-harness runner delegation: an explicit per-agent `runner:` field (ADR-0037) that routed a whole run out to a child harness through a shell shim, documented in the convention skill's agent-layer reference and reachable from the build skill's delegation-execution reference.

Two paths for the same act is the problem. A parent facing a workflow with no same-name registration on the current host had a documented sideways exit — reroute through a runner, or reconstruct the workflow inline — and either exit converts a visible capability failure into an invisible behavior change: the run appears to proceed while the pinned model, effort, preloaded skills, and abort-and-report contract the wrapper carries are all silently gone. Change 0371 cut the maintained dispatch surface over to host-native dispatch alone: the generated `docket:dispatch` block now states the never-fall-back rule explicitly, the `runner:` cross-harness delegation section is gone from `skills/docket-convention/references/agent-layer.md`, and the runner-dispatch references are gone from `skills/docket-build/references/delegation-execution.md`. After that cut the maintained surface documents no `runner:` key and no runner shim, so ADR-0037's opt-in no longer names anything a reader can reach.

## Decision

Native host dispatch is authoritative for registered docket agents. A parent invokes a registered `docket-*` agent through its own harness's native named-agent dispatch; the generated machine-neutral `docket:dispatch` block is the contract for that invocation.

A workflow with no same-name registration on the current host is a visible capability failure. It fails visibly and is NEVER rerouted through a shell runner, another harness, a generic agent, or an inline reconstruction of its contract: there is no shell fallback, no cross-harness fallback, no generic-agent fallback, and no inline fallback.

Cross-harness runner delegation is retired from the maintained dispatch surface, superseding ADR-0037's `runner:`-field opt-in on the generated agent layer.

This decision relates to ADR-0036 — the committed, machine-neutral `docket:dispatch` block, preserved unchanged as the contract this decision makes authoritative — and to ADR-0074, the gate tri-state, preserved and untouched.

## Consequences

One invocation path, so a missing registration is loud rather than silently absorbed: the failure names the unregistered workflow instead of producing a run that looks complete while executing under none of the wrapper's pins. Readers of the maintained surface no longer meet a `runner:` key or a runner shim, so the agent-layer reference and the build skill's delegation-execution reference describe exactly one topology.

The cost is that cross-harness delegation is no longer an option on the maintained surface: a workflow that a host does not register cannot be run on that host at all, and the fix is to register it there, never to route around it. Reaching a child harness's models or subscription now requires a new decision rather than an existing key.

This dispositions only the maintained dispatch surface. The frozen Bash delegation facade — `scripts/runner-dispatch.sh`, `scripts/runners/*`, and the ADRs recording its internal mechanics (0067, 0079, 0080, 0087, 0088) — remains physically present and is owned and removed by a separate change; those ADRs are deliberately not dispositioned here. ADR-0038 is already Superseded by ADR-0079 and needs no action.

## Alternatives considered

Keep `runner:` as a dormant, undocumented escape hatch. Rejected: an undocumented path is still a path, and the failure mode this decision removes is precisely a parent quietly taking one. Dormancy also leaves two topologies in the code with only one described, which is worse than either state.

Allow inline reconstruction of a missing workflow's contract as a degradation. Rejected: reconstructing a workflow inline discards the pins and the abort-and-report rule that make an autonomous run auditable, and it produces a run that reports success while having executed something else. A tiered inline path exists in docket only where the contract is verifiable git state; dispatch of a pinned workflow agent is not that shape.

Reroute to a generic agent when no same-name registration exists. Rejected for the same reason: the identity of the dispatched agent is load-bearing, so a generic substitute is a different run wearing the same name.
