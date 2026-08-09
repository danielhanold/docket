---
id: 247
slug: make-shared-metadata-worktree-contention-survivable-and-scop
title: 'Make shared metadata worktree contention survivable and scope its commits'
status: proposed
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [110, 119]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

Consolidates #0110 and #0119 (2026-08-07 triage): the two halves of the shared-`.docket`-worktree concurrency problem. #0119's own auto-groom abstain said its blocking policy decision "should probably be settled alongside #0110" — a per-session-worktree answer to #0110 would delete the shared-vs-exclusive framing #0119's guard is built on. One design conversation, one change.

Verified 2026-08-07:

- **Contention (#0110, priority high).** `scripts/lib/docket-preflight.sh:70-72` is unchanged: `fetch && pull --rebase || return 1` — no retry, no backoff, no dirty-tree discrimination. `grep -rn "flock\|lockfile\|\.lock" scripts/` returns zero hits. The shared worktree is dirty for the whole multi-tool-call edit→commit window of any agent, so a concurrent agent's preflight hard-fails. Observed live during 0109; interactive sessions racing autonomous loops hit it routinely.
- **Blast radius (#0119).** Two pathspec-less commits inside the shared worktree sweep up another agent's staged work: `scripts/docket-status.sh:282` (`commit_and_push_generated`) and `:846` (refresh-artifacts-links). The scoped precedent exists in the same file (`:880`, `-- "$archived"`). `terminal-publish.sh:327` is pathspec-less but in its own dedicated worktree (safe); `:206` is already scoped.
- **The policy fork #0119 abstained on:** a pathspec-scoped commit exits 128 mid-rebase (e.g. during another agent's preflight rebase), converting a today-succeeding board pass into a hard halt of every autonomous skill — availability vs correctness. The answer depends on #0110's architecture choice.

## What changes

Settle the architecture in one brainstorm, then implement both halves consistently:

- Either make collisions **survivable** (bounded retry/backoff in preflight sync; dirty-tree discrimination; possibly an advisory lock) or **impossible** (per-session metadata worktrees with a lease/prune story) — the #0110 fork.
- Scope the two `docket-status.sh` commit calls to the paths they own, with the failure posture the architecture decision implies, plus a shape-keyed guard (no new pathspec-less `git commit` in the shared worktree).

## Out of scope

- The sweep's skip-publish marking question — #0118, separate.
- Parallel backlog drain design (#0008, deferred feat) — this change only has to make today's interactive+autonomous overlap safe.

## Open questions

- Retry vs per-session worktrees (the core fork — needs Daniel).
- If retry: how many attempts, and does the CAS loop distinguish "remote moved" from "local dirty".
