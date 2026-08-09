<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0247 — Make shared metadata worktree contention survivable and scope its commits](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0247-make-shared-metadata-worktree-contention-survivable-and-scop.md)**
<!-- docket:backlink:end -->

# Make shared metadata worktree contention survivable and scope its commits — design

Change: 0247 (consolidates killed #0110 and #0119; their analysis, including #0119's two-pass
critic-settled findings, carries over and is reused below rather than re-derived).

## Problem

Two halves of one concurrency defect in the shared `.docket` metadata worktree:

1. **Contention.** `scripts/lib/docket-preflight.sh` syncs with a bare
   `fetch && pull --rebase || return 1` — no retry, no discrimination. The shared worktree is
   dirty for another agent's entire multi-tool-call edit→commit window (seconds to minutes), so a
   concurrent preflight hard-fails on a perfectly normal transient state. Observed live (0109);
   reproducible by construction. No lock exists anywhere in `scripts/`.
2. **Blast radius.** Two pathspec-less `git commit` calls in `scripts/docket-status.sh` — the one
   inside `commit_and_push_generated` and the sweep's `"docket($id): refresh artifacts links"`
   commit — sweep up whatever another agent has staged at that instant, committing someone else's
   in-flight work under this run's message and push. (#0119's audit confirmed these are the only
   two exposed sites; every other shared-tree commit already carries a pathspec, and
   `terminal-publish.sh`'s `$pub` commit is exclusive-worktree **and** index-driven, so it stays
   out of scope — a pathspec there would change behavior, not harden it.)

## Architecture decision — survivable, not impossible

**Keep the single shared `.docket` worktree; make collisions survivable with a bounded,
discriminating retry in the preflight sync.** Rejected alternatives:

- **Per-session metadata worktrees** (collisions impossible): a large architecture change needing
  a mint/lease/prune lifecycle, N checkouts, and a rewrite of every shared-tree invariant, guard,
  and learning built since (the `no-checkout-in-shared-worktree` family, ADR-0046's clean-tree
  gates, #0119's whole framing). Today's real concurrency level is "one interactive session
  overlapping one autonomous loop" — parallel fan-out is #0008, which is *deferred*. Buying the
  heavyweight answer ahead of the workload that would justify it is the wrong default; if #0008
  revives, this decision is its natural re-opening point (recorded in the ADR).
- **Advisory lock** around the write→commit→push critical section: the section spans multiple
  tool calls, so no single script can own the lock, and a crashed agent strands it — the
  lease/expiry machinery needed to fix that is most of the per-session cost with none of the
  benefit.
- **Shrink the dirty window** (direct skills to write-and-commit in one call): narrows but never
  closes the race, and is prose discipline rather than mechanism; worth doing opportunistically
  but not as the fix.

Record the decision as an ADR (survivable-over-impossible; correctness-over-availability posture
below) via the standard build-time `docket-adr` step; list it in `adrs:`.

## Half 1 — preflight sync: bounded, discriminating retry

In `scripts/lib/docket-preflight.sh`'s metadata-worktree sync (both the worktree and the
main-mode branch of the sync function):

1. **Fetch first, then decide.** After `git fetch origin <metadata_branch>`, compare
   `HEAD` to the fetched remote ref. **If the local branch is already up to date (or ahead only),
   skip the `pull --rebase` entirely** — a dirty tree with no remote movement must never fail the
   sync. This alone removes the most common collision (the other agent has not pushed yet).
2. **If the remote moved and the rebase is needed**, attempt it; on failure, retry the whole
   fetch→compare→rebase step with backoff: **5 attempts, sleeping 2s, 4s, 8s, 8s between them
   (~22s total budget)**. The collision window is "another agent between edit and push"; most
   windows close in seconds once the other agent commits, and an autonomous caller re-running
   preflight later covers the long tail. The budget is a constant at the top of the function with
   a comment naming this rationale (per `tolerance-constant-calibrated-on-one-machine`: record the
   reasoning, not just the number).
3. **Never `--autostash`** — on a shared tree it stashes another agent's in-flight edits (#0110).
   Assert this with a repo grep in the test file (no `--autostash` in any metadata-tree sync
   path).
4. **On exhaustion, fail with a discriminating diagnostic**, still `return 1` (fail-closed, as
   today): name whether the blocker was a dirty tracked tree (`git status --porcelain
   --untracked-files=no` non-empty — likely another agent mid-write, or a human's leftover; retry
   later or inspect) or an in-progress rebase/merge in the shared tree (wedged — needs a human),
   versus an ordinary fetch/network failure. Untracked-only files must not count as dirty
   (ADR-0046's two-sided lesson).

No lock file, no new state, no new config knob. Tests: a fixture-repo test exercising (a)
dirty-tree + no-remote-movement → success without rebase; (b) dirty-tree + remote moved →
retries, then succeeds once the fixture "other agent" commits; (c) exhaustion → non-zero with the
discriminating diagnostic; (d) untracked-only file never fails the sync.

## Half 2 — scope the two commits; wedged-tree posture

1. Scope both exposed sites: add `-- "$rel"` to `commit_and_push_generated`'s commit, and scope
   the sweep's refresh-artifacts-links add/commit pair with `-- "$archived"` (today its add
   carries a bare `"$archived"` without `--` and its commit carries no pathspec at all). The
   #0083 mark path in the same file is the local precedent idiom to copy.
2. **Wedged-tree posture: halt, with a new token — never overload `push-failed`.** A pathspec
   commit exits 128 mid-rebase/mid-merge where the old pathspec-less form exited 0 — but that old
   exit 0 *committed an interrupted rebase's staged content under a board-refresh message*, which
   is corruption, not availability. So: before committing, probe the shared worktree for an
   in-progress rebase/merge (`rebase-merge`/`rebase-apply`/`MERGE_HEAD` under `git -C "$mw"
   rev-parse --git-dir`); if wedged, `commit_and_push_generated` returns a **new report token
   `blocked-wedged-tree`** (no commit attempted). `docket-status.sh`'s report lines carry it
   through (`board inline blocked-wedged-tree`, same for the learnings index);
   `--must-land` treats it as not-landed (exit non-zero → the autonomous caller STOPs and
   abort-and-reports, per the convention); best-effort callers log and continue, as they do for
   `changed-push-failed` today. Update `scripts/docket-status.md`'s report-line vocabulary and
   exit-code mapping accordingly; `changed-push-failed` keeps its exact current meaning.
   Note Half 1 makes the wedged state rarer and shorter-lived at its source; this posture is the
   backstop, not the common path.
3. **Shape-keyed guard**, new `tests/test_shared_worktree_commit_scope.sh` (auto-discovered):
   default-deny — flag every `git … commit` in `scripts/**/*.sh` lacking a `--` pathspec, with a
   justified exception list for exclusive-worktree sites keyed `<basename>:<-C target var>`
   (existence floor so a stale exception reddens; never line numbers, ADR-0054). Build it per
   #0119's prototyped findings, which are adopted verbatim as requirements: run detection on
   **quoted-string-masked** text (raw text false-positives on `reclaim-claims.sh`'s `;`-bearing
   message and `mint-stub.sh`'s ` #$FROM`); evaluate the predicate **per `;`/`&`/`|` segment**,
   never whole-line (demonstrated false negative on the #0083 mark path); match `commit` as an
   exact-token git subcommand under an explicit driver set that includes `git`, `$GIT`, **and**
   `docket-config.sh`'s local `g` wrapper (which writes the metadata branch). Mutation-test the
   guard: strip a pathspec from one of the two fixed sites and watch it redden.

## Out of scope

- The sweep's skip-publish marking (#0118 — which edits adjacent `docket-status.sh` lines; build
  order should watch for the file collision, recorded in `related:`).
- Parallel backlog drain (#0008, deferred) — this change only makes today's
  interactive+autonomous overlap safe; #0008's revival is the trigger to revisit per-session
  worktrees.
- The push-side CAS loops (already correct) and feature-branch commit paths (per-change
  worktrees, not shared).

## Assumptions

Every decision an interactive brainstorm would have raised, the chosen default, and why — the
human's deferred audit trail.

1. **Core fork: survivable (retry) over impossible (per-session worktrees).**
   Chosen: bounded discriminating retry, shared tree kept. Rejected: per-session worktrees
   (heavyweight lifecycle machinery ahead of a workload that is deferred in #0008; invalidates the
   shared-tree guard/learning corpus), advisory lock (cannot span tool calls; strands on crash),
   dirty-window shrinking alone (doesn't close the race). This is the conservative default: the
   smallest mechanism that fixes the observed failure, reversible, and it leaves the per-session
   door explicitly open at #0008's revival. The stub's open question flagged this fork for Daniel;
   the stub was nonetheless armed `auto_groomable: true` at the same triage that wrote that line,
   which we read as authorization to attempt the conservative default with a full audit trail —
   if that reading is wrong, rejecting this spec (or answering the fork differently) reopens only
   Half 1's mechanism; Half 2 is valid under either fork until the shared tree is actually
   removed.
2. **Retry parameters: 5 attempts, 2/4/8/8s backoff (~22s), constant with rationale comment.**
   Rejected: unbounded retry (an autonomous loop must terminate), a config knob (no evidence yet
   that any repo needs a different budget — add the knob when a second calibration exists).
3. **Up-to-date fast path before rebasing.** Chosen: fetch, compare, skip the rebase when the
   remote hasn't moved. Rejected: always rebasing (fails on a dirty tree even with nothing to
   do — the most common collision). Low risk: pure narrowing of the failure surface.
4. **Wedged-tree posture: halt with a new `blocked-wedged-tree` token** (the #0119 abstain's
   decision 1). Chosen: correctness over availability. The "availability" the old behavior bought
   was committing a half-rebased shared tree's staged content under this run's message — itself a
   corruption bug, not a feature to preserve; the convention's own doctrine for an autonomous run
   meeting an abnormal precondition is abort-and-report; and Half 1 makes the state rare and
   self-healing at the source. A new token (not overloading `push-failed`) keeps
   `scripts/docket-status.md`'s existing vocabulary meanings intact — the #0119 abstain
   identified overloading as a documented-contract rewrite, so we don't. Rejected: skip-and-
   continue silently (a must-land board pass that silently didn't land violates the board's
   never-trails invariant), attempting the commit anyway and mapping 128 to `push-failed`
   (mislabels, and the abstain traced it to a confusing three-retry death anyway).
5. **Guard design adopted verbatim from #0119's settled critic findings** (masked text,
   per-segment predicate, explicit driver set including the local `g` wrapper, default-deny with
   keyed exception list + existence floor). These survived two adversarial passes already;
   re-deriving them would only lose fidelity.
6. **`terminal-publish.sh:$pub` stays unscoped and on the guard's exception list** — exclusive
   `mktemp -d` worktree, index-driven commit; a pathspec would change behavior (#0119, settled).
7. **One ADR** records the survivable-over-impossible choice and the halt posture; no new config
   knobs, no new lifecycle state.
8. **Couplings**: `related: [8, 118]` (forward links only — #0008's revival re-opens the fork;
   #0118 collides on `docket-status.sh`). #0110/#0119/#0130 are archived; `discovered_from:
   [110, 119]` already records lineage. No `depends_on:` — nothing must merge first.
