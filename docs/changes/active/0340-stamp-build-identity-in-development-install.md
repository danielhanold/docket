---
id: 340
slug: stamp-build-identity-in-development-install
title: "Stamp build identity into the `development install` binary"
status: 'implemented'
priority: medium
type: fix
created: 2026-08-22
updated: '2026-08-23'
depends_on: []
stacked_on:
related: [317]
discovered_from: [335]
adrs: []
spec: docs/superpowers/specs/2026-08-23-stamp-build-identity-in-development-install-design.md
plan: 'docs/superpowers/plans/2026-08-23-stamp-build-identity-in-development-install.md'
results:
trivial: false
auto_groomable:
branch: 'feat/stamp-build-identity-in-development-install'
pr: 'github.com/danielhanold/docket#233'
blocked_by:
reconciled: true
claimed_at: '2026-08-23T22:05:44Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-23-stamp-build-identity-in-development-install-design.md` |
| Plan | `docs/superpowers/plans/2026-08-23-stamp-build-identity-in-development-install.md` |
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
binary from a checkout, so `docket version` reports a truthful version, commit, and
build date (with a `-dirty` marker when the checkout has uncommitted changes).
Version comes from `git describe --tags --always --dirty`; git probes run through an
injectable runner seam beside the existing `GoRunner` in
`internal/install/devmode.go`. On any git failure the build proceeds unstamped
(compiled-in defaults) — identity is a nicety, never an install gate. Reuses the
existing `internal/buildinfo` variables and the release build's `-X` stamping format
rather than introducing a parallel mechanism.

## Out of scope

- The release/packaging build path (change 0317 already stamps it).
- Adding new fields to `internal/buildinfo`.
- Any change to the AGENTS.md rebuild-on-merge convention (it stays as a safety net).

## Open questions

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-23

### 2026-08-23 — reconciled against current main

Verified the design against current source; no scope change.

- `internal/buildinfo` carries the `Version`/`Commit`/`BuildDate` ldflags vars and `Info`/`Current()` exactly as the spec assumes — untouched, no new fields needed.
- `internal/install/devmode.go` `buildBinary` still runs a bare `go build -o <staged> ./cmd/docket` through the injectable `GoRunner` seam (wired to `install.DefaultGoRunner` in `internal/cli/root.go`); the new git-probe runner seam lands beside it as designed.
- Change 0317's `internal/release/package.go` is NOT merged (the `internal/release/` directory is absent on main; 0317 is `implemented`, PR open, not `done`). The spec already anticipated this: duplicate the three-`-X` format string locally rather than sharing 0317's helper, and do not take a dependency on 0317's branch. The exact `-X github.com/danielhanold/docket/internal/buildinfo.{Version,Commit,BuildDate}=...` format is confirmed by the existing injection test at `cmd/docket/main_test.go`.
- No related/archived change or ADR invalidates the approach; relations left unchanged.
