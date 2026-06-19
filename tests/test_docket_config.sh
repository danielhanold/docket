#!/usr/bin/env bash
# tests/test_docket_config.sh — hermetic fixtures for scripts/docket-config.sh (change 0026).
# Run: bash tests/test_docket_config.sh   (no network; temp repos + bare origins)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# --- fixture builder: a clone with a bare origin -----------------------------
# mkrepo <dir> : create a bare origin + a working clone at <dir>, identity set,
#   one commit on `main` (origin/HEAD -> main). Echoes nothing; populates $dir.
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir"
  git -C "$dir" config user.email t@t.test
  git -C "$dir" config user.name  Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"
  git -C "$dir" add README.md
  git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
# run <dir> [args...] : run the resolver against <dir>, echo stdout
run(){ local d="$1"; shift; bash "$SCRIPT" --repo-dir "$d" "$@"; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- (A) absent .docket.yml -> all defaults (docket-mode) --------------------
mkrepo "$tmp/a"
out="$(run "$tmp/a" --export)"; eval "$out"
assert "absent cfg: METADATA_BRANCH default docket"    '[ "$METADATA_BRANCH" = docket ]'
assert "absent cfg: DOCKET_MODE docket"                '[ "$DOCKET_MODE" = docket ]'
assert "absent cfg: METADATA_WORKTREE .docket"         '[ "$METADATA_WORKTREE" = .docket ]'
assert "absent cfg: INTEGRATION_BRANCH auto->main"     '[ "$INTEGRATION_BRANCH" = main ]'
assert "absent cfg: DEFAULT_BRANCH main"               '[ "$DEFAULT_BRANCH" = main ]'
assert "absent cfg: CHANGES_DIR default"               '[ "$CHANGES_DIR" = docs/changes ]'
assert "absent cfg: ADRS_DIR default"                  '[ "$ADRS_DIR" = docs/adrs ]'
assert "absent cfg: RESULTS_DIR default"               '[ "$RESULTS_DIR" = docs/results ]'
assert "absent cfg: FINALIZE_GATE default local"       '[ "$FINALIZE_GATE" = local ]'
assert "absent cfg: FINALIZE_TEST_COMMAND empty"       '[ -z "$FINALIZE_TEST_COMMAND" ]'
assert "absent cfg: BOARD_SURFACES default inline"     '[ "$BOARD_SURFACES" = inline ]'
assert "absent cfg: AUTO_GROOM default false"          '[ "$AUTO_GROOM" = false ]'

# --- (B) main-mode pin -> METADATA_WORKTREE '.', BOOTSTRAP PROCEED -----------
mkrepo "$tmp/b"
cat > "$tmp/b/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/b" add .docket.yml; git -C "$tmp/b" commit --quiet -m cfg
git -C "$tmp/b" push --quiet origin main
out="$(run "$tmp/b" --export)"; eval "$out"
assert "main-mode: METADATA_BRANCH main"               '[ "$METADATA_BRANCH" = main ]'
assert "main-mode: DOCKET_MODE main"                   '[ "$DOCKET_MODE" = main ]'
assert "main-mode: METADATA_WORKTREE dot"              '[ "$METADATA_WORKTREE" = . ]'
assert "main-mode: BOOTSTRAP PROCEED"                  '[ "$BOOTSTRAP" = PROCEED ]'

# --- (C) explicit config (main-mode to skip bootstrap): dirs, gate, surfaces, escaping
mkrepo "$tmp/c"
cat > "$tmp/c/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: develop
changes_dir: planning/changes
adrs_dir: planning/adrs
results_dir: planning/results
auto_groom: true
board_surfaces: [inline, github]
finalize:
  gate: ci
  test_command: go test ./... -count=1
EOF
git -C "$tmp/c" add .docket.yml; git -C "$tmp/c" commit --quiet -m cfg
git -C "$tmp/c" push --quiet origin main
out="$(run "$tmp/c" --export)"; eval "$out"
assert "explicit: INTEGRATION_BRANCH verbatim develop" '[ "$INTEGRATION_BRANCH" = develop ]'
assert "explicit: CHANGES_DIR override"                '[ "$CHANGES_DIR" = planning/changes ]'
assert "explicit: ADRS_DIR override"                   '[ "$ADRS_DIR" = planning/adrs ]'
assert "explicit: RESULTS_DIR override"                '[ "$RESULTS_DIR" = planning/results ]'
assert "explicit: AUTO_GROOM true"                     '[ "$AUTO_GROOM" = true ]'
assert "explicit: FINALIZE_GATE ci"                    '[ "$FINALIZE_GATE" = ci ]'
assert "explicit: BOARD_SURFACES two (plurality)"      '[ "$BOARD_SURFACES" = "inline github" ]'
assert "explicit: FINALIZE_TEST_COMMAND w/ spaces"     '[ "$FINALIZE_TEST_COMMAND" = "go test ./... -count=1" ]'

# --- (D) board_surfaces: [] -> disabled (empty), distinct from unset ---------
mkrepo "$tmp/d"
printf 'metadata_branch: main\nboard_surfaces: []\n' > "$tmp/d/.docket.yml"
git -C "$tmp/d" add .docket.yml; git -C "$tmp/d" commit --quiet -m cfg
git -C "$tmp/d" push --quiet origin main
out="$(run "$tmp/d" --export)"; eval "$out"
assert "board []: BOARD_SURFACES empty"                '[ -z "$BOARD_SURFACES" ]'

# --- (E) direct-pipe caller (LEARNINGS #22: $() hides a dropped trailing \n) -
n="$(run "$tmp/c" --export | grep -c '=')"
assert "direct-pipe: 13 KEY=value lines emitted"       '[ "$n" -eq 13 ]'
last="$(run "$tmp/c" --export | tail -n1)"
assert "direct-pipe: last line is BOOTSTRAP"           'case "$last" in BOOTSTRAP=*) true;; *) false;; esac'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
