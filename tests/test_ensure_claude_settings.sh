#!/usr/bin/env bash
# tests/test_ensure_claude_settings.sh — hermetic tests for scripts/ensure-claude-settings.sh
# (change 0027). Run: bash tests/test_ensure_claude_settings.sh
# Env-seam cases need no network; one bare-origin fixture exercises the real docket-config.sh path.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/ensure-claude-settings.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

RULE_MAIN='Bash(git -C * push origin HEAD:main)'
RULE_DEV='Bash(git -C * push origin HEAD:develop)'

# jq helpers (keep nested quoting out of the assert conditions)
has_rule(){   jq -e --arg r "$2" '(.permissions.allow // []) | index($r)' "$1" >/dev/null 2>&1; }
rule_count(){ jq --arg r "$2" '[(.permissions.allow // [])[] | select(. == $r)] | length' "$1"; }
has_key(){    jq -e --arg k "$2" 'has($k)' "$1" >/dev/null 2>&1; }

# plain git repo (one commit, no origin) — for the env-seam cases
mkgit(){
  local d="$1"; mkdir -p "$d"; git -C "$d" init --quiet
  git -C "$d" config user.email t@t.test; git -C "$d" config user.name Test
  : > "$d/README.md"; git -C "$d" add README.md; git -C "$d" commit --quiet -m init
}
# bare-origin clone on main (for the real-resolver case); silence empty-clone warning (LEARNINGS #26)
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test; git -C "$dir" config user.name Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"; git -C "$dir" add README.md; git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
# run helper in <dir>; optional <branch> sets the DOCKET_INTEGRATION_BRANCH env seam
run(){
  local d="$1" br="${2:-}"
  if [ -n "$br" ]; then ( cd "$d" && DOCKET_INTEGRATION_BRANCH="$br" bash "$SCRIPT" )
  else ( cd "$d" && bash "$SCRIPT" ); fi
}

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- (1) create-when-absent: no .claude/ -> file created with the rule ----------
mkgit "$tmp/a"
run "$tmp/a" main >/dev/null
S="$tmp/a/.claude/settings.local.json"
assert "create: settings.local.json exists" '[ -f "$S" ]'
assert "create: rule present (HEAD:main)"   'has_rule "$S" "$RULE_MAIN"'

# --- (2) idempotent: second run adds no duplicate (count EXACTLY 1) -------------
run "$tmp/a" main >/dev/null
assert "idempotent: rule count is exactly 1" '[ "$(rule_count "$S" "$RULE_MAIN")" -eq 1 ]'

# --- (3) preserve existing keys + unrelated rule --------------------------------
mkgit "$tmp/b"
mkdir -p "$tmp/b/.claude"
cat > "$tmp/b/.claude/settings.local.json" <<'JSON'
{ "permissions": { "allow": ["Bash(ls)"] }, "env": { "KEEP": "1" } }
JSON
run "$tmp/b" main >/dev/null
SB="$tmp/b/.claude/settings.local.json"
assert "preserve: pre-existing rule kept"        'has_rule "$SB" "Bash(ls)"'
assert "preserve: unrelated top-level key kept"  'has_key  "$SB" "env"'
assert "preserve: new rule added"                'has_rule "$SB" "$RULE_MAIN"'

# --- (4) branch resolution via env seam: develop tail --------------------------
mkgit "$tmp/c"
run "$tmp/c" develop >/dev/null
SC="$tmp/c/.claude/settings.local.json"
assert "branch resolution: develop tail"        'has_rule "$SC" "$RULE_DEV"'
assert "branch resolution: no stray main rule"  '! has_rule "$SC" "$RULE_MAIN"'

# --- (5a) no git writes: helper makes no commit and stages nothing -------------
mkgit "$tmp/d"
before="$(git -C "$tmp/d" rev-parse HEAD)"
run "$tmp/d" main >/dev/null
after="$(git -C "$tmp/d" rev-parse HEAD)"
assert "no git writes: HEAD unchanged (no commit)" '[ "$before" = "$after" ]'
assert "no git writes: nothing staged"             'git -C "$tmp/d" diff --cached --quiet'

# --- (5b) the migrate gitignore entry string actually ignores the file ----------
mkgit "$tmp/e"
printf '.claude/settings.local.json\n' > "$tmp/e/.gitignore"
git -C "$tmp/e" add .gitignore; git -C "$tmp/e" commit --quiet -m ignore
run "$tmp/e" main >/dev/null
assert "gitignore string ignores settings.local.json" \
  '[ -z "$(git -C "$tmp/e" status --porcelain -- .claude/settings.local.json)" ]'

# --- (6) REAL resolver path: helper consults docket-config.sh (no env seam) ----
# main-mode + integration_branch: develop -> docket-config.sh emits develop, no ref needed,
# bootstrap guard skipped (main-mode). Proves the #26 wiring is real, not a vacuous seam.
mkrepo "$tmp/f"
printf 'metadata_branch: main\nintegration_branch: develop\n' > "$tmp/f/.docket.yml"
git -C "$tmp/f" add .docket.yml; git -C "$tmp/f" commit --quiet -m cfg
git -C "$tmp/f" push --quiet origin main
run "$tmp/f" >/dev/null            # NO env seam -> exercises scripts/docket-config.sh
SF="$tmp/f/.claude/settings.local.json"
assert "real resolver: develop tail from docket-config.sh" 'has_rule "$SF" "$RULE_DEV"'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
