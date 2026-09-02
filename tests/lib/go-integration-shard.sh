#!/usr/bin/env bash
# tests/lib/go-integration-shard.sh — shared executor for the Go integration shard
# runners (change 0333). Each tests/test_go_integration_*.sh runner declares three
# literals — SHARD_PKG, SHARD_PREFIX, SHARD_MODE — defines the canonical assert,
# sources this file, then calls shard_inspect_maybe and run_integration_shard.
#
# INSPECTION MODE. Under DOCKET_SHARD_INSPECT=1 the runner prints its three
# declarations and exits 0 WITHOUT running go test. The contract test
# (tests/test_go_integration_contract.sh) reads shard membership from this live
# invocation, never from a duplicated registry — the runner's own execution path
# and its inspected declaration cannot drift apart because they are one file.
#
# -count=1 IS MANDATORY on the executing run: correctness, completeness, and
# performance evidence must never be served from Go's test-result cache
# (learning cached-runner-serves-a-mutated-tree). This helper never converts a
# failed go test into an empty success: output is captured and replayed on the
# failure path, and an empty selection is a red assert, not a no-op.
#
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the
# CACHES note in that file's header): <git common dir>/docket-go-cache/{mod,build},
# shared across worktrees, concurrent-safe, -modcacherw required.

# The SINGLE source of truth for the race-detector decision. Both the inspection
# line (shard_inspect_maybe) and the executed `go test` invocation
# (run_integration_shard) read THIS value — never a second copy — so a regression
# that drops -race here empties both at once and the contract's race-direction
# check reddens (learning guards-are-code / correspondence-guard-runs-one-way: the
# label correspondence proved a race test is ASSIGNED to a race runner; this proves
# the runner actually instruments). The value IS the literal `go test` argument:
# `-race` for a race shard, the empty string for a normal one (no flag added). This
# body carries no runner result-marker substring, so the source-hygiene guard
# (internal/repoguard) rule (a) does not census it as an assert-family definition.
shard_race_flag(){
  if [ "$SHARD_MODE" = "race" ]; then printf '%s' '-race'; fi
}

shard_inspect_maybe(){
  if [ "${DOCKET_SHARD_INSPECT:-0}" = "1" ]; then
    # First three lines and their spellings are unchanged (the contract's field
    # parser and every runner depend on them); the race= line is ADDED, carrying
    # the SAME value the executing run passes to go test.
    printf 'package=%s\nprefix=%s\nmode=%s\nrace=%s\n' \
      "$SHARD_PKG" "$SHARD_PREFIX" "$SHARD_MODE" "$(shard_race_flag)"
    exit 0
  fi
}

run_integration_shard(){
  # The assert below already prints the canonical failure marker and sets fail=1;
  # this diagnostic goes to stderr WITHOUT that marker prefix. A function body
  # that spells the failure marker (even in a comment) is censused as an
  # assert-family definition by the source-hygiene guard (internal/repoguard) rule (a) and
  # then demanded to be byte-exact canonical, so neither the printf nor this
  # comment may carry it. return 1 unwinds to the runner's `exit "$fail"`, which
  # is 1 once the assert has reddened.
  assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
  if ! command -v go >/dev/null 2>&1; then
    printf 'this integration shard cannot certify anything without a Go toolchain\n' >&2
    return 1
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

  race_flag="$(shard_race_flag)"

  # Change 0373: under the suite runner, DOCKET_GO_TEST_CONCURRENCY bounds this
  # child's share of the machine (go test package parallelism and runtime
  # procs). Absent (solo run), Go's defaults apply unchanged.
  go_conc_args=""
  if [ -n "${DOCKET_GO_TEST_CONCURRENCY:-}" ]; then
    go_conc_args="-p ${DOCKET_GO_TEST_CONCURRENCY}"
    export GOMAXPROCS="${DOCKET_GO_TEST_CONCURRENCY}"
  fi

  # The prefix must select at least one test — a renamed corpus must redden the
  # shard, never let it pass vacuously. -list compiles but does not execute.
  listed="$(go test -tags integration $go_conc_args -list "^${SHARD_PREFIX}" "$SHARD_PKG" 2>&1)"
  listed_rc=$?
  declared="$(grep -c -E -e '^Test' <<<"$listed")"
  assert "prefix ^${SHARD_PREFIX} selects at least one tagged test in ${SHARD_PKG}" \
    '[ "$listed_rc" -eq 0 ] && [ "$declared" -ge 1 ] || { printf "%s\n" "$listed" >&2; false; }'

  # The shard itself. -v so per-test PASS markers can be counted against the
  # declared selection, catching a -run filter that silently narrowed.
  test_out="$(go test -tags integration $race_flag $go_conc_args -count=1 -run "^${SHARD_PREFIX}" -v "$SHARD_PKG" 2>&1)"
  test_rc=$?
  assert "go test -tags integration ${race_flag:+$race_flag }-run ^${SHARD_PREFIX} ${SHARD_PKG} passes" \
    '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'
  ran="$(printf '%s\n' "$test_out" | grep -c -E -e "^--- PASS: ${SHARD_PREFIX}")"
  assert "every selected test actually ran and passed (${declared} declared)" '[ "$ran" -eq "$declared" ]'
}
