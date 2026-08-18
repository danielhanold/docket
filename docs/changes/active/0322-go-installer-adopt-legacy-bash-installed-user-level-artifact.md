---
id: 322
slug: go-installer-adopt-legacy-bash-installed-user-level-artifact
title: 'Bootstrap Go development installation and adopt legacy user-level artifacts'
status: in-progress
priority: critical
type: feat
created: 2026-08-14
updated: 2026-08-18
claimed_at: 2026-08-18T18:47:19Z
depends_on: [311]
stacked_on:
related: [316, 326]
discovered_from: [311]
adrs: [96]
spec: docs/superpowers/specs/2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md
plan: docs/superpowers/plans/2026-08-18-development-install-bootstrap-and-legacy-adoption.md
results:
trivial: false
auto_groomable:
branch: feat/go-installer-adopt-legacy-bash-installed-user-level-artifact
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md) |
| Plan | [2026-08-18-development-install-bootstrap-and-legacy-adoption.md](https://github.com/danielhanold/docket/blob/feat/go-installer-adopt-legacy-bash-installed-user-level-artifact/docs/superpowers/plans/2026-08-18-development-install-bootstrap-and-legacy-adoption.md) |
| ADRs | [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md) |
<!-- docket:artifacts:end -->

## Why

Change 0311 shipped the source-linked development installer but left its legacy-reproduction
ownership seam unwired. Existing contributors therefore cannot use the intended Go installer:
their Bash-generated user-level agents and dispatch material collide as unknown files, while the
legacy `install.sh` never creates the Go executable needed to invoke `development install`.

## What changes

Wire byte-proven adoption of known Bash-generated user-level artifacts, and make the checkout's
`install.sh` a thin source-bootstrap entry point. It uses an installed `docket development install`
when available and otherwise runs the same operation through `go run ./cmd/docket`; both paths
build and install the reviewed source binary, source-link the authored assets, render current
harness material, and publish ownership state through change 0311's transaction engine.

## Out of scope

Repository-local artifact adoption, broad overwrite or deletion switches, release download and
packaging, repository configuration contraction, metadata transactions, finalize/recovery
behavior, Bash product removal, and hard cutover. A transient `go run` process is authorized only
to perform development installation; this change does not make arbitrary from-source transaction
commands a sanctioned control plane for shared Docket metadata.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-18 — reconcile against HEAD (2cb14827), implemented via v0.9.2 Bash bridge

Verified current reality against the spec. Change 0311 is `done` (archived 2026-08-14) and landed
**more than this change's "Why" framing implies**: the `development install` command and the entire
`internal/install` engine already exist and are complete for release + development modes. Scope is
therefore narrower than a from-scratch installer — it is exactly the two seams the spec names.

Confirmed already built at HEAD (dropped from the plan):
- `development` cobra group + `install` subcommand with `--source`/`--bin-dir`/`--harness` flags
  (`internal/cli/root.go:256-290`); `"development"`/`"development install"` already in the
  asset-independent allowlist (`internal/cli/install.go:76-77`).
- `app.RunDevelopmentInstall` → `install.DevelopmentInstall` (`internal/app/install.go:60-62`,
  `internal/install/devmode.go:75`): binary build+staging, source-link (ModeDevelopment), drift
  gate, atomic binary placement as an owned target, lock/transaction/recovery, and
  `state/install.json` publication (`txn.go:493`, `state.go:97`, `roots.go:104`).
- The three-proof ownership classifier incl. the `LegacyReproducer` seam
  (`internal/install/inspect.go:50,64,269`) — interface + plumbing done.
- Capability fence integrated on the dev-install path (`devmode.go:83` → `config.PreflightMutation`);
  this change must NOT modify it (spec exclusions). Fence lives in `internal/config/preflight.go`
  + `internal/config/schema.go` — untouched here.
- All four harness renderers (claude/codex/cursor/opencode) with goldens.

Genuinely remaining scope (what this change builds):
1. Convert repo-root `install.sh` from the legacy 4-primitive Bash installer
   (`ensure-global-config.sh` / `link-skills.sh` / `sync-agents.sh` / `ensure-docket-env.sh`,
   still present at `install.sh:29-46`) into the POSIX bootstrapper: tri-state binary discovery →
   `docket development install --source <checkout>` when a compatible binary exists, else
   `go run ./cmd/docket development install --source <checkout>`; no legacy primitive runs. + tests.
2. Implement the frozen, deterministic legacy byte reproducer for the closed user-level inventory
   (native user-level docket agent defs, Cursor's `docket-dispatch.mdc` rule, docket-managed
   dispatch blocks in harness instruction files) and wire it non-nil at the two production call
   sites `service.go:320` and `service.go:431` (currently `nil`); update the
   `legacyNotAdoptedNote` behavior (`internal/app/install.go:150-157`).
3. New golden legacy fixtures + filesystem/adoption tests (no `internal/install/testdata/` exists
   yet) across applicable harness and global-pin shapes; mutating any byte/marker/path/input/kind
   must make adoption refuse.

Design NOT invalidated — scope adjusted only. Spec left unchanged (its "completes existing
development-install and ownership contracts" framing is accurate). No adjacent follow-up work
surfaced that is not already tracked (0316 owns finalize/recovery, 0323 owns uninstall,
0326 owns config contraction). No auto-capture mints.
