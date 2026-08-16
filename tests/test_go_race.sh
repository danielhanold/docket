#!/usr/bin/env bash
# tests/test_go_race.sh — the module data-race gate (change 0308), sharded in
# change 0309.
#
# Runs `go test -race` over every package EXCEPT internal/repository/transaction,
# which tests/test_go_race_transaction.sh now carries (see that file's header for
# the shard rationale and the 60s-ceiling arithmetic). The two shards partition
# `go list ./...` exactly; the completeness guard at the foot of this file proves
# it. The exclusion is derived from `go list`, so a new package joins this shard
# automatically.
#
# WHY THIS IS ITS OWN FILE and not a fifth check inside
# tests/test_go_toolchain.sh. The detector is expensive — instrumented binaries
# run several times slower, and internal/gitcli alone goes from 12s to ~49s
# because its real-git fixtures and deadline tests re-exec instrumented test
# binaries dozens of times. Folded into the Go gate, that single file's measured
# runtime lands above the 60s hard ceiling in tests/test_runtime_budgets.sh —
# and that ceiling is a RELIEF COUNTER, deliberately positioned to catch slow
# work being laundered into one row's budget. Raising it to fit is the evasion
# it exists to detect. Sharding is the sanctioned answer: two rows, each under
# the ceiling, running concurrently in the parallel phase.
#
# WHY REPO-WIDE and not `-race ./internal/gitcli`. The adapter surfaces there
# are the ones held concurrently by design today — one Client and one pinned
# ObjectSource, many in-flight reads — but an enumerated package list gates only
# the packages someone remembered to enumerate, and the package that grows a
# race is by definition the one nobody thought of. `./...` is the shape-keyed
# spelling and needs no maintenance as packages are added.
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

# The race gate is SHARDED (change 0309): this file runs every package EXCEPT
# internal/repository/transaction, whose real-git concurrency fixtures pushed the
# combined `-race ./...` run past the 60s hard ceiling. tests/test_go_race_transaction.sh
# runs that one package. The exclusion is DERIVED from `go list` with a single
# literal import path — never a hand-enumerated package list — so a newly added
# package lands on this shard automatically, and the completeness guard below
# proves the two shards partition `go list ./...` exactly.
TXN_PKG="github.com/danielhanold/docket/internal/repository/transaction"
all_pkgs="$(go list ./... 2>/dev/null)"
main_pkgs="$(printf '%s\n' "$all_pkgs" | grep -v -F -x -e "$TXN_PKG")"
txn_pkgs="$(printf '%s\n' "$all_pkgs" | grep -F -x -e "$TXN_PKG")"

# The detector's verdict for this shard. A race is reported on stderr and turns
# the exit non-zero, so the captured output is replayed on failure rather than
# summarized — the WARNING block names the two conflicting stacks and is the
# whole diagnostic. main_pkgs is deliberately UNQUOTED so it word-splits into one
# argument per package.
# shellcheck disable=SC2086 # deliberate word-splitting: one package per line.
race_out="$(go test -race $main_pkgs 2>&1)"
race_rc=$?
assert "go test -race (all packages except the transaction shard) passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'

# Completeness guard: the two race shards must together cover `go list ./...`
# exactly once each — no package silently dropped from the race gate, none run
# twice. Both sets are DERIVED from `go list` here (never hand-enumerated), and
# the sibling shard is checked to actually target the transaction package, so a
# drift in either file's selector reddens rather than quietly narrowing coverage.
union_pkgs="$(printf '%s\n%s\n' "$main_pkgs" "$txn_pkgs" | grep -v '^$' | sort -u)"
all_sorted="$(printf '%s\n' "$all_pkgs" | grep -v '^$' | sort -u)"
overlap="$(comm -12 <(printf '%s\n' "$main_pkgs" | grep -v '^$' | sort -u) <(printf '%s\n' "$txn_pkgs" | grep -v '^$' | sort -u))"
sibling="$REPO/tests/test_go_race_transaction.sh"
assert "the transaction shard's package exists in the module" '[ -n "$txn_pkgs" ]'
assert "the two race shards' union equals go list ./... (no package dropped)" '[ "$union_pkgs" = "$all_sorted" ]'
assert "the two race shards are disjoint (no package run twice)" '[ -z "$overlap" ]'
assert "the sibling shard targets the transaction package" 'grep -qF -- "go test -race ./internal/repository/transaction/" "$sibling"'

exit "$fail"
