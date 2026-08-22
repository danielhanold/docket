---
id: 337
slug: finalize-leaves-a-permanent-terminal-backlink-pending-leg-un
title: 'Finalize leaves a permanent terminal-backlink-pending leg under terminal_publish: false'
status: proposed
priority: medium
type: fix
created: 2026-08-22
updated: 2026-08-22
depends_on: []
stacked_on:
related: []
discovered_from: [336]
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

Under `terminal_publish: false` (this repo's default), finalize's close-out archives the change file
on the metadata branch only — the archived change file is never copied onto the integration branch.
But a merged change's build artifacts (its `plan:` and `results:` files, and any other artifact
carrying a `docket:backlink` block) *do* land on the integration branch when the PR merges. Those
backlink blocks point home to the change file by path, and at close-out that path changes from
`active/<id>-<slug>.md` to `archive/<YYYY-MM-DD>-<id>-<slug>.md`.

The integration-ref backlink leg of `finalize closeout`/`cleanup` tries to re-stamp those on-branch
artifact backlinks to the new archive path, but under `terminal_publish: false` that archive path
does not exist on the integration branch (the change file was never published there). The leg
therefore returns `invalid-state` and never lands, leaving:

- `finalize closeout` emitting a `terminal-backlink-pending` warning finding, and
- `finalize cleanup` returning `disposition: pending` / `reason: terminal-backlink-pending`
  indefinitely, and
- the on-`main` artifacts (e.g. plan/results) permanently backlinking to a stale `active/…` path
  that no longer exists on any branch.

The maintenance sweep is documented as the retry owner, but the retry hits the same `invalid-state`
every pass, so the finding never clears — it is a permanent pending leg, not a transient one.

Observed live finalizing change 0336 (dogfood run, 2026-08-22): PR #227 merged and archived cleanly
to `done`, but cleanup stayed `pending` on this leg across two retries; the merged
`docs/superpowers/plans/2026-08-21-finalize-effective-merge-method.md` and
`docs/results/2026-08-21-…-results.md` on `origin/main` still backlink to
`docs/changes/active/0336-…md`.

## What changes

Decide and implement the correct integration-ref backlink behavior when `terminal_publish: false`.
Candidate directions to evaluate during brainstorm:

- Skip the integration-ref backlink re-stamp entirely under `terminal_publish: false` (no archive
  path exists on the integration branch to point at), so closeout/cleanup report clean success
  rather than a permanent `terminal-backlink-pending`; and/or
- Re-stamp the on-branch artifact backlinks to a form valid without a published change file — e.g.
  point at the metadata-branch archive path, or neutralize/remove the block — so no artifact is
  left pointing at a non-existent `active/…` path.

Whichever is chosen, the outcome must be that a `terminal_publish: false` close-out reaches a
clean terminal state with no permanently-pending cleanup leg and no stale on-branch backlinks.

## Out of scope

- The `terminal_publish: true` path, where the archived change file *is* published onto the
  integration branch and the re-stamp target legitimately exists — that leg is expected to land.
- The merge-method selection work of change 0336 itself (already merged).
- Any change to what artifacts a feature branch merges onto the integration branch.

## Open questions

- Is `invalid-state` the intended signal here, or a latent bug in the leg's precondition check
  (should it detect `terminal_publish: false` up front and short-circuit to success)?
- Should the on-`main` artifact backlink point at the metadata-branch archive path, be rewritten to
  a branchless form, or be removed at close-out under `terminal_publish: false`?
- Are there already-merged changes in this repo's history carrying stale `active/…` backlinks that
  need a one-time backfill, or is forward-only behavior sufficient?
