---
id: 118
slug: 'worktree-wide-gate-admission-and-explicit-human-cancellation'
title: 'Worktree-wide gate admission and explicit human-cancellation authority'
status: 'Accepted'
date: '2026-09-14'
supersedes: []
reverses: []
relates_to: [87, 95, 107, 111, 117]
change: 375
---

## Context

Repeated gate starts and interrupted implementation runs could leave two executions, or two surviving writers, acting on one worktree. Change 0405 already prevented duplicate starts within one recovery scope and supported sequential tests through explicit acknowledgement, but separate scopes, scopeless starts (for example finalize's local gate), competing automatic relaunches, and raw single launches still lacked worktree-wide admission. Separately, a dispatched run had no honest, explicit human Stop: killing a process or closing a tab did not tell the gate the run was over, and a resume could race a still-live predecessor.

## Decision

One canonical worktree carries at most one reserved-or-running gate execution, enforced by a durable worktree execution slot that is reserved BEFORE launch and released ONLY on reconciled terminal-verdict-plus-teardown evidence (a driven PASSED/FAILED terminal, or a proven raw teardown via stop). Admission keys on the canonical worktree path (symlink aliases resolve to one slot) and is independent per distinct worktree of the same repo. A run epoch, bound to the run-gate record, fences the worktree so only that epoch's own sequential drives may re-admit; a foreign or empty epoch cannot detach the worktree from its owner. Human Stop is an explicit cataloged cancellation (`run.cancel`, keyed by run-gate key plus epoch) that durably fences the epoch before shutdown and reports success only on full task/process/mutation accounting (cancelled / cancellation-pending / already-cancelled / refused); cancelling charges no suite attempt and resets no budget. Resume (`run.gate-before --resume`) refuses to start a second run over an unstopped one, and supersedes a confirmed-cancelled epoch atomically for exactly one replacement. This EXTENDS ADR-0107's cooperative-handoff and direct-parent-takeover rules (takeover additionally cannot revive a cancelled epoch); it does not supersede them, and preserves ADR-0115/0116 budget semantics.

## Consequences

Enables safe concurrent gates on distinct worktrees of one repo while making a second execution on the same worktree fail closed (worktree-busy / unresolved-execution / stale-run-epoch), with credential-free diagnostic locators and named next-actions. Adds a real, honest human-cancellation path and a bounded, one-replacement resume, plus a computed launch-site admission guard so a new launch site that bypasses admission reddens. Costs: raw `gate.launch` now holds the worktree slot until an explicit stop proves teardown (operators and tests must release it); adapters with no cancellation or death callback report `owner-lifecycle-unavailable` and name `run.cancel` as the remedy rather than claiming universal UI-Stop integration. Fail-closed everywhere: an unreadable or ambiguous state blocks rather than freeing a worktree over a possibly-live process.

## Alternatives considered

Keep admission scoped to the recovery scope alone (change 0405's shape) and rely on callers not to start scopeless or cross-scope gates — rejected because finalize's local gate, competing automatic relaunches, and raw single launches all sit outside any scope, so the invariant would hold only by convention. Release the worktree slot on process death or a liveness heartbeat instead of reconciled terminal-plus-teardown evidence — rejected as fail-open: an unreadable or ambiguous state would free a worktree over a possibly-live writer. Treat a killed process or closed tab as an implicit cancellation — rejected because it leaves the gate with no durable record, so a resume can race a still-live predecessor; an explicit cataloged `run.cancel` with epoch fencing and full accounting is the only honest Stop. Supersede ADR-0107 with a single unified ownership model — rejected as unnecessary scope: cooperative handoff and direct-parent takeover remain correct and only need the added constraint that takeover cannot revive a cancelled epoch.
