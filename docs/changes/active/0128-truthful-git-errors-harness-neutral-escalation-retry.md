---
id: 128
slug: truthful-git-errors-harness-neutral-escalation-retry
title: Truthful Git failures and harness-neutral sandbox escalation retry
status: implemented
priority: high
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: [68, 72, 73, 79]
discovered_from: [127]
adrs: [29, 33, 37, 38]
spec: docs/superpowers/specs/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-design.md
plan: docs/superpowers/plans/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md
results: docs/results/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-results.md
trivial: false
auto_groomable:
branch: feat/truthful-git-errors-harness-neutral-escalation-retry
claimed_at: 2026-07-22T14:11:07Z
pr: https://github.com/danielhanold/docket/pull/121
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-design.md) |
| Plan | [2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md](https://github.com/danielhanold/docket/blob/feat/truthful-git-errors-harness-neutral-escalation-retry/docs/superpowers/plans/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md) |
| Results | [2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-results.md](https://github.com/danielhanold/docket/blob/feat/truthful-git-errors-harness-neutral-escalation-retry/docs/results/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-results.md) |
| PR | [#121](https://github.com/danielhanold/docket/pull/121) |
| ADRs | [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0037](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0037-runner-delegation-explicit-runner-field.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md) |
<!-- docket:artifacts:end -->

## Why

A Codex-hosted `docket-implement-next 127` run halted at Step 0 even though the configured network
access and the GitHub SSH connection both worked. Codex's `workspace-write` sandbox protected
`.git` as read-only, so `git fetch` could not create
`.git/refs/remotes/origin/HEAD.lock`. `docket-config.sh` discarded that stderr and translated every
fetch failure into “cannot reach origin,” causing the implementer to classify a recoverable
harness-approval boundary as a hard network error. Nothing was claimed or built.

This is two defects at different boundaries: the deterministic resolver reports a cause it did
not establish, and the agent workflow does not define the portable recovery step for a sandbox
denial. Together they turn ordinary least-privilege Codex operation into a false `halted`
disposition before every change can begin.

## What changes

- Make `docket-config.sh` capture and preserve `git fetch` stderr, emit a neutral deterministic
  fetch-failure wrapper, and stop diagnosing all nonzero fetches as network failures.
- Add a harness-neutral convention for Git operations rejected by a sandbox or permission
  boundary: retry the exact command once through the host harness's native approval/escalation
  mechanism, then apply the existing failure posture if approval is unavailable, denied, or the
  retry fails.
- Apply that convention to Step-0 preflight and later Docket Git operations without teaching shell
  scripts to elevate themselves, using `sudo`, changing command arguments, or broadly disabling a
  sandbox.
- Pin the deterministic diagnostic and the workflow rule with hermetic, mutation-tested guards;
  update the config contract so it distinguishes a fetch failure from a proven network failure.

## Out of scope

- Automatically approving an escalation, changing a user's Codex/Claude/Cursor security settings,
  or switching a session to full-access mode.
- Retrying authentication, connectivity, repository-state, merge-conflict, or other ordinary Git
  failures as permission problems.
- Adding a shell-level privilege escalation mechanism; only the hosting harness can cross its own
  sandbox boundary.
- Redesigning the Docket facade or its finite command inventory.

## Open questions

None. The deterministic script fix and the harness-owned single-retry boundary are settled in the
linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-22 — reconcile (docket-implement-next)

Verified the settled design against `origin/main`, the current facade/config resolver and its
hermetic test seams, related completed changes #0068, #0072, #0073, and #0079, cited ADRs
0029/0033/0037/0038, and the newest archived guard work. The incident remains reproducible in a
least-privilege Codex session: the exact canonical preflight succeeds when retried through the
host approval boundary, while the current resolver still suppresses the permission diagnostic and
mislabels it as a network failure.

- **Scope remains valid.** The facade is still the sole executable Docket boundary; no runner,
  facade-inventory, or shell-elevation redesign is needed.
- **Implementation seam is current.** Stage 1 of `docket-config.sh` is the single fetch/error
  site, `tests/test_docket_config.sh` provides the resolver guard surface, and the convention is
  inherited by every operating skill.
- **No new ADR.** The implementation applies the existing facade and harness-owned approval
  boundaries recorded by ADR-0029 and ADR-0033; the runner ADRs are relevant context only.
- **No follow-up stub surfaced.** The guarded, one-command retry rule fully covers the discovered
  harness boundary without expanding the change's scope.
