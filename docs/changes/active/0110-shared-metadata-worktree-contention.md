---
id: 110
slug: shared-metadata-worktree-contention
title: Concurrent agents collide on the shared .docket worktree's dirty-tree window
status: proposed
priority: high
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: [8]
discovered_from: [109]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`docket.sh preflight` fails outright when another agent is mid-write in the shared `.docket`
metadata worktree:

```
error: cannot pull with rebase: You have unstaged changes.
docket-preflight: metadata worktree sync failed
```

This has now happened more than once and is reproducible by construction, not bad luck.

Git commits are atomic, but the **edit-to-commit window is not**. Every docket writer runs its
sequence across separate tool calls — write the file, run `render-change-links.sh`, `git add`,
`git commit`, `git push` — and the worktree is dirty for that entire span. That is not
microseconds: it is however long the agent spends thinking, rendering, and composing a commit
message. Seconds to minutes.

`scripts/lib/docket-preflight.sh:66` hard-fails on it:

```sh
"$git" -C "$wt" fetch origin "$METADATA_BRANCH" \
  && "$git" -C "$wt" pull --rebase origin "$METADATA_BRANCH" \
  || { echo "docket-preflight: metadata worktree sync failed"; return 1; }
```

`pull --rebase` refuses to run against unstaged changes to tracked files, so one agent's perfectly
normal in-flight write becomes another agent's hard failure. The root cause is that git's
atomicity guarantees cover the object database and ref updates, **not the working tree** —
and `.docket` is a single checkout shared by every concurrent agent, so they share one dirty-state
surface. There is no lock anywhere in `scripts/` (no `flock`, no lockfile).

Worth noting the design already handles the race it anticipated: `archive-change.sh`,
`docket-status.sh`, and `mint-stub.sh` all run pull-rebase-retry CAS loops on **push**. The gap is
one layer earlier — working-tree contention *before* a commit exists to race with.

Today the failure is survivable by hand (retry and hope the other agent committed), which is
exactly why it deserves a fix: an autonomous run has no human to retry it, and abort-and-report
turns a transient collision into a dead loop.

## What changes

To be settled in brainstorm. Candidate directions, none yet chosen:

- **Bounded retry with backoff in preflight** instead of a hard fail. Cheap, treats the symptom.
  Note that `git pull --rebase --autostash` is *actively dangerous* here: on a shared tree it would
  stash another agent's in-flight edits.
- **Per-session metadata worktrees** (e.g. `.docket-<session>`), eliminating sharing entirely.
  Costs more checkouts and needs a pruning story.
- **Shrink the dirty window** — direct skills to write-and-commit within a single call so the tree
  is clean between tool calls. Narrows the race without closing it.
- **An advisory lock** around the write→commit→push critical section. The textbook answer, but it
  fits badly: the critical section spans multiple tool calls, so there is no single script to wrap.

## Out of scope

- Parallel *feature* work — that is change 0008 (parallel backlog drain), which concerns fanning
  out `docket-implement-next` and explicitly treats claim-CAS as solved. This change is only about
  contention on the shared metadata working tree.
- The push-side CAS loops, which already work.
- Any daemon, server, or agent-to-agent messaging. Coordination stays in git plus the filesystem.

## Open questions

- Is the right fix to make collisions survivable (retry) or impossible (per-session worktrees)?
- Can an advisory lock actually span tool calls, given an agent may crash mid-section and strand
  the lock? What is the lease/expiry story if so?
- Should preflight distinguish "another agent is writing" from "a human left the tree dirty"?
  The former is transient and retryable; the latter needs a human and should abort-and-report.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
