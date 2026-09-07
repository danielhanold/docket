---
id: 376
slug: gate-drive-start-human-output-omits-drive-id-generation
title: '`docket gate drive start` human-readable output omits drive_id/generation'
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-30
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [375, 405]
discovered_from: [372]
adrs: [107]
spec:
plan: 'docs/superpowers/plans/2026-09-07-gate-drive-start-human-output-omits-drive-id-generation.md'
results:
trivial: true
auto_groomable:
branch: 'fix/gate-drive-start-human-output-omits-drive-id-generation'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T14:50:53Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-09-07-gate-drive-start-human-output-omits-drive-id-generation.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-07-gate-drive-start-human-output-omits-drive-id-generation.md) |
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md) |
<!-- docket:artifacts:end -->

## Why

During the change-0372 build, a caller reran `gate.drive.start` to recover ownership information missing from its first human-readable response, launching a second drive (tracked separately as #375).

Review against current main on 2026-09-07 confirms that `GateDriveResult.HumanText` already prints `drive_id`. It deliberately omits the ownership generation, consistent with the gate driver's existing credential boundary. The remaining defect is caller guidance: `skills/docket-build/references/gate-caller-loop.md` mentions the JSON transport, while actionable start/handoff instructions, including the build-task worker contract and implement-next's evidence re-mint, do not explicitly require capturing JSON from the first call. A drive id alone does not authorize advancement or handoff.

The human approved retaining and narrowing 0376 to a caller-guidance fix on 2026-09-07. Printing credentials in human text is no longer the proposed solution.

## What changes

**Approved scope: Require JSON capture for gate-drive ownership operations.**

- Make the shared gate-caller contract explicitly require `--json` whenever a workflow consumes a `gate.drive` result. Capture and validate the first response before using it: the drive identifier and ownership generation from `start`, the single-use handoff token from `handoff`, the fresh generation from `claim`/`takeover`, and the scope identifier and separated capabilities from `prepare-scope`. Preserve each operation's existing token meaning and the rule that parent capability stays with the parent.
- Derive the affected maintained caller sites from a whole-repo search, then update the actionable instructions and examples at those sites. This includes the build-task worker, the build controller, and implement-next's direct evidence re-mint/re-gate paths. Keep operation argv catalog-resolved, and regenerate any embedded distribution copies through their existing generator.
- State that a missing, malformed, or incomplete required response is a caller-contract failure, not permission to rerun `start` to recover credentials. Use the caller's existing blocked/halt reporting posture (a build-task worker returns `BLOCKED` with the missing-response reason). Existing handoff, claim, and event-authorized takeover remain subject to their existing ownership prerequisites; the documentation must not invent credentials, infer them from a drive id, or mint a new recovery path.
- Preserve the current human-output credential boundary and the existing driver behavior. Keep each caller's WAITING/continuation policy intact while clarifying the transport.

### Acceptance criteria

1. A caller following the start instructions captures the drive id and generation from that same JSON response and can supply the required values to its existing advance or handoff step without launching another drive.
2. Every maintained gate-drive caller that consumes a returned token or capability explicitly uses JSON or directly invokes a shared requirement that does so. Sibling operations have no equivalent capture gap.
3. The documented missing-response path maps to the caller's existing blocked/halt outcome and never recommends rerunning `start` for credential recovery.
4. Existing credential-redaction behavior and per-role continuation rules remain intact. Review the source and generated instruction surfaces; any added or changed guard must fail when its JSON-capture requirement is removed. Run the configured whole suite at the later build gate.

### Trivial rationale

The human-approved decision is settled: this is a bounded clarification of the transport for existing commands and existing result fields. It introduces no CLI field, protocol, ownership transition, or architectural decision, so a separate design spec is unnecessary. Implementation can proceed from this tightened scope.

## Out of scope

- Printing ownership generations, handoff tokens, or scope capabilities in human-readable output; making that output the machine capture format.
- Making `start` idempotent or adding worktree-wide duplicate-drive prevention (#375).
- Repairing the prepare-scope/start handshake or concurrent-scope identity issues (#405).
- Adding credential recovery commands, changing gate outcomes, or widening handoff/takeover authorization.
- Rewriting frozen plans, results, historical specs, or Accepted ADR prose.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-09-07

2026-09-07: Reconciled against current main (origin/main @ 0d1a7e1b). Confirmed the approved caller-guidance scope is still accurate: `GateDriveResult.HumanText` prints `drive_id` and deliberately omits the ownership generation, and the shared caller contract mentions `--json` only in passing without requiring capture of the first response. Whole-repo search located the maintained caller instruction surfaces to update: skills/docket-build/references/gate-caller-loop.md (shared contract), skills/docket-build-task/SKILL.md, skills/docket-build/SKILL.md, and skills/docket-implement-next/SKILL.md (Step 6 evidence re-mint / re-gate). Embedded distribution copies under internal/assets/embedded/tree/ are byte-identical to source and MUST be regenerated via the genassets generator (go generate ./internal/assets/...), not hand-edited, or the DiffTree check fails. No scope change and no relation change (related: [375,405], discovered_from: [372], adrs: [107] all still correct); 375 (idempotent start / duplicate-drive prevention) and 405 (prepare-scope/start handshake) remain out of scope. Proceeding to plan + build.
