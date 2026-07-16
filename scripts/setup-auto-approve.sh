#!/usr/bin/env bash
# scripts/setup-auto-approve.sh — one-time, HUMAN-ATTENDED setup for finalize's auto-approve
# (change 0062). (1) Installs scripts/templates/docket-approve.yml onto the integration branch as
# .github/workflows/docket-approve.yml (direct admin push — same posture as terminal-publish);
# (2) flips the repo Actions setting can_approve_pull_request_reviews=true via `gh api` PUT,
# preserving default_workflow_permissions (read-modify-write, never blind-set); (3) prints what it
# changed and reminds the human to set finalize.auto_approve: true in .docket.yml.
# NEVER invoked by an autonomous skill. Idempotent. Contract: scripts/setup-auto-approve.md.
# Mock seams: GIT, GH.
set -uo pipefail

GIT="${GIT:-git}"
GH="${GH:-gh}"
REMOTE="origin"
INT_BRANCH=""
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="$SELF_DIR/templates/docket-approve.yml"

die(){ printf 'setup-auto-approve: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --integration-branch) [ $# -ge 2 ] || die "--integration-branch needs an arg"; INT_BRANCH="$2"; shift ;;
    --remote)             [ $# -ge 2 ] || die "--remote needs an arg"; REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

[ -f "$TEMPLATE" ] || die "template not found: $TEMPLATE (broken install?)"
$GIT rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repo"

# Resolve the integration branch from origin/HEAD when not given.
if [ -z "$INT_BRANCH" ]; then
  $GIT remote set-head "$REMOTE" -a >/dev/null 2>&1 || true
  INT_BRANCH="$($GIT symbolic-ref --quiet --short "refs/remotes/$REMOTE/HEAD" 2>/dev/null | sed "s#^$REMOTE/##")"
  [ -n "$INT_BRANCH" ] || die "could not resolve integration branch from $REMOTE/HEAD — pass --integration-branch"
fi

$GIT fetch "$REMOTE" "$INT_BRANCH" >/dev/null 2>&1 || die "fetch $REMOTE/$INT_BRANCH failed"

# --- (1) install the workflow onto the integration branch via a transient worktree -----------
pub="$($GIT rev-parse --show-toplevel)/.setup-approve-wt"
teardown(){
  $GIT -C "$pub" checkout --detach >/dev/null 2>&1 || true
  $GIT worktree remove --force "$pub" >/dev/null 2>&1 || true
  $GIT branch -D setup-approve >/dev/null 2>&1 || true
}
$GIT worktree prune
$GIT worktree add -B setup-approve "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision setup-approve worktree"
# Skip the team's shared hooks on docket's own asset commit (best-effort).
"$SELF_DIR/disable-worktree-hooks.sh" --worktree "$pub" >/dev/null 2>&1 || true

mkdir -p "$pub/.github/workflows" || { teardown; die "mkdir .github/workflows failed"; }
cp "$TEMPLATE" "$pub/.github/workflows/docket-approve.yml" || { teardown; die "copy template failed"; }
$GIT -C "$pub" add .github/workflows/docket-approve.yml

if $GIT -C "$pub" diff --cached --quiet; then
  echo "setup-auto-approve: workflow already up to date on $INT_BRANCH (no commit needed)"
else
  $GIT -C "$pub" commit -m "chore(docket): install docket-approve.yml auto-approve workflow" >/dev/null \
    || { teardown; die "commit failed"; }
  # Push HEAD explicitly; surface the workflow-scope caveat on rejection (HTTPS token auth needs it).
  if ! $GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH" 2>"$pub/.push.err"; then
    if grep -qi "workflow" "$pub/.push.err"; then
      teardown
      die "push rejected — pushing .github/workflows/ over HTTPS needs the 'workflow' OAuth scope; re-auth with that scope (gh auth refresh -s workflow) or use an SSH remote, then re-run"
    fi
    teardown; die "push to $REMOTE/$INT_BRANCH failed: $(cat "$pub/.push.err")"
  fi
  echo "setup-auto-approve: installed .github/workflows/docket-approve.yml on $INT_BRANCH"
fi
teardown
$GIT worktree prune

# --- (2) flip the repo Actions setting (read-modify-write) ------------------------------------
slug="$($GH repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || $GH repo view 2>/dev/null | head -n1)"
[ -n "$slug" ] || die "could not resolve owner/repo via gh"
cur="$($GH api "repos/$slug/actions/permissions/workflow" 2>/dev/null)" \
  || die "could not read Actions permissions (need repo admin + a token with 'repo' scope)"
dwp="$(printf '%s' "$cur" | sed -n 's/.*"default_workflow_permissions":"\([^"]*\)".*/\1/p')"
dwp="${dwp:-read}"
$GH api -X PUT "repos/$slug/actions/permissions/workflow" \
  -f "default_workflow_permissions=$dwp" \
  -F "can_approve_pull_request_reviews=true" >/dev/null \
  || die "could not set can_approve_pull_request_reviews (org policy may override the repo setting)"
echo "setup-auto-approve: set can_approve_pull_request_reviews=true on $slug (default_workflow_permissions=$dwp preserved)"

# --- (3) reminder ----------------------------------------------------------------------------
cat <<EOF
setup-auto-approve: done. Next:
  - Set 'finalize.auto_approve: true' in this repo's committed .docket.yml (this script never edits committed config).
  - Verify: gh api repos/$slug/actions/permissions/workflow
EOF
