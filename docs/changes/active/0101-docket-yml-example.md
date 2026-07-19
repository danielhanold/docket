---
id: 101
slug: docket-yml-example
title: .docket.yml.example — the canonical all-comprehensive config reference
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [81]
discovered_from: []
adrs: [19, 39]
spec: docs/superpowers/specs/2026-07-19-docket-yml-example-design.md
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
| Spec | [2026-07-19-docket-yml-example-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-docket-yml-example-design.md) |
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0039](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0039-config-example-mirrors-wrapper-defaults.md) |
<!-- docket:artifacts:end -->

## Why

docket's config documentation is scattered across three drifting surfaces — this repo's own
`.docket.yml` (expansive but incomplete), `config.yml.example` (the change-0081 global
starter, with duplicated reclaim/auto_capture prose), and the script contracts. No single
file shows a user every key, its default, its documentation, and which layer (repo,
`.docket.local.yml`, global) may set it. Users have nowhere authoritative to copy from.

## What changes

A new committed `.docket.yml.example` at the repo root becomes the canonical reference,
Helm-values style: every key active at its shipped default with full per-key documentation
and a scope tag (repo-only coordination-fenced vs any-layer). Presence-sensitive keys
(`agents:`, `agent_harnesses:`) ship commented with a loud marker. A new `auto` sentinel
(≡ unset) for `finalize.test_command` and `github_project` lets those defaults ship
explicitly. `config.yml.example` is deleted; `install.sh` scaffolds a minimal pointer-only
global config instead; this repo's `.docket.yml` slims to its set values + pointer; the
README retargets. A new test file enforces example = resolver defaults (fidelity),
key completeness, and the relocated ADR-0039 agents-mirror equality. A new ADR supersedes
ADR-0039. The example's header states the standing rule: every new config flag lands in
this file — value + docs — in the same PR.

## Out of scope

- Codegen of the example from the resolver (manual mirror + tests is the accepted trade-off).
- New config keys or behavior changes beyond the `auto` sentinel.
- Validating harness model IDs in the commented codex/cursor blocks.
- Moving the authoritative scope-classification table out of `scripts/docket-config.md`.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
