#!/usr/bin/env bash
# scripts/runners/opencode.sh — the opencode runner adapter (change 0205). Owns everything
# child-specific for delegating a whole agent run to opencode via `opencode run`: preflight
# (binary), prompt assembly from the built-in wrapper source, flag mapping (model verbatim per
# ADR-0015; effort -> --variant; repo root -> --dir; permission posture -> --auto), foreground
# execution, stdout relay. Invoked by runner-dispatch.sh — not directly by skills.
# Contract: scripts/runners/opencode.md. Mock seam: OPENCODE_BIN. Env in (from the facade):
# DOCKET_REPO_ROOT (absolute run anchor — main worktree unless the caller named one, required), DOCKET_RUNNER_CFG_PERMISSIONS (default `ask`).
#
# WHY `ask` REFUSES: opencode prompts for approval before editing a file or running a command. A
# delegated run has no human channel to answer with, so without --auto it blocks on the first
# prompt until something times out. Refusing up front turns a silent hang into a diagnostic. The
# opposite posture — defaulting to --auto — would hand blanket auto-approval to anyone who merely
# typed `runner: opencode`, so the grant is an explicit, visible line in config instead.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
OPENCODE_BIN="${OPENCODE_BIN:-opencode}"

die(){ printf 'runners/opencode: %s\n' "$*" >&2; exit 1; }
warn(){ printf 'runners/opencode: %s\n' "$*" >&2; }

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

# --- permission posture: resolved BEFORE preflight so the refusal never depends on the binary ---
# An enum, not a boolean, structurally parallel to runners.codex.sandbox — it leaves room for a
# future deny-list value without a boolean->enum migration. An unrecognized value DIES rather than
# falling back to `ask`: a typo must not be indistinguishable from a deliberate refusal.
PERMISSIONS="${DOCKET_RUNNER_CFG_PERMISSIONS:-ask}"
case "$PERMISSIONS" in
  auto-approve) ;;
  ask) die "runners.opencode.permissions is 'ask' (the default) — a delegated run cannot answer opencode's approval prompts and would hang. Set 'runners.opencode.permissions: auto-approve' to approve everything not explicitly denied by your own opencode deny rules, or drop 'runner: opencode' from this agent." ;;
  # The remedy names quoting because the facade's block-mapping reader does NOT strip quotes, so
  # `permissions: "auto-approve"` arrives here as the literal `"auto-approve"` and lands on this
  # leg. Showing the value with its quotes is not a hint a reader can act on (ADR-0065).
  *)   die "runners.opencode.permissions must be 'ask' or 'auto-approve' (got '$PERMISSIONS') — write the value unquoted" ;;
esac

# --- preflight: binary (abort-and-report; never degrade to a native run) --------
# Auth is NOT probed. `opencode auth list`'s exit code on a machine with zero credentials is
# unverified, and a probe whose failure semantics are unknown would convert an unusual-but-working
# setup into a hard abort. Authentication is a documented prerequisite — see opencode.md.
command -v "$OPENCODE_BIN" >/dev/null 2>&1 || die "opencode CLI not on PATH — install opencode (https://opencode.ai) or unset runner: opencode"

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
# ADR-0015: the model ID is passed VERBATIM and never validated. Unlike codex — whose vocabulary
# tops out at xhigh, forcing a max->xhigh mapping — opencode's --variant takes docket's `max`
# natively, so the effort vocabulary passes straight through with NO mapping table.
# `inherit` is DOCKET'S OWN "no pin" sentinel (never a vendor model ID). DEFENSIVE TWIN:
# runner-dispatch.sh's normalization is the single owner and a dispatched run never arrives here
# holding the sentinel — this line covers the direct hand invocation opencode.md documents, which
# bypasses the facade. Without it, `--model inherit` would reach opencode as a literal
# provider/model string. Sentinel normalization, not model-ID validation (ADR-0015).
case "$MODEL" in inherit) MODEL="" ;; esac
if [ -z "$MODEL" ] && [ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ]; then
  warn "WARN effort '$EFFORT' dropped — --variant is a provider-specific model option and no model is resolved. Set an explicit model to pin effort on opencode."
  EFFORT=""
fi

cmd=( "$OPENCODE_BIN" run --dir "$DOCKET_REPO_ROOT" )
[ -n "$MODEL" ] && cmd+=( --model "$MODEL" )
[ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ] && cmd+=( --variant "$EFFORT" )
[ "$PERMISSIONS" = "auto-approve" ] && cmd+=( --auto )
# `--` ends option parsing so the prompt is always taken as the positional `message`, never as
# flags. Latent with today's agent bodies (all open with a letter), but a future wrapper whose body
# opens with a markdown bullet or a `--flag` example would otherwise have its prompt partly consumed
# by opencode's parser. Verified against opencode 1.18.11: `opencode run --version` prints the
# version (flag consumed), while `opencode run -- --version` sends `--version` as the message.
cmd+=( -- "$prompt" )

# --- foreground execution + relay ---------------------------------------------------
# opencode has no --output-last-message analogue, so the child's default formatted stdout IS the
# relay, verbatim — the cursor.sh posture. The alternative, --format json, would require parsing an
# unversioned event schema here, where a wrong parse silently TRUNCATES the relay; decoration in a
# faithful relay is the smaller failure. Any nonzero exit propagates as-is — abort-and-report,
# never a retry or a degrade.
"${cmd[@]}"
rc=$?
if [ "$rc" != "0" ]; then
  printf 'runners/opencode: opencode run exited %s\n' "$rc" >&2
  exit "$rc"
fi
exit 0
