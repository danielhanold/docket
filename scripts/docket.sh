#!/bin/sh
# scripts/docket.sh — the one executable docket facade (change 0068). Its public entry is a POSIX
# bootstrap so even a host with an unsupported default Bash can fail cleanly or hand off to the
# configured runtime before any Bash-4-specific code is parsed. The implementation remains a
# finite table of named
# operations; NO run/exec/shell/eval escape hatch; NEVER evaluates, sources, or executes
# caller-supplied shell text. Config flows model-ward: `env`/`preflight` print raw KEY=value on
# stdout for the model to read as literals; `bootstrap`'s stdout is the resolver's default %q
# shell format, but no agent is meant to eval or source that either (re-run `preflight` for the
# readable block). The subcommand table below (and in scripts/docket.md) IS the permission inventory.
#
# Usage: docket.sh <operation> [args...]
#   preflight                 Step-0 side effects (sync the metadata worktree), then print env
#   bootstrap                 guarded CREATE_ORPHAN orphan-`docket` create (fresh repo, once, human-attended)
#   env                       print resolved KEY=value config (read-only)
#   docket-status [args]      the docket-status orchestrator
#   board-refresh [args]      gated BOARD.md writer
#   archive-change [args]     move a change to archive/
#   terminal-publish [args]   publish terminal records onto the integration branch
#   cleanup-feature-branch    delete a merged feature branch + worktree
#   github-mirror [args]      GitHub Issues/Projects mirror
#   sync-integration-branch   fast-forward the local integration branch
#   render-change-links       per-change Artifacts link block (pure renderer)
#   render-adr-index          ADR index (pure renderer)
#   render-learnings-index    learnings index (pure renderer)
#   adr-checks [args]         ADR consistency checks
#   board-checks [args]       board consistency checks
#   reclaim-claims [args]     reclaim expired-lease, no-branch in-progress claims back to proposed
#   mint-stub [args]          mint one discovered-work stub (auto-capture; CAS-correct)
#   mark-publish-deferred [args]  add/remove the `## Publish deferred` marker on a change file
#   runner-dispatch [args]    delegate one agent run to a child harness (runner adapter)
#
# Contract: scripts/docket.md. Mock seams: SCRIPTS_DIR (helper dir), GIT, CONFIG_EXPORT_CMD.

# The bootstrap supplies the implementation marker as bash -c's $0. It cannot collide with a
# caller argument, so an operation named like the marker cannot bypass interpreter selection.
if [ "$0" != docket-bash-runtime ]; then
  _docket_runtime_remedy='run docket/install.sh after installing Bash 4+ (on macOS: brew install bash)'
  if [ -z "${DOCKET_BASH_PATH:-}" ]; then
    printf 'docket: runtime.bash is not configured — %s\n' "$_docket_runtime_remedy" >&2
    exit 1
  fi
  case "$DOCKET_BASH_PATH" in
    /*) ;;
    *) printf "docket: runtime.bash must be an absolute path, got '%s' — %s\n" "$DOCKET_BASH_PATH" "$_docket_runtime_remedy" >&2; exit 1 ;;
  esac
  if [ ! -x "$DOCKET_BASH_PATH" ]; then
    printf 'docket: runtime.bash is not an executable file: %s — %s\n' "$DOCKET_BASH_PATH" "$_docket_runtime_remedy" >&2
    exit 1
  fi
  _docket_runtime_version="$(LC_ALL=C "$DOCKET_BASH_PATH" --version 2>/dev/null)" || {
    printf 'docket: runtime.bash could not report its version: %s — %s\n' "$DOCKET_BASH_PATH" "$_docket_runtime_remedy" >&2
    exit 1
  }
  _docket_runtime_first="$(printf '%s\n' "$_docket_runtime_version" | sed -n '1p')"
  case "$_docket_runtime_first" in
    'GNU bash, version '*) ;;
    *) printf "docket: runtime.bash did not identify itself as GNU Bash: %s reported '%s' — %s\n" "$DOCKET_BASH_PATH" "${_docket_runtime_first:-no version}" "$_docket_runtime_remedy" >&2; exit 1 ;;
  esac
  _docket_runtime_major="$(printf '%s\n' "$_docket_runtime_first" | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')"
  case "$_docket_runtime_major" in
    ''|*[!0-9]*) _docket_runtime_major=0 ;;
  esac
  if [ "$_docket_runtime_major" -lt 4 ]; then
    printf "docket: runtime.bash must be Bash 4 or newer, got '%s' from %s — %s\n" "$_docket_runtime_first" "$DOCKET_BASH_PATH" "$_docket_runtime_remedy" >&2
    exit 1
  fi
  exec "$DOCKET_BASH_PATH" -c '_docket_script=$1; shift; . "$_docket_script"' \
    docket-bash-runtime "$0" "$@"
fi

set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"
GIT="${GIT:-git}"
# shellcheck source=lib/docket-preflight.sh
. "$SELF_DIR"/lib/docket-preflight.sh

# The exposed wrapped-helper operations (op name == helper basename). Single source of the
# dispatch allowlist; the sentinel test greps THIS array and the docket.md table.
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks reclaim-claims mint-stub runner-dispatch mark-publish-deferred"

usage(){ sed -n '/^# Usage:/,/^# Contract:/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }
reject(){ printf 'docket: unknown operation: %s\n' "${1:-<none>}" >&2; printf 'supported operations: preflight env bootstrap %s\n' "$WRAPPED_OPS" >&2; exit 2; }

op="${1:-}"; [ $# -gt 0 ] && shift
case "$op" in
  -h|--help) usage; exit 0 ;;
  "" ) reject "" ;;
  env)
    exec "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  preflight)
    docket_preflight "$SCRIPTS_DIR" || exit 1
    exec "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  bootstrap)
    exec "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/docket-config.sh --bootstrap "$@" ;;
  *)
    for _o in $WRAPPED_OPS; do
      if [ "$op" = "$_o" ]; then exec "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/"$op".sh "$@"; fi
    done
    reject "$op" ;;
esac
