# Shared orchestration simplification — deferred discovery

Goal #2 is separate from [Codex runner completion](../active/0432-complete-native-codex-runner.md) and [the supervisor in 412](../active/0412-forked-implement-next-build-agent-still-backgrounds-the-gate.md).
This is a discovery note, not an approved redesign or a prerequisite for Codex support.

## Agreed boundary

Finish native Codex compatibility with the existing contract first. A necessary shared
bug fix is allowed with cross-harness regression coverage; changing the contract,
consolidating ownership models, or retiring mechanisms requires separate design approval.

## Initial findings, 2026-09-16

The 431 reports show successful implementation and tests can coexist with an incomplete
workflow because of scope, epoch, metadata binding, acknowledgement, and handoff failures.
These reports motivate an audit, but do not prove which mechanisms are redundant.

Questions to evaluate across harnesses:
- Which component authoritatively decides child termination, task verification, and overall completion?
- Which ownership checks protect mutation, and which merely constrain observation?
- Could mechanical command construction be centralized without changing authorization?
- What complexity could be removed in exchange for explicit, human-visible recovery?

Preserve exact-HEAD evidence, stale-owner fencing, single-writer/execution guarantees,
honest incomplete outcomes, and durable cancellation accounting in any proposed redesign.

## Recording future findings

For each finding record: source run/commit, observed behavior versus inference,
Codex-specific or shared relevance, candidate simplification, safety property to preserve,
and the deferred decision. Never copy credentials, private gate keys, or handoff tokens.

Raw diagnostics remain in the launch kit; include sanitized reproductions and conclusions
here so this file remains useful without that machine-local directory.

## Receipt interpretation finding, 2026-09-16

Source: 431's final continuation on repair `95660e8f…`, source checkpoint `ed80a72a…`;
the sanitized trace is in [432's handoff investigation](0432-codex-runner-handoff.md).
Observed: `gate.drive.handoff` delivered its token in `drive.generation`, and
`run.gate-claim` successfully redeemed it and delivered fresh owner authority in
top-level `generation`. Both callers subsequently reported a missing handoff token.
The different envelopes and operation-dependent meaning of `generation` are shared
protocol properties; the demonstrated misinterpretation occurred in native Codex.

Candidate future simplification: evaluate a clearer common presentation of receipt
semantics and next permitted actions, including the distinction between continuation
redemption and direct drive claim. This is not evidence that either ownership layer is
redundant and does not justify renaming fields or consolidating authority in 432.
Preserve single-use redemption, stale-owner fencing, parent/child capability separation,
and private credentials. Decision deferred; 432 should first consume the existing
protocol correctly and test that consumption.

Related observation: claim closes the old recovery scope, whereas uninterrupted
completion acknowledges an open scope. Recovery callers need a distinct terminal
consumption path. Whether to unify those paths is a separate design question; weakening
closed-scope rejection is not an authorized compatibility fix.

## Pre-created branch ownership finding, 2026-09-17

Source: human-directed preparation of change 432, using installed Docket commit
`1b6f8b2d0abfbb21ac4f2118252defdca917ecf1`. Its user-approved local and remote branch
`fix/complete-native-codex-runner` both remained at
`ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8`, the preserved 431 checkpoint.

Observed: `change.claim` succeeded and recorded the existing branch, moving 432 to
`in-progress`. A plain Git worktree was attached to that branch without moving its
tip. `workspace.inspect` then returned `foreign`; `workspace.prepare` returned
`invalid-state` with disposition `blocked`. The branch/worktree had no Docket workspace
manifest. The installed capability catalog exposes inspection and preparation, but
no workspace-adoption operation. No manifest was fabricated, no branch was recreated,
and no repair code was written. The human chose to pause for supported adoption.

The shared workspace implementation explains the boundary: `Service.Inspect` classifies
an absent manifest as foreign, and `Service.Prepare` requires a fresh allocation's
branch, remote branch, path, and registration all to be absent. A successful change
claim does not establish workspace ownership. This is a shared lifecycle limitation,
not evidence of a Codex-specific transport failure or a broken ownership guard.

Candidate future simplification: evaluate one explicit, supported adoption transaction
for a human-approved pre-created branch and/or worktree, with a clear distinction
between change claim and workspace ownership. Preserve repository identity, exact
branch and HEAD, approved base ancestry, existing files and commits, single-writer
ownership, and refusal on conflicting or unproven ownership. Never treat branch-name
agreement alone as authorization or delete/recreate the branch to satisfy allocation.

Decision deferred to shared orchestration design; this note authorizes no new ownership
mechanism or guard bypass in 432. The refusal currently blocks 432's managed workspace
preparation, while shared simplification remains separate from its receipt-consumption
repair scope. See [432's handoff](0432-codex-runner-handoff.md) for branch provenance and
the instruction to surface ownership/adoption refusals.
