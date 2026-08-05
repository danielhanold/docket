---
slug: atomic-generated-write
hook: "Never redirect a renderer straight into the file it generates — > truncates on open, so a failed render destroys the last-good file before its exit code is even read."
topics: [shell, dataloss, generated]
changes: [67, 109, 83, 207]
created: 2026-07-16
updated: 2026-08-05
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

**The caveat this prescription owes you: `mv` replaces the inode, so the temp file's mode wins.**
`mktemp` output is non-executable, so the same temp+`mv` idiom applied to an executable file silently
demotes `100755` to `100644`. Whenever the target might be executable — any in-place rewrite of a
script, as opposed to a renderer emitting a data file — carry the mode across explicitly: `cp -p "$f"
"$tmp"` before writing, `chmod --reference="$f" "$tmp"`, or a literal `chmod 755` after the `mv`. Then
prove it: `git diff --summary` shows a `mode change` line, and `git ls-tree` shows the bit.

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
- 2026-07-20 (#109, PR #112) — The mode-loss face, hit by a rename sweep that adopted this very
  temp+`mv` prescription for BSD/GNU portability. It silently turned `scripts/docket-config.sh` and
  `scripts/ensure-global-config.sh` from `100755` into `100644`. Note *where* it surfaced: every check
  the plan enumerated per-file passed, and the breakage appeared only as three unrelated-looking
  failures in the **whole-suite** run — the concrete receipt for AGENTS.md's "run the whole suite at
  the build gate, never only the tests the plan enumerates" rule. Fixed with an explicit `chmod 755`
  and verified via `git ls-tree` + `git diff --summary`. Not a live bug in docket's own scripts: every
  in-repo temp+`mv` target is a non-executable data or doc file, and the two scripts that rewrite a
  tracked artifact through a temp file already handle mode deliberately (`board-refresh.sh` normalizes
  to 644 with a comment; `ensure-docket-env.sh` captures and restores the prior mode).
- 2026-07-21 (#83, PR #114) — **temp+`mv` is only half the protection: a COMPOUND render block
  reports the exit status of its LAST command, so a failed base copy passes the `|| die` and the
  `mv` publishes the fragment.** `mark-publish-deferred.sh` rendered with
  `{ cat "$tmp.2"; printf …; } > "$tmp.3" || die`. The idiom looks right — same-directory temp, `mv`
  at the end, exit code checked — and it still loses the whole record: an ENOSPC/EIO on the `cat`
  leaves `$tmp.3` holding only the marker section, the `printf` succeeds, the block's status is 0,
  the `die` never fires, and the `mv` writes a body-less file over the archived change record at
  exit 0. The prescription this finding already carries (`[ -s "$tmp" ]`) does not catch it either:
  the fragment is non-empty. Fixed by splitting the block so each write is checked separately, plus
  a size postcondition — documented in code and contract as a **gross-truncation check, not a proof
  of fidelity**, because no cheap size assertion can distinguish a legitimately shrinking render
  from a partial one. The general rule: `-s` proves *something* was written, never that *everything*
  was; when a render concatenates independent sources, check each source, not just the block.
- 2026-08-05 (#207, PR #159) — **The redirect need not be a renderer's, and temp+`mv` is not always
  the right prescription.** `sync-agents.sh` generated each wrapper as
  `emit_wrapper … > "$dir/docket-<name>.<ext>"` — a *function* whose stdout the caller redirects. On
  a bad `runner:` config the function did `log ERROR` then `exit 1` **inline, mid-body**. Both faces
  of `>` fired at once: the shell truncated the target at open, so the offending agent was left with
  a **zero-length wrapper**; and `exit 1` killed the whole script, so every agent later in glob order
  was never regenerated and a user-level failure meant the project-level pass never ran at all. The
  mechanism was years old and latent — change 0205's runner-wide required-model rule simply raised
  its trip rate, because a model-less `runner:` had been a *documented supported configuration* and
  docket cannot reach or migrate the config layers (`.docket.local.yml`, `~/.config/docket/config.yml`)
  that may still carry one. Note what the fix was **not**: temp+`mv` per file would have saved each
  wrapper individually and still shipped a mixed-state agent directory — fresh, stale, and
  half-regenerated wrappers with nothing announcing that generation stopped early. The right unit of
  atomicity was the whole run, so the remedy was a pre-flight gate over every (pass, agent, harness)
  triple that reports **all** offenders and fails before the first write
  ([[validate-the-whole-input-set-first]]), with the in-loop check demoted to a can't-happen
  assertion. Decide what the atomic unit *is* before reaching for the per-file idiom: when a run
  writes N generated files that are only meaningful as a set, validate the set, not each write.
