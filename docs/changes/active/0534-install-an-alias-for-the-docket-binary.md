---
id: 534
slug: 'install-an-alias-for-the-docket-binary'
title: 'Install an alias for the docket binary'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-06'
updated: '2026-10-06'
depends_on: []
stacked_on:
related: [532]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-06-install-an-alias-for-the-docket-binary-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/install-an-alias-for-the-docket-binary'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-06T13:50:52Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-06-install-an-alias-for-the-docket-binary-design.md](../../superpowers/specs/2026-10-06-install-an-alias-for-the-docket-binary-design.md) |
<!-- docket:artifacts:end -->

## Why

The binary is reachable only as `docket`. A second, shorter name, `dckt`, is needed by the private-repository instruction delivery in this series. That delivery writes a user-level hook and pointer that must not contain the word "docket", so they invoke `dckt`. The same name is also simply shorter to type.

There are two install paths, and both must produce the alias:

- the public release downloader (`install.sh` published with each release, the path users take from v1.0);
- the development installer behind the repository-root `install.sh`.

## What changes

- Both installers create `dckt` as a symlink to the `docket` binary in the same bin directory: the release downloader records it in its ownership record, and the Go development installer creates it inside its install transaction.
- The alias follows the binary. It is created or refreshed only when absent or already owned, and `docket uninstall` leaves it in place exactly as it leaves the binary.
- A pre-existing `dckt` that docket does not own is never overwritten. The install finishes with a warning, and `docket install check` reports it.
- The downloader's contract and its tests admit the added `ln -s` and cover the alias.
- Documented only in installer output and the downloader's usage text.

## Out of scope

- Renaming the binary, or changing the capability catalog's spelling (`docket`) that skills resolve commands from.
- Shell aliases, completions, or package-manager formulas.
- Using the alias anywhere other than the user-level surfaces that need it (a separate change in this series).

## Reconcile log

### 2026-10-06

Reconciled against main 8ee926604. Traced internal/release/downloader/install.sh, internal/install/devmode.go, uninstall.go: no dckt alias exists anywhere yet; #529-#531 private-visibility work landed but touches neither installer. #532 (related) still proposed and is the consumer. Scope unchanged.
