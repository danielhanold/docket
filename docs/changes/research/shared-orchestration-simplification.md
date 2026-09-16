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
