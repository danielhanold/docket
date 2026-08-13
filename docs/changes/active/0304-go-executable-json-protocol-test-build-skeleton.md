---
id: 304
slug: go-executable-json-protocol-test-build-skeleton
title: 'Go executable, JSON protocol, and test/build skeleton'
status: implemented
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-13
depends_on: []
stacked_on:
related: []
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-13-go-executable-json-protocol-test-build-skeleton-design.md
plan: docs/superpowers/plans/2026-08-13-go-executable-json-protocol-test-build-skeleton.md
results: docs/results/2026-08-13-go-executable-json-protocol-test-build-skeleton-results.md
trivial: false
auto_groomable:
branch: feat/go-executable-json-protocol-test-build-skeleton
claimed_at: 2026-08-13T12:43:14Z
pr: https://github.com/danielhanold/docket/pull/204
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-go-executable-json-protocol-test-build-skeleton-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-go-executable-json-protocol-test-build-skeleton-design.md) |
| Plan | [2026-08-13-go-executable-json-protocol-test-build-skeleton.md](https://github.com/danielhanold/docket/blob/feat/go-executable-json-protocol-test-build-skeleton/docs/superpowers/plans/2026-08-13-go-executable-json-protocol-test-build-skeleton.md) |
| Results | [2026-08-13-go-executable-json-protocol-test-build-skeleton-results.md](https://github.com/danielhanold/docket/blob/feat/go-executable-json-protocol-test-build-skeleton/docs/results/2026-08-13-go-executable-json-protocol-test-build-skeleton-results.md) |
| PR | [#204](https://github.com/danielhanold/docket/pull/204) |
<!-- docket:artifacts:end -->

## Why

Every migration slice needs one buildable Go module, executable entry point, stable protocol
envelope, and test convention before independently developed packages can compose safely.

## What changes

Establish the Go 1.26 module and Cobra-based `docket` executable; an application-owned protocol-v1
result envelope and text/JSON presenter; `version` and runtime-diagnostic commands; injectable build
metadata; baseline format, vet, test, and four-target build checks; and fixture-layout conventions.

## Out of scope

Configuration, document/domain/repository behavior, Git or `gh`, metadata mutation, status and
health, installation, harness emission, retained workflows, release packaging, and cutover behavior
owned by changes 0305–0318.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-13

Reconciled against current reality; **no scope change**. Verified:

- `origin/main` carries **no Go source, `go.mod`, `cmd/`, `internal/`, or root `testdata/`** — this
  change is still genuinely greenfield and collides with nothing on the code line.
- The host toolchain is **go1.26.5 darwin/arm64**, exactly the `toolchain go1.26.5` line and one of
  the four approved target tuples the spec names, so the version/diagnostic goldens are testable as
  written.
- **Cobra v1.10.2 resolves** from the module proxy (fetched into the module cache during reconcile
  along with `pflag v1.0.9` and `mousetrap v1.1.0`), so the pin in the spec is live, not aspirational.
- The whole-suite gate is unchanged: `finalize.test_command` still resolves to `scripts/run-tests.sh`,
  which auto-discovers `tests/test_*.sh` and measures each file against `tests/runtime-budgets.tsv` —
  the producer + budget-entry obligation in the spec's *Build and test contract* still lands where
  the spec says it does, and no new CI provider is needed.
- Parent record **0303** (Go migration program record and Bash backlog disposition) is archived
  `done`, and both governing specs — the program map and the architecture design — are present on
  `docket`. The dependent stack (0305–0318) is untouched and still `waiting-on-*`, so the out-of-scope
  boundary in the spec remains accurate.

No spec edits were required and no adjacent follow-up work cleared the auto-capture admission gates
(the two open `docket-status` health findings, on changes 189 and 44, are pre-existing and already
surfaced by their own health checks — not discoveries of this pass).
