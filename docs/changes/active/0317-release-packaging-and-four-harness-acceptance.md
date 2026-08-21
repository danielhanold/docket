---
id: 317
slug: release-packaging-and-four-harness-acceptance
title: 'Release packaging and four-harness acceptance'
status: 'in-progress'
priority: critical
type: feat
created: 2026-08-12
updated: '2026-08-21'
depends_on: [311, 316]
stacked_on:
related: [318, 322, 323]
discovered_from: [303]
adrs: [60]
spec: docs/superpowers/specs/2026-08-20-release-packaging-and-four-harness-acceptance-design.md
plan: 'docs/superpowers/plans/2026-08-20-release-packaging-and-four-harness-acceptance.md'
results:
trivial: false
auto_groomable:
branch: 'feat/release-packaging-and-four-harness-acceptance'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-21T02:58:15Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-20-release-packaging-and-four-harness-acceptance-design.md` |
| Plan | `docs/superpowers/plans/2026-08-20-release-packaging-and-four-harness-acceptance.md` |
| ADRs | `docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md` |
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

### 2026-08-21

2026-08-21 — Reconciled at claim. Verified against current reality: dependencies 311 (installer embedded assets + four harnesses) and 316 (finalize/recovery/reclaim/archive/stacks) are both merged and `done`, so the landed foundation the spec names is in place — `cmd/docket`, `internal/buildinfo` ldflags identity (Version/Commit/BuildDate), `internal/install` + `install`/`install check` commands, and the four native harness definitions (Claude, Codex, Cursor, OpenCode). No `release`/packaging package, POSIX downloader, or `.github/workflows/release-candidate` exists yet — all remain 317's net-new deliverables, so no work has been done elsewhere to drop. Scope, out-of-scope boundaries against 0318/0322/0323, and relations (depends_on [311,316], related [318,322,323], adrs [60]) remain accurate; no adjustment needed. The buildable branch deliverable is the deterministic Go packager + artifact contract, the `/bin/sh` checksum-verifying release downloader with its ownership record, the non-publishing release-candidate GitHub Actions workflow, and the hermetic package/downloader tests. The fresh-session four-harness live acceptance is external truth that no in-repo test can promote (per the spec's own gate); it is carried to the human merge gate as a documented manual checklist in the results record rather than automated in-branch.
