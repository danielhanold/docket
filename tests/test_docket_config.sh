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
# MKREPO_TEMPLATE: the baseline every mkrepo fixture is copied from; built once, eagerly,
#   at file scope (see the _mkrepo_build_template call below for why it cannot be lazy).
MKREPO_TEMPLATE=""
_mkrepo_build_template(){
  MKREPO_TEMPLATE="$tmp/.mkrepo-template"
  local dir="$MKREPO_TEMPLATE" bare="$MKREPO_TEMPLATE.origin.git"
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
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  mkdir -p "$(dirname "$dir")"
  rm -rf "$dir" "$bare"
  cp -R "$MKREPO_TEMPLATE" "$dir"
  cp -R "$MKREPO_TEMPLATE.origin.git" "$bare"
  git -C "$dir" remote set-url origin "$bare"
}
# run <dir> [args...] : run the resolver against <dir>, echo stdout
run(){ local d="$1"; shift; ensure_test_runtime "$XDG_CONFIG_HOME" "$d"; bash "$SCRIPT" --repo-dir "$d" "$@"; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# Built eagerly, at file scope, on purpose: mkrepo is reached from run_resolver_with,
# which callers consume inside command substitution -- i.e. in a SUBSHELL. A lazy
# `[ -n "$MKREPO_TEMPLATE" ] || _mkrepo_build_template` inside mkrepo would assign
# MKREPO_TEMPLATE only in that subshell, so the parent would still see it empty and
# rebuild the template on every single call -- correct, silently, and with no speedup.
_mkrepo_build_template

# --- fixture independence (change 0174) --------------------------------------
# mkrepo now copies a once-built template. These assertions pin the property that
# makes that safe: fixtures are independent of each other AND of the template.
# The "did advance" assert is anti-vacuity — without it, a silently failed push
# would make every "unchanged" assertion below pass for the wrong reason.
mkrepo "$tmp/indep-a"
mkrepo "$tmp/indep-b"
indep_tpl_before="$(git -C "$MKREPO_TEMPLATE.origin.git" rev-parse refs/heads/main)"
indep_b_before="$(git -C "$tmp/indep-b.origin.git" rev-parse refs/heads/main)"
echo mutated > "$tmp/indep-a/MUTATION.md"
git -C "$tmp/indep-a" add MUTATION.md
git -C "$tmp/indep-a" commit --quiet -m mutate
git -C "$tmp/indep-a" push --quiet origin main
assert "0174 independence: the mutated fixture's own origin DID advance (mutation was real)" \
  '[ "$(git -C "$tmp/indep-a.origin.git" rev-parse refs/heads/main)" != "$indep_tpl_before" ]'
assert "0174 independence: a sibling fixture's origin is untouched" \
  '[ "$(git -C "$tmp/indep-b.origin.git" rev-parse refs/heads/main)" = "$indep_b_before" ]'
assert "0174 independence: the template's origin is untouched" \
  '[ "$(git -C "$MKREPO_TEMPLATE.origin.git" rev-parse refs/heads/main)" = "$indep_tpl_before" ]'
assert "0174 independence: a sibling worktree never sees the mutation" \
  '[ ! -e "$tmp/indep-b/MUTATION.md" ]'
assert "0174 independence: the template worktree never sees the mutation" \
  '[ ! -e "$MKREPO_TEMPLATE/MUTATION.md" ]'
assert "0174 independence: each fixture points at its OWN origin" \
  '[ "$(git -C "$tmp/indep-a" config remote.origin.url)" = "$tmp/indep-a.origin.git" ]'
assert "0174 fixture parity: origin/HEAD still resolves after the copy" \
  '[ "$(git -C "$tmp/indep-b" rev-parse --abbrev-ref origin/HEAD)" = "origin/main" ]'

# Template integrity: the independence block above proves the property HERE; this snapshot
# plus the re-assertion just before the final exit extends it over every mkrepo call in the
# file, so a future test that dirties the shared template cannot go unnoticed.
tplint_refs="$(git -C "$MKREPO_TEMPLATE.origin.git" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort)"
tplint_head="$(git -C "$MKREPO_TEMPLATE" rev-parse HEAD)"
tplint_branch="$(git -C "$MKREPO_TEMPLATE" rev-parse --abbrev-ref HEAD)"

fake_bash(){ # fake_bash <path> <version> [executable]
  local path="$1" version="$2" executable="${3:-yes}"
  printf '#!/bin/sh\n[ "$1" = --version ] && { printf "%%s\\n" "%s"; exit 0; }\nexit 2\n' \
    "$version" >"$path"
  [ "$executable" = yes ] && chmod +x "$path"
}
mkdir -p "$tmp/runtime-bin"
fake_bash "$tmp/runtime-bin/default-bash" 'GNU bash, version 5.2.0(1)-release'

# Existing resolver fixtures predate the required runtime. Seed one into a writable machine-local
# layer unless a fixture already supplies an explicit runtime block (including an invalid one).
# Dedicated absence tests call the resolver directly and bypass this helper.
ensure_test_runtime(){ # ensure_test_runtime <xdg-root> <repo>
  local x="$1" d="$2" global="$1/docket/config.yml" local_cfg="$2/.docket.local.yml"
  if { [ -f "$global" ] && grep -q '^[[:space:]]*runtime[[:space:]]*:' "$global"; } \
     || { [ -f "$local_cfg" ] && grep -q '^[[:space:]]*runtime[[:space:]]*:' "$local_cfg"; }; then
    return
  fi
  if [ ! -e "$global" ] || [ -f "$global" ]; then
    mkdir -p "$x/docket"
    printf '\nruntime:\n  bash: %s\n' "$tmp/runtime-bin/default-bash" >>"$global"
  elif [ ! -e "$local_cfg" ] || [ -f "$local_cfg" ]; then
    printf '\nruntime:\n  bash: %s\n' "$tmp/runtime-bin/default-bash" >>"$local_cfg"
  fi
}

# Hermetic: never read the dev machine's real global config (change 0050 — docket-config.sh
# now reads ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml). Point XDG at a void.
export XDG_CONFIG_HOME="$tmp/xdg-void"
# rung <xdgdir> <repodir> [args...] : run the resolver with the global layer rooted at <xdgdir>
rung(){ local x="$1" d="$2"; shift 2; ensure_test_runtime "$x" "$d"; XDG_CONFIG_HOME="$x" bash "$SCRIPT" --repo-dir "$d" "$@"; }
rung_rc(){ local x="$1" d="$2"; shift 2; ensure_test_runtime "$x" "$d"; XDG_CONFIG_HOME="$x" bash "$SCRIPT" --repo-dir "$d" "$@" >/dev/null 2>&1; echo $?; }

# trace_external_commands TRACE REPO XDG: derive executable command names from one real resolver
# run. The shim guard below wraps only this observed population, never a hand-picked parser list.
trace_external_commands(){ # trace_external_commands TRACE REPO XDG -> NAME<TAB>ABSOLUTE_PATH
  local trace="$1" repo="$2" xdg="$3" traced word resolved
  XDG_CONFIG_HOME="$xdg" PS4='+ ' bash -x "$SCRIPT" --repo-dir "$repo" --export --format plain \
    >/dev/null 2>"$trace"
  while IFS= read -r traced; do
    while [[ "$traced" == +* ]]; do traced="${traced#+}"; done
    traced="${traced# }"
    traced="${traced#"${traced%%[![:space:]]*}"}"
    word="${traced%%[[:space:]]*}"
    case "$word" in ''|*=*|\[*|\{*|\}*|\(*|\)*|if|then|else|fi|for|in|do|done|case|esac|while) continue ;; esac
    resolved="$(type -P "$word" 2>/dev/null || true)"
    [ -n "$resolved" ] && printf '%s\t%s\n' "$word" "$resolved"
  done < "$trace"
}

# --- (A) absent .docket.yml -> all defaults (docket-mode) --------------------
mkrepo "$tmp/a"
AUTO_GROOM=__poison__; FINALIZE_GATE=__poison__; ADRS_DIR=__poison__; DOCKET_MODE=__poison__; DEFAULT_BRANCH=__poison__; METADATA_WORKTREE=__poison__; CHANGES_DIR=__poison__; INTEGRATION_BRANCH=__poison__; TERMINAL_PUBLISH=__poison__; BOARD_SURFACES=__poison__; FINALIZE_TEST_COMMAND=__poison__; METADATA_BRANCH=__poison__; RESULTS_DIR=__poison__
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
BOOTSTRAP=__poison__; DOCKET_MODE=__poison__; METADATA_WORKTREE=__poison__; METADATA_BRANCH=__poison__
out="$(run "$tmp/b" --export)"; eval "$out"
assert "main-mode: METADATA_BRANCH main"               '[ "$METADATA_BRANCH" = main ]'
assert "main-mode: DOCKET_MODE main"                   '[ "$DOCKET_MODE" = main ]'
assert "main-mode: METADATA_WORKTREE dot"              '[ "$METADATA_WORKTREE" = . ]'
assert "main-mode: BOOTSTRAP PROCEED"                  '[ "$BOOTSTRAP" = PROCEED ]'

# plain format absolutizes METADATA_WORKTREE in main-mode too: '.' -> the repo root itself
# (no /.docket suffix), covering the MW_EMIT="$REPO_ABS" branch in scripts/docket-config.sh.
b_abs="$(cd "$tmp/b" && pwd -P)"
b_plain="$(run "$tmp/b" --export --format plain)"
assert "plain format absolutizes METADATA_WORKTREE (main-mode => repo root)" 'grep <<<"$b_plain" -qxF "METADATA_WORKTREE=$b_abs"'

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
AUTO_GROOM=__poison__; FINALIZE_GATE=__poison__; ADRS_DIR=__poison__; CHANGES_DIR=__poison__; INTEGRATION_BRANCH=__poison__; FINALIZE_TEST_COMMAND=__poison__; BOARD_SURFACES=__poison__; RESULTS_DIR=__poison__
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
assert "board []: emits BOARD_SURFACES=none"           'grep <<<"$out" -qxF "BOARD_SURFACES=none"'
assert "board []: plain format never emits an empty BOARD_SURFACES" \
  '! grep <<<"$out_plain" -qxF "BOARD_SURFACES="'
assert "board []: shell format never emits quoted-empty BOARD_SURFACES" \
  '! grep <<<"$out" -qxF "BOARD_SURFACES='\'''\''"'
BOARD_SURFACES=__poison__
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
  'grep <<<"$out" -qxF "BOARD_SURFACES=none"'

# --- (E) direct-pipe caller (LEARNINGS #22: $() hides a dropped trailing \n) -
n="$(run "$tmp/c" --export | grep -c '=')"
assert "direct-pipe: 37 KEY=value lines emitted"       '[ "$n" -eq 37 ]'   # 34 -> 37: change 0276's DUMMY_MODE trio
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
BOOTSTRAP=__poison__
out="$(run "$tmp/b1" --export)"; eval "$out"
assert "2x2 migrated -> PROCEED"            '[ "$BOOTSTRAP" = PROCEED ]'

# (B2) fresh: ¬DOCKET ∧ ¬LIVE -> CREATE_ORPHAN
mkrepo "$tmp/b2"
BOOTSTRAP=__poison__
out="$(run "$tmp/b2" --export)"; eval "$out"
assert "2x2 fresh -> CREATE_ORPHAN"         '[ "$BOOTSTRAP" = CREATE_ORPHAN ]'

# (B3) existing single-branch: ¬DOCKET ∧ LIVE -> STOP_MIGRATE
mkrepo "$tmp/b3"; seed_live "$tmp/b3"
BOOTSTRAP=__poison__
out="$(run "$tmp/b3" --export)"; eval "$out"
assert "2x2 single-branch -> STOP_MIGRATE"  '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# (B4) half-migrated: DOCKET ∧ LIVE -> STOP_MIGRATE
mkrepo "$tmp/b4"; seed_live "$tmp/b4"; make_docket "$tmp/b4"
BOOTSTRAP=__poison__
out="$(run "$tmp/b4" --export)"; eval "$out"
assert "2x2 half-migrated -> STOP_MIGRATE"  '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# --- opt-in --bootstrap write (the only mutation; guarded to ¬DOCKET ∧ ¬LIVE) ---
origin_has_docket(){ git -C "$1.origin.git" rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1; }

# (W1) default --export in fresh cell: NO write, verdict CREATE_ORPHAN
mkrepo "$tmp/w1"
BOOTSTRAP=__poison__
out="$(run "$tmp/w1" --export)"; eval "$out"
assert "read-only default: no orphan created" '! origin_has_docket "$tmp/w1"'
assert "read-only default: verdict CREATE_ORPHAN" '[ "$BOOTSTRAP" = CREATE_ORPHAN ]'

# (W2) --bootstrap in fresh cell: creates origin/docket, re-reports PROCEED
mkrepo "$tmp/w2"
BOOTSTRAP=__poison__
out="$(run "$tmp/w2" --bootstrap --export)"; eval "$out"
assert "bootstrap fresh: origin/docket created" 'origin_has_docket "$tmp/w2"'
assert "bootstrap fresh: verdict now PROCEED"   '[ "$BOOTSTRAP" = PROCEED ]'

# (W2-gi) --bootstrap in the fresh cell also SEEDS the managed .gitignore block in the
# primary tree, prints a loud COMMIT notice, and commits NOTHING (change 0057).
w2gi="$tmp/w2gi"; mkrepo "$w2gi"                       # fresh docket-mode repo (¬DOCKET ∧ ¬LIVE)
head_before="$(git -C "$w2gi" rev-parse HEAD 2>/dev/null || echo none)"
bs_err="$(run "$w2gi" --bootstrap --export 2>&1 >/dev/null)"
assert "0057 bootstrap: block seeded in primary tree" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$w2gi/.gitignore"'
assert "0057 bootstrap: loud COMMIT notice printed"   'grep <<<"$bs_err" -qi "commit"'
assert "0057 bootstrap: nothing auto-committed"       '[ "$(git -C "$w2gi" rev-parse HEAD 2>/dev/null || echo none)" = "$head_before" ]'
assert "0057 bootstrap: .gitignore left UNstaged"     '[ -z "$(git -C "$w2gi" diff --cached --name-only 2>/dev/null)" ]'

# (W1-gi) default --export in the fresh cell stays strictly READ-ONLY: no .gitignore written.
w1gi="$tmp/w1gi"; mkrepo "$w1gi"
run "$w1gi" --export >/dev/null 2>&1
assert "0057 export: read-only — no .gitignore seeded" '[ ! -e "$w1gi/.gitignore" ]'

# (W3) --bootstrap in STOP_MIGRATE cell: GUARD holds — no orphan written
mkrepo "$tmp/w3"; seed_live "$tmp/w3"
BOOTSTRAP=__poison__
out="$(run "$tmp/w3" --bootstrap --export)"; eval "$out"
assert "bootstrap guard: no write in single-branch cell" '! origin_has_docket "$tmp/w3"'
assert "bootstrap guard: verdict stays STOP_MIGRATE"     '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# (W4) --bootstrap in migrated cell: idempotent no-op, PROCEED (origin/docket SHA unchanged)
mkrepo "$tmp/w4"; make_docket "$tmp/w4"
w4_before="$(git -C "$tmp/w4.origin.git" rev-parse refs/heads/docket)"
BOOTSTRAP=__poison__
out="$(run "$tmp/w4" --bootstrap --export)"; eval "$out"
w4_after="$(git -C "$tmp/w4.origin.git" rev-parse refs/heads/docket)"
assert "bootstrap migrated: PROCEED"            '[ "$BOOTSTRAP" = PROCEED ]'
assert "bootstrap migrated: origin/docket SHA unchanged (no-op)" '[ "$w4_before" = "$w4_after" ]'

# --- fail-closed error paths (non-zero exit, stderr diagnostic, no KEY=value) ----
run_rc(){ local d="$1"; shift; ensure_test_runtime "$XDG_CONFIG_HOME" "$d"; bash "$SCRIPT" --repo-dir "$d" "$@" >/dev/null 2>&1; echo $?; }

# (F1) unreachable origin -> exit≠0, no output
mkrepo "$tmp/f1"
rm -rf "$tmp/f1.origin.git"                       # destroy the remote
assert "unreachable origin: nonzero exit" '[ "$(run_rc "$tmp/f1" --export)" -ne 0 ]'
assert "unreachable origin: emits nothing" '[ -z "$(bash "$SCRIPT" --repo-dir "$tmp/f1" --export 2>/dev/null)" ]'

# (F1a) a fetch failure preserves Git's real diagnostics behind Docket's neutral wrapper.
# The fake scans all argv because g() prepends -C <repo> before the Git subcommand.
fake_git="$tmp/fetch-fail-git"
cat >"$fake_git" <<'EOF'
#!/usr/bin/env bash
for arg in "$@"; do
  if [ "$arg" = fetch ]; then
    printf 'fake-git: permission boundary denied\n' >&2
    printf 'fake-git: cannot write origin lock\n' >&2
    exit 47
  fi
done
exit 0
EOF
chmod +x "$fake_git"
mkrepo "$tmp/fetch-failure"
fetch_rc=0
fetch_stdout="$(GIT="$fake_git" bash "$SCRIPT" --repo-dir "$tmp/fetch-failure" --export 2>"$tmp/fetch-failure.err")" || fetch_rc=$?
fetch_stderr="$(<"$tmp/fetch-failure.err")"
fetch_first_line="${fetch_stderr%%$'\n'*}"
assert "fetch failure: nonzero exit" '[ "$fetch_rc" -ne 0 ]'
assert "fetch failure: emits no KEY=value stdout" '[ -z "$fetch_stdout" ]'
assert "fetch failure: neutral wrapper is the first stderr line" \
  '[ "$fetch_first_line" = "docket-config: git fetch origin failed" ]'
assert "fetch failure: neutral wrapper is present" \
  'grep -qxF "docket-config: git fetch origin failed" <<<"$fetch_stderr"'
assert "fetch failure: first fake diagnostic is preserved" \
  'grep -qxF "fake-git: permission boundary denied" <<<"$fetch_stderr"'
assert "fetch failure: second fake diagnostic is preserved" \
  'grep -qxF "fake-git: cannot write origin lock" <<<"$fetch_stderr"'
assert "fetch failure: old network diagnosis is absent" \
  '! grep -qF "cannot reach origin" <<<"$fetch_stderr" && ! grep -qF "check the remote/network" <<<"$fetch_stderr"'

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
assert "bad metadata_branch: diagnostic mentions metadata_branch" 'grep <<<"$err" -q metadata_branch'

# (F5) --repo-dir with no following argument -> usage error (exit≠0 + diagnostic), no set -u crash
rc5=0; err5="$(bash "$SCRIPT" --repo-dir 2>&1 >/dev/null)" || rc5=$?
assert "F5 --repo-dir no arg: nonzero exit" '[ "$rc5" -ne 0 ]'
assert "F5 --repo-dir no arg: diagnostic mentions --repo-dir" 'grep <<<"$err5" -q -- "--repo-dir"'

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

# --- (G) skills: absent -> the five built-in defaults (three superpowers, two docket-owned) ---
mkrepo "$tmp/g"
printf 'metadata_branch: main\n' > "$tmp/g/.docket.yml"
git -C "$tmp/g" add .docket.yml; git -C "$tmp/g" commit --quiet -m cfg; git -C "$tmp/g" push --quiet origin main
SKILL_BUILD=__poison__; SKILL_REVIEW=__poison__; SKILL_BRAINSTORM=__poison__; SKILL_PLAN=__poison__; SKILL_FINISH=__poison__
out="$(run "$tmp/g" --export)"; eval "$out"
assert "skills absent: BRAINSTORM default" '[ "$SKILL_BRAINSTORM" = superpowers:brainstorming ]'
assert "skills absent: PLAN default"       '[ "$SKILL_PLAN" = superpowers:writing-plans ]'
assert "skills absent: BUILD default"      '[ "$SKILL_BUILD" = docket-build ]'
assert "skills absent: REVIEW default"     '[ "$SKILL_REVIEW" = docket-review ]'
# The old superpowers defaults must be GONE, not merely shadowed — a resolver that emitted both
# (or fell through to SDD on some layer path) would satisfy a presence-only assert.
assert "skills absent: BUILD default is no longer SDD" \
  '[ "$SKILL_BUILD" != superpowers:subagent-driven-development ]'
assert "skills absent: REVIEW default is no longer superpowers review" \
  '[ "$SKILL_REVIEW" != superpowers:requesting-code-review ]'
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
SKILL_BUILD=__poison__; SKILL_REVIEW=__poison__; SKILL_PLAN=__poison__
out="$(run "$tmp/h" --export)"; eval "$out"
assert "skills auto: BUILD is auto"         '[ "$SKILL_BUILD" = auto ]'
assert "skills custom: REVIEW verbatim"     '[ "$SKILL_REVIEW" = my-org:custom-review ]'
assert "skills partial: PLAN still default" '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# --- (I) skills: TAB-indented block parses (LEARNINGS #46 — whitespace class) ---
mkrepo "$tmp/i"
printf 'metadata_branch: main\nskills:\n\tplan: auto\n' > "$tmp/i/.docket.yml"
git -C "$tmp/i" add .docket.yml; git -C "$tmp/i" commit --quiet -m cfg; git -C "$tmp/i" push --quiet origin main
SKILL_PLAN=__poison__
out="$(run "$tmp/i" --export)"; eval "$out"
assert "skills tab-indent: PLAN auto"       '[ "$SKILL_PLAN" = auto ]'

# --- (0176) snapshot reader boundaries: scalar + nested-map first-match rules ---
# All three layers participate so the resolver's output, rather than a private helper, pins the
# immutable snapshot contract. The controls before each boundary assertion ensure a missing
# fixture/layer cannot make the mutation-sensitive assertion pass vacuously.
mkrepo "$tmp/snapshot"
mkdir -p "$tmp/snapshot.xdg/docket"
cat > "$tmp/snapshot.xdg/docket/config.yml" <<'EOF'
skills:
  plan: global-plan
EOF
cat > "$tmp/snapshot/.docket.yml" <<'EOF'
metadata_branch: main
change_types: [spike]
change_types: [feat]
build: wrong-top-level-value
skills:
	build: auto
review: wrong-top-level-review
skills:
  build: wrong-later-block
  review: committed-review
EOF
git -C "$tmp/snapshot" add .docket.yml
git -C "$tmp/snapshot" commit --quiet -m cfg
git -C "$tmp/snapshot" push --quiet origin main
cat > "$tmp/snapshot/.docket.local.yml" <<'EOF'
skills:
  finish: local-finish
EOF
snapshot_out="$(rung "$tmp/snapshot.xdg" "$tmp/snapshot" --export --format plain)"
assert "0176 snapshot fixture loads the global layer" \
  'grep -qxF "SKILL_PLAN=global-plan" <<<"$snapshot_out"'
assert "0176 snapshot reader keeps a tab-indented skills leaf over a bare top-level leaf" \
  'grep -qxF "SKILL_BUILD=auto" <<<"$snapshot_out"'
assert "0176 snapshot fixture loads the local layer" \
  'grep -qxF "SKILL_FINISH=local-finish" <<<"$snapshot_out"'
assert "0176 snapshot reader keeps the first duplicate scalar" \
  'grep -qxF "CHANGE_TYPES=spike" <<<"$snapshot_out"'
assert "0176 snapshot fixture reaches the second skills block after top-level content" \
  'grep -qxF "SKILL_BUILD=auto" <<<"$snapshot_out"'
assert "0176 snapshot reader returns the first matching leaf across repeated blocks and exits at top-level content" \
  'grep -qxF "SKILL_REVIEW=committed-review" <<<"$snapshot_out"'

# The resolver's command-cost ceiling is measured by interposing wrappers on precisely the
# executable command population observed in an unmodified resolver trace. This guards actual
# process execution rather than a source-level list of parser spellings.
spawn_trace="$tmp/0176-resolver.trace"
spawn_shims="$tmp/0176-command-shims"
spawn_log="$tmp/0176-spawn.log"
mkdir -p "$spawn_shims"
while IFS=$'\t' read -r command_name command_path; do
  case "$command_name" in *[![:alnum:]_+-]*|'') continue ;; esac
  printf '#!/usr/bin/env bash\nprintf "%%s\\n" "${0##*/}" >> %q\nexec %q "$@"\n' \
    "$spawn_log" "$command_path" > "$spawn_shims/$command_name"
  chmod +x "$spawn_shims/$command_name"
done < <(trace_external_commands "$spawn_trace" "$tmp/snapshot" "$tmp/snapshot.xdg")
spawn_out="$(PATH="$spawn_shims:$PATH" rung "$tmp/snapshot.xdg" "$tmp/snapshot" --export --format plain)"
assert "0176 spawn guard preserves representative resolved export" \
  'grep -qxF "SKILL_BUILD=auto" <<<"$spawn_out"'
command_count="$(wc -l < "$spawn_log" | tr -d '[:space:]')"
assert "0176 spawn guard measured a non-empty command population" \
  '[ "$command_count" -gt 0 ]'
assert "0176 snapshot resolver stays under the spawned-command ceiling" \
  '[ "$command_count" -le 120 ]'

# --- (J) skills: unknown role key -> warned on stderr, ignored; known keys still resolve ---
mkrepo "$tmp/j"
printf 'metadata_branch: main\nskills:\n  bogus: x\n  plan: auto\n' > "$tmp/j/.docket.yml"
git -C "$tmp/j" add .docket.yml; git -C "$tmp/j" commit --quiet -m cfg; git -C "$tmp/j" push --quiet origin main
jerr="$(run "$tmp/j" --export 2>&1 >/dev/null)"
SKILL_PLAN=__poison__
out="$(run "$tmp/j" --export 2>/dev/null)"; eval "$out"
assert "skills unknown key: warned on stderr"       'grep <<<"$jerr" -qi "unknown skills role"'
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
SKILL_BUILD=__poison__; AUTO_GROOM=__poison__; FINALIZE_GATE=__poison__; BOARD_SURFACES=__poison__; SKILL_PLAN=__poison__
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
SKILL_BUILD=__poison__; AUTO_GROOM=__poison__; SKILL_REVIEW=__poison__; SKILL_PLAN=__poison__
out="$(rung "$tmp/l.xdg" "$tmp/l" --export)"; eval "$out"
assert "0050 L: per-repo auto_groom false beats global true" '[ "$AUTO_GROOM" = false ]'
assert "0050 L: skills merge — repo plan wins over global"   '[ "$SKILL_PLAN" = superpowers:writing-plans ]'
assert "0050 L: skills merge — global review holds"          '[ "$SKILL_REVIEW" = my-org:global-review ]'
assert "0050 L: skills merge — unset role stays default"     '[ "$SKILL_BUILD" = docket-build ]'

# --- (Q) XDG_CONFIG_HOME honored; HOME/.config is the fallback ---------------
mkrepo "$tmp/q"
mkdir -p "$tmp/q.home/.config/docket"
printf 'auto_groom: true\nruntime:\n  bash: %s\n' "$tmp/runtime-bin/default-bash" > "$tmp/q.home/.config/docket/config.yml"
AUTO_GROOM=__poison__
out="$(env -u XDG_CONFIG_HOME HOME="$tmp/q.home" bash "$SCRIPT" --repo-dir "$tmp/q" --export)"; eval "$out"
assert "0050 Q: XDG unset -> \$HOME/.config fallback read"   '[ "$AUTO_GROOM" = true ]'

# --- (E') emit-interface guard: exactly 37 lines with a global file present ---
# 34 -> 37 (change 0276): the DUMMY_MODE_{ENABLED,PERSONA,SURFACES} trio.
n50="$(rung "$tmp/k.xdg" "$tmp/k" --export | grep -c '=')"
assert "0050 E': 37 KEY=value lines with global layer" '[ "$n50" -eq 37 ]'

# --- (M) coordination-key fence: warned-and-ignored, never honored, never fatal ---
mkrepo "$tmp/m"
mkdir -p "$tmp/m.xdg/docket"
cat > "$tmp/m.xdg/docket/config.yml" <<'EOF'
metadata_branch: main
changes_dir: elsewhere/changes
auto_groom: true
EOF
merr="$(rung "$tmp/m.xdg" "$tmp/m" --export 2>&1 >/dev/null)"
AUTO_GROOM=__poison__; CHANGES_DIR=__poison__; METADATA_BRANCH=__poison__
out="$(rung "$tmp/m.xdg" "$tmp/m" --export 2>/dev/null)"; eval "$out"
assert "0050 M: fence warns metadata_branch"        'grep <<<"$merr" -q "metadata_branch"'
assert "0050 M: fence names per-repo-only"          'grep <<<"$merr" -qi "per-repo-only"'
assert "0050 M: fence warns changes_dir"            'grep <<<"$merr" -q "changes_dir"'
assert "0050 M: global metadata_branch NOT honored" '[ "$METADATA_BRANCH" = docket ]'
assert "0050 M: CHANGES_DIR stays default"          '[ "$CHANGES_DIR" = docs/changes ]'
assert "0050 M: global-able key in same file still honored" '[ "$AUTO_GROOM" = true ]'
assert "0050 M: fence is not fatal (exit 0)"        '[ "$(rung_rc "$tmp/m.xdg" "$tmp/m" --export)" -eq 0 ]'

# --- (N) global board_surfaces: github token dropped; [] and [inline] work -------
mkrepo "$tmp/n"
mkdir -p "$tmp/n.xdg/docket"
printf 'board_surfaces: [inline, github]\n' > "$tmp/n.xdg/docket/config.yml"
nerr="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>&1 >/dev/null)"
BOARD_SURFACES=__poison__
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"; eval "$out"
assert "0050 N: global github token warned"         'grep <<<"$nerr" -q "github"'
assert "0050 N: global github token dropped"        '[ "$BOARD_SURFACES" = inline ]'
printf 'board_surfaces: []\n' > "$tmp/n.xdg/docket/config.yml"
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"
assert "0050 N: global [] honored (board disabled, encoded as none)" \
  'grep <<<"$out" -qxF "BOARD_SURFACES=none"'
eval "$out"
# per-repo github is untouched by the fence:
mkrepo "$tmp/n2"
printf 'metadata_branch: main\nboard_surfaces: [inline, github]\n' > "$tmp/n2/.docket.yml"
git -C "$tmp/n2" add .docket.yml; git -C "$tmp/n2" commit --quiet -m cfg
git -C "$tmp/n2" push --quiet origin main
n2err="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>&1 >/dev/null)"
BOARD_SURFACES=__poison__
out="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>/dev/null)"; eval "$out"
assert "0050 N: per-repo github honored"            '[ "$BOARD_SURFACES" = "inline github" ]'
assert "0050 N: per-repo github NOT warned"         '! grep <<<"$n2err" -q "board_surfaces token github"'

# --- (O) misplacement guard: ~/.config/docket/.docket.yml is warned, never read ---
mkrepo "$tmp/o"
mkdir -p "$tmp/o.xdg/docket"
printf 'auto_groom: true\n' > "$tmp/o.xdg/docket/.docket.yml"
oerr="$(rung "$tmp/o.xdg" "$tmp/o" --export 2>&1 >/dev/null)"
AUTO_GROOM=__poison__
out="$(rung "$tmp/o.xdg" "$tmp/o" --export 2>/dev/null)"; eval "$out"
assert "0050 O: misplacement warned, names config.yml" 'grep <<<"$oerr" -q "config.yml"'
assert "0050 O: misplaced file NOT read (auto_groom default)" '[ "$AUTO_GROOM" = false ]'
assert "0050 O: misplacement not fatal (exit 0)"    '[ "$(rung_rc "$tmp/o.xdg" "$tmp/o" --export)" -eq 0 ]'

# --- (P) malformed global file: warned, built-ins fallback, repos not bricked -----
mkrepo "$tmp/p"
mkdir -p "$tmp/p.xdg/docket/config.yml"            # a DIRECTORY at the config path
perr="$(rung "$tmp/p.xdg" "$tmp/p" --export 2>&1 >/dev/null)"
AUTO_GROOM=__poison__
out="$(rung "$tmp/p.xdg" "$tmp/p" --export 2>/dev/null)"; eval "$out"
assert "0050 P: malformed global warned"            'grep <<<"$perr" -qi "not a readable regular file"'
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
SKILL_BUILD=__poison__; SKILL_REVIEW=__poison__; SKILL_PLAN=__poison__
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
AUTO_GROOM=__poison__; FINALIZE_GATE=__poison__; FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/xdg-l2" "$tmp/l2" --export)"; eval "$out"
assert "0051 L2: local auto_groom beats committed"       '[ "$AUTO_GROOM" = true ]'
assert "0051 L2: local finalize.gate beats global"       '[ "$FINALIZE_GATE" = both ]'
assert "0051 L2: local finalize.test_command honored"    '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'

# (L3) fenced keys in the local file: loudly warned-and-ignored, never honored, never fatal.
mkrepo "$tmp/l3"
printf 'metadata_branch: main\nchanges_dir: sneaky/changes\ngithub_project: {owner: x, number: 1}\n' > "$tmp/l3/.docket.local.yml"
errout="$(rung "$tmp/l3-noxdg" "$tmp/l3" --export 2>&1 >/dev/null)"; rc=$?
CHANGES_DIR=__poison__; METADATA_BRANCH=__poison__
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
BOARD_SURFACES=__poison__
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
BOARD_SURFACES=__poison__
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
TERMINAL_PUBLISH=__poison__
out="$(run "$tmp/tp" --export)"; eval "$out"
assert "0064: repo terminal_publish false is honored" '[ "$TERMINAL_PUBLISH" = false ]'

# explicit true round-trips
mkrepo "$tmp/tp2"
printf 'metadata_branch: docket\nterminal_publish: true\n' > "$tmp/tp2/.docket.yml"
git -C "$tmp/tp2" add .docket.yml; git -C "$tmp/tp2" commit --quiet -m cfg
git -C "$tmp/tp2" push --quiet origin main
TERMINAL_PUBLISH=__poison__
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
assert "0064 fence: global terminal_publish warns"        'grep <<<"$tperr" -q "terminal_publish"'
assert "0064 fence: warning says per-repo-only"           'grep <<<"$tperr" -qi "per-repo-only"'
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
assert "0064 fence: .docket.local.yml terminal_publish warns" 'grep <<<"$lerr" -q "terminal_publish"'
assert "0064 fence: local names .docket.local.yml"            'grep <<<"$lerr" -q ".docket.local.yml"'
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
assert "shell format still %q-quotes empty values" 'grep <<<"$shell_out" -qxF "FINALIZE_TEST_COMMAND='\'''\''"'
assert "shell format METADATA_WORKTREE stays relative .docket" 'grep <<<"$shell_out" -qxF "METADATA_WORKTREE=.docket"'

# plain format: raw KEY=value, no %q, no export prefix, empty => bare "KEY="
plain_out="$(run "$tmp/fmt" --export --format plain)"
assert "plain format emits raw empty value (no quotes)" 'grep <<<"$plain_out" -qxF "FINALIZE_TEST_COMMAND="'
assert "plain format has no export prefix" '! grep <<<"$plain_out" -q "^export "'
assert "plain format emits BOOTSTRAP" 'grep <<<"$plain_out" -qxF "BOOTSTRAP=PROCEED"'
assert "plain format emits raw enum values unquoted" 'grep <<<"$plain_out" -qxF "DOCKET_MODE=docket"'
# METADATA_WORKTREE absolutized in plain mode
fmt_abs="$(cd "$tmp/fmt" && pwd -P)"
assert "plain format absolutizes METADATA_WORKTREE (docket-mode)" 'grep <<<"$plain_out" -qxF "METADATA_WORKTREE=$fmt_abs/.docket"'
assert "plain format keeps CHANGES_DIR as repo-relative subpath" 'grep <<<"$plain_out" -qxF "CHANGES_DIR=docs/changes"'

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
  'grep <<<"$z_root_plain" -qxF "REPO_ROOT=$z_abs"'

# plain format from the .docket/ LINKED WORKTREE: byte-identical REPO_ROOT and METADATA_WORKTREE.
# Pre-0075 this yielded REPO_ROOT=<repo>/.docket and METADATA_WORKTREE=<repo>/.docket/.docket.
z_dk_plain="$(cd "$tmp/z/.docket" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT from the .docket/ worktree is the MAIN root, not .docket" \
  'grep <<<"$z_dk_plain" -qxF "REPO_ROOT=$z_abs"'
assert "0075 plain: METADATA_WORKTREE from .docket/ is <root>/.docket, NOT <root>/.docket/.docket" \
  'grep <<<"$z_dk_plain" -qxF "METADATA_WORKTREE=$z_abs/.docket"'

# plain format from a SUBDIRECTORY: the spec's stated behavior CHANGE (§1) — pinned deliberately.
z_sub_plain="$(cd "$tmp/z/sub" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT from <repo>/sub is the repo root (§1 behavior change, pinned)" \
  'grep <<<"$z_sub_plain" -qxF "REPO_ROOT=$z_abs"'
assert "0075 plain: METADATA_WORKTREE from <repo>/sub is <root>/.docket, not <sub>/.docket" \
  'grep <<<"$z_sub_plain" -qxF "METADATA_WORKTREE=$z_abs/.docket"'

# The machine-local layer is read from the REPO ROOT even when invoked from a subdirectory
# (§1: LCFG="$REPO_DIR/.docket.local.yml"). auto_groom is a global-able (non-fenced) key.
printf 'auto_groom: true\n' > "$tmp/z/.docket.local.yml"
z_sub_shell="$(cd "$tmp/z/sub" && bash "$SCRIPT" --export)"
AUTO_GROOM=__poison__; eval "$z_sub_shell"
assert "0075: <repo>/.docket.local.yml is read when invoked from <repo>/sub (§1 behavior change)" \
  '[ "$AUTO_GROOM" = true ]'
rm -f "$tmp/z/.docket.local.yml"

# REPO_ROOT is PLAIN-ONLY: ensure-claude-settings.sh sets its own REPO_ROOT and eval's the SHELL
# export, so a shell-format REPO_ROOT would silently capture that name. Assert BOTH directions so
# the guard is provably able to fire (a bare `! grep` that can never match proves nothing).
z_shell="$(cd "$tmp/z" && bash "$SCRIPT" --export)"
assert "0075 shell: REPO_ROOT is NOT emitted (would capture ensure-claude-settings.sh's own var)" \
  '! grep <<<"$z_shell" -q "^REPO_ROOT="'
assert "0075 control: the plain export DOES carry REPO_ROOT (proves the absence-assert can fire)" \
  'grep <<<"$z_root_plain" -q "^REPO_ROOT="'

# --repo-dir still overrides verbatim, from anywhere (the whole existing suite depends on it).
mkrepo "$tmp/z2"
z2_abs="$(cd "$tmp/z2" && pwd -P)"
z2_plain="$(cd "$tmp/z/.docket" && bash "$SCRIPT" --repo-dir "$tmp/z2" --export --format plain)"
assert "0075: --repo-dir still overrides the anchor verbatim" \
  'grep <<<"$z2_plain" -qxF "REPO_ROOT=$z2_abs"'

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
assert "no fence warning for learnings keys" '! grep <<<"$lrn_c_err" -qi "learnings.*per-repo-only"'

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
  'grep <<<"$lrn_f1_err" -qF "learnings.enabled"'

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
  'grep <<<"$lrn_f2_err" -qF "learnings.cap"'

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
assert "RECLAIM_LEASE_TTL defaults to 72" 'grep <<<"$out" -qxF "RECLAIM_LEASE_TTL=72"'
assert "RECLAIM_AUTO defaults to false"   'grep <<<"$out" -qxF "RECLAIM_AUTO=false"'

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
assert "RECLAIM_LEASE_TTL reads the block" 'grep <<<"$out2" -qxF "RECLAIM_LEASE_TTL=12"'
assert "RECLAIM_AUTO reads the block"      'grep <<<"$out2" -qxF "RECLAIM_AUTO=true"'

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
assert "no fence warning for reclaim keys" '! grep <<<"$rcl_c_err" -qi "reclaim.*per-repo-only"'

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
  'grep <<<"$rcl_f1_err" -qF "reclaim.lease_ttl"'
rcl_f2_err="$(run_resolver_with "reclaim:\n  auto: maybe\n" 2>&1 >/dev/null)"
assert "unparseable reclaim.auto: mentions reclaim.auto" \
  'grep <<<"$rcl_f2_err" -qF "reclaim.auto"'

# ============================================================================
# Change 0167 — the build: block (BUILD_CHECKPOINT)
# NOTE (guards-are-code (e)): clear the asserted vars BEFORE each eval — an aborting run emits
# NOTHING, and eval "" would silently leave the previous case's value in place.
# ============================================================================

# --- (BLD-a) default when no layer sets the block -----------------------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-a"
out="$(run "$tmp/bld-a" --export)"; eval "$out"
assert "BUILD_CHECKPOINT defaults to false" 'grep <<<"$out" -qxF "BUILD_CHECKPOINT=false"'

# --- (BLD-b) repo-committed block is honored ----------------------------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-b"
cat > "$tmp/bld-b/.docket.yml" <<'EOF'
metadata_branch: main
build:
  checkpoint: true
EOF
git -C "$tmp/bld-b" add .docket.yml; git -C "$tmp/bld-b" commit --quiet -m cfg
git -C "$tmp/bld-b" push --quiet origin main
out2="$(run "$tmp/bld-b" --export)"; eval "$out2"
assert "BUILD_CHECKPOINT reads the block" 'grep <<<"$out2" -qxF "BUILD_CHECKPOINT=true"'

# --- (BLD-c) global-able (ADR-0019 — NOT coordination-fenced) -----------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-c"
mkdir -p "$tmp/bld-c.xdg/docket"
cat > "$tmp/bld-c.xdg/docket/config.yml" <<'EOF'
build:
  checkpoint: true
EOF
bld_c_err="$(rung "$tmp/bld-c.xdg" "$tmp/bld-c" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/bld-c.xdg" "$tmp/bld-c" --export 2>/dev/null)"; eval "$out"
assert "build.checkpoint is global-able (not fenced)" '[ "$BUILD_CHECKPOINT" = "true" ]'
assert "no fence warning for build.checkpoint" '! grep <<<"$bld_c_err" -qi "build.*per-repo-only"'

# --- (BLD-d) repo-local layer wins over repo-committed ------------------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-d"
cat > "$tmp/bld-d/.docket.yml" <<'EOF'
metadata_branch: main
build:
  checkpoint: true
EOF
git -C "$tmp/bld-d" add .docket.yml; git -C "$tmp/bld-d" commit --quiet -m cfg
git -C "$tmp/bld-d" push --quiet origin main
printf 'build:\n  checkpoint: false\n' > "$tmp/bld-d/.docket.local.yml"
out="$(run "$tmp/bld-d" --export)"; eval "$out"
assert "local layer beats repo-committed for build.checkpoint" '[ "$BUILD_CHECKPOINT" = "false" ]'

# --- (BLD-e) SHADOW GUARD — a bare checkpoint: OUTSIDE the build: block --------
# must not leak in. This is the whole reason the block is read via yaml_block_body: `checkpoint`
# is a generic word another block could otherwise shadow.
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-e"
cat > "$tmp/bld-e/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  checkpoint: true
EOF
git -C "$tmp/bld-e" add .docket.yml; git -C "$tmp/bld-e" commit --quiet -m cfg
git -C "$tmp/bld-e" push --quiet origin main
out="$(run "$tmp/bld-e" --export)"; eval "$out"
assert "a foreign block's checkpoint: does not shadow build.checkpoint" '[ "$BUILD_CHECKPOINT" = "false" ]'

# --- (BLD-f) fail closed on garbage -------------------------------------------
assert "non-bool checkpoint aborts nonzero" '! run_resolver_with "build:\n  checkpoint: maybe\n" >/dev/null 2>&1'
bld_f_err="$(run_resolver_with "build:\n  checkpoint: maybe\n" 2>&1 >/dev/null)"
assert "unparseable build.checkpoint: mentions build.checkpoint" \
  'grep <<<"$bld_f_err" -qF "build.checkpoint"'

# --- (BLD-g) export presence and POSITION -------------------------------------
# Position matters: scripts/docket-config.md documents the order as a contract, and pipe
# consumers may rely on it. Anchor on the neighbour rather than a bare "is present" (the 0102 R7
# precedent) — identity, not just adjacency, so a reorder is caught by name, not by line count.
out_g="$(run "$tmp/bld-a" --export)"
out_g_plain="$(run "$tmp/bld-a" --export --format plain)"
assert "BUILD_CHECKPOINT is emitted" \
  'grep -q "^BUILD_CHECKPOINT=" <<<"$out_g"'
assert "BUILD_CHECKPOINT is emitted directly after RECLAIM_AUTO" \
  '[ "$(grep -n "^BUILD_CHECKPOINT=" <<<"$out_g" | cut -d: -f1)" \
     = "$(( $(grep -n "^RECLAIM_AUTO=" <<<"$out_g" | cut -d: -f1) + 1 ))" ]'
assert "BUILD_CHECKPOINT present in plain format too" \
  'grep -q "^BUILD_CHECKPOINT=" <<<"$out_g_plain"'

# --- (BLD-h) the contract doc documents it ------------------------------------
assert "docket-config.md has a build.checkpoint table row" \
  'grep -qE "^\| \`build\.checkpoint\` \| \`false\` \| yes \|" "$REPO/scripts/docket-config.md"'
assert "docket-config.md lists the export name" \
  'grep -q "^BUILD_CHECKPOINT$" "$REPO/scripts/docket-config.md"'

# ============================================================================
# Change 0218 — the review: block (REVIEW_MIN_FIX_SEVERITY)
# Structural clone of the build: block above. NOTE (guards-are-code (e)): clear the asserted var
# BEFORE each eval — an aborting run emits NOTHING, and eval "" would silently leave the previous
# case's value in place.
#
# ORDERING: the export-SHAPE asserts (presence, position, plain-format) run FIRST, before any
# fixture that dereferences $REVIEW_MIN_FIX_SEVERITY. Under `set -u` a missing export does not
# redden an assert — `eval ""` leaves the variable unbound and assert()'s own eval kills the suite
# — so a shape assert placed after a deref-ing fixture can only ever be reached when it was going
# to pass anyway. Mutation-verified: deleting the `emit REVIEW_MIN_FIX_SEVERITY` line reddens
# `REVIEW_MIN_FIX_SEVERITY is emitted` by name in this order; in the order this block used to
# have, the same deletion killed the run at the first deref-ing fixture and never reached it.
# ============================================================================

# --- (RMF-a) default when no layer sets the block -----------------------------
# Asserted on the EMITTED LINE, never on the variable — RMX-a's rule. This fixture sits ahead of
# even the shape asserts because it builds the repo they read, so it is the one assert that must
# survive the export not existing at all. Do NOT "simplify" it to
# `[ "$REVIEW_MIN_FIX_SEVERITY" = minor ]`: the eval below is deliberately unread, and a deref here
# would abort the suite under `set -u` instead of reddening this assert by name.
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-a"
out="$(run "$tmp/rmf-a" --export)"; eval "$out"
assert "REVIEW_MIN_FIX_SEVERITY defaults to minor" \
  'grep <<<"$out" -qxF "REVIEW_MIN_FIX_SEVERITY=minor"'

# --- (RMF-b) export presence, POSITION, and both formats ----------------------
out_rmf="$(run "$tmp/rmf-a" --export)"
out_rmf_plain="$(run "$tmp/rmf-a" --export --format plain)"
assert "REVIEW_MIN_FIX_SEVERITY is emitted" \
  'grep -q "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmf"'
assert "REVIEW_MIN_FIX_SEVERITY is emitted directly after BUILD_CHECKPOINT" \
  '[ "$(grep -n "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmf" | cut -d: -f1)" \
     = "$(( $(grep -n "^BUILD_CHECKPOINT=" <<<"$out_rmf" | cut -d: -f1) + 1 ))" ]'
assert "REVIEW_MIN_FIX_SEVERITY present in plain format too" \
  'grep -q "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmf_plain"'

# --- (RMF-c) repo-committed block is honored ----------------------------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-c"
cat > "$tmp/rmf-c/.docket.yml" <<'EOF'
metadata_branch: main
review:
  min_fix_severity: important
EOF
git -C "$tmp/rmf-c" add .docket.yml; git -C "$tmp/rmf-c" commit --quiet -m cfg
git -C "$tmp/rmf-c" push --quiet origin main
out2="$(run "$tmp/rmf-c" --export)"; eval "$out2"
assert "REVIEW_MIN_FIX_SEVERITY reads the block" \
  'grep <<<"$out2" -qxF "REVIEW_MIN_FIX_SEVERITY=important"'

# --- (RMF-d) global-able (ADR-0019 — NOT coordination-fenced) -----------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-d"
mkdir -p "$tmp/rmf-d.xdg/docket"
cat > "$tmp/rmf-d.xdg/docket/config.yml" <<'EOF'
review:
  min_fix_severity: blocker
EOF
rmf_d_err="$(rung "$tmp/rmf-d.xdg" "$tmp/rmf-d" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/rmf-d.xdg" "$tmp/rmf-d" --export 2>/dev/null)"; eval "$out"
assert "review.min_fix_severity is global-able (not fenced)" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "blocker" ]'
# The negative half names BOTH spellings the fence could warn under — the block header `review`
# and the leaf `min_fix_severity`. Matching only the block name would be decorative: the fence
# reads keys with config_scalar_get, and a block header resolves to an EMPTY value, so a `review`
# entry in the fence list prints nothing at all. The leaf is the spelling that can actually
# produce a warning (config_line_scalar_get strips indentation), and mutation-testing this assert
# means adding `min_fix_severity` to the fence list. Its non-vacuity companion is the positive
# assert directly above: it proves the global layer was read in the first place.
assert "no fence warning for review.min_fix_severity" \
  '! grep -qiE "(review|min_fix_severity).*per-repo-only" <<<"$rmf_d_err"'

# --- (RMF-e) repo-local layer wins over repo-committed ------------------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-e"
cat > "$tmp/rmf-e/.docket.yml" <<'EOF'
metadata_branch: main
review:
  min_fix_severity: blocker
EOF
git -C "$tmp/rmf-e" add .docket.yml; git -C "$tmp/rmf-e" commit --quiet -m cfg
git -C "$tmp/rmf-e" push --quiet origin main
printf 'review:\n  min_fix_severity: minor\n' > "$tmp/rmf-e/.docket.local.yml"
out="$(run "$tmp/rmf-e" --export)"; eval "$out"
assert "local layer beats repo-committed for review.min_fix_severity" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "minor" ]'

# --- (RMF-f) SHADOW GUARD — a bare min_fix_severity: OUTSIDE the review: block -
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-f"
cat > "$tmp/rmf-f/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  min_fix_severity: blocker
EOF
git -C "$tmp/rmf-f" add .docket.yml; git -C "$tmp/rmf-f" commit --quiet -m cfg
git -C "$tmp/rmf-f" push --quiet origin main
out="$(run "$tmp/rmf-f" --export)"; eval "$out"
assert "a foreign block's min_fix_severity: does not shadow review.min_fix_severity" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "minor" ]'

# --- (RMF-g) THE skills.review COLLISION GUARD --------------------------------
# `skills:` already carries a `review:` LEAF, and this knob introduces a top-level `review:`
# BLOCK. The two must not see each other, in either direction. That is the invariant this knob's
# correctness rests on, so assert it rather than assume it.
#
# GUARDS ARE CODE — read this before simplifying the fixtures. The OBVIOUS fixture for the shadow
# direction (a lone `skills:` block carrying `review: docket-review`) is DECORATIVE: it was
# written, mutation-tested, and found to redden nothing. config_block_header rejects that line for
# TWO independent reasons — it is indented, AND it has a value after the colon — so relaxing
# either one alone leaves the fixture green; relaxing BOTH also leaves it green, because the
# would-be block has no `min_fix_severity` leaf under it to find. Both were run, not reasoned:
# under each relaxation and under both together, that fixture stayed green.
#
# The fixture that makes the column-0 requirement genuinely load-bearing is (RMF-g2) below: an
# indented, VALUELESS `review:` with a `min_fix_severity` leaf nested under it. Verified by
# mutation: deleting the `"$line" != [[:space:]]*` conjunct from config_block_header reddens
# RMF-g2 and nothing else in this file, and so does deleting both conjuncts.

# (RMF-g1) COEXISTENCE — both spellings in one file, each resolving to its own value. `skills:`
# uses a NON-DEFAULT review skill on purpose: asserting the shipped default would pass just as
# well against a resolver that never read the block at all.
unset REVIEW_MIN_FIX_SEVERITY
SKILL_REVIEW=__poison__
mkrepo "$tmp/rmf-g"
cat > "$tmp/rmf-g/.docket.yml" <<'EOF'
metadata_branch: main
skills:
  review: my-org:custom-review
review:
  min_fix_severity: important
EOF
git -C "$tmp/rmf-g" add .docket.yml; git -C "$tmp/rmf-g" commit --quiet -m cfg
git -C "$tmp/rmf-g" push --quiet origin main
out="$(run "$tmp/rmf-g" --export)"; eval "$out"
assert "the review: block resolves alongside a skills.review leaf" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "important" ]'
assert "skills.review still resolves normally alongside the review: block" \
  '[ "$SKILL_REVIEW" = "my-org:custom-review" ]'

# (RMF-g2) THE COLUMN-0 INVARIANT — the assert the header matcher's indentation check is what
# keeps green. An INDENTED, valueless `review:` carrying a min_fix_severity leaf is the shape that
# reads as this block's header the moment config_block_header stops requiring column 0; without
# that check the nested `blocker` leaks out as review.min_fix_severity.
unset REVIEW_MIN_FIX_SEVERITY
SKILL_REVIEW=__poison__
mkrepo "$tmp/rmf-g2"
cat > "$tmp/rmf-g2/.docket.yml" <<'EOF'
metadata_branch: main
skills:
  review:
    min_fix_severity: blocker
EOF
git -C "$tmp/rmf-g2" add .docket.yml; git -C "$tmp/rmf-g2" commit --quiet -m cfg
git -C "$tmp/rmf-g2" push --quiet origin main
out="$(run "$tmp/rmf-g2" --export)"; eval "$out"
assert "an INDENTED review: is not read as the review: block header" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "minor" ]'
# The reverse direction under this shape, which RMF-g1 asserts only under the valued spelling: the
# nested leaf must not become skills.review's VALUE either. Without it the SKILL_REVIEW poison above
# is a dead assignment — an assert that reads as dropped. A valueless `skills: review:` resolves to
# the shipped default; here that is not the weak assertion RMF-g1's comment warns about, because the
# poison makes "the resolver never ran" distinguishable from "the resolver read nothing under it".
assert "a nested min_fix_severity does not leak out as skills.review's value" \
  '[ "$SKILL_REVIEW" = "docket-review" ]'

# --- (RMF-h) fail closed on garbage -------------------------------------------
assert "non-enum min_fix_severity aborts nonzero" \
  '! run_resolver_with "review:\n  min_fix_severity: critical\n" >/dev/null 2>&1'
rmf_h_err="$(run_resolver_with "review:\n  min_fix_severity: critical\n" 2>&1 >/dev/null)"
assert "unparseable review.min_fix_severity: mentions review.min_fix_severity" \
  'grep -qF "review.min_fix_severity" <<<"$rmf_h_err"'

# --- (RMF-i) the contract doc documents it ------------------------------------
assert "docket-config.md has a review.min_fix_severity table row" \
  'grep -qE "^\| \`review\.min_fix_severity\` \| \`minor\` \| yes \|" "$REPO/scripts/docket-config.md"'
assert "docket-config.md lists the export name" \
  'grep -q "^REVIEW_MIN_FIX_SEVERITY$" "$REPO/scripts/docket-config.md"'

# ============================================================================
# Change 0218 — review.max_fix_tasks (REVIEW_MAX_FIX_TASKS)
# The second leaf of the review: block, resolved through the SAME review_key helper as
# min_fix_severity (so the RMF-g2 column-0 invariant above covers this leaf too — it pins the
# helper's block scoping, not one leaf's). It is a COUNT, so it validates like reclaim.lease_ttl /
# learnings.cap: non-negative integer or abort. Same NOTE as the RMF block (guards-are-code (e)):
# clear the asserted var BEFORE each eval — an aborting run emits NOTHING, and eval "" would
# silently leave the previous case's value in place.
#
# ORDERING — the RMF block's, and for the same reason: the export-SHAPE asserts (presence, position,
# plain-format) run FIRST, before any fixture that dereferences $REVIEW_MAX_FIX_TASKS. Under
# `set -u` a missing export does not redden an assert — `eval ""` leaves the variable unbound and
# assert()'s own eval kills the suite — so a shape assert placed after a deref-ing fixture can only
# ever be reached when it was going to pass anyway. Mutation-verified: deleting the emit line
# reddens `REVIEW_MAX_FIX_TASKS is emitted` by name in this order, and aborted the run before
# reaching it in the shape-asserts-last order this file used to carry.
# ============================================================================

# --- (RMX-a) default when no layer sets the leaf ------------------------------
# Asserted on the EMITTED LINE, deliberately without an eval: this is the first assert about a
# brand-new export, so it is the one that must survive the export not existing at all.
mkrepo "$tmp/rmx-a"
out="$(run "$tmp/rmx-a" --export)"
assert "REVIEW_MAX_FIX_TASKS defaults to 10" \
  'grep -qxF "REVIEW_MAX_FIX_TASKS=10" <<<"$out"'

# --- (RMX-b) export presence, POSITION, and both formats ----------------------
out_rmx="$(run "$tmp/rmx-a" --export)"
out_rmx_plain="$(run "$tmp/rmx-a" --export --format plain)"
assert "REVIEW_MAX_FIX_TASKS is emitted" \
  'grep -q "^REVIEW_MAX_FIX_TASKS=" <<<"$out_rmx"'
assert "REVIEW_MAX_FIX_TASKS is emitted directly after REVIEW_MIN_FIX_SEVERITY" \
  '[ "$(grep -n "^REVIEW_MAX_FIX_TASKS=" <<<"$out_rmx" | cut -d: -f1)" \
     = "$(( $(grep -n "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmx" | cut -d: -f1) + 1 ))" ]'
assert "REVIEW_MAX_FIX_TASKS present in plain format too" \
  'grep -q "^REVIEW_MAX_FIX_TASKS=" <<<"$out_rmx_plain"'

# --- (RMX-c) repo-committed block is honored ----------------------------------
unset REVIEW_MAX_FIX_TASKS
mkrepo "$tmp/rmx-c"
cat > "$tmp/rmx-c/.docket.yml" <<'EOF'
metadata_branch: main
review:
  max_fix_tasks: 3
EOF
git -C "$tmp/rmx-c" add .docket.yml; git -C "$tmp/rmx-c" commit --quiet -m cfg
git -C "$tmp/rmx-c" push --quiet origin main
out2="$(run "$tmp/rmx-c" --export)"; eval "$out2"
assert "REVIEW_MAX_FIX_TASKS reads the block" \
  'grep -qxF "REVIEW_MAX_FIX_TASKS=3" <<<"$out2" && [ "$REVIEW_MAX_FIX_TASKS" = "3" ]'

# --- (RMX-d) global-able (ADR-0019 — NOT coordination-fenced) -----------------
unset REVIEW_MAX_FIX_TASKS
mkrepo "$tmp/rmx-d"
mkdir -p "$tmp/rmx-d.xdg/docket"
cat > "$tmp/rmx-d.xdg/docket/config.yml" <<'EOF'
review:
  max_fix_tasks: 25
EOF
rmx_d_err="$(rung "$tmp/rmx-d.xdg" "$tmp/rmx-d" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/rmx-d.xdg" "$tmp/rmx-d" --export 2>/dev/null)"; eval "$out"
assert "review.max_fix_tasks is global-able (not fenced)" \
  '[ "$REVIEW_MAX_FIX_TASKS" = "25" ]'
# Same reasoning as RMF-d's negative half: the fence reads keys with config_scalar_get, so a block
# HEADER entry resolves empty and prints nothing — `max_fix_tasks`, the leaf, is the only spelling
# that can actually produce a warning, and mutation-testing this assert means adding it to the fence
# list (done; it reddens). Its non-vacuity companion is the positive assert directly above.
assert "no fence warning for review.max_fix_tasks" \
  '! grep -qiE "(review|max_fix_tasks).*per-repo-only" <<<"$rmx_d_err"'

# --- (RMX-e) repo-local layer wins over repo-committed ------------------------
unset REVIEW_MAX_FIX_TASKS
mkrepo "$tmp/rmx-e"
cat > "$tmp/rmx-e/.docket.yml" <<'EOF'
metadata_branch: main
review:
  max_fix_tasks: 4
EOF
git -C "$tmp/rmx-e" add .docket.yml; git -C "$tmp/rmx-e" commit --quiet -m cfg
git -C "$tmp/rmx-e" push --quiet origin main
printf 'review:\n  max_fix_tasks: 7\n' > "$tmp/rmx-e/.docket.local.yml"
out="$(run "$tmp/rmx-e" --export)"; eval "$out"
assert "local layer beats repo-committed for review.max_fix_tasks" \
  '[ "$REVIEW_MAX_FIX_TASKS" = "7" ]'

# --- (RMX-f) SHADOW GUARD — a bare max_fix_tasks: OUTSIDE the review: block ---
unset REVIEW_MAX_FIX_TASKS
mkrepo "$tmp/rmx-f"
cat > "$tmp/rmx-f/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  max_fix_tasks: 99
EOF
git -C "$tmp/rmx-f" add .docket.yml; git -C "$tmp/rmx-f" commit --quiet -m cfg
git -C "$tmp/rmx-f" push --quiet origin main
out="$(run "$tmp/rmx-f" --export)"; eval "$out"
assert "a foreign block's max_fix_tasks: does not shadow review.max_fix_tasks" \
  '[ "$REVIEW_MAX_FIX_TASKS" = "10" ]'

# --- (RMX-g) BOTH leaves out of ONE review: block, beside skills.review -------
# RMF-g1/g2 above pin the block-vs-leaf separation using min_fix_severity alone. What is new here
# is the MULTI-LEAF read: the review: block now has two leaves, and a reader that stopped at the
# block's first leaf would serve min_fix_severity and silently default max_fix_tasks. Mutation-
# verified: closing the block after its first leaf reddens the first assert by name.
unset REVIEW_MIN_FIX_SEVERITY REVIEW_MAX_FIX_TASKS
SKILL_REVIEW=__poison__
mkrepo "$tmp/rmx-g"
cat > "$tmp/rmx-g/.docket.yml" <<'EOF'
metadata_branch: main
skills:
  review: my-org:custom-review
review:
  min_fix_severity: important
  max_fix_tasks: 2
EOF
git -C "$tmp/rmx-g" add .docket.yml; git -C "$tmp/rmx-g" commit --quiet -m cfg
git -C "$tmp/rmx-g" push --quiet origin main
out="$(run "$tmp/rmx-g" --export)"; eval "$out"
assert "both review: leaves resolve from one block" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "important" ] && [ "$REVIEW_MAX_FIX_TASKS" = "2" ]'
assert "skills.review still resolves alongside a two-leaf review: block" \
  '[ "$SKILL_REVIEW" = "my-org:custom-review" ]'

# --- (RMX-h) fail closed on garbage — and 0 IS legal --------------------------
# The validator is reclaim.lease_ttl's / learnings.cap's: `''|*[!0-9]*` aborts. That admits 0, and
# admitting it is deliberate — see the resolver comment. Assert the boundary explicitly so a later
# "tighten it to a positive integer" edit has to argue with a named guard rather than a silence.
#
# The 0 case reads the EMITTED LINE rather than eval-ing and dereferencing, for the reason the
# block header gives: it is a fixture a mutation can make ABORT, and on an abort a deref would kill
# the suite instead of reddening this assert. Mutation-verified: adding `0` to the reject list
# reddens both asserts below by name.
rmx_zero_out="$(run_resolver_with "review:\n  max_fix_tasks: 0\n" 2>/dev/null)"; rmx_zero_rc=$?
assert "review.max_fix_tasks: 0 does not abort the resolver" '[ "$rmx_zero_rc" -eq 0 ]'
assert "review.max_fix_tasks: 0 is legal (fix nothing but blockers)" \
  'grep -qxF "REVIEW_MAX_FIX_TASKS=0" <<<"$rmx_zero_out"'
assert "non-numeric max_fix_tasks aborts nonzero" \
  '! run_resolver_with "review:\n  max_fix_tasks: many\n" >/dev/null 2>&1'
assert "negative max_fix_tasks aborts nonzero" \
  '! run_resolver_with "review:\n  max_fix_tasks: -1\n" >/dev/null 2>&1'
assert "fractional max_fix_tasks aborts nonzero" \
  '! run_resolver_with "review:\n  max_fix_tasks: 2.5\n" >/dev/null 2>&1'
rmx_h_err="$(run_resolver_with "review:\n  max_fix_tasks: many\n" 2>&1 >/dev/null)"
assert "unparseable review.max_fix_tasks: mentions review.max_fix_tasks" \
  'grep -qF "review.max_fix_tasks" <<<"$rmx_h_err"'

# --- (RMX-i) the contract doc documents it ------------------------------------
assert "docket-config.md has a review.max_fix_tasks table row" \
  'grep -qE "^\| \`review\.max_fix_tasks\` \| \`10\` \| yes \|" "$REPO/scripts/docket-config.md"'
assert "docket-config.md lists the export name" \
  'grep -q "^REVIEW_MAX_FIX_TASKS$" "$REPO/scripts/docket-config.md"'

# --- Change 0091 — auto_capture (global-able boolean, default false) ---------------------------
# Mirrors auto_groom's four-layer resolution, but fails CLOSED on a non-boolean (the reclaim.auto /
# learnings.enabled / terminal_publish precedent): silently defaulting a typo to `false` would make
# an opted-in repo quietly stop capturing, which is invisible rather than loud.

# (AC-a) default — change 0127 rewrote this family onto the nested map. The leaf keeps auto_groom's
# four-layer resolution and still fails CLOSED on a non-boolean.
mkrepo "$tmp/ac-a"
out_ac="$(run "$tmp/ac-a" --export --format plain 2>/dev/null)"
assert "AUTO_CAPTURE_ENABLED defaults to false" 'grep <<<"$out_ac" -qxF "AUTO_CAPTURE_ENABLED=false"'

# (AC-b) repo-committed .docket.yml wins over the built-in
mkrepo "$tmp/ac-b"
printf 'auto_capture:\n  enabled: true\n' > "$tmp/ac-b/.docket.yml"
git -C "$tmp/ac-b" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-b" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-b" push -q origin HEAD:main >/dev/null 2>&1
out_ac_b="$(run "$tmp/ac-b" --export --format plain 2>/dev/null)"
assert "AUTO_CAPTURE_ENABLED reads repo .docket.yml" 'grep <<<"$out_ac_b" -qxF "AUTO_CAPTURE_ENABLED=true"'

# (AC-c) global layer is honored (NOT fenced) and emits no per-repo-only warning
mkrepo "$tmp/ac-c"
mkdir -p "$tmp/ac-c.xdg/docket"
printf 'auto_capture:\n  enabled: true\n' > "$tmp/ac-c.xdg/docket/config.yml"
ac_c_out="$(rung "$tmp/ac-c.xdg" "$tmp/ac-c" --export --format plain 2>/dev/null)"
ac_c_err="$(rung "$tmp/ac-c.xdg" "$tmp/ac-c" --export --format plain 2>&1 >/dev/null)"
assert "auto_capture is global-able (not fenced)" 'grep <<<"$ac_c_out" -qxF "AUTO_CAPTURE_ENABLED=true"'
assert "no fence warning for auto_capture" '! grep <<<"$ac_c_err" -qi "auto_capture.*per-repo-only"'

# (AC-d) repo-local .docket.local.yml outranks repo-committed AND global
mkrepo "$tmp/ac-d"
mkdir -p "$tmp/ac-d.xdg/docket"
printf 'auto_capture:\n  enabled: false\n' > "$tmp/ac-d.xdg/docket/config.yml"
printf 'auto_capture:\n  enabled: false\n' > "$tmp/ac-d/.docket.yml"
git -C "$tmp/ac-d" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-d" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-d" push -q origin HEAD:main >/dev/null 2>&1
printf 'auto_capture:\n  enabled: true\n' > "$tmp/ac-d/.docket.local.yml"
ac_d_out="$(rung "$tmp/ac-d.xdg" "$tmp/ac-d" --export --format plain 2>/dev/null)"
assert "repo-local auto_capture.enabled outranks repo-committed and global" \
  'grep <<<"$ac_d_out" -qxF "AUTO_CAPTURE_ENABLED=true"'

# (AC-e) fail closed on garbage, and the diagnostic names the leaf
mkrepo "$tmp/ac-e"
printf 'auto_capture:\n  enabled: maybe\n' > "$tmp/ac-e/.docket.yml"
git -C "$tmp/ac-e" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-e" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-e" push -q origin HEAD:main >/dev/null 2>&1
assert "non-bool auto_capture.enabled aborts nonzero" '! run "$tmp/ac-e" --export >/dev/null 2>&1'
ac_e_err="$(run "$tmp/ac-e" --export 2>&1 >/dev/null)"
assert "unparseable auto_capture.enabled names the leaf" \
  'grep <<<"$ac_e_err" -qF "auto_capture.enabled"'

# (AC-f) emit ORDER is pinned: the three change-0127 exports follow AUTO_GROOM in a fixed sequence
# (scripts/docket-config.md lists them in this order; a reordering there is a silent contract
# break). Identity, not just adjacency: each variable's OWN line number is looked up by name, so a
# swap within the group is caught rather than passing vacuously.
ac_f_out="$(run "$tmp/ac-a" --export --format plain 2>/dev/null)"
ac_g_line="$(grep <<<"$ac_f_out" -n '^AUTO_GROOM=' | cut -d: -f1)"
ac_ct_line="$(grep <<<"$ac_f_out" -n '^CHANGE_TYPES=' | cut -d: -f1)"
ac_ace_line="$(grep <<<"$ac_f_out" -n '^AUTO_CAPTURE_ENABLED=' | cut -d: -f1)"
ac_act_line="$(grep <<<"$ac_f_out" -n '^AUTO_CAPTURE_TYPES=' | cut -d: -f1)"
assert "CHANGE_TYPES is emitted directly after AUTO_GROOM" \
  '[ -n "$ac_g_line" ] && [ -n "$ac_ct_line" ] && [ "$ac_ct_line" -eq "$(( ac_g_line + 1 ))" ]'
assert "AUTO_CAPTURE_ENABLED is emitted directly after CHANGE_TYPES" \
  '[ -n "$ac_ace_line" ] && [ "$ac_ace_line" -eq "$(( ac_ct_line + 1 ))" ]'
assert "AUTO_CAPTURE_TYPES is emitted directly after AUTO_CAPTURE_ENABLED" \
  '[ -n "$ac_act_line" ] && [ "$ac_act_line" -eq "$(( ac_ace_line + 1 ))" ]'

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
FINALIZE_GATE=__poison__; FINALIZE_TEST_COMMAND=__poison__
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
FINALIZE_TEST_COMMAND=__poison__
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
FINALIZE_TEST_COMMAND=__poison__
out="$(run "$tmp/s3" --export)"; eval "$out"
assert "test_command AUTO is NOT the sentinel (case-sensitive)" '[ "$FINALIZE_TEST_COMMAND" = "AUTO" ]'

# --- (S4-S9) changes 0106 + 0112: the sentinel's CROSS-LAYER masking --------
# The collapse (`[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""`) runs AFTER the
# three-rung `lcl` → committed → `gbl` resolution chain for finalize.test_command. That
# placement is the whole point: a HIGHER layer writing `test_command: auto` MASKS a LOWER
# layer's real command, which is the correct reading of an explicit re-statement of the
# default. Collapse per-layer instead and the behavior silently INVERTS — the higher `auto`
# becomes empty, the `:-` chain falls through, and the lower command resurfaces.
# Sections s/s2/s3 above are all single-layer, so none of them can see this. These do.
#
# 0106 pinned three of the ordered rung pairs; 0112 completed the matrix with s7/s8/s9.
# Each fixture below carries a machine-readable marker line naming its pair as
# (rung holding `auto` -> rung holding the real command). Change 0258's leg-2 guard, near the
# end of this file, computes the expected ordered-pair set from `config_scalar_get`'s layer
# dispatch and asserts set equality against those markers -- so the markers, not this comment,
# are the claim of record, and a FOURTH config layer grows the expected set from 6 pairs to 12
# and reddens the guard until six new fixtures exist.
# The forward cases (a higher `auto` masks a lower real command) are s4, s5 and s9; the reverse
# cases (a lower `auto` must NOT wipe a higher real command) are s6, s7 and s8.
# s7 is the one earned on unique discriminating power: a committed-rung-specific clear appended
# after the collapse leaves all five of 0106's asserts green and reddens s7 alone. s8 and s9 are
# matrix-completeness witnesses that share s6's and s4/s5's mutations respectively -- no claim of
# a unique witness is made for them.
#
# s4 and s5 assert an EMPTY value, which is also what an ABSENT key yields — so each first
# asserts its lower rung really does resolve (the control), then adds the masking layer.
# Each eval is preceded by a poison value: an aborted run emits nothing, and a bare
# `eval ""` would otherwise leave the previous fixture's value standing.

# (s4) FORWARD, lcl() path: .docket.local.yml `auto` over a committed real command.
# RUNG_PAIR: local->committed
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
# RUNG_PAIR: committed->global
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

# (s6) REVERSE: a LOWER layer's `auto` must NOT wipe a HIGHER layer's real command.
# Required by the two-sided-proof rule: the forward cases above prove the collapse is not too
# LOOSE; this proves it is not too TIGHT. A blanket "any layer says auto => unset" scan would
# pass every forward case and fail only here. s6 deliberately does NOT redden under the
# per-layer mutation that reddens s4/s5 — which is exactly why it needs its own mutation.
# RUNG_PAIR: global->committed
mkrepo "$tmp/s6"
mkdir -p "$tmp/s6.xdg/docket"
printf 'finalize:\n  test_command: auto\n' > "$tmp/s6.xdg/docket/config.yml"
cat > "$tmp/s6/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: make test
EOF
git -C "$tmp/s6" add .docket.yml; git -C "$tmp/s6" commit --quiet -m cfg
git -C "$tmp/s6" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s6.xdg" "$tmp/s6" --export)"; eval "$out"
assert "0106 s6: global auto does NOT wipe committed real command" '[ "$FINALIZE_TEST_COMMAND" = "make test" ]'

# (s7) REVERSE, lcl() path: a committed `auto` must NOT wipe a LOCAL real command.
# The dangerous cell, and the reason this change exists. A real repo whose .docket.local.yml
# sets a command over a committed `test_command: auto` must keep its local command; the
# committed-rung-specific clear that would silently drop it passes every 0106 assert.
# RUNG_PAIR: committed->local
mkrepo "$tmp/s7"
mkdir -p "$tmp/s7.xdg/docket"
cat > "$tmp/s7/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: auto
EOF
git -C "$tmp/s7" add .docket.yml; git -C "$tmp/s7" commit --quiet -m cfg
git -C "$tmp/s7" push --quiet origin main
printf 'finalize:\n  test_command: make local-test\n' > "$tmp/s7/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s7.xdg" "$tmp/s7" --export)"; eval "$out"
assert "0112 s7: committed auto does NOT wipe local real command" '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'

# (s8) REVERSE, skip-rung: a global `auto` must NOT wipe a LOCAL real command.
# Committed rung leaves the KEY absent -- .docket.yml still exists and still pins main-mode,
# matching s5's first phase. (Dropping the file entirely also resolves correctly; it would just
# route this fixture through BOOTSTRAP=CREATE_ORPHAN, which is why the file is kept.)
# RUNG_PAIR: global->local
mkrepo "$tmp/s8"
mkdir -p "$tmp/s8.xdg/docket"
printf 'finalize:\n  test_command: auto\n' > "$tmp/s8.xdg/docket/config.yml"
cat > "$tmp/s8/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/s8" add .docket.yml; git -C "$tmp/s8" commit --quiet -m cfg
git -C "$tmp/s8" push --quiet origin main
printf 'finalize:\n  test_command: make local-test\n' > "$tmp/s8/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s8.xdg" "$tmp/s8" --export)"; eval "$out"
assert "0112 s8: global auto does NOT wipe local real command (committed key absent)" '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'

# (s9) FORWARD, skip-rung: a local `auto` masks a GLOBAL real command, committed key absent.
# Expects an EMPTY value, which is also what an absent key yields -- so it carries a control
# assert first, the same reason s4 and s5 do.
# RUNG_PAIR: local->global
mkrepo "$tmp/s9"
mkdir -p "$tmp/s9.xdg/docket"
printf 'finalize:\n  test_command: make global\n' > "$tmp/s9.xdg/docket/config.yml"
cat > "$tmp/s9/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/s9" add .docket.yml; git -C "$tmp/s9" commit --quiet -m cfg
git -C "$tmp/s9" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s9.xdg" "$tmp/s9" --export)"; eval "$out"
assert "0112 s9 control: global real command resolves before masking" '[ "$FINALIZE_TEST_COMMAND" = "make global" ]'
printf 'finalize:\n  test_command: auto\n' > "$tmp/s9/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s9.xdg" "$tmp/s9" --export)"; eval "$out"
assert "0112 s9: local auto masks global real command (committed key absent)" '[ -z "$FINALIZE_TEST_COMMAND" ]'

# ============================================================================
# Change 0102 — finalize.require_pr_approval layer resolution
# ============================================================================
# The key was documented (README, .docket.example.yml, the finalize SKILL) but resolved NOWHERE:
# a value in .docket.local.yml or the global config was neither honored nor warned-and-ignored.
# It is deliberately NOT coordination-fenced — global-able is the point (see (R4) below).

# --- (R1) built-in default when unset in every layer -------------------------
mkrepo "$tmp/r1"
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
out="$(rung "$tmp/r1.xdg" "$tmp/r1" --export)"; eval "$out"
assert "0102 R1: unset everywhere -> built-in false" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = false ]'

# --- (R2) each layer honored, and the precedence between them ----------------
# Global only.
mkrepo "$tmp/r2"
mkdir -p "$tmp/r2.xdg/docket"
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r2.xdg/docket/config.yml"
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
out="$(rung "$tmp/r2.xdg" "$tmp/r2" --export)"; eval "$out"
assert "0102 R2: global finalize.require_pr_approval honored" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = true ]'

# Repo-committed beats global.
mkrepo "$tmp/r3"
printf 'metadata_branch: main\nfinalize:\n  require_pr_approval: false\n' > "$tmp/r3/.docket.yml"
git -C "$tmp/r3" add .docket.yml; git -C "$tmp/r3" commit --quiet -m cfg
git -C "$tmp/r3" push --quiet origin main
mkdir -p "$tmp/r3.xdg/docket"
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r3.xdg/docket/config.yml"
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
out="$(rung "$tmp/r3.xdg" "$tmp/r3" --export)"; eval "$out"
assert "0102 R3: repo-committed false beats global true" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = false ]'

# Repo-local beats repo-committed (and global).
mkrepo "$tmp/r4"
printf 'metadata_branch: main\nfinalize:\n  require_pr_approval: false\n' > "$tmp/r4/.docket.yml"
git -C "$tmp/r4" add .docket.yml; git -C "$tmp/r4" commit --quiet -m cfg
git -C "$tmp/r4" push --quiet origin main
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r4/.docket.local.yml"
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
out="$(rung "$tmp/r4.xdg" "$tmp/r4" --export)"; eval "$out"
assert "0102 R4: repo-local true beats repo-committed false" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = true ]'

# --- (R5) NOT coordination-fenced: machine layers are HONORED and UNWARNED ---
# The direct inverse of the fenced-key assertions at (0051 L3). This is the assert that would
# have caught the original bug, and the one that reddens if someone "helpfully" adds the key to
# the coordination-key fence loop, `for _fkey in metadata_branch integration_branch …`.
errout="$(XDG_CONFIG_HOME="$tmp/r4.xdg" bash "$SCRIPT" --repo-dir "$tmp/r4" --export 2>&1 >/dev/null)"
assert "0102 R5: no per-repo-only warning for require_pr_approval" \
  '! grep -q "require_pr_approval" <<<"$errout"'
assert "0102 R5: the key is absent from the coordination-key fence loop" \
  '! grep -q "require_pr_approval" <<<"$(sed -n "/^for _fkey in /p" "$SCRIPT")"'

# --- (R6) fail closed on a non-boolean --------------------------------------
mkrepo "$tmp/r6"
printf 'metadata_branch: main\nfinalize:\n  require_pr_approval: yes\n' > "$tmp/r6/.docket.yml"
git -C "$tmp/r6" add .docket.yml; git -C "$tmp/r6" commit --quiet -m cfg
git -C "$tmp/r6" push --quiet origin main
rc6="$(rung_rc "$tmp/r6.xdg" "$tmp/r6" --export)"
err6="$(XDG_CONFIG_HOME="$tmp/r6.xdg" bash "$SCRIPT" --repo-dir "$tmp/r6" --export 2>&1 >/dev/null)"
assert "0102 R6: non-boolean aborts (non-zero exit)"        '[ "$rc6" != "0" ]'
assert "0102 R6: diagnostic names the key"                  'grep -q "require_pr_approval" <<<"$err6"'
assert "0102 R6: diagnostic shows the offending value"      'grep -q "yes" <<<"$err6"'
assert "0102 R6: no KEY=value block on the abort path" \
  '[ -z "$(XDG_CONFIG_HOME="$tmp/r6.xdg" bash "$SCRIPT" --repo-dir "$tmp/r6" --export 2>/dev/null)" ]'

# --- (R6b) fail closed on a non-boolean from the GLOBAL layer ---------------
# R6 (above) proves the repo-committed layer aborts. docket-config.md's rewritten invariant
# bullets claim the SAME abort fires from the global layer and from .docket.local.yml too — true
# (every rung feeds FINALIZE_REQUIRE_PR_APPROVAL into the one `case` at
# scripts/docket-config.sh's `finalize.require_pr_approval must be 'true' or 'false'` validation,
# before Stage 3 ever runs) but, before this pair, unproven.
mkrepo "$tmp/r6b"
mkdir -p "$tmp/r6b.xdg/docket"
printf 'finalize:\n  require_pr_approval: yes\n' > "$tmp/r6b.xdg/docket/config.yml"
rc6b="$(rung_rc "$tmp/r6b.xdg" "$tmp/r6b" --export)"
err6b="$(XDG_CONFIG_HOME="$tmp/r6b.xdg" bash "$SCRIPT" --repo-dir "$tmp/r6b" --export 2>&1 >/dev/null)"
assert "0102 R6b: non-boolean in the GLOBAL layer aborts (non-zero exit)" '[ "$rc6b" != "0" ]'
assert "0102 R6b: diagnostic names the key"             'grep -q "require_pr_approval" <<<"$err6b"'
assert "0102 R6b: diagnostic shows the offending value" 'grep -q "yes" <<<"$err6b"'

# --- (R6c) fail closed on a non-boolean from .docket.local.yml --------------
mkrepo "$tmp/r6c"
printf 'finalize:\n  require_pr_approval: yes\n' > "$tmp/r6c/.docket.local.yml"
rc6c="$(rung_rc "$tmp/r6c.xdg" "$tmp/r6c" --export)"
err6c="$(XDG_CONFIG_HOME="$tmp/r6c.xdg" bash "$SCRIPT" --repo-dir "$tmp/r6c" --export 2>&1 >/dev/null)"
assert "0102 R6c: non-boolean in .docket.local.yml aborts (non-zero exit)" '[ "$rc6c" != "0" ]'
assert "0102 R6c: diagnostic names the key"             'grep -q "require_pr_approval" <<<"$err6c"'
assert "0102 R6c: diagnostic shows the offending value" 'grep -q "yes" <<<"$err6c"'

# --- (R7) export presence and POSITION --------------------------------------
# Position matters: scripts/docket-config.md documents the order as a contract, and pipe
# consumers may rely on it. Anchor on the neighbour rather than a bare "is present".
out7="$(rung "$tmp/r1.xdg" "$tmp/r1" --export)"
out7_plain="$(rung "$tmp/r1.xdg" "$tmp/r1" --export --format plain)"
assert "0102 R7: FINALIZE_REQUIRE_PR_APPROVAL is emitted" \
  'grep -q "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<"$out7"'
assert "0102 R7: emitted directly after FINALIZE_TEST_COMMAND" \
  '[ "$(grep -n "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<"$out7" | cut -d: -f1)" \
     = "$(( $(grep -n "^FINALIZE_TEST_COMMAND=" <<<"$out7" | cut -d: -f1) + 1 ))" ]'
out7_plain="$(rung "$tmp/r1.xdg" "$tmp/r1" --export --format plain)"
assert "0102 R7: present in plain format too" \
  'grep -q "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<"$out7_plain"'

# --- (R8) the contract doc documents it -------------------------------------
assert "0102 R8: docket-config.md has a require_pr_approval table row" \
  'grep -qE "^\| \`require_pr_approval\` \(finalize\) \| \`false\` \| yes \|" "$REPO/scripts/docket-config.md"'
assert "0102 R8: docket-config.md lists the export name" \
  'grep -q "^FINALIZE_REQUIRE_PR_APPROVAL$" "$REPO/scripts/docket-config.md"'

# --- (R9) machine-local vs global, with repo-committed never setting the key -
# R2 and R4 already pit global-alone and repo-local-vs-repo-committed; this is the missing
# fixture that pits repo-local directly against global with .docket.yml never touching the key.
# repo-local's winning value (false) here is also the built-in default, so the assert below on
# its own would pass even if the global fixture below were silently never written/read. Guard
# against that vacuity the way (R3) does: first prove the global layer is genuinely being read
# in THIS fixture (global alone -> true, a non-default value) before adding repo-local and
# checking precedence.
mkrepo "$tmp/r9"
mkdir -p "$tmp/r9.xdg/docket"
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r9.xdg/docket/config.yml"
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
out="$(rung "$tmp/r9.xdg" "$tmp/r9" --export)"; eval "$out"
assert "0102 R9: global alone is honored here (guards against vacuity below)" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = true ]'
printf 'finalize:\n  require_pr_approval: false\n' > "$tmp/r9/.docket.local.yml"
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
out="$(rung "$tmp/r9.xdg" "$tmp/r9" --export)"; eval "$out"
assert "0102 R9: repo-local false beats global true (repo-committed unset)" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = false ]'

# ============================================================================
# Change 0190 — finalize.skip_results_only_delta (the arming key)
# ============================================================================
# The ARMING switch for the second limb of finalize's post-rebase suite-skip predicate (the
# docs-only ancestor delta). Default `false`, so a repo that never sets it keeps change 0170's
# equality-only predicate byte-for-byte.
#
# UNLIKE its two finalize siblings this key IS coordination-fenced, and (S5) below is the
# deliberate INVERSE of (R5) above — the two asserts contradict each other on purpose, so a later
# "make the finalize keys consistent" edit cannot quietly move either one without reddening the
# other. `gate` and `require_pr_approval` express a POLICY a maintainer may legitimately hold
# differently per machine; this key asserts a FACT about the repo's own test suite (that no
# executable suite component reads <results_dir> as a content source), and a machine-scoped layer
# would assert that fact for every repo the machine touches.

# --- (S1) built-in default when unset in every layer -------------------------
mkrepo "$tmp/s1d"
FINALIZE_SKIP_RESULTS_ONLY_DELTA=__poison__
out="$(rung "$tmp/s1d.xdg" "$tmp/s1d" --export)"; eval "$out"
assert "0190 S1: unset everywhere -> built-in false" \
  '[ "$FINALIZE_SKIP_RESULTS_ONLY_DELTA" = false ]'

# --- (S2) the repo-committed override is honored -----------------------------
# Probed with `true`, the NON-default, for the 0084 terminal_publish reason: probing with `false`
# would pass unchanged even if the committed read were dropped entirely.
mkrepo "$tmp/s2d"
printf 'metadata_branch: main\nfinalize:\n  skip_results_only_delta: true\n' > "$tmp/s2d/.docket.yml"
git -C "$tmp/s2d" add .docket.yml; git -C "$tmp/s2d" commit --quiet -m cfg
git -C "$tmp/s2d" push --quiet origin main
FINALIZE_SKIP_RESULTS_ONLY_DELTA=__poison__
out="$(rung "$tmp/s2d.xdg" "$tmp/s2d" --export)"; eval "$out"
assert "0190 S2: repo-committed true is honored" \
  '[ "$FINALIZE_SKIP_RESULTS_ONLY_DELTA" = true ]'

# --- (S3) FENCED in the global layer: warned, ignored, never fatal -----------
mkrepo "$tmp/s3d"
mkdir -p "$tmp/s3d.xdg/docket"
printf 'finalize:\n  skip_results_only_delta: true\n' > "$tmp/s3d.xdg/docket/config.yml"
s3err="$(rung "$tmp/s3d.xdg" "$tmp/s3d" --export 2>&1 >/dev/null)"
# Unset before the eval, the terminal_publish precedent: an aborting run emits nothing, so
# eval "" would leave a value from an earlier block standing and the "stays false" assert would
# pass on stale state rather than on this run's output.
unset FINALIZE_SKIP_RESULTS_ONLY_DELTA
out="$(rung "$tmp/s3d.xdg" "$tmp/s3d" --export 2>/dev/null)"; eval "$out"
assert "0190 S3: global value warns"                     'grep -q "skip_results_only_delta" <<<"$s3err"'
assert "0190 S3: warning says per-repo-only"             'grep -qi "per-repo-only" <<<"$s3err"'
assert "0190 S3: global value NOT honored (stays false)" \
  '[ "${FINALIZE_SKIP_RESULTS_ONLY_DELTA-unset}" = false ]'
assert "0190 S3: a global value is not fatal" \
  '[ "$(rung_rc "$tmp/s3d.xdg" "$tmp/s3d" --export)" -eq 0 ]'

# --- (S4) FENCED in .docket.local.yml too ------------------------------------
mkrepo "$tmp/s4d"
printf 'finalize:\n  skip_results_only_delta: true\n' > "$tmp/s4d/.docket.local.yml"
s4err="$(rung "$tmp/s4d.xdg" "$tmp/s4d" --export 2>&1 >/dev/null)"
unset FINALIZE_SKIP_RESULTS_ONLY_DELTA
out="$(rung "$tmp/s4d.xdg" "$tmp/s4d" --export 2>/dev/null)"; eval "$out"
assert "0190 S4: .docket.local.yml value warns"       'grep -q "skip_results_only_delta" <<<"$s4err"'
assert "0190 S4: the warning names .docket.local.yml" 'grep -q ".docket.local.yml" <<<"$s4err"'
assert "0190 S4: repo-local value NOT honored (stays false)" \
  '[ "${FINALIZE_SKIP_RESULTS_ONLY_DELTA-unset}" = false ]'

# --- (S5) fence MEMBERSHIP, the structural inverse of (R5) -------------------
assert "0190 S5: the key IS a member of the coordination-key fence loop" \
  'grep -q "skip_results_only_delta" <<<"$(sed -n "/^for _fkey in /p" "$SCRIPT")"'

# --- (S6) fail closed on a non-boolean --------------------------------------
# Inverted from require_pr_approval's argument: there, defaulting a typo to `false` would DISARM a
# gate the user believes is armed; here, defaulting a typo to `true` would ARM a gate-weakening
# skip the repo never opted into. Both directions fail closed on the same `case`.
mkrepo "$tmp/s6d"
printf 'metadata_branch: main\nfinalize:\n  skip_results_only_delta: yes\n' > "$tmp/s6d/.docket.yml"
git -C "$tmp/s6d" add .docket.yml; git -C "$tmp/s6d" commit --quiet -m cfg
git -C "$tmp/s6d" push --quiet origin main
rcs6="$(rung_rc "$tmp/s6d.xdg" "$tmp/s6d" --export)"
errs6="$(XDG_CONFIG_HOME="$tmp/s6d.xdg" bash "$SCRIPT" --repo-dir "$tmp/s6d" --export 2>&1 >/dev/null)"
assert "0190 S6: non-boolean aborts (non-zero exit)"   '[ "$rcs6" != "0" ]'
assert "0190 S6: diagnostic names the key"             'grep -q "skip_results_only_delta" <<<"$errs6"'
assert "0190 S6: diagnostic shows the offending value" 'grep -q "yes" <<<"$errs6"'
assert "0190 S6: no KEY=value block on the abort path" \
  '[ -z "$(XDG_CONFIG_HOME="$tmp/s6d.xdg" bash "$SCRIPT" --repo-dir "$tmp/s6d" --export 2>/dev/null)" ]'

# --- (S7) export presence and POSITION --------------------------------------
outs7="$(rung "$tmp/s1d.xdg" "$tmp/s1d" --export)"
assert "0190 S7: FINALIZE_SKIP_RESULTS_ONLY_DELTA is emitted" \
  'grep -q "^FINALIZE_SKIP_RESULTS_ONLY_DELTA=" <<<"$outs7"'
assert "0190 S7: emitted directly after FINALIZE_REQUIRE_PR_APPROVAL" \
  '[ "$(grep -n "^FINALIZE_SKIP_RESULTS_ONLY_DELTA=" <<<"$outs7" | cut -d: -f1)" \
     = "$(( $(grep -n "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<"$outs7" | cut -d: -f1) + 1 ))" ]'
outs7_plain="$(rung "$tmp/s1d.xdg" "$tmp/s1d" --export --format plain)"
assert "0190 S7: present in plain format too" \
  'grep -q "^FINALIZE_SKIP_RESULTS_ONLY_DELTA=" <<<"$outs7_plain"'

# --- (S8) the contract doc documents it -------------------------------------
assert "0190 S8: docket-config.md has a skip_results_only_delta table row scoped no (fenced)" \
  'grep -qE "^\| \`skip_results_only_delta\` \(finalize\) \| \`false\` \| no \(fenced\) \|" "$REPO/scripts/docket-config.md"'
assert "0190 S8: docket-config.md lists the export name" \
  'grep -q "^FINALIZE_SKIP_RESULTS_ONLY_DELTA$" "$REPO/scripts/docket-config.md"'

# ============================================================================
# Change 0132 — machine-local Bash runtime
# ============================================================================

fake_bash "$tmp/runtime-bin/global-bash" 'GNU bash, version 5.2.0(1)-release'
fake_bash "$tmp/runtime-bin/local-bash" 'GNU bash, version 5.2.0(1)-release'
fake_bash "$tmp/runtime-bin/committed-bash" 'GNU bash, version 5.2.0(1)-release'

# Global runtime resolves when the repo-local layer is absent.
mkrepo "$tmp/runtime-global"
mkdir -p "$tmp/runtime-global.xdg/docket"
printf 'runtime:\n  bash: %s\n' "$tmp/runtime-bin/global-bash" \
  >"$tmp/runtime-global.xdg/docket/config.yml"
runtime_global_out="$(rung "$tmp/runtime-global.xdg" "$tmp/runtime-global" --export)"
assert "0132 runtime: global runtime is emitted exactly" \
  'grep -qxF "DOCKET_BASH_PATH=$tmp/runtime-bin/global-bash" <<<"$runtime_global_out"'
assert "0132 runtime: shell export follows METADATA_WORKTREE" \
  '[ "$(grep -n "^DOCKET_BASH_PATH=" <<<"$runtime_global_out" | cut -d: -f1)" \
     = "$(( $(grep -n "^METADATA_WORKTREE=" <<<"$runtime_global_out" | cut -d: -f1) + 1 ))" ]'
runtime_global_plain="$(rung "$tmp/runtime-global.xdg" "$tmp/runtime-global" --export --format plain)"
assert "0132 runtime: global runtime is present in plain format" \
  'grep -qxF "DOCKET_BASH_PATH=$tmp/runtime-bin/global-bash" <<<"$runtime_global_plain"'
assert "0132 runtime: plain export follows REPO_ROOT" \
  '[ "$(grep -n "^DOCKET_BASH_PATH=" <<<"$runtime_global_plain" | cut -d: -f1)" \
     = "$(( $(grep -n "^REPO_ROOT=" <<<"$runtime_global_plain" | cut -d: -f1) + 1 ))" ]'

# Repo-local runtime overrides a distinct valid global runtime.
printf 'runtime:\n  bash: %s\n' "$tmp/runtime-bin/local-bash" \
  >"$tmp/runtime-global/.docket.local.yml"
runtime_local_out="$(rung "$tmp/runtime-global.xdg" "$tmp/runtime-global" --export)"
assert "0132 runtime: repo-local runtime overrides global" \
  'grep -qxF "DOCKET_BASH_PATH=$tmp/runtime-bin/local-bash" <<<"$runtime_local_out"'
assert "0132 runtime: losing global runtime is not emitted" \
  '! grep -qF "$tmp/runtime-bin/global-bash" <<<"$runtime_local_out"'

# A committed runtime is diagnosed and ignored; the valid global runtime still wins.
mkrepo "$tmp/runtime-committed"
mkdir -p "$tmp/runtime-committed.xdg/docket"
printf 'runtime:\n  bash: %s\n' "$tmp/runtime-bin/global-bash" \
  >"$tmp/runtime-committed.xdg/docket/config.yml"
printf 'runtime:\n  bash: %s\n' "$tmp/runtime-bin/committed-bash" \
  >"$tmp/runtime-committed/.docket.yml"
git -C "$tmp/runtime-committed" add .docket.yml
git -C "$tmp/runtime-committed" commit --quiet -m cfg
git -C "$tmp/runtime-committed" push --quiet origin main
runtime_committed_err="$(XDG_CONFIG_HOME="$tmp/runtime-committed.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-committed" --export 2>&1 >/dev/null)"
runtime_committed_out="$(rung "$tmp/runtime-committed.xdg" "$tmp/runtime-committed" --export)"
assert "0132 runtime fence: committed runtime is warned-and-ignored" \
  'grep -q "committed.*runtime.bash.*ignored" <<<"$runtime_committed_err"'
assert "0132 runtime fence: global runtime wins over committed runtime" \
  'grep -qxF "DOCKET_BASH_PATH=$tmp/runtime-bin/global-bash" <<<"$runtime_committed_out"'
assert "0132 runtime fence: committed runtime is not emitted" \
  '! grep -qF "$tmp/runtime-bin/committed-bash" <<<"$runtime_committed_out"'

# Presence drives the committed-layer fence, independently of scalar parsing. Empty and duplicate
# committed declarations are both warned-and-ignored (never validated); a valid machine-local
# fallback still resolves.
for committed_case in empty duplicate; do
  mkrepo "$tmp/runtime-committed-$committed_case"
  mkdir -p "$tmp/runtime-committed-$committed_case.xdg/docket"
  printf 'runtime:\n  bash: %s\n' "$tmp/runtime-bin/global-bash" \
    >"$tmp/runtime-committed-$committed_case.xdg/docket/config.yml"
  case "$committed_case" in
    empty) printf "runtime:\n  bash: ''\n" >"$tmp/runtime-committed-$committed_case/.docket.yml" ;;
    duplicate)
      printf 'runtime:\n  bash: relative-invalid\nruntime:\n  bash: /also/missing\n' \
        >"$tmp/runtime-committed-$committed_case/.docket.yml"
      printf 'runtime:\n  bash: %s\n' "$tmp/runtime-bin/local-bash" \
        >"$tmp/runtime-committed-$committed_case/.docket.local.yml" ;;
  esac
  git -C "$tmp/runtime-committed-$committed_case" add .docket.yml
  git -C "$tmp/runtime-committed-$committed_case" commit --quiet -m cfg
  git -C "$tmp/runtime-committed-$committed_case" push --quiet origin main
  committed_err="$(XDG_CONFIG_HOME="$tmp/runtime-committed-$committed_case.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-committed-$committed_case" --export 2>&1 >/dev/null)"; committed_rc=$?
  committed_out="$(rung "$tmp/runtime-committed-$committed_case.xdg" "$tmp/runtime-committed-$committed_case" --export 2>/dev/null)"
  assert "0132 committed $committed_case: ignored layer does not abort" '[ "$committed_rc" -eq 0 ]'
  assert "0132 committed $committed_case: presence emits fence warning" \
    'grep -q "committed.*runtime.bash.*ignored" <<<"$committed_err"'
  expected_fallback="$tmp/runtime-bin/global-bash"
  [ "$committed_case" = duplicate ] && expected_fallback="$tmp/runtime-bin/local-bash"
  assert "0132 committed $committed_case: valid machine-local fallback wins" \
    'grep -qxF "DOCKET_BASH_PATH=$expected_fallback" <<<"$committed_out"'
done

# Duplicate runtime authorities in one machine-local layer fail closed. This catches files left by
# an older installer where a stale managed block coexists with a valid hand-authored declaration.
mkrepo "$tmp/runtime-ambiguous"
mkdir -p "$tmp/runtime-ambiguous.xdg/docket"
printf '# >>> docket (runtime.bash) >>>\nruntime:\n  bash: %s\n# <<< docket (runtime.bash) <<<\nruntime:\n  bash: %s\n' \
  "$tmp/runtime-bin/committed-bash" "$tmp/runtime-bin/global-bash" \
  > "$tmp/runtime-ambiguous.xdg/docket/config.yml"
runtime_ambiguous_out="$(XDG_CONFIG_HOME="$tmp/runtime-ambiguous.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-ambiguous" --export 2>"$tmp/runtime-ambiguous.err")"; runtime_ambiguous_rc=$?
assert "0132 runtime authority: duplicate global declarations abort" '[ "$runtime_ambiguous_rc" -ne 0 ]'
assert "0132 runtime authority: ambiguous export is empty" '[ -z "$runtime_ambiguous_out" ]'
assert "0132 runtime authority: diagnostic asks for one declaration" \
  'grep -Eqi "multiple|exactly one|one runtime" "$tmp/runtime-ambiguous.err"'

# Invalid explicit machine-local values fail closed and emit no captured runtime.
mkrepo "$tmp/runtime-invalid"
mkdir -p "$tmp/runtime-invalid.xdg/docket"
fake_bash "$tmp/runtime-bin/not-executable" 'GNU bash, version 5.2.0(1)-release' no
fake_bash "$tmp/runtime-bin/legacy-bash" 'GNU bash, version 3.2.57(1)-release'
fake_bash "$tmp/runtime-bin/not-bash" 5.2.0
for runtime_case in relative missing nonexec legacy notbash; do
  case "$runtime_case" in
    relative) runtime_value='bash' ;;
    missing)  runtime_value="$tmp/runtime-bin/does-not-exist" ;;
    nonexec)  runtime_value="$tmp/runtime-bin/not-executable" ;;
    legacy)   runtime_value="$tmp/runtime-bin/legacy-bash" ;;
    notbash)  runtime_value="$tmp/runtime-bin/not-bash" ;;
  esac
  printf 'runtime:\n  bash: %s\n' "$runtime_value" \
    >"$tmp/runtime-invalid.xdg/docket/config.yml"
  runtime_invalid_out="$(rung "$tmp/runtime-invalid.xdg" "$tmp/runtime-invalid" --export 2>/dev/null)"
  runtime_invalid_rc="$(rung_rc "$tmp/runtime-invalid.xdg" "$tmp/runtime-invalid" --export)"
  runtime_invalid_err="$(XDG_CONFIG_HOME="$tmp/runtime-invalid.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-invalid" --export 2>&1 >/dev/null)"
  assert "0132 runtime invalid $runtime_case: resolver aborts" '[ "$runtime_invalid_rc" != 0 ]'
  assert "0132 runtime invalid $runtime_case: export is empty" '[ -z "$runtime_invalid_out" ]'
  # No per-variable `[ -z "$DOCKET_BASH_PATH" ]` assert here (change 0148). `export is empty` one
  # line above is the SOLE CHANNEL: docket-config.sh --export writes shell assignments to stdout and
  # nothing else, and a subprocess has no channel into the parent's environment — so an empty export
  # admits NO exported variable, and a per-variable restatement is implied, not additive. The
  # deleted assert was also unfalsifiable: a `DOCKET_BASH_PATH=""` seed above it forced the value the
  # assert demanded. Do NOT "repair" it by inserting an `eval "$out"` on a provably-empty export —
  # that is a no-op added solely to satisfy a guard's site-detection heuristic.
  assert "0132 runtime invalid $runtime_case: diagnostic names runtime.bash" \
    'grep -qF "runtime.bash" <<<"$runtime_invalid_err"'
  assert "0132 runtime invalid $runtime_case: diagnostic gives install/upgrade remedy" \
    'grep -Eq "docket/install.sh|brew install bash" <<<"$runtime_invalid_err"'
done

# Absence is distinct from a configured path that names a missing file: both fail closed, but the
# former has no runtime block at all. Bypass rung() so its legacy-fixture seed cannot mask this.
mkrepo "$tmp/runtime-absent"
runtime_absent_out="$(XDG_CONFIG_HOME="$tmp/runtime-absent.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-absent" --export 2>/dev/null)"
runtime_absent_rc=$?
runtime_absent_err="$(XDG_CONFIG_HOME="$tmp/runtime-absent.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-absent" --export 2>&1 >/dev/null)"
assert "0132 runtime absent: resolver aborts" '[ "$runtime_absent_rc" != 0 ]'
assert "0132 runtime absent: export is empty" '[ -z "$runtime_absent_out" ]'
# No per-variable `[ -z "$DOCKET_BASH_PATH" ]` assert here (change 0148). `export is empty` one
# line above is the SOLE CHANNEL: docket-config.sh --export writes shell assignments to stdout and
# nothing else, and a subprocess has no channel into the parent's environment — so an empty export
# admits NO exported variable, and a per-variable restatement is implied, not additive. The
# deleted assert was also unfalsifiable: a `DOCKET_BASH_PATH=""` seed above it forced the value the
# assert demanded. Do NOT "repair" it by inserting an `eval "$out"` on a provably-empty export —
# that is a no-op added solely to satisfy a guard's site-detection heuristic.
assert "0132 runtime absent: diagnostic names runtime.bash" \
  'grep -qF "runtime.bash" <<<"$runtime_absent_err"'
assert "0132 runtime absent: diagnostic gives install/upgrade remedy" \
  'grep -Eq "docket/install.sh|brew install bash" <<<"$runtime_absent_err"'

# A valid machine-local executable path may contain apostrophes and backslashes. The YAML fixture
# uses the canonical single-quoted encoding (apostrophe doubled; backslash literal), while shell
# export remains eval-safe and plain format remains byte-literal.
odd_runtime_dir="$tmp/runtime-bin/odd'quote\\slash"
mkdir -p "$odd_runtime_dir"
fake_bash "$odd_runtime_dir/bash" 'GNU bash, version 5.2.0(1)-release'
mkrepo "$tmp/runtime-odd"
mkdir -p "$tmp/runtime-odd.xdg/docket"
odd_yaml_path="${odd_runtime_dir//\'/\'\'}/bash"
printf "runtime:\n  bash: '%s'\n" "$odd_yaml_path" >"$tmp/runtime-odd.xdg/docket/config.yml"
DOCKET_BASH_PATH=__poison__
runtime_odd_out="$(rung "$tmp/runtime-odd.xdg" "$tmp/runtime-odd" --export)"
eval "$runtime_odd_out"
assert "0132 odd runtime: shell export evaluates to exact executable path" \
  '[ "$DOCKET_BASH_PATH" = "$odd_runtime_dir/bash" ]'
runtime_odd_plain="$(rung "$tmp/runtime-odd.xdg" "$tmp/runtime-odd" --export --format plain)"
assert "0132 odd runtime: plain export preserves apostrophe and backslash" \
  'grep -qxF "DOCKET_BASH_PATH=$odd_runtime_dir/bash" <<<"$runtime_odd_plain"'

# Control bytes are never valid in an executable identity. A carriage return can survive the
# scalar parser when it appears inside quotes, so give it a real executable target: without an
# explicit resolver validation this fixture passes every later absolute/executable/version check.
cr_runtime="$tmp/runtime-bin/carriage"$'\r'"return-bash"
fake_bash "$cr_runtime" 'GNU bash, version 5.2.0(1)-release'
mkrepo "$tmp/runtime-control-byte"
mkdir -p "$tmp/runtime-control-byte.xdg/docket"
printf "runtime:\n  bash: '%s'\n" "$cr_runtime" \
  >"$tmp/runtime-control-byte.xdg/docket/config.yml"
runtime_control_out="$(XDG_CONFIG_HOME="$tmp/runtime-control-byte.xdg" bash "$SCRIPT" \
  --repo-dir "$tmp/runtime-control-byte" --export 2>"$tmp/runtime-control-byte.err")"
runtime_control_rc=$?
assert "0132 runtime control byte: carriage return is rejected before execution" \
  '[ "$runtime_control_rc" -ne 0 ]'
assert "0132 runtime control byte: rejected runtime emits no export" \
  '[ -z "$runtime_control_out" ]'
assert "0132 runtime control byte: diagnostic names forbidden CR/LF bytes" \
  'grep -qF "carriage returns or newlines" "$tmp/runtime-control-byte.err"'

# --- change 0153: a too-deeply-nested runtime.bash leaf is a NAMED error, never an absence ---
# Both rc consumers hard-code non-zero to mean "multiple declarations", so an unmapped rc 3 would
# emit an actively FALSE diagnostic naming a duplicate that does not exist.
mkrepo "$tmp/runtime-deep"
mkdir -p "$tmp/runtime-deep.xdg/docket"
printf 'runtime:\n  codex:\n    bash: %s\n' "$tmp/runtime-bin/global-bash" \
  > "$tmp/runtime-deep.xdg/docket/config.yml"
deep_rc="$(rung_rc "$tmp/runtime-deep.xdg" "$tmp/runtime-deep" --export)"
deep_err="$(XDG_CONFIG_HOME="$tmp/runtime-deep.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-deep" --export 2>&1 >/dev/null)"
assert "0153 deep global: resolver aborts" '[ "$deep_rc" != 0 ]'
assert "0153 deep global: diagnostic names the nesting depth, not a duplicate" \
  'grep -qF "exactly one level" <<<"$deep_err"'
assert "0153 deep global: diagnostic does NOT claim multiple declarations" \
  '! grep -qF "multiple runtime.bash declarations" <<<"$deep_err"'
assert "0153 deep global: diagnostic names the offending file" \
  'grep -qF "config.yml" <<<"$deep_err"'

# The repo-local twin — same shape, different file, and it must name ITS file.
mkrepo "$tmp/runtime-deep-local"
printf 'runtime:\n  codex:\n    bash: %s\n' "$tmp/runtime-bin/local-bash" \
  > "$tmp/runtime-deep-local/.docket.local.yml"
deepl_rc="$(rung_rc "$tmp/runtime-deep-local.xdg" "$tmp/runtime-deep-local" --export)"
deepl_err="$(XDG_CONFIG_HOME="$tmp/runtime-deep-local.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-deep-local" --export 2>&1 >/dev/null)"
assert "0153 deep local: resolver aborts" '[ "$deepl_rc" != 0 ]'
assert "0153 deep local: diagnostic names .docket.local.yml, not a duplicate" \
  'grep -qF ".docket.local.yml" <<<"$deepl_err" && ! grep -qF "multiple runtime.bash declarations" <<<"$deepl_err"'

# NON-REGRESSION: a genuine duplicate must STILL get the duplicate message, not the depth one.
mkrepo "$tmp/runtime-dup"
printf 'runtime:\n  bash: /a\n  bash: /b\n' > "$tmp/runtime-dup/.docket.local.yml"
dup_err="$(XDG_CONFIG_HOME="$tmp/runtime-dup.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-dup" --export 2>&1 >/dev/null)"
assert "0153: a real duplicate still gets the duplicate diagnostic" \
  'grep -qF "multiple runtime.bash declarations" <<<"$dup_err"'

# ============================================================================
# change 0127 — change_types + the nested auto_capture map
# ============================================================================
ct_get(){ printf '%s\n' "$2" | sed -n "s/^$1=//p"; }
# The repo-COMMITTED layer is read from `origin/HEAD:.docket.yml` (docket-config.sh's `g show`), never
# from the worktree, so a fixture must commit AND push it or it resolves as absent.
ct_commit(){ # ct_commit <repo-dir>
  git -C "$1" add .docket.yml >/dev/null 2>&1
  git -C "$1" commit -qm "cfg" >/dev/null 2>&1
  git -C "$1" push -q origin HEAD:main >/dev/null 2>&1
}

# --- defaults ---------------------------------------------------------------
mkrepo "$tmp/ct-default"
ct_out="$(run "$tmp/ct-default" --export --format plain)"
assert "0127 ct: CHANGE_TYPES defaults to the built-in taxonomy" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "chore docs feat fix refactor perf" ]'
assert "0127 ct: AUTO_CAPTURE_ENABLED defaults false" \
  '[ "$(ct_get AUTO_CAPTURE_ENABLED "$ct_out")" = "false" ]'
assert "0127 ct: AUTO_CAPTURE_TYPES defaults to the literal all" \
  '[ "$(ct_get AUTO_CAPTURE_TYPES "$ct_out")" = "all" ]'
assert "0127 ct: the retired AUTO_CAPTURE export is gone" \
  '! grep -q "^AUTO_CAPTURE=" <<<"$ct_out"'

# --- repo-committed map, both leaves ----------------------------------------
mkrepo "$tmp/ct-repo"
printf 'change_types: [feat, fix, chore]\nauto_capture:\n  enabled: true\n  types: [feat]\n' \
  >"$tmp/ct-repo/.docket.yml"
ct_commit "$tmp/ct-repo"
ct_out="$(run "$tmp/ct-repo" --export --format plain)"
assert "0127 ct: repo change_types replaces the built-in list wholesale" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "feat fix chore" ]'
assert "0127 ct: repo auto_capture.enabled resolves" \
  '[ "$(ct_get AUTO_CAPTURE_ENABLED "$ct_out")" = "true" ]'
assert "0127 ct: repo auto_capture.types resolves" \
  '[ "$(ct_get AUTO_CAPTURE_TYPES "$ct_out")" = "feat" ]'

# --- PER-LEAF inheritance: local overrides enabled, inherits types -----------
mkrepo "$tmp/ct-leaf"
printf 'auto_capture:\n  enabled: false\n  types: [fix]\n' >"$tmp/ct-leaf/.docket.yml"
ct_commit "$tmp/ct-leaf"
printf 'auto_capture:\n  enabled: true\n'                  >"$tmp/ct-leaf/.docket.local.yml"
ct_out="$(run "$tmp/ct-leaf" --export --format plain)"
assert "0127 ct: local layer overrides auto_capture.enabled" \
  '[ "$(ct_get AUTO_CAPTURE_ENABLED "$ct_out")" = "true" ]'
assert "0127 ct: auto_capture.types is INHERITED from the repo layer (per-leaf fallback)" \
  '[ "$(ct_get AUTO_CAPTURE_TYPES "$ct_out")" = "fix" ]'

# --- whole-list replacement, never concatenation ----------------------------
mkrepo "$tmp/ct-replace"
printf 'change_types: [chore, docs, feat]\n' >"$tmp/ct-replace/.docket.yml"
ct_commit "$tmp/ct-replace"
printf 'change_types: [feat]\n'             >"$tmp/ct-replace/.docket.local.yml"
ct_out="$(run "$tmp/ct-replace" --export --format plain)"
assert "0127 ct: a higher-layer list REPLACES the lower list, never merges" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "feat" ]'

# --- a NOVEL type, not merely a subset of the built-ins ---------------------
# Every replacement case above happens to restate built-in tokens, so none of them proves a token
# outside DOCKET_CHANGE_TYPES_DEFAULT survives resolution. README documents exactly this shape
# (`change_types: [chore, docs, feat, fix, spike]`) as how you extend the taxonomy, and
# auto_capture.types must be able to name it — that pairing is what the README example promises.
mkrepo "$tmp/ct-novel"
printf 'change_types: [chore, docs, feat, fix, spike]\nauto_capture:\n  enabled: true\n  types: [spike, fix]\n' \
  >"$tmp/ct-novel/.docket.yml"
ct_commit "$tmp/ct-novel"
ct_out="$(run "$tmp/ct-novel" --export --format plain)"
assert "0127 ct: a custom type outside the built-in taxonomy resolves" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "chore docs feat fix spike" ]'
assert "0127 ct: auto_capture.types may name that custom type" \
  '[ "$(ct_get AUTO_CAPTURE_TYPES "$ct_out")" = "spike fix" ]'
# The removal half of "replaced, never merged": perf is a built-in, and dropping it must stick.
assert "0127 ct: a built-in dropped from the list is genuinely gone" \
  '! grep <<<"$(ct_get CHANGE_TYPES "$ct_out")" -qw perf'

# --- cross-layer precedence: repo-local > repo-committed > global > built-in -
mkrepo "$tmp/ct-prec"
mkdir -p "$tmp/ct-prec.xdg/docket"
printf 'change_types: [chore]\n' >"$tmp/ct-prec.xdg/docket/config.yml"
ct_out="$(rung "$tmp/ct-prec.xdg" "$tmp/ct-prec" --export --format plain)"
assert "0127 ct: global layer resolves when both repo layers are silent" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "chore" ]'
printf 'change_types: [docs]\n' >"$tmp/ct-prec/.docket.yml"
ct_commit "$tmp/ct-prec"
ct_out="$(rung "$tmp/ct-prec.xdg" "$tmp/ct-prec" --export --format plain)"
assert "0127 ct: repo-committed beats global" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "docs" ]'
printf 'change_types: [perf]\n' >"$tmp/ct-prec/.docket.local.yml"
ct_out="$(rung "$tmp/ct-prec.xdg" "$tmp/ct-prec" --export --format plain)"
assert "0127 ct: repo-local beats repo-committed" \
  '[ "$(ct_get CHANGE_TYPES "$ct_out")" = "perf" ]'

# --- an explicit `all` survives serialization as the literal all -------------
mkrepo "$tmp/ct-all"
printf 'auto_capture:\n  enabled: true\n  types: all\n' >"$tmp/ct-all/.docket.yml"
ct_commit "$tmp/ct-all"
ct_out="$(run "$tmp/ct-all" --export --format plain)"
assert "0127 ct: an explicitly written 'all' stays the literal all" \
  '[ "$(ct_get AUTO_CAPTURE_TYPES "$ct_out")" = "all" ]'

# --- fail-closed cases -------------------------------------------------------
ct_fail_n=0
ct_fails(){ # ct_fails <label> <yaml-body> <expected-substring>
  ct_fail_n=$((ct_fail_n + 1))
  local d="$tmp/ctf-$ct_fail_n" err rc
  mkrepo "$d"
  printf '%b' "$2" >"$d/.docket.yml"
  ct_commit "$d"
  err="$(run "$d" --export --format plain 2>&1 >/dev/null)"; rc=$?
  assert "0127 ct-fail: $1 exits non-zero" '[ "'"$rc"'" -ne 0 ]'
  assert "0127 ct-fail: $1 diagnostic mentions '$3'" 'grep -qF -- "'"$3"'" <<<"'"$err"'"'
}
ct_fails "legacy scalar true"      'auto_capture: true\n'                                    'auto_capture'
ct_fails "legacy scalar false"     'auto_capture: false\n'                                   'auto_capture'
ct_fails "empty change_types"      'change_types: []\n'                                      'change_types'
ct_fails "duplicate change_types"  'change_types: [feat, feat]\n'                            'duplicate'
ct_fails "malformed type token"    'change_types: [Feat]\n'                                  'change_types'
ct_fails "reserved type in list"   'change_types: [feat, all]\n'                             'reserved'
ct_fails "non-boolean enabled"     'auto_capture:\n  enabled: yes\n'                         'auto_capture.enabled'
ct_fails "types outside taxonomy"  'change_types: [feat]\nauto_capture:\n  types: [docs]\n'  'docs'
ct_fails "duplicate types"         'auto_capture:\n  types: [feat, feat]\n'                  'duplicate'
ct_fails "empty types list"        'auto_capture:\n  types: []\n'                            'auto_capture.types'

# The legacy diagnostic must print a remedy valid in the state that produced it
# (learning: printed-remedy-state-validity).
mkrepo "$tmp/ct-legacy"
printf 'auto_capture: true\n' >"$tmp/ct-legacy/.docket.yml"
ct_commit "$tmp/ct-legacy"
ct_err="$(run "$tmp/ct-legacy" --export --format plain 2>&1 >/dev/null)" || true
assert "0127 ct: legacy diagnostic shows the nested replacement shape" \
  'grep -q "enabled:" <<<"$ct_err" && grep -q "types:" <<<"$ct_err"'
assert "0127 ct: legacy diagnostic carries the user's OWN value into the remedy" \
  'grep -q "enabled: true" <<<"$ct_err"'
mkrepo "$tmp/ct-legacy-f"
printf 'auto_capture: false\n' >"$tmp/ct-legacy-f/.docket.yml"
ct_commit "$tmp/ct-legacy-f"
ct_errf="$(run "$tmp/ct-legacy-f" --export --format plain 2>&1 >/dev/null)" || true
assert "0127 ct: legacy diagnostic remedy is branched on the actual value, not fixed" \
  'grep -q "enabled: false" <<<"$ct_errf"'

# A machine-scoped legacy scalar is caught too — the fence never silently swallows it.
mkrepo "$tmp/ct-legacy-local"
printf 'auto_capture: true\n' >"$tmp/ct-legacy-local/.docket.local.yml"
assert "0127 ct: a legacy scalar in the LOCAL layer also fails closed" \
  '! run "$tmp/ct-legacy-local" --export --format plain >/dev/null 2>&1'

# An observed type absent from the effective taxonomy must not break resolution — config governs
# creation, never the readability of history.
mkrepo "$tmp/ct-narrow"
printf 'change_types: [feat]\n' >"$tmp/ct-narrow/.docket.yml"
ct_commit "$tmp/ct-narrow"
assert "0127 ct: a narrowed taxonomy still resolves cleanly" \
  'run "$tmp/ct-narrow" --export --format plain >/dev/null 2>&1'

# ============================================================================
# change 0128 — cross-layer auto_capture.types validation + glob-proof inline lists
# ============================================================================

# --- (A) the two keys resolve through INDEPENDENT chains ---------------------
# `change_types` resolves by WHOLE-LIST replacement (the first layer that sets it wins outright);
# `auto_capture.types` resolves PER-LEAF inside the block. Both are documented features, and
# composing them must not abort the resolver: the membership check runs against the change_types
# the author AT THE LAYER THAT SUPPLIED `types` could see, never against the globally effective
# CHANGE_TYPES. Validating against the latter let a higher layer that merely narrowed
# change_types invalidate a LOWER layer's perfectly valid block — and since every skill's Step 0
# runs `docket.sh preflight`, one machine-local narrowing bricked docket on that machine entirely.
# Pinned in BOTH directions: cross-layer composes, same-layer inconsistency still dies.

# A1 — committed `types`, machine-local `change_types` narrowing. Asserted at both `enabled`
# polarities: the disabled case is the sharper one — the leaf governs nothing there, so aborting
# the whole resolver over it is pure collateral damage.
for xl_en in false true; do
  xl_d="$tmp/ac-xlayer-$xl_en"
  mkrepo "$xl_d"
  printf 'auto_capture:\n  enabled: %s\n  types: [docs]\n' "$xl_en" >"$xl_d/.docket.yml"
  ct_commit "$xl_d"                                    # committed layer: types, NO change_types
  printf 'change_types: [feat, fix]\n' >"$xl_d/.docket.local.yml"   # local layer: narrows only
  xl_out="$(run "$xl_d" --export --format plain 2>"$xl_d.err")"; xl_rc=$?
  assert "0128 xlayer(enabled=$xl_en): local change_types narrowing does not abort the resolver" \
    '[ "$xl_rc" -eq 0 ]'
  assert "0128 xlayer(enabled=$xl_en): committed auto_capture.types survives the narrowing" \
    '[ "$(ct_get AUTO_CAPTURE_TYPES "$xl_out")" = "docs" ]'
  assert "0128 xlayer(enabled=$xl_en): the local change_types still replaces the taxonomy" \
    '[ "$(ct_get CHANGE_TYPES "$xl_out")" = "feat fix" ]'
done

# A2 — the global layer supplies `types`; the repo's committed .docket.yml narrows change_types.
# Same shape one rung down: the global author could only ever see the built-in taxonomy.
mkrepo "$tmp/ac-xlayer-g"
mkdir -p "$tmp/ac-xlayer-g.xdg/docket"
printf 'auto_capture:\n  enabled: true\n  types: [docs]\n' >"$tmp/ac-xlayer-g.xdg/docket/config.yml"
printf 'change_types: [feat, fix]\n' >"$tmp/ac-xlayer-g/.docket.yml"
ct_commit "$tmp/ac-xlayer-g"
xlg_out="$(rung "$tmp/ac-xlayer-g.xdg" "$tmp/ac-xlayer-g" --export --format plain 2>"$tmp/ac-xlayer-g.err")"; xlg_rc=$?
assert "0128 xlayer(global): a committed change_types narrowing does not invalidate global types" \
  '[ "$xlg_rc" -eq 0 ]'
assert "0128 xlayer(global): global auto_capture.types resolves through the narrowing" \
  '[ "$(ct_get AUTO_CAPTURE_TYPES "$xlg_out")" = "docs" ]'
assert "0128 xlayer(global): the committed change_types still wins the taxonomy" \
  '[ "$(ct_get CHANGE_TYPES "$xlg_out")" = "feat fix" ]'

# A3 — the other direction: ONE layer stating both keys inconsistently is still a hard error, and
# the diagnostic names the offending token AND the layer whose author wrote it. Cross-layer
# tolerance must not degrade into never checking.
mkrepo "$tmp/ac-samelayer-c"
printf 'change_types: [feat, fix]\nauto_capture:\n  enabled: true\n  types: [docs]\n' \
  >"$tmp/ac-samelayer-c/.docket.yml"
ct_commit "$tmp/ac-samelayer-c"
slc_out="$(run "$tmp/ac-samelayer-c" --export --format plain 2>"$tmp/ac-samelayer-c.err")"; slc_rc=$?
assert "0128 same-layer(committed): change_types/types inconsistency still exits non-zero" \
  '[ "$slc_rc" -ne 0 ]'
assert "0128 same-layer(committed): diagnostic names the offending token" \
  'grep -qF "docs" "$tmp/ac-samelayer-c.err"'
assert "0128 same-layer(committed): diagnostic names the committed layer" \
  'grep -qF "committed" "$tmp/ac-samelayer-c.err"'
assert "0128 same-layer(committed): the rejected run emits nothing" '[ -z "$slc_out" ]'

mkrepo "$tmp/ac-samelayer-l"
printf 'change_types: [feat, fix]\nauto_capture:\n  enabled: true\n  types: [docs]\n' \
  >"$tmp/ac-samelayer-l/.docket.local.yml"
sll_out="$(run "$tmp/ac-samelayer-l" --export --format plain 2>"$tmp/ac-samelayer-l.err")"; sll_rc=$?
assert "0128 same-layer(local): change_types/types inconsistency still exits non-zero" \
  '[ "$sll_rc" -ne 0 ]'
assert "0128 same-layer(local): diagnostic names the offending token" \
  'grep -qF "docs" "$tmp/ac-samelayer-l.err"'
assert "0128 same-layer(local): diagnostic names the local layer" \
  'grep -qF "local" "$tmp/ac-samelayer-l.err"'
assert "0128 same-layer(local): the rejected run emits nothing" '[ -z "$sll_out" ]'

# --- (B) inline-list values must NEVER pathname-expand ------------------------
# Normalization once ended in an unquoted `$(echo $body)` with no `set -f`, so `change_types: [f*]`
# resolved the taxonomy from FILENAMES IN THE RESOLVER'S CWD: the same committed config produced a
# different taxonomy on different machines, silently, because every expanded lowercase filename
# passes the well-formedness check downstream. Decoy entries named after real types make the
# regression observable — mirrors the `set -f` decoy in tests/test_sync_agents.sh.
glob_cwd="$tmp/inline-glob-decoy-cwd"
mkdir -p "$glob_cwd/feat" "$glob_cwd/fix"       # `f*` expands to exactly `feat fix` here
# run_in <cwd> <repo-dir> [args...] : run the resolver FROM <cwd> (--repo-dir keeps the repo
# anchor independent of it), so the decoys are on the expansion path and nowhere else.
run_in(){ local c="$1" d="$2"; shift 2; ensure_test_runtime "$XDG_CONFIG_HOME" "$d"
  ( cd "$c" && bash "$SCRIPT" --repo-dir "$d" "$@" ); }

mkrepo "$tmp/glob-ct"
printf 'change_types: [f*]\n' >"$tmp/glob-ct/.docket.yml"
ct_commit "$tmp/glob-ct"
glob_ct_out="$(run_in "$glob_cwd" "$tmp/glob-ct" --export --format plain 2>"$tmp/glob-ct.err")"; glob_ct_rc=$?
assert "0128 glob: change_types [f*] does NOT expand against the resolver's cwd" \
  '! grep -qxF "CHANGE_TYPES=feat fix" <<<"$glob_ct_out"'
assert "0128 glob: change_types [f*] is rejected as malformed instead" '[ "$glob_ct_rc" -ne 0 ]'
assert "0128 glob: the change_types diagnostic quotes the LITERAL token" \
  'grep -qF "f*" "$tmp/glob-ct.err"'

mkrepo "$tmp/glob-act"
printf 'auto_capture:\n  enabled: true\n  types: [f*]\n' >"$tmp/glob-act/.docket.yml"
ct_commit "$tmp/glob-act"
glob_act_out="$(run_in "$glob_cwd" "$tmp/glob-act" --export --format plain 2>"$tmp/glob-act.err")"; glob_act_rc=$?
assert "0128 glob: auto_capture.types [f*] does NOT expand against the resolver's cwd" \
  '! grep -qxF "AUTO_CAPTURE_TYPES=feat fix" <<<"$glob_act_out"'
assert "0128 glob: auto_capture.types [f*] is rejected as a non-member instead" \
  '[ "$glob_act_rc" -ne 0 ]'
assert "0128 glob: the auto_capture.types diagnostic quotes the LITERAL token" \
  'grep -qF "f*" "$tmp/glob-act.err"'

# --- (T) prelude correspondence guard (change 0126) ---------------------------
# Every `eval "$V"` of resolver output must clear the exported variables the
# asserts between it and the NEXT eval site read. The window is "anything since
# the previous eval site", not "the preceding line": the hazard is a stale value
# left by the PREVIOUS fixture's eval, so a clearing anywhere in between kills
# it. That is also what lets the pre-existing `unset`-idiom blocks satisfy this
# guard byte-untouched (change 0126, spec assumption 2).
#
# Corpus is the WHOLE file minus this section's own marker-delimited self-block.
# Deliberately NOT truncated at an end-of-file marker: the file's tail is where
# new fixtures land, so truncation would make them permanently invisible.

prelude_report(){
  local file="$1" keys="$2"
  awk -v keys="$keys" '
    { L[NR] = $0 }
    END {
      n = NR
      split(keys, ka, " "); for (i in ka) KEY[ka[i]] = 1

      # --- locate this guards own self-block (FIRST occurrence of each marker;
      # the literal necessarily appears twice - the marker and the pattern that
      # searches for it).
      sstart = 0; send = 0
      for (i = 1; i <= n; i++) {
        if (sstart == 0 && index(L[i], SELFSTART) > 0) sstart = i
        else if (sstart != 0 && send == 0 && index(L[i], SELFEND) > 0) send = i
      }

      # --- discover sites: eval "$V" where V came from a command substitution
      ns = 0
      split("", cmdsub)
      for (i = 1; i <= n; i++) {
        if (sstart != 0 && i >= sstart && i <= send) continue
        line = L[i]; s = line; sub(/^[ \t]+/, "", s)
        if (s ~ /^#/) continue
        tmp = line
        while (match(tmp, /[A-Za-z_][A-Za-z0-9_]*="?\$\(/)) {
          seg = substr(tmp, RSTART, RLENGTH); sub(/=.*$/, "", seg); cmdsub[seg] = 1
          tmp = substr(tmp, RSTART + RLENGTH)
        }
        tmp = line
        while (match(tmp, /eval[ \t]+"\$[A-Za-z_][A-Za-z0-9_]*"/)) {
          seg = substr(tmp, RSTART, RLENGTH); v = seg
          sub(/^eval[ \t]+"\$/, "", v); sub(/"$/, "", v)
          if (v in cmdsub) { ns++; SL[ns] = i }
          tmp = substr(tmp, RSTART + RLENGTH)
        }
      }

      exempt = 0; okc = 0; viol = 0
      for (k = 1; k <= ns; k++) {
        lo = SL[k]; hi = (k < ns ? SL[k+1] - 1 : n)

        # asserted exported variables in this segment (matches ${VAR-unset} too,
        # otherwise every existing unset-idiom assert reads as an empty
        # intersection and is wrongly exempted)
        split("", need)
        for (i = lo; i <= hi; i++) {
          if (sstart != 0 && i >= sstart && i <= send) continue
          s = L[i]; t = s; sub(/^[ \t]+/, "", t); if (t ~ /^#/) continue
          tmp = s
          while (match(tmp, /\$\{?[A-Za-z_][A-Za-z0-9_]*/)) {
            w = substr(tmp, RSTART, RLENGTH); sub(/^\$\{?/, "", w)
            if (w in KEY) need[w] = 1
            tmp = substr(tmp, RSTART + RLENGTH)
          }
        }

        # clearing window: since the previous site, through this eval line.
        # A clear counts only as `VAR=__poison__` or `unset VAR`. `VAR=""` does
        # NOT count: the empty string is a real resolver output, so before an
        # assert like `[ -z "$VAR" ]` the "clear" value and the asserted value are
        # identical and the assert stays exactly as vacuous as with no prelude at
        # all. `unset` stays legal because those blocks assert via ${VAR-unset},
        # which is discriminating (and set -u-safe).
        split("", cleared)
        wlo = (k > 1 ? SL[k-1] + 1 : 1)
        for (i = wlo; i <= lo; i++) {
          if (sstart != 0 && i >= sstart && i <= send) continue
          c = L[i]; q = c; sub(/^[ \t]+/, "", q); if (q ~ /^#/) continue
          tmp = c
          while (match(tmp, /[A-Za-z_][A-Za-z0-9_]*=__poison__/)) {
            w = substr(tmp, RSTART, RLENGTH); sub(/=.*$/, "", w); cleared[w] = 1
            tmp = substr(tmp, RSTART + RLENGTH)
          }
          if (c ~ /(^|[;&| \t])unset[ \t]/) {
            tmp = c; sub(/^.*unset[ \t]+/, "", tmp)
            nf = split(tmp, uu, /[ \t;]+/)
            for (j = 1; j <= nf; j++) if (uu[j] ~ /^[A-Z_][A-Z0-9_]*$/) cleared[uu[j]] = 1
          }
        }

        cnt = 0; miss = ""
        for (w in need) { cnt++; if (!(w in cleared)) miss = miss " " w }
        if (cnt == 0)        { exempt++; print "SITE " SL[k] " exempt" }
        else if (miss == "") { okc++;    print "SITE " SL[k] " ok" }
        else                 { viol++;   print "SITE " SL[k] " viol" miss }
      }
      print "TOTALS sites=" ns " exempt=" exempt " ok=" okc " viol=" viol
    }
  ' SELFSTART="$T_SELF_START" SELFEND="$T_SELF_END" "$file"
}

# docket:prelude-guard:self:start
# The two marker literals below are the guard's own scan patterns. Everything
# between the start and end marker is subtracted from the corpus.
T_SELF_START='docket:prelude-guard:self'':start'
T_SELF_END='docket:prelude-guard:self'':end'
T_EVAL_LITERAL='eval "$'
# docket:prelude-guard:self:end

# Key derivation is HERMETIC, like every other fixture in this file. A bare
# `bash "$SCRIPT" --export` would resolve against the INVOKING cwd: run the suite
# from anywhere outside a git repo and the resolver aborts, $t_keys goes empty and
# the keycount floor reddens the whole suite for a reason that has nothing to do
# with the code under test. It would also couple the guard to this repo's own
# committed .docket.yml. A throwaway fixture repo gives the same key set with
# neither dependency.
mkrepo "$tmp/tkeys"
t_keys="$(run "$tmp/tkeys" --export 2>/dev/null | sed 's/=.*//' | sort | tr '\n' ' ')"

# Vacuity floor: if the resolver's --export ever breaks, changes format, or the
# pipeline above silently swallows an error, $t_keys goes empty and EVERY site
# becomes "exempt by derivation" (no key can ever match an empty KEY set) —
# the guard would report sites=N exempt=N viol=0 and go GREEN having checked
# nothing. This floor makes an empty/truncated key set a RED suite instead
# (28 keys today; floor set well below that so ordinary drift doesn't trip it,
# but an empty or badly-truncated set does).
t_keycount="$(printf '%s\n' "$t_keys" | tr ' ' '\n' | sed '/^$/d' | wc -l | tr -d ' ')"

t_out="$(prelude_report "${BASH_SOURCE[0]}" "$t_keys")"
t_sites="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS sites=\([0-9]*\) .*/\1/p')"
t_viol="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS .* viol=\([0-9]*\)$/\1/p')"
# t_exempt is diagnostic-only, on purpose: since change 0149 replaced the absolute exempt ceiling
# with the proportional `ok` floor below, no assert reads this variable and nothing prints it.
# Kept extracted (not deleted) for a reader diffing this TOTALS line by eye — an unread variable
# here is deliberate, not an oversight.
t_exempt="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS .* exempt=\([0-9]*\) .*/\1/p')"
t_ok="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS .* ok=\([0-9]*\) .*/\1/p')"
# Print the TOTALS line AND every violating site. Printing totals alone leaves the
# next author staring at `viol=1` with no line number and no variable name.
printf '%s\n' "$t_out" | /usr/bin/grep -E '^(TOTALS|SITE .* viol)'

# Population floor, from a STRUCTURALLY DIFFERENT extractor: a plain grep of the
# raw literal, minus the known non-sites: the assert() helper at :8 (whose eval
# takes a positional rather than a cmdsub var), every COMMENT line that merely
# mentions the literal in prose (a real site is never a comment line — this
# mirrors the site-discovery awk's own `if (s ~ /^#/) continue`), and this
# guard's own T_EVAL_LITERAL holder (a quoted copy of the pattern, not a site).
# Each count is derived at runtime, not hand-counted, so file drift (new
# fixtures, new prose elsewhere) cannot silently desync the two extractors.
t_raw="$(/usr/bin/grep -cF "$T_EVAL_LITERAL" "${BASH_SOURCE[0]}")"
t_helper="$(/usr/bin/grep -cE '^assert\(\)\{' "${BASH_SOURCE[0]}")"
t_comments="$(/usr/bin/grep -E '^[[:space:]]*#' "${BASH_SOURCE[0]}" | /usr/bin/grep -cF "$T_EVAL_LITERAL")"
t_selflit="$(/usr/bin/grep -cE '^T_EVAL_LITERAL=' "${BASH_SOURCE[0]}")"
t_selfrefs="$(awk -v s="$T_SELF_START" -v e="$T_SELF_END" '
  index($0,s)>0 && !st {st=NR} index($0,e)>0 && st && !en {en=NR}
  END{ if (st && en) print en-st+1; else print 0 }' "${BASH_SOURCE[0]}")"

assert "0126 T: guard reached a real population (>= 60 sites)" '[ "$t_sites" -ge 60 ]'
assert "0126 T: the derived key set is non-vacuous (>= 20 keys)" '[ "$t_keycount" -ge 20 ]'
# Coverage floor — the twin of the keycount floor, and the successor to change 0126's absolute
# `t_exempt <= 5` ceiling (change 0149). An EMPTY key set is caught by t_keycount above; a WRONG one
# would make every site "exempt by derivation" and the guard would report viol=0 having checked
# nothing. The old ceiling was ABSOLUTE: a fixed 5 against a real 3 left two sites of headroom that
# did not grow with the file, so several legitimately-exempt fixtures landing together would trip it
# — a loud false red as it aged.
#
# A floor on `ok` is preferred to a ratio on `exempt` for two reasons: it measures the property that
# matters (coverage PROVEN) rather than its complement, and because `viol` must independently be 0,
# the floor bounds `exempt` without naming it. Note the arithmetic direction: `t_exempt * 5 <=
# t_sites` — the obvious "proportional" rewrite — would permit 12 exempt sites at 64, which is
# LOOSER than the absolute 5 it replaces. At today's 64 sites this floor permits 6 non-`ok` sites
# where the ceiling permitted 5: one site of immediate slack traded for slack that SCALES.
#
# MEASURED FINDING, recorded so the ceiling is not reinstated by someone re-deriving the original
# worry: this guard is NOT the rename detector. Renaming one emitted export key turns four ordinary
# asserts red while TOTALS comes back byte-identical (exempt does not move at all, because no eval
# window reads a key exclusively); renaming five aborts the run under `set -u` with an unbound
# variable before section (T) ever executes. Both cheaper layers already catch it.
assert "0126 T: the guard proved something at >=90% of sites (ok=$t_ok of $t_sites)" \
  '[ $(( t_ok * 10 )) -ge $(( t_sites * 9 )) ]'
assert "0126 T: site count agrees with the independent grep extractor" \
  '[ "$t_sites" -eq "$(( t_raw - t_helper - t_comments - t_selflit ))" ]'
assert "0126 T: the self-block is bounded and non-empty" '[ "$t_selfrefs" -ge 3 ]'
assert "0126 T: every eval site clears the exported vars its asserts read" \
  '[ "$t_viol" -eq 0 ]'

# Change 0148 post-conditions. `t_exempt` is deliberately NOT a tripwire here: it measured 3 both
# before and after the deletions, so an assert on its movement would pass while certifying nothing.
# The real invariants are that the guard still proves something everywhere, and that the
# require_pr_approval site kept a NON-EMPTY need set after losing its DOCKET_BASH_PATH poison —
# it retains FINALIZE_REQUIRE_PR_APPROVAL (an emitted key), so it does not fall into the exempt
# bucket and t_exempt legitimately stays 3. The site's line number is derived, not hand-counted:
# it is the last `eval "$out"` line for the r9 fixture seen before the assert that immediately
# follows it, so file drift above this point cannot silently desync the two.
r9_poison_site_line="$(awk '
  /out="\$\(rung "\$tmp\/r9\.xdg" "\$tmp\/r9" --export\)"; eval "\$out"/ { last = NR }
  /0102 R9: repo-local false beats global true/ { print last; exit }
' "${BASH_SOURCE[0]}")"
assert "0148: the require_pr_approval site still has a non-empty need set (not exempt)" \
  '/usr/bin/grep -qE "^SITE $r9_poison_site_line (ok|viol)" <<<"$t_out"'

# ============================================================================
# Change 0223 — gate_observation_budget (GATE_OBSERVATION_BUDGET)
# The build gate's artifact-observation budget, in MINUTES. A FLAT top-level key, so it resolves
# through config_scalar_get's per-field chain repo-local > repo-committed > global > built-in,
# exactly like auto_groom — behavioral local execution timing, not shared non-re-derivable state,
# so ADR-0019's coordination fence does not apply and a global value must be HONORED, not warned.
# Fail closed on garbage (the learnings.cap / review.max_fix_tasks precedent): a typo'd budget
# silently defaulting to 30 would make a fail-closed halt fire at a duration nobody chose.
#
# Every assert below reads the EMITTED LINES rather than eval-ing the export block. Two reasons,
# both the RMX block's: this is a brand-new export, so the asserts must survive it not existing at
# all (under `set -u` an eval of an empty export block leaves the var unbound and assert()'s own
# eval kills the suite instead of reddening), and the garbage fixtures deliberately ABORT, which
# emits nothing at all.
# Fixtures use this file's existing mkrepo/run/rung/run_resolver_with helpers — no second
# fixture shape.
# ============================================================================

# --- (GOB-a) the built-in default with no config file anywhere ----------------
mkrepo "$tmp/gob-a"
gob_out_default="$(run "$tmp/gob-a" --export)"
assert "GOB-a: default is 30 with no config" \
  'grep -qxF "GATE_OBSERVATION_BUDGET=30" <<<"$gob_out_default"'
assert "GOB-a: present in plain format too" \
  'grep -q "^GATE_OBSERVATION_BUDGET=" <<<"$(run "$tmp/gob-a" --export --format plain)"'

# --- (GOB-b) a repo-committed value wins over the built-in --------------------
mkrepo "$tmp/gob-b"
cat > "$tmp/gob-b/.docket.yml" <<'EOF'
metadata_branch: main
gate_observation_budget: 45
EOF
git -C "$tmp/gob-b" add .docket.yml; git -C "$tmp/gob-b" commit --quiet -m cfg
git -C "$tmp/gob-b" push --quiet origin main
gob_out_committed="$(run "$tmp/gob-b" --export)"
assert "GOB-b: repo-committed value is honored" \
  'grep -qxF "GATE_OBSERVATION_BUDGET=45" <<<"$gob_out_committed"'

# --- (GOB-c) global-able (ADR-0019 — NOT coordination-fenced) -----------------
mkrepo "$tmp/gob-c"
mkdir -p "$tmp/gob-c.xdg/docket"
printf 'gate_observation_budget: 15\n' > "$tmp/gob-c.xdg/docket/config.yml"
gob_out_global="$(rung "$tmp/gob-c.xdg" "$tmp/gob-c" --export 2>/dev/null)"
gob_err_global="$(rung "$tmp/gob-c.xdg" "$tmp/gob-c" --export 2>&1 >/dev/null)"
assert "GOB-c: global-layer value is honored" \
  'grep -qxF "GATE_OBSERVATION_BUDGET=15" <<<"$gob_out_global"'
# Non-vacuity companion is the positive assert directly above: the fence would resolve the key to
# the built-in AND warn, so only the pair distinguishes "honored" from "silently ignored".
assert "GOB-c: no fence warning for gate_observation_budget" \
  '! grep -qiE "gate_observation_budget.*per-repo-only" <<<"$gob_err_global"'

# --- (GOB-d) machine-local beats repo-committed (the top of the chain) --------
mkrepo "$tmp/gob-d"
cat > "$tmp/gob-d/.docket.yml" <<'EOF'
metadata_branch: main
gate_observation_budget: 45
EOF
git -C "$tmp/gob-d" add .docket.yml; git -C "$tmp/gob-d" commit --quiet -m cfg
git -C "$tmp/gob-d" push --quiet origin main
printf 'gate_observation_budget: 5\n' > "$tmp/gob-d/.docket.local.yml"
gob_out_local="$(run "$tmp/gob-d" --export)"
assert "GOB-d: .docket.local.yml outranks the committed value" \
  'grep -qxF "GATE_OBSERVATION_BUDGET=5" <<<"$gob_out_local"'

# --- (GOB-e) fail closed on a non-integer -------------------------------------
gob_out_bad="$(run_resolver_with "gate_observation_budget: many\n" 2>/dev/null)"; gob_rc_bad=$?
gob_err_bad="$(run_resolver_with "gate_observation_budget: many\n" 2>&1 >/dev/null)"
assert "GOB-e: a non-integer aborts" '[ "$gob_rc_bad" -ne 0 ]'
assert "GOB-e: the diagnostic names the key" \
  'grep -qF "gate_observation_budget" <<<"$gob_err_bad"'
assert "GOB-e: a negative budget aborts nonzero" \
  '! run_resolver_with "gate_observation_budget: -1\n" >/dev/null 2>&1'
assert "GOB-e: a fractional budget aborts nonzero" \
  '! run_resolver_with "gate_observation_budget: 2.5\n" >/dev/null 2>&1'

# --- (GOB-f) 0 is legal --------------------------------------------------------
# It means "observe once, then fail closed", matching the review.max_fix_tasks / learnings.cap
# precedent that gives 0 no magic meaning.
gob_out_zero="$(run_resolver_with "gate_observation_budget: 0\n" 2>/dev/null)"; gob_rc_zero=$?
assert "GOB-f: 0 does not abort the resolver" '[ "$gob_rc_zero" -eq 0 ]'
assert "GOB-f: 0 is legal" \
  'grep -qxF "GATE_OBSERVATION_BUDGET=0" <<<"$gob_out_zero"'

# --- (GOB-g) ORDER: after REVIEW_MAX_FIX_TASKS, before SKILL_BRAINSTORM -------
# The contract doc promises a stable emit order and pipe consumers may rely on it. Line numbers
# are derived per key rather than pattern-matched as a trio, so a missing key reads as an empty
# extraction (and reddens the non-vacuity floor) instead of shifting a positional match.
gob_ln(){ grep -n "^$1=" <<<"$gob_out_default" | cut -d: -f1; }
gob_n_rmx="$(gob_ln REVIEW_MAX_FIX_TASKS)"
gob_n_gob="$(gob_ln GATE_OBSERVATION_BUDGET)"
gob_n_brs="$(gob_ln SKILL_BRAINSTORM)"
assert "GOB-g: all three emit positions were extracted (rmx=$gob_n_rmx gob=$gob_n_gob brs=$gob_n_brs)" \
  '[ -n "$gob_n_rmx" ] && [ -n "$gob_n_gob" ] && [ -n "$gob_n_brs" ]'
assert "GOB-g: emitted between REVIEW_MAX_FIX_TASKS and SKILL_BRAINSTORM" \
  '[ "${gob_n_gob:-0}" -gt "${gob_n_rmx:-0}" ] && [ "${gob_n_gob:-0}" -lt "${gob_n_brs:-0}" ]'

# ---- change 0271: delegation_observation_budget (DOB-a … DOB-f) ----------------
# Sibling of gate_observation_budget: same layering, same fail-closed integer check,
# different default (60) because a delegated AGENT RUN is a longer unit than a suite run.
# Fixtures use this file's existing mkrepo/run/run_resolver_with helpers — the same
# fixture shape the GOB block above uses; every assert reads the EMITTED LINES rather
# than eval-ing the export block, for the reasons the GOB header states.
mkrepo "$tmp/dob-a"
dob_out_default="$(run "$tmp/dob-a" --export)"
assert "DOB-a: defaults to 60 with no config" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=60" <<<"$dob_out_default"'

mkrepo "$tmp/dob-b"
cat > "$tmp/dob-b/.docket.yml" <<'EOF'
metadata_branch: main
delegation_observation_budget: 15
EOF
git -C "$tmp/dob-b" add .docket.yml; git -C "$tmp/dob-b" commit --quiet -m cfg
git -C "$tmp/dob-b" push --quiet origin main
dob_out_committed="$(run "$tmp/dob-b" --export)"
assert "DOB-b: committed layer is honored" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=15" <<<"$dob_out_committed"'

mkrepo "$tmp/dob-c"
cat > "$tmp/dob-c/.docket.yml" <<'EOF'
metadata_branch: main
delegation_observation_budget: 15
EOF
git -C "$tmp/dob-c" add .docket.yml; git -C "$tmp/dob-c" commit --quiet -m cfg
git -C "$tmp/dob-c" push --quiet origin main
printf 'delegation_observation_budget: 5\n' > "$tmp/dob-c/.docket.local.yml"
dob_out_local="$(run "$tmp/dob-c" --export)"
assert "DOB-c: repo-local outranks committed" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=5" <<<"$dob_out_local"'

# 0 is legal and carries no magic — it means "observe once, then fail closed".
dob_out_zero="$(run_resolver_with "delegation_observation_budget: 0\n" 2>/dev/null)"
assert "DOB-d: 0 is legal, not a disabled gate" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=0" <<<"$dob_out_zero"'

# Fail CLOSED on garbage — a typo'd budget silently defaulting would make the
# fail-closed halt fire at a duration nobody chose.
run_resolver_with "delegation_observation_budget: soon\n" >/dev/null 2>&1; dob_rc_bad=$?
dob_err_bad="$(run_resolver_with "delegation_observation_budget: soon\n" 2>&1 >/dev/null)"
assert "DOB-e: a non-integer budget is fatal" '[ "$dob_rc_bad" != "0" ]'
assert "DOB-f: the diagnostic names the key" \
  'grep -qF "delegation_observation_budget" <<<"$dob_err_bad"'
# ============================================================================
# Change 0258 leg 1 — the doc's export fence vs the resolver's real emission
# ============================================================================
# scripts/docket-config.md's `### Emit` section sells SEQUENCE as contract ("printed as
# `KEY=value` lines to stdout in this order"), and (R7) above cites that promise as the reason
# its own adjacency assert exists. Until this guard the fence itself was pinned only by
# per-key PRESENCE greps plus two adjacency clusters (R7; the AUTO_GROOM -> CHANGE_TYPES ->
# AUTO_CAPTURE_* identity cluster), so a doc-side reorder stayed green. Those stay: they are
# their own changes' mutation witnesses on their own fixtures.
#
# The verdict here is whole-sequence equality, not membership. One string compare is
# inherently two-way -- a reorder, an addition, a removal, or a count-stable rename on EITHER
# side reddens.
#
# The doc side anchors on the `### Emit` heading and the first fenced block after it (ADR-0054:
# a quoted-clause anchor, never a line number), reducing each fence line to its first
# whitespace-delimited token so the `REPO_ROOT ... (plain format only -- see below)` annotation
# is stripped. An anchor that silently stops matching yields an EMPTY extraction, which would
# compare green against nothing -- so the control asserts pin the population first.
emit_fence_tokens(){  # first fenced block after the `### Emit` heading; first token per line
  awk '
    /^### Emit[[:space:]]*$/ { seen = 1; next }
    seen && /^```/           { if (infence) exit; infence = 1; next }
    infence && NF            { print $1 }
  ' "$REPO/scripts/docket-config.md"
}
doc_plain_keys="$(emit_fence_tokens)"
doc_shell_keys="$(grep -v '^REPO_ROOT$' <<<"$doc_plain_keys")"

assert "0258 L1 control: the \`### Emit\` fence extraction is non-empty" \
  '[ -n "$doc_plain_keys" ]'
assert "0258 L1 control: the extracted fence contains DOCKET_MODE" \
  'grep -qx DOCKET_MODE <<<"$doc_plain_keys"'
assert "0258 L1 control: the extracted fence contains BOOTSTRAP" \
  'grep -qx BOOTSTRAP <<<"$doc_plain_keys"'
assert "0258 L1 control: dropping REPO_ROOT shortened the shell sequence by exactly one" \
  '[ "$(grep -c . <<<"$doc_plain_keys")" -eq "$(( $(grep -c . <<<"$doc_shell_keys") + 1 ))" ]'

mkrepo "$tmp/l1"
mkdir -p "$tmp/l1.xdg/docket"
cat > "$tmp/l1/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/l1" add .docket.yml; git -C "$tmp/l1" commit --quiet -m cfg
git -C "$tmp/l1" push --quiet origin main
emit_plain_keys="$(rung "$tmp/l1.xdg" "$tmp/l1" --export --format plain | cut -d= -f1)"
emit_shell_keys="$(rung "$tmp/l1.xdg" "$tmp/l1" --export | cut -d= -f1)"

assert "0258 L1 control: the resolver emitted a non-empty key sequence" \
  '[ -n "$emit_plain_keys" ] && [ -n "$emit_shell_keys" ]'
assert "0258 L1: plain emission order equals the doc fence, in order and entry for entry" \
  '[ "$emit_plain_keys" = "$doc_plain_keys" ]'
assert "0258 L1: shell emission order equals the doc fence minus REPO_ROOT" \
  '[ "$emit_shell_keys" = "$doc_shell_keys" ]'

# The doc states the two counts in prose as well; derive them from the same extraction so
# growing the fence forces the numerals to move with it.
l1_plain_n="$(grep -c . <<<"$doc_plain_keys")"
l1_shell_n="$(grep -c . <<<"$doc_shell_keys")"
l1_sentence="$l1_shell_n lines in \`shell\` format; $l1_plain_n in \`plain\`"
assert "0258 L1: the doc's line-count prose tracks the fence ($l1_shell_n/$l1_plain_n)" \
  'grep -qF -- "$l1_sentence" "$REPO/scripts/docket-config.md"'

# ============================================================================
# Change 0258 leg 2 — rung-pair completeness, computed from the resolver
# ============================================================================
# Section (S4-S9) above pins the ordered rung pairs of the three-layer finalize.test_command
# chain. Until this guard the "six pairs" claim lived only in that section's header comment,
# so a FOURTH config layer would take the ordered-pair count from 6 to 12 and leave six
# masking cells silently unpinned with nothing to say so.
#
# The EXPECTED side is derived from `config_scalar_get`'s layer dispatch in
# scripts/docket-config.sh -- the single choke point every layer read funnels through, since
# `lcl`/`gbl` are one-line wrappers over it and the committed read calls it directly, so a
# fourth layer cannot land without adding an arm. The `*)` die arm is excluded by the
# lowercase-name shape of the match.
#
# The PINNED side is declared by the per-fixture marker lines added to s4-s9, collected across
# the `tests/test_docket_config*.sh` family glob -- never a ${BASH_SOURCE[0]} whole-file scan,
# so change 0251's split of this file cannot blind the collection.
#
# The verdict is SET equality: a gap, a duplicate, and an unknown layer name all redden, and
# count equality falls out of it (no hand-written "6", no ">= 6" floor).
#
# Accepted residual (spec, Design): a marker line could outlive deletion of its fixture body.
# The marker sits inside the fixture block so the natural edit removes both; a lying orphan is
# the same trust class as a lying assert label and is left to review.
rp_layers="$(sed -n '/^config_scalar_get()/,/^}/p' "$REPO/scripts/docket-config.sh" \
  | grep -E '^[[:space:]]*[a-z_]+\)[[:space:]]+config_scalar_from_lines' \
  | sed -E 's/^[[:space:]]*([a-z_]+)\).*/\1/' | LC_ALL=C sort)"
rp_n="$(grep -c . <<<"$rp_layers")"

assert "0258 L2 control: config_scalar_get dispatches at least three config layers (n=$rp_n)" \
  '[ "$rp_n" -ge 3 ]'
for rp_l in committed global local; do
  assert "0258 L2 control: layer $rp_l is dispatched by config_scalar_get" \
    'grep -qx "$rp_l" <<<"$rp_layers"'
done

# All ordered pairs over the derived layer set: n*(n-1) of them.
rp_expected="$(awk '{ a[NR] = $0 }
  END { for (i = 1; i <= NR; i++) for (j = 1; j <= NR; j++) if (i != j) print a[i] "->" a[j] }' \
  <<<"$rp_layers" | LC_ALL=C sort)"
rp_pinned="$(grep -hE '^# RUNG_PAIR: ' "$REPO"/tests/test_docket_config*.sh \
  | sed -E 's/^# RUNG_PAIR: //' | LC_ALL=C sort)"

assert "0258 L2 control: the family glob yielded a non-empty pinned pair population" \
  '[ -n "$rp_pinned" ]'
assert "0258 L2 control: $rp_n layers imply $(( rp_n * (rp_n - 1) )) ordered pairs" \
  '[ "$(grep -c . <<<"$rp_expected")" -eq "$(( rp_n * (rp_n - 1) ))" ]'
assert "0258 L2: the pinned rung pairs are exactly the resolver's ordered-pair set" \
  '[ "$rp_pinned" = "$rp_expected" ]'

# ---- change 0276: dummy_mode (DM-a … DM-l) ------------------------------------
# Persona-calibrated human-facing prose. Three leaves in one nested block, read leaf-by-leaf
# exactly like auto_capture:/learnings:, so a machine layer can flip `enabled` while inheriting
# `persona`. Every assert reads emitted lines (the GOB header states why).
#
# PLAIN format throughout (the 0127 ct_get precedent): two of the three values carry SPACES, and
# the default shell format is `%q`-quoted, so `DUMMY_MODE_SURFACES=dialogue pr` is emitted as
# `DUMMY_MODE_SURFACES=dialogue\ pr` there and an exact-line assert would never match. dm_run_with
# is run_resolver_with with that one difference — the shared helper is left alone rather than
# grown a parameter its existing callers do not need.
dm_run_with(){  # dm_run_with <yaml-fragment> -> resolver stdout in PLAIN format
  local frag="$1" d="$tmp/dm-frag-$RANDOM"
  mkrepo "$d"
  printf 'metadata_branch: main\n%b' "$frag" > "$d/.docket.yml"
  git -C "$d" add .docket.yml; git -C "$d" commit --quiet -m cfg
  git -C "$d" push --quiet origin main
  run "$d" --export --format plain
}

mkrepo "$tmp/dm-a"
dm_out_default="$(run "$tmp/dm-a" --export --format plain)"
assert "DM-a: enabled defaults to false" \
  'grep -qxF "DUMMY_MODE_ENABLED=false" <<<"$dm_out_default"'
assert "DM-a: surfaces defaults to the literal all" \
  'grep -qxF "DUMMY_MODE_SURFACES=all" <<<"$dm_out_default"'

# The default persona is emitted even when dummy mode is OFF: the spec's rule is that skills never
# special-case an empty persona. Bound the assert to a distinctive clause rather than the whole
# paragraph, so a re-wrap of the constant does not redden it.
assert "DM-b: the shipped default persona is exported when none is configured" \
  'grep -q "^DUMMY_MODE_PERSONA=.*mid-level software engineer" <<<"$dm_out_default"'
assert "DM-b: the default persona glosses project-internal vocabulary" \
  'grep -q "^DUMMY_MODE_PERSONA=.*one-clause explanation" <<<"$dm_out_default"'

mkrepo "$tmp/dm-c"
cat > "$tmp/dm-c/.docket.yml" <<'EOF'
metadata_branch: main
dummy_mode:
  enabled: true
  persona: "Reads YAML, not bash. Explain scripts by outcome."
EOF
git -C "$tmp/dm-c" add .docket.yml; git -C "$tmp/dm-c" commit --quiet -m cfg
git -C "$tmp/dm-c" push --quiet origin main
dm_out_committed="$(run "$tmp/dm-c" --export --format plain)"
assert "DM-c: committed layer is honored for enabled" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_out_committed"'
assert "DM-c: a quoted persona survives with its spaces and punctuation" \
  'grep -qxF "DUMMY_MODE_PERSONA=Reads YAML, not bash. Explain scripts by outcome." <<<"$dm_out_committed"'
assert "DM-c: an unset surfaces leaf still defaults to all" \
  'grep -qxF "DUMMY_MODE_SURFACES=all" <<<"$dm_out_committed"'

# Per-leaf fallback: the local layer flips one leaf and INHERITS the other two.
mkrepo "$tmp/dm-d"
cat > "$tmp/dm-d/.docket.yml" <<'EOF'
metadata_branch: main
dummy_mode:
  enabled: false
  persona: "Committed persona."
EOF
git -C "$tmp/dm-d" add .docket.yml; git -C "$tmp/dm-d" commit --quiet -m cfg
git -C "$tmp/dm-d" push --quiet origin main
printf 'dummy_mode:\n  enabled: true\n' > "$tmp/dm-d/.docket.local.yml"
dm_out_local="$(run "$tmp/dm-d" --export --format plain)"
assert "DM-d: repo-local outranks committed on the leaf it sets" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_out_local"'
assert "DM-d: the leaf the local layer did NOT set is inherited, not defaulted" \
  'grep -qxF "DUMMY_MODE_PERSONA=Committed persona." <<<"$dm_out_local"'

# Blank persona with dummy mode ON: default persona + a NOTICE on stderr. Never a warning, never
# disabled — the spec is explicit that a blank persona is a supported configuration.
dm_blank_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: \"\"\n" 2>&1 >/dev/null)"
dm_blank_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: \"\"\n" 2>/dev/null)"
assert "DM-e: a blank persona does not abort the resolver" \
  '[ -n "$dm_blank_out" ]'
assert "DM-e: a blank persona still resolves enabled: true" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_blank_out"'
assert "DM-e: a blank persona falls back to the default persona" \
  'grep -q "^DUMMY_MODE_PERSONA=.*mid-level software engineer" <<<"$dm_blank_out"'
# Bind the word "notice" to what it is a notice ABOUT, so a rewrite that keeps the word and drops
# the subject reddens (learnings: prose-guard-binds-phrase-to-claim).
assert "DM-e: the fallback prints a notice naming the persona" \
  'grep -qE "notice:[^.]{0,120}persona" <<<"$dm_blank_err"'
assert "DM-e: the fallback is not phrased as a warning" \
  '! grep -qE "warning:[^.]{0,120}dummy_mode.persona" <<<"$dm_blank_err"'

# A BLOCK SCALAR is a hard error. The reader is single-line, so `persona: >` would otherwise
# resolve to the literal ">" and export a one-character persona that looks configured.
dm_fold_rc=0
dm_fold_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: >\n    folded text here\n" 2>&1 >/dev/null)" || dm_fold_rc=$?
assert "DM-f: a folded block scalar aborts the resolver" '[ "$dm_fold_rc" -ne 0 ]'
assert "DM-f: the diagnostic names the key and the supported form" \
  'grep -qE "dummy_mode.persona[^.]{0,160}single-line" <<<"$dm_fold_err"'
dm_lit_rc=0
dm_lit_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: |-\n    literal text here\n" 2>&1 >/dev/null)" || dm_lit_rc=$?
assert "DM-f: a literal block scalar with a chomp indicator also aborts" '[ "$dm_lit_rc" -ne 0 ]'
assert "DM-f: the literal-form diagnostic also names the key" \
  'grep -qF -- "dummy_mode.persona" <<<"$dm_lit_err"'

# A `#` inside the persona is eaten by the shared reader BEFORE unquoting, leaving an unbalanced
# leading quote. Refuse it loudly rather than exporting the fragment.
dm_hash_rc=0
dm_hash_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: \"knows git # and yaml\"\n" 2>&1 >/dev/null)" || dm_hash_rc=$?
assert "DM-g: a persona containing '#' aborts instead of exporting a fragment" \
  '[ "$dm_hash_rc" -ne 0 ]'
assert "DM-g: the diagnostic names the offending character and the key" \
  'grep -qE "dummy_mode.persona[^.]{0,160}#" <<<"$dm_hash_err"'

# surfaces: an explicit subset is kept in order; an unknown token is warned-and-ignored, never fatal.
dm_sub_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  surfaces: [dialogue, pr]\n" 2>/dev/null)"
assert "DM-h: an explicit subset replaces the literal all" \
  'grep -qxF "DUMMY_MODE_SURFACES=dialogue pr" <<<"$dm_sub_out"'
dm_unk_rc=0
dm_unk_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  surfaces: [dialogue, bogus]\n" 2>&1 >/dev/null)" || dm_unk_rc=$?
dm_unk_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  surfaces: [dialogue, bogus]\n" 2>/dev/null)"
assert "DM-i: an unknown surface token does not abort the run" '[ "$dm_unk_rc" -eq 0 ]'
assert "DM-i: the unknown token is dropped and the known one kept" \
  'grep -qxF "DUMMY_MODE_SURFACES=dialogue" <<<"$dm_unk_out"'
# `[^:]`, not the `[^.]` the other DM asserts use: the warning names the dotted key
# (`dummy_mode.surfaces`) between "warning:" and the token, so a dot-excluding window can never
# span it. A colon-excluding window keeps the same "one diagnostic, not two" binding.
assert "DM-i: the unknown token is named in a warning" \
  'grep -qE "warning:[^:]{0,160}bogus" <<<"$dm_unk_err"'

# An empty list is legal and means "no eligible surface" — the spec's equivalent-to-disabled case.
dm_empty_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  surfaces: []\n" 2>/dev/null)"
assert "DM-j: an empty surfaces list is legal" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_empty_out"'
assert "DM-j: an empty surfaces list exports an empty value, not 'all'" \
  'grep -qxF "DUMMY_MODE_SURFACES=" <<<"$dm_empty_out"'

# enabled fails CLOSED on garbage, like learnings.enabled / auto_capture.enabled.
dm_bad_rc=0
dm_bad_err="$(dm_run_with "dummy_mode:\n  enabled: sometimes\n" 2>&1 >/dev/null)" || dm_bad_rc=$?
assert "DM-k: a non-boolean enabled aborts the resolver" '[ "$dm_bad_rc" -ne 0 ]'
assert "DM-k: the diagnostic names the key" \
  'grep -qF -- "dummy_mode.enabled" <<<"$dm_bad_err"'

# ORDER: the trio emits after SKILL_FINISH and before BOOTSTRAP. Line numbers are derived per key
# so a missing key reads as an empty extraction rather than shifting a positional match.
dm_ln(){ grep -n "^$1=" <<<"$dm_out_default" | cut -d: -f1; }
dm_n_fin="$(dm_ln SKILL_FINISH)"
dm_n_en="$(dm_ln DUMMY_MODE_ENABLED)"
dm_n_pe="$(dm_ln DUMMY_MODE_PERSONA)"
dm_n_su="$(dm_ln DUMMY_MODE_SURFACES)"
dm_n_boot="$(dm_ln BOOTSTRAP)"
assert "DM-l: all five emit positions were extracted (fin=$dm_n_fin en=$dm_n_en pe=$dm_n_pe su=$dm_n_su boot=$dm_n_boot)" \
  '[ -n "$dm_n_fin" ] && [ -n "$dm_n_en" ] && [ -n "$dm_n_pe" ] && [ -n "$dm_n_su" ] && [ -n "$dm_n_boot" ]'
assert "DM-l: the trio emits between SKILL_FINISH and BOOTSTRAP, in enabled/persona/surfaces order" \
  '[ "${dm_n_en:-0}" -gt "${dm_n_fin:-0}" ] && [ "${dm_n_pe:-0}" -gt "${dm_n_en:-0}" ] \
   && [ "${dm_n_su:-0}" -gt "${dm_n_pe:-0}" ] && [ "${dm_n_su:-0}" -lt "${dm_n_boot:-0}" ]'

assert "0174 template integrity: the shared template is unmutated after the full run" \
  '[ "$(git -C "$MKREPO_TEMPLATE.origin.git" for-each-ref --format="%(refname) %(objectname)" | LC_ALL=C sort)" = "$tplint_refs" ] &&
   [ "$(git -C "$MKREPO_TEMPLATE" rev-parse HEAD)" = "$tplint_head" ] &&
   [ "$(git -C "$MKREPO_TEMPLATE" rev-parse --abbrev-ref HEAD)" = "$tplint_branch" ]'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
