#!/usr/bin/env bash
# tests/test_release_partition_fidelity.sh — the population floor of the 0362
# release partition. The structural contract (tests/test_go_integration_contract.sh)
# discovers packages from what EXISTS, so deleting internal/release's tagged files
# and runner together shrinks discovery instead of reddening it. This file pins the
# population against the committed map tests/fixtures/release-partition-map.tsv:
# every mapped name must be listed by `go test -list` in its declared corpus.
# FAIL-CLOSED: a listing error, an empty/malformed map, or a missing name is red.
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the CACHES
# note in that file's header): <git common dir>/docket-go-cache/{mod,build}.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the fidelity floor cannot certify anything without a Go toolchain\n'
  exit 1
fi
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
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

MAP="tests/fixtures/release-partition-map.tsv"
map_rows="$(grep -vE -e '^#|^$' "$MAP" 2>/dev/null)"
assert "the partition map exists and is non-empty" '[ -n "$map_rows" ]'
n_rows="$(grep -c -E -e '.' <<<"$map_rows")"
badshape="$(awk -F'\t' 'NF!=3 || $1!~/^Test/ || $2!~/^Test/ || ($3!="integration" && $3!="default"){print NR": "$0}' <<<"$map_rows")"
assert "every map row is <old>TAB<new>TAB<integration|default>" \
  '[ -z "$badshape" ] || { printf "%s\n" "$badshape" >&2; false; }'
dup_old="$(cut -f1 <<<"$map_rows" | LC_ALL=C sort | uniq -d)"
dup_new="$(cut -f2 <<<"$map_rows" | LC_ALL=C sort | uniq -d)"
assert "no old name is mapped twice and no new name is claimed twice" \
  '[ -z "$dup_old" ] && [ -z "$dup_new" ] || { echo "  dup old:$dup_old dup new:$dup_new" >&2; false; }'

# Live corpora — each listing's exit status asserted before its output is trusted.
dout="$(go test -list '^Test' ./internal/release/ 2>&1)"; drc=$?
assert "go test -list (default tags) succeeds for internal/release" \
  '[ "$drc" -eq 0 ] || { printf "%s\n" "$dout" >&2; false; }'
iout="$(go test -tags integration -list '^Test' ./internal/release/ 2>&1)"; irc=$?
assert "go test -tags integration -list succeeds for internal/release" \
  '[ "$irc" -eq 0 ] || { printf "%s\n" "$iout" >&2; false; }'
dlist="$(grep -E -e '^Test' <<<"$dout" | LC_ALL=C sort -u)"
ilist="$(grep -E -e '^Test' <<<"$iout" | LC_ALL=C sort -u)"

# The floor, both directions (learning correspondence-guard-runs-one-way):
# (a) every mapped name is alive in its declared corpus;
missing=""
while IFS=$'\t' read -r old new corpus; do
  [ -n "$new" ] || continue
  case "$corpus" in
    integration) grep -qxF -- "$new" <<<"$ilist" || missing="$missing $new(integration)";;
    default)     grep -qxF -- "$new" <<<"$dlist" || missing="$missing $new(default)";;
  esac
done <<<"$map_rows"
assert "every mapped release test is alive in its declared corpus (population floor)" \
  '[ -z "$missing" ] || { echo "  missing:$missing" >&2; false; }'
# (b) every live release test is in the map — an unmapped addition must be a
# conscious map edit, or the floor silently stops describing the population.
live_all="$(printf '%s\n%s\n' "$dlist" "$ilist" | grep -E -e '.' | LC_ALL=C sort -u)"
mapped_new="$(cut -f2 <<<"$map_rows" | LC_ALL=C sort -u)"
unmapped="$(comm -23 <(printf '%s\n' "$live_all") <(printf '%s\n' "$mapped_new") | tr '\n' ' ')"
assert "every live internal/release test appears in the partition map (reverse direction)" \
  '[ -z "${unmapped// /}" ] || { echo "  unmapped:$unmapped" >&2; false; }'

exit "$fail"
