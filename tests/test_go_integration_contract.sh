#!/usr/bin/env bash
# tests/test_go_integration_contract.sh — the fail-closed completeness contract over
# the Go integration partition (change 0333). Discovery is live: tagged tests come
# from `go test -tags integration -list` and shard membership from each runner's
# own inspection mode (DOCKET_SHARD_INSPECT=1) — never a second registry. Checks:
#   (1) every *_integration_test.go / *_race_integration_test.go in the three
#       packages opens with `//go:build integration` on line 1
#   (2) the tagged corpus lists cleanly per package, is non-empty overall, and
#       every tagged-only test carries a TestIntegration/TestRaceIntegration prefix
#   (3) every discovered runner inspects to a well-formed declaration
#   (4) every tagged test matches EXACTLY ONE runner (same package, name-prefix)
#   (5) TestRaceIntegration… tests match a race runner and TestIntegration… tests
#       never do — both directions (learning correspondence-guard-runs-one-way)
#   (6) no integration-prefixed test is visible to the default-tag corpus
#   (7) every runner selects at least one test (a stale runner cannot no-op)
#   (8) go vet -tags integration passes for all three packages
#
# FAIL-CLOSED. A probe error is never read as clean absence: every `go test -list`
# and `go vet` invocation's exit status is asserted before its output is trusted,
# and an empty tagged corpus, an empty runner set, or an empty per-runner selection
# is a red assert, not a silent pass.
#
# The assert helper is the tree's canonical one byte for byte (rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist).
#
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the CACHES
# note in that file's header): <git common dir>/docket-go-cache/{mod,build}, shared
# across worktrees, concurrent-safe, -modcacherw required.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the contract cannot certify anything without a Go toolchain\n'
  exit 1
fi

# Keep whatever GOFLAGS the caller set; append rather than replace.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

# Pin the caches out of the job's throwaway HOME — see CACHES in this header.
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

TAB=$'\t'
NL=$'\n'
PKGS="internal/app internal/githubcli internal/gitcli"

# (1) build-constraint placement — first line, exactly.
bad_tag=""
for pkg in $PKGS; do
  for f in "$pkg"/*_integration_test.go; do
    [ -e "$f" ] || continue
    [ "$(sed -n '1p' "$f")" = "//go:build integration" ] || bad_tag="$bad_tag $f"
  done
done
assert "every *_integration_test.go opens with //go:build integration" \
  '[ -z "$bad_tag" ] || { echo "  missing/misplaced tag:$bad_tag" >&2; false; }'

# (2) tagged corpus, per package. A listing or compile failure is fatal (fail-closed);
# an empty corpus is fatal; a tagged-only test with neither structural prefix is fatal.
# tagged-only = (visible under -tags integration) MINUS (visible with default tags) —
# a set subtraction that isolates the tests the tag adds, cheaply (two -list per pkg).
tagged=""            # lines: <pkg><TAB><TestName> for each Test(Race)?Integration tagged-only test
stray=""
for pkg in $PKGS; do
  iout="$(go test -tags integration -list '^Test' "./$pkg" 2>&1)"; irc=$?
  assert "go test -tags integration -list succeeds for $pkg" \
    '[ "$irc" -eq 0 ] || { printf "%s\n" "$iout" >&2; false; }'
  dout="$(go test -list '^Test' "./$pkg" 2>&1)"; drc=$?
  assert "go test -list (default tags) succeeds for $pkg" \
    '[ "$drc" -eq 0 ] || { printf "%s\n" "$dout" >&2; false; }'
  ilist="$(grep -E -e '^Test' <<<"$iout" | LC_ALL=C sort -u)"
  dlist="$(grep -E -e '^Test' <<<"$dout" | LC_ALL=C sort -u)"
  taggedonly="$(comm -23 <(printf '%s\n' "$ilist") <(printf '%s\n' "$dlist"))"
  prefixed="$(grep -E -e '^Test(Race)?Integration' <<<"$taggedonly" || true)"
  while read -r t; do
    [ -n "$t" ] && tagged="${tagged}${pkg}${TAB}${t}${NL}"
  done <<<"$prefixed"
  unprefixed="$(grep -E -e '^Test' <<<"$taggedonly" | grep -vE -e '^Test(Race)?Integration' || true)"
  [ -n "$unprefixed" ] && stray="$stray $pkg:{$(printf '%s' "$unprefixed" | tr '\n' ',')}"
done
assert "the tagged corpus is non-empty" '[ -n "$tagged" ]'
assert "every tagged-only test carries a TestIntegration/TestRaceIntegration prefix" \
  '[ -z "$stray" ] || { echo "  unprefixed tagged tests:$stray" >&2; false; }'

# (3) runner discovery + inspection. The contract excludes itself by exact name.
runners="$(find tests -maxdepth 1 -name 'test_go_integration_*.sh' ! -name 'test_go_integration_contract.sh' | LC_ALL=C sort)"
assert "at least one shard runner exists" '[ -n "$runners" ]'
decl=""              # lines: <runner><TAB><pkg-dir-no-dot-slash><TAB><prefix><TAB><mode>
bad_decl=""
while read -r r; do
  [ -n "$r" ] || continue
  out="$(DOCKET_SHARD_INSPECT=1 bash "$r" 2>&1)"; rc=$?
  p="$(sed -n 's/^package=//p' <<<"$out")"; x="$(sed -n 's/^prefix=//p' <<<"$out")"; m="$(sed -n 's/^mode=//p' <<<"$out")"
  ok=1
  [ "$rc" -eq 0 ] || ok=0
  case "$p" in ./internal/app|./internal/githubcli|./internal/gitcli) ;; *) ok=0 ;; esac
  case "$m" in normal|race) ;; *) ok=0 ;; esac
  case "$x" in TestIntegration?*|TestRaceIntegration?*) ;; *) ok=0 ;; esac
  if [ "$ok" = 1 ]; then
    decl="${decl}${r}${TAB}${p#./}${TAB}${x}${TAB}${m}${NL}"
  else
    bad_decl="$bad_decl $r"
  fi
done <<<"$runners"
assert "every runner inspects to a well-formed declaration" \
  '[ -z "$bad_decl" ] || { echo "  malformed:$bad_decl" >&2; false; }'

# (4)+(5): one matching pass over tagged × declarations.
unmatched=""; multi=""; wrongmode=""
while IFS="$TAB" read -r pkg t; do
  [ -n "$t" ] || continue
  hits=0; hitmode=""
  while IFS="$TAB" read -r r rp rx rm; do
    [ -n "$r" ] || continue
    [ "$rp" = "$pkg" ] || continue
    case "$t" in "$rx"*) hits=$((hits+1)); hitmode="$rm";; esac
  done <<<"$decl"
  [ "$hits" -eq 0 ] && unmatched="$unmatched $pkg:$t"
  [ "$hits" -gt 1 ] && multi="$multi $pkg:$t"
  case "$t" in
    TestRaceIntegration*) [ "$hits" -eq 1 ] && [ "$hitmode" != "race" ]   && wrongmode="$wrongmode $pkg:$t(want race)";;
    TestIntegration*)     [ "$hits" -eq 1 ] && [ "$hitmode" != "normal" ] && wrongmode="$wrongmode $pkg:$t(want normal)";;
  esac
done <<<"$tagged"
assert "every tagged test matches exactly one runner (none unmatched)" \
  '[ -z "$unmatched" ] || { echo " $unmatched" >&2; false; }'
assert "every tagged test matches exactly one runner (none doubled)" \
  '[ -z "$multi" ] || { echo " $multi" >&2; false; }'
assert "race-prefixed tests run in race shards, and only they do" \
  '[ -z "$wrongmode" ] || { echo " $wrongmode" >&2; false; }'

# (7) reverse direction: every runner selects at least one tagged test.
empty_runners=""
while IFS="$TAB" read -r r rp rx rm; do
  [ -n "$r" ] || continue
  n="$(grep -c -E -e "^${rp}${TAB}${rx}" <<<"$tagged")"
  [ "$n" -ge 1 ] || empty_runners="$empty_runners $r"
done <<<"$decl"
assert "no runner is a stale empty no-op" \
  '[ -z "$empty_runners" ] || { echo " $empty_runners" >&2; false; }'

# (6) the default corpus must not see any integration-prefixed test.
leak=""
for pkg in $PKGS; do
  out="$(go test -list '^Test(Race)?Integration' "./$pkg" 2>&1)"; rc=$?
  assert "go test -list (default tags) succeeds for $pkg (leak probe)" \
    '[ "$rc" -eq 0 ] || { printf "%s\n" "$out" >&2; false; }'
  l="$(grep -E -e '^Test(Race)?Integration' <<<"$out" || true)"
  [ -z "$l" ] || leak="$leak $pkg:{$(printf '%s' "$l" | tr '\n' ',')}"
done
assert "no integration-prefixed test is visible to the default-tag corpus" \
  '[ -z "$leak" ] || { echo " $leak" >&2; false; }'

# (8) tagged static analysis — default `go vet ./...` cannot see this corpus.
vet_out="$(go vet -tags integration ./internal/app/ ./internal/githubcli/ ./internal/gitcli/ 2>&1)"
vet_rc=$?
assert "go vet -tags integration passes for all three packages" \
  '[ "$vet_rc" -eq 0 ] || { printf "%s\n" "$vet_out" >&2; false; }'

exit "$fail"
