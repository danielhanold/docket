---
id: 239
slug: detect-merged-s-gh-pr-list-fallback-ignores-repo-so-a-scoped
title: detect_merged's gh pr list fallback ignores --repo, so a scoped pass queries the wrong repository
status: killed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [219]
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
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while fixing a review blocker on change 0219's new `detect_orphan_pr` leg,
which had resolved a `repo` value and then never spent it. Fixing that exposed the same latent shape
one function away: `detect_merged`'s own per-change fallback in `scripts/docket-status.sh` invokes
`"$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt` with **no `--repo`**, so
`gh` infers the repository from the process cwd.

**Opportunity** — `docket-status.sh` accepts a `--repo` flag, and `board_pass` / `github_mirror`
already forward it. `detect_merged`'s batched graphql arm honors it (it interpolates the resolved
`owner`/`name` into the query), but its `gh pr list` fallback — the arm taken for every
`implemented` change with no `pr:` recorded — does not. Under a `--repo`-scoped pass the two arms of
one function therefore query **different repositories**, and the fallback silently answers about
whatever repo the cwd happens to be.

**Independent value** — this is a correctness bug in the merge sweep, the mechanism that archives
merged changes to `done`. It stands entirely with change 0219 reverted: the code predates 0219 and
0219 does not touch `detect_merged`. A wrong answer here is worse than a missing one — the fallback
decides whether a change is swept, so a cwd-inferred repo can archive against the wrong PR data.

**Boundary** — pass `--repo "$repo"` to `detect_merged`'s `gh pr list` fallback, and add a test
asserting the flag reaches the stub's argv under an explicit `REPO_FLAG` (change 0219 established
the argv-recording stub idiom in `tests/test_docket_status.sh`, so the fixture pattern already
exists). It stops there: no change to the batched graphql arm, no change to the sweep's posture,
reasons, or output tokens, and no widening to other `gh` call sites.

**Reason for deferral** — 0219's scope is the `aborted-run` Step 7 seam: a fourth git-only leg plus
the GitHub enrichment that resolves leg C's ambiguity. `detect_merged` is the *merge sweep*, a
different subsystem with its own tests and its own failure posture; repairing it on 0219's branch
would put an unrelated behavior change inside a diff a human is reviewing for the abort oracle, and
0219's own review already flagged it as out of scope for the finding that surfaced it.

## Why killed

Consolidated into #0250 at the 2026-08-07 backlog triage: the 0219-harvest pair (repo-scoped fallback + idle-secs correspondence guard) lands as one small test-heavy change in docket-status.sh.
