---
id: 107
slug: 'event-authorized-parent-takeover-extends-fingerprinted-gate'
title: 'Event-authorized parent takeover extends fingerprinted gate-drive ownership'
status: 'Accepted'
date: '2026-09-02'
supersedes: [98]
reverses: []
relates_to: [24, 75, 95]
change: 359
---

## Context

ADR-0098 made gate waiting a first-class, resumable state and moved ownership between roles by a strictly *cooperative* transfer: the current owner authorizes a single-use handoff and the next role claims it. That transfer cannot recover a drive whose owner returned **without** handing off — a dispatched child that crashed, was killed, or whose harness dropped its return. In that gap the outer run gate read healthy, still-tracked work as `run-incomplete`, spent its one retry, and terminally stopped a run that was actually progressing (changes 0333/0363; change 0359's evidence tables). The cooperative-only model had no authorized way for a parent to reclaim its direct child's drive, so a lost return became a lost run.

## Decision

1. **Durable recovery scopes at each dispatch boundary.** One scope per parent/child dispatch boundary carries two *separately-minted* opaque capabilities — a child capability handed to the dispatched child and a parent capability retained by the preparing parent and never exposed to the child. Scope records are hash-persisted (only sha256 hashes of the two capabilities and of the outer gate-context token are stored, never the raw values), CAS-transitioned under a per-scope flock, and fail closed on an unknown schema version, an identity mismatch, a second live drive bound to one scope, or a missing/wrong capability.

2. **Event-authorized parent takeover.** A parent takeover is authorized *only* by the direct child's dispatch-return event — a workflow fact the caller asserts simply by calling — plus proof of the exact parent capability. It atomically supersedes the child owner generation (a single-use scope-claim gate selects exactly one winner under a race, then swaps in a fresh parent-minted owner generation), so the stale child owner thereafter fails owner-superseded. It is **never** authorized by a timer, heartbeat, log-activity check, claim-age, or process-name liveness guess. Every ambiguity, identity drift, outstanding handoff, expired deadline, or lost race fails closed to a HALTED document — never a launch, a stop, or a duplicated process.

3. **Nonterminal continuation at the outer run gate.** The outer run gate treats a tracked drive as a nonterminal continuation: it emits `gate-continue`, keeps the same gate key, and preserves the retry (the retry CAS is unreachable on the continuation path — the tracked-drive check runs *before* any retry consumption). It achieves this without modifying the run-waiting predicate: the verdict path performs the outer takeover and then synthesizes a *normal* single-use handoff, which the **unchanged** `RunVerify` run-waiting predicate validates. Only a genuinely quiescent incomplete (no scope, or zero candidate drives) falls through to the existing retry path.

4. **Explicit resume attribution on `gate-before`.** `gate-before` accepts an explicit resume id and pre-binds attribution to it *only* when it is a verified in-progress change with a valid workspace identity — never by a timestamp game. It prepares the outer recovery scope and prints the outer child capability as the dispatch context, which the parent copies into the child's dispatch prompt.

**Preserved from ADR-0098:** structured `WAITING` as a first-class, representable gate state; the fixed-once deadline (set at drive start, never extended); fingerprinted single-owner advancement — a continuation re-validates the worktree fingerprint, so it certifies the original bytes; the single-use handoff as the **preferred** ownership transfer, with takeover the exceptional path used only for a direct child that returned without handing off; and every ambiguity failing closed to `HALTED`, never to a red suite result.

## Consequences

- The drive record advances to schema v2 and the gate record to v2; both fail closed against old (pre-upgrade) records, so an in-flight record written before the upgrade halts rather than being silently migrated — correct fail-closed behavior.
- A grandparent still cannot skip a live parent: takeover authorizes only the *direct* parent of a scope, so ownership recovery walks the dispatch chain one boundary at a time.
- Observe mode remains structurally unable to authorize a retry or a continuation — it has no branch to `gate-retry-once` or `gate-continue` and renders a run-waiting id as a plain observation.
- A lost child return is now recoverable rather than terminal: the outer gate continues the same attempt on the same key with its retry intact, so a run that is genuinely progressing is no longer stopped by a dropped dispatch return.

## Alternatives considered

- **Keep the cooperative-only handoff of ADR-0098.** Rejected: it has no authorized path for a parent to reclaim a direct child's drive, so a dropped dispatch return remains a terminally lost run — the exact defect this decision exists to close.
- **Authorize takeover by liveness heuristics** (a timer, heartbeat, log-activity check, claim age, or process-name probe). Rejected: every such signal is a guess about a process the gate does not own, and a wrong guess duplicates a running drive or stops a healthy one. Takeover is authorized only by the direct child's dispatch-return event plus proof of the parent capability.
- **Hand the child a single capability reused by the parent.** Rejected: a capability the child can read is a capability the child can replay, so takeover would no longer prove parent identity. The two capabilities are minted separately and the parent's is never exposed.
- **Change the `RunVerify` run-waiting predicate to recognize a tracked drive directly.** Rejected: it widens the predicate that guards every run. Instead the verdict path synthesizes a normal single-use handoff, leaving the predicate unchanged and the continuation validated by the existing rule.
- **Treat a tracked drive as a retry** (`gate-retry-once`). Rejected: a continuation is the same attempt still owning tracked work, so consuming the single retry for it would exhaust the run's one real recovery before any genuine failure.
