---
id: 28
slug: wire-closeout-call-sites
title: Wire the close-out call sites to the extracted scripts
status: proposed
priority: high
created: 2026-06-19
updated: 2026-06-19
depends_on: []
related: [22, 23, 25, 26]
adrs: [7]
spec: docs/superpowers/specs/2026-06-19-wire-closeout-call-sites-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Change 0025 built and tested the three deterministic close-out scripts
(`archive-change.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`) and
**explicitly scoped** rewiring the four close-out call sites to invoke them. That
rewire never landed: a grep of `skills/` finds **no** reference to any of the three,
while the parallel extractions 0011 (`github-mirror.sh`) and 0022 (`render-board.sh`)
*did* land their rewire — `docket-status` names them inline. So the close-out path
still re-derives and hand-executes the full git dance on every run.

Hand-execution is not only costly, it is **wrong under partial failure**. Closing out
change 0026 (the run that surfaced this), the hand-rolled archive staged the `git mv`
rename but dropped the `status: done` edit — a `git add` that listed the
already-moved `active/` path aborted on the non-matching pathspec and staged nothing,
so a `status: implemented` archive was published to `main` and needed a corrective
commit + re-publish. `archive-change.sh` does the move, the frontmatter set, and the
change-file-only commit as one fail-closed primitive and cannot fail that way. The fix
is to use the script that already exists. (See LEARNINGS #22/#25/#26 — the whole
cluster says the same thing.)

## What changes

Per [the spec](../../superpowers/specs/2026-06-19-wire-closeout-call-sites-design.md):

- Rewire the **four call sites** to invoke the scripts on the 0025 contract — the
  model authors the commit message (`--message`), the script owns the plumbing +
  CAS-retry and is fail-closed, the skill trusts the exit code:
  `docket-finalize-change` (archive → `archive-change.sh`, the *Terminal publish*
  procedure → `terminal-publish.sh`, cleanup → `cleanup-feature-branch.sh`),
  `docket-status`'s merge sweep, `docket-new-change`'s proposed-kill, and
  `docket-implement-next`'s reconcile-kill.
- The mechanics stay centralized: finalize owns the terminal-publish procedure the
  other three reference, so the edits are reference updates — not N re-descriptions —
  exactly like `docket-status`'s existing `render-board.sh` / `github-mirror.sh`
  naming.

## Out of scope

- **No script behavior changes** and **no new tests** — `test_closeout.sh` already
  covers the scripts; this change adds no script code.
- **No regression guard / CI check** that the skills reference the scripts (considered
  and declined for this scope).
- **No finalize post-archive verification gate** (a separate idea).
- **`migrate-to-docket.sh`'s duplicated helpers** (LEARNINGS #26 tracks the twin).

## Open questions

- **Why 0025's rewire never landed** — descoped during 0025's build, or landed and
  later clobbered (plausibly by 0026's Step-0 skill edits on a bad rebase). The
  reconcile pass should `git log -p` the relevant skill sections to choose re-apply vs.
  write-fresh; the end state is identical either way. (Being analyzed in a separate
  post-mortem; not a blocker.)

## Reconcile log
