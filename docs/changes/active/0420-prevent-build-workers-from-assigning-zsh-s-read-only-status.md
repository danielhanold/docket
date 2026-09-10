---
id: 420
slug: 'prevent-build-workers-from-assigning-zsh-s-read-only-status'
title: 'Prevent build workers from assigning zsh''s read-only status parameter'
status: 'implemented'
priority: 'critical'
type: 'fix'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [263, 376, 416]
discovered_from: [323]
adrs: [107]
spec:
plan: 'docs/superpowers/plans/2026-09-09-prevent-build-workers-from-assigning-zsh-reserved-status.md'
results: 'docs/results/2026-09-09-prevent-build-workers-from-assigning-zsh-s-read-only-status-results.md'
trivial: true
auto_groomable:
branch_prefix:
branch: 'fix/prevent-build-workers-from-assigning-zsh-s-read-only-status'
pr: 'https://github.com/danielhanold/docket/pull/296'
blocked_by:
reconciled: true
claimed_at: '2026-09-10T01:53:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-09-09-prevent-build-workers-from-assigning-zsh-reserved-status.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-09-prevent-build-workers-from-assigning-zsh-reserved-status.md) |
| Results | [2026-09-09-prevent-build-workers-from-assigning-zsh-s-read-only-status-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-09-prevent-build-workers-from-assigning-zsh-s-read-only-status-results.md) |
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md) |
<!-- docket:artifacts:end -->

## Why

A resumed change 0323 build worker captured the first gate.drive.start response and then executed `status=$?`. In zsh, `status` is a read-only special parameter, so the shell aborted before the worker could parse the drive id and owner generation or make any TDD edits. The coordinator remained live without visible feature-worktree activity until a human intervened. The maintained agent contract requires JSON capture but does not give workers a shell-safe exit-code capture shape or mechanically prevent this reserved-name failure.

## What changes

Strengthen the maintained build-controller, build-task, and shared gate-caller instructions so gate responses and exit codes are captured with explicit shell-safe names such as `gate_reply` and `gate_rc`, compatible with both zsh and bash, and never assigned to zsh special parameters such as `status` or `pipestatus`. Add a whole-repository syntactic guard over maintained executable agent-facing workflow instructions that detects reserved-parameter assignments at gate-call capture sites, with a mutation test proving that inserting `status=$?` makes the guard fail. Regenerate embedded skill assets through the existing generator and keep source and distributed instruction surfaces byte-aligned.

### Acceptance criteria

1. A build worker following the maintained gate-start capture instructions can run under zsh without assigning a read-only special parameter and can parse the original response's drive id and owner generation.
2. The instructions provide explicit shell-safe response and exit-code variable names and preserve the rule that a missing or malformed first response halts rather than rerunning `gate.drive.start`.
3. A whole-repository guard covers every maintained executable gate-call capture instruction; inserting `status=$?` at a covered site makes the guard fail for the intended reason.
4. Source and generated skill surfaces remain byte-aligned, and the configured whole suite passes at the build gate.

### Trivial rationale

The failure and correction are mechanically settled by the exact worker transcript: `status=$?` fails under zsh with `read-only variable: status`, while a non-reserved local such as `gate_rc` preserves the same exit code. This change adds no protocol field, state transition, or architecture decision, so a separate design specification is unnecessary.

## Out of scope

Changing gate-driver ownership, identity, continuation, or retry semantics; making gate starts idempotent; weakening fail-closed response validation; broadly linting immutable archives, accepted ADRs, historical plans, or unrelated shell code; and resuming or implementing change 0323 as part of this fix.

## Reconcile log

### 2026-09-10

2026-09-09: Reconciled against current main (2f83683c). Confirmed still accurate and unbuilt: a whole-repository grep finds no `status=$?` or reserved-parameter assignment at any gate-call capture site, so the transcripted failure is unregressed but unguarded. The maintained gate-call capture instructions (docket-build SKILL, docket-build-task SKILL, docket-implement-next SKILL, and docket-build/references/gate-caller-loop.md) still describe capturing the JSON drive id and owner generation without naming shell-safe capture variables (e.g. `gate_reply`/`gate_rc`) or forbidding zsh special parameters. No repoguard test yet guards gate-call capture sites against reserved-parameter assignment. Source skills under skills/ and their embedded copies under internal/assets/embedded/tree/skills/ must stay byte-aligned via the existing generator. Scope, acceptance criteria, and out-of-scope remain correct as written; no relation changes needed.
