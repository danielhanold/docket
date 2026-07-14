---
id: 73
slug: cursor-sandbox-permissions-guide
title: Cursor sandbox & permissions guide — copyable config, trust tiers, troubleshooting
status: done
priority: medium
created: 2026-07-13
updated: 2026-07-14
depends_on: [68, 72]
related: [48, 65, 68, 72]
adrs: [20, 33]
spec: docs/superpowers/specs/2026-07-14-cursor-sandbox-permissions-guide-design.md
plan: docs/superpowers/plans/2026-07-14-cursor-sandbox-permissions-guide.md
results: docs/results/2026-07-14-cursor-sandbox-permissions-guide-results.md
trivial: false
auto_groomable:
branch: feat/cursor-sandbox-permissions-guide
pr: https://github.com/danielhanold/docket/pull/83
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-14-cursor-sandbox-permissions-guide-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-14-cursor-sandbox-permissions-guide-design.md) |
| Plan | [2026-07-14-cursor-sandbox-permissions-guide.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-14-cursor-sandbox-permissions-guide.md) |
| Results | [2026-07-14-cursor-sandbox-permissions-guide-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-14-cursor-sandbox-permissions-guide-results.md) |
| PR | [#83](https://github.com/danielhanold/docket/pull/83) |
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md) |
<!-- docket:artifacts:end -->

## Why

Cursor users have no docket-owned guidance for running the skills under Auto-run in Sandbox.
With the facade (0068) and the rewired skills (0072) in place, the entire docket runtime surface
is two command shapes — that finally makes a small, copyable, stable permission configuration
possible, and the guide that explains it worth writing.

## What changes

Three published artifacts plus a README pointer — user-facing docs, built on a feature branch like
any code. Design detail lives in the spec.

- **`docs/cursor/permissions.md`** — the guide: the three independent gates (command approval,
  filesystem, network) and reload behavior; why docket's operations must be allowlisted to run
  **outside** the sandbox (an out-of-workspace program plus network), so no `sandbox.json` entry
  can substitute for a terminal permission; the trust tiers; the security consequences; why the
  `eval`/`bash`/generic-runner workarounds are not acceptable; and a scope statement that a
  consuming repo's build-time commands (feature-branch git, test suites, `gh`) remain that repo's
  own permission surface.
- **`docs/cursor/permissions.example.json`** — the copyable fragment. It allowlists the facade
  `docket.sh` spellings observed in the verification log: canonical
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh`, both short `:?}` quote styles, and the
  resolved-absolute path. The facade is the trust boundary; per-operation helper entries were
  rejected.
- **`docs/cursor/sandbox.example.json`** — the copyable sandbox fragment granting a read path to
  the docket clone, which lives outside the workspace.
- **Trust tiers**, derived from `scripts/docket.md` (already the declared permission inventory),
  never hand-copied: daily operations behind the facade; the human-initiated setup/migration tier
  (`install.sh`, `migrate-to-docket.sh`, `sync-agents.sh`, `ensure-*.sh`) that is never
  allowlisted; and the internals the facade must not expose.
- **The security consequences, stated plainly**: the one allowlist entry authorizes, unprompted,
  `docket-status`'s guarded sweep (archives changes, publishes terminal records onto the
  integration branch, deletes merged feature branches and worktrees), `terminal-publish`'s direct
  push, `github-mirror`'s external GitHub writes, and `cleanup-feature-branch`'s deletions.
- **Troubleshooting with provenance** — each observed failure mode stamped with the Cursor version
  and observation date, transcribed from the spec's verification-log appendix. Nothing is asserted
  that the appendix does not record; a mode that does not reproduce is cut, not softened.
- **An ADR** recording why trust is granted at the facade rather than per operation.

## Build precondition — live Cursor verification

**Fulfilled 2026-07-14.** The spec's `## Appendix — verification log` records the live Cursor
3.11.19 session (Allowlist with Sandbox; four facade spellings including short `:?}` forms;
autoRun-only contrast; demotion / invalid-JSON / mode-lock; compound retest). Reconcile must still
abort if that appendix is removed or emptied.

## Out of scope

- Automatically editing a user's Cursor configuration during `install.sh` (ADR-0020 posture:
  generated agent artifacts stay machine-local and human-applied).
- Equivalent permission formats for every supported harness; this change documents Cursor.
- Any change to scripts or skills (0068/0072 own those).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-14 — reconcile (build claim)

Verified the design is true against current `origin/main` and `docket`; no scope change, no
obsolescence, no fundamental invalidation. Body and spec left as-is.

- **Build precondition (fail-closed backstop): PASS.** The spec's `## Appendix — verification log`
  exists and is richly non-empty (Cursor 3.11.19 / 2026-07-14 session: Run-Mode lock table, command
  forms B1–B7, reproduced/cut failure modes, reload behavior, and the §G short-`:?}` retest). The
  reconcile abort condition (missing/empty appendix) does not trip.
- **Trust-tier source unchanged.** `scripts/docket.md` still carries the Subcommand inventory (the
  declared permission inventory — 14 ops incl. the 3 verbs) and the Not-exposed section (internals
  `docket-config.sh`/`disable-worktree-hooks.sh`/`render-board.sh` plus the human-initiated tier
  `install.sh`/`migrate-to-docket.sh`/`sync-agents.sh`/`ensure-docket-env.sh`/`ensure-claude-settings.sh`).
  The guide's trust tiers and test assertion 5 derive from these sections at build time — still valid.
- **Canonical-spelling sentinel present.** `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh`
  is enforced in the convention/tests, so the fragment's canonical form can be derived, not retyped.
- **Cited ADRs current:** ADR-0020, ADR-0027, ADR-0029 all `Accepted`; titles match the spec's use.
- **No collision:** `docs/cursor/` does not exist on `origin/main`; this change creates it fresh.
- **Deps satisfied:** 0068 (facade) and 0072 (skill rewiring) are both `done` (archived).
- **Suite shape:** tests are hermetic per-file `tests/test_*.sh` (no top-level runner); the new
  guard file is `tests/test_cursor_permissions_docs.sh`.
