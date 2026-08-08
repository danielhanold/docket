---
slug: shell-portability
hook: "Treat awk whitespace classes, --leading grep patterns, and symlinked temp paths as suspect — and test each on both GNU and BSD."
topics: [shell, grep, awk]
changes: [25, 38, 46, 71, 117, 186, 250, 255]
created: 2026-06-19
updated: 2026-08-08
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
When a plan hands you awk/shell it authored, treat whitespace classes, `--`-leading patterns, and
symlinked temp paths as suspect, and test each on both GNU and BSD. Any grep pattern carrying a
shell-ish `$` must be fixed-string (`-F`) — PATH grep is ugrep and reads a mid-pattern `$` as an
end-of-line anchor, returning 0 on a file that does match. Declare a `--`-leading pattern with
`grep -E -e "<pat>"` or `grep -qF -- "<pat>"`; use `[^[:space:]]` never `[^ ]` for awk indent classes and
test tab-indented input; `pwd -P` both the path and the prefix before stripping a worktree prefix.
A non-interactive flag on a tool that CAN prompt is load-bearing, not style: BSD `mv` onto an
unwritable destination hangs forever on a tty and exits **0 without installing** when stdin is at
EOF, so a trailing `|| die` cannot catch it — write `mv -f`. Corollary for guards: forcing a failure
via a filesystem flag (`chflags uchg`, a read-only mode) is sound only if the tool under test does
not prompt.

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
- 2026-07-28 (#117, PR #129 — merged) — Two more traps, both invisible on this machine. (a) **ERE
  `\?` is POSIX-undefined:** `grep -E '^adr-unpublished\t?\t'` was intended to match a literal `?`
  field, but where the escape degrades the `?` becomes a quantifier and the pattern matches EVERY
  `adr-unpublished` line — the assert built on it goes vacuous while staying green. Normalize such
  asserts to fixed-string matching. (b) **`for x in "${arr[@]}"` on an EMPTY array** throws
  `unbound variable` under `set -u` on bash 4.0–4.3. The repo's enforced floor is bash *major* 4,
  not 4.4, so the everyday state "ADR directory exists but is empty" aborted the script before its
  findings printed. Use `${arr[@]+"${arr[@]}"}`. No 4.0–4.3 exists in this environment, so the fix
  landed on code-identity with an already-proven sibling fix rather than a live repro — the trap is
  structurally invisible to a local green suite.
- 2026-08-01 (#186, PR #148 — merged) — **A bare `mv` is two bugs, and the loud one hid the silent
  one.** `backfill-change-types.sh` installed each staged file with a bare `mv`; BSD `mv` prompts
  (`override rw-r--r-- … ? (y/n [n])`) when the destination is unwritable, but only on a tty. So the
  test that made a destination immutable to exercise rollback **hung forever** from a real terminal —
  the suite had been unfinishable by hand since 2026-07-23 while every agent shell and the finalize
  gate (no tty) reported green. Under a pty with stdin at EOF the same `mv` prints `not overwritten`
  and exits **0**: the file is never installed, `if ! mv` never fires, no rollback runs, and the
  script reports success on a half-migrated backlog — exactly what its undo exists to prevent. Fix is
  `mv -f`, pinned by both a non-tty rollback block and a pty guard. Two side lessons: the `cp -p`
  twins were deliberately left alone (`cp` prompts only under `-i`, and `-f` on a rollback-*restore*
  path would unlink the destination the undo exists to preserve); and the mutation evidence was
  briefly falsified because PATH `grep` is ugrep, which reads the mid-pattern `$` in
  `grep -c 'mv -f "$out"'` as an end-of-line anchor and returns `0` even on the fixed file
  ([[grep-is-ugrep]] hazard, in a new place — a falsified *guard verification* rather than a
  portability bug). 15 further bare-`mv` install sites are tracked as #0189.
- 2026-08-07 (#250, PR #175 — merged) — **The ugrep `$` hazard is the mirror case, and it recurred
  twice in one change.** The repo's standing rule ([[grep-is-ugrep]]) is phrased around PATH `grep`
  being ugrep 7.5.0 and therefore *more permissive* than `/usr/bin/grep` — it accepts patterns BSD
  grep rejects, so portability bugs pass locally. This is the opposite direction: ugrep is
  **stricter** on a mid-pattern `$`, reading `grep -c 'mv -f "$out"'` or `grep -c '$sha'` as an
  end-of-line anchor and returning **0 against a genuinely matching file**. First seen falsifying
  #186's mutation evidence; here it appeared again in *two* places in one change — once in the
  plan's hand-run verification command, once latent in the committed guard's mutation-landed
  witness, where it would have been a permanently false-clear alarm. Fix both times: `grep -cF`.
  Rule: **any grep pattern containing a shell-ish `$` must be fixed-string (`-F`)**, and a
  verification command that returns 0 is not evidence until you have proven the pattern can match
  anything at all.
- 2026-08-08 (#255, PR #182) — Two awk traps, both of which cost real build time and neither of
  which looks like a logic bug. (a) **No literal apostrophe may appear anywhere inside an awk
  program embedded in a single-quoted shell word — comments included.** One was introduced during a
  fix; the shell word truncated silently, `bash -n` still **passed**, and the failure surfaced only
  when the file was sourced. Write `\047` / `\042` instead. (b) **macOS ships BWK awk, where
  `close` is a builtin and cannot be used as a parameter name.** It is a parse error that surfaces
  only at runtime and makes every awk-path assert fail simultaneously, which reads as a broad
  logic regression rather than a naming collision. Both are invisible to a GNU-awk-only check.
