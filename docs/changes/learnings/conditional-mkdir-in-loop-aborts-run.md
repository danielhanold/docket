---
slug: conditional-mkdir-in-loop-aborts-run
hook: "A conditional mkdir in a per-item loop needs || continue — under set -e a bad target aborts the ENTIRE run, not just that item."
topics: [shell, errexit, loops]
changes: [80]
created: 2026-07-15
updated: 2026-07-15
promotion_state: retained
promoted_to:
---

## Apply
A conditional `mkdir` in a per-item loop needs a `|| continue` (fail one item, not the run),
and the regression test must assert a LATER item still processes and the run exits 0 — not just
that the bad item is skipped.

## War story
- 2026-07-15 (#80, PR #87) — Under `set -euo pipefail`, `[ -d "$dir" ] || mkdir -p "$dir"` inside a
  loop does not just skip a bad target — a pre-existing NON-directory at `$dir` (stray file or
  dangling symlink) makes `mkdir -p` fail, and `set -e` aborts the ENTIRE script, leaving a partial
  install across every remaining harness. Whole-branch review reproduced both triggers empirically.
