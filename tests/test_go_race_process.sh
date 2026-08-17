#!/usr/bin/env bash
# tests/test_go_race_process.sh — the race shard for internal/process
# (change 0314).
#
# WHY THIS IS A SIBLING SHARD of tests/test_go_race.sh and not a fifth check
# inside it. The whole-module race gate (`go test -race ./...`) is already at
# the 60s hard ceiling in tests/test_runtime_budgets.sh (its row IS the ceiling,
# with no next raise). Change 0314's internal/process package is the heaviest
# real-process suite in the tree under the detector: its tests re-exec the
# instrumented test binary as a per-run Setsid supervisor and wait on real
# children through launch/observe/stop/recover, so every case pays the detector's
# several-times-slower instrumented build AND real process/group syscalls. Folded
# into the main race row it breaches the ceiling — and that ceiling is a relief
# counter aimed at exactly this move, slow work laundered into one row's budget.
# Sharding is the sanctioned answer, the same remedy changes 0309 and 0313 used
# for the transaction and workspace packages and change 0324 used for the runner
# suite: the heavy package moves into its own budgeted row, tests/test_go_race.sh
# runs the DERIVED complement, and the four shards' package sets partition
# `go list ./...` exactly — a partition asserted in tests/test_go_race.sh so a
# future package can never silently fall out of the race gate.
#
# WHY THE PROCESS PACKAGE is the one pulled out: it is the package this change
# introduces, so isolating it keeps the shared row from absorbing new growth and
# gives the new package its own headroom to grow before ITS next shard.
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

# The detector's verdict for exactly the process package. A race is reported on
# stderr and turns the exit non-zero, so the captured output is replayed on
# failure rather than summarized — the WARNING block names the two conflicting
# stacks and is the whole diagnostic. The package selector matches the same import
# path tests/test_go_race.sh EXCLUDES from its own run; the completeness guard
# there proves the four shards partition `go list ./...`.
race_out="$(go test -race ./internal/process/ 2>&1)"
race_rc=$?
assert "go test -race ./internal/process/ passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'

exit "$fail"
