---
id: 340
slug: stamp-build-identity-in-development-install
title: "Stamp build identity into the `development install` binary"
status: proposed
priority: medium
type: fix
created: 2026-08-22
updated: 2026-08-22
depends_on: []
stacked_on:
related: [317]
discovered_from: [335]
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

`docket version` reports `docket development (commit unknown, built unknown)` even
immediately after a fresh `docket development install --source <checkout>`. The
`internal/buildinfo` ldflags identity fields (Version/Commit/BuildDate) already exist
and are wired for release builds (change 0317), but the `development install` build
path does not inject them — so a locally-built binary carries no build identity.

The practical cost: there is no way to tell whether the installed binary matches the
current `main` source. That drift is exactly what the just-added AGENTS.md convention
("rebuild the `docket` binary after every merge to main") works around by always
rebuilding. Stamping a real identity would make the drift *visible* instead of merely
mitigated — you could check the binary against source rather than rebuilding blindly.

Surfaced while finalizing change 0335.

## What changes

Make `docket development install` inject build-identity ldflags when it compiles the
binary from a checkout, so `docket version` reports a truthful commit + build date
(and a dirty-tree marker when the checkout has uncommitted changes). Reuse the
existing `internal/buildinfo` variables and the release build's stamping approach
rather than introducing a parallel mechanism.

## Out of scope

- The release/packaging build path (change 0317 already stamps it).
- Adding new fields to `internal/buildinfo`.
- Any change to the AGENTS.md rebuild-on-merge convention (it stays as a safety net).

## Open questions

- Version string source for a development build: `git describe --tags --dirty`, or
  raw commit SHA + a `-dirty` suffix? What should `Version` read when the checkout is
  not at a tag?
- How to represent a dirty working tree (uncommitted changes) in the stamped identity.
- Where the ldflags are assembled — inside the Go build invocation `development
  install` issues, computed from the `--source` checkout's git state.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
