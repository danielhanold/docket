---
id: 392
slug: 'installer-tolerant-config-read-break-the-schema-bump-bootstr'
title: 'Installer-tolerant config read: break the schema-bump bootstrap deadlock'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-01'
updated: '2026-09-01'
depends_on: []
stacked_on:
related: []
discovered_from: [374]
adrs: [19]
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
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md) |
<!-- docket:artifacts:end -->

## Why

docket's config parser enforces a deliberate strict typo policy: an unrecognized key in .docket.yml is a hard `invalid configuration` error, so typos like `finalze:` are caught rather than silently dropped. The unintended consequence is a self-hosting bootstrap deadlock. When a merged change extends the .docket.yml schema (change 0374 added a `build:` block), the still-installed pre-schema binary begins rejecting ALL config reads with `invalid configuration` — and because `docket development install` triggers a repository-phase config read at startup, the installer itself can no longer run. The one tool that would rebuild the binary out of the corner is blocked by the very field it needs to be rebuilt to understand: a hard deadlock, recoverable today only by hand (`go build`/`go run` out-of-band, manual binary swap, then a tracked reinstall). This will recur for every future schema-extending change; the CLAUDE.md 'rebuild after merge to main' rule is load-bearing precisely because of it. Discovered while finalizing change 0374; an interim CLAUDE.md caveat documents the manual recovery until this lands.

## What changes

On the install/rebuild path ONLY, downgrade the forward-compat error class specifically — an unrecognized/unknown top-level key in the repository config layer — from a fatal `invalid configuration` error to a non-fatal warning, and proceed with defaults for the repository phase so `docket development install --source` always completes even when the installed binary predates the on-disk schema. The leniency is scoped narrowly: only the unknown-field class degrades; genuinely malformed YAML, wrong-typed values, and coordination-fence violations still fail the install unchanged. Every other command (status, finalize, prepare, …) keeps the strict typo policy fully intact — an old binary still refuses to OPERATE on an unknown field, it just can never be blocked from REBUILDING. The seam is the install repository phase's config read (internal/install repophase + its resolveRepo path); the install service already avoids the repo layer (internal/install/service.go:61), so this narrows to that one read. Tests: (1) a `.docket.yml` with an unknown top-level key + a schema-older binary → `development install` completes with a warning, not an error; (2) a malformed/mis-typed `.docket.yml` → install still fails.

## Out of scope

General-parser forward-compatibility (making status/finalize/all commands tolerate a future top-level block) — deliberately rejected in brainstorm in favor of the narrow installer-only fix. Schema versioning (a `schema_version` field gating tolerance). Any change to the strict typo policy in normal (non-install) commands or to the existing curated warn-and-ignore surfaces (unknown skills.* roles, board.sorting.* sections).
