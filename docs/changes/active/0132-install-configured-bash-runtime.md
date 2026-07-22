---
id: 132
slug: install-configured-bash-runtime
title: Install and use a configured Bash 4+ runtime
status: in-progress
priority: high
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: [34, 68, 128, 131]
discovered_from: [128]
adrs: [14, 19, 29]
spec: docs/superpowers/specs/2026-07-22-install-configured-bash-runtime-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/install-configured-bash-runtime
pr:
blocked_by:
reconciled: false
claimed_at: 2026-07-22T15:48:57Z
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-22-install-configured-bash-runtime-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-22-install-configured-bash-runtime-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md) |
<!-- docket:artifacts:end -->

## Why

The local finalize gate can accidentally select macOS's legacy `/bin/bash` 3.2 even when a modern
Bash is installed, turning a required Bash-4 validator into a false code failure. Docket needs one
explicit, validated interpreter path that installation discovers and all Docket-owned execution
uses.

## What changes

- Discover a compatible Bash 4+ during installation, persist it in machine-local global config,
  and export it as `DOCKET_BASH_PATH`.
- Make installation and preflight fail closed with upgrade instructions when no compatible Bash is
  available.
- Route Docket helpers and auto-detected shell tests through the configured interpreter without
  changing the meaning of repository-specific `finalize.test_command`.

## Out of scope

- Rewriting arbitrary repository test commands or scripts that explicitly require `/bin/bash`.
- The noninteractive board-conflict fixture stall tracked separately by change 0131.

## Open questions

None; the installation, configuration, and execution-boundary decisions are settled in the linked
spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
