---
id: 331
slug: 'docket-implement-next-s-re-mint-path-never-names-docket-gate'
title: 'docket-implement-next''s re-mint path never names docket gate launch, so a resumed run cannot produce the run directory evidence record requires'
status: done
priority: 'high'
type: 'fix'
created: '2026-08-19'
updated: 2026-08-21
depends_on: [316]
stacked_on:
related: [330]
discovered_from: [316]
adrs: [66, 74, 95]
spec: 'docs/superpowers/specs/2026-08-20-docket-implement-next-evidence-remint-gate-launch-design.md'
plan: 'docs/superpowers/plans/2026-08-20-docket-implement-next-remint-gate-launch.md'
results:
trivial: false
auto_groomable:
branch: 'feat/docket-implement-next-s-re-mint-path-never-names-docket-gate'
pr: 'github.com/danielhanold/docket#224'
blocked_by:
reconciled: true
claimed_at: 
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-20-docket-implement-next-evidence-remint-gate-launch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-20-docket-implement-next-evidence-remint-gate-launch-design.md) |
| Plan | [2026-08-20-docket-implement-next-remint-gate-launch.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-08-20-docket-implement-next-remint-gate-launch.md) |
| PR | github.com/danielhanold/docket#224 |
| ADRs | [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

An autonomous `docket-implement-next` run resuming change 0316 wedged four times: each attempt ran
the full suite green, then halted because it could not record exact-head evidence. The recovery
instruction says to re-run the suite when evidence is missing or stale, but it never names
`docket gate launch`, the operation that creates the durable run directory required by
`docket evidence record`.

A direct suite invocation therefore produces a verdict but no recordable run handle. The ordinary
build path hides the omission because `docket-build` already launches the supervised gate. The
controller-owned re-mint path is common after a halt/resume or any review fix that moves HEAD, so
leaving the chain implicit makes repeated green runs predictably halt.

## What changes

Make Step 6's re-mint path an executable chain: launch the resolved suite with `docket gate launch
--root <absolute-run-root> --cwd <absolute-feature-worktree> -- <resolved-suite-command>`, observe
the returned run directory under `docket-build`'s existing bounded posture until it reaches terminal
`passed`, then record and verify evidence against that run directory and the exact feature HEAD.

Extend the existing Step 6 contract tests with a whitespace-tolerant, section-scoped guard that
proves the producer/consumer ordering and required launch arguments. The test must reject a
temporary copy with the `gate launch` producer removed and must separately prove its section
extractor is non-vacuous. Regenerate the shipped embedded skill bundle mechanically.

Audit the rest of `docket-implement-next` for the same local missing-producer shape. Keep any broader
workflow or runtime gap as separate work.

## Out of scope

Changing the gate or evidence command interfaces, relaxing exact-head evidence requirements,
duplicating `docket-build`'s observation policy, changing `docket change mark-implemented`, adding a
general Markdown linter, or restoring the deferred learnings harvest from change 0316.

## Reconcile log

### 2026-08-21

### 2026-08-21 — reconcile against current reality

Confirmed the defect is still live on `origin/main`: `skills/docket-implement-next/SKILL.md` Step 6 ("Validate the build evidence") instructs "re-run the full suite once to mint the record yourself" and later consumes `docket evidence record --run <absolute-run-dir>`, but never names `docket gate launch` — the operation that produces the durable run directory. The producer/consumer gap the spec targets is unchanged.

Scope, relations, and citations verified current and left unchanged: `depends_on: [316]` is `done` (dependency satisfied); `related: [330]` (post-merge results appending) is genuinely separate follow-up work, not folded in; `discovered_from: [316]`; cited ADRs 66/74/95 remain Accepted and on point. The Go gate/evidence command behavior is already correct and covered — the fix stays a skill-contract-plus-Bash-test change with a mechanical embedded-asset regeneration, matching the spec. `tests/test_docket_review.sh` and the embedded asset at `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` both exist as the spec assumes. No re-scope required; proceeding to plan and build as specified.
