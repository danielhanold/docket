---
id: 247
slug: make-shared-metadata-worktree-contention-survivable-and-scop
title: 'Make shared metadata worktree contention survivable and scope its commits'
status: proposed
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-09
depends_on: []
related: [8, 118]
discovered_from: [110, 119]
adrs: []
spec: docs/superpowers/specs/2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0110 and #0119 (2026-08-07 triage): the two halves of the shared-`.docket`-worktree concurrency problem. #0119's own auto-groom abstain said its blocking policy decision "should probably be settled alongside #0110" — a per-session-worktree answer to #0110 would delete the shared-vs-exclusive framing #0119's guard is built on. One design conversation, one change.

Verified 2026-08-07:

- **Contention (#0110, priority high).** `scripts/lib/docket-preflight.sh:70-72` is unchanged: `fetch && pull --rebase || return 1` — no retry, no backoff, no dirty-tree discrimination. `grep -rn "flock\|lockfile\|\.lock" scripts/` returns zero hits. The shared worktree is dirty for the whole multi-tool-call edit→commit window of any agent, so a concurrent agent's preflight hard-fails. Observed live during 0109; interactive sessions racing autonomous loops hit it routinely.
- **Blast radius (#0119).** Two pathspec-less commits inside the shared worktree sweep up another agent's staged work: `scripts/docket-status.sh:282` (`commit_and_push_generated`) and `:846` (refresh-artifacts-links). The scoped precedent exists in the same file (`:880`, `-- "$archived"`). `terminal-publish.sh:327` is pathspec-less but in its own dedicated worktree (safe); `:206` is already scoped.
- **The policy fork #0119 abstained on:** a pathspec-scoped commit exits 128 mid-rebase (e.g. during another agent's preflight rebase), converting a today-succeeding board pass into a hard halt of every autonomous skill — availability vs correctness. The answer depends on #0110's architecture choice.

## What changes

Architecture settled (2026-08-09 auto-groom, critic-gated; full decision trail in the linked spec's `## Assumptions`): collisions become **survivable, not impossible** — the shared `.docket` worktree stays; per-session worktrees and advisory locks are rejected (heavyweight lifecycle machinery ahead of a workload #0008 defers; a lock cannot span tool calls). Recorded as an ADR at build time.

- **Preflight sync** (`scripts/lib/docket-preflight.sh`): fetch first and skip the rebase entirely when the remote has not moved (a dirty tree with nothing to pull never fails); otherwise a bounded discriminating retry (5 attempts, 2/4/8/8s backoff); never `--autostash`; on exhaustion fail closed with a diagnostic distinguishing dirty-tracked-tree vs wedged (mid-rebase/merge) vs ordinary fetch failure. Untracked-only files never count as dirty.
- **Scope the two `docket-status.sh` commits** (`commit_and_push_generated` and the sweep's refresh-artifacts-links pair) with `--` pathspecs, per the #0083 mark-path idiom. Wedged-tree posture: probe for an in-progress rebase/merge before committing and return a new report token `blocked-wedged-tree` (never overload `push-failed`); `--must-land` treats it as not-landed (halt), best-effort callers log and continue. Update `scripts/docket-status.md`'s vocabulary.
- **Shape-keyed guard** `tests/test_shared_worktree_commit_scope.sh`: default-deny on pathspec-less `git … commit` in `scripts/**`, built to #0119's critic-settled requirements (masked quoted strings, per-segment predicate, explicit driver set including `docket-config.sh`'s local `g` wrapper), keyed exception list with existence floor, mutation-tested.

## Out of scope

- The sweep's skip-publish marking question — #0118, separate (collides on adjacent `docket-status.sh` lines; see `related:`).
- Parallel backlog drain design (#0008, deferred feat) — this change only has to make today's interactive+autonomous overlap safe; #0008's revival is the trigger to revisit per-session worktrees.
- The push-side CAS loops (already correct) and feature-branch commit paths (not shared).

## Open questions

None — resolved in the linked spec. The core fork (retry vs per-session worktrees) was committed to retry as the conservative default with the reasoning and the reversal path recorded in the spec's `## Assumptions` block for human audit.
