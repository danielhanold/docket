---
id: 68
slug: cursor-sandbox-permissions-docs
title: Document safe Cursor sandbox permissions for docket workflows
status: proposed
priority: medium
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [48, 63, 65]
adrs: [20, 25, 27]
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
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0025](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0025-docket-worktrees-disable-git-hooks.md), [ADR-0027](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0027-terminal-publish-repo-scoped-script-gated.md) |
<!-- docket:artifacts:end -->

## Why

Cursor users have no docket-owned guidance for running the skills under Auto-run in Sandbox.
The gap is operational, not cosmetic: docket's documented command spelling uses
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<script>.sh`, while a terminal allowlist entry
must match the command spelling Cursor actually classifies. A partial string ending inside the
`${DOCKET_SCRIPTS_DIR:?...}` token does not behave like a safe directory prefix, and broad entries
such as `eval`, `bash`, or a blanket script-directory wildcard grant substantially more authority
than a routine docket pass needs.

An audit of the repository's shell surface found 57 `.sh` files across root setup/migration tools,
runtime helpers, libraries, and tests. They do not share one risk profile. Read-only checks and
deterministic renderers can reasonably auto-run; installer/configuration scripts write outside the
repo; migration and terminal-close-out helpers commit and push shared history; cleanup deletes local
and remote branches; GitHub mirroring writes external issues/projects; and tests intentionally run
destructive fixtures in temporary repositories. Treating “under `DOCKET_SCRIPTS_DIR`” as the sole
trust boundary obscures those differences.

The sandbox filesystem configuration is also not a substitute for terminal permissions. Adding the
docket clone as a read-only path lets an agent read helper scripts, but docket workflows still write
git refs/config, metadata worktrees, boards, and sometimes remote state. Network allowlists and
terminal approval are separate gates and need separate troubleshooting guidance.

## What changes

- Add a Cursor-focused permissions section to the README or a dedicated linked guide, covering
  `~/.cursor/permissions.json`, `~/.cursor/sandbox.json`, reload behavior, and the distinction
  between command approval, filesystem access, and network access.
- Publish copyable, valid JSON for the exact config-export bootstrap and the canonical routine
  helper spellings used by docket skills. Explain Cursor's command/token-boundary prefix behavior
  and why an arbitrary `eval` or `bash` permission is not an acceptable workaround.
- Include both docket's canonical `${DOCKET_SCRIPTS_DIR:?...}` command strings and resolved
  absolute-path equivalents. Cursor matches the command text submitted to the terminal without
  expanding environment variables for the allowlist, and agents may submit either spelling.
- Classify docket's shell scripts by trust level:
  - daily workflow entry points suitable for explicit allowlisting, including the guarded archive,
    terminal-publish, feature-branch cleanup, GitHub mirror, and integration-sync helpers used by
    normal status/finalize/ADR operations;
  - one-time machine setup, global harness generation, and repository migration tools that should
    require direct human initiation;
  - internal libraries/tests that should not appear in a runtime allowlist.
- Document the security consequence that allowlisting `docket-status.sh` trusts its guarded internal
  sweep path, which can archive merged changes, publish terminal records, and clean feature branches.
- Document that the daily lifecycle entries intentionally authorize shared-history writes, external
  GitHub board updates, and provenance-guarded local/remote feature-branch cleanup; they are included
  because these are normal docket operations rather than one-time administration.
- Add a lightweight drift check or documented inventory source so new shell entry points cannot be
  added without reconsidering their Cursor permission classification.
- Include troubleshooting examples for the observed failure modes: invalid JSON causing the whole
  file to be ignored, a quoted/guarded environment-variable spelling not matching a different
  spelling, a missing leaf command in a compound diagnostic, protected `.git` writes, and network
  fetches that remain blocked by `sandbox.json`.

## Out of scope

- Automatically editing a user's `~/.cursor/permissions.json` or `~/.cursor/sandbox.json` during
  `install.sh`.
- Blanket approval of every shell file in the docket clone, the `bash` command, or arbitrary `eval`.
- Changing docket script behavior, close-out semantics, or the existing provenance/fail-closed
  guards.
- Defining equivalent permission formats for every supported harness; this change documents Cursor.

## Open questions

- Is the best home a concise README subsection with a dedicated `docs/` reference, or a single
  self-contained README section?
- Should a test derive the documented script inventory from skill call sites and fail when a new
  direct entry point has no permission classification?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
