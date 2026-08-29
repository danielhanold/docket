---
id: 318
slug: config-contraction-self-hosting-and-hard-cutover
title: 'Go-only source cutover'
status: 'in-progress'
priority: critical
type: refactor
created: 2026-08-12
updated: '2026-08-29'
depends_on: [317, 352, 363]
stacked_on:
related: [322, 326, 361, 366]
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'refactor/config-contraction-self-hosting-and-hard-cutover'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-29T16:48:15Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket's Go lifecycle is complete, but maintained workflows, configuration, tests, and operator
documentation still route through the production Bash facade and its helper/runtime tree. That
dual implementation prevents the reviewed Go source from becoming the one authoritative product
and keeps the repository's canonical suite tied to the implementation being retired.

## What changes

- Derive a whole-repository inventory of legacy executable paths, retained product invariants,
  dependent guards, immutable history, and the two approved POSIX product surfaces.
- Rewrite maintained skills, agents, generated dispatch material, workflows, setup checks,
  configuration, and operator instructions to use the PATH-resolved Go CLI and JSON contracts.
- Remove the production Bash facade, helper/runtime tree, compatibility paths, environment
  bridges, legacy-runtime lifecycle dependencies, and mechanism-only tests in the same PR.
- Preserve every surviving product invariant with mutation-sensitive Go coverage or true `/bin/sh`
  coverage owned by repository-root `install.sh` and the release downloader.
- Add `docket development test` as the Go-native whole-suite implementation, entered from source
  through a branch-faithful Go bootstrap and used by contributors, finalization, and the
  release-candidate source gate.
- Replace active documentation with the Go-only model and identify `v1.0.0-rc1` as the upcoming
  first public Go candidate without asserting that publication already exists.

## Out of scope

Whole-backlog migration-ledger dispositions, manual migration learning records, post-merge
candidate packaging, native target smokes, genuinely fresh Claude/Codex/Cursor/OpenCode lifecycle
proofs, `v0.9.2` rollback rehearsal, tag or GitHub Release creation, public-install verification,
and final release evidence or metadata closeout. Those human-attended gates belong to change 0366,
which depends on this source cutover. Also excluded: stable `v1.0.0`, Homebrew, Windows,
signing/notarization, SBOM or provenance signing, uninstall, version-tree garbage collection,
unrelated capability changes, and rewrites of historical or frozen records.

## Design decisions

Change 0318 remains one reviewable code PR and keeps its existing id, slug, recorded branch, and
claim continuity. The source gate tests the exact checkout under review rather than a stale
installed binary. Historical records and frozen `v0.9.2` fixtures are not rewritten; an active
baseline change creates a new versioned fixture with provenance. Generator behavior may be proved
inside this PR, while external fresh-process and public-release truth is reserved for 0366.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Run halted

### 2026-08-29

**Disposition: `halted` — the revised (narrowed) scope still exceeds a single autonomous `docket-implement-next` run.** No plan was written and no code was built; the change is left `in-progress`, `reconciled: false`, with its claim lease refreshed. A human is needed to decide how to sequence the work.

This is not a fundamentally-invalidated design — the revised spec (`docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md`) is sound and its split of human-attended release/self-host work into change 0366 is correct. The blocker is *magnitude and atomicity*, exactly the judgment the dispatch charter asked me to surface rather than build a partial slice around.

### Why it does not fit one run

**1. Whole-repository deletion surface.** The production Bash facade is 36 top-level `scripts/*.sh` totalling 12,339 lines (49 shell scripts across the subtree). The spec requires removing it *together with every consumer* in the same PR.

**2. Whole-repository consumer rewrite.** `DOCKET_SCRIPTS_DIR` is referenced by 141 tracked files (94 under `docs/`, 19 under `internal/`, 10 skills, 9 tests, plus agents/workflows/setup); `runtime.bash` by 58. All 14 skills that invoke `docket.sh` and the 17 native agent definitions, plus every generated dispatch block and workflow, must be rewritten to the PATH-resolved Go CLI and regenerated through their generators.

**3. A brand-new high-risk Go subsystem.** `docket development test` does not exist — only `docket development install` does. The spec requires building it as the *sole* whole-suite implementation: a Go reimplementation of the 819-line `scripts/run-tests.sh` with source-copy fidelity (build-and-enter the checkout via `go run ./cmd/docket development test`), per-target isolation, safe parallelism, one identity-matched result per target, interruption/cancellation handling, deterministic aggregation, and the screen-then-serial-confirm wall-clock budget protocol — all preserved semantically, not textually.

**4. Invariant preservation with new mutation-tested coverage.** Every surviving invariant currently guarded by shell tests (36 Go test files alone reference `.sh`; the retained POSIX suites for `install.sh` and the release downloader must run under `/bin/sh` without the removed runtime) must be re-covered by mutation-sensitive Go tests or explicitly retained POSIX tests, each proven to redden when its premise is stripped.

**5. Documentation rewrite.** All active contributor, install, release, troubleshooting, agent, and setup docs must be rewritten to the Go-only model with the two POSIX exceptions.

**6. Atomicity forbids slicing.** Removal and consumer-rewrite must land in one reviewable, independently-mergeable PR (spec: "in the same PR", acceptance criterion 22). A partial slice would leave the repository non-functional between steps, so the run cannot honestly ship an intermediate PR.

**7. The final suite gate runs through the very orchestrator being invented.** `finalize.test_command` is retargeted from `scripts/run-tests.sh` to `docket development test` mid-change, so the closing green-suite gate depends on the untested new runner correctly orchestrating 661 Go files plus the POSIX suites — over a suite with known wall-clock budget fragility (project memory: `internal/app` has previously blown Go's 600s per-package timeout under parallel load). Producing a reliably green run here in one autonomous forked pass is not within the implement-next reliability envelope (dispatched plan-writer + per-task build workers + one suite gate).

### Recommended human action

Sequence 0318 as a small stack of dependent changes that can each ship a green, reviewable PR, for example: (a) build and land `docket development test` (the Go runner + source-copy bootstrap + budget/isolation semantics) with `finalize.test_command` cut over, tests still green over the *existing* facade; (b) migrate skills/agents/generated dispatch/workflows/docs to the PATH-resolved Go CLI while the facade still exists as a thin shim; (c) delete the facade, helper/runtime tree, and mechanism-only tests, replacing surviving invariants with mutation-tested Go/POSIX coverage. Each step is individually autonomous-runnable; the current single-PR framing is not. If the single-PR framing is a hard requirement, this needs a human-supervised multi-session build rather than an autonomous implement-next drain.

(Dated 2026-08-29. Counts above are point-in-time snapshots of the reconciled base, not acceptance authority — the spec's own inventory step re-derives them.)
