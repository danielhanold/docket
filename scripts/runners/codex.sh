#!/usr/bin/env bash
# scripts/runners/codex.sh — the codex runner adapter (change 0079). Owns everything
# child-specific for delegating a whole agent run to OpenAI Codex CLI via `codex exec`:
# preflight (binary + auth), prompt assembly from the built-in wrapper source, flag
# mapping (model verbatim per ADR-0015; effort -> model_reasoning_effort; sandbox/network
# from the runners.codex config), foreground execution, final-message relay on stdout.
# Invoked by runner-dispatch.sh — not directly by skills. Contract: scripts/runners/codex.md.
# Mock seam: CODEX_BIN. Env in (from the facade): DOCKET_REPO_ROOT (absolute, required),
# DOCKET_RUNNER_CFG_SANDBOX (default workspace-write), DOCKET_RUNNER_CFG_NETWORK (default true).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
CODEX_BIN="${CODEX_BIN:-codex}"

die(){ printf 'runners/codex: %s\n' "$*" >&2; exit 1; }

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

# --- preflight: binary + auth (abort-and-report; never degrade to a native run) --
command -v "$CODEX_BIN" >/dev/null 2>&1 || die "codex CLI not on PATH — install Codex CLI (https://github.com/openai/codex) or unset runner: codex"
"$CODEX_BIN" login status >/dev/null 2>&1 || die "codex CLI is not authenticated — run: codex login"

# --- prompt assembly: skills to load + the wrapper body + passthrough args -------
# skills: [a, b] frontmatter line -> "a b" (sed emits at most one line per file shape;
# first-line capture kept variable-side to stay pipefail-safe — LEARNINGS)
skills_line="$(sed -n 's/^skills:[[:space:]]*\[\(.*\)\].*/\1/p' "$SRC")"
skills_line="$(head -n1 <<<"$skills_line" | tr ',' ' ')"
skills_line="$(echo $skills_line)"
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
SANDBOX="${DOCKET_RUNNER_CFG_SANDBOX:-workspace-write}"
NETWORK="${DOCKET_RUNNER_CFG_NETWORK:-true}"
case "$EFFORT" in max) EFFORT="xhigh" ;; esac   # codex's reasoning-effort vocabulary tops out at xhigh

cmd=( "$CODEX_BIN" exec -C "$DOCKET_REPO_ROOT" --sandbox "$SANDBOX" --color never )
if [ "$SANDBOX" = "workspace-write" ] && [ "$NETWORK" = "true" ]; then
  cmd+=( -c "sandbox_workspace_write.network_access=true" )
fi
[ -n "$MODEL" ]  && cmd+=( -m "$MODEL" )
[ -n "$EFFORT" ] && cmd+=( -c "model_reasoning_effort=$EFFORT" )
last_msg="$(mktemp)"
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
