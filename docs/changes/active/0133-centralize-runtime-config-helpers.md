---
id: 133
slug: centralize-runtime-config-helpers
title: Centralize shared Bash runtime configuration helpers
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: [18, 132]
discovered_from: [132]
adrs: [14, 19, 29]
spec: docs/superpowers/specs/2026-07-22-centralize-runtime-config-helpers-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: refactor
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-22-centralize-runtime-config-helpers-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-22-centralize-runtime-config-helpers-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0132 established a machine-local GNU Bash 4+ runtime, but its parser and validator helpers
now have parallel implementations in install, bootstrap, and config resolution paths. A correction
to one copy can silently leave another path with different configuration behavior.

## What changes

- Add one bootstrap-compatible shared runtime-helper library for the duplicated
  `runtime.bash` parser, declaration counter, serializability check, and GNU Bash 4+ validator.
- Route `install.sh`, `ensure-global-config.sh`, and `docket-config.sh` through that library while
  preserving their current authority, discovery, marker, precedence, and diagnostic policies.
- Add focused helper and caller-level regression coverage, including a mutation check, without
  relaxing the required configured runtime.

## Out of scope

- General YAML-parser adoption, runtime discovery-order changes, or changes to post-install Bash
  4+ enforcement.

## Open questions

None; the shared-mechanics boundary and bootstrap-compatibility requirement are settled in the
linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
