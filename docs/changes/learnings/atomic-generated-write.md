---
slug: atomic-generated-write
hook: "Never redirect a renderer straight into the file it generates — > truncates on open, so a failed render destroys the last-good file before its exit code is even read."
topics: [shell, dataloss, generated]
changes: [67]
created: 2026-07-16
updated: 2026-07-16
promotion_state: candidate
promoted_to:
---

## Apply
Render to a same-directory temp file, check it, then `mv` — never `renderer > generated-file`. `>`
truncates at open, so the last-good content is gone *before* the renderer's exit code exists to be
checked, and the caller's own "only commit if the bytes changed" gate then reads the truncation as a
legitimate change and commits the emptied file. The shape:
`tmp=$(mktemp <dir>/.render.XXXXXX) && renderer > "$tmp" && [ -s "$tmp" ] && mv "$tmp" <target> || rm -f "$tmp"`.
The non-emptiness test (`-s`) is part of the check, not a nicety — a renderer that exits 0 having
written nothing is precisely the case an exit code cannot catch. On a render failure, commit the
substantive files anyway, leave the last-good generated file untouched, and SURFACE the failure —
never report the run as clean.

Two structural corollaries. A primitive that exists precisely to prevent this hazard protects nobody
while it stays PRIVATE to one caller — the hazard simply recurs at the next call site, which
reinvents the redirect. Give a generated file ONE gated writer plus a sentinel forbidding anyone else
from redirecting into it (docket's `board-refresh.sh` + `REDIRECT_RE`), or every future caller is
free to reintroduce the bug. And any needs-you advisory computed from the SOURCE files must fire
INDEPENDENTLY of the render — otherwise a render failure mutes exactly the escalation that a broken
state most needs.

## War story
- 2026-07-16 (#67, PR #91) — The change that introduced the learnings index shipped its own harvest
  step with `render-learnings-index.sh … > README.md`. A render failure would truncate the index to
  empty **and** the harvest's "commit only if bytes changed" probe would be SATISFIED by that
  truncation — so a failed render would commit an emptied index over a good one. `docket-status.sh`
  already carried `learnings_regen_index`, a render→temp→`mv` primitive built for exactly this
  hazard, but it was private to that script, so the new caller reinvented the redirect it had already
  solved. Found at review, not by the suite: every test was green because no fixture made the renderer
  fail. Fixed to render→temp→`mv`, and the same review found the second face of it — the over-cap and
  promotion-pending advisories were computed downstream of the render, so a render failure silently
  muted both needs-you signals at the exact moment something was already wrong. Both fixed;
  `learnings-refresh.sh` (a single gated writer, the `board-refresh.sh` shape) is filed as a follow-up
  because nothing structurally stops the next caller from reintroducing the redirect.
