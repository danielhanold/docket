---
id: 392
slug: 'installer-tolerant-config-read-break-the-schema-bump-bootstr'
title: 'Installer-tolerant config read: break the schema-bump bootstrap deadlock'
status: 'done'
priority: high
type: 'fix'
created: '2026-09-01'
updated: '2026-09-04'
depends_on: []
stacked_on:
related: []
discovered_from: [374]
adrs: [19, 102, 110]
spec: 'docs/superpowers/specs/2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr-design.md'
plan: 'docs/superpowers/plans/2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/installer-tolerant-config-read-break-the-schema-bump-bootstr'
pr: 'https://github.com/danielhanold/docket/pull/276'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr-design.md) |
| Plan | [2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr.md) |
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md), [ADR-0110](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0110-install-path-configuration-reads-tolerate-unknown-keys-the-s.md) |
<!-- docket:artifacts:end -->

## Why

docket's config parser enforces a deliberate strict typo policy: an unrecognized key in .docket.yml is a hard `invalid configuration` error, so typos like `finalze:` are caught rather than silently dropped. The unintended consequence is a self-hosting bootstrap deadlock. When a merged change extends the .docket.yml schema (change 0374 added a `build:` block), the still-installed pre-schema binary begins rejecting ALL config reads with `invalid configuration` — and because `docket development install` triggers a repository-phase config read at startup, the installer itself can no longer run. The one tool that would rebuild the binary out of the corner is blocked by the very field it needs to be rebuilt to understand: a hard deadlock, recoverable today only by hand (`go build`/`go run` out-of-band, manual binary swap, then a tracked reinstall). This will recur for every future schema-extending change; the CLAUDE.md 'rebuild after merge to main' rule is load-bearing precisely because of it. Discovered while finalizing change 0374; an interim CLAUDE.md caveat documents the manual recovery until this lands.

## What changes

On the install path only, tolerate unknown configuration keys. `config.ResolveContext` gains a `TolerateUnknownKeys` flag that reclassifies the `unknown-key` diagnostic class — at any depth — from error to warning, with a remedy naming both causes (a newer docket than the one running, or a typo); the snapshot stays valid and the unknown subtree resolves to defaults. The flag is set at exactly one site, the CLI's shared `installOptions`, so `docket install`, `docket install check`, and `docket development install` all complete against a config written for a newer schema, and the development-install parent reaches the build-and-hand-off step instead of refusing. Malformed YAML, duplicate keys, wrong types, bad values, and the coordination fence stay fatal on the install path; every other command keeps the strict typo policy unchanged. The install reads stop discarding their diagnostics: warning-severity diagnostics surface in the install result (`warnings`) and as one human line each. One ADR records the decision; CLAUDE.md's schema-bump rebuild caveat is replaced by a one-line note.

## Out of scope

General-parser forward-compatibility for operating commands (status, finalize, prepare, and the rest) and schema versioning — rejected in brainstorm in favour of the narrow install-only fix. Any relaxation of the other invalid diagnostic classes or the coordination fence, on any path. A parent-side warning line for `development install` (the candidate prints the sole document). Changes to the existing curated warn-and-ignore surfaces (unknown `skills.*` roles, `board.sorting.*` sections).

## Reconcile log

### 2026-09-04

2026-09-04: Reconciled against current main. All spec touchpoints verified present and unchanged: config.ResolveContext (internal/config/config.go:27), invalidClass/CodeUnknownKey (internal/config/resolve.go:35-36), ResolveRepoPhase (internal/app/repophase.go:57), installOptions (internal/cli/install.go:197), InstallResult/NewInstallResult/withRepoReporting (internal/app/install.go). No general-parser forward-compat has landed in the interim, so the narrow install-only design still holds. depends_on empty; adrs [19,102] and discovered_from [374] remain accurate. No scope change; proceeding to plan and build.
