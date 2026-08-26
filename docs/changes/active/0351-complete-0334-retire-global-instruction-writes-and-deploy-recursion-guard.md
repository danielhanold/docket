---
id: 351
slug: complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard
title: "Complete change 0334: stop writing global instruction files and actually deploy the recursion guard"
status: 'in-progress'
priority: critical
type: fix
created: 2026-08-26
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [334, 294, 346]
discovered_from: [334]
adrs: []
spec: docs/superpowers/specs/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md
plan: 'docs/superpowers/plans/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding.md'
results:
trivial: false
auto_groomable:
branch: 'fix/complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-26T17:54:01Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md) |
| Plan | [2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding.md) |
<!-- docket:artifacts:end -->

## Why

Change 0334 merged the compact, non-recursive dispatch design, but a live development install did
not deliver its intended state. The Go harness planners still recreate parent-facing instructions
in personal global files, and the already-running executable renders wrappers after building the
new executable. A renderer-only change can therefore install the new binary with old wrappers on
the first run, leaving the recursion guard absent until a second install.

The global-write retirement and fresh-render defect share one installation transaction and remain
one change. Change 0346's stale-source-checkout problem is related but independent and is not folded
into this work.

## What changes

- Make the freshly built development binary the sole planner and mutator for one install invocation;
  the old binary only validates and builds the temporary candidate.
- Stop planning global parent-facing dispatch targets while retaining global skills and agent
  wrappers. Remove prior global blocks or rules only with exact Docket ownership proof; preserve and
  refuse on modified or malformed artifacts.
- Let `docket install` and `docket development install` discover the containing repository or accept
  `--repo-dir`. Reconcile only parent-facing surfaces selected by that repository's explicit
  `agent_harnesses`; no explicit selection means no repository write.
- Journal machine files, safe global cleanup, repository surfaces, and their isolated ownership
  records as one preflighted all-or-nothing operation.
- Verify the compact routing and installed recursion guard in fresh processes for all four harnesses.

## Out of scope

- Changing the compact dispatch rule or recursion-guard wording.
- Change 0346's source-update behavior and change 0349's finalize-resolver cap.
- Per-repository agent wrappers or skills.
- Full repository initialization, metadata-branch setup, Git commits, or legacy repository migration.

## Design decisions

The linked spec fixes the root cause with a fresh-binary handoff, scopes repository authority to an
explicit repo-layer `agent_harnesses`, isolates ownership per working tree, and extends the existing
journal across machine and repository targets. An explicit empty harness list retires unchanged
owned repository surfaces; an absent key touches nothing. Any target conflict aborts the entire run
before mutation, and synchronous failures roll back the complete operation.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-26

### 2026-08-26 — reconcile (docket-implement-next)

Verified against current reality before planning:

- Parent change 0334 is `done` (merged); this change correctly completes its unshipped installer state. `discovered_from: [334]` and `related: [334, 294, 346]` remain accurate (294 is `killed`, 346 is `proposed` and correctly kept independent — not folded in, per the spec's non-goals).
- Confirmed both defects still live in source: all four harness planners (e.g. `internal/harness/claude/claude.go`) still emit the `docket:dispatch` managed-block target under the harness's global `root`, and `internal/install/devmode.go` still has the currently-running binary plan and render the install (no fresh-binary candidate handoff). No `--repo-dir` flag, no repository `agent_harnesses` surface reconciliation, and no per-working-tree `<git-dir>/docket/install.json` ownership record exist yet.
- Scope, out-of-scope, and design decisions in the spec are current; change 0349 (finalize-resolver cap) remains out of scope. No relation, section, or spec edits required — proceeding to plan against the change and spec as written.

## Live repro & priority rationale (2026-08-26)

Bumped to **critical** — this is a live regression, not cleanup. `docket-implement-next` (whether dispatched or run via `/docket-implement-next`) self-dispatches recursively ~3 levels deep until the nested-subagent limit is reached, so no non-trivial change can be built through the dispatch/slash path right now.

Verified mechanism:

- The wrapper Claude Code actually loads in a repo is the **project-level** `.claude/agents/docket-*.md`. On this machine all 17 were the pre-0334 **no-guard** copies (untracked, dated Aug 18), shadowing the guarded user-level `~/.claude/agents/` copies. Without the guard, a running `docket-implement-next` reads the `docket:dispatch` block (in both the global `~/.claude/CLAUDE.md` and the project `CLAUDE.md`) and re-dispatches itself — the recursion.
- Local unblock applied while this change is pending: deleted the stale project-level `.claude/agents/docket-*.md` so the guarded user-level wrappers take effect (to be confirmed in a fresh session).

Open question for implementation: `## Out of scope` excludes per-repository agent wrappers and guard-wording changes, and the spec keeps wrappers global — but the copies that shadow the guard are **project-level** wrappers. Confirm 351's install/cleanup actually removes or updates project-level `.claude/agents/` wrappers (or documents deleting them); otherwise the guard stays shadowed in-repo even after 351 deploys it globally.

