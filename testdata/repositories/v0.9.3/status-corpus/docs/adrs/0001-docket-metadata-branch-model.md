---
id: 1
slug: docket-metadata-branch-model
title: Planning metadata on an orphan docket branch; publish terminal records by copy, not merge
status: Accepted
date: 2026-06-03
supersedes: []
reverses: []
relates_to: []
change: 2
---

## Context

docket needs a durable, queryable source of truth for planning state (changes, statuses, ADRs, board, specs), shared across agents, machines, and time, with git as the only persistence mechanism. v1 kept that state directly on the integration branch (`main`): freshness was guaranteed (feature branches always cut from a branch carrying the latest planning state), but it **polluted code history** with a continuous stream of planning-only commits. A `metadata_branch: docket` knob existed but shipped as an unsupported v1 rough edge with three unsolved cross-branch problems: spec files read cross-tree during build, reconcile pushes landing on `docket` while feature branches cut from the integration branch, and **no mechanism to get finalized state onto the code line**.

A tempting "fix" for the last problem is `git merge docket` into the integration branch at finalize. It is wrong: a branch merge imports the branch's **entire commit history**, dragging every `proposed→in-progress→implemented` churn commit and the whole live `active/` surface onto the code line — exactly the pollution the separation exists to remove.

## Decision

Planning metadata lives on a dedicated **orphan `docket` branch** (no shared history, metadata-only), accessed through a **persistent, gitignored `.docket/` worktree** so the main working tree never switches branches. On a terminal transition (`done`/`killed`), the finalized records — the archived change file, its `spec:`, and its `Accepted` ADRs — are **published to the integration branch by selectively copying from `origin/docket`** (`git checkout origin/docket -- <paths>`, one dedicated commit), **never** by merging the branch. The live `BOARD.md` is never published; to read the board you view `docket`. Plan/results/code reach the integration branch separately, via the feature-branch PR merge.

**The rule a reader needs:** to publish *only the finalized state*, use a selective file copy, not a branch merge. `git merge docket` is the wrong tool; `git checkout origin/docket -- <paths>` is right.

## Consequences

- **Enables:** clean, code-only history on the integration branch, while the "why" (change + spec + ADRs) still lands next to the code at terminal state; a complete git-native planning audit trail on `docket`; no database or external service.
- **Costs:** a second checkout on disk (`.docket/`); cross-tree reads during build (the reconciled spec is read from `.docket/`, never carried on the feature branch); the publish is the single carefully-guarded metadata→code-line flow (sourced from `origin/docket`, archive-first ordering, fast-forward CAS push).
- **Given up:** the v1 simplicity of "everything on one branch." `main`-mode remains a pinned opt-out for teams that want it.
- `.docket/` deliberately sits at the repo root, **not** under `.worktrees/`, to avoid colliding with a feature worktree slug (`.worktrees/<slug>`) and to stay outside the ephemeral-worktree prune blast radius.
