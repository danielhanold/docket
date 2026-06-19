#!/usr/bin/env bash
# scripts/ensure-claude-settings.sh — grant docket's terminal-publish push to the integration
# branch, per-repo and LOCALLY, by writing one Claude Code allow-rule into the repo's
# <repo>/.claude/settings.local.json (change 0027). Idempotent + standalone-runnable: a fresh
# cloner of an already-migrated repo can run this directly to grant themselves the rule (the
# file is gitignored and per-user, so migrate's one-time run does not cover later clones).
#
# The rule pre-authorizes ONLY terminal-publish's exact command shape —
#   git -C <transient-worktree> push origin HEAD:<integration_branch>
# (force-push and other-branch pushes stay guarded). It mirrors the real command, so no
# skill/convention edit is needed.
#
# Usage: ensure-claude-settings.sh      # operate on the repo containing $PWD
#   The integration branch is resolved via the sibling scripts/docket-config.sh (change 0026),
#   unless DOCKET_INTEGRATION_BRANCH is set (manual override + hermetic test seam).
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"   # this script's dir -> sibling docket-config.sh
die() { printf 'ensure-claude-settings: %s\n' "$*" >&2; exit 1; }

# --- resolve the TARGET repo root from $PWD (like migrate-to-docket.sh) ---
REPO_ROOT="$("$GIT" rev-parse --show-toplevel 2>/dev/null)" \
  || die "not inside a git repo — cd into the repo you want to grant, then re-run."

# --- resolve the integration branch (env override > docket-config.sh) ---
if [ -n "${DOCKET_INTEGRATION_BRANCH:-}" ]; then
  INTEGRATION_BRANCH="$DOCKET_INTEGRATION_BRANCH"
else
  cfg="$("$HERE/docket-config.sh" --export --repo-dir "$REPO_ROOT")" \
    || die "could not resolve config via docket-config.sh (is origin reachable?)"
  eval "$cfg"
  [ -n "${INTEGRATION_BRANCH:-}" ] || die "docket-config.sh did not report INTEGRATION_BRANCH"
fi

RULE="Bash(git -C * push origin HEAD:$INTEGRATION_BRANCH)"
SETTINGS_DIR="$REPO_ROOT/.claude"
SETTINGS="$SETTINGS_DIR/settings.local.json"

# --- ensure the local Claude config exists (create the whole thing if absent) ---
created=0
if [ ! -f "$SETTINGS" ]; then
  mkdir -p "$SETTINGS_DIR"
  printf '{}\n' > "$SETTINGS"
  created=1
fi

# Refuse to clobber a corrupt pre-existing file.
jq empty "$SETTINGS" 2>/dev/null || die "$SETTINGS is not valid JSON — fix or remove it, then re-run."

# Was the rule already present? (for the idempotency report only)
already=0
if jq -e --arg r "$RULE" '(.permissions.allow // []) | index($r)' "$SETTINGS" >/dev/null 2>&1; then
  already=1
fi

# --- idempotently merge the rule, preserving every existing key/rule + order ---
tmp="$(mktemp)"
if jq --arg rule "$RULE" '
      .permissions //= {}
      | .permissions.allow //= []
      | if (.permissions.allow | index($rule)) == null
        then .permissions.allow += [$rule]
        else .
        end
    ' "$SETTINGS" > "$tmp"; then
  mv "$tmp" "$SETTINGS"
else
  rm -f "$tmp"; die "failed to update $SETTINGS"
fi

# --- one-line summary ---
rel="${SETTINGS#"$REPO_ROOT"/}"
if [ "$already" -eq 1 ]; then
  printf 'ensure-claude-settings: rule already present in %s — no change.\n' "$rel"
elif [ "$created" -eq 1 ]; then
  printf 'ensure-claude-settings: created %s and granted: %s\n' "$rel" "$RULE"
else
  printf 'ensure-claude-settings: added grant to %s: %s\n' "$rel" "$RULE"
fi
