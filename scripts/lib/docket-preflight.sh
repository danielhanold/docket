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

# shellcheck source=docket-root.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docket-root.sh"

docket_preflight(){
  local scripts_dir="$1"
  local git="${GIT:-git}"
  local cfg
  if [ -n "${CONFIG_EXPORT_CMD:-}" ]; then
    cfg="$($CONFIG_EXPORT_CMD)" \
      || { echo "docket-preflight: config export failed" >&2; return 1; }
  else
    cfg="$("$DOCKET_BASH_PATH" "$scripts_dir"/docket-config.sh --export)" \
      || { echo "docket-preflight: config export failed" >&2; return 1; }
  fi
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  echo "docket-preflight: repo not migrated — run migrate-to-docket.sh" >&2; return 1 ;;
    CREATE_ORPHAN) echo "docket-preflight: fresh repo — bootstrap is opt-in; run docket.sh bootstrap (or a docket skill) to create the docket branch" >&2; return 1 ;;
    *) echo "docket-preflight: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; return 1 ;;
  esac

  # --- repo anchor (change 0075, defect D2) ------------------------------------------------
  # The eval'd SHELL format keeps METADATA_WORKTREE relative (".docket" / "."), and git — plus the
  # -d test below — would resolve that against the CALLER's CWD. Run from <repo>/.docket that
  # created a real <repo>/.docket/.docket worktree and still exited 0 (observed live, change 0073).
  # Anchor the path to the MAIN worktree before anything touches it. Not a git repo => leave the
  # value alone and let the git calls below fail exactly as they did before.
  local root
  root="$(docket_main_worktree)"
  METADATA_WORKTREE="$(docket_anchor_path "${METADATA_WORKTREE:-}")"

  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    local gitc="${root:-.}"
    if [ ! -d "$wt" ]; then
      # Fail-closed guard (change 0075): the metadata worktree must never land INSIDE a LINKED
      # worktree of this repo. The MAIN worktree legitimately contains it (<root>/.docket), so the
      # main worktree — the first entry of `worktree list` — is excluded; every other entry is a
      # linked worktree, and <repo>/.docket/.docket is never a legitimate target. Without this, a
      # caller that hands preflight a bad path silently mints debris that only `git worktree list`
      # reveals.
      if _docket_target_inside_linked_worktree "$git" "$gitc" "$wt"; then
        echo "docket-preflight: refusing to create metadata worktree at $wt — it is inside an existing worktree of this repo" >&2
        return 1
      fi
      "$git" -C "$gitc" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$git" -C "$gitc" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-preflight: cannot create metadata worktree $wt" >&2; return 1; }
    fi
    # change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent;
    # self-heals existing installs). Best-effort — a failure here must not block preflight.
    "$DOCKET_BASH_PATH" "$scripts_dir"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-preflight: warning — could not disable hooks on $wt (continuing)" >&2
    "$git" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$git" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-preflight: metadata worktree sync failed" >&2; return 1; }
  else
    "$git" -C "${root:-.}" pull --rebase >&2 || { echo "docket-preflight: metadata sync failed" >&2; return 1; }
  fi
}

# _docket_target_inside_linked_worktree <git> <repo-dir> <target> — true (0) when <target> lies at
# or inside a LINKED worktree of the repo at <repo-dir>. The MAIN worktree (the first entry of
# `git worktree list --porcelain`) is deliberately EXCLUDED: it is the one worktree that
# legitimately contains the metadata worktree. Every other entry is a linked worktree, and a
# metadata worktree inside one of those is the D2 shape.
_docket_target_inside_linked_worktree(){
  local git="$1" repo_dir="$2" target="$3" first=1 wt
  while IFS= read -r wt; do
    [ -n "$wt" ] || continue
    if [ "$first" = 1 ]; then first=0; continue; fi   # skip the main worktree
    case "$target/" in
      "$wt/"*) return 0 ;;
    esac
  done < <("$git" -C "$repo_dir" worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')
  return 1
}
