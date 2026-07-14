#!/usr/bin/env bash
# scripts/board-refresh.sh — gated, atomic writer for the `inline` board surface (change 0059).
# Wraps render-board.sh (left completely unchanged): renders and replaces BOARD.md only when the
# caller's already-resolved $BOARD_SURFACES tokens include `inline`; otherwise touches nothing —
# no create, no truncate, no delete. This script OWNS the BOARD.md write decision so a disabled
# or GitHub-only configuration never truncates a prior board. No git operations whatsoever; the
# caller owns git add/commit/push.
#
# Usage: board-refresh.sh --changes-dir DIR --surfaces "TOKENS" [--repo OWNER/REPO]
#   --changes-dir DIR   required; the metadata working tree (active/, archive/, BOARD.md live here).
#   --surfaces "TOKENS"  required AS A FLAG, and its value must be NON-EMPTY (change 0071).
#                        Space-separated tokens (the caller's resolved $BOARD_SURFACES, verbatim).
#                        A missing flag is a wiring bug (exit 2). An EMPTY VALUE is also a wiring
#                        bug (exit 2) — it means the caller never resolved its config. The
#                        deliberate off-state is the reserved, exclusive token `none` (no-op,
#                        exit 0, BOARD.md never created or truncated).
#   --repo OWNER/REPO   optional; forwarded verbatim to render-board.sh.
# Only the exact `inline` token enables a render+write. Unknown tokens (typos, `github`) warn to
# stderr and are ignored — they never abort and never block `inline` from taking effect.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Mock seam: override RENDER_BOARD in tests to inject a stub (mirrors render-board.sh's GIT seam).
RENDER_BOARD="${RENDER_BOARD:-$SCRIPT_DIR/render-board.sh}"

CHANGES_DIR=""
REPO=""
SURFACES=""
SURFACES_SET=0
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --surfaces) SURFACES="$2"; SURFACES_SET=1; shift ;;
    --repo) REPO="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'board-refresh: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

[ -n "$CHANGES_DIR" ] || { printf 'board-refresh: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ] || { printf 'board-refresh: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }
[ "$SURFACES_SET" -eq 1 ] || {
  printf 'board-refresh: missing --surfaces (pass --surfaces none to disable the board)\n' >&2; exit 2;
}
# Change 0071 — the positive sentinel. An empty VALUE is a wiring bug, not a configuration: it is
# what an unresolved `$BOARD_SURFACES` degrades to, and treating it as "board disabled" is how a
# stale board used to ship behind a success exit code. The deliberate off-state must SAY so.
[ -n "$SURFACES" ] || {
  printf 'board-refresh: empty --surfaces value (unresolved config?); pass --surfaces none to disable the board\n' >&2
  exit 2
}

# Tokenize --surfaces. `none` is the reserved, EXCLUSIVE off-token: it disables every surface and
# may not be combined with any other token (a contradiction exits 2 rather than silently picking a
# winner). Only the exact `inline` token enables a render+write; unknown tokens warn and are
# ignored (a typo must never abort a build).
inline_enabled=0
none_seen=0
other_seen=0
for tok in $SURFACES; do
  case "$tok" in
    none)   none_seen=1 ;;
    inline) inline_enabled=1; other_seen=1 ;;
    github) other_seen=1 ;;
    *) printf 'board-refresh: unknown surface token ignored: %s\n' "$tok" >&2; other_seen=1 ;;
  esac
done

if [ "$none_seen" -eq 1 ] && [ "$other_seen" -eq 1 ]; then
  printf "board-refresh: 'none' is exclusive — it cannot be combined with other surfaces: %s\n" "$SURFACES" >&2
  exit 2
fi

if [ "$none_seen" -eq 1 ]; then
  printf 'board-refresh: board disabled (none) — no-op\n'
  exit 0
fi

if [ "$inline_enabled" -eq 0 ]; then
  printf 'board-refresh: inline disabled — no-op\n'
  exit 0
fi

# Atomic write: render into a temp file INSIDE the changes dir (same filesystem as BOARD.md, so
# the final mv is an atomic rename), and only move it onto BOARD.md after render-board.sh exits 0.
# A renderer failure never truncates the prior board. The trap cleans up the temp file on any exit.
tmp_board="$(mktemp "$CHANGES_DIR/.board-refresh.XXXXXX")"
trap 'rm -f "$tmp_board"' EXIT

render_args=(--changes-dir "$CHANGES_DIR")
[ -n "$REPO" ] && render_args+=(--repo "$REPO")

# Capture the renderer's exit code directly (no `!` — under `set -uo pipefail` without `-e`,
# `rc=$?` right after the command is the renderer's real code; negating with `!` would always
# yield 0 in the then-branch and swallow the failure signal).
"$RENDER_BOARD" "${render_args[@]}" > "$tmp_board"
rc=$?
if [ "$rc" -ne 0 ]; then
  printf 'board-refresh: render-board.sh failed (exit %d); BOARD.md left untouched\n' "$rc" >&2
  exit "$rc"
fi

# Second gate: the render exited 0 but must also be NON-EMPTY before it replaces BOARD.md. A
# zero-exit-but-empty render (a future render-board.sh regression, or an injected stub) would
# otherwise mv an empty file over a good board. Leave BOARD.md byte-identical (the EXIT trap
# removes the temp file) and exit non-zero so the caller skips its git add/commit — the
# belt-and-suspenders companion to the non-zero-exit branch above.
if [ ! -s "$tmp_board" ]; then
  printf 'board-refresh: render produced empty output; BOARD.md left untouched\n' >&2
  exit 1
fi

# mktemp creates the temp file at 0600; normalize to 0644 (the git-tracked, pushed board's mode)
# before the rename so a successful write matches what a plain `> BOARD.md` redirect would leave.
chmod 644 "$tmp_board"
mv "$tmp_board" "$CHANGES_DIR/BOARD.md" || { printf 'board-refresh: failed to replace BOARD.md\n' >&2; exit 1; }
printf 'board-refresh: inline rendered %s\n' "$CHANGES_DIR/BOARD.md"
exit 0
