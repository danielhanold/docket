---
id: 73
slug: cursor-sandbox-permissions-guide
title: Cursor sandbox & permissions guide — copyable config, trust tiers, troubleshooting
status: proposed
priority: medium
created: 2026-07-13
updated: 2026-07-13
depends_on: [68, 72]
related: [48, 65, 68, 72]
adrs: [20]
spec:
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
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

Cursor users have no docket-owned guidance for running the skills under Auto-run in Sandbox.
With the facade (0068) and the rewired skills (0072) in place, the entire docket runtime surface
is two command shapes — that finally makes a small, copyable, stable permission configuration
possible, and the guide that explains it worth writing.

## What changes

- A Cursor-focused permissions guide: `~/.cursor/permissions.json`, `~/.cursor/sandbox.json`,
  reload behavior, and the distinction between command approval, filesystem access, and network
  access (the sandbox filesystem config is not a substitute for terminal permissions).
- A copyable, JSON-validated permission fragment trusting only the canonical and
  resolved-absolute facade invocations; documentation tests compare it against the canonical
  command forms.
- The trust-tier classification of docket's shell surface: daily operations behind the facade;
  one-time machine setup / harness generation / migration tools requiring direct human
  initiation; internal libraries and tests the facade must not expose.
- The security consequences, stated plainly: allowlisting `docket-status` trusts its guarded
  sweep (archives merged changes, publishes terminal records, cleans feature branches); the
  daily-lifecycle entries intentionally authorize shared-history writes, external GitHub board
  updates, and provenance-guarded branch cleanup.
- Why arbitrary `eval`, `bash`, a bootstrap-command prefix, or a generic command-runner
  subcommand is not an acceptable workaround for prefix/leaf classification.
- Troubleshooting the observed failure modes — invalid JSON silently disabling the whole file,
  spelling mismatches in guarded expansions, one unmatched leaf demoting a compound program,
  protected `.git` writes, network fetches still blocked by `sandbox.json` — each recorded
  **with provenance** (Cursor version + observation date), since the guide's claims rest on
  observed classifier behavior.
- Scope statement: the facade stabilizes docket's own metadata/lifecycle operations;
  build-time commands (feature-branch git, test suites, `gh`) remain the consuming repo's
  permission surface and are documented as such, not hidden.

## Out of scope

- Automatically editing a user's Cursor configuration during `install.sh` (ADR-0020 posture:
  generated agent artifacts stay machine-local and human-applied).
- Equivalent permission formats for every supported harness; this change documents Cursor.
- Any change to scripts or skills (0068/0072 own those).

## Open questions

- Pin down each classifier observation with Cursor version + session date before publishing.
- Whether the fragment should also cover the read-only sandbox path for the docket clone.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
