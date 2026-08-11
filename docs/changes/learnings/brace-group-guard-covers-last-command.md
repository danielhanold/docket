---
slug: brace-group-guard-covers-last-command
hook: "`{ a; b; } > file || die` guards only `b` — a grouped or redirected command list takes its status from its LAST command, so a failed first half writes a silently truncated file at exit 0."
topics: [shell, guards, dataloss]
changes: [277]
created: 2026-08-11
updated: 2026-08-11
promotion_state: candidate
promoted_to:
---

## Apply
A brace group, a subshell, and a pipeline all report the status of their **last** command. So the
common idiom for assembling a file out of several pieces —

```bash
{ cat "$ORIGINAL"; printf '%s\n' "$EXTRA"; } > "$OUT" || die "could not write $OUT"
```

— has a `|| die` that is only ever reached when the **final** `printf` fails. A failed `cat` (the
source was deleted, reaped, or never created) prints its own diagnostic to stderr and the group
still exits 0, so `$OUT` is created, is non-empty, and is missing the part that mattered. Every
downstream emptiness or existence check passes. This is worse than a plain unguarded command,
because the visible `|| die` reads at review as coverage of the whole group.

The rule: **guard each command that can fail, individually** — not the group, and not the
redirection.

```bash
cat "$ORIGINAL" > "$OUT" || die "could not read $ORIGINAL"
printf '%s\n' "$EXTRA" >> "$OUT" || die "could not append to $OUT"
```

`set -o pipefail` does not help here (it covers pipelines, not brace groups), and `set -e` does not
fire inside a command list whose status is being tested by `||`. The same shape hides in
`( … ) > f || die`, in `for … done > f || die`, and in any function whose body ends in a cheap
`printf` — a function's status is likewise its last command's.

The tell at review: a `|| die` message that names the **output** ("could not write X") attached to a
group that also **reads** something. If the failure the message describes is not the failure the
status can express, the guard is decoration.

Related: [[guards-are-code]], [[best-effort-helper-on-a-sole-deliverable-path]], [[pipefail]].

## War story
- 2026-08-11 (#277, PR #194 — merged) — The run gate's bounded re-dispatch assembles a retry brief
  from the caller's original brief plus the retry context:
  `{ cat "$BRIEF_PATH"; printf …; } > "$RETRY_BRIEF" || die`. During a long delegated run the
  caller's temp file can be reaped, so `cat` fails, the `printf` succeeds, the group exits 0, and
  the re-dispatch runs on a brief holding **only the retry context — the original task silently
  stripped**. That is the exact silent-omission failure the whole change exists to eliminate,
  reintroduced by the guard idiom rather than by the logic
  ([[fix-reintroduces-its-own-defect-class]]). Caught at whole-branch review, not by the suite: the
  retry brief was non-empty, so both the byte check and the content check passed. Each half now
  carries its own guard.
