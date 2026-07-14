#!/usr/bin/env bash
# scripts/lib/docket-preflight.sh — the shared Step-0 preflight (change 0068). Sourced by
# scripts/docket.sh and scripts/docket-status.sh; extracts the metadata-worktree sync that was
# docket-status.sh's private ensure_and_sync_worktree so there is ONE sync implementation.
#
# docket_preflight <scripts_dir>
#   1. resolve config: eval "$(${CONFIG_EXPORT_CMD:-<scripts_dir>/docket-config.sh --export})"
#      into the CALLER's scope (DOCKET_MODE, METADATA_BRANCH, METADATA_WORKTREE, BOOTSTRAP, …).
#   2. enforce the bootstrap verdict fail-closed (non-PROCEED => return 1 + stderr diagnostic).
#   3. ensure + sync the metadata worktree (docket-mode) or the primary tree (main-mode);
#      disable the metadata worktree's shared git hooks (best-effort, change 0063).
#   Returns 0 on success. Prints nothing on stdout. Honors the GIT and CONFIG_EXPORT_CMD seams.
# This file is a sourced helper: it is documented within its callers' contracts (docket.md,
# docket-status.md), not by a co-located .md (test_script_contracts_coverage.sh scopes lib/ out).

docket_preflight(){
  local scripts_dir="$1"
  local git="${GIT:-git}"
  local cfg
  cfg="$(${CONFIG_EXPORT_CMD:-"$scripts_dir"/docket-config.sh --export})" \
    || { echo "docket-preflight: config export failed" >&2; return 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  echo "docket-preflight: repo not migrated — run migrate-to-docket.sh" >&2; return 1 ;;
    CREATE_ORPHAN) echo "docket-preflight: fresh repo — bootstrap is opt-in; run docket-config.sh --bootstrap (or a docket skill) to create the docket branch" >&2; return 1 ;;
    *) echo "docket-preflight: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; return 1 ;;
  esac

  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    if [ ! -d "$wt" ]; then
      "$git" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$git" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-preflight: cannot create metadata worktree $wt" >&2; return 1; }
    fi
    # change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent;
    # self-heals existing installs). Best-effort — a failure here must not block preflight.
    "$scripts_dir"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-preflight: warning — could not disable hooks on $wt (continuing)" >&2
    "$git" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$git" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-preflight: metadata worktree sync failed" >&2; return 1; }
  else
    "$git" pull --rebase >&2 || { echo "docket-preflight: metadata sync failed" >&2; return 1; }
  fi
}
