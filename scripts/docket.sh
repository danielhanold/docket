#!/usr/bin/env bash
# scripts/docket.sh — the one executable docket facade (change 0068). A finite table of named
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
#   adr-checks [args]         ADR consistency checks
#   board-checks [args]       board consistency checks
#   runner-dispatch [args]    delegate one agent run to a child harness (runner adapter)
#
# Contract: scripts/docket.md. Mock seams: SCRIPTS_DIR (helper dir), GIT, CONFIG_EXPORT_CMD.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"
GIT="${GIT:-git}"
# shellcheck source=lib/docket-preflight.sh
. "$SELF_DIR"/lib/docket-preflight.sh

# The exposed wrapped-helper operations (op name == helper basename). Single source of the
# dispatch allowlist; the sentinel test greps THIS array and the docket.md table.
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index adr-checks board-checks runner-dispatch"

usage(){ sed -n '/^# Usage:/,/^# Contract:/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }
reject(){ printf 'docket: unknown operation: %s\n' "${1:-<none>}" >&2; printf 'supported operations: preflight env bootstrap %s\n' "$WRAPPED_OPS" >&2; exit 2; }

op="${1:-}"; [ $# -gt 0 ] && shift
case "$op" in
  -h|--help) usage; exit 0 ;;
  "" ) reject "" ;;
  env)
    exec "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  preflight)
    docket_preflight "$SELF_DIR" || exit 1
    exec "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  bootstrap)
    exec "$SCRIPTS_DIR"/docket-config.sh --bootstrap "$@" ;;
  *)
    for _o in $WRAPPED_OPS; do
      if [ "$op" = "$_o" ]; then exec "$SCRIPTS_DIR"/"$op".sh "$@"; fi
    done
    reject "$op" ;;
esac
