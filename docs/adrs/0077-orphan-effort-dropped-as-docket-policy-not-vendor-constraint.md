---
id: 77
slug: orphan-effort-dropped-as-docket-policy-not-vendor-constraint
title: An effort with no resolved model is dropped as docket policy, not because opencode would reject it
status: Accepted
date: 2026-08-08
supersedes: []
reverses: []
relates_to: [15, 60]
change: 245
---

## Context

`emit_opencode_md` in `sync-agents.sh` emits `reasoningEffort:` only when a model also resolves.
When the model is empty or the `inherit` sentinel, it logs a WARN and emits no effort key at all.

The pre-existing comment at that site justified the drop with a **technical** claim: "a provider
option with no provider selected has nothing to reach." Change 0245's build ran a live probe
against **opencode 1.18.14** (`opencode debug agent docket-status`) and that claim did not
survive: a hand-written agent carrying `reasoningEffort: high` with **no** `model:` still reports
`options: {reasoningEffort: "high"}`. opencode would in fact honor the orphan effort. The drop was
being justified by a vendor constraint that does not exist.

The same probe re-confirmed the claim the emitter actually depends on, previously verified only at
1.18.11: opencode forwards unrecognized agent-frontmatter keys to the provider as model options
(surfacing as `options.reasoningEffort`), and it splits a double-prefixed OpenRouter id into a
providerID and a modelID itself. So the forwarding mechanism is real and current; only the
"nothing to reach" rationale was false.

That leaves a choice the code was making implicitly and can no longer make by appeal to opencode:
emit the orphan effort because the vendor accepts it, or keep dropping it for docket's own reasons.

## Decision

**The drop stays — as docket's own design choice, not a vendor constraint.**

docket refuses to pin an effort it cannot attribute to a resolved model. A generated file never
carries an effort whose target is unnamed, because an effort key alone does not say what it is an
effort *for*: a reader of the wrapper (human or tooling) cannot tell which model the pin governs,
and docket would be reporting a pin whose subject it does not know. The WARN is the interface —
it tells the user their effort was dropped and why, so a half-configuration is loud rather than
inferred from a missing key.

The source comment must state this as policy. It may no longer claim opencode cannot honor an
orphan effort, because the 1.18.14 probe shows it can.

This sits alongside ADR-0015 (verbatim model/effort passthrough, no vendor allowlists — docket
still never inspects or validates the effort token it drops) and ADR-0060 (per-harness named
emitters; a pin that cannot be honestly expressed is dropped **loudly** at generation time, never
silently). ADR-0060's rule covered pins the target contract cannot express; this extends the same
posture to a pin the target *would* accept but docket declines to make.

## Consequences

**Enables.** Every generated opencode wrapper is self-describing: a `reasoningEffort:` present in
a docket-generated file always has a named `model:` beside it, so the pin's target is readable off
the file. docket never reports a pin whose subject is unknown, which keeps the agent layer's
"what is actually pinned" story checkable by reading the artifact rather than by reasoning about
resolution order.

**Costs.** A configuration opencode would have accepted is refused. An opencode user who wants
effort pinned must also pin a model — docket will not emit a half-pin. Someone who deliberately
wanted the inherited model plus a docket-set effort cannot express that through docket, and the
WARN is their only signal.

**Given up.** The ability to justify this behavior by pointing at the vendor. It is now docket's
position, so it must be re-argued (not merely re-probed) if a future harness makes orphan effort
the natural configuration.
