#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_finalize_native_e2e.sh — native completion rehearsal (0432).
#
# Native receipt and recovered completion rehearsal (0432). Kept separate from
# the existing finalize matrix because its 30s budget cannot absorb this path.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the e2e gate cannot certify anything without a Go toolchain\n'
  exit 1
fi

# Keep whatever GOFLAGS the caller set; append rather than replace.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

# Pin the caches out of the job's throwaway HOME — shared with the other Go suite shards.
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    # `--git-common-dir` answers relative to the working tree in a plain clone
    # and absolute from a linked worktree; normalize before building on it.
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

# Change 0373: under the suite runner, DOCKET_GO_TEST_CONCURRENCY bounds this
# child's share of the machine (go test package parallelism and runtime procs).
# Absent (solo run), Go's defaults apply unchanged.
go_conc_args=""
if [ -n "${DOCKET_GO_TEST_CONCURRENCY:-}" ]; then
  go_conc_args="-p ${DOCKET_GO_TEST_CONCURRENCY}"
  export GOMAXPROCS="${DOCKET_GO_TEST_CONCURRENCY}"
fi

test_out="$(go test -tags e2e $go_conc_args -run '^TestNativeCompletionE2E' -count=1 -v ./internal/app/ 2>&1)"
test_rc=$?
assert "native completion rehearsal passes" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'
declared="$(grep -c -- '^func TestNativeCompletionE2E' internal/app/native_completion_e2e_test.go)"
ran="$(printf '%s\n' "$test_out" | grep -c -- '^--- PASS: TestNativeCompletionE2E')"
assert "every native rehearsal test ran" '[ "$declared" -ge 1 ] && [ "$declared" -eq "$ran" ]'
exit "$fail"
