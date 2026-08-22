#!/usr/bin/env bash
# tests/test_docket_config.sh — hermetic fixtures for scripts/docket-config.sh (change 0026).
# Run: bash tests/test_docket_config.sh   (no network; temp repos + bare origins)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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
# implement-next no longer delegates a finish role skill: change 0315 replaced Step 7's
# SKILL_FINISH invocation with the direct `docket workspace publish` + `docket pr publish`
# operations (Claude still authors the PR title/body).
#
# Change 0316 retired the last holdout. The finish role does NOT survive in finalize's close-out
# any more, because the whole role is a DEFERRED capability under the Go runtime: `skills.finish`
# carries `dispDeferredActive` in internal/config/schema.go -- "any explicit value blocks" -- so a
# repo that sets it cannot mutate at all, and 0316's *Out of scope* names skill rebinding among the
# capabilities it defers. A skill that still invoked SKILL_FINISH would be instructing an agent to
# drive a path the binary refuses. So the assertion is inverted: finalize must NOT name it.
#
# The inverted form is guarded against vacuity -- an absent or empty SKILL.md would satisfy a bare
# `! grep` while proving nothing -- so the file's existence and non-emptiness are asserted first.
assert "finalize SKILL.md exists and is non-empty (non-vacuity anchor for the next assert)" \
  '[ -s "$REPO/skills/docket-finalize-change/SKILL.md" ]'
assert "finalize no longer invokes the deferred finish role (SKILL_FINISH)" \
  '! grep -qF "SKILL_FINISH" "$REPO/skills/docket-finalize-change/SKILL.md"'

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

assert "0174 template integrity: the shared template is unmutated after the full run" \
  '[ "$(git -C "$MKREPO_TEMPLATE.origin.git" for-each-ref --format="%(refname) %(objectname)" | LC_ALL=C sort)" = "$tplint_refs" ] &&
   [ "$(git -C "$MKREPO_TEMPLATE" rev-parse HEAD)" = "$tplint_head" ] &&
   [ "$(git -C "$MKREPO_TEMPLATE" rev-parse --abbrev-ref HEAD)" = "$tplint_branch" ]'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
