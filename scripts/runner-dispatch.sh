#!/usr/bin/env bash
# scripts/runner-dispatch.sh — the runner-neutral delegation facade (change 0079), behind
# `docket.sh runner-dispatch`. Validates arguments, anchors the repo root (ADR-0034),
# resolves the runners.<name>: config block across layers (repo-local > repo-committed >
# global; per-key), exports it as DOCKET_RUNNER_CFG_<KEY>, and execs the named adapter
# scripts/runners/<name>.sh. Registration IS the adapter file's existence. Unknown runner
# => loud nonzero (abort-and-report). Contract: scripts/runner-dispatch.md.
# Mock seams: RUNNERS_DIR, GIT (via lib/docket-root.sh).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNERS_DIR="${RUNNERS_DIR:-$SELF_DIR/runners}"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR/lib/docket-root.sh"

die(){ printf 'runner-dispatch: %s\n' "$*" >&2; exit 1; }

RUNNER=""; AGENT=""; MODEL=""; EFFORT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --runner) RUNNER="${2:-}"; shift 2 ;;
    --agent)  AGENT="${2:-}";  shift 2 ;;
    --model)  MODEL="${2:-}";  shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$RUNNER" ] || die "--runner is required"
[ -n "$AGENT" ]  || die "--agent is required"
# The runner name becomes a path component below — reject anything that could traverse out
# of RUNNERS_DIR (the facade family is a finite table, never an escape hatch).
case "$RUNNER" in
  *[!A-Za-z0-9._-]*|*..*) die "invalid runner name '$RUNNER'" ;;
esac
ADAPTER="$RUNNERS_DIR/$RUNNER.sh"
if [ ! -f "$ADAPTER" ]; then
  registered="$(ls "$RUNNERS_DIR" 2>/dev/null | sed -n 's/\.sh$//p' | tr '\n' ' ')"
  die "unknown runner '$RUNNER' — no adapter at $ADAPTER (registered runners: ${registered:-<none>})"
fi

REPO_ROOT="$(docket_main_worktree)"
[ -n "$REPO_ROOT" ] || die "not inside a git repository"
export DOCKET_REPO_ROOT="$REPO_ROOT"

# --- runners.<name>: config, per-key across layers (local > committed > global) -----
# Same nested-section awk shape as sync-agents.sh's section_body (kept self-contained
# here; sync-agents.sh has the twin — tracked divergence, see LEARNINGS on twins).
yaml_section(){  # $1=key ; reads stdin -> the dedented body under <key>:, '' when absent
  awk -v key="$1" '
    function ind(s,   m){ m=match(s, /[^[:space:]]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }
  '
}
runner_block(){  # $1=file -> the dedented body under runners.<RUNNER>., '' when absent
  [ -f "$1" ] || return 0
  yaml_section runners < "$1" | yaml_section "$RUNNER"
}

GLOBAL_CFG="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml"
seen_keys=" "
for f in "$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"; do
  blk="$(runner_block "$f")"
  [ -n "$blk" ] || continue
  while IFS= read -r line; do
    k="$(sed -nE 's/^[[:space:]]*([A-Za-z0-9._-]+)[[:space:]]*:.*/\1/p' <<<"$line")"
    [ -n "$k" ] || continue
    case "$seen_keys" in *" $k "*) continue ;; esac   # first (highest-precedence) layer wins per key
    seen_keys="$seen_keys$k "                          # claim the key for THIS layer before parsing its
                                                       # value, so a malformed high-precedence value still
                                                       # masks lower layers (precedence is per-key, not per-value)
    v="$(sed -nE 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p' <<<"$line")"
    [ -n "$v" ] || continue
    uk="$(tr '[:lower:]' '[:upper:]' <<<"$k" | tr '.-' '__')"
    export "DOCKET_RUNNER_CFG_$uk=$v"
  done <<<"$blk"
done

# --- handoff: foreground, adapter owns everything child-specific --------------------
args=( --agent "$AGENT" )
[ -n "$MODEL" ]  && args+=( --model "$MODEL" )
[ -n "$EFFORT" ] && args+=( --effort "$EFFORT" )
exec bash "$ADAPTER" "${args[@]}" -- "$@"
