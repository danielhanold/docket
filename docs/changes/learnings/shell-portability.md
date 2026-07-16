---
slug: shell-portability
hook: "Treat awk whitespace classes, --leading grep patterns, and symlinked temp paths as suspect — and test each on both GNU and BSD."
topics: [shell, grep, awk]
changes: [25, 38, 46, 71]
created: 2026-06-19
updated: 2026-07-14
promotion_state: candidate
promoted_to:
---

## Apply
When a plan hands you awk/shell it authored, treat whitespace classes, `--`-leading patterns, and
symlinked temp paths as suspect, and test each on both GNU and BSD. Declare a `--`-leading pattern with
`grep -E -e "<pat>"` or `grep -qF -- "<pat>"`; use `[^[:space:]]` never `[^ ]` for awk indent classes and
test tab-indented input; `pwd -P` both the path and the prefix before stripping a worktree prefix.

## War story
- 2026-06-19 / 2026-06-21 / 2026-07-08 / 2026-07-14 (#25 PR #36; #38 PR #46; #46 PR #56; #71 PR #81 —
  merged, one shell-portability family) — Portability traps in tooling the plan itself authored. (a)
  **grep for a `--flag`:** a bare ERE that *leads* with `--` is parsed as a grep **option**
  (`unrecognized option`, exit 2); over-escaping to dodge that (`\-\-yes\b`) springs GNU grep's
  `stray \ before -` stderr warning, which BSD grep stays silent about — so it hides on macOS. Declare
  the pattern with `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`. #71 re-hit this inside a NEGATED
  assert, where the leading `!` inverted grep's exit-2 error into a green `ok` — the trap stops being a
  loud crash and becomes a permanently vacuous guard (guards family, (h)).
  (b) **awk whitespace class:** `ind()` used `[^ ]` (a literal-space class), so a **tab-indented**
  config layer was silently dropped — use `[^[:space:]]` and test tab-indented input. (c) **macOS path
  resolution:** `mktemp` yields `/var/…` but git reports `/private/var/…`, so stripping a worktree
  prefix matched nothing — `pwd -P` both the path and the prefix before stripping.
