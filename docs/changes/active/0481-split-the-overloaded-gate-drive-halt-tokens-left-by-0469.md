---
id: 481
slug: 'split-the-overloaded-gate-drive-halt-tokens-left-by-0469'
title: 'Split the overloaded gate-drive halt tokens left by 0469'
status: 'in-progress'
priority: 'medium'
type: 'refactor'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: [469]
stacked_on:
related: []
discovered_from: [469]
adrs: [129]
spec: 'docs/superpowers/specs/2026-10-01-split-the-overloaded-gate-drive-halt-tokens-left-by-0469-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/split-the-overloaded-gate-drive-halt-tokens-left-by-0469'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-01T11:59:50Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-01-split-the-overloaded-gate-drive-halt-tokens-left-by-0469-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-01-split-the-overloaded-gate-drive-halt-tokens-left-by-0469-design.md) |
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0469 renamed two gate-drive halt tokens, and each new name now also covers one condition it does not describe. `worktree-changed` (formerly `identity-mismatch`) also fires when a takeover finds the drive's recorded scope identity (branch, worktree, change, task, or phase) no longer matches its scope, in `internal/gatedrive/takeover.go` — the worktree itself did not change. `launch-unconfirmed` (formerly `unresolved-execution`) also fires when `resolveDriveRun` loses the drive's worktree-slot link to its run (slot unreadable or reassigned) — the launch itself is not in doubt. Behavior is correct; the token misleads whoever reads the halt and picks a remedy. Deferred from PR #358 review finding 1, and first flagged during 0469's initial run.

## What changes

Give each mis-covered condition an accurate halt token:

- **Takeover scope drift** halts with the existing `scope-identity-mismatch` token (`ErrScopeIdentityMismatch`) — the same token `start` already refuses with for this condition, and ADR-0129 already retains it. No new vocabulary.
- **Lost run link** (both `resolveDriveRun` slot branches) halts with one new token, `run-link-lost`.

Update the emitting sites, the tests that assert them (including the run-verdict takeover pass-through fixture), and the glossary, and record the outcome with a dated `## Update` to ADR-0129. Skill prose is unchanged — its `launch-unconfirmed` mentions describe the worktree-admission refusal, which stays correct.

## Out of scope

Changing when the gate drive halts or how it recovers; the correct `worktree-changed` / `launch-unconfirmed` sites and the `ErrLaunchUnconfirmed` refusal kind; finalize's `mapDriveHaltCause` mapping (both tokens stay `unavailable`); any other ADR-0129 rename rows; the leftover 0469 wording ("repair" in `change relink`, "Step 0"), tracked separately.

## Reconcile log

### 2026-10-01

2026-10-01 — Reconciled against main 85bace7 (0469 merged). A whole-repo grep confirms the spec trace: takeover.go scopeIdentityMatch halts "worktree-changed"; driver.go resolveDriveRun returns "launch-unconfirmed" on both slot branches; the run-verdict fixture uses worktree-changed. The repoguard retired-vocabulary fixtures (rows 68/69) and the admission_test history-cause lists use the tokens as historical data and stay unchanged. Scope unchanged.
