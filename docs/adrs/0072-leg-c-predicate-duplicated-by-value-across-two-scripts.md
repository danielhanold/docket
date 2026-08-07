---
id: 72
slug: leg-c-predicate-duplicated-by-value-across-two-scripts
title: Leg C's predicate is duplicated by value across two scripts, never shared
status: Accepted
date: 2026-08-07
supersedes: []
reverses: []
relates_to: []
change: 219
---

## Context

`aborted-run` leg C lives in `scripts/board-checks.sh`, which is **git-only by contract**: it shells
no `gh` and makes no network call, so it stays offline-safe and cheap. Leg C detects "built but not
delivered" — commits on the feature branch, `pr:` empty, branch tip idle past 2h — but it cannot
RESOLVE the finding it emits: a PR that exists and merely went unrecorded, and a run that died
before opening one, produce identical evidence in git. Distinguishing them requires asking GitHub.

Change 0219 put that resolution in `scripts/docket-status.sh`'s new `detect_orphan_pr`, beside
`detect_merged` where `gh` already lives. For the two findings to always agree — the property both
the code comments and `scripts/docket-status.md` sell as load-bearing — the enrichment must fire on
exactly leg C's population. That means `detect_orphan_pr` has to evaluate leg C's predicate.

## Decision

The predicate is **reimplemented in the second script and kept in sync BY VALUE, not shared by
import**. This is deliberate duplication of non-trivial logic: the 2h idle floor, the
local-then-remote-tracking ref resolution, the ahead-of-BOTH-`show-ref`-verified-bases guard with
its empty-array count gate (where no base resolving must read as SILENCE, never "ahead of nothing"),
and the pushed/unpushed discrimination on `refs/remotes/origin/<branch>`.

The reason is `board-checks.sh`'s independence: it must stay runnable on its own, offline, with no
dependency on `docket-status.sh`. The two scripts share no library today, and `board-checks.sh`
sources only `lib/docket-frontmatter.sh`. Extracting leg C's predicate into a shared library would
either drag network-aware code into the offline-safe script's dependency graph or create a third
component both must load — and the offline guarantee is the thing being protected.

The scope grew during the build: change 0219's spec anticipated duplicating only the 2h constant.
The whole-predicate duplication was forced by a review blocker, which found that the first
implementation had reused only the floor and NOT the ahead-of-bases guard — and therefore fired on
the "stopped with nothing built" signature that leg C deliberately stays silent about, while
asserting "is pushed" without ever checking a remote ref. Getting agreement between the two legs
required duplicating the whole predicate, not just its constant.

## Consequences

- **Enables:** `board-checks.sh` keeps its git-only/offline contract intact and stays independently
  runnable; the offline-safe check keeps emitting leg C's finding when `gh` is unavailable, and only
  the enrichment goes quiet.
- **Costs:** two implementations of one predicate can drift. There is no compile-time or test-time
  link between them — nothing fails if a future change retunes leg C's floor or its base handling
  and forgets `detect_orphan_pr`. The mitigation today is prose: each site's comments name the other
  and state that the values are kept in sync by value.
- **Gives up:** the single-source-of-truth property a shared helper would give, accepted in exchange
  for the offline guarantee.
- A future change that makes the two scripts share a library should revisit this.
