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
