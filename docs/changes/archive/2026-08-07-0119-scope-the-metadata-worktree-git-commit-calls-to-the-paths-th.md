---
id: 119
slug: scope-the-metadata-worktree-git-commit-calls-to-the-paths-th
title: Scope the metadata-worktree git commit calls to the paths they own
status: killed
priority: medium
created: 2026-07-21
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [83]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
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

`scripts/docket-status.sh`'s `## Artifacts` commit calls a pathspec-less `git commit` inside the
**shared** `.docket` metadata worktree. That worktree is shared with concurrent autonomous loops, so
a pathspec-less commit sweeps up whatever another agent happens to have staged at that instant —
committing someone else's in-flight work under this run's message, on this run's push.

Change #0083 hit the same class from the other side (its CAS retry wedged the shared worktree by
`die`-ing mid-rebase) and adopted the stricter `git commit … -- <path>` idiom for its own new mark
path. The older call site was left as-is and noted in the #0083 results as "not filed, noted."

This is the `no-checkout-in-shared-worktree` / `cas-re-read-fresh-origin` family: operations in
`.docket` must scope themselves to the paths they own, because the tree is not exclusively theirs.

## What changes

- Audit `scripts/docket-status.sh` for pathspec-less `git commit` / `git add -A` style calls in the
  metadata worktree; scope each to the paths that call site owns.
- Sweep the other in-repo scripts that commit in `.docket` for the same shape.
- Add a guard keyed on the *shape* (a `git commit` in a metadata-worktree code path with no `--`
  pathspec), mutation-tested per `guards-are-code` — not an enumerated list of call sites.

## Out of scope

- The feature-branch commit paths, which run in per-change worktrees that are not shared.
- Any change to what the artifacts commit actually contains.

## Open questions

- Is a shape-keyed guard tractable here, or does the dynamic construction of these calls force a
  call-site-pinned audit instead?
- Are there call sites where a pathspec is genuinely impossible (an unknown-ahead-of-time file set)?
  If so, they need an explicit clean-tree precondition rather than a pathspec.

## Triage note (2026-07-26, change 0124)

Confirmed still live. The exposed call sites in the shared metadata worktree are exactly two:

- `scripts/docket-status.sh:278` — `"$GIT" -C "$mw" commit -q -m "$commit_msg"`
- `scripts/docket-status.sh:643` — `"$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links"`

For contrast, `scripts/docket-status.sh:677` (the change 0083 mark path) already carries
`-- "$archived"` — that is the target idiom, and it is in the same file, so the fix has a local
precedent to copy rather than invent.

The wider sweep bullet has two more to weigh, neither in the shared tree:
`scripts/reclaim-claims.sh:93` and `scripts/mint-stub.sh:218` already pass pathspecs;
`scripts/terminal-publish.sh:327` is pathspec-less but commits in its own dedicated publish
worktree (`$pub`), not the shared `.docket` — decide deliberately whether the guard should cover it
anyway on shape grounds, or whether scoping the guard to shared-tree call sites is the honest line.

## Auto-groom blocked

**2026-07-26 — `docket-auto-groom` abstained after one full design pass and one bounded revision
round.** A draft spec was written and attacked twice by the critic; two decisions did not survive,
one of them a policy call the drain has no authority to make. No spec was emitted. Most of the
analysis is settled and reusable — an interactive groom should start from the findings below rather
than from scratch.

### Settled (both critic passes ruled these sound — reuse them)

- **The defect set is exactly two call sites**, both in `scripts/docket-status.sh`: the commit in
  `commit_and_push_generated`, and the sweep's `"docket($id): refresh artifacts links"` commit.
  Every other shared-tree commit (`archive-change.sh`, `mint-stub.sh`, `reclaim-claims.sh`,
  `terminal-publish.sh`'s marker-clear, the #0083 mark path) already carries a pathspec.
- **`terminal-publish.sh`'s `$pub` commit stays out of scope** — and for a stronger reason than the
  triage note gives: that worktree is `mktemp -d`-created and torn down by `teardown`, *and* the
  commit is **index-driven** (gated on `git diff --cached --quiet`). Since a pathspec commit
  ignores the index, adding one there would change its behavior, not harden it.
- **A shape-keyed guard is the right instrument, in the default-deny direction** — flag every
  `git … commit` in `scripts/**.sh` that carries no `--` pathspec, with a justified exception list
  for exclusive worktrees, rather than a recognizer list of metadata-worktree variable names (which
  is default-allow and misses the next script that names its worktree differently).
- Guard lives in a new `tests/test_shared_worktree_commit_scope.sh` (the suite is discovered as
  `tests/test_*.sh`, so no registration). Exceptions keyed `<basename>:<-C target var>`, never line
  numbers (ADR-0054), with an existence floor so a stale exception reddens.
- `related:` should become `[83, 110, 118, 130]` (and arguably `85`). None is a build-order
  dependency, but **#0110** (shared-metadata-worktree contention) lists *per-session metadata
  worktrees, eliminating sharing entirely* as a live direction — if that lands, this guard's whole
  shared-vs-exclusive framing is what gets rewritten. **#0118** edits the adjacent lines. **#0130**
  is still `proposed`, so there is no landed BSD-portability guard to lean on.

### Undecidable / unresolved — what a human must supply

**1. (The blocking one — a policy call.) Scoping `commit_and_push_generated`'s commit changes the
autonomous loop's failure posture, and the drain may not choose that.** A pathspec commit is a
*partial* commit: `git commit -m x -- f` exits 128 (`fatal: cannot do a partial commit during a
merge`) in a mid-merge/mid-rebase tree, where today's pathspec-less form exits 0. The shared
`.docket` worktree left mid-rebase is exactly the state this stub's `## Why` cites and that #0110
documents. Traced through: the failure cannot be swallowed (that yields a `changed-pushed` false
success), so it must early-return a failure token; `board_pass_must_land`'s three attempts all
re-fail because its `pull --rebase … || true` cannot proceed mid-rebase; `--must-land` then exits
non-zero, and per the convention a must-land caller **STOPs and abort-and-reports**. So the fix
converts a previously-succeeding board pass in a wedged shared tree into a **hard halt of every
autonomous skill that runs one**. That may be the correct trade — a wedged tree arguably *should*
halt the loop rather than commit an interrupted rebase's staged content — but it is a deliberate
availability-vs-correctness decision about docket's own loop, plus a documented-contract rewrite
(`scripts/docket-status.md`'s `board inline changed push-failed` line means "committed locally, push
failed"; the reused token would no longer mean that, and `learnings index changed push-failed`
inherits the same divergence through the shared helper). **What a human should decide:** halt vs.
degrade, and if halt, whether a new report token is minted instead of overloading `push-failed`.

**2. The guard's recognizer needs three under-specified points nailed down, and the naive copy is
provably wrong.** The obvious move — copy `tests/test_docket_status.sh`'s `digest_tokens` pipeline
— mis-flags two *already-correct* sites, because that pipeline is anchored on a literal that never
appears inside a quoted string: `reclaim-claims.sh`'s commit message contains a `;` (splits the
segment before its `--`) and `mint-stub.sh`'s contains ` #$FROM` (the trailing-comment strip also
eats the continuation backslash, severing the `--`). A quoted-string masking pass fixes both
(prototyped by the critic against the real tree). Still open after that:
   - detection must run on the **masked** text, not the raw text (raw re-imports the false positive
     via `|| die "commit failed…"` tails);
   - the predicate must be evaluated **per `;&|` segment**, not over the whole joined line — the
     whole-line form is a demonstrated false negative on the #0083 mark path itself, where deleting
     the commit's `-- "$archived"` stays green on the neighbouring `add`'s `--`;
   - "`commit` as a git subcommand" needs an explicit driver set and an exact-token match: a bare
     word match flags `local tree commit`, `emit unknown-commit-ref`, and `git commit-tree`, while
     a `git|$GIT`-only driver match misses `docket-config.sh`'s locally-defined `g` git wrapper —
     which writes the metadata branch, so that miss would silently turn default-deny into
     default-allow.

### Recommendation

**Keep it — do not kill or defer.** The defect is real, confirmed live twice, and the fix is two
lines plus a guard. It is blocked only on decision (1), which is a maintainer policy call and
should probably be settled alongside **#0110**, since #0110 could remove the shared tree that makes
the whole invariant necessary. Re-arm by answering (1), deleting this section, and flipping
`auto_groomable` back to `true` — (2) is fully specified above and needs no human input.

## Why killed

Consolidated into #0247 at the 2026-08-07 backlog triage: settled alongside #0110 per this change's own auto-groom abstain (the architecture choice decides the commit-scoping failure posture). The two defect sites and the shape-keyed guard ask carry over verbatim.
