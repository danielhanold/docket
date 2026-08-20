#!/usr/bin/env bash
# tests/test_go_race.sh — the whole-module data-race gate (change 0308), collapsed
# back to a single `go test -race ./...` run by change 0332.
#
# HISTORY. Changes 0309, 0313, and 0314 sharded this gate four ways so each piece
# could fit under the parallel phase's hard 60s budget ceiling. Change 0332
# measured the shards and collapsed them: the shards existed to fit the PARALLEL
# phase, and the parallel phase is exactly what 0332 removed this gate from. In
# the serial lane the four shard invocations ran sequentially and summed to
# ~299s, while a single `go test -race ./...` invocation is ~206s because go test
# overlaps packages internally — so once serialized, the shard structure was not
# just unnecessary scaffolding but slower than not sharding. One `./...` run
# covers the module by construction: nothing to partition, no completeness guard
# to maintain.
#
# LANE AND CEILING. tests/runtime-budgets.tsv pins this file `serial` with a
# 300s row — the table's one documented exemption to the hard 60s ceiling (see
# the exemption note at RELIEF COUNTER A in tests/test_runtime_budgets.sh). The
# serial pin is the point of change 0332: `go test -race` spawns GOMAXPROCS-wide
# race workers, and inside the parallel `-j` fan-out those workers oversubscribe
# the cores the shell test jobs need, inflating every OTHER file's wall clock —
# the load-dependent gate that halted change 0329. Run alone in the serial phase
# this gate uses the whole machine, which is what an isolated gate should do, so
# its internal parallelism is deliberately NOT capped (no GOMAXPROCS/-p pin).
# The ~206s is dominated by internal/app's ~190s integration suite — a cost the
# race detector barely moves (~1.05x multiplier) and that no lane or `go list`
# shard can split, because internal/app is one Go package. The durable fix, a
# test-level partition of internal/app, is owned by follow-up change 0333; when
# it lands, this gate's row and the exemption shrink with it.
#
# WHY THIS IS ITS OWN FILE and not a fifth check inside
# tests/test_go_toolchain.sh. The detector is expensive — instrumented binaries
# run several times slower — and this file's 300s exemption is deliberately
# scoped to the race gate alone. Folding the detector into the Go gate would
# drag that file's row through the same exemption and blur the two verdicts.
#
# WHY REPO-WIDE and not an enumerated package list. The adapter surfaces held
# concurrently by design today are known — but an enumerated list gates only the
# packages someone remembered to enumerate, and the package that grows a race is
# by definition the one nobody thought of. `./...` is the shape-keyed spelling
# and needs no maintenance as packages are added.
#
# THIS FILE DOES NOT REPLACE THE PLAIN RUN. tests/test_go_toolchain.sh keeps its
# own `go test ./...`, which is also the single owner of the four-tuple CGO-off
# cross-build (TestCrossCompileApprovedTargets). The two runs answer different
# questions and their caches are independent — an instrumented build is a
# separate build-cache entry — so neither makes the other free.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing. The race
# detector additionally requires cgo and a host C toolchain on a supported
# platform (linux/darwin/windows on amd64, and arm64 on linux/darwin); where it
# is unavailable `go test -race` fails loudly rather than silently degrading to
# an uninstrumented run, which is the behavior this gate wants.
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
# chosen its own, which is the same location and the same reasoning as
# tests/test_go_toolchain.sh (see the CACHES note in that file's header: outside
# every working tree, shared across worktrees, concurrent-safe, and `-modcacherw`
# required so an ordinary `rm -rf` can still remove it). Only the first run after
# a fresh clone needs the network.
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

# The detector's verdict. A race is reported on stderr and turns the exit
# non-zero, so the captured output is replayed on failure rather than
# summarized — the WARNING block names the two conflicting stacks and is the
# whole diagnostic.
race_out="$(go test -race ./... 2>&1)"
race_rc=$?
assert "go test -race ./... (the whole module) passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'

exit "$fail"
