#!/usr/bin/env bash
# tests/test_setup_auto_approve.sh — hermetic tests for scripts/setup-auto-approve.sh (change
# 0062). Real local git (bare origin + clone); gh stubbed via the GH seam. No network.
# Run: bash tests/test_setup_auto_approve.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$ROOT/scripts/setup-auto-approve.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- a gh stub that records calls and serves a fixed actions-permissions payload -------------
ghstub="$tmp/ghbin"; mkdir -p "$ghstub"
cat > "$ghstub/gh" <<'STUB'
#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
case "$1 $2" in
  "api -X")  # a PUT — record and succeed
    exit 0 ;;
esac
# a plain `gh api repos/.../actions/permissions/workflow` GET: return current settings
if [ "$1" = "api" ] && printf '%s' "$*" | grep -q "permissions/workflow"; then
  echo '{"default_workflow_permissions":"read","can_approve_pull_request_reviews":false}'
  exit 0
fi
if [ "$1" = "repo" ] && [ "$2" = "view" ]; then echo "acme/widget"; exit 0; fi
exit 0
STUB
chmod +x "$ghstub/gh"

# --- a bare origin + clone with main + a docket orphan ---------------------------------------
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test; git -C "$dir" config user.name Test
  git -C "$dir" checkout --quiet -b main; : > "$dir/README.md"
  git -C "$dir" add README.md; git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
mkrepo "$tmp/r"
export GH_LOG="$tmp/gh.log"; : > "$GH_LOG"
runsetup(){ ( cd "$tmp/r" && GH="$ghstub/gh" GH_LOG="$GH_LOG" bash "$SCRIPT" "$@" ); }

# (A) installs the workflow file onto the integration branch (pushed to origin/main)
out="$(runsetup --integration-branch main 2>&1)"; rc=$?
assert "setup exits 0" '[ "$rc" -eq 0 ]'
assert "workflow landed on origin/main" \
  'git -C "$tmp/r" ls-tree -r --name-only origin/main | grep -qx ".github/workflows/docket-approve.yml"'

# (B) read-modify-write: PUT preserves default_workflow_permissions=read, sets approve=true
assert "PUT sends can_approve_pull_request_reviews=true" 'grep -q "can_approve_pull_request_reviews=true" "$GH_LOG"'
assert "PUT preserves default_workflow_permissions=read" 'grep -q "default_workflow_permissions=read" "$GH_LOG"'

# (C) prints the reminder to set finalize.auto_approve in .docket.yml
assert "reminds about finalize.auto_approve knob" 'printf "%s" "$out" | grep -q "finalize.auto_approve"'

# (D) idempotent: a second run still exits 0 and leaves exactly one workflow file
out2="$(runsetup --integration-branch main 2>&1)"; rc2=$?
assert "second run idempotent (exit 0)" '[ "$rc2" -eq 0 ]'
assert "still exactly one workflow file" \
  '[ "$(git -C "$tmp/r" ls-tree -r --name-only origin/main | grep -c "docket-approve.yml")" -eq 1 ]'

# (E) leaves no leftover setup worktree
assert "no leftover setup worktree" '! git -C "$tmp/r" worktree list | grep -q "setup-approve"'

exit $fail
