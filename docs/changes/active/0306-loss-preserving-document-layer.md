---
id: 306
slug: loss-preserving-document-layer
title: 'Loss-preserving document layer'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-13
depends_on: [304]
stacked_on:
related: [266]
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-13-loss-preserving-document-layer-design.md
plan: docs/superpowers/plans/2026-08-13-loss-preserving-document-layer.md
results:
trivial: false
auto_groomable:
branch: feat/loss-preserving-document-layer
claimed_at: 2026-08-13T18:03:13Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-loss-preserving-document-layer-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-loss-preserving-document-layer-design.md) |
| Plan | [2026-08-13-loss-preserving-document-layer.md](https://github.com/danielhanold/docket/blob/feat/loss-preserving-document-layer/docs/superpowers/plans/2026-08-13-loss-preserving-document-layer.md) |
<!-- docket:artifacts:end -->

## Why

Go must read existing Markdown records as typed YAML without allowing the first intentional field
or managed-block write to normalize or destroy unrelated source bytes.

## What changes

Implement an immutable source-byte document model, typed semantic YAML decoding, top-level field
location mapping, validate-all loss-preserving field and managed-block patches, canonical
new-document syntax, frozen `v0.9.2` compatibility goldens, and parser/patch fuzz targets.

## Out of scope

Configuration behavior, repository-wide domain policy, Git reads, metadata transactions,
record-specific renderer content, and individual planning or implementation workflow operations.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-08-13 — Reconciled against origin/main after 0304 (Go skeleton) and 0305 (configuration
  envelope) both merged. Everything the spec assumes holds: `go.yaml.in/yaml/v3 v3.0.4` is already
  a direct dependency in `go.mod`; the frozen-fixture tree `testdata/repositories/v0.9.2/` exists
  with the tree-wide `PROVENANCE.md` convention (each new fixture extends it with a line, per
  0305's review fix f132b7af); `go test ./...` is wired into the whole suite via
  `tests/test_go_toolchain.sh` with shared Go caches; `internal/document` does not yet exist. No
  scope adjustment, no drops, no new constraints. Spec left as approved.
