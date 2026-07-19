#!/usr/bin/env bash
# tests/test_docket_yml_example.sh — run: bash tests/test_docket_yml_example.sh
# Guards .docket.yml.example, docket's canonical all-comprehensive config reference (change 0101).
# The example is PURE DOCUMENTATION — no docket tooling reads it — so these tests are the only
# thing keeping it honest. Replaces tests/test_config_example.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EX="$REPO/.docket.yml.example"
CFGSCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Hermetic: never read OR WRITE the dev machine's real global config. See the
# config-layer-write-and-read-hazards learning — this suite reaches ensure-global-config.sh.
export XDG_CONFIG_HOME="$tmp/xdg-void"

# fixture builder: a clone with a bare origin, one commit on main (origin/HEAD -> main).
# Mirrors tests/test_docket_config.sh's mkrepo.
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

assert ".docket.yml.example exists at repo root" '[ -f "$EX" ]'

# --- (1) FIDELITY: example == shipped defaults -------------------------------
# Copy the example in as .docket.yml on a fixture's default branch; the resolver's export must be
# BYTE-IDENTICAL to the same fixture with no config file at all. This proves (a) every active value
# equals the shipped default, (b) both `auto` sentinels resolve to the unset behavior, and (c) no
# active key in the example collides with the resolver's FLAT leaf-key reader.
mkrepo "$tmp/none"
mkrepo "$tmp/full"
cp "$EX" "$tmp/full/.docket.yml"
git -C "$tmp/full" add .docket.yml
git -C "$tmp/full" commit --quiet -m cfg
git -C "$tmp/full" push --quiet origin main

# Guard against the fidelity asserts passing VACUOUSLY: the fixture setup above (cp/add/commit/
# push) is unchecked and this suite runs with no -e, so if any one of those four commands silently
# failed, $tmp/full's origin/main would carry no .docket.yml, both sides would resolve as
# no-config, and the byte-identity assert below would go green while proving nothing. Prove the
# example actually reached the fixture's origin/main BEFORE trusting the comparison.
assert "fidelity fixture: example reached the fixture's origin/main" \
  'git -C "$tmp/full" show origin/main:.docket.yml 2>/dev/null | grep -q "^metadata_branch:"'

# --repo-dir differs between the two fixtures, and plain format emits absolute REPO_ROOT /
# METADATA_WORKTREE paths — normalize those two lines out before diffing.
norm(){ grep -vE '^(REPO_ROOT|METADATA_WORKTREE)=' ; }
exp_none="$(bash "$CFGSCRIPT" --repo-dir "$tmp/none" --export --format plain 2>/dev/null | norm)"
exp_full="$(bash "$CFGSCRIPT" --repo-dir "$tmp/full" --export --format plain 2>/dev/null | norm)"

assert "fidelity: export is non-empty (guard against both sides failing silently)" \
  '[ -n "$exp_none" ] && [ "$(printf "%s\n" "$exp_none" | wc -l)" -ge 15 ]'
assert "fidelity: example resolves byte-identically to no config at all" \
  '[ "$exp_none" = "$exp_full" ]'
if [ "$exp_none" != "$exp_full" ]; then
  echo "--- diff (no-config vs example-as-.docket.yml) ---"
  diff <(printf '%s\n' "$exp_none") <(printf '%s\n' "$exp_full") || true
  echo "---"
fi

exit $fail
