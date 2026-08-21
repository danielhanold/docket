---
id: 317
slug: release-packaging-and-four-harness-acceptance
title: 'Release packaging and four-harness acceptance'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-20
depends_on: [311, 316]
stacked_on:
related: [318, 322, 323]
discovered_from: [303]
adrs: [60]
spec: docs/superpowers/specs/2026-08-20-release-packaging-and-four-harness-acceptance-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-20-release-packaging-and-four-harness-acceptance-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-20-release-packaging-and-four-harness-acceptance-design.md) |
| ADRs | [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md) |
<!-- docket:artifacts:end -->

## Why

A complete local engine is not a release until the same candidate bytes are checksummed, exercised
on every supported target, installed and upgraded safely, and invoked directly through every
first-class harness.

## What changes

Add deterministic Darwin/Linux amd64/arm64 candidate packaging, a checksum-verifying POSIX release
downloader, a non-publishing native smoke matrix, and fresh-session direct-invocation acceptance for
Claude, Codex, Cursor, and OpenCode. The live matrix uses one known-answer read-only status scenario
so it proves the release/harness boundary without retesting upstream lifecycle behavior.

## Out of scope

Behavior owned by changes 0305–0316; the source-development bootstrap from 0322; uninstall and
version-tree collection from 0323; Homebrew, Windows, signing/notarization, cross-harness runners,
and public plugins; and 0318's release publication, self-hosting, Bash removal, documentation
replacement, and hard cutover.

## Design decisions

One repo-owned packager produces a non-public candidate bundle once; native jobs smoke those exact
bytes rather than rebuilding per platform. The release downloader keeps its own exact binary
ownership record, runs the landed embedded-asset installer before an atomic binary replacement,
and refuses unknown destinations without a force path. Live acceptance requires a fresh native
process and observed named-agent child in each harness; generated goldens cannot substitute.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
