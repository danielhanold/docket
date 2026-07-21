---
slug: pipefail
hook: "Never producer | early-exiting-consumer under set -o pipefail — capture into a variable first."
topics: [shell, pipefail, testing]
changes: [11, 16, 46, 83]
created: 2026-06-16
updated: 2026-07-21
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Never `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`, or any reader that may stop
before EOF) under `set -o pipefail` — capture into a variable first, then grep/`head <<<"$var"`.

## War story
- 2026-06-16 / 2026-07-08 (#11 PR #11; #16 PR #30; #46 PR #56 — merged, one pipefail family) — A test
  piped a live-producing script straight into `grep -q`; grep exits on first match, the still-writing
  producer takes SIGPIPE, and `pipefail` turned that 141 into an intermittent failure — review later
  found the same shape with `head`, and #46 hit it again in production code (`printf … | section_body`,
  whose consumer `exit`s early; guarded with `|| true`).
- 2026-07-21 (#83, PR #114) — **The inverse failure mode: the same shape makes a guard fail OPEN.**
  Every prior instance in this family turned SIGPIPE 141 into a spurious *failure*. `terminal-publish.sh`'s
  postcondition block held both faces at once. `printf … | grep -q …` over a full integration-branch
  `ls-tree` gives a consuming repo with a few thousand tracked files an intermittent false
  `postcondition: … missing` (the known face) — but the neighbouring worktree-survival check
  `… | grep -q "pub-$T" && die` **skips the `die` entirely** when the pipe takes a 141, because the
  non-zero status makes the `&&` not fire. A guard whose whole job is to abort silently evaporates,
  and the run exits 0. The branch's headline guarantee rested on that block. Read `pipefail` hazards
  in both directions: ask not only "can this report a failure that did not happen" but "can this skip
  a check that should have fired" — the second is invisible in a green suite. Converted to
  here-strings. Found while in the file for unrelated work; it was pre-existing.
