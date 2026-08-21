---
id: 336
slug: finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al
title: 'Finalize''s Go merge verb hardcodes --merge; honor the repo''s allowed methods (prefer rebase, never squash)'
status: proposed
priority: medium
type: fix
created: 2026-08-21
updated: 2026-08-21
depends_on: []
stacked_on:
related: []
discovered_from: [316]
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

docket's Go-runtime finalize merge verb hardcodes the GitHub merge method: `MergePullRequest`
in `internal/githubcli/merge.go` (around line 159) always passes `gh pr merge --merge`, i.e. a
merge commit. This was introduced in commit `e120c0a2` as part of the 0316 Go-runtime migration;
the previous bash finalize path used rebase.

On any repo that disallows merge commits, GitHub rejects the `--merge` and `finalize merge` returns
`merge-denied`, halting the run. This repo is exactly that case — it allows only **rebase** and
**squash** (`allow_merge_commit=false`) — so *every* finalize merge routed through the new runtime
fails here. It first surfaced closing out change 330 (PR #225), which had to be landed by hand with
`gh pr merge --rebase` before finalize could archive it as a merged-recovery.

## What changes

Make the finalize merge verb honor the repository's allowed merge methods instead of hardcoding one:

- Prefer **rebase** — the maintainer's general preference and what this repo has always used.
- **Never select squash.** Squash rewrites history in a way we do not want finalize to choose on its
  own, even when the repo permits it.
- Fall back sensibly when rebase is unavailable, resolving against the repo's actually-allowed
  methods (queryable via the GitHub API, e.g. `allow_merge_commit` / `allow_rebase_merge` /
  `allow_squash_merge`), rather than assuming any single method.

## Out of scope

- Changing repo-side merge-method configuration.
- Reworking the broader finalize sequence, the rebase/gate/publish steps, or the merged-recovery
  archive path — only the merge-method selection is in scope.

## Open questions

- **Edge case: merge-commit is the repo's *only* allowed method.** With squash off the table and
  rebase disallowed, the desired behavior is unclear. Options for the brainstorm to settle: fall
  back to a merge commit (the one allowed method), or refuse with a clear `merge-denied`-style block
  telling the human to enable rebase / merge manually. Do **not** silently squash.
- Should the allowed-methods preference order be configurable (a `finalize.merge_method:` knob), or
  is a fixed "rebase, else merge-commit, never squash" policy sufficient?
- **Adjacent finding — scope decision for grooming (may be a separate change).** `docket context
  finalize` returns a false `pr-unknown` unless `--repo-dir` is passed: an empty `repo-dir` makes the
  PR prober's `DiscoverRepository` reject the directory. Every finalize verb currently needs an
  explicit `--repo-dir` on this repo. Decide whether to fix the empty-repo-dir default here or split
  it into its own change.
