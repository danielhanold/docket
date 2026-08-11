#!/usr/bin/env bash
# scripts/runners/cursor.sh — the cursor runner adapter (change 0135). Owns everything
# child-specific for delegating a whole agent run to Cursor's CLI via `cursor-agent -p`:
# preflight (binary), prompt assembly from the built-in wrapper source, flag mapping (model
# verbatim per ADR-0015; effort ridden INSIDE the model value, matching Cursor's own
# `<id>[effort=<e>]` encoding — Cursor has no separate effort flag), foreground execution,
# final-message relay on stdout. Invoked by runner-dispatch.sh — not directly by skills.
# Contract: scripts/runners/cursor.md. Mock seam: CURSOR_BIN. Env in (from the facade):
# DOCKET_REPO_ROOT (absolute run anchor — main worktree unless the caller named one, required).
#
# RECORDED RISK: cursor-agent is known to be unreliable and to lag the Cursor IDE in features, so
# this adapter rests on a shakier foundation than runners/codex.sh. Its failure posture is pinned
# accordingly — any failure, timeout, or missing-feature error is a LOUD abort-and-report, never a
# silent fall-back to running the agent inline in the parent. A silent degrade here would
# reproduce change 0135's own root cause in a new location.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
CURSOR_BIN="${CURSOR_BIN:-cursor-agent}"

die(){ printf 'runners/cursor: %s\n' "$*" >&2; exit 1; }
warn(){ printf 'runners/cursor: %s\n' "$*" >&2; }

AGENT=""; MODEL=""; EFFORT=""; BRIEF_FILE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --agent)  [ $# -ge 2 ] || die "--agent requires a value";  AGENT="$2";  shift 2 ;;
    --model)  [ $# -ge 2 ] || die "--model requires a value";  MODEL="$2";  shift 2 ;;
    --effort) [ $# -ge 2 ] || die "--effort requires a value"; EFFORT="$2"; shift 2 ;;
    # `--brief-file` keeps the shift-then-conditional-shift shape it was written with; the arms
    # above use the `[ $# -ge 2 ] || die` shape instead. Both guard the SAME hazard — bash's
    # `shift` FAILS rather than truncating when the flag is the last argument and this loop has no
    # trailing shift, so an unguarded value-taking flag in final position spins here forever,
    # making every refusal below it unreachable. Here: shift the flag, then the value only if a
    # value is actually there, so the inline "requires a path" refusal is the one a caller sees.
    --brief-file) BRIEF_FILE="${2:-}"; [ -n "$BRIEF_FILE" ] || die "--brief-file requires a path"; shift; [ $# -gt 0 ] && shift ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$AGENT" ] || die "--agent is required"
# --- change 0277: the brief-file channel, and its exclusion with trailing argv -------
# The caller's brief is the child's ONLY input, and argv is a lossy way to carry it: a model must
# quote it correctly in one shot, and `$*` then joins the arguments on the first character of IFS.
# `--brief-file` removes both hazards — the caller writes a quoted heredoc and passes a path.
# BOTH CHANNELS AT ONCE IS REFUSED, never merged: preferring either silently drops or duplicates
# the child's whole input, and concatenation has no defensible ordering. Refusal is the only shape
# with no silent-wrong-answer mode. runner-dispatch.sh refuses it first; this is the DEFENSIVE TWIN
# for the direct hand invocation this contract documents, which bypasses the facade.
if [ -n "$BRIEF_FILE" ]; then
  [ $# -eq 0 ] || die "both --brief-file and trailing arguments were given — pass the brief in the file OR after '--', never both"
  [ -f "$BRIEF_FILE" ] && [ -r "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is not a readable file"
  [ -s "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is empty — a child launched with no task does not error, it improvises"
fi
[ -n "${DOCKET_REPO_ROOT:-}" ] || die "DOCKET_REPO_ROOT is not set (invoke via docket.sh runner-dispatch)"

SRC="$AGENTS_SRC/docket-$AGENT.md"
[ -f "$SRC" ] || die "no built-in agent source for '$AGENT' (expected $SRC)"

# --- preflight: binary (abort-and-report; never degrade to a native run) --------
command -v "$CURSOR_BIN" >/dev/null 2>&1 || die "cursor-agent CLI not on PATH — install Cursor's CLI or unset runner: cursor"

# --- prompt assembly: skills to load + the wrapper body + passthrough args -------
# skills: [a, b] frontmatter line -> "a b" (sed emits at most one line per file shape;
# first-line capture kept variable-side to stay pipefail-safe — LEARNINGS)
skills_line="$(sed -n 's/^skills:[[:space:]]*\[\(.*\)\].*/\1/p' "$SRC")"
skills_line="$(head -n1 <<<"$skills_line" | tr ',' ' ')"
# collapse whitespace + trim WITHOUT word-splitting/globbing (a bare `echo $x` would glob-expand)
skills_line="$(printf '%s' "$skills_line" | tr -s '[:space:]' ' ' | sed 's/^ *//; s/ *$//')"
# body = everything after the second frontmatter fence
body="$(awk '/^---[[:space:]]*$/{d++; next} d>=2{print}' "$SRC")"
prompt=""
if [ -n "$skills_line" ]; then
  prompt="First, load these skills by name, in this order:"
  for s in $skills_line; do prompt="$prompt
- invoke skill \`$s\`"; done
  prompt="$prompt

Then execute the following instructions exactly:

"
fi
prompt="$prompt$body"
# The payload, from whichever channel carries it. `$*` joined the positional parameters on the
# first character of IFS, so a multi-line brief passed as several arguments was flattened to one
# line and its plan-task structure, code blocks, and file lists all lost their boundaries —
# silently. The loop below preserves both order and line structure, so the surviving argv path
# stops being lossy even though the shim no longer teaches it.
# The brief file is appended via command substitution, which preserves its content and line
# structure but drops trailing newlines — so the append is faithful line-for-line rather than
# byte-verbatim, and a trailing blank line is not significant. Substitution rather than a format
# string is deliberate: a model-authored brief is untrusted
# input holding single quotes, backslashes, `%`, and backticks, so it must never pass through
# `eval` or a `printf` format string.
payload=""
if [ -n "$BRIEF_FILE" ]; then
  payload="$(cat "$BRIEF_FILE")"
  # THE EMPTINESS CHECK IS THE PREDICATE THE PAYLOAD ITSELF USES. The validation above measures
  # BYTES (`-s`), this read measures CONTENT: `$(cat …)` strips trailing newlines, so a
  # newline-only brief passes `-s` and arrives here EMPTY, which suppresses the whole task-context
  # block below and launches the child with no task at all — the improvise defect, silently.
  [ -n "${payload//[[:space:]]/}" ] || die "--brief-file '$BRIEF_FILE' carries no content — it holds only whitespace, and a child launched with no task does not error, it improvises"
elif [ $# -gt 0 ]; then
  payload="$1"; shift
  for a in "$@"; do payload="$payload
$a"; done
  # The same gap on the argv leg: arity is not content, so `-- ""` is arguments-present and
  # payload-empty. Same predicate, same refusal.
  [ -n "${payload//[[:space:]]/}" ] || die "the trailing arguments after '--' carry no content — they hold only whitespace, and a child launched with no task does not error, it improvises"
fi
if [ -n "$payload" ]; then
  prompt="$prompt

Additional caller arguments / task context:
$payload"
fi

# --- flag mapping -----------------------------------------------------------------
# ADR-0015: the model ID is passed VERBATIM and never validated. Cursor has no --effort flag —
# reasoning effort is a model parameter encoded inside the model value, the same `<id>[effort=<e>]`
# shape the wrapper emitter uses. With no model resolved the effort has nowhere to attach, so it is
# dropped — LOUDLY, because a silently-dropped pin is this change's own root cause.
# `inherit` is DOCKET'S OWN "no pin" sentinel (never a vendor model ID). DEFENSIVE TWIN:
# runner-dispatch.sh's normalization is the single owner and a dispatched run never arrives here
# holding the sentinel — this line covers the direct hand invocation cursor.md documents, which
# bypasses the facade. Without it, an explicit `model: inherit` reaches cursor-agent as a literal
# model ID (`--model inherit[effort=xhigh]`), where its compatible-model fallback silently
# substitutes something and takes the effort pin down with it, while the WARN branch below stays
# unreachable — change 0135's defect, reproduced on the hand path.
# This is sentinel normalization, not model-ID validation (ADR-0015): no vendor value is inspected.
case "$MODEL" in inherit) MODEL="" ;; esac
if [ -n "$MODEL" ] && [ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ]; then
  MODEL="${MODEL}[effort=${EFFORT}]"   # braces only to keep `$MODEL[` out of array-index territory
elif [ -z "$MODEL" ] && [ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ]; then
  warn "WARN effort '$EFFORT' dropped — Cursor encodes effort inside the model value, and no model is resolved. Set an explicit model to pin effort on Cursor."
fi

cmd=( "$CURSOR_BIN" -p --output-format text )
[ -n "$MODEL" ] && cmd+=( --model "$MODEL" )
cmd+=( "$prompt" )

# --- foreground execution + final-message relay ------------------------------------
# `--output-format text` makes the child's stdout its final message only; this adapter relays it
# verbatim. Any nonzero exit is propagated as-is — abort-and-report, never a retry or a degrade.
"${cmd[@]}"
rc=$?
if [ "$rc" != "0" ]; then
  printf 'runners/cursor: cursor-agent exited %s\n' "$rc" >&2
  exit "$rc"
fi
exit 0
