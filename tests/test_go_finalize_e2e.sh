#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_finalize_e2e.sh — the hermetic finalize end-to-end matrix
# (change 0316, Task 17).
#
# Runs the TestE2E* matrix in internal/app/finalize_e2e_test.go: each test builds
# the real ./cmd/docket binary and a protocol-faithful fake `gh`, then drives the
# whole terminal half of the workflow purely through CLI argv against disposable
# bare-remote repositories with hermetically isolated configuration.
#
# WHY THIS IS ITS OWN FILE and not folded into tests/test_go_toolchain.sh's plain
# `go test ./...`. The matrix compiles a binary and runs full finalize lifecycles
# against real Git, so even parallel its wall clock is far heavier than an
# ordinary unit package; folded into the Go gate it pushes that single row past
# the 60s hard ceiling in tests/test_runtime_budgets.sh (the RELIEF COUNTER that
# exists to catch slow work laundered into one budget). Sharding is the sanctioned
# answer: the matrix carries the `e2e` build tag, so the plain `go test ./...` and
# `go vet ./...` in tests/test_go_toolchain.sh EXCLUDE it (it neither runs twice
# nor inflates that row), and this file is the one place it runs — behind
# `-tags e2e`, in its own budgeted, parallel row. The build tag also lets the
# matrix use t.Parallel(): planningDepsFor's t.Setenv forbids it, so the tagged
# file isolates the in-process global-config layer once with os.Setenv instead.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing.
#
# The assert helper is the tree's canonical one byte for byte: rule (a) of
# the source-hygiene guard (internal/repoguard) is a byte-exact allowlist, and
# the suite runner accounts results on the `ok - ` / `NOT OK - ` markers
# it prints.
#
# CACHES. The suite runner gives every job a private HOME, so with
# GOMODCACHE/GOCACHE unset `go` finds neither a module cache nor a build cache
# and recompiles cold — network-dependent — on every suite run. This file pins
# both to `<git common dir>/docket-go-cache/{mod,build}` when the caller has not
# chosen its own, the same location and reasoning as tests/test_go_toolchain.sh
# (see the CACHES note in that file's header: outside every working tree, shared
# across worktrees, concurrent-safe, and `-modcacherw` required so an ordinary
# `rm -rf` can still remove it). Only the first run after a fresh clone needs the
# network.
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

# Pin the caches out of the job's throwaway HOME — see CACHES in this header.
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

e2e_file="$REPO/internal/app/finalize_e2e_test.go"

# The plain Go gate must NOT also run this matrix: prove the file carries the
# `e2e` build tag on its first line, so tests/test_go_toolchain.sh's default-tag
# `go test ./...`/`go vet ./...` exclude it and it runs here alone.
first_line="$(sed -n '1p' "$e2e_file")"
assert "the hermetic e2e file is behind the e2e build tag (excluded from the plain Go gate)" '[ "$first_line" = "//go:build e2e" ]'

# Vet the tagged file — the default-tag `go vet ./...` in the Go gate never sees
# it, so this is the only vet that covers the matrix and its harness.
vet_out="$(go vet -tags e2e ./internal/app/ 2>&1)"
vet_rc=$?
assert "go vet -tags e2e ./internal/app/ passes" '[ "$vet_rc" -eq 0 ] || { printf "%s\n" "$vet_out" >&2; false; }'

# The matrix itself. -count=1 defeats the test cache so a mutated tree is never
# served a stale green (learning cached-runner-serves-a-mutated-tree). -v so the
# per-test PASS markers can be counted, catching a vacuous `-run` filter that
# matched nothing.
test_out="$(go test -tags e2e -run "TestE2E" -count=1 -v ./internal/app/ 2>&1)"
test_rc=$?
assert "go test -tags e2e -run TestE2E ./internal/app/ passes" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'

# Completeness: the matrix must actually have RUN its top-level TestE2E tests,
# not passed vacuously because a rename left `-run TestE2E` matching nothing. The
# declared count is derived from the source; the run count from the -v markers.
declared="$(grep -c -- '^func TestE2E' "$e2e_file")"
ran="$(printf '%s\n' "$test_out" | grep -c -- '^--- PASS: TestE2E')"
assert "the e2e file declares its TestE2E matrix" '[ "$declared" -ge 8 ]'
assert "every declared top-level TestE2E test actually ran and passed (no vacuous filter)" '[ "$ran" -eq "$declared" ]'

exit "$fail"
