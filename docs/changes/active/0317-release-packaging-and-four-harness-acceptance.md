---
id: 317
slug: release-packaging-and-four-harness-acceptance
title: 'Release packaging and four-harness acceptance'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [311, 316]
stacked_on:
related: []
discovered_from: [303]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

A complete local engine is not a release until users can verify, install, upgrade, and invoke it
directly through every first-class harness on every supported target.

## What changes

Build checksummed Darwin/Linux amd64/arm64 artifacts, the POSIX downloader, install and upgrade
smokes, and live direct-invocation acceptance for Claude, Codex, Cursor, and OpenCode.

## Out of scope

Homebrew, Windows, cross-harness runners, public plugins, and the final self-hosting deletion pass.

## Open questions

Settle CI/release workflow details and the exact minimal retained acceptance scenario during
grooming against the complete engine.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
