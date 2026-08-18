---
id: 322
slug: go-installer-adopt-legacy-bash-installed-user-level-artifact
title: 'Bootstrap Go development installation and adopt legacy user-level artifacts'
status: in-progress
priority: critical
type: feat
created: 2026-08-14
updated: 2026-08-18
claimed_at: 2026-08-18T15:51:30Z
depends_on: [311]
stacked_on:
related: [316, 326]
discovered_from: [311]
adrs: []
spec: docs/superpowers/specs/2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/go-installer-adopt-legacy-bash-installed-user-level-artifact
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md) |
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
