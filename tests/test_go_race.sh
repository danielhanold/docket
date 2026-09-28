#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_race.sh — the whole-module data-race gate (change 0308), a single
# `go test -race -count=1 ./...` run over the default (fast) corpus.
#
# HISTORY. Changes 0309, 0313, and 0314 sharded this gate four ways so each piece
# could fit under the parallel phase's hard 60s budget ceiling; change 0332
# collapsed the shards back to one `go test -race -count=1 ./...` run. One `./...`
# run covers the module by construction: nothing to partition, no completeness
# guard to maintain.
#
# PARTITION AND LANE. Change 0333 partitioned the slow real-git, subprocess, and
# process-lifecycle integration corpus of internal/app, internal/githubcli, and
# internal/gitcli behind the `integration` build tag — dedicated shard runners
# (tests/test_go_integration_*.sh) own it, and tests/test_go_integration_contract.sh
# proves that partition is total. This gate therefore covers the FAST default
# corpus only. Change 0465 made that an enforced invariant for internal/app, the
# package whose default corpus had regrown to ~920 tests (337 of them real-git,
# 225s of its 237s under -race): the default-tag internal/app test corpus never
# starts a real `git` process. testsupport.InstallNoGitGuard (internal/testsupport,
# installed from the package's TestMain) shadows git on PATH in the default build
# and fails the package on any attempt, so a new real-git test cannot land here
# unnoticed. Change 0466 extended the partition and the guard to
# internal/repository/transaction and internal/workspace, and moved
# internal/gatedrive's real-supervisor and real-git tests behind the tag too.
# gatedrive is process-bound rather than git-bound, so it has no git guard; this
# file's budget row is its growth detector. With that tail gone, `go test -race`'s
# GOMAXPROCS-wide race workers do not oversubscribe the cores the other parallel
# jobs need (change 0332's reason for the serial lane, and change 0329's
# load-dependent build-gate halt), so this gate rides the PARALLEL lane under an
# ordinary row in tests/runtime-budgets.tsv like every other file.
#
# BACKSTOP TIMEOUT (change 0465). `go test` is given an explicit -timeout
# (RACE_TIMEOUT below) of 8m, sized from CI-projected data rather than an idle
# local run. The measured post-partition worst package is
# internal/repository/transaction at 48.7s (local, idle, -p 2). Change 0466 then
# partitioned that package; the measured worst default-corpus package is now
# internal/cli at 24.0s (same shape). The backstop and its floor in
# internal/repoguard keep 0465's larger 48.7s input, so the margin only grew.
# The change's own CI data puts the macos-15 runner at ~2.4-3.8x slower than
# local (this gate: 238s local vs 581-908s on passing CI runs), projecting that
# package to ~120-186s in CI (up to ~230s with load noise). 8m is at least twice
# that worst projection, so a loaded runner does not trip the backstop, and still
# below Go's 10m per-package default. TestRaceGatePassesTimeoutBackstopBelowGoDefault
# (internal/repoguard) pins both bounds. It is NOT a growth allowance — the
# guard and the budget row are the growth detectors. It exists so an overrun fails with the named
# "no package ran past the … -timeout backstop" assert and the offending FAIL line,
# instead of a 10m goroutine-dump panic. Never raise it to make a slow package fit;
# move the slow tests behind the integration tag instead.
#
# WHY -count=1. The detector's verdict must never be served from Go's test-result
# cache: a cached "ok" certifies a previous tree, not this one. -count=1 forces a
# real run every time.
#
# WHY THIS IS ITS OWN FILE and not a fifth check inside
# tests/test_go_toolchain.sh. The detector is expensive — instrumented binaries
# run several times slower and build to a separate cache entry — so folding it
# into the Go gate would drag that file's row up and blur the two verdicts. They
# answer different questions and are budgeted separately.
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
# the source-hygiene guard (internal/repoguard) is a byte-exact allowlist, and
# the suite runner accounts results on the `ok - ` / `NOT OK - ` markers
# it prints.
#
# CACHES. The suite runner gives every job a private HOME, so with
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

# Change 0373: under the suite runner, DOCKET_GO_TEST_CONCURRENCY bounds this
# child's share of the machine (go test package parallelism and runtime procs).
# Absent (solo run), Go's defaults apply unchanged.
go_conc_args=""
if [ -n "${DOCKET_GO_TEST_CONCURRENCY:-}" ]; then
  go_conc_args="-p ${DOCKET_GO_TEST_CONCURRENCY}"
  export GOMAXPROCS="${DOCKET_GO_TEST_CONCURRENCY}"
fi

# The detector's verdict. A race is reported on stderr and turns the exit
# non-zero, so the captured output is replayed on failure rather than
# summarized — the WARNING block names the two conflicting stacks and is the
# whole diagnostic.
# The backstop — see BACKSTOP TIMEOUT in this header (change 0465).
RACE_TIMEOUT="8m"
race_out="$(go test -race $go_conc_args -timeout "$RACE_TIMEOUT" -count=1 ./... 2>&1)"
race_rc=$?
assert "go test -race -count=1 ./... (the whole module) passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'
assert "no package ran past the ${RACE_TIMEOUT} -timeout backstop" '! grep -q -E -e "^panic: test timed out after" <<<"$race_out" || { grep -E -e "^(FAIL[[:space:]]|panic: test timed out)" <<<"$race_out" >&2; printf "%s\n" "tests/test_go_race.sh: a package ran past the ${RACE_TIMEOUT} backstop — the fast default corpus has outgrown this gate; move real-git, subprocess, and process-lifecycle tests behind //go:build integration (see PARTITION AND LANE in this file)" >&2; false; }'

exit "$fail"
