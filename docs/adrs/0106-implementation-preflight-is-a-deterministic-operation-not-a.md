---
id: 106
slug: 'implementation-preflight-is-a-deterministic-operation-not-a'
title: 'Implementation preflight is a deterministic operation, not a composition dispatch'
status: 'Accepted'
date: '2026-09-02'
supersedes: []
reverses: []
relates_to: [12, 24, 101]
change: 397
---

## Context

docket-implement-next's Step-0 preflight was a foreground dispatch of the docket-status subagent that ran an implementation-scope maintenance sweep plus a status read and returned a prose report the parent re-validated. Measured on `main` at 78d42319 (2026-09-02): the sweep itself finishes in under four seconds and emits 159 bytes, but the topology around it cost roughly two minutes and ~85k tokens per run — a fresh subagent bootstrap carrying a ~78 KB skill preload, a re-run of the capability bootstrap and repository.prepare, a 164 KB status read, a prose report the parent then validated, and a second 164 KB status read by the parent for selection; 130 KB of that status payload was the `records` artifact-integrity inventory (615 corpus records) that neither preflight, selection, nor the human status read consumes. A child that returns before its sweep finishes is also a completion-signalling failure class (change 0389's six-minute early return) that an inline shell call cannot exhibit.

## Decision

docket-implement-next's Step-0 preflight runs the cataloged `maintenance.preflight` operation inline — one process sequencing the implementation-scope MaintenanceSweep with a compact Status read (no records, no changes) and returning one protocol-v1 envelope with a Go-computed clean|problem verdict over the sweep entries, the problem_entries subset, and the post-sweep metadata revision. The parent keys on the envelope `result` and the `preflight` verdict, never on prose or a process exit code. The docket-status composition dispatch is retired for step 0 only; docket-status remains a live skill/agent for see-only reads and explicit full maintenance. Simultaneously, `docket status --json` drops the `records` inventory by default behind a `--records` opt-in, since no preflight, selection, or human read consumed it.

## Consequences

An implementation run's Step 0 costs seconds and a few hundred tokens in every harness; the parent gets the same evidence from one typed envelope with no prose intermediary; the child-completion failure class is removed because an inline shell call has no early return; every reader of `docket status --json` stops paying for the corpus inventory unless it asks. Cost: one more cataloged operation to maintain, and a behavior flip on an existing JSON field (guarded by a whole-repo consumer audit).

## Alternatives considered

Keep the dispatch and slim its payload — rejected: the bootstrap, preload, and prose round-trip dominate the cost, and the early-return failure class survives any payload trimming. Have the parent call `maintenance.sweep` and `status` as two separate inline commands — rejected: the verdict would then be computed in agent prose over two envelopes rather than in Go over one, which is the judgment the operation exists to remove. Leave `status --json`'s `records` inventory in place and filter it at the reader — rejected: every reader pays the serialization cost, and no reader consumed it.
