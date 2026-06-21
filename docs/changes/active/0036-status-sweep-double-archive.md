---
id: 36
slug: status-sweep-double-archive
title: docket-status sweep — delegate archiving to archive-change.sh (remove the double-archive)
status: proposed
priority: low
created: 2026-06-21
updated: 2026-06-21
depends_on: [35]
related: [35]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Why

`docket-status`'s merge sweep archives a merged change twice. Steps c–e do a **manual**
archive — `git mv active/ → archive/`, set `status: done` / `results:` / `updated:`, run the
link renderer, then commit the change-file-only and push. Step f then **re-invokes**
`scripts/archive-change.sh`, which does the same `git mv` + `status: done` + change-file-only
commit + push over again. The second pass is idempotent (it reuses the already-archived file
and no-ops when bytes match), so the sweep is *correct* today — but it is convoluted: two code
paths that must stay in lock-step describe one operation, and `archive-change.sh` already
exists precisely to be the single archive primitive (it was extracted in change 0026 to remove
hand-staging failure modes). `docket-finalize-change` already delegates its archive entirely to
the script; the sweep should too. This was surfaced by the change #0035 whole-branch review as
a pre-existing tidy (not introduced there).

## What changes

Make the sweep delegate archiving entirely to `archive-change.sh`, the same way
`docket-finalize-change` does:

- Drop the manual `git mv` + field-edit + commit steps (c–e) and let `archive-change.sh`
  (step f) own the move, the `status: done` / `results:` / `updated:` writes, the
  change-file-only commit, and the push-with-rebase-retry.
- Preserve the behaviors the manual steps currently carry that the script does not yet — in
  particular the `## Artifacts` link re-render that change #0035 places in step d (it must
  still run, committed and pushed to `origin/docket` **before** terminal-publish, matching the
  finalize ordering). Decide whether that re-render belongs inside the post-archive renderer
  step (as finalize sequences it) so the two skills converge on one flow.
- Keep the two skills' archive descriptions byte-aligned per the convention's "must not
  diverge" note.

## Out of scope

- `terminal-publish.sh` mechanics and the publish copy-set (unchanged).
- `docket-finalize-change`'s archive (already delegates to the script — only the sweep is being
  brought into line).
- The board pass, health checks, and learnings harvest (untouched).

## Open questions

- Does any behavior in the current manual steps c–e have no equivalent in `archive-change.sh`
  (beyond the #0035 renderer call), such that collapsing to the script would silently drop it?
  The reconcile/brainstorm should diff the two paths field-by-field before removing either.
- After #0035 merges, where exactly should the renderer re-render sit in the delegated flow so
  finalize and the sweep stay identical?
