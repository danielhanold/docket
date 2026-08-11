#!/usr/bin/env bash
# scripts/runners/codex.sh — the codex runner adapter (change 0079). Owns everything
# child-specific for delegating a whole agent run to OpenAI Codex CLI via `codex exec`:
# preflight (binary + auth), prompt assembly from the built-in wrapper source, flag
# mapping (model verbatim per ADR-0015; effort -> model_reasoning_effort; sandbox/network
# from the runners.codex config), foreground execution, final-message relay on stdout.
# Invoked by runner-dispatch.sh — not directly by skills. Contract: scripts/runners/codex.md.
# Mock seam: CODEX_BIN. Env in (from the facade):
# DOCKET_REPO_ROOT (absolute run anchor — main worktree unless the caller named one, required),
# DOCKET_RUNNER_CFG_SANDBOX (default workspace-write), DOCKET_RUNNER_CFG_NETWORK (default true).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
CODEX_BIN="${CODEX_BIN:-codex}"

die(){ printf 'runners/codex: %s\n' "$*" >&2; exit 1; }

AGENT=""; MODEL=""; EFFORT=""; BRIEF_FILE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --agent)  AGENT="${2:-}"; shift 2 ;;
    --model)  MODEL="${2:-}"; shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    # `shift 2` is this loop's house form, but bash's `shift` FAILS rather than truncating when the
    # flag is the last argument and this loop has no trailing shift — so a value-taking flag in
    # final position would spin here forever, making the "requires a path" refusal below
    # unreachable. Shift the flag, then the value only if a value is actually there.
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

# --- preflight: binary + auth (abort-and-report; never degrade to a native run) --
command -v "$CODEX_BIN" >/dev/null 2>&1 || die "codex CLI not on PATH — install Codex CLI (https://github.com/openai/codex) or unset runner: codex"
"$CODEX_BIN" login status >/dev/null 2>&1 || die "codex CLI is not authenticated — run: codex login"

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
SANDBOX="${DOCKET_RUNNER_CFG_SANDBOX:-workspace-write}"
NETWORK="${DOCKET_RUNNER_CFG_NETWORK:-true}"
# network is a boolean gate below; a non-boolean value must fail loud, never silently
# disable network (explicit config is never silently ignored — same posture as the facade).
# The remedy names quoting for the same reason as runners/opencode.sh's permissions leg: the
# facade's block-mapping reader does not strip quotes, so `network: "true"` arrives as the literal
# `"true"` and lands here. Showing the quoted value alone is not actionable (ADR-0065).
case "$NETWORK" in true|false) ;; *) die "runners.codex.network must be 'true' or 'false' (got '$NETWORK') — write the value unquoted" ;; esac
# `auto` is DOCKET's own "no pin" sentinel, never a vendor effort token — normalize it away before
# the mapping below, exactly as runners/cursor.sh and runners/opencode.sh do. Generated shims never
# forward it (emit_shim filters `auto`), so this is unreachable through config; it matters for a
# hand invocation, which the adapter contracts explicitly contemplate, and it makes the shared
# "`auto` behaves identically on every runner" claim true at the adapter layer too (change 0205).
case "$EFFORT" in auto) EFFORT="" ;; esac
# `inherit` is DOCKET'S OWN "no pin" sentinel for the MODEL, the twin of the `auto` effort sentinel
# directly above and never a vendor model ID. DEFENSIVE TWIN: runner-dispatch.sh's normalization is
# the single owner and a dispatched run never arrives here holding the sentinel — this line covers
# the direct hand invocation codex.md documents, which bypasses the facade entirely. Without it,
# `codex exec -m inherit` hands the child a non-existent model ID. Sentinel normalization, not
# model-ID validation (ADR-0015): no vendor value is inspected.
case "$MODEL" in inherit) MODEL="" ;; esac
case "$EFFORT" in max) EFFORT="xhigh" ;; esac   # codex's reasoning-effort vocabulary tops out at xhigh

cmd=( "$CODEX_BIN" exec -C "$DOCKET_REPO_ROOT" --sandbox "$SANDBOX" --color never )
if [ "$SANDBOX" = "workspace-write" ] && [ "$NETWORK" = "true" ]; then
  cmd+=( -c "sandbox_workspace_write.network_access=true" )
fi
[ -n "$MODEL" ]  && cmd+=( -m "$MODEL" )
[ -n "$EFFORT" ] && cmd+=( -c "model_reasoning_effort=$EFFORT" )
last_msg="$(mktemp "${TMPDIR:-/tmp}/codex.XXXXXX")"
cmd+=( --output-last-message "$last_msg" "$prompt" )

# --- foreground execution + final-message relay ------------------------------------
# codex's own event stream stays on stderr; THIS script's stdout is the child's final
# message only (the shim relays it verbatim).
"${cmd[@]}" 1>&2
rc=$?
if [ -s "$last_msg" ]; then cat "$last_msg"; fi
rm -f "$last_msg"
if [ "$rc" != "0" ]; then
  printf 'runners/codex: codex exec exited %s\n' "$rc" >&2
  exit "$rc"
fi
exit 0
