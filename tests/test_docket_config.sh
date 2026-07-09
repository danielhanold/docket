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
  git clone --quiet "$bare" "$dir" 2>/dev/null
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

# Hermetic: never read the dev machine's real global config (change 0050 — docket-config.sh
# now reads ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml). Point XDG at a void.
export XDG_CONFIG_HOME="$tmp/xdg-void"
# rung <xdgdir> <repodir> [args...] : run the resolver with the global layer rooted at <xdgdir>
rung(){ local x="$1" d="$2"; shift 2; XDG_CONFIG_HOME="$x" bash "$SCRIPT" --repo-dir "$d" "$@"; }
rung_rc(){ local x="$1" d="$2"; shift 2; XDG_CONFIG_HOME="$x" bash "$SCRIPT" --repo-dir "$d" "$@" >/dev/null 2>&1; echo $?; }

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
assert "direct-pipe: 18 KEY=value lines emitted"       '[ "$n" -eq 18 ]'
last="$(run "$tmp/c" --export | tail -n1)"
assert "direct-pipe: last line is BOOTSTRAP"           'case "$last" in BOOTSTRAP=*) true;; *) false;; esac'

# --- bootstrap 2×2 fixtures (docket-mode; mkrepo leaves origin/main = README only) ---
# seed_live <dir> : put the live planning surface on origin/main (=> LIVE=1)
seed_live(){
  local d="$1"
  mkdir -p "$d/docs/changes/active"
  : > "$d/docs/changes/active/0001-x.md"
  : > "$d/docs/changes/README.md"
  : > "$d/docs/changes/BOARD.md"
  git -C "$d" add docs; git -C "$d" commit --quiet -m live
  git -C "$d" push --quiet origin main
}
# make_docket <dir> : create an empty origin/docket (=> DOCKET=1) without a local branch
make_docket(){
  local d="$1" t c
  t="$(git -C "$d" mktree </dev/null)"
  c="$(git -C "$d" commit-tree "$t" -m seed)"
  git -C "$d" push --quiet origin "$c:refs/heads/docket"
  git -C "$d" fetch --quiet origin docket
}

# (B1) migrated: DOCKET ∧ ¬LIVE -> PROCEED
mkrepo "$tmp/b1"; make_docket "$tmp/b1"
out="$(run "$tmp/b1" --export)"; eval "$out"
assert "2x2 migrated -> PROCEED"            '[ "$BOOTSTRAP" = PROCEED ]'

# (B2) fresh: ¬DOCKET ∧ ¬LIVE -> CREATE_ORPHAN
mkrepo "$tmp/b2"
out="$(run "$tmp/b2" --export)"; eval "$out"
assert "2x2 fresh -> CREATE_ORPHAN"         '[ "$BOOTSTRAP" = CREATE_ORPHAN ]'

# (B3) existing single-branch: ¬DOCKET ∧ LIVE -> STOP_MIGRATE
mkrepo "$tmp/b3"; seed_live "$tmp/b3"
out="$(run "$tmp/b3" --export)"; eval "$out"
assert "2x2 single-branch -> STOP_MIGRATE"  '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# (B4) half-migrated: DOCKET ∧ LIVE -> STOP_MIGRATE
mkrepo "$tmp/b4"; seed_live "$tmp/b4"; make_docket "$tmp/b4"
out="$(run "$tmp/b4" --export)"; eval "$out"
assert "2x2 half-migrated -> STOP_MIGRATE"  '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# --- opt-in --bootstrap write (the only mutation; guarded to ¬DOCKET ∧ ¬LIVE) ---
origin_has_docket(){ git -C "$1.origin.git" rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1; }

# (W1) default --export in fresh cell: NO write, verdict CREATE_ORPHAN
mkrepo "$tmp/w1"
out="$(run "$tmp/w1" --export)"; eval "$out"
assert "read-only default: no orphan created" '! origin_has_docket "$tmp/w1"'
assert "read-only default: verdict CREATE_ORPHAN" '[ "$BOOTSTRAP" = CREATE_ORPHAN ]'

# (W2) --bootstrap in fresh cell: creates origin/docket, re-reports PROCEED
mkrepo "$tmp/w2"
out="$(run "$tmp/w2" --bootstrap --export)"; eval "$out"
assert "bootstrap fresh: origin/docket created" 'origin_has_docket "$tmp/w2"'
assert "bootstrap fresh: verdict now PROCEED"   '[ "$BOOTSTRAP" = PROCEED ]'

# (W3) --bootstrap in STOP_MIGRATE cell: GUARD holds — no orphan written
mkrepo "$tmp/w3"; seed_live "$tmp/w3"
out="$(run "$tmp/w3" --bootstrap --export)"; eval "$out"
assert "bootstrap guard: no write in single-branch cell" '! origin_has_docket "$tmp/w3"'
assert "bootstrap guard: verdict stays STOP_MIGRATE"     '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# (W4) --bootstrap in migrated cell: idempotent no-op, PROCEED (origin/docket SHA unchanged)
mkrepo "$tmp/w4"; make_docket "$tmp/w4"
w4_before="$(git -C "$tmp/w4.origin.git" rev-parse refs/heads/docket)"
out="$(run "$tmp/w4" --bootstrap --export)"; eval "$out"
w4_after="$(git -C "$tmp/w4.origin.git" rev-parse refs/heads/docket)"
assert "bootstrap migrated: PROCEED"            '[ "$BOOTSTRAP" = PROCEED ]'
assert "bootstrap migrated: origin/docket SHA unchanged (no-op)" '[ "$w4_before" = "$w4_after" ]'

# --- fail-closed error paths (non-zero exit, stderr diagnostic, no KEY=value) ----
run_rc(){ local d="$1"; shift; bash "$SCRIPT" --repo-dir "$d" "$@" >/dev/null 2>&1; echo $?; }

# (F1) unreachable origin -> exit≠0, no output
mkrepo "$tmp/f1"
rm -rf "$tmp/f1.origin.git"                       # destroy the remote
assert "unreachable origin: nonzero exit" '[ "$(run_rc "$tmp/f1" --export)" -ne 0 ]'
assert "unreachable origin: emits nothing" '[ -z "$(bash "$SCRIPT" --repo-dir "$tmp/f1" --export 2>/dev/null)" ]'

# (F2) cached-but-stale origin/HEAD must NOT mask an unreachable origin (keys on fetch rc,
#      not git show — LEARNINGS / spec §7). origin/HEAD + .docket.yml are cached locally,
#      so `git show origin/HEAD:.docket.yml` would still succeed with stale bytes.
mkrepo "$tmp/f2"
echo 'metadata_branch: docket' > "$tmp/f2/.docket.yml"
git -C "$tmp/f2" add .docket.yml; git -C "$tmp/f2" commit --quiet -m cfg
git -C "$tmp/f2" push --quiet origin main
git -C "$tmp/f2" fetch --quiet origin              # populate caches
rm -rf "$tmp/f2.origin.git"                         # now unreachable
assert "stale cache does not mask unreachable origin" '[ "$(run_rc "$tmp/f2" --export)" -ne 0 ]'
assert "stale cache: emits nothing" '[ -z "$(bash "$SCRIPT" --repo-dir "$tmp/f2" --export 2>/dev/null)" ]'

# (F3) integration ref absent (docket-mode) -> ls-tree rc≠0 -> hard error
mkrepo "$tmp/f3"
printf 'metadata_branch: docket\nintegration_branch: nope\n' > "$tmp/f3/.docket.yml"
git -C "$tmp/f3" add .docket.yml; git -C "$tmp/f3" commit --quiet -m cfg
git -C "$tmp/f3" push --quiet origin main
assert "absent integration ref: nonzero exit" '[ "$(run_rc "$tmp/f3" --export)" -ne 0 ]'
assert "absent integration ref: emits nothing" '[ -z "$(bash "$SCRIPT" --repo-dir "$tmp/f3" --export 2>/dev/null)" ]'

# (F4) bad metadata_branch -> unparseable -> hard error
mkrepo "$tmp/f4"
echo 'metadata_branch: banana' > "$tmp/f4/.docket.yml"
git -C "$tmp/f4" add .docket.yml; git -C "$tmp/f4" commit --quiet -m cfg
git -C "$tmp/f4" push --quiet origin main
assert "bad metadata_branch: nonzero exit" '[ "$(run_rc "$tmp/f4" --export)" -ne 0 ]'
err="$(bash "$SCRIPT" --repo-dir "$tmp/f4" --export 2>&1 >/dev/null)"
assert "bad metadata_branch: diagnostic mentions metadata_branch" 'printf "%s" "$err" | grep -q metadata_branch'

# (F5) --repo-dir with no following argument -> usage error (exit≠0 + diagnostic), no set -u crash
rc5=0; err5="$(bash "$SCRIPT" --repo-dir 2>&1 >/dev/null)" || rc5=$?
assert "F5 --repo-dir no arg: nonzero exit" '[ "$rc5" -ne 0 ]'
assert "F5 --repo-dir no arg: diagnostic mentions --repo-dir" 'printf "%s" "$err5" | grep -q -- "--repo-dir"'

# --- skill-wiring sentinels (the SKILLs are code on the integration branch) ------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention names docket-config.sh" 'grep -qF "/docket-config.sh" "$CONV"'
assert "convention defines the DOCKET_SCRIPTS_DIR resolved form" \
  'grep -qF "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}" "$CONV"'
assert "convention documents DOCKET_ namespacing" \
  'grep -qiF "DOCKET_-namespaced" "$CONV"'
assert "convention documents the Skill layer" 'grep -qF "Skill layer" "$CONV"'
assert "convention names SKILL_ resolution vars" \
  'grep -qF "SKILL_BRAINSTORM" "$CONV" && grep -qF "SKILL_FINISH" "$CONV"'
assert "convention documents the auto sentinel + degrade rule" \
  'grep -qiF "degrade to auto" "$CONV"'
for s in docket-implement-next docket-status docket-new-change docket-groom-next \
         docket-finalize-change docket-adr docket-auto-groom; do
  f="$REPO/skills/$s/SKILL.md"
  assert "$s Step 0 invokes docket-config.sh" 'grep -qF "/docket-config.sh" "$f"'
done
assert "new-change brainstorm uses SKILL_BRAINSTORM" \
  'grep -qF "SKILL_BRAINSTORM" "$REPO/skills/docket-new-change/SKILL.md"'
assert "groom-next brainstorm uses SKILL_BRAINSTORM" \
  'grep -qF "SKILL_BRAINSTORM" "$REPO/skills/docket-groom-next/SKILL.md"'
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
assert "implement-next plan uses SKILL_PLAN"     'grep -qF "SKILL_PLAN" "$IMPL"'
assert "implement-next build uses SKILL_BUILD"   'grep -qF "SKILL_BUILD" "$IMPL"'
assert "implement-next review uses SKILL_REVIEW" 'grep -qF "SKILL_REVIEW" "$IMPL"'
assert "implement-next finish uses SKILL_FINISH" 'grep -qF "SKILL_FINISH" "$IMPL"'
assert "finalize finish uses SKILL_FINISH" \
  'grep -qF "SKILL_FINISH" "$REPO/skills/docket-finalize-change/SKILL.md"'

# --- (G) skills: absent -> five superpowers defaults (byte-identical behavior) ---
mkrepo "$tmp/g"
printf 'metadata_branch: main\n' > "$tmp/g/.docket.yml"
git -C "$tmp/g" add .docket.yml; git -C "$tmp/g" commit --quiet -m cfg; git -C "$tmp/g" push --quiet origin main
out="$(run "$tmp/g" --export)"; eval "$out"
assert "skills absent: BRAINSTORM default" '[ "$SKILL_BRAINSTORM" = superpowers:brainstorming ]'
assert "skills absent: PLAN default"       '[ "$SKILL_PLAN" = superpowers:writing-plans ]'
assert "skills absent: BUILD default"      '[ "$SKILL_BUILD" = superpowers:subagent-driven-development ]'
assert "skills absent: REVIEW default"     '[ "$SKILL_REVIEW" = superpowers:requesting-code-review ]'
assert "skills absent: FINISH default"     '[ "$SKILL_FINISH" = superpowers:finishing-a-development-branch ]'

# --- (H) skills: explicit overrides incl. `auto`, a custom name, and a partial map ---
mkrepo "$tmp/h"
cat > "$tmp/h/.docket.yml" <<'EOF'
metadata_branch: main
skills:
  build: auto
  review: my-org:custom-review
  brainstorm: superpowers:brainstorming
EOF
git -C "$tmp/h" add .docket.yml; git -C "$tmp/h" commit --quiet -m cfg; git -C "$tmp/h" push --quiet origin main
out="$(run "$tmp/h" --export)"; eval "$out"
assert "skills auto: BUILD is auto"         '[ "$SKILL_BUILD" = auto ]'
assert "skills custom: REVIEW verbatim"     '[ "$SKILL_REVIEW" = my-org:custom-review ]'
assert "skills partial: PLAN still default" '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# --- (I) skills: TAB-indented block parses (LEARNINGS #46 — whitespace class) ---
mkrepo "$tmp/i"
printf 'metadata_branch: main\nskills:\n\tplan: auto\n' > "$tmp/i/.docket.yml"
git -C "$tmp/i" add .docket.yml; git -C "$tmp/i" commit --quiet -m cfg; git -C "$tmp/i" push --quiet origin main
out="$(run "$tmp/i" --export)"; eval "$out"
assert "skills tab-indent: PLAN auto"       '[ "$SKILL_PLAN" = auto ]'

# --- (J) skills: unknown role key -> warned on stderr, ignored; known keys still resolve ---
mkrepo "$tmp/j"
printf 'metadata_branch: main\nskills:\n  bogus: x\n  plan: auto\n' > "$tmp/j/.docket.yml"
git -C "$tmp/j" add .docket.yml; git -C "$tmp/j" commit --quiet -m cfg; git -C "$tmp/j" push --quiet origin main
jerr="$(run "$tmp/j" --export 2>&1 >/dev/null)"
out="$(run "$tmp/j" --export 2>/dev/null)"; eval "$out"
assert "skills unknown key: warned on stderr"       'printf "%s" "$jerr" | grep -qi "unknown skills role"'
assert "skills unknown key: known PLAN still parsed" '[ "$SKILL_PLAN" = auto ]'
assert "skills unknown key: does not abort (exit 0)" '[ "$(run_rc "$tmp/j" --export)" -eq 0 ]'

# ============================================================================
# Change 0050 — global config layer (~/.config/docket/config.yml)
# ============================================================================

# --- (K) global-only keys honored (repo has no .docket.yml) ------------------
mkrepo "$tmp/k"
mkdir -p "$tmp/k.xdg/docket"
cat > "$tmp/k.xdg/docket/config.yml" <<'EOF'
auto_groom: true
finalize:
  gate: ci
skills:
  build: auto
EOF
out="$(rung "$tmp/k.xdg" "$tmp/k" --export)"; eval "$out"
assert "0050 K: global auto_groom honored"          '[ "$AUTO_GROOM" = true ]'
assert "0050 K: global finalize.gate honored"       '[ "$FINALIZE_GATE" = ci ]'
assert "0050 K: global skills.build honored"        '[ "$SKILL_BUILD" = auto ]'
assert "0050 K: unset key stays built-in (inline)"  '[ "$BOARD_SURFACES" = inline ]'
assert "0050 K: unset skill role stays default"     '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# --- (L) per-repo overrides global, field-by-field skills merge --------------
mkrepo "$tmp/l"
cat > "$tmp/l/.docket.yml" <<'EOF'
metadata_branch: main
auto_groom: false
skills:
  plan: superpowers:writing-plans
EOF
git -C "$tmp/l" add .docket.yml; git -C "$tmp/l" commit --quiet -m cfg
git -C "$tmp/l" push --quiet origin main
mkdir -p "$tmp/l.xdg/docket"
cat > "$tmp/l.xdg/docket/config.yml" <<'EOF'
auto_groom: true
skills:
  plan: auto
  review: my-org:global-review
EOF
out="$(rung "$tmp/l.xdg" "$tmp/l" --export)"; eval "$out"
assert "0050 L: per-repo auto_groom false beats global true" '[ "$AUTO_GROOM" = false ]'
assert "0050 L: skills merge — repo plan wins over global"   '[ "$SKILL_PLAN" = superpowers:writing-plans ]'
assert "0050 L: skills merge — global review holds"          '[ "$SKILL_REVIEW" = my-org:global-review ]'
assert "0050 L: skills merge — unset role stays default"     '[ "$SKILL_BUILD" = superpowers:subagent-driven-development ]'

# --- (Q) XDG_CONFIG_HOME honored; HOME/.config is the fallback ---------------
mkrepo "$tmp/q"
mkdir -p "$tmp/q.home/.config/docket"
printf 'auto_groom: true\n' > "$tmp/q.home/.config/docket/config.yml"
out="$(env -u XDG_CONFIG_HOME HOME="$tmp/q.home" bash "$SCRIPT" --repo-dir "$tmp/q" --export)"; eval "$out"
assert "0050 Q: XDG unset -> \$HOME/.config fallback read"   '[ "$AUTO_GROOM" = true ]'

# --- (E') emit-interface guard: still exactly 18 lines with a global file present ---
n50="$(rung "$tmp/k.xdg" "$tmp/k" --export | grep -c '=')"
assert "0050 E': still 18 KEY=value lines with global layer" '[ "$n50" -eq 18 ]'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
