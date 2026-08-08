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

AGENT=""; MODEL=""; EFFORT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --agent)  AGENT="${2:-}"; shift 2 ;;
    --model)  MODEL="${2:-}"; shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$AGENT" ] || die "--agent is required"
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
if [ $# -gt 0 ]; then
  prompt="$prompt

Additional caller arguments / task context:
$*"
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
if [ "$MODEL" = "inherit" ]; then MODEL=""; fi
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
