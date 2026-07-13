---
id: 68
slug: cursor-sandbox-permissions-docs
title: Provide a stable Cursor command boundary for docket workflows
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
The gap is architectural, not cosmetic. Cursor classifies every leaf command in a submitted shell
program before deciding whether the whole program may run outside the sandbox. A branch that never
executes still matters: adding an unreachable, non-allowlisted `exit` to an otherwise allowlisted
`if` program is enough to send the entire command into the sandbox. Docket's Step-0 prose currently
asks agents to assemble config resolution, branching, worktree creation, hook setup, sync, status,
and diagnostics into multiline shell programs. Every new leaf command therefore creates another
permission failure mode.

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

- Add one finite docket command facade, `scripts/docket.sh`. It accepts only documented named
  subcommands and rejects unknown operations; it never evaluates caller-supplied shell text.
- Add a source-only `preflight` operation. Invoked as a standalone terminal call, it resolves and
  exports docket config, enforces the bootstrap verdict, ensures the metadata worktree, disables
  hooks there, and fetches/pulls the metadata branch before returning. The trusted script may use
  internal shell control flow, but agents no longer submit that control flow to Cursor.
- Dispatch existing daily helpers through named facade operations, including status, archive,
  terminal publication, feature-branch cleanup, GitHub mirror, integration sync, checks, and
  renderers. Keep behavior in the existing helpers; the facade is a narrow routing boundary, not a
  second implementation.
- Update operating skills that need exported config to invoke preflight as their own terminal call
  and to use facade operations instead of emitting multiline Step-0 shell programs. `docket-status`
  delegates to its orchestrator directly, and the orchestrator reuses the same preflight
  implementation rather than retaining duplicate worktree-sync logic.
- Publish copyable, valid Cursor configuration that trusts only the canonical and resolved-absolute
  facade invocations. Remove the docket-specific `eval`, shell builtin, per-helper, and broad
  `git -C .docket` entries made unnecessary by the facade. Preserve unrelated user permissions.
- Add a Cursor-focused permissions guide covering `~/.cursor/permissions.json`,
  `~/.cursor/sandbox.json`, reload behavior, and the distinction between command approval,
  filesystem access, and network access.
- Explain Cursor's prefix behavior and why arbitrary `eval`, `bash`, a bootstrap-command prefix,
  or a generic command-runner subcommand is not an acceptable workaround.
- Classify docket's shell scripts by trust level:
  - daily operations exposed through the finite facade, including the guarded archive,
    terminal-publish, feature-branch cleanup, GitHub mirror, and integration-sync flows;
  - one-time machine setup, global harness generation, and repository migration tools that should
    require direct human initiation;
  - internal libraries/tests that the facade must not expose.
- Document the security consequence that allowlisting the status operation trusts its guarded internal
  sweep path, which can archive merged changes, publish terminal records, and clean feature branches.
- Document that the daily lifecycle entries intentionally authorize shared-history writes, external
  GitHub board updates, and provenance-guarded local/remote feature-branch cleanup; they are included
  because these are normal docket operations rather than one-time administration.
- Define the facade's subcommand table as the permission inventory. A test fails when a skill names
  an operation absent from that table or when a new direct runtime helper bypasses the facade.
- Include troubleshooting examples for the observed failure modes: invalid JSON causing the whole
  file to be ignored, a quoted/guarded environment-variable spelling not matching a different
  spelling, an unmatched leaf command demoting a compound program into the sandbox, protected
  `.git` writes, and network fetches that remain blocked by `sandbox.json`.

## Command boundary

The supported top-level forms are deliberately small:

```bash
source "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}/docket.sh" preflight
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}/docket.sh" <named-operation> [operation-arguments...]
```

The first form is source-only because preflight must export resolved variables into the persistent
agent shell. It returns non-zero without exiting the caller's shell when config, bootstrap, worktree,
or sync checks fail. Executing `preflight` as a child process is rejected so a caller cannot
mistakenly continue without the exported environment.

The executable form dispatches only finite named operations. It must not contain `run`, `exec`,
`shell`, or any equivalent escape hatch that accepts an arbitrary command. Existing helper argument
validation and provenance guards remain authoritative.

## Error handling

- Config and bootstrap failures fail closed with the existing actionable diagnostics.
- Worktree creation follows the current state-specific remote/local branch behavior and is
  idempotent.
- Metadata fetch or rebase failure stops preflight before any metadata read or write.
- Unknown facade operations and invalid arguments exit non-zero and list supported operations.
- A failed daily helper preserves its existing exit code and stderr; the facade does not mask it.

## Verification

- Hermetic tests cover docket-mode and main-mode preflight, existing/missing worktrees, hook setup,
  sync failures, bootstrap stop verdicts, and environment export into the sourcing shell.
- Dispatch tests cover every daily operation, argument forwarding, exit-code preservation, and
  rejection of unknown/arbitrary operations.
- Wiring tests assert that operating skills contain no inline Step-0 `if`, worktree, fetch/pull, or
  `eval` programs and invoke only facade operations from the declared inventory.
- `docket-status.sh` tests prove the orchestrator uses shared preflight behavior and does not perform
  a second metadata sync.
- Documentation tests validate the published `permissions.json` fragment as JSON and compare its
  facade entries with the canonical command forms.

## Out of scope

- Automatically editing a user's `~/.cursor/permissions.json` or `~/.cursor/sandbox.json` during
  `install.sh`.
- Blanket approval of every shell file in the docket clone, the `bash` command, or arbitrary `eval`.
- A generic facade operation that executes caller-provided shell text.
- Changing docket script behavior, close-out semantics, or the existing provenance/fail-closed
  guards.
- Defining equivalent permission formats for every supported harness; this change documents Cursor.

## Decisions

- The facade is the trust boundary; individual shell leaves are not allowlisted to support docket.
- Preflight is source-only and always submitted as a standalone terminal call.
- The facade exposes a finite subcommand inventory and has no arbitrary-command escape hatch.
- Cursor guidance lives in a dedicated document linked from the README so the rationale and
  troubleshooting examples do not overwhelm setup instructions.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
