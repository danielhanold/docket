---
id: 366
slug: 'human-attended-v1-0-0-rc1-acceptance-and-publication'
title: 'Human-attended v1.0.0-rc1 acceptance and publication'
status: 'proposed'
priority: 'critical'
type: 'chore'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [318]
stacked_on:
related: [317, 322, 326, 352, 361, 363]
discovered_from: [318]
adrs: [95, 96, 99]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Change 0318 will make the reviewed source Go-only, but source acceptance is not release acceptance. The exact merged candidate still needs human-attended proof across native targets and genuinely fresh harness processes, an isolated rollback rehearsal, explicit migration-ledger dispositions, and irreversible publication. Keeping those external and post-merge gates separate lets 0318 remain one autonomous code PR without weakening the final cutover standard.

## What changes

- Audit and disposition the complete active migration backlog through the installed Go product, preserving independent post-Go work and recording manual migration learnings.
- Package one candidate from the exact merged 0318 commit and use the same checksum-identified bytes for all native tuple, self-host, rollback, publication, and public-install gates.
- Run Darwin/Linux on amd64/arm64 native smokes, then complete the retained mutating lifecycle plus restart/resume recovery in genuinely fresh Claude, Codex, Cursor, and OpenCode sessions.
- Rehearse an isolated v0.9.2 rollback without introducing a runtime fallback.
- At an explicit human irreversible boundary, create and verify the v1.0.0-rc1 tag, GitHub Release, assets, checksums, and clean public installation.
- Collate durable evidence and close the migration/release metadata only after every gate passes.

## Out of scope

Source-cutover implementation owned by 0318; changes to the accepted candidate after packaging; rebuilding or substituting bytes mid-protocol; a Bash fallback or compatibility launcher; stable v1.0.0 promotion; Homebrew, Windows, signing/notarization, SBOM or provenance signing; uninstall or version-tree garbage collection; and redesign of Docket storage, JSON protocol, harness topology, or Git/GitHub adapters. A source defect found during acceptance returns to a separate reviewed code change and invalidates the candidate rather than being repaired inline.
