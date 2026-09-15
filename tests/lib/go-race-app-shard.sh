# Shared derived partition for tests/test_go_race_app_*.sh. Callers declare a
# zero-based SHARD_INDEX and common SHARD_COUNT; every top-level internal/app
# test is assigned by its POSIX cksum modulo that count. The parent race gate
# inspects all sibling declarations before excluding internal/app.

race_app_inspect_maybe() {
  if [ "${DOCKET_RACE_APP_INSPECT:-}" = 1 ]; then
    printf 'package=%s\nindex=%s\ncount=%s\n' "$SHARD_PKG" "$SHARD_INDEX" "$SHARD_COUNT"
    exit 0
  fi
}

validate_race_app_shards() {
  runners="$(find tests -maxdepth 1 -name 'test_go_race_app_*.sh' | LC_ALL=C sort)"
  assert "internal/app race shard discovery is non-empty" '[ -n "$runners" ]'
  declarations=""
  malformed=""
  while IFS= read -r runner; do
    [ -n "$runner" ] || continue
    inspected="$(DOCKET_RACE_APP_INSPECT=1 bash "$runner" 2>&1)"; inspect_rc=$?
    inspected_package="$(sed -n 's/^package=//p' <<<"$inspected")"
    inspected_index="$(sed -n 's/^index=//p' <<<"$inspected")"
    inspected_count="$(sed -n 's/^count=//p' <<<"$inspected")"
    case "$inspected_index:$inspected_count" in
      *[!0-9:]*|:|*:0) malformed="$malformed $runner" ;;
    esac
    [ "$inspect_rc" -eq 0 ] || malformed="$malformed $runner"
    [ "$inspected_package" = "./internal/app" ] || malformed="$malformed $runner"
    declarations="${declarations}${inspected_index}\t${inspected_count}\n"
  done <<<"$runners"
  assert "every internal/app race shard has a valid declaration" \
    '[ -z "$malformed" ] || { printf "malformed race shard:%s\n" "$malformed" >&2; false; }'
  declared_counts="$(printf '%b' "$declarations" | awk -F '\t' 'NF == 2 {print $2}' | LC_ALL=C sort -u)"
  declared_count="$(sed -n '1p' <<<"$declared_counts")"
  count_lines="$(grep -c -E -e '.' <<<"$declared_counts")"
  declared_indices="$(printf '%b' "$declarations" | awk -F '\t' 'NF == 2 {print $1}' | LC_ALL=C sort -n -u)"
  expected_indices=""
  if [ "$count_lines" -eq 1 ] && [ -n "$declared_count" ]; then
    i=0
    while [ "$i" -lt "$declared_count" ]; do
      expected_indices="${expected_indices}${i}\n"
      i=$((i+1))
    done
  fi
  runner_count="$(grep -c -E -e '.' <<<"$runners")"
  assert "internal/app race shards form one complete zero-based partition" \
    '[ "$count_lines" -eq 1 ] && [ "$runner_count" -eq "$declared_count" ] && [ "$declared_indices" = "$(printf "%b" "$expected_indices" | sed "/^$/d")" ]'
}

run_race_app_shard() {
  assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
  if ! command -v go >/dev/null 2>&1; then
    printf 'NOT OK - the race shard cannot certify anything without a Go toolchain\n'
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

  go_conc_args=""
  if [ -n "${DOCKET_GO_TEST_CONCURRENCY:-}" ]; then
    go_conc_args="-p ${DOCKET_GO_TEST_CONCURRENCY}"
    export GOMAXPROCS="${DOCKET_GO_TEST_CONCURRENCY}"
  fi

  listed="$(go test -list '^Test' "$SHARD_PKG" 2>&1)"; list_rc=$?
  assert "go test -list derives internal/app's default test census" \
    '[ "$list_rc" -eq 0 ] || { printf "%s\n" "$listed" >&2; false; }'
  pattern='^('
  selected=0
  malformed=""
  while IFS= read -r test_name; do
    case "$test_name" in
      Test*[![:alnum:]_]*) malformed="$malformed $test_name"; continue ;;
      Test*) ;;
      *) continue ;;
    esac
    crc="$(printf '%s\n' "$test_name" | cksum | awk '{print $1}')"
    if [ $((crc % SHARD_COUNT)) -eq "$SHARD_INDEX" ]; then
      pattern="${pattern}${test_name}|"
      selected=$((selected+1))
    fi
  done <<<"$listed"
  pattern="${pattern%|})$"
  assert "the derived internal/app test census has safe names" \
    '[ -z "$malformed" ] || { printf "unsafe test names:%s\n" "$malformed" >&2; false; }'
  assert "this internal/app race partition selects at least one test" '[ "$selected" -gt 0 ]'

  race_out="$(go test -race $go_conc_args -count=1 -run "$pattern" -v "$SHARD_PKG" 2>&1)"; race_rc=$?
  if [ "$race_rc" -ne 0 ]; then
    printf '%s\n' "$race_out" >&2
  fi
  passed="$(grep -c -E -e '^--- PASS: Test[^/[:space:]]+ \(' <<<"$race_out")"
  assert "race-instrumented internal/app partition passes" '[ "$race_rc" -eq 0 ]'
  assert "every selected internal/app race test ran and passed ($selected selected)" '[ "$passed" -eq "$selected" ]'
}
