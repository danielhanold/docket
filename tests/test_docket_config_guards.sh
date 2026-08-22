#!/usr/bin/env bash
# tests/test_docket_config_guards.sh — tail shard of the test_docket_config family (change 0251).
# Split from tests/test_docket_config.sh at the change-0102 section boundary (a measured cut:
# ~35s head / ~30s tail serial). Carries the layer-resolution sections from change 0102 onward
# AND, as one unit, the (T) prelude-correspondence guard (change 0126) with its discovered-
# family-corpus population, the 0148 post-conditions, the 0223 gate/delegation observation-
# budget exports, the 0258 leg-1/leg-2 emit-fence and rung-pair completeness controls, and the
# change-0276 dummy_mode section. The prelude below replicates tests/test_docket_config.sh
# (family convention); the guard scans the whole "tests/test_docket_config*.sh" family, so both
# shards self-register with it.
# Run: bash tests/test_docket_config_guards.sh   (no network; temp repos + bare origins)
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

# Template integrity snapshot (change 0174). Both family shards take their own and re-assert it
# just before the final exit, so a fixture in this shard that dirties the shared template cannot
# go unnoticed. This shard does not run the independence block (head-only); the snapshot is taken
# against the freshly built, unmutated template.
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

run_resolver_with(){
  local frag="$1" d="$tmp/reclaim-garbage-$RANDOM"
  mkrepo "$d"
  printf 'metadata_branch: main\n%b' "$frag" > "$d/.docket.yml"
  git -C "$d" add .docket.yml; git -C "$d" commit --quiet -m cfg
  git -C "$d" push --quiet origin main
  run "$d" --export
}

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
# Population unit is the DISCOVERED `tests/test_docket_config*.sh` family corpus,
# never a single ${BASH_SOURCE[0]} scan (mirrors the "0258 leg 2" control's family
# glob below; ADR-0050, learning backstop-must-compute-not-reenumerate): a new
# shard self-registers with this guard exactly as it self-registers with the
# runner. Each file's own scan covers the WHOLE file minus its marker-delimited
# self-block (only the marker-carrying shard has one). Deliberately NOT truncated
# at an end-of-file marker: the file's tail is where new fixtures land, so
# truncation would make them permanently invisible. SITE lines carry the file
# basename (`SITE <basename>:<line>`) so a corpus of several shards stays legible.

prelude_report(){
  local file="$1" keys="$2" fbase="$3"
  awk -v keys="$keys" -v fbase="$fbase" '
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
        if (cnt == 0)        { exempt++; print "SITE " fbase ":" SL[k] " exempt" }
        else if (miss == "") { okc++;    print "SITE " fbase ":" SL[k] " ok" }
        else                 { viol++;   print "SITE " fbase ":" SL[k] " viol" miss }
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

# The population is the DISCOVERED family corpus, never an enumerated list
# (ADR-0050; learning backstop-must-compute-not-reenumerate): a new shard
# self-registers with this guard exactly as it self-registers with the runner.
# Mirrors the "0258 leg 2" control's `"$REPO"/tests/test_docket_config*.sh` glob
# below — same spelling, LC_ALL=C sorted.
t_corpus=()
while IFS= read -r tc_f; do t_corpus+=("$tc_f"); done \
  < <(printf '%s\n' "$REPO"/tests/test_docket_config*.sh | LC_ALL=C sort)
assert "0126 T: the family glob resolved to real files" '[ -e "${t_corpus[0]}" ]'
# Post-split population floor (change 0251; learning marker-scoped-guard-needs-a-population-floor):
# the family is now at least a head/tail pair, so a rename that quietly collapsed the corpus back to
# one file — losing the other shard's sites from the summed floors — is itself a red suite.
assert "0126 T: the family corpus spans at least two files" '[ "${#t_corpus[@]}" -ge 2 ]'

# Aggregate the site report and both extractors ACROSS the corpus. Every count is
# summed from per-file runs; the self-block subtraction (t_selflit/t_selfrefs)
# applies only to the one shard that carries the guard's own markers.
t_out=""
t_sites=0; t_exempt=0; t_ok=0; t_viol=0
t_raw=0; t_helper=0; t_comments=0
t_selfcount=0; t_selflit=0; t_selfrefs=0
for tc_f in "${t_corpus[@]}"; do
  tc_base="$(basename "$tc_f")"
  tc_out="$(prelude_report "$tc_f" "$t_keys" "$tc_base")"
  t_out="$t_out$tc_out"$'\n'

  # per-file TOTALS, summed into the whole-corpus figures.
  tc_sites="$(printf '%s\n' "$tc_out" | sed -n 's/^TOTALS sites=\([0-9]*\) .*/\1/p')"
  tc_exempt="$(printf '%s\n' "$tc_out" | sed -n 's/^TOTALS .* exempt=\([0-9]*\) .*/\1/p')"
  tc_ok="$(printf '%s\n' "$tc_out" | sed -n 's/^TOTALS .* ok=\([0-9]*\) .*/\1/p')"
  tc_viol="$(printf '%s\n' "$tc_out" | sed -n 's/^TOTALS .* viol=\([0-9]*\)$/\1/p')"
  t_sites=$(( t_sites + tc_sites )); t_exempt=$(( t_exempt + tc_exempt ))
  t_ok=$(( t_ok + tc_ok )); t_viol=$(( t_viol + tc_viol ))

  # Structurally-different extractor, summed per file: a plain grep of the raw
  # literal, minus the known non-sites — the one canonical assert() helper per
  # family file (whose eval takes a positional rather than a cmdsub var) and
  # every COMMENT line that merely mentions the literal in prose (a real site is
  # never a comment line — this mirrors the site-discovery awk's own
  # `if (s ~ /^#/) continue`). Each count is derived at runtime, not hand-counted,
  # so file drift cannot silently desync the two extractors.
  tc_raw="$(/usr/bin/grep -cF "$T_EVAL_LITERAL" "$tc_f")"
  tc_helper="$(/usr/bin/grep -cE '^assert\(\)\{' "$tc_f")"
  tc_comments="$(/usr/bin/grep -E '^[[:space:]]*#' "$tc_f" | /usr/bin/grep -cF "$T_EVAL_LITERAL")"
  t_raw=$(( t_raw + tc_raw )); t_helper=$(( t_helper + tc_helper ))
  t_comments=$(( t_comments + tc_comments ))

  # Self-block subtraction is scoped to the marker-carrying shard: only it holds a
  # quoted T_EVAL_LITERAL copy (counted by t_raw but not a site) and the bounded
  # self-block the site scan excludes.
  tc_hasself="$(/usr/bin/grep -cF -- "$T_SELF_START" "$tc_f")"
  if [ "$tc_hasself" -gt 0 ]; then
    t_selfcount=$(( t_selfcount + 1 ))
    t_selflit="$(/usr/bin/grep -cE '^T_EVAL_LITERAL=' "$tc_f")"
    t_selfrefs="$(awk -v s="$T_SELF_START" -v e="$T_SELF_END" '
      index($0,s)>0 && !st {st=NR} index($0,e)>0 && st && !en {en=NR}
      END{ if (st && en) print en-st+1; else print 0 }' "$tc_f")"
  fi
done
# t_exempt is diagnostic-only, on purpose: since change 0149 replaced the absolute exempt ceiling
# with the proportional `ok` floor below, no assert reads this variable and nothing prints it.
# Kept summed (not deleted) for a reader diffing the TOTALS lines by eye — an unread variable
# here is deliberate, not an oversight.
: "$t_exempt"
# Print every per-file TOTALS line AND every violating site. Printing totals alone
# leaves the next author staring at `viol=1` with no file, line, or variable name.
printf '%s\n' "$t_out" | /usr/bin/grep -E '^(TOTALS|SITE .* viol)'

# Exactly one corpus file carries the guard's own markers: the self-block
# subtraction above (t_selflit/t_selfrefs) is only sound if precisely one shard
# holds T_SELF_START. Two would double-count; zero would silently disable it.
assert "0126 T: exactly one corpus file carries the guard self-block markers" \
  '[ "$t_selfcount" -eq 1 ]'

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
# bucket and t_exempt legitimately stays 3. The site is derived, not hand-counted:
# it is the last `eval "$out"` line for the r9 fixture seen before the assert that
# immediately follows it, keyed to `<basename>:<line>` of whichever corpus shard
# holds the fixture — so file drift above it, or the r9 fixture moving to another
# shard, cannot silently desync the two.
r9_poison_site=""
for tc_f in "${t_corpus[@]}"; do
  tc_r9line="$(awk '
    /out="\$\(rung "\$tmp\/r9\.xdg" "\$tmp\/r9" --export\)"; eval "\$out"/ { last = NR }
    /0102 R9: repo-local false beats global true/ { print last; exit }
  ' "$tc_f")"
  if [ -n "$tc_r9line" ]; then r9_poison_site="$(basename "$tc_f"):$tc_r9line"; break; fi
done
assert "0148: the require_pr_approval site still has a non-empty need set (not exempt)" \
  '/usr/bin/grep -qE "^SITE $r9_poison_site (ok|viol)" <<<"$t_out"'

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

assert '0258 L1 control: the `### Emit` fence extraction is non-empty' \
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
# The anchor carries literal backticks, so its backticked spans are built from single-quoted
# pieces and concatenated onto the expansions — no backtick may sit inside double quotes (0221).
l1_sentence="$l1_shell_n"' lines in `shell` format; '"$l1_plain_n"' in `plain`'
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

# A `#` INSIDE the persona value is eaten by the shared reader BEFORE unquoting, so the value would
# be exported truncated. Refuse it loudly rather than exporting the fragment — and key the refusal
# on the RAW leaf text, never on the residue truncation happens to leave behind, which is a
# property of the QUOTED shape only and misfires on a legal persona (the DM-g pair below).
dm_hash_rc=0
dm_hash_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: \"knows git # and yaml\"\n" 2>&1 >/dev/null)" || dm_hash_rc=$?
assert "DM-g: a quoted persona containing '#' aborts instead of exporting a fragment" \
  '[ "$dm_hash_rc" -ne 0 ]'
assert "DM-g: the diagnostic names the offending character and the key" \
  'grep -qE "dummy_mode.persona[^.]{0,160}#" <<<"$dm_hash_err"'

# The UNQUOTED shape never had a quote to unbalance, so a residue-keyed guard misses it entirely
# and exports `knows git` with no abort and no notice. Same truncation, so the same refusal.
dm_bare_rc=0
dm_bare_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: knows git # and yaml\n" 2>&1 >/dev/null)" || dm_bare_rc=$?
dm_bare_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: knows git # and yaml\n" 2>/dev/null || true)"
assert "DM-g: an UNQUOTED persona containing '#' aborts too" '[ "$dm_bare_rc" -ne 0 ]'
assert "DM-g: the unquoted diagnostic names the offending character and the key" \
  'grep -qE "dummy_mode.persona[^.]{0,160}#" <<<"$dm_bare_err"'
# Anti-vacuity for the abort: pin that the FRAGMENT specifically is what never reaches the export,
# so a future guard that aborts for some unrelated reason still has to keep this true.
assert "DM-g: the truncated fragment is never exported" \
  '! grep -qxF "DUMMY_MODE_PERSONA=knows git" <<<"$dm_bare_out"'

# Legal persona, must NOT abort: the text itself BEGINS with a quote character, which is exactly
# the residue a truncation-keyed guard looks for — no `#` appears anywhere in the config.
mkrepo "$tmp/dm-lq"
cat > "$tmp/dm-lq/.docket.yml" <<'EOF'
metadata_branch: main
dummy_mode:
  enabled: true
  persona: '"explain it like a PM" reader'
EOF
git -C "$tmp/dm-lq" add .docket.yml; git -C "$tmp/dm-lq" commit --quiet -m cfg
git -C "$tmp/dm-lq" push --quiet origin main
dm_lq_rc=0
dm_lq_out="$(run "$tmp/dm-lq" --export --format plain 2>/dev/null)" || dm_lq_rc=$?
dm_lq_err="$(run "$tmp/dm-lq" --export --format plain 2>&1 >/dev/null || true)"
assert "DM-g: a persona whose text begins with a quote character does not abort" \
  '[ "$dm_lq_rc" -eq 0 ]'
assert "DM-g: that persona is exported verbatim, inner quotes and all" \
  'grep -qxF "DUMMY_MODE_PERSONA=\"explain it like a PM\" reader" <<<"$dm_lq_out"'
assert "DM-g: no diagnostic names a '#' that appears nowhere in the config" \
  '! grep -qF -- "#" <<<"$dm_lq_err"'

# Legal persona, must NOT abort: a `#` AFTER the closing quote is an ordinary YAML trailing comment,
# which the shared reader strips correctly. Refusing it would be the same false abort in a new coat.
dm_tc_rc=0
dm_tc_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: \"Reads YAML, not bash.\"   # tone knob\n" 2>/dev/null)" || dm_tc_rc=$?
assert "DM-g: a trailing comment after the closing quote does not abort" '[ "$dm_tc_rc" -eq 0 ]'
assert "DM-g: the commented line still exports the full persona" \
  'grep -qxF "DUMMY_MODE_PERSONA=Reads YAML, not bash." <<<"$dm_tc_out"'

# The refusal reads the RAW text of the layer that WINS, so it must reach every rung — and only
# that rung. A broken persona in a layer a higher one already overrides changes nothing that gets
# exported, so refusing on it would let a stale global/committed file brick a repo that fixed it.
mkrepo "$tmp/dm-lay"
cat > "$tmp/dm-lay/.docket.yml" <<'EOF'
metadata_branch: main
dummy_mode:
  enabled: true
  persona: "committed # broken"
EOF
git -C "$tmp/dm-lay" add .docket.yml; git -C "$tmp/dm-lay" commit --quiet -m cfg
git -C "$tmp/dm-lay" push --quiet origin main
printf 'dummy_mode:\n  persona: "local # broken"\n' > "$tmp/dm-lay/.docket.local.yml"
dm_lay_rc=0
dm_lay_err="$(run "$tmp/dm-lay" --export --format plain 2>&1 >/dev/null)" || dm_lay_rc=$?
assert "DM-g: a '#'-bearing persona in the repo-LOCAL layer aborts too" '[ "$dm_lay_rc" -ne 0 ]'
assert "DM-g: the local-layer diagnostic names the key" \
  'grep -qF -- "dummy_mode.persona" <<<"$dm_lay_err"'
printf 'dummy_mode:\n  persona: "local override, intact"\n' > "$tmp/dm-lay/.docket.local.yml"
dm_shadow_rc=0
dm_shadow_out="$(run "$tmp/dm-lay" --export --format plain 2>/dev/null)" || dm_shadow_rc=$?
assert "DM-g: a broken persona a higher layer already overrides does not abort" \
  '[ "$dm_shadow_rc" -eq 0 ]'
assert "DM-g: the overriding layer's persona is what gets exported" \
  'grep -qxF "DUMMY_MODE_PERSONA=local override, intact" <<<"$dm_shadow_out"'

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

# `all` inside a LIST must never fall through the unknown-token path: `all` is not an admitted
# surface token, so warn-and-drop would resolve `[all]` to the empty string — "no surfaces", the
# exact inverse of what was asked. Aborts loudly, on the auto_capture.types precedent.
dm_lall_rc=0
dm_lall_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  surfaces: [all]\n" 2>&1 >/dev/null)" || dm_lall_rc=$?
dm_lall_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  surfaces: [all]\n" 2>/dev/null || true)"
assert "DM-h: 'all' inside a list aborts instead of resolving to no surfaces" \
  '[ "$dm_lall_rc" -ne 0 ]'
assert "DM-h: the diagnostic names the bare-scalar spelling" \
  'grep -qF -- "dummy_mode.surfaces" <<<"$dm_lall_err" && grep -qF -- "surfaces: all" <<<"$dm_lall_err"'
assert "DM-h: the inverted empty resolution never reaches the export" \
  '! grep -qxF "DUMMY_MODE_SURFACES=" <<<"$dm_lall_out"'

# The block-scalar refusal is keyed on SHAPE, not on an enumeration: YAML allows the chomp and
# indent indicators in EITHER order, so `>-2` and `|+4` must be refused exactly like `>2-`/`|-`.
dm_rev_rc=0
dm_rev_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: >-2\n   folded reversed\n" 2>&1 >/dev/null)" || dm_rev_rc=$?
assert "DM-f: a folded block scalar with reversed indicator order aborts" '[ "$dm_rev_rc" -ne 0 ]'
assert "DM-f: the reversed-order diagnostic names the key" \
  'grep -qF -- "dummy_mode.persona" <<<"$dm_rev_err"'
dm_rev2_rc=0
dm_rev2_out="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: |+4\n    literal reversed\n" 2>/dev/null || true)"
dm_rev2_err="$(dm_run_with "dummy_mode:\n  enabled: true\n  persona: |+4\n    literal reversed\n" 2>&1 >/dev/null)" || dm_rev2_rc=$?
assert "DM-f: a literal block scalar with reversed indicator order aborts too" '[ "$dm_rev2_rc" -ne 0 ]'
assert "DM-f: the two-character persona is never exported" \
  '! grep -qxF "DUMMY_MODE_PERSONA=|+4" <<<"$dm_rev2_out"'

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
