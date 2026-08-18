---
id: 326
slug: pre-go-mutation-configuration-contraction
title: 'Pre-Go mutation configuration contraction'
status: in-progress
priority: critical
type: chore
created: 2026-08-18
updated: 2026-08-18
claimed_at: 2026-08-18T19:37:01Z
depends_on: [315]
stacked_on:
related: [316, 318, 322]
discovered_from: [316]
adrs: []
spec: docs/superpowers/specs/2026-08-18-pre-go-mutation-configuration-contraction-design.md
plan: docs/superpowers/plans/2026-08-18-pre-go-mutation-configuration-contraction.md
results:
trivial: false
auto_groomable:
branch: feat/pre-go-mutation-configuration-contraction
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-18-pre-go-mutation-configuration-contraction-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-18-pre-go-mutation-configuration-contraction-design.md) |
| Plan | [2026-08-18-pre-go-mutation-configuration-contraction.md](https://github.com/danielhanold/docket/blob/feat/pre-go-mutation-configuration-contraction/docs/superpowers/plans/2026-08-18-pre-go-mutation-configuration-contraction.md) |
<!-- docket:artifacts:end -->

## Why

Docket's repository currently opts into deferred capabilities that Go v1 correctly refuses before
mutation. Leaving their contraction until the final self-hosting change makes the first Go-managed
implementation run impossible and creates a circular dependency at change 0316.

## What changes

Turn off the three active deferred capabilities in committed `.docket.yml`, remove the
repository-local agent-routing request, and turn off repository-local automatic capture on the
migration host. Verify the full four-layer resolved configuration permits Go mutation while
preserving global model/effort overrides and the fail-closed capability policy.

## Out of scope

Changing Go's configuration schema or capability classifier, modifying global agent pins,
installing the Go binary, adopting legacy harness artifacts, implementing any lifecycle operation,
removing Bash, publishing a release, or claiming that this configuration check alone proves
self-hosting.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-18 — reconcile against HEAD, implemented via v0.9.2 Bash bridge

Verified current reality. Change is valid and unchanged in scope; the Go mutation fence is still
active (0322 merged its installer/adoption but not the config contraction — that is this change).
Dependency 0315 is `done`; sibling 0322 is now `done` (installation is its job, not this change's).

Confirmed config state to contract:
- Committed `.docket.yml`: `finalize.skip_results_only_delta: true` (~L27), `terminal_publish: true`
  (~L33), `build.checkpoint: true` (~L40) — the three deferred switches to set explicitly `false`.
- Machine-local `.docket.local.yml` (gitignored — confirmed, will not ride the PR): carries
  `auto_capture.enabled: true` and a repository-local `agents:` block (claude + codex pins). Both are
  the repo-layer requests Go v1 defers; the operator turns off auto_capture and drops the repo-local
  `agents:` block on the migration host, recorded as redacted evidence (no private values in-repo).
- Global `~/.config/docket/config.yml` exists and carries its own `agents:` pins + `agent_harnesses` —
  the supported Go-v1 machine-wide override layer; left untouched (spec's corrected diagnosis: only
  repository/repository-local agent pins block, not global).

Confirmed infrastructure already present (this change writes fixtures, not new machinery):
- `docket diagnostic config --repo-dir <dir> --for-mutation --json` and `MutationAllowed`
  (`internal/app/config.go`, `internal/cli`), with existing config tests (`internal/app/config_test.go`
  incl. `TestPreflightUnsupported`). This change must NOT modify `internal/config` or reclassify any
  capability (spec exclusions).

Net build scope: (1) the tracked `.docket.yml` edit — three switches to explicit `false`, comments
preserved, no reordering; (2) Go config fixtures proving the pre-change four-layer state reproduces
the classifier's blockers, the post-change state (global pins kept, repo pins removed, auto_capture
off) reports `MutationAllowed: true`, and one-at-a-time negative fixtures still fail closed. The
machine-local edit + the real `diagnostic config --for-mutation` run are operator/verification steps,
recorded redacted in results. No auto-capture mints; no adjacent follow-up surfaced.
