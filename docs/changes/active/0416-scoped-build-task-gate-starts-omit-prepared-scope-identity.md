---
id: 416
slug: 'scoped-build-task-gate-starts-omit-prepared-scope-identity'
title: 'Scoped build-task gate starts omit prepared scope identity'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-09'
updated: '2026-09-09'
depends_on: []
stacked_on:
related: [359, 376, 405]
discovered_from: [323]
adrs: [107]
spec:
plan:
results:
trivial: true
auto_groomable:
branch_prefix:
branch: 'fix/scoped-build-task-gate-starts-omit-prepared-scope-identity'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-09T21:35:53Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md) |
<!-- docket:artifacts:end -->

## Why

Change 0323 halted before its first implementation task because the dispatched build worker followed the maintained docket-build-task instructions and invoked gate.drive.start with the scope id, child capability, and run root, but without the change id, task id, phase, or branch pinned by gate.drive.prepare-scope. The driver correctly rejects those empty values through scopeIdentityMatch, and the CLI maps that ownership failure to invalid-input / invalid-request with no drive. The failure reproduces with the documented call shape; reusing the same scope while adding the four pinned identity fields produces an applied, passing drive. The source, embedded, and installed worker contracts contain the same omission, while the CLI regression always supplies every field and the existing workflow guard checks JSON capture but not scope-identity propagation. As written, every scoped build-task worker can halt before running its first test.

## What changes

Require the build controller to include one complete start-ready scope bundle in every scoped task dispatch: the canonical feature worktree, change id, task id, phase, branch, scope id, child capability, and outer gate context when present. Require the build-task worker to start each task-owned drive from that worktree and pass the bundle unchanged to `gate.drive.start`, including `--repo-dir`, `--change-id`, `--task-id`, `--phase`, `--branch`, `--scope-id`, `--child-cap`, and `--gate-context` when present. The parent capability remains parent-only.

Add a whole-repository syntactic guard over maintained scoped task-start instructions so an instruction site cannot omit any pinned identity field or the gate-context pass-through. Derive the sites from repository search, mutation-test every protected field, and regenerate the embedded skill assets through the existing generator. Keep the existing fail-closed driver behavior and JSON capture contract.

### Acceptance criteria

1. A build-task worker following the maintained dispatch and start instructions binds a scoped task-owned drive whose repository, worktree, change, task, phase, branch, and gate-context identity matches the prepared scope.
2. The former worker call shape that supplies only scope id, child capability, and run root remains rejected before launch; supplying the complete documented bundle produces an applied drive and preserves parent takeover attribution.
3. A whole-repository guard covers every maintained scoped task-start instruction site, and removing each required identity or gate-context field makes the guard fail for the intended reason.
4. Source and generated skill surfaces remain byte-aligned, and the configured whole suite passes at the build gate.

### Trivial rationale

The driver, CLI fields, ownership transition, and authorization rule already implement the intended behavior. The defect is a mechanically incomplete caller contract: the controller pins identity that its worker instructions do not pass back. The reproduced omitted-field failure and successful complete-field round trip settle the design, so no separate specification or ADR is needed. Broader concurrent-scope and duplicate-drive behavior remains with change 0405.

## Out of scope

Changing the gate driver to infer or hydrate omitted scope identity; weakening scopeIdentityMatch; changing capability, handoff, takeover, or credential-redaction semantics; redesigning the invalid-request error vocabulary; resolving broader concurrent-scope or duplicate-drive questions tracked by change 0405; and resuming or implementing change 0323.

## Reconcile log

### 2026-09-09

Reconciled against current source. Design holds unchanged. The gate.drive.start CLI already accepts every identity flag (--repo-dir/--change-id/--task-id/--phase/--branch/--scope-id/--child-cap/--gate-context), and gate.drive.prepare-scope pins them, so the defect is purely a mechanically incomplete caller contract: skills/docket-build-task/SKILL.md instructs the worker to start the task-owned drive with only --owner task --scope-id --child-cap --run-root --json, and skills/docket-build/SKILL.md hands the worker only scope-id + child-cap. Clarification of the change body's 'source, embedded, and installed worker contracts' wording: there are two checked-in copies — the source skill under skills/ and its byte-identical embed under internal/assets/embedded/tree/skills/ — and the 'installed' surface is written verbatim from the embed at install time (internal/cli/install.go via internal/assets/embedded.go), so fixing source + running `go generate ./internal/assets/` covers all three. The whole-repo syntactic guard will live in internal/repoguard as a new *_test.go modeled on the existing TestGateDriveJSONCapture (gatedrive_json_capture_test.go), scanning maintained workflow markdown (isWorkflowMD corpus, which already includes the embedded mirror) with a population/coverage floor and mutation-tested required-field regexes. Related 405 (broader concurrent-scope / duplicate-drive questions) and resuming/implementing 323 remain out of scope.
