---
id: 374
slug: 'separate-build-and-finalize-test-configuration'
title: 'Separate build and finalize test configuration'
status: 'in-progress'
priority: 'high'
type: 'refactor'
created: '2026-08-30'
updated: '2026-09-01'
depends_on: [370]
stacked_on:
related: [167, 316, 318, 352, 360, 370]
discovered_from: []
adrs: [63, 74, 95, 99]
spec: 'docs/superpowers/specs/2026-08-30-separate-build-and-finalize-test-configuration-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/separate-build-and-finalize-test-configuration'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-01T02:08:24Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-30-separate-build-and-finalize-test-configuration-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-separate-build-and-finalize-test-configuration-design.md) |
| ADRs | [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Build and finalize currently share `finalize.test_command`, even though they certify different
workflow moments. A repository with no tests can disable finalize's gate but cannot disable the
build gate; setting the command to `true` is the only practical workaround and records a test pass
that never happened. The documented `auto` fallback is also stale: the Go gate treats it as an
empty, unavailable command, while build still refers to removed Bash-era discovery.

## What changes

- Give build and finalize independent gate and command settings, including an explicit build-off
  path with truthful skipped evidence.
- Remove runtime `auto`; local gates with no command halt with a typed setup remedy rather than
  guessing or entering repair.
- Add one deterministic test-discovery planner used by repository init, legacy migration, and a
  new `repository configure-tests` upgrade path for already-initialized repositories.
- Keep generated config human-gated: init and configure-tests leave pending unstaged edits;
  migration includes exact bytes in its existing confirmation preview.
- Split native gate/evidence/context ownership so build reads only build policy and finalize reads
  only finalize policy. Finalize may reuse green build evidence only when both head and command
  match; a skipped build never waives finalize.
- Replace the shared-command decision in ADR-0063 through the ADR workflow and update every
  maintained config, skill, bundled asset, self-hosting, documentation, and mutation-test touch
  point derived from the post-0370 tree.

## Out of scope

Runtime test discovery; silent installer/status/autonomous config edits; a universal test
orchestrator; worker-focused test selection; CI-only finalize policy, approval, merge, or repair
changes; and rewrites of historical changes, accepted ADR bodies, frozen plans/results, or legacy
fixtures.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
