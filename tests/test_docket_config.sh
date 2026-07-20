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
assert "absent cfg: TERMINAL_PUBLISH default false"    '[ "$TERMINAL_PUBLISH" = false ]'

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

# plain format absolutizes METADATA_WORKTREE in main-mode too: '.' -> the repo root itself
# (no /.docket suffix), covering the MW_EMIT="$REPO_ABS" branch in scripts/docket-config.sh.
b_abs="$(cd "$tmp/b" && pwd -P)"
b_plain="$(run "$tmp/b" --export --format plain)"
assert "plain format absolutizes METADATA_WORKTREE (main-mode => repo root)" 'printf "%s\n" "$b_plain" | grep -qxF "METADATA_WORKTREE=$b_abs"'

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

# --- (D) board_surfaces: [] -> the reserved `none` token, distinct from unset (change 0071) ----
# 0071 inverts the polarity: an EMPTY BOARD_SURFACES no longer means "board disabled" — it means
# "nobody resolved this", and every consumer now treats it as a wiring bug. The deliberate
# off-state is encoded POSITIVELY as `none`. Asserted on the EMITTED LINE, not on an eval'd
# variable: `eval "$out"` after a run that emitted nothing leaves the PREVIOUS case's value in
# place, so a variable assert cannot tell "emitted nothing" from "emitted the right thing".
#
# The never-empty invariant must be checked per FORMAT: the default `shell` format's emit()
# uses printf '%s=%q\n', and bash's %q renders an empty string as `BOARD_SURFACES=''` — never
# as a bare `BOARD_SURFACES=`. Grepping shell-format output for a bare `BOARD_SURFACES=` is
# vacuous — it would still pass even with the `none` normalizer removed, since that regression
# emits `BOARD_SURFACES=''` there, not a bare line. So: check the bare-empty-never-happens shape
# against `--format plain` (where a regression genuinely renders bare, since plain's emit() does
# no quoting), and check the quoted-empty-never-happens shape against the default shell format
# (the shape a regression actually emits there).
mkrepo "$tmp/d"
printf 'metadata_branch: main\nboard_surfaces: []\n' > "$tmp/d/.docket.yml"
git -C "$tmp/d" add .docket.yml; git -C "$tmp/d" commit --quiet -m cfg
git -C "$tmp/d" push --quiet origin main
out="$(run "$tmp/d" --export)"
out_plain="$(run "$tmp/d" --export --format plain)"
assert "board []: emits BOARD_SURFACES=none"           'printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES=none"'
assert "board []: plain format never emits an empty BOARD_SURFACES" \
  '! printf "%s\n" "$out_plain" | grep -qxF "BOARD_SURFACES="'
assert "board []: shell format never emits quoted-empty BOARD_SURFACES" \
  '! printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES='\'''\''"'
eval "$out"
assert "board []: BOARD_SURFACES is the none token"     '[ "$BOARD_SURFACES" = none ]'

# --- (D2) the never-empty invariant holds across layers (change 0071) --------------------------
# A global layer whose ONLY token is `github` is machine-fenced (0050) and filtered to nothing.
# That filtered-to-empty path is a second way the resolver used to emit "" — it must also land
# on `none`, or the sentinel has a hole exactly where the fence bites.
mkrepo "$tmp/d2"
mkdir -p "$tmp/d2.xdg/docket"
printf 'board_surfaces: [github]\n' > "$tmp/d2.xdg/docket/config.yml"
out="$(rung "$tmp/d2.xdg" "$tmp/d2" --export 2>/dev/null)"
assert "board fenced-to-empty: emits BOARD_SURFACES=none" \
  'printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES=none"'

# --- (E) direct-pipe caller (LEARNINGS #22: $() hides a dropped trailing \n) -
n="$(run "$tmp/c" --export | grep -c '=')"
assert "direct-pipe: 24 KEY=value lines emitted"       '[ "$n" -eq 24 ]'
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

# (W2-gi) --bootstrap in the fresh cell also SEEDS the managed .gitignore block in the
# primary tree, prints a loud COMMIT notice, and commits NOTHING (change 0057).
w2gi="$tmp/w2gi"; mkrepo "$w2gi"                       # fresh docket-mode repo (¬DOCKET ∧ ¬LIVE)
head_before="$(git -C "$w2gi" rev-parse HEAD 2>/dev/null || echo none)"
bs_err="$(run "$w2gi" --bootstrap --export 2>&1 >/dev/null)"
assert "0057 bootstrap: block seeded in primary tree" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$w2gi/.gitignore"'
assert "0057 bootstrap: loud COMMIT notice printed"   'printf "%s" "$bs_err" | grep -qi "commit"'
assert "0057 bootstrap: nothing auto-committed"       '[ "$(git -C "$w2gi" rev-parse HEAD 2>/dev/null || echo none)" = "$head_before" ]'
assert "0057 bootstrap: .gitignore left UNstaged"     '[ -z "$(git -C "$w2gi" diff --cached --name-only 2>/dev/null)" ]'

# (W1-gi) default --export in the fresh cell stays strictly READ-ONLY: no .gitignore written.
w1gi="$tmp/w1gi"; mkrepo "$w1gi"
run "$w1gi" --export >/dev/null 2>&1
assert "0057 export: read-only — no .gitignore seeded" '[ ! -e "$w1gi/.gitignore" ]'

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
# 0074 retired the direct `…/docket-config.sh --bootstrap` invocation from Step-0; the convention
# still NAMES the resolver descriptively (Config section: `docket-config.sh --export`), which
# ADR-0030 permits. Key on the noun, not the retired slash-prefixed invocation form.
assert "convention names docket-config.sh (descriptively)" 'grep -qF -- "docket-config.sh" "$CONV"'
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
  assert "$s Step 0 runs docket.sh preflight (config resolution via the facade)" 'grep -qF "docket.sh preflight" "$f"'
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

# --- (E') emit-interface guard: still exactly 24 lines with a global file present ---
n50="$(rung "$tmp/k.xdg" "$tmp/k" --export | grep -c '=')"
assert "0050 E': still 24 KEY=value lines with global layer" '[ "$n50" -eq 24 ]'

# --- (M) coordination-key fence: warned-and-ignored, never honored, never fatal ---
mkrepo "$tmp/m"
mkdir -p "$tmp/m.xdg/docket"
cat > "$tmp/m.xdg/docket/config.yml" <<'EOF'
metadata_branch: main
changes_dir: elsewhere/changes
auto_groom: true
EOF
merr="$(rung "$tmp/m.xdg" "$tmp/m" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/m.xdg" "$tmp/m" --export 2>/dev/null)"; eval "$out"
assert "0050 M: fence warns metadata_branch"        'printf "%s" "$merr" | grep -q "metadata_branch"'
assert "0050 M: fence names per-repo-only"          'printf "%s" "$merr" | grep -qi "per-repo-only"'
assert "0050 M: fence warns changes_dir"            'printf "%s" "$merr" | grep -q "changes_dir"'
assert "0050 M: global metadata_branch NOT honored" '[ "$METADATA_BRANCH" = docket ]'
assert "0050 M: CHANGES_DIR stays default"          '[ "$CHANGES_DIR" = docs/changes ]'
assert "0050 M: global-able key in same file still honored" '[ "$AUTO_GROOM" = true ]'
assert "0050 M: fence is not fatal (exit 0)"        '[ "$(rung_rc "$tmp/m.xdg" "$tmp/m" --export)" -eq 0 ]'

# --- (N) global board_surfaces: github token dropped; [] and [inline] work -------
mkrepo "$tmp/n"
mkdir -p "$tmp/n.xdg/docket"
printf 'board_surfaces: [inline, github]\n' > "$tmp/n.xdg/docket/config.yml"
nerr="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"; eval "$out"
assert "0050 N: global github token warned"         'printf "%s" "$nerr" | grep -q "github"'
assert "0050 N: global github token dropped"        '[ "$BOARD_SURFACES" = inline ]'
printf 'board_surfaces: []\n' > "$tmp/n.xdg/docket/config.yml"
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"
assert "0050 N: global [] honored (board disabled, encoded as none)" \
  'printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES=none"'
eval "$out"
# per-repo github is untouched by the fence:
mkrepo "$tmp/n2"
printf 'metadata_branch: main\nboard_surfaces: [inline, github]\n' > "$tmp/n2/.docket.yml"
git -C "$tmp/n2" add .docket.yml; git -C "$tmp/n2" commit --quiet -m cfg
git -C "$tmp/n2" push --quiet origin main
n2err="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>/dev/null)"; eval "$out"
assert "0050 N: per-repo github honored"            '[ "$BOARD_SURFACES" = "inline github" ]'
assert "0050 N: per-repo github NOT warned"         '! printf "%s" "$n2err" | grep -q "board_surfaces token github"'

# --- (O) misplacement guard: ~/.config/docket/.docket.yml is warned, never read ---
mkrepo "$tmp/o"
mkdir -p "$tmp/o.xdg/docket"
printf 'auto_groom: true\n' > "$tmp/o.xdg/docket/.docket.yml"
oerr="$(rung "$tmp/o.xdg" "$tmp/o" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/o.xdg" "$tmp/o" --export 2>/dev/null)"; eval "$out"
assert "0050 O: misplacement warned, names config.yml" 'printf "%s" "$oerr" | grep -q "config.yml"'
assert "0050 O: misplaced file NOT read (auto_groom default)" '[ "$AUTO_GROOM" = false ]'
assert "0050 O: misplacement not fatal (exit 0)"    '[ "$(rung_rc "$tmp/o.xdg" "$tmp/o" --export)" -eq 0 ]'

# --- (P) malformed global file: warned, built-ins fallback, repos not bricked -----
mkrepo "$tmp/p"
mkdir -p "$tmp/p.xdg/docket/config.yml"            # a DIRECTORY at the config path
perr="$(rung "$tmp/p.xdg" "$tmp/p" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/p.xdg" "$tmp/p" --export 2>/dev/null)"; eval "$out"
assert "0050 P: malformed global warned"            'printf "%s" "$perr" | grep -qi "not a readable regular file"'
assert "0050 P: built-ins fallback (auto_groom)"    '[ "$AUTO_GROOM" = false ]'
assert "0050 P: malformed global not fatal (exit 0)" '[ "$(rung_rc "$tmp/p.xdg" "$tmp/p" --export)" -eq 0 ]'

# ============================================================================
# Change 0051 — machine-local layer: <repo>/.docket.local.yml
# Precedence per field: repo-local > repo-committed > global > built-in.
# ============================================================================

# (L1) local beats committed beats global (skills.build), per-field independence:
# build set in all three layers -> local wins; review set only globally -> global wins.
mkrepo "$tmp/l1"
cat > "$tmp/l1/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
skills:
  build: committed-build
EOF
git -C "$tmp/l1" add .docket.yml; git -C "$tmp/l1" commit --quiet -m cfg
git -C "$tmp/l1" push --quiet origin main
mkdir -p "$tmp/xdg-l1/docket"
printf 'skills:\n  build: global-build\n  review: global-review\n' > "$tmp/xdg-l1/docket/config.yml"
printf 'skills:\n  build: local-build\n' > "$tmp/l1/.docket.local.yml"
out="$(rung "$tmp/xdg-l1" "$tmp/l1" --export)"; eval "$out"
assert "0051 L1: local skills.build beats committed+global"  '[ "$SKILL_BUILD" = local-build ]'
assert "0051 L1: unset-local review falls to global"         '[ "$SKILL_REVIEW" = global-review ]'
assert "0051 L1: unset-everywhere plan falls to built-in"    '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# (L2) scalars: local auto_groom beats committed; local finalize.gate beats global.
mkrepo "$tmp/l2"
cat > "$tmp/l2/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
auto_groom: false
EOF
git -C "$tmp/l2" add .docket.yml; git -C "$tmp/l2" commit --quiet -m cfg
git -C "$tmp/l2" push --quiet origin main
mkdir -p "$tmp/xdg-l2/docket"
printf 'finalize:\n  gate: ci\n' > "$tmp/xdg-l2/docket/config.yml"
printf 'auto_groom: true\nfinalize:\n  gate: both\n  test_command: make local-test\n' > "$tmp/l2/.docket.local.yml"
out="$(rung "$tmp/xdg-l2" "$tmp/l2" --export)"; eval "$out"
assert "0051 L2: local auto_groom beats committed"       '[ "$AUTO_GROOM" = true ]'
assert "0051 L2: local finalize.gate beats global"       '[ "$FINALIZE_GATE" = both ]'
assert "0051 L2: local finalize.test_command honored"    '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'

# (L3) fenced keys in the local file: loudly warned-and-ignored, never honored, never fatal.
mkrepo "$tmp/l3"
printf 'metadata_branch: main\nchanges_dir: sneaky/changes\ngithub_project: {owner: x, number: 1}\n' > "$tmp/l3/.docket.local.yml"
errout="$(rung "$tmp/l3-noxdg" "$tmp/l3" --export 2>&1 >/dev/null)"; rc=$?
out="$(rung "$tmp/l3-noxdg" "$tmp/l3" --export 2>/dev/null)"; eval "$out"
assert "0051 L3: fenced local keys not fatal (rc=0)"     '[ "$rc" = "0" ]'
assert "0051 L3: warns metadata_branch is per-repo-only" 'grep -q "metadata_branch" <<<"$errout" && grep -qi "per-repo-only" <<<"$errout"'
assert "0051 L3: warning names the local file"           'grep -q "docket.local.yml" <<<"$errout"'
assert "0051 L3: fenced local metadata_branch IGNORED (mode stays docket-default)" '[ "$METADATA_BRANCH" = docket ]'
assert "0051 L3: fenced local changes_dir IGNORED"       '[ "$CHANGES_DIR" = docs/changes ]'

# (L4) board_surfaces from the local layer: honored, but its github token is machine-fenced.
mkrepo "$tmp/l4"
cat > "$tmp/l4/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/l4" add .docket.yml; git -C "$tmp/l4" commit --quiet -m cfg
git -C "$tmp/l4" push --quiet origin main
printf 'board_surfaces: [inline, github]\n' > "$tmp/l4/.docket.local.yml"
errout="$(rung "$tmp/l4-noxdg" "$tmp/l4" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/l4-noxdg" "$tmp/l4" --export 2>/dev/null)"; eval "$out"
assert "0051 L4: local board_surfaces honored minus github" '[ "$BOARD_SURFACES" = inline ]'
assert "0051 L4: warns the github token is per-repo-only"   'grep -qi "github" <<<"$errout" && grep -qi "per-repo-only" <<<"$errout"'
# committed github stays honored (regression pin for the per-repo path):
mkrepo "$tmp/l4b"
cat > "$tmp/l4b/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
board_surfaces: [inline, github]
EOF
git -C "$tmp/l4b" add .docket.yml; git -C "$tmp/l4b" commit --quiet -m cfg
git -C "$tmp/l4b" push --quiet origin main
out="$(run "$tmp/l4b" --export)"; eval "$out"
assert "0051 L4: committed github token still honored" '[ "$BOARD_SURFACES" = "inline github" ]'

# (L5) malformed local file (a directory): warn + skip, repo still works.
mkrepo "$tmp/l5"
mkdir "$tmp/l5/.docket.local.yml"
errout="$(rung "$tmp/l5-noxdg" "$tmp/l5" --export 2>&1 >/dev/null)"; rc=$?
assert "0051 L5: malformed local not fatal (rc=0)"  '[ "$rc" = "0" ]'
assert "0051 L5: warns local layer ignored"          'grep -qi "docket.local.yml" <<<"$errout" && grep -qi "ignored" <<<"$errout"'

# (L6) unknown skills role in the LOCAL block: warned + ignored.
mkrepo "$tmp/l6"
printf 'skills:\n  bogusrole: x\n' > "$tmp/l6/.docket.local.yml"
errout="$(rung "$tmp/l6-noxdg" "$tmp/l6" --export 2>&1 >/dev/null)"; rc=$?
assert "0051 L6: unknown local role not fatal (rc=0)" '[ "$rc" = "0" ]'
assert "0051 L6: warns unknown role"                  'grep -qi "unknown skills role" <<<"$errout" && grep -q "bogusrole" <<<"$errout"'

# ============================================================================
# Change 0064 — terminal_publish: coordination-key fence + TERMINAL_PUBLISH emit
# ============================================================================

# --- (0064) terminal_publish: repo-committed value honored; fenced in machine layers ---
mkrepo "$tmp/tp"
printf 'metadata_branch: docket\nterminal_publish: false\n' > "$tmp/tp/.docket.yml"
git -C "$tmp/tp" add .docket.yml; git -C "$tmp/tp" commit --quiet -m cfg
git -C "$tmp/tp" push --quiet origin main
out="$(run "$tmp/tp" --export)"; eval "$out"
assert "0064: repo terminal_publish false is honored" '[ "$TERMINAL_PUBLISH" = false ]'

# explicit true round-trips
mkrepo "$tmp/tp2"
printf 'metadata_branch: docket\nterminal_publish: true\n' > "$tmp/tp2/.docket.yml"
git -C "$tmp/tp2" add .docket.yml; git -C "$tmp/tp2" commit --quiet -m cfg
git -C "$tmp/tp2" push --quiet origin main
out="$(run "$tmp/tp2" --export)"; eval "$out"
assert "0064: repo terminal_publish true is honored" '[ "$TERMINAL_PUBLISH" = true ]'

# fence: a GLOBAL terminal_publish is warned-and-ignored, never honored, never fatal
# change 0084: the probe value is `true` — the NON-default. With the default at `false`, probing
# with `false` would make the assertion vacuous (the ignored value and the default coincide, so it
# would pass even if the fence honored the value). Probing with `true` keeps it discriminating: if
# the fence ever regresses, `true` wins and this goes red.
mkrepo "$tmp/tp3"
mkdir -p "$tmp/tp3.xdg/docket"
printf 'terminal_publish: true\n' > "$tmp/tp3.xdg/docket/config.yml"
tperr="$(rung "$tmp/tp3.xdg" "$tmp/tp3" --export 2>&1 >/dev/null)"
# Unset before the eval that follows: a run that ABORTS emits nothing, so eval "" is a no-op —
# without unsetting first, TERMINAL_PUBLISH would silently keep its value from an earlier block
# in this file and the "stays false" assert below would pass vacuously on stale state instead of
# on this run's actual (non-)output. `${TERMINAL_PUBLISH-unset}` below reads it back safely under
# `set -u` whether the eval set it or left it unset.
unset TERMINAL_PUBLISH
out="$(rung "$tmp/tp3.xdg" "$tmp/tp3" --export 2>/dev/null)"; eval "$out"
assert "0064 fence: global terminal_publish warns"        'printf "%s" "$tperr" | grep -q "terminal_publish"'
assert "0064 fence: warning says per-repo-only"           'printf "%s" "$tperr" | grep -qi "per-repo-only"'
assert "0064 fence: global value NOT honored (stays false)" '[ "${TERMINAL_PUBLISH-unset}" = false ]'
assert "0064 fence: global terminal_publish is not fatal"  '[ "$(rung_rc "$tmp/tp3.xdg" "$tmp/tp3" --export)" -eq 0 ]'

# fence: a MACHINE-LOCAL .docket.local.yml terminal_publish is warned-and-ignored too
# change 0084: probes with `true` (the non-default) for the same reason as the global block above.
mkrepo "$tmp/tp4"
printf 'terminal_publish: true\n' > "$tmp/tp4/.docket.local.yml"
lerr="$(run "$tmp/tp4" --export 2>&1 >/dev/null)"; rc=$?
# Same stale-value hazard as the global block above — unset before the eval, and read back via
# the safe default-expansion so an abort (empty eval) is caught as "unset", not misread as a
# leftover "false" from an earlier block.
unset TERMINAL_PUBLISH
out="$(run "$tmp/tp4" --export 2>/dev/null)"; eval "$out"
assert "0064 fence: .docket.local.yml terminal_publish warns" 'printf "%s" "$lerr" | grep -q "terminal_publish"'
assert "0064 fence: local names .docket.local.yml"            'printf "%s" "$lerr" | grep -q ".docket.local.yml"'
assert "0064 fence: local value NOT honored (stays false)"     '[ "${TERMINAL_PUBLISH-unset}" = false ]'
assert "0064 fence: local terminal_publish is not fatal (rc=0)" '[ "$rc" -eq 0 ]'

# fail-closed: an unparseable repo value aborts (never silently coerced to true)
mkrepo "$tmp/tp5"
printf 'metadata_branch: docket\nterminal_publish: flase\n' > "$tmp/tp5/.docket.yml"
git -C "$tmp/tp5" add .docket.yml; git -C "$tmp/tp5" commit --quiet -m cfg
git -C "$tmp/tp5" push --quiet origin main
assert "0064: unparseable terminal_publish exits non-zero" \
  '! run "$tmp/tp5" --export >/dev/null 2>&1'
assert "0064: unparseable terminal_publish emits nothing"  \
  '[ -z "$(run "$tmp/tp5" --export 2>/dev/null)" ]'

# --- (0064) surfacing: the knob is documented end-to-end (learning #49) ---
CONV_SKILL="$REPO/skills/docket-convention/SKILL.md"
assert "0064 doc: convention schema block documents terminal_publish" \
  'grep -q "terminal_publish" "$CONV_SKILL"'
assert "0064 doc: convention fence list includes terminal_publish" \
  'grep -q "terminal_publish" <<<"$(grep -A2 "Coordination-key fence" "$CONV_SKILL")"'
assert "0064 doc: README documents terminal_publish" \
  'grep -q "terminal_publish" "$REPO/README.md"'
assert "0064 doc: sample .docket.yml carries the commented knob" \
  'grep -q "terminal_publish" "$REPO/.docket.yml"'
assert "0064 doc: config contract classifies terminal_publish as fenced" \
  'grep -q "terminal_publish" "$REPO/scripts/docket-config.md"'

# ============================================================================
# (Z) --format plain — raw model-facing presentation (change 0068)
# ============================================================================
mkrepo "$tmp/fmt"
# docket branch so bootstrap verdict resolves to PROCEED (mkrepo leaves a live main surface,
# so create the orphan docket branch the way docket-config --bootstrap would).
git -C "$tmp/fmt" push --quiet origin "$(git -C "$tmp/fmt" commit-tree "$(git -C "$tmp/fmt" mktree </dev/null)" -m orphan):refs/heads/docket" 2>/dev/null
git -C "$tmp/fmt" fetch --quiet origin docket 2>/dev/null

# shell format (default) is UNCHANGED: %q-quoted, eval-able, empty => KEY=''
shell_out="$(run "$tmp/fmt" --export)"
assert "shell format still %q-quotes empty values" 'printf "%s" "$shell_out" | grep -qxF "FINALIZE_TEST_COMMAND='\'''\''"'
assert "shell format METADATA_WORKTREE stays relative .docket" 'printf "%s" "$shell_out" | grep -qxF "METADATA_WORKTREE=.docket"'

# plain format: raw KEY=value, no %q, no export prefix, empty => bare "KEY="
plain_out="$(run "$tmp/fmt" --export --format plain)"
assert "plain format emits raw empty value (no quotes)" 'printf "%s\n" "$plain_out" | grep -qxF "FINALIZE_TEST_COMMAND="'
assert "plain format has no export prefix" '! printf "%s\n" "$plain_out" | grep -q "^export "'
assert "plain format emits BOOTSTRAP" 'printf "%s\n" "$plain_out" | grep -qxF "BOOTSTRAP=PROCEED"'
assert "plain format emits raw enum values unquoted" 'printf "%s\n" "$plain_out" | grep -qxF "DOCKET_MODE=docket"'
# METADATA_WORKTREE absolutized in plain mode
fmt_abs="$(cd "$tmp/fmt" && pwd -P)"
assert "plain format absolutizes METADATA_WORKTREE (docket-mode)" 'printf "%s\n" "$plain_out" | grep -qxF "METADATA_WORKTREE=$fmt_abs/.docket"'
assert "plain format keeps CHANGES_DIR as repo-relative subpath" 'printf "%s\n" "$plain_out" | grep -qxF "CHANGES_DIR=docs/changes"'

# plain mode still fails closed on an aborting resolver: nothing on stdout, non-zero exit
# (#64b: clear the asserted capture first so a prior value can never masquerade as success).
plain_abort=""
plain_abort="$(bash "$SCRIPT" --repo-dir "$tmp/does-not-exist" --export --format plain 2>/dev/null)"; abort_rc=$?
assert "plain mode aborts non-zero on bad repo" '[ "$abort_rc" -ne 0 ]'
assert "plain mode emits NOTHING on abort" '[ -z "$plain_abort" ]'

# unknown --format value is a wiring error (exit 2)
run "$tmp/fmt" --export --format bogus >/dev/null 2>&1; fmt_rc=$?
assert "unknown --format exits 2" '[ "$fmt_rc" -eq 2 ]'

# --- (Z) change 0075: the repo anchor + REPO_ROOT ----------------------------
# The resolver must resolve the SAME primary root no matter which worktree/subdir the caller
# stands in. Every OTHER test in this file passes --repo-dir explicitly, so this section is the
# only coverage of the DEFAULT resolution — which is exactly the thing 0075 changes.
mkrepo "$tmp/z"
z_abs="$(cd "$tmp/z" && pwd -P)"
git -C "$tmp/z" branch --quiet docket
git -C "$tmp/z" worktree add --quiet "$tmp/z/.docket" docket >/dev/null 2>&1
mkdir -p "$tmp/z/sub"

# plain format from the MAIN ROOT: REPO_ROOT present and absolute.
z_root_plain="$(cd "$tmp/z" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT emitted, absolute, = the main worktree" \
  'printf "%s\n" "$z_root_plain" | grep -qxF "REPO_ROOT=$z_abs"'

# plain format from the .docket/ LINKED WORKTREE: byte-identical REPO_ROOT and METADATA_WORKTREE.
# Pre-0075 this yielded REPO_ROOT=<repo>/.docket and METADATA_WORKTREE=<repo>/.docket/.docket.
z_dk_plain="$(cd "$tmp/z/.docket" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT from the .docket/ worktree is the MAIN root, not .docket" \
  'printf "%s\n" "$z_dk_plain" | grep -qxF "REPO_ROOT=$z_abs"'
assert "0075 plain: METADATA_WORKTREE from .docket/ is <root>/.docket, NOT <root>/.docket/.docket" \
  'printf "%s\n" "$z_dk_plain" | grep -qxF "METADATA_WORKTREE=$z_abs/.docket"'

# plain format from a SUBDIRECTORY: the spec's stated behavior CHANGE (§1) — pinned deliberately.
z_sub_plain="$(cd "$tmp/z/sub" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT from <repo>/sub is the repo root (§1 behavior change, pinned)" \
  'printf "%s\n" "$z_sub_plain" | grep -qxF "REPO_ROOT=$z_abs"'
assert "0075 plain: METADATA_WORKTREE from <repo>/sub is <root>/.docket, not <sub>/.docket" \
  'printf "%s\n" "$z_sub_plain" | grep -qxF "METADATA_WORKTREE=$z_abs/.docket"'

# The machine-local layer is read from the REPO ROOT even when invoked from a subdirectory
# (§1: LCFG="$REPO_DIR/.docket.local.yml"). auto_groom is a global-able (non-fenced) key.
printf 'auto_groom: true\n' > "$tmp/z/.docket.local.yml"
z_sub_shell="$(cd "$tmp/z/sub" && bash "$SCRIPT" --export)"
AUTO_GROOM=""; eval "$z_sub_shell"
assert "0075: <repo>/.docket.local.yml is read when invoked from <repo>/sub (§1 behavior change)" \
  '[ "$AUTO_GROOM" = true ]'
rm -f "$tmp/z/.docket.local.yml"

# REPO_ROOT is PLAIN-ONLY: ensure-claude-settings.sh sets its own REPO_ROOT and eval's the SHELL
# export, so a shell-format REPO_ROOT would silently capture that name. Assert BOTH directions so
# the guard is provably able to fire (a bare `! grep` that can never match proves nothing).
z_shell="$(cd "$tmp/z" && bash "$SCRIPT" --export)"
assert "0075 shell: REPO_ROOT is NOT emitted (would capture ensure-claude-settings.sh's own var)" \
  '! printf "%s\n" "$z_shell" | grep -q "^REPO_ROOT="'
assert "0075 control: the plain export DOES carry REPO_ROOT (proves the absence-assert can fire)" \
  'printf "%s\n" "$z_root_plain" | grep -q "^REPO_ROOT="'

# --repo-dir still overrides verbatim, from anywhere (the whole existing suite depends on it).
mkrepo "$tmp/z2"
z2_abs="$(cd "$tmp/z2" && pwd -P)"
z2_plain="$(cd "$tmp/z/.docket" && bash "$SCRIPT" --repo-dir "$tmp/z2" --export --format plain)"
assert "0075: --repo-dir still overrides the anchor verbatim" \
  'printf "%s\n" "$z2_plain" | grep -qxF "REPO_ROOT=$z2_abs"'

# ============================================================================
# Change 0067 — the learnings: block (LEARNINGS_ENABLED, LEARNINGS_CAP)
# NOTE (guards-are-code (e)): clear the asserted vars BEFORE each eval — an aborting run emits
# NOTHING, and eval "" would silently leave the previous case's value in place; the assert would
# then pass vacuously on stale state instead of on this run's actual (non-)output.
# ============================================================================

# --- (LRN-a) defaults when no layer sets the block ----------------------------
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo "$tmp/lrn-a"
out="$(run "$tmp/lrn-a" --export)"; eval "$out"
assert "learnings.enabled defaults to true"  '[ "$LEARNINGS_ENABLED" = "true" ]'
assert "learnings.cap defaults to 300"       '[ "$LEARNINGS_CAP" = "300" ]'

# --- (LRN-b) repo-committed block is honored -----------------------------------
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo "$tmp/lrn-b"
cat > "$tmp/lrn-b/.docket.yml" <<'EOF'
metadata_branch: main
learnings:
  enabled: false
  cap: 120
EOF
git -C "$tmp/lrn-b" add .docket.yml; git -C "$tmp/lrn-b" commit --quiet -m cfg
git -C "$tmp/lrn-b" push --quiet origin main
out="$(run "$tmp/lrn-b" --export)"; eval "$out"
assert "repo learnings.enabled honored" '[ "$LEARNINGS_ENABLED" = "false" ]'
assert "repo learnings.cap honored"     '[ "$LEARNINGS_CAP" = "120" ]'

# --- (LRN-c) BOTH keys are global-able (ADR-0019 — NOT fenced) -----------------
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo "$tmp/lrn-c"
mkdir -p "$tmp/lrn-c.xdg/docket"
cat > "$tmp/lrn-c.xdg/docket/config.yml" <<'EOF'
learnings:
  enabled: false
  cap: 42
EOF
lrn_c_err="$(rung "$tmp/lrn-c.xdg" "$tmp/lrn-c" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/lrn-c.xdg" "$tmp/lrn-c" --export 2>/dev/null)"; eval "$out"
assert "learnings.enabled is global-able (not fenced)" '[ "$LEARNINGS_ENABLED" = "false" ]'
assert "learnings.cap is global-able (not fenced)"     '[ "$LEARNINGS_CAP" = "42" ]'
assert "no fence warning for learnings keys" '! printf "%s" "$lrn_c_err" | grep -qi "learnings.*per-repo-only"'

# --- (LRN-d) repo-local layer wins over repo-committed -------------------------
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo "$tmp/lrn-d"
cat > "$tmp/lrn-d/.docket.yml" <<'EOF'
metadata_branch: main
learnings:
  cap: 120
EOF
git -C "$tmp/lrn-d" add .docket.yml; git -C "$tmp/lrn-d" commit --quiet -m cfg
git -C "$tmp/lrn-d" push --quiet origin main
printf 'learnings:\n  cap: 7\n' > "$tmp/lrn-d/.docket.local.yml"
out="$(run "$tmp/lrn-d" --export)"; eval "$out"
assert "local layer beats repo-committed for cap" '[ "$LEARNINGS_CAP" = "7" ]'

# --- (LRN-e) SHADOW GUARD — a bare enabled:/cap: OUTSIDE the learnings: block ---
# must not leak in. This is the whole reason the block is read via yaml_block_body.
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo "$tmp/lrn-e"
cat > "$tmp/lrn-e/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  enabled: false
  cap: 9
EOF
git -C "$tmp/lrn-e" add .docket.yml; git -C "$tmp/lrn-e" commit --quiet -m cfg
git -C "$tmp/lrn-e" push --quiet origin main
out="$(run "$tmp/lrn-e" --export)"; eval "$out"
assert "a foreign block's enabled: does not shadow learnings.enabled" '[ "$LEARNINGS_ENABLED" = "true" ]'
assert "a foreign block's cap: does not shadow learnings.cap"         '[ "$LEARNINGS_CAP" = "300" ]'

# --- (LRN-f) fail closed on garbage (the terminal_publish precedent) ----------
mkrepo "$tmp/lrn-f1"
cat > "$tmp/lrn-f1/.docket.yml" <<'EOF'
metadata_branch: main
learnings:
  enabled: yes
EOF
git -C "$tmp/lrn-f1" add .docket.yml; git -C "$tmp/lrn-f1" commit --quiet -m cfg
git -C "$tmp/lrn-f1" push --quiet origin main
lrn_f1_err="$(bash "$SCRIPT" --repo-dir "$tmp/lrn-f1" --export 2>&1 >/dev/null)"
assert "unparseable learnings.enabled: nonzero exit" '[ "$(run_rc "$tmp/lrn-f1" --export)" -ne 0 ]'
assert "unparseable learnings.enabled: mentions learnings.enabled" \
  'printf "%s" "$lrn_f1_err" | grep -qF "learnings.enabled"'

mkrepo "$tmp/lrn-f2"
cat > "$tmp/lrn-f2/.docket.yml" <<'EOF'
metadata_branch: main
learnings:
  cap: lots
EOF
git -C "$tmp/lrn-f2" add .docket.yml; git -C "$tmp/lrn-f2" commit --quiet -m cfg
git -C "$tmp/lrn-f2" push --quiet origin main
lrn_f2_err="$(bash "$SCRIPT" --repo-dir "$tmp/lrn-f2" --export 2>&1 >/dev/null)"
assert "non-integer learnings.cap: nonzero exit" '[ "$(run_rc "$tmp/lrn-f2" --export)" -ne 0 ]'
assert "non-integer learnings.cap: mentions learnings.cap" \
  'printf "%s" "$lrn_f2_err" | grep -qF "learnings.cap"'

# ============================================================================
# Change 0089 — the reclaim: block (RECLAIM_LEASE_TTL, RECLAIM_AUTO)
# NOTE (guards-are-code (e)): clear the asserted vars BEFORE each eval — an aborting run emits
# NOTHING, and eval "" would silently leave the previous case's value in place; the assert would
# then pass vacuously on stale state instead of on this run's actual (non-)output.
# ============================================================================

# run_resolver_with <yaml-fragment> : commit a fresh repo whose .docket.yml is
# "metadata_branch: main\n" + <yaml-fragment>, run the resolver against it, echo stdout.
# (Mirrors run/mkrepo above; used for the fail-closed garbage-path asserts, which only care
# about the resolver's exit code / lack of output, not its resolved values.)
run_resolver_with(){
  local frag="$1" d="$tmp/reclaim-garbage-$RANDOM"
  mkrepo "$d"
  printf 'metadata_branch: main\n%b' "$frag" > "$d/.docket.yml"
  git -C "$d" add .docket.yml; git -C "$d" commit --quiet -m cfg
  git -C "$d" push --quiet origin main
  run "$d" --export
}

# --- (RCL-a) defaults when no layer sets the block ----------------------------
unset RECLAIM_LEASE_TTL RECLAIM_AUTO
mkrepo "$tmp/rcl-a"
out="$(run "$tmp/rcl-a" --export)"; eval "$out"
assert "RECLAIM_LEASE_TTL defaults to 72" 'echo "$out" | grep -qxF "RECLAIM_LEASE_TTL=72"'
assert "RECLAIM_AUTO defaults to false"   'echo "$out" | grep -qxF "RECLAIM_AUTO=false"'

# --- (RCL-b) repo-committed block is honored -----------------------------------
unset RECLAIM_LEASE_TTL RECLAIM_AUTO
mkrepo "$tmp/rcl-b"
cat > "$tmp/rcl-b/.docket.yml" <<'EOF'
metadata_branch: main
reclaim:
  lease_ttl: 12
  auto: true
EOF
git -C "$tmp/rcl-b" add .docket.yml; git -C "$tmp/rcl-b" commit --quiet -m cfg
git -C "$tmp/rcl-b" push --quiet origin main
out2="$(run "$tmp/rcl-b" --export)"; eval "$out2"
assert "RECLAIM_LEASE_TTL reads the block" 'echo "$out2" | grep -qxF "RECLAIM_LEASE_TTL=12"'
assert "RECLAIM_AUTO reads the block"      'echo "$out2" | grep -qxF "RECLAIM_AUTO=true"'

# --- (RCL-c) BOTH keys are global-able (ADR-0019 — NOT fenced) -----------------
unset RECLAIM_LEASE_TTL RECLAIM_AUTO
mkrepo "$tmp/rcl-c"
mkdir -p "$tmp/rcl-c.xdg/docket"
cat > "$tmp/rcl-c.xdg/docket/config.yml" <<'EOF'
reclaim:
  lease_ttl: 6
  auto: true
EOF
rcl_c_err="$(rung "$tmp/rcl-c.xdg" "$tmp/rcl-c" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/rcl-c.xdg" "$tmp/rcl-c" --export 2>/dev/null)"; eval "$out"
assert "reclaim.lease_ttl is global-able (not fenced)" '[ "$RECLAIM_LEASE_TTL" = "6" ]'
assert "reclaim.auto is global-able (not fenced)"      '[ "$RECLAIM_AUTO" = "true" ]'
assert "no fence warning for reclaim keys" '! printf "%s" "$rcl_c_err" | grep -qi "reclaim.*per-repo-only"'

# --- (RCL-d) repo-local layer wins over repo-committed -------------------------
unset RECLAIM_LEASE_TTL RECLAIM_AUTO
mkrepo "$tmp/rcl-d"
cat > "$tmp/rcl-d/.docket.yml" <<'EOF'
metadata_branch: main
reclaim:
  lease_ttl: 12
EOF
git -C "$tmp/rcl-d" add .docket.yml; git -C "$tmp/rcl-d" commit --quiet -m cfg
git -C "$tmp/rcl-d" push --quiet origin main
printf 'reclaim:\n  lease_ttl: 3\n' > "$tmp/rcl-d/.docket.local.yml"
out="$(run "$tmp/rcl-d" --export)"; eval "$out"
assert "local layer beats repo-committed for lease_ttl" '[ "$RECLAIM_LEASE_TTL" = "3" ]'

# --- (RCL-e) SHADOW GUARD — a bare lease_ttl:/auto: OUTSIDE the reclaim: block ---
# must not leak in. This is the whole reason the block is read via yaml_block_body — `auto` in
# particular is a generic word a future top-level block could otherwise shadow.
unset RECLAIM_LEASE_TTL RECLAIM_AUTO
mkrepo "$tmp/rcl-e"
cat > "$tmp/rcl-e/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  lease_ttl: 9
  auto: true
EOF
git -C "$tmp/rcl-e" add .docket.yml; git -C "$tmp/rcl-e" commit --quiet -m cfg
git -C "$tmp/rcl-e" push --quiet origin main
out="$(run "$tmp/rcl-e" --export)"; eval "$out"
assert "a foreign block's lease_ttl: does not shadow reclaim.lease_ttl" '[ "$RECLAIM_LEASE_TTL" = "72" ]'
assert "a foreign block's auto: does not shadow reclaim.auto"           '[ "$RECLAIM_AUTO" = "false" ]'

# --- (RCL-f) fail closed on garbage --------------------------------------------
assert "non-integer lease_ttl aborts nonzero" '! run_resolver_with "reclaim:\n  lease_ttl: soon\n" >/dev/null 2>&1'
assert "non-bool auto aborts nonzero"         '! run_resolver_with "reclaim:\n  auto: maybe\n" >/dev/null 2>&1'

rcl_f1_err="$(run_resolver_with "reclaim:\n  lease_ttl: soon\n" 2>&1 >/dev/null)"
assert "unparseable reclaim.lease_ttl: mentions reclaim.lease_ttl" \
  'printf "%s" "$rcl_f1_err" | grep -qF "reclaim.lease_ttl"'
rcl_f2_err="$(run_resolver_with "reclaim:\n  auto: maybe\n" 2>&1 >/dev/null)"
assert "unparseable reclaim.auto: mentions reclaim.auto" \
  'printf "%s" "$rcl_f2_err" | grep -qF "reclaim.auto"'

# --- Change 0091 — auto_capture (global-able boolean, default false) ---------------------------
# Mirrors auto_groom's four-layer resolution, but fails CLOSED on a non-boolean (the reclaim.auto /
# learnings.enabled / terminal_publish precedent): silently defaulting a typo to `false` would make
# an opted-in repo quietly stop capturing, which is invisible rather than loud.

# (AC-a) default
mkrepo "$tmp/ac-a"
out_ac="$(run "$tmp/ac-a" --export 2>/dev/null)"
assert "AUTO_CAPTURE defaults to false" 'echo "$out_ac" | grep -qxF "AUTO_CAPTURE=false"'

# (AC-b) repo-committed .docket.yml wins over the built-in
mkrepo "$tmp/ac-b"
printf 'auto_capture: true\n' > "$tmp/ac-b/.docket.yml"
git -C "$tmp/ac-b" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-b" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-b" push -q origin HEAD:main >/dev/null 2>&1
out_ac_b="$(run "$tmp/ac-b" --export 2>/dev/null)"
assert "AUTO_CAPTURE reads repo .docket.yml" 'echo "$out_ac_b" | grep -qxF "AUTO_CAPTURE=true"'

# (AC-c) global layer is honored (NOT fenced) and emits no per-repo-only warning
mkrepo "$tmp/ac-c"
mkdir -p "$tmp/ac-c.xdg/docket"
printf 'auto_capture: true\n' > "$tmp/ac-c.xdg/docket/config.yml"
ac_c_out="$(rung "$tmp/ac-c.xdg" "$tmp/ac-c" --export 2>/dev/null)"
ac_c_err="$(rung "$tmp/ac-c.xdg" "$tmp/ac-c" --export 2>&1 >/dev/null)"
assert "auto_capture is global-able (not fenced)" 'echo "$ac_c_out" | grep -qxF "AUTO_CAPTURE=true"'
assert "no fence warning for auto_capture" '! printf "%s" "$ac_c_err" | grep -qi "auto_capture.*per-repo-only"'

# (AC-d) repo-local .docket.local.yml outranks repo-committed AND global
mkrepo "$tmp/ac-d"
mkdir -p "$tmp/ac-d.xdg/docket"
printf 'auto_capture: false\n' > "$tmp/ac-d.xdg/docket/config.yml"
printf 'auto_capture: false\n' > "$tmp/ac-d/.docket.yml"
git -C "$tmp/ac-d" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-d" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-d" push -q origin HEAD:main >/dev/null 2>&1
printf 'auto_capture: true\n' > "$tmp/ac-d/.docket.local.yml"
ac_d_out="$(rung "$tmp/ac-d.xdg" "$tmp/ac-d" --export 2>/dev/null)"
assert "repo-local auto_capture outranks repo-committed and global" \
  'echo "$ac_d_out" | grep -qxF "AUTO_CAPTURE=true"'

# (AC-e) fail closed on garbage, and the diagnostic names the key
mkrepo "$tmp/ac-e"
printf 'auto_capture: maybe\n' > "$tmp/ac-e/.docket.yml"
git -C "$tmp/ac-e" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-e" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-e" push -q origin HEAD:main >/dev/null 2>&1
assert "non-bool auto_capture aborts nonzero" '! run "$tmp/ac-e" --export >/dev/null 2>&1'
ac_e_err="$(run "$tmp/ac-e" --export 2>&1 >/dev/null)"
assert "unparseable auto_capture: names auto_capture" \
  'printf "%s" "$ac_e_err" | grep -qF "auto_capture"'

# (AC-f) emit ORDER is pinned: AUTO_CAPTURE immediately follows AUTO_GROOM (the contract in
# scripts/docket-config.md lists them adjacently; a reordering there is a silent contract break).
# Identity, not just adjacency: look up each variable's OWN line number by name, so a swap of the
# two (still adjacent, but AUTO_CAPTURE before AUTO_GROOM) is caught rather than passing vacuously.
ac_f_out="$(run "$tmp/ac-a" --export 2>/dev/null)"
ac_g_line="$(printf '%s\n' "$ac_f_out" | grep -n '^AUTO_GROOM=' | cut -d: -f1)"
ac_c_line="$(printf '%s\n' "$ac_f_out" | grep -n '^AUTO_CAPTURE=' | cut -d: -f1)"
assert "AUTO_CAPTURE is emitted directly after AUTO_GROOM" \
  '[ -n "$ac_g_line" ] && [ -n "$ac_c_line" ] && [ "$ac_c_line" -eq "$(( ac_g_line + 1 ))" ]'

# --- (S) finalize.test_command: auto  ==  unset (change 0101 sentinel) -------
# `auto` is the example file's way of shipping the default EXPLICITLY. It must resolve
# byte-identically to an absent key, or the sentinel leaks into finalize as a command to run.
mkrepo "$tmp/s"
cat > "$tmp/s/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  gate: local
  test_command: auto
EOF
git -C "$tmp/s" add .docket.yml; git -C "$tmp/s" commit --quiet -m cfg
git -C "$tmp/s" push --quiet origin main
out="$(run "$tmp/s" --export)"; eval "$out"
assert "test_command auto: FINALIZE_TEST_COMMAND empty" '[ -z "$FINALIZE_TEST_COMMAND" ]'
assert "test_command auto: FINALIZE_GATE still local"   '[ "$FINALIZE_GATE" = local ]'

# An explicit non-sentinel value is still honored verbatim (the sentinel is not a blanket clear).
mkrepo "$tmp/s2"
cat > "$tmp/s2/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: make test
EOF
git -C "$tmp/s2" add .docket.yml; git -C "$tmp/s2" commit --quiet -m cfg
git -C "$tmp/s2" push --quiet origin main
out="$(run "$tmp/s2" --export)"; eval "$out"
assert "explicit test_command honored verbatim" '[ "$FINALIZE_TEST_COMMAND" = "make test" ]'

# Case-sensitivity: only the literal lowercase `auto` is the sentinel (integration_branch precedent).
mkrepo "$tmp/s3"
cat > "$tmp/s3/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: AUTO
EOF
git -C "$tmp/s3" add .docket.yml; git -C "$tmp/s3" commit --quiet -m cfg
git -C "$tmp/s3" push --quiet origin main
out="$(run "$tmp/s3" --export)"; eval "$out"
assert "test_command AUTO is NOT the sentinel (case-sensitive)" '[ "$FINALIZE_TEST_COMMAND" = "AUTO" ]'

# --- (S4/S5/S6) change 0106: the sentinel's CROSS-LAYER masking -------------
# The collapse at scripts/docket-config.sh:201 runs AFTER the :194 resolution chain. That
# placement is the whole point: a HIGHER layer writing `test_command: auto` MASKS a LOWER
# layer's real command, which is the correct reading of an explicit re-statement of the
# default. Collapse per-layer instead and the behavior silently INVERTS — the higher `auto`
# becomes empty, the `:-` chain falls through, and the lower command resurfaces.
# Sections s/s2/s3 above are all single-layer, so none of them can see this. These do.
#
# s4 and s5 assert an EMPTY value, which is also what an ABSENT key yields — so each first
# asserts its lower rung really does resolve (the control), then adds the masking layer.
# Each eval is preceded by a poison value: an aborted run emits nothing, and a bare
# `eval ""` would otherwise leave the previous fixture's value standing.

# (s4) FORWARD, lcl() path: .docket.local.yml `auto` over a committed real command.
mkrepo "$tmp/s4"
mkdir -p "$tmp/s4.xdg/docket"
cat > "$tmp/s4/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: make test
EOF
git -C "$tmp/s4" add .docket.yml; git -C "$tmp/s4" commit --quiet -m cfg
git -C "$tmp/s4" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s4.xdg" "$tmp/s4" --export)"; eval "$out"
assert "0106 s4 control: committed real command resolves before masking" '[ "$FINALIZE_TEST_COMMAND" = "make test" ]'
printf 'finalize:\n  test_command: auto\n' > "$tmp/s4/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s4.xdg" "$tmp/s4" --export)"; eval "$out"
assert "0106 s4: local auto masks committed real command" '[ -z "$FINALIZE_TEST_COMMAND" ]'

# (s5) FORWARD, gbl() path: committed `auto` over a global real command.
mkrepo "$tmp/s5"
mkdir -p "$tmp/s5.xdg/docket"
printf 'finalize:\n  test_command: make global\n' > "$tmp/s5.xdg/docket/config.yml"
cat > "$tmp/s5/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/s5" add .docket.yml; git -C "$tmp/s5" commit --quiet -m cfg
git -C "$tmp/s5" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s5.xdg" "$tmp/s5" --export)"; eval "$out"
assert "0106 s5 control: global real command resolves before masking" '[ "$FINALIZE_TEST_COMMAND" = "make global" ]'
cat > "$tmp/s5/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: auto
EOF
git -C "$tmp/s5" add .docket.yml; git -C "$tmp/s5" commit --quiet -m cfg2
git -C "$tmp/s5" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s5.xdg" "$tmp/s5" --export)"; eval "$out"
assert "0106 s5: committed auto masks global real command" '[ -z "$FINALIZE_TEST_COMMAND" ]'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
