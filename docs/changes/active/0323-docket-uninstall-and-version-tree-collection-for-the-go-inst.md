---
id: 323
slug: docket-uninstall-and-version-tree-collection-for-the-go-inst
title: 'docket uninstall and version-tree collection for the Go installer'
status: 'in-progress'
priority: medium
type: feat
created: 2026-08-14
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [311, 317, 322, 351]
discovered_from: [311]
adrs: [96, 110]
spec: 'docs/superpowers/specs/2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md'
plan: 'docs/superpowers/plans/2026-09-09-docket-uninstall-and-version-tree-collection.md'
results: 'docs/results/2026-09-10-docket-uninstall-and-version-tree-collection-for-the-go-inst-results.md'
trivial: false
auto_groomable:
branch: 'feat/docket-uninstall-and-version-tree-collection-for-the-go-inst'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-10T19:52:30Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md) |
| Plan | [2026-09-09-docket-uninstall-and-version-tree-collection.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-09-docket-uninstall-and-version-tree-collection.md) |
| Results | [2026-09-10-docket-uninstall-and-version-tree-collection-for-the-go-inst-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-10-docket-uninstall-and-version-tree-collection-for-the-go-inst-results.md) |
| ADRs | [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0110](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0110-install-path-configuration-reads-tolerate-unknown-keys-the-s.md) |
<!-- docket:artifacts:end -->

## Why

Release installs accumulate immutable asset versions, and users have no command to remove Docket's user-level harness integrations. Change 0311's deep review identified uninstall and safe version collection as an independent follow-up. Provide a predictable removal path and reclaim verified asset trees that no remaining installation references, including installations whose harnesses use different versions after scoped upgrades.

## What changes

- Add `docket uninstall` for all recorded harness integrations or an explicit harness selection, with ownership checks, dry-run output, and journaled recovery.
- Add `docket install collect` and automatically collect unused versions after successful release installs, development installs, and uninstalls.
- Derive references from the complete installed state and recorded links, verify current and legacy version-tree contents, and preserve unprovable targets.
- Make interrupted collection resumable and report cleanup failures separately from successful installation.
- Preserve the CLI and its ownership records, and support reinstalling from a valid empty-harness state.

## Out of scope

Removing development or release CLI binaries; deleting global configuration or source checkouts; changing repository-local instruction surfaces, metadata, or ownership records; force/purge switches; scanning the user's home or repositories for arbitrary references; and background scheduling. The release downloader's separate binary ownership lifecycle remains unchanged.

## Reconcile log

### 2026-09-09

2026-09-09 — Reconciled against origin/main at 881d7cfb. Changes 0311, 0317, 0322, and 0351 are merged and archived; the Go installer, release/development installation, ownership records, legacy reproducer, and global-dispatch retirement foundations described by the spec are present. Change 0323 remains the focused follow-up for uninstall, reference-derived version-tree collection, resumable collection journaling, and associated CLI/schema/documentation/tests. No dependency, stack base, relation, or scope adjustment is required; no adjacent follow-up work was surfaced beyond already-tracked changes.

### 2026-09-10

2026-09-10 — Reconciled against origin/main at 2f83683c. Change 0416 is now merged and fixes the scoped task-owned gate-start identity handshake that halted the prior build at Task 1; the existing plan can resume at that task without scope changes. The approved spec remains current, changes 0311, 0317, 0322, and 0351 remain merged, and no dependency, stack base, relation, or adjacent follow-up adjustment is required.

### 2026-09-10

2026-09-10 — Reconciled against origin/main at 6f98577d. Change 0416 is merged and repairs the scoped build-task gate-start identity handoff that caused the prior Task 1 halt; changes 0420 and 0421 update gate capture and budget infrastructure without changing installer behavior. The approved spec and linked ADRs 0096 and 0110 remain current. The feature workspace already contains the committed plan and results, and the intended Task 1 state-boundary edits remain the only uncommitted code. Scope, relations, and the no-new-follow-up assessment remain unchanged.

### 2026-09-10

2026-09-10 — Reconciled against origin/main at 0f84b9e3. Since the prior pass at 6f98577d, only change 0405's sequential gate-drive lifecycle work landed; it does not alter installer scope. The existing plan and results remain committed on the feature branch, and the owned Task 1 edits remain the only uncommitted implementation files. The approved spec, ADRs 0096/0110, relations, and focused scope remain current; no new dependency, stack-base, relation, or follow-up adjustment is required.

## Run halted

### 2026-09-10

Task 7 cannot be completed safely because the native gate-driver handoff contract is unavailable. The standard worker's focused RED/GREEN verification passed, but its required mutation-test drive reached WAITING and the driver's JSON response omitted the single-use handoff token. The event-authorized takeover reported HALTED with cause `handoff-outstanding`; after the keyed continuation claimed the same drive, one bounded advance produced the expected mutation-test FAILED result, but the outer gate verdict was `gate-stop gate-unavailable takeover-ambiguous`.

No Task 7 commit was created. The feature worktree remains owned and dirty with the nine assigned Task 7 files. The preserved deliberate mutation currently removes the `install.collect` capability annotation from `internal/cli/root.go`; restore that annotation before resuming. The prior Task 7 edits otherwise passed the focused command:

- `go test -count=1 ./internal/app ./internal/cli ./cmd/docket -run 'Test(Uninstall|InstallCollect|InstallResult|Capabilities|OperationBindings|InstallCommands|AssetIndependent|Schema)'`
- Focused RED confirmed the missing `install.collect` capability metadata; GREEN passed after restoring it.

The parent gate key is terminally stopped for this run; no replacement worker, raw gate observation, fabricated handoff token, or fresh retry was started.
