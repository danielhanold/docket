---
id: 127
slug: 'scoped-metadata-validation-for-named-operations'
title: 'Scoped metadata validation for named operations'
status: 'Accepted'
date: '2026-09-24'
supersedes: []
reverses: []
relates_to: [93]
change: 449
---

## Context

Change 0309's transaction engine required an error-free complete corpus before and after every metadata mutation (grounded further in changes 0310 and 0312). As a result a single malformed or invalid unrelated change record blocked every other change's claim, lifecycle writes, finalize block/clear-block, and final archive/closeout; the canonical board renderer aborted on one bad record; and named reads probed live branch facts for every stacked change in the repository. Change 0337's scoped backlink loader already established the precedent that "a pre-existing error in a record the mutation cannot touch must not refuse the patch".

## Decision

Named metadata operations (those with an explicit change/ADR subject) pass a bounded in-memory validation subject set to the existing transaction engine: the root record, its structural closure (depends_on, stack ancestors, and, for closeout, descendants), and every record the plan writes (unioned after planning). Associative related/discovered_from/adrs links are not followed, per ADR-0093's structural-vs-associative distinction.

Errors touching any subject refuse. Other errors are tolerated only when they are exact pre-existing findings (full structured comparison including Related, Detail, severity, and multiplicity, compared as multisets) on records whose raw blob ids are unchanged; any new or changed error anywhere refuses. An empty or unresolvable subject set, a resolver error, and every operation without a subject contract keep strict whole-corpus validation.

No baseline is persisted; scope and baseline are recomputed per attempt, including after lease contention. The inline board renders usable records and lists unrenderable ones in a repair notice rather than aborting or silently omitting them. Named live branch-fact probes are bounded to the named change's own base/stack.

This departs from change 0309's error-free-complete-corpus mutation contract for named operations only; #0309's Accepted text is not rewritten.

## Consequences

An unrelated broken record no longer halts named implement/finalize workflows. Read-only health checks (status, repository check) still report the whole repository. Strictness is preserved for scope-less operations. The cost is a second relevance/equality layer in the engine that must stay fail-closed (mutation-tested). Whole-repository `status` still fails on an unrelated invalid branch name (follow-up).

## Alternatives considered

Keep the error-free-complete-corpus contract for every mutation (rejected: one bad record halts all unrelated work). Tolerate all errors outside the subject set without a baseline comparison (rejected: would let a mutation introduce or change errors elsewhere). Persist a validation baseline (rejected: stale under concurrency; recomputing per attempt is simpler and fail-closed). Follow associative links when computing scope (rejected per ADR-0093: they carry no structural dependency).
