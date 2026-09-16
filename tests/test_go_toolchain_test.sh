#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_toolchain_test.sh — default host tests for every package outside
# internal/app. Two derived siblings partition internal/app; this parent validates
# their complete declarations before excluding the package from its live census.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the host test gate cannot certify anything without a Go toolchain\n'
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

go_conc_args=""
if [ -n "${DOCKET_GO_TEST_CONCURRENCY:-}" ]; then
  go_conc_args="-p ${DOCKET_GO_TEST_CONCURRENCY}"
  export GOMAXPROCS="${DOCKET_GO_TEST_CONCURRENCY}"
fi

. "$REPO/tests/lib/go-app-shard.sh"
APP_SHARD_GLOB='test_go_toolchain_app_*.sh'
APP_SHARD_FAMILY=toolchain
APP_SHARD_FLAG=''
validate_app_shards

module="$(go list -m 2>/dev/null)"
assert "go list -m resolves the module path" '[ -n "$module" ]'
# Keep cold-cache download diagnostics on stderr and out of the package argv.
package_out="$(go list ./...)"; package_rc=$?
assert "go list ./... derives the host test package census" '[ "$package_rc" -eq 0 ]'
app_package="$module/internal/app"
app_package_hits="$(grep -cxF -- "$app_package" <<<"$package_out")"
assert "the host test package census contains internal/app exactly once" '[ "$app_package_hits" -eq 1 ]'
rest_packages="$(grep -vxF -- "$app_package" <<<"$package_out")"
assert "the non-app host test package census is non-empty" '[ -n "$rest_packages" ]'

test_out="$(go test $go_conc_args -count=1 $rest_packages 2>&1)"; test_rc=$?
assert "go test -count=1 passes for every non-app module package" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'
exit "$fail"
