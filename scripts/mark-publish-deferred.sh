#!/usr/bin/env bash
# scripts/mark-publish-deferred.sh — the sole writer of the `## Publish deferred` marker
# (change 0083). A terminal close-out whose publish step is EXPECTED (terminal_publish: true,
# docket-mode) but consciously deferred or blocked leaves this dated section on the archived
# change file, so the gap is visible where a human reads it instead of living only in a chat
# thread (the #0043 failure mode). `board-checks.sh`'s `publish-deferred` check reads it;
# `terminal-publish.sh` removes it on a successful publish.
#
# PURE FILE EDITOR: no git, no network, no commit, no push. The caller stages, commits, and
# pushes on the metadata branch per docket's field-write rule. This keeps the file edit
# deterministic and testable in one place (ADR-0012 script-vs-model boundary), mirroring
# render-change-links.sh.
#
# Usage:
#   mark-publish-deferred.sh --mode add --change-file PATH --reason deferred|blocked
#                            [--detail TEXT] [--date YYYY-MM-DD] [--integration-branch B] [--id N]
#   mark-publish-deferred.sh --mode remove --change-file PATH
#
#   --mode add     Write the marker. IDEMPOTENT BY REPLACEMENT: an existing section is removed
#                  first, so a re-mark never appends a second heading (the presence-encoded-state
#                  failure re-hit on `## Finalize blocked`). The section is appended LAST.
#   --mode remove  Strip the marker. A file without one is a no-op that exits 0, leaving the file
#                  byte-untouched (no write at all — decided by a PRECONDITION grep before any temp
#                  file exists, so the trailing-blank-line trim below never runs).
#   --id / --integration-branch
#                  Interpolated into the rendered body, therefore as untrusted as --detail: an id
#                  must be a non-negative integer, a branch name must carry no control character.
#                  A smuggled column-0 `## ` heading would terminate the marker section, making
#                  `--mode remove` strand everything after it in the record permanently.
#   --reason       Fixed prefix: `deferred` (a human gate that was never answered) or `blocked`
#                  (a wall the run could not pass, e.g. a protected-branch push denial).
#   --detail       Short free text after the prefix. MODEL-AUTHORED ⇒ UNTRUSTED: rejected at
#                  intake if it carries any control character (newline, CR, TAB). Written through
#                  awk ENVIRON, never interpolated into a sed replacement, so `&` and `\1` in
#                  ordinary English survive verbatim.
#
# Exit codes: 0 = the file now matches the requested state. 1 = a real error (bad args, missing
# or unreadable file, a rejected --detail/--id/--integration-branch, or a failed render). The file
# is left BYTE-UNTOUCHED on every exit-1 path — the render writes to a temp file, and every step
# of it is checked individually so a partial render can never reach the mv.
#
# Invariants:
#   - The heading is matched WHOLE-LINE (`$0 == "## Publish deferred"`), never as a substring:
#     change files routinely MENTION marker names in prose, and a substring match would delete
#     from an inline mention to the next heading. Mirrors has_section's `-x` rule.
#   - The section ends at the next COLUMN-0 `## ` heading or EOF. `### ` sub-headings inside the
#     section do not terminate it (`^## ` cannot match `### `, whose third char is `#`).
set -uo pipefail

MODE="" CHANGE_FILE="" REASON="" DETAIL="" DATE="" INT_BRANCH="main" ID=""
MARKER='## Publish deferred'

die(){ printf '%s\n' "mark-publish-deferred: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift ;;
    --change-file) CHANGE_FILE="${2:-}"; shift ;;
    --reason) REASON="${2:-}"; shift ;;
    --detail) DETAIL="${2:-}"; shift ;;
    --date) DATE="${2:-}"; shift ;;
    --integration-branch) INT_BRANCH="${2:-}"; shift ;;
    --id) ID="${2:-}"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

case "$MODE" in add|remove) ;; *) die "invalid --mode: '$MODE' (expected add|remove)" ;; esac
[ -n "$CHANGE_FILE" ] || die "missing --change-file"
[ -f "$CHANGE_FILE" ] || die "change file not found: $CHANGE_FILE"
[ -r "$CHANGE_FILE" ] && [ -w "$CHANGE_FILE" ] || die "change file not readable/writable: $CHANGE_FILE"

# strip_marker FILE — print FILE to stdout with the `## Publish deferred` section removed.
# Whole-line heading match; the section ends at the next column-0 `## ` heading or EOF.
strip_marker(){
  MPD_MARKER="$MARKER" awk '
    BEGIN { skip = 0 }
    {
      if (skip) {
        # A column-0 `## ` heading ends the section. `### ` does not match (3rd char is #).
        if ($0 ~ /^## /) { skip = 0 } else { next }
      }
      if ($0 == ENVIRON["MPD_MARKER"]) { skip = 1; next }
      print
    }
  ' "$1"
}

# write_atomic FILE CONTENT-PRODUCER... — render to a temp file, then move into place. Never
# redirect a producer straight into the file it rewrites: `>` truncates on open, so a failed
# render would destroy the last-good file before its exit code is read (atomic-generated-write).
tmp="$(mktemp)" || die "mktemp failed"
# Every intermediate this script writes is derived from $tmp, so one trap covers them all. Listing
# them explicitly (rather than `rm -f "$tmp"*`) keeps the cleanup from depending on a glob that a
# future intermediate could silently escape.
cleanup(){ rm -f "$tmp" "$tmp.2" "$tmp.3"; }
trap cleanup EXIT

if [ "$MODE" = remove ]; then
  # PRECONDITION, not a post-hoc byte comparison (change 0083 review, minor 6). A file with no
  # whole-line `## Publish deferred` heading is a TRUE no-op: no temp file is even written, so the
  # file cannot be touched. The old test — strip, then `cmp -s` the stripped output against the
  # input — was WRONG for a file that does not end in a newline: awk terminates its last output
  # line regardless, so the stripped copy differed by exactly the appended `\n`, `cmp` reported
  # "changed", and the no-op path fell through and rewrote the file. That silently falsified this
  # script's own documented "byte-untouched, not merely line-equivalent" claim. The `-x` whole-line
  # match is the same rule the removal itself uses, so gate and action agree by construction: an
  # inline PROSE MENTION of the marker is not a heading and does not arm the write.
  grep -qxF -- "$MARKER" "$CHANGE_FILE" || exit 0
  strip_marker "$CHANGE_FILE" > "$tmp" || die "strip failed"
  # A marker WAS present and just got stripped: trim any trailing blank lines that left behind,
  # then restore a single terminating newline.
  awk 'BEGIN{n=0} {lines[++n]=$0} END{ last=n; while (last>0 && lines[last]=="") last--; for(i=1;i<=last;i++) print lines[i] }' "$tmp" > "$tmp.2" || die "trim failed"
  mv "$tmp.2" "$CHANGE_FILE" || die "write failed"
  exit 0
fi

# ----- add -----
case "$REASON" in deferred|blocked) ;; *) die "invalid --reason: '$REASON' (expected deferred|blocked)" ;; esac
# Model-authored free text is untrusted input. Reject by SHAPE (any control character), never by
# enumerating bad strings: a newline would inject whole lines into the body, and a TAB would shift
# the findings channel's columns downstream.
case "$DETAIL" in
  *[[:cntrl:]]*) die "--detail must be a single line with no control characters (newline/CR/TAB)" ;;
esac
# `--id` and `--integration-branch` are interpolated into the rendered body exactly like --detail,
# so they are exactly as untrusted (change 0083 review, finding 3). Unvalidated, they were WORSE
# than --detail: a newline in either injects arbitrary lines INTO the marker section, and a
# column-0 `## ` heading among them TERMINATES the section for `strip_marker`. `--mode remove`
# then stops at the injected heading and leaves the tail behind permanently — corruption the sole
# writer of this marker cannot undo — and `terminal-publish.sh`, which only checks that the marker
# heading is gone, publishes the corrupted record with exit 0. Shape checks, at intake, next to
# --detail's: an id is a number, a branch name has no control characters (git forbids them too).
if [ -n "$ID" ]; then
  case "$ID" in
    *[!0-9]*) die "invalid --id: '$ID' (expected a non-negative integer)" ;;
  esac
fi
case "$INT_BRANCH" in
  *[[:cntrl:]]*) die "--integration-branch must carry no control characters (newline/CR/TAB)" ;;
esac
[ -n "$DATE" ] || DATE="$(date -u +%Y-%m-%d)"
case "$DATE" in
  [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;;
  *) die "invalid --date: '$DATE' (expected YYYY-MM-DD, UTC)" ;;
esac

# Replace, never append: strip any existing section first (idempotent re-mark).
strip_marker "$CHANGE_FILE" > "$tmp" || die "strip failed"
awk 'BEGIN{n=0} {lines[++n]=$0} END{ last=n; while (last>0 && lines[last]=="") last--; for(i=1;i<=last;i++) print lines[i] }' "$tmp" > "$tmp.2" || die "trim failed"

id_hint=""
[ -n "$ID" ] && id_hint=" --id $ID"

# The base copy is SPLIT OUT of the group below (change 0083 review, finding 2). A command
# group's exit status is its LAST command's, so `{ cat "$tmp.2"; printf …; } > "$tmp.3" || die`
# read the status of the final printf and NEVER of the cat. A failed `cat` (ENOSPC, EIO) therefore
# left `$tmp.3` holding only the marker section, `die` did not fire, and the `mv` below replaced
# the archived record with it: the whole body destroyed, exit 0, by the sole writer of a durable
# record. Demonstrated: `{ cat /nonexistent; printf 'TAIL\n'; } > out || echo die` writes TAIL and
# never echoes. Now the failure-prone read stands alone with its own `|| die`.
cat "$tmp.2" > "$tmp.3" || die "render failed: could not copy the change file body"
{
  printf '\n%s\n\n' "$MARKER"
  printf '### %s — terminal-publish to `%s` not completed\n\n' "$DATE" "$INT_BRANCH"
  # ENVIRON, not interpolation: `&` and `\1` are ordinary English and must survive verbatim.
  MPD_REASON="$REASON" MPD_DETAIL="$DETAIL" awk 'BEGIN{
    d = ENVIRON["MPD_DETAIL"]
    if (d == "") printf "**%s** — no further detail recorded.\n\n", ENVIRON["MPD_REASON"]
    else         printf "**%s** — %s\n\n", ENVIRON["MPD_REASON"], d
  }' </dev/null
  printf 'Close-out steps 1–2 (archive, `## Artifacts` re-render) landed on the metadata branch;\n'
  printf 'the terminal-publish step (copying the archived change file + its `spec:` + its Accepted\n'
  printf 'ADRs onto `%s`) did **not** run. The record is on the metadata branch only.\n\n' "$INT_BRANCH"
  printf '**Re-arm:** complete the publish (`docket.sh terminal-publish%s …`), or record a decision\n' "$id_hint"
  printf 'not to. A successful publish removes this section automatically.\n'
} >> "$tmp.3" || die "render failed: could not append the marker section"

# Size postcondition — a SECOND, independent backstop against a truncated render reaching the
# `mv`. `add` only ever APPENDS to the stripped base, so the rendered file can never be smaller
# than that base; a shorter one means the body was lost somewhere above. This catches the case
# `|| die` structurally cannot: a read that truncates and still reports success. It is a
# gross-truncation check, not a proof of fidelity — a loss smaller than the marker section it
# appends would slip through — so it backs the per-step `|| die`s up, it does not replace them.
base_bytes="$(wc -c < "$tmp.2")" || die "render postcondition: cannot size the change file body"
out_bytes="$(wc -c < "$tmp.3")" || die "render postcondition: cannot size the rendered file"
[ "$out_bytes" -ge "$base_bytes" ] \
  || die "render postcondition: rendered file ($out_bytes bytes) is smaller than the body it was built from ($base_bytes bytes) — refusing to overwrite $CHANGE_FILE"

mv "$tmp.3" "$CHANGE_FILE" || die "write failed"
exit 0
