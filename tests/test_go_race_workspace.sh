#!/usr/bin/env bash
# tests/test_go_race_workspace.sh — the race shard for internal/workspace
# (change 0313).
#
# WHY THIS IS A SIBLING SHARD of tests/test_go_race.sh and not more assertions
# inside it. The whole-module race gate is sharded (change 0309's precedent): one
# file cannot carry `go test -race ./...` under the 60s hard ceiling in
# tests/test_runtime_budgets.sh. Change 0313's internal/workspace package brings a
# real-git, multi-clone workspace matrix — fresh/existing/resume/blocked prepare,
# proof-gated cleanup, lease-push publication, the CWD/symlink invocation matrix —
# whose fixtures re-exec instrumented git dozens of times. Folding them into the
# main shard measured that one file at 62s standalone serial, ABOVE the ceiling
# with no next raise available. The ceiling is a relief counter; raising it is the
# evasion it exists to catch. Sharding is the sanctioned answer, exactly as change
# 0309 cut tests/test_go_race_transaction.sh: the new heavy package moves into its
# own budgeted row, and tests/test_go_race.sh runs the complement. The three
# files' package sets partition `go list ./...` exactly, and that partition is
# asserted in tests/test_go_race.sh so a future package can never silently fall
# out of the race gate.
#
# WHY THE WORKSPACE PACKAGE is the one pulled out (rather than gitcli): it is the
# package this change introduces, so isolating it keeps the shared row from
# absorbing new growth and gives the new package its own headroom to grow before
# ITS next shard. gitcli remains on the main shard where change 0308 placed it;
# transaction has its own shard from change 0309.
#
# THIS FILE DOES NOT REPLACE THE PLAIN RUN. tests/test_go_toolchain.sh keeps its
# own `go test ./...` (uninstrumented); the instrumented build is a separate
# build-cache entry, so neither run makes the other free.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing. The race
# detector additionally requires cgo and a host C toolchain on a supported
# platform; where it is unavailable `go test -race` fails loudly rather than
# silently degrading to an uninstrumented run.
#
# The assert helper is the tree's canonical one byte for byte: rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist, and
# scripts/run-tests.sh accounts results on the `ok - ` / `NOT OK - ` markers
# it prints.
#
# CACHES. scripts/run-tests.sh gives every job a private HOME, so with
# GOMODCACHE/GOCACHE unset `go` finds neither a module cache nor a build cache
# and recompiles cold — network-dependent — on every suite run. This file pins
# both to `<git common dir>/docket-go-cache/{mod,build}` when the caller has not
# chosen its own, the same location and reasoning as tests/test_go_race.sh and
# tests/test_go_toolchain.sh (see the CACHES note in those headers). Only the
# first run after a fresh clone needs the network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the race gate cannot certify anything without a Go toolchain\n'
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

# The detector's verdict for exactly the workspace package. A race is reported on
# stderr and turns the exit non-zero, so the captured output is replayed on
# failure rather than summarized — the WARNING block names the two conflicting
# stacks and is the whole diagnostic. The package selector matches the same import
# path tests/test_go_race.sh EXCLUDES from its own run; the completeness guard
# there proves the three shards partition `go list ./...`.
race_out="$(go test -race ./internal/workspace/ 2>&1)"
race_rc=$?
assert "go test -race ./internal/workspace/ passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'

exit "$fail"
