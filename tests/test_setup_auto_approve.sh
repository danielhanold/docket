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
# a plain `gh api repos/.../actions/permissions/workflow` GET: return current settings.
# NON-default on purpose ("write", not the script's own fallback "read"): a regression that
# blind-sets "read" and ignores the read would still pass if this stub echoed the fallback value
# back, so the read-modify-write guarantee needs a GET value the fallback cannot accidentally match.
if [ "$1" = "api" ] && printf '%s' "$*" | grep -q "permissions/workflow"; then
  echo '{"default_workflow_permissions":"write","can_approve_pull_request_reviews":false}'
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

# (B) read-modify-write: PUT preserves the READ value (non-default "write"), sets approve=true.
# The stub's GET returns "write" (not the script's own fallback "read"), so this assertion only
# passes if the script actually read and re-sent the existing value — a regression that blind-sets
# "read" and ignores the read would fail this (see RED/GREEN evidence in the task-3 report).
assert "PUT sends can_approve_pull_request_reviews=true" 'grep -q "can_approve_pull_request_reviews=true" "$GH_LOG"'
assert "PUT preserves default_workflow_permissions=write (read, not blind-set)" \
  'grep -q "default_workflow_permissions=write" "$GH_LOG"'

# (C) prints the reminder to set finalize.auto_approve in .docket.yml
assert "reminds about finalize.auto_approve knob" 'printf "%s" "$out" | grep -q "finalize.auto_approve"'

# (D) idempotent: a second run still exits 0 and leaves exactly one workflow file
out2="$(runsetup --integration-branch main 2>&1)"; rc2=$?
assert "second run idempotent (exit 0)" '[ "$rc2" -eq 0 ]'
assert "still exactly one workflow file" \
  '[ "$(git -C "$tmp/r" ls-tree -r --name-only origin/main | grep -c "docket-approve.yml")" -eq 1 ]'

# (E) leaves no leftover setup worktree
assert "no leftover setup worktree" '! git -C "$tmp/r" worktree list | grep -q "setup-approve"'

# --- (F) workflow-OAuth-scope push-rejection hint: a GIT wrapper that passes through to real git
# for everything except `push` (matched as a token anywhere in the args, so `-C DIR push ...`
# still trips it), where it emits a stderr line mentioning "workflow" and fails — hermetically
# exercising the script's targeted re-auth hint without needing real HTTPS auth or network.
gitreject="$tmp/git-push-reject.sh"
cat > "$gitreject" <<'STUB'
#!/usr/bin/env bash
for a in "$@"; do
  if [ "$a" = "push" ]; then
    echo "remote: refusing to allow an OAuth App to create or update workflow \`.github/workflows/docket-approve.yml\` without \`workflow\` scope" >&2
    exit 1
  fi
done
exec git "$@"
STUB
chmod +x "$gitreject"
mkrepo "$tmp/pr"
out3="$(cd "$tmp/pr" && GIT="$gitreject" GH="$ghstub/gh" GH_LOG="$tmp/gh-pr.log" bash "$SCRIPT" --integration-branch main 2>&1)"; rc3=$?
assert "workflow-scope push rejection exits non-zero" '[ "$rc3" -ne 0 ]'
assert "workflow-scope push rejection output mentions workflow" \
  'printf "%s" "$out3" | grep -qi "workflow"'
# NOTE: this must grep for wording the SCRIPT itself adds, not wording already present in the
# raw git/GitHub stderr our stub emits (that stderr already happens to contain "workflow" near
# "scope") — otherwise a regression that drops the script's own hint and falls through to the
# generic "push ... failed: <raw stderr>" branch would still false-pass this assertion.
assert "workflow-scope push rejection surfaces the script's own re-auth/SSH guidance" \
  'printf "%s" "$out3" | grep -Eqi "gh auth refresh|ssh remote"'
assert "workflow-scope push rejection leaves no leftover setup worktree" \
  '! git -C "$tmp/pr" worktree list | grep -q "setup-approve"'

# --- (G) self-heals from a leftover setup-approve worktree/branch left by an interrupted prior
# run: manually provision the fixed-name worktree/branch before invoking setup, then assert the
# run still exits 0, installs the workflow, and leaves no leftover worktree afterward (proving it
# self-heals rather than wedging on a fixed `git worktree add -B` name/path collision).
mkrepo "$tmp/heal"
git -C "$tmp/heal" worktree add -B setup-approve "$tmp/heal/.setup-approve-wt" origin/main >/dev/null 2>&1
out4="$(cd "$tmp/heal" && GH="$ghstub/gh" GH_LOG="$tmp/gh-heal.log" bash "$SCRIPT" --integration-branch main 2>&1)"; rc4=$?
assert "leftover-worktree run self-heals (exits 0)" '[ "$rc4" -eq 0 ]'
assert "leftover-worktree run installs workflow" \
  'git -C "$tmp/heal" ls-tree -r --name-only origin/main | grep -qx ".github/workflows/docket-approve.yml"'
assert "leftover-worktree run leaves no leftover setup worktree" \
  '! git -C "$tmp/heal" worktree list | grep -q "setup-approve"'

# --- (H) default-branch resolution: omit --integration-branch entirely, so the script must
# resolve it from origin/HEAD (mkrepo already ran `git remote set-head origin -a`, pointing
# origin/HEAD at main) — the every-other-test default of passing --integration-branch main
# explicitly never exercises this path.
mkrepo "$tmp/def"
out5="$(cd "$tmp/def" && GH="$ghstub/gh" GH_LOG="$tmp/gh-def.log" bash "$SCRIPT" 2>&1)"; rc5=$?
assert "default-integration-branch run exits 0" '[ "$rc5" -eq 0 ]'
assert "default-integration-branch run lands workflow on origin/main" \
  'git -C "$tmp/def" ls-tree -r --name-only origin/main | grep -qx ".github/workflows/docket-approve.yml"'

exit $fail
