---
id: 304
slug: go-executable-json-protocol-test-build-skeleton
title: 'Go executable, JSON protocol, and test/build skeleton'
status: proposed
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-go-executable-json-protocol-test-build-skeleton-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-go-executable-json-protocol-test-build-skeleton-design.md) |
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
