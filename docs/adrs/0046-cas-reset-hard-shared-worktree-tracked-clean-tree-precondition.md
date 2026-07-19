---
id: 46
slug: cas-reset-hard-shared-worktree-tracked-clean-tree-precondition
title: A compare-and-swap reset --hard in a shared metadata worktree requires a tracked-files-only clean-tree precondition
status: Accepted
date: 2026-07-18
supersedes: []
reverses: []
relates_to: [4, 12]
change: 91
---

## Context

docket's deterministic scripts push change-file mutations to the shared `docket` metadata branch
under a compare-and-swap loop. The established CAS rule (learned during change 0089,
`reclaim-claims.sh`) is that after a non-fast-forward rejection the retry must re-derive its
eligibility from fresh origin — `fetch` + `reset --hard <remote>/<branch>` — never by re-reading
the working tree it just wrote, because that always reads back its own pending write and
mislabels every real race as a no-op. `reset --hard` is safe there only because those scripts
push per item, so the local branch never carries more than one unpushed commit.

Change 0091's `scripts/mint-stub.sh` is the second script to use that pattern — the 0089
learnings note (`docs/changes/learnings/cas-re-read-fresh-origin.md`) explicitly flagged that a
recurrence should graduate the pattern from a script contract to an ADR; this is that recurrence.
Building it surfaced a hazard the first instance did not confront: `reset --hard` discards *any*
uncommitted work in the worktree, not just the script's own commit. `mint-stub.sh` runs from
inside autonomous `docket-implement-next` and close-out runs, in the `.docket` metadata worktree
that interactive sessions and other autonomous loops share concurrently. Code review reproduced
it: an unrelated uncommitted edit to another change file was destroyed by the retry, and the
script still reported success.

The first attempted fix — gating the reset on `git status --porcelain` being empty — was itself
wrong in the other direction: that reports untracked files too, which `reset --hard` never
destroys, so an incidental `.DS_Store` or editor swap file made the mint hard-fail on exactly the
contended path the feature exists to serve. Review reproduced that regression as well.

## Decision

A CAS `reset --hard` in a shared worktree is gated on a clean-tree precondition scoped to
**tracked modifications only** (`git status --porcelain --untracked-files=no`, or an equivalent
`git diff --quiet HEAD`). When tracked changes are present the script refuses and exits with an
error rather than resetting; untracked files never block it. The refusal is a real error,
distinguished from a lost race — a lost race retries, a refusal or any other git failure dies
immediately with a diagnostic and leaves no unpushed commit behind.

## Consequences

It preserves the CAS correctness property (the retry still re-derives from fresh origin) while
making the destructive step conditional on there being nothing of anyone else's to destroy. It
costs availability in one narrow case — a genuinely contended push while another writer holds
uncommitted tracked work fails instead of racing — which is the correct trade, since the
alternative is silent data loss in another agent's worktree. It requires that both directions be
pinned by tests: one fixture proving an unrelated tracked edit survives (the safety property),
and one proving an untracked file does NOT block the retry (the availability property) — a
single-direction test would let either the over-broad gate or the removed gate pass unnoticed.
And it generalizes: any future docket script adopting the fresh-origin CAS reset inherits this
precondition rather than rediscovering the hazard.
