---
id: 98
slug: structured-gate-waiting-and-ownership-handoff
title: "Gate waiting is structured, resumable, and ownership-handed-off"
status: 'Superseded by ADR-0107'
date: 2026-08-25
supersedes: []
reverses: []
relates_to: [24, 95]
change: 342
---

## Context

Autonomous docket build, implement, and finalize agents must run the whole test suite as a gate, but
the suite outlives a single foreground call. A forked or dispatched subagent that backgrounds the
suite and *yields* to await a completion event cannot receive its own resumption notification
(ADR-0024), so the run stalls half-done and the caller reads the resulting report as `completed`.
The prior mitigation — a synchronous full-budget polling loop that a controller had to hold open for
the suite's entire duration — avoids the deadlock but does not compose with slice-bounded agent
execution: it blocks one call for as long as the gate takes.

Both shapes are unsatisfactory for the same underlying reason: waiting was never a first-class,
representable state. It was either a yield (undeliverable) or a held-open call (unbounded).

## Decision

Introduce a **resumable native gate driver** with three first-class properties.

1. **Structured waiting.** The gate is driven in short *synchronous slices*, each returning a typed
   outcome — `WAITING`, `PASSED`, `FAILED`, or `HALTED`. A `WAITING` outcome carries an explicit
   **continuation**, persisted to a durable owner-private drive store **outside the agent
   transcript**. No agent ever yields-and-waits; every call returns.

2. **Fingerprinted ownership handoff.** Continuing a drive is guarded by a per-dimension repository
   **execution-identity fingerprint** plus a **single-use handoff receipt** over a generation
   compare-and-swap. Only an exact fingerprint match consumes the receipt and mints a new owner
   generation, so exactly one owner advances, and a `WAITING` drive with no outstanding handoff
   cannot be claimed by anyone.

3. **Nearest-owner continuation.** Waiting terminates at the **nearest controller** and consumes
   neither repair nor escalation budget. Failure, process death, deadline expiry, and ambiguous
   ownership remain **distinct** outcomes, and at most one non-overlapping relaunch is permitted
   under the original fixed-once deadline.

The rule a reader needs: **never background a gate and yield; drive it in slices, and transfer
ownership only through a fingerprint-guarded, single-use handoff.** Executable workflows compose the
driver layer, never the raw gate primitives beneath it.

## Consequences

Every driver call is synchronous and slice-bounded, so a gate composes with slice-bounded agent
execution instead of monopolizing a call. Because the continuation lives in durable storage rather
than a transcript, a *fresh process* can resume the drive and consume the exact terminal status —
which is what makes the typed outcome auditable rather than reconstructed from prose.

**ADR-0024 remains correct and is not reversed.** A forked agent still cannot yield and receive its
own notification. This design removes the need to: every call is synchronous, and the continuation
sits outside the transcript. The cost is that transferring ownership now requires a deliberate,
fingerprint-guarded handoff rather than an implicit re-entry.

**ADR-0095 remains authoritative** for raw native process supervision. This driver *composes* it,
adding the resumable typed-waiting and ownership layer above the raw supervisor rather than
replacing it.

Costs accepted: a new durable drive store with its CAS and lock discipline, and a
mutation-proven architectural boundary guard, now required to keep executable workflows from
composing raw gate primitives outside the driver layer.
