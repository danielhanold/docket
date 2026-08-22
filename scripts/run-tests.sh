#!/usr/bin/env bash
# scripts/run-tests.sh — parallel runner for docket's OWN test suite (change 0227).
#
# The suite is 86 hermetic per-file scripts with no ordering dependencies, so serial execution buys
# nothing and costs ~10 minutes. This runs N at a time, each job in its own HOME/TMPDIR/git-config
# sandbox, buffering per-file output and reporting a deterministic aggregate.
#
# WHAT ISOLATION MEANS HERE: a job cannot see the developer's home directory, the developer's global
# git config, another job's temp files, or an interactive prompt. It is NOT a container — a test that
# writes inside the repo still writes inside the repo, which is why tests/runtime-budgets.tsv carries
# a `serial` mode for files that legitimately cannot share the tree.
#
# STDOUT vs STDERR: stdout carries the report, emitted after every job has finished and sorted by
# basename, so it is byte-stable across -j (modulo the wall clock). Stderr carries a live progress
# ticker in COMPLETION order, which is racy by construction — that is the point of a ticker, and it
# is why nothing keys on its order.
#
# Usage: run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check]
#                     [--strict-budget] [--budget-state PATH] [--print-budget-state-path] [TEST ...]
#   -j N               parallel jobs (default: CPU count; -j 1 is serial)
#   --verbose          print every file's output, not only failing files'
#   --timings PATH     write <relpath>\t<seconds>\t<rc>\t<passes>\t<failures> per file
#   --budgets PATH     budget table (default: tests/runtime-budgets.tsv when present)
#   --no-budget-check  skip the budget comparison entirely — no breach is measured or reported
#   --strict-budget    make a breach FATAL (exit 4); by default a breach is reported, not fatal
#   --budget-state PATH  override the advisory budget-state store path (default: under the git dir)
#   --print-budget-state-path  print the resolved budget-state store path and exit 0 (runs nothing)
#   TEST ...           test files to run (default: tests/test_*.sh)
# Exit: 0 every test file passed — including green-but-over-budget, which is reported loudly and
#       is fatal only under --strict-budget; 1 a test file failed; 3 a job produced no result at
#       all, so the run certified nothing (harness failure, not a test failure); 4 --strict-budget
#       and every test passed but a budget was exceeded; 5 the source-hygiene preflight found a
#       violation, so zero test files were executed; 2 usage error (including two targets that
#       share a basename), unmet Bash floor, or an unusable source-hygiene checker; 130/143
#       interrupted by SIGINT/SIGTERM, which reaps the in-flight jobs and reports what was lost
#       instead of producing a report.
#
# Dev tooling for THIS repo's suite — deliberately NOT a docket.sh facade op, like profile-asserts.sh.
set -uo pipefail

# `wait -n` is Bash 4.3+, above docket's own 4+ runtime floor. Re-exec under the configured runtime
# when the invoking interpreter is older (macOS still ships 3.2 as /bin/bash); the sentinel keeps a
# runtime that is itself pre-4.3 from re-exec'ing forever. Mirrors the prologue in profile-asserts.sh.
if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ] || { [ "${BASH_VERSINFO[0]:-0}" -eq 4 ] && [ "${BASH_VERSINFO[1]:-0}" -lt 3 ]; }; then
  if [ -z "${DOCKET_RUNTESTS_REEXEC:-}" ] && [ -n "${DOCKET_BASH_PATH:-}" ] && [ -x "${DOCKET_BASH_PATH:-}" ]; then
    DOCKET_RUNTESTS_REEXEC=1 exec "$DOCKET_BASH_PATH" "$0" "$@"
  fi
  printf 'run-tests: needs GNU Bash 4.3+ (wait -n); configured runtime.bash is %s — install Bash 4.3+ and re-run docket/install.sh\n' \
    "${DOCKET_BASH_PATH:-unset}" >&2
  exit 2
fi

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_BASH="${DOCKET_BASH_PATH:-$(command -v bash)}"

# Default ceiling for a file with no budget row. Also the value tests/test_runtime_budgets.sh
# asserts no row exceeds.
DEFAULT_CEILING=60
# A wall-clock assertion on a shared developer machine must tolerate load, or it becomes a flake
# that teaches people to pass --no-budget-check. It must ALSO tolerate this runner's own
# contention: a budget row is a claim about a file's cost measured SERIALLY, but enforcement
# happens during a parallel run where every job competes. Measured inflation on the change-0227
# hardware reached 2.22x (test_render_board.sh 18s -> 40s; test_harness_defaults.sh 39s -> 86s),
# so 3/2 rejected 11 healthy files. 5/2 covers the measured worst case with margin while still
# catching the regrowth this table exists to prevent — a file that doubles its OWN serial cost
# breaches, because the ceiling it is measured against did not move.
# Breach = measured > ceiling * 5/2.
#
# AND THAT IS WHY A BREACH IS ADVISORY BY DEFAULT. This constant is calibrated to ONE machine's
# measured contention, so the comparison it drives is hardware- and load-dependent in both
# directions (change 0229 tracks settling a contention-independent basis for it). A measurement
# that shaky may inform a merge, but it must not BLOCK one — and it especially must not block one
# by exiting non-zero, because "non-zero" is the only budget vocabulary this runner's callers
# have. finalize's configured-bash-finalize block and docket-build's build gate both read any
# non-zero exit as "the suite is red" and answer it by dispatching a repair agent to root-cause
# failing tests, of which a breach has none. The breach therefore leaves by the channel every
# caller does read — the report — and turns fatal only for a caller that opted in with
# --strict-budget. What that costs is stated plainly in scripts/run-tests.md: nothing in this repo
# runs the strict path automatically today, so the guard against regrowth is a loud line in every
# run's output plus tests/test_runtime_budgets.sh's structural discipline, and closing that gap is
# change 0229's job.
SLACK_NUM=5; SLACK_DEN=2

cpu_count(){
  if command -v nproc >/dev/null 2>&1; then nproc
  elif command -v sysctl >/dev/null 2>&1; then sysctl -n hw.ncpu 2>/dev/null || echo 4
  else echo 4; fi
}

# ---- budget state store: path resolution (change 0251) ------------------------------------------
# Defined up here, ahead of the option loop, so --print-budget-state-path can resolve the store path
# and exit BEFORE any discovery or the source-hygiene preflight runs (it "runs nothing"). The store's
# lock/load/write functions and their arrays live below the budget table, where the run uses them.
BS_SCHEMA=1
budget_state_path(){
  if [ -n "${BUDGET_STATE_OVERRIDE:-}" ]; then printf '%s' "$BUDGET_STATE_OVERRIDE"; return; fi
  local gd; gd="$(git -C "$REPO" rev-parse --git-dir 2>/dev/null)" || { printf ''; return; }
  # rev-parse may print a relative path; anchor it. Linked worktrees get their own git dir,
  # hence their own history (spec: "Persistent state store").
  case "$gd" in /*) ;; *) gd="$REPO/$gd" ;; esac
  printf '%s/docket/run-tests-budget-state.tsv' "$gd"
}

JOBS=""; VERBOSE=0; TIMINGS=""; BUDGETS=""; BUDGET_CHECK=1; BUDGET_STRICT=0; TARGETS=()
BUDGET_STATE_OVERRIDE=""; PRINT_STATE_PATH=0
while [ $# -gt 0 ]; do
  case "$1" in
    -j) JOBS="${2:-}"; shift 2 || exit 2 ;;
    -j*) JOBS="${1#-j}"; shift ;;
    --verbose) VERBOSE=1; shift ;;
    --timings) TIMINGS="${2:-}"; shift 2 || exit 2 ;;
    --budgets) BUDGETS="${2:-}"; shift 2 || exit 2 ;;
    --no-budget-check) BUDGET_CHECK=0; shift ;;
    --strict-budget) BUDGET_STRICT=1; shift ;;
    --budget-state) BUDGET_STATE_OVERRIDE="${2:-}"; shift 2 || exit 2 ;;
    --print-budget-state-path) PRINT_STATE_PATH=1; shift ;;
    # Range ends at the blank comment line AFTER the Exit block, not at `# Exit:` itself — Exit
    # wraps onto a continuation line, and a range ending on its first line silently drops the rest.
    -h|--help) sed -n '/^# Usage:/,/^#$/p' "${BASH_SOURCE[0]}" | sed -e '/^# *$/d' -e 's/^# \{0,1\}//'; exit 0 ;;
    --) shift; TARGETS+=("$@"); break ;;
    -*) printf 'run-tests: unknown option: %s\n' "$1" >&2; exit 2 ;;
    *) TARGETS+=("$1"); shift ;;
  esac
done

# Debug flag (change 0251): print the resolved store path and exit 0 immediately after option
# parsing — before discovery, the hygiene preflight, or any test launch. Order-independent: a
# --budget-state that appeared anywhere on the line has already set BUDGET_STATE_OVERRIDE.
if [ "$PRINT_STATE_PATH" = 1 ]; then printf '%s\n' "$(budget_state_path)"; exit 0; fi

JOBS="${JOBS:-$(cpu_count)}"
case "$JOBS" in ''|*[!0-9]*|0) printf 'run-tests: -j needs a positive integer, got "%s"\n' "$JOBS" >&2; exit 2 ;; esac

# Contradictory, and the contradiction is the dangerous direction: --no-budget-check measures no
# breach at all, so silently letting it win would hand a caller that explicitly asked to be gated
# on budgets a guard that is disarmed and green. Refuse instead of picking a winner.
if [ "$BUDGET_CHECK" = 0 ] && [ "$BUDGET_STRICT" = 1 ]; then
  printf 'run-tests: --no-budget-check and --strict-budget contradict — one skips the comparison, the other gates on it\n' >&2
  exit 2
fi

# Test-only seam (change 0251): tests/test_run_tests_budget_state.sh points discovery at a
# fixture suite so the state machine can be exercised without running the real corpus.
# Production behavior is identical when the variable is unset.
TESTS_DIR="${DOCKET_RUNTESTS_TESTS_DIR:-$REPO/tests}"
if [ "${#TARGETS[@]}" -eq 0 ]; then
  DEFAULT_CORPUS=1
  while IFS= read -r f; do TARGETS+=("$f"); done < <(find "$TESTS_DIR" -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)
else
  DEFAULT_CORPUS=0
fi
[ "${#TARGETS[@]}" -gt 0 ] || { printf 'run-tests: no test files to run\n' >&2; exit 2; }

# A mistyped path must be a usage error, not a silent rc=127 "test failure": the two read the same
# in the summary, and only one of them is a bug in the suite.
#
# The same posture covers colliding BASENAMES, and for the same reason. Logs, stat records and
# budget rows are all keyed on the basename (see "Basename is the join key" in run-tests.md), so
# `a/test_x.sh b/test_x.sh` — or one path passed twice — launches two jobs writing the same
# $WORK/logs/<base>.log and $WORK/stat/<base> concurrently: interleaved logs, doubled assert
# counts, a double-printed report row, and a SUITE line that is quietly wrong. That is strictly
# worse than the mistyped path above, because nothing about it looks like an error. Reject it here,
# before any job launches, rather than corrupting the run.
missing=0; collide=0
declare -A FIRST_PATH_OF=()
for t in "${TARGETS[@]}"; do
  [ -f "$t" ] || { printf 'run-tests: no such test file: %s\n' "$t" >&2; missing=1; continue; }
  tbase="${t##*/}"
  if [ -n "${FIRST_PATH_OF[$tbase]:-}" ]; then
    printf 'run-tests: duplicate test basename: %s (%s and %s) — logs, stat records and budget rows are keyed on basename, so both jobs would write the same files\n' \
      "$tbase" "${FIRST_PATH_OF[$tbase]}" "$t" >&2
    collide=1
  else
    FIRST_PATH_OF[$tbase]="$t"
  fi
done
{ [ "$missing" = 0 ] && [ "$collide" = 0 ]; } || exit 2

[ -n "$BUDGETS" ] || { [ -f "$REPO/tests/runtime-budgets.tsv" ] && BUDGETS="$REPO/tests/runtime-budgets.tsv"; }

if [ -n "$TIMINGS" ] && ! : > "$TIMINGS" 2>/dev/null; then
  printf 'run-tests: --timings path is not writable: %s\n' "$TIMINGS" >&2; exit 2
fi

# ---- source-hygiene preflight (change 0221) -----------------------------------------------------
# A backtick in test source executes when the SHELL READS THE LINE — before the file's first assert,
# before anything this runner could inspect. So the scan has to happen HERE: after every usage check
# above (a mistyped path and an unwritable --timings path stay the exit-2 usage errors they always
# were, rather than being pre-empted by a scan of files the caller got wrong), and before
# "---- budget table ----" and every launch below it. Detection after execution is not prevention —
# a violation must abort with ZERO test files executed, which is the claim this placement makes and
# the reason it must not drift downward into the report loop.
#
# FAIL-CLOSED IN BOTH DIRECTIONS. A gate that waves the run through when its own checker is missing
# certifies safety it did not provide, so an unusable checker refuses the run instead — exit 2, the
# same "the runner will not start" family as the Bash floor above, and NOT exit 5, which stays the
# single meaning "the preflight ran and found a violation". Readable-and-regular is the right test,
# not executable: the scan is invoked as `bash "$HYGIENE"`, which never needs the execute bit.
HYGIENE="$REPO/scripts/check-test-source-hygiene.sh"
if [ ! -f "$HYGIENE" ] || [ ! -r "$HYGIENE" ]; then
  printf 'run-tests: source-hygiene checker missing or unreadable: %s\n' "$HYGIENE" >&2
  printf 'run-tests: the preflight cannot certify these targets, so no test file is executed — restore scripts/check-test-source-hygiene.sh and re-run.\n' >&2
  exit 2
fi
# Captured into a variable and printed, never `producer | consumer`: an early-exiting consumer under
# `set -o pipefail` turns a SIGPIPE into an intermittent 141 (AGENTS.md, Shell). `--` so a target
# spelled with a leading dash reaches the checker as a path.
hyg_out="$(bash "$HYGIENE" -- "${TARGETS[@]}" 2>&1)"; hyg_rc=$?
case "$hyg_rc" in
  0) ;;
  1)
    printf 'run-tests: test-source hygiene violation — aborting with zero test files executed.\n' >&2
    [ -n "$hyg_out" ] && printf '%s\n' "$hyg_out" >&2
    printf 'run-tests: each line above names a backtick the shell would EXECUTE while reading that file. See scripts/check-test-source-hygiene.md for the classes and the remedy.\n' >&2
    exit 5 ;;
  *)
    # Not a verdict on the targets: the checker itself could not complete, so nothing was certified.
    printf 'run-tests: the source-hygiene preflight did not complete (checker exit %s) — no test file is executed.\n' "$hyg_rc" >&2
    [ -n "$hyg_out" ] && printf '%s\n' "$hyg_out" >&2
    exit 2 ;;
esac

# ---- budget table -----------------------------------------------------------------------------
# Keyed by BASENAME so a table row written repo-relative matches a target given as an absolute path.
declare -A CEILING=() MODE=()
if [ -n "$BUDGETS" ] && [ -f "$BUDGETS" ]; then
  # `|| [ -n "$bfile" ]` so a final row with no trailing newline is still read.
  while IFS=$'\t' read -r bfile bsec bmode || [ -n "${bfile:-}" ]; do
    case "$bfile" in ''|'#'*) continue ;; esac
    # A malformed seconds field falls back to the default ceiling rather than crashing the run in
    # $(( )). tests/test_runtime_budgets.sh is what makes a malformed row loud; the runner's job is
    # to keep running the tests.
    case "${bsec:-}" in ''|*[!0-9]*) bsec="$DEFAULT_CEILING" ;; esac
    CEILING["${bfile##*/}"]="$bsec"
    case "${bmode:-}" in serial) MODE["${bfile##*/}"]=serial ;; *) MODE["${bfile##*/}"]=parallel ;; esac
  done < "$BUDGETS"
fi

ceiling_of(){ local k="${1##*/}"; printf '%s' "${CEILING[$k]:-$DEFAULT_CEILING}"; }
mode_of(){    local k="${1##*/}"; printf '%s' "${MODE[$k]:-parallel}"; }

# ---- budget state store (change 0251) ---------------------------------------------------
# Advisory infrastructure: fail-open everywhere. Nothing authoritative reads stored state —
# --strict-budget re-measures current candidates directly (spec assumption 11). budget_state_path
# and BS_SCHEMA are defined above the option loop; the store's resolved path, lock, and I/O live
# here. File format: header `# docket-run-tests-budget-state v1`, then `# next_due_sequence N`, then
# tab-separated rows `context_key state initial_overrun_streak overruns_since_confirmation
# last_parallel_seconds last_solo_seconds budget_seconds last_confirmation_result due_sequence
# test_path`.
STATE_FILE="$(budget_state_path)"
STATE_USABLE=1   # flipped to 0 on any store problem; the run continues without history
STATE_LOCKED=0
state_lock(){   # bounded: ~3s of 0.1s attempts; failure is a warning, never a run failure
  local i=0
  while [ "$i" -lt 30 ]; do
    if mkdir "$STATE_FILE.lock" 2>/dev/null; then STATE_LOCKED=1; return 0; fi
    i=$((i + 1)); sleep 0.1
  done
  printf 'run-tests: budget-state lock not acquired (%s.lock) — this run reads and writes no budget history. Remove the lock dir by hand if its owner is dead.\n' "$STATE_FILE" >&2
  return 1
}
state_unlock(){ [ "$STATE_LOCKED" = 1 ] && rmdir "$STATE_FILE.lock" 2>/dev/null; STATE_LOCKED=0; }
declare -A BS_STATE=() BS_STREAK=() BS_SINCE=() BS_LASTPAR=() BS_LASTSOLO=() BS_CEIL=() BS_CONFRES=() BS_DUESEQ=() BS_PATHOF=()
BS_NEXT_SEQ=1
state_load(){    # under the lock; malformed rows ignored + reported once, wrong schema discarded
  [ -f "$STATE_FILE" ] || return 0
  # Capture-then-match, never `head | grep -q`: an early-exiting consumer under pipefail turns a
  # SIGPIPE into an intermittent 141 (AGENTS.md, Shell).
  local hdr; hdr="$(head -n1 "$STATE_FILE" 2>/dev/null)"
  grep -qF "docket-run-tests-budget-state v$BS_SCHEMA" <<<"$hdr" || return 0
  local reported=0 k st streak since lp ls bc cr ds tp
  BS_NEXT_SEQ="$(sed -n '2s/^# next_due_sequence \([0-9]*\)$/\1/p' "$STATE_FILE")"; BS_NEXT_SEQ="${BS_NEXT_SEQ:-1}"
  while IFS=$'\t' read -r k st streak since lp ls bc cr ds tp; do
    case "$k" in ''|'#'*) continue ;; esac
    if [ -z "$tp" ] || [ -z "$st" ]; then
      [ "$reported" = 0 ] && { printf 'run-tests: malformed budget-state record ignored in %s\n' "$STATE_FILE" >&2; reported=1; }
      continue
    fi
    BS_STATE[$k]="$st"; BS_STREAK[$k]="$streak"; BS_SINCE[$k]="$since"; BS_LASTPAR[$k]="$lp"
    BS_LASTSOLO[$k]="$ls"; BS_CEIL[$k]="$bc"; BS_CONFRES[$k]="$cr"; BS_DUESEQ[$k]="$ds"; BS_PATHOF[$k]="$tp"
  done < "$STATE_FILE"
}
state_write(){   # full replacement via temp-beside-destination + atomic rename; explicit chmod
  local dir tmpf k
  dir="$(dirname "$STATE_FILE")"
  mkdir -p "$dir" 2>/dev/null || { STATE_USABLE=0; return 1; }
  tmpf="$(mktemp "$dir/.run-tests-budget-state.XXXXXX")" 2>/dev/null || { STATE_USABLE=0; return 1; }
  {
    printf '# docket-run-tests-budget-state v%s\n' "$BS_SCHEMA"
    printf '# next_due_sequence %s\n' "$BS_NEXT_SEQ"
    for k in "${!BS_STATE[@]}"; do
      printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$k" "${BS_STATE[$k]}" "${BS_STREAK[$k]:-0}" \
        "${BS_SINCE[$k]:-0}" "${BS_LASTPAR[$k]:--}" "${BS_LASTSOLO[$k]:--}" "${BS_CEIL[$k]:-0}" \
        "${BS_CONFRES[$k]:--}" "${BS_DUESEQ[$k]:--}" "${BS_PATHOF[$k]}"
    done | LC_ALL=C sort
  } > "$tmpf" || { rm -f "$tmpf"; STATE_USABLE=0; return 1; }
  chmod 600 "$tmpf"   # umask makes the mktemp mode a request, not a promise
  mv -f "$tmpf" "$STATE_FILE" || { rm -f "$tmpf"; STATE_USABLE=0; return 1; }
}

# ---- execution-context key + qualifying-overrun state machine (change 0251) ---------------------
# A parallel measurement is a SCREENING observation, tracked per test AND execution context — never
# an authoritative breach (spec "Reporting": a parallel screen crossing is never labeled OVER
# BUDGET). The key embeds every dimension that makes two measurements incomparable (spec
# "Execution-context isolation"): a -j16 contention profile says nothing about -j4, and a ceiling or
# mode change starts a fresh record rather than reinterpreting an old one. Ceiling/mode/schema
# changes therefore invalidate through the key ITSELF — a changed component is simply a different
# key, so the old record is neither read nor advanced.
CTX_CPUS="$(cpu_count)"; CTX_OS="$(uname -s)"; CTX_ARCH="$(uname -m)"
context_key(){  # context_key <test-path> <ceiling> <mode>
  printf '%s|j%s|c%s|%s|%s|b%s|m%s|s%s' "$1" "$JOBS" "$CTX_CPUS" "$CTX_OS" "$CTX_ARCH" "$2" "$3" "$BS_SCHEMA"
}

# apply_screen_observations: fold this run's parallel-executed measurements (collected into the
# PE_* arrays during the report loop) into the loaded BS_* state. Called ONLY on a qualifying run,
# under the lock, between state_load and state_write. Spec "Initial confirmation trigger" +
# "Periodic revalidation":
#   overrun (parallel_time > ceiling * 5/2):
#     unobserved/watching        -> initial streak +1 (a consecutive-evidence counter)
#     parallel-sensitive/breach  -> overruns_since_confirmation +1 (accumulated, not consecutive)
#   below threshold (a "clean" qualifying measurement):
#     unobserved/watching        -> initial streak resets to 0 (the asymmetry is deliberate)
#     parallel-sensitive/breach  -> since-counter is neither incremented NOR reset (spec tests 7/8)
# A record first becomes due (streak reaches 5, or since reaches 10) is stamped with a monotonic
# due_sequence for the confirmation scheduler's tie-break (Task 4 consumes it). A clean file with no
# prior record has no history worth a row.
apply_screen_observations(){
  local i t ceil secs over k st streak since lp ls cr ds
  for i in "${!PE_PATH[@]}"; do
    t="${PE_PATH[$i]}"; ceil="${PE_CEIL[$i]}"; secs="${PE_SECS[$i]}"; over="${PE_OVER[$i]}"
    k="$(context_key "$t" "$ceil" parallel)"
    [ "$over" = 1 ] || [ -n "${BS_STATE[$k]:-}" ] || continue   # clean + no record: nothing to track
    st="${BS_STATE[$k]:-unobserved}"; streak="${BS_STREAK[$k]:-0}"; since="${BS_SINCE[$k]:-0}"
    lp="$secs"; ls="${BS_LASTSOLO[$k]:--}"; cr="${BS_CONFRES[$k]:--}"; ds="${BS_DUESEQ[$k]:--}"
    if [ "$over" = 1 ]; then
      case "$st" in
        unobserved|watching)
          st=watching; streak=$((streak + 1))
          if [ "$streak" -ge 5 ] && [ "$ds" = "-" ]; then ds="$BS_NEXT_SEQ"; BS_NEXT_SEQ=$((BS_NEXT_SEQ + 1)); fi ;;
        parallel-sensitive|confirmed-breach)
          since=$((since + 1))
          if [ "$since" -ge 10 ] && [ "$ds" = "-" ]; then ds="$BS_NEXT_SEQ"; BS_NEXT_SEQ=$((BS_NEXT_SEQ + 1)); fi ;;
      esac
    else
      case "$st" in
        unobserved|watching) streak=0 ;;
        # parallel-sensitive/confirmed-breach: leave the since-counter exactly where it was.
      esac
    fi
    BS_STATE[$k]="$st"; BS_STREAK[$k]="$streak"; BS_SINCE[$k]="$since"
    BS_LASTPAR[$k]="$lp"; BS_LASTSOLO[$k]="$ls"; BS_CEIL[$k]="$ceil"
    BS_CONFRES[$k]="$cr"; BS_DUESEQ[$k]="$ds"; BS_PATHOF[$k]="$t"
  done
}

# emit_screen_report: print one classification line per CURRENT candidate (a parallel-executed test
# that crossed the screening threshold this run), in LC_ALL=C path order, reading the just-updated
# BS_* state. Spec "Reporting" label spellings are exact. Task 4 adds the SERIAL CONFIRMATION lines.
emit_screen_report(){
  local i rec t idx ceil secs k st streak since ls
  local -a rows=() sorted=()
  for i in "${!PE_PATH[@]}"; do
    [ "${PE_OVER[$i]}" = 1 ] || continue
    rows+=("${PE_PATH[$i]}"$'\t'"$i")
  done
  [ "${#rows[@]}" -gt 0 ] || return 0
  mapfile -t sorted < <(printf '%s\n' "${rows[@]}" | LC_ALL=C sort -t$'\t' -k1,1)
  for rec in "${sorted[@]}"; do
    t="${rec%%$'\t'*}"; idx="${rec##*$'\t'}"
    ceil="${PE_CEIL[$idx]}"; secs="${PE_SECS[$idx]}"
    k="$(context_key "$t" "$ceil" parallel)"
    st="${BS_STATE[$k]:-unobserved}"; streak="${BS_STREAK[$k]:-0}"; since="${BS_SINCE[$k]:-0}"; ls="${BS_LASTSOLO[$k]:--}"
    case "$st" in
      parallel-sensitive|confirmed-breach)
        printf 'PARALLEL-SENSITIVE: %s — %ss under -j%s; last solo measurement %ss; recheck progress %s/10\n' \
          "$t" "$secs" "$JOBS" "$ls" "$since" ;;
      *)
        printf 'BUDGET WATCH: %s — %ss under -j%s; consecutive parallel-overrun streak %s/5\n' \
          "$t" "$secs" "$JOBS" "$streak" ;;
    esac
  done
}

# ---- ordering: longest budget first, so the tail starts immediately ----------------------------
PAR=(); SER=()
for t in "${TARGETS[@]}"; do
  if [ "$(mode_of "$t")" = serial ]; then SER+=("$t"); else PAR+=("$t"); fi
done
if [ "${#PAR[@]}" -gt 1 ]; then
  mapfile -t PAR < <(
    for t in "${PAR[@]}"; do printf '%s\t%s\n' "$(ceiling_of "$t")" "$t"; done |
      LC_ALL=C sort -t$'\t' -k1,1nr -k2,2 | cut -f2-
  )
fi

# Started here rather than at the launch loop so the interrupt handler below can always report an
# elapsed time under `set -u`; the difference is a mkdir and a trap install.
SUITE_START=$(date +%s)
WORK="$(mktemp -d "${TMPDIR:-/tmp}/run-tests.XXXXXX")"; trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/logs" "$WORK/stat" "$WORK/jobs"

# ---- interruption ------------------------------------------------------------------------------
# Without this, Ctrl-C is a data-destroying operation. Output is buffered until every job finishes,
# so an interrupted run has printed nothing but the stderr ticker; the EXIT trap then deletes $WORK
# out from under jobs that are still running and still writing into it. Worse, the jobs SURVIVE:
# this runner has no job control, so bash sets SIGINT to ignored in every async child, and the test
# processes those children forked inherit that — the terminal's Ctrl-C reaches the runner and
# nothing else. The result is orphaned test processes writing into a deleted directory and no
# report at all, on a run whose likeliest interactive ending is exactly Ctrl-C.
#
# NOT `kill 0`. A group-wide kill is only safe when the runner is a process-group leader, which it
# is solely when an interactive shell's job control made it one. Invoked the way this script
# actually gets invoked in anger — from another script, from finalize's `eval`, from a test file —
# job control is off and the runner shares its caller's process group, so `kill 0` would take the
# caller and its siblings down with it. It would also re-enter this handler by signalling the
# runner itself. So signal exactly what we launched, by pid, and nothing else.
#
# Order matters: reap first, THEN remove $WORK. Removing it concurrently is the same race the EXIT
# trap already loses.
on_signal(){  # on_signal <signame> <exit-code>
  trap - INT TERM  # a second Ctrl-C must not re-enter this while it is reaping
  local pf sp tp
  # Each in-flight job publishes "<subshell-pid> <test-pid>" and unlinks it on the way out, so this
  # glob is the set of jobs still running. Test process first, then the subshell waiting on it.
  for pf in "$WORK"/jobs/*/pid; do
    [ -f "$pf" ] || continue
    while read -r sp tp; do kill -TERM "$tp" "$sp" 2>/dev/null; done < "$pf"
  done
  wait 2>/dev/null
  local finished; finished="$(find "$WORK/stat" -type f 2>/dev/null | wc -l | tr -d ' ')"
  # Say what was lost. A run that dies silently after two minutes teaches the reader nothing about
  # whether it was nearly done or had barely started.
  printf 'run-tests: interrupted (%s) after %ss — %s of %s test files had finished; their output is discarded and no report is produced.\n' \
    "$1" "$(( $(date +%s) - SUITE_START ))" "$finished" "${#TARGETS[@]}" >&2
  rm -rf "$WORK"
  exit "$2"
}
trap 'on_signal INT 130' INT
trap 'on_signal TERM 143' TERM

launch(){  # launch <test-path>
  local t="$1" base; base="${t##*/}"; base="${base%.sh}"
  (
    jobdir="$WORK/jobs/$base"
    mkdir -p "$jobdir/home/.config" "$jobdir/tmp"
    export HOME="$jobdir/home"
    export TMPDIR="$jobdir/tmp"
    export XDG_CONFIG_HOME="$jobdir/home/.config"
    export GIT_CONFIG_GLOBAL="$jobdir/home/.gitconfig"
    export GIT_CONFIG_SYSTEM="$jobdir/home/.gitconfig-system"
    : > "$GIT_CONFIG_SYSTEM"
    # A synthetic identity, not an absent one: a test that commits must still be able to. Written
    # directly rather than through `git config` — three fewer forks per job, and the file lands at
    # $HOME/.gitconfig so a pre-2.32 git that ignores GIT_CONFIG_GLOBAL still reads exactly this.
    printf '[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n' \
      > "$GIT_CONFIG_GLOBAL"
    # Nothing may block on a human: a hung prompt in a background job is invisible.
    export GIT_TERMINAL_PROMPT=0 GIT_ASKPASS=true GIT_EDITOR=true EDITOR=true VISUAL=true
    export GIT_PAGER=cat PAGER=cat GIT_MERGE_AUTOEDIT=no
    start=$(date +%s)
    # Backgrounded and waited on, rather than run in the foreground, for one reason: it is the only
    # way this subshell can learn the test process's pid. The interrupt handler needs BOTH — killing
    # the subshell alone would leave the test itself orphaned, which is half of what the handler
    # exists to prevent. `wait` yields the test's own exit status, so rc is unchanged.
    "$TEST_BASH" "$t" > "$WORK/logs/$base.log" 2>&1 &
    tpid=$!
    printf '%s %s\n' "$BASHPID" "$tpid" > "$jobdir/pid"
    wait "$tpid"
    rc=$?
    # Unlink on the way out so the handler's glob is the set of jobs still IN FLIGHT — a finished
    # job's pid is reusable, and signalling a recycled pid is a worse bug than the one being fixed.
    rm -f "$jobdir/pid"
    end=$(date +%s)
    # NOT `grep -c ... || echo 0`: grep -c PRINTS 0 and EXITS 1 on no match, so the `||` branch
    # appends a second 0 and the field becomes a two-line value that corrupts the stat record.
    p="$(grep -cE '^ok[[:space:]]*-' "$WORK/logs/$base.log" 2>/dev/null)"; p="${p:-0}"
    f="$(grep -cE '^NOT OK' "$WORK/logs/$base.log" 2>/dev/null)"; f="${f:-0}"
    printf '%s\t%s\t%s\t%s\n' "$rc" "$((end - start))" "$p" "$f" > "$WORK/stat/$base"
    printf '  %-52s %s\n' "$base" "$([ "$rc" = 0 ] && echo PASS || echo FAIL)" >&2
  ) &
}

running=0
for t in ${PAR[@]+"${PAR[@]}"}; do
  # Hold at most $JOBS in flight. `wait -n` returns as soon as ONE job finishes, so the slot frees
  # on completion rather than on a batch boundary. It returns 127 with no children, which cannot
  # spin: `running` is decremented every iteration, so the loop drains either way.
  while [ "$running" -ge "$JOBS" ]; do wait -n 2>/dev/null; running=$((running - 1)); done
  launch "$t"; running=$((running + 1))
done
wait
for t in ${SER[@]+"${SER[@]}"}; do launch "$t"; wait; done
SUITE_WALL=$(( $(date +%s) - SUITE_START ))

# ---- report: deterministic, sorted by basename, independent of completion order ----------------
files=0; passed=0; failed=0; asserts=0; overbudget=0; noresult=0
failed_names=""; over_names=""; noresult_names=""
# Parallel-executed measurements this run, collected for the post-report screening state machine
# (change 0251): path, ceiling, injected/measured seconds, and whether the screen threshold crossed.
PE_PATH=(); PE_CEIL=(); PE_SECS=(); PE_OVER=()

mapfile -t ORDERED < <(
  for t in "${TARGETS[@]}"; do printf '%s\t%s\n' "${t##*/}" "$t"; done |
    LC_ALL=C sort -t$'\t' -k1,1 -k2,2 | cut -f2-
)

for t in "${ORDERED[@]}"; do
  base="${t##*/}"; base="${base%.sh}"
  # No stat record means the job's subshell died between launch and its write — an OOM kill under
  # -j, a full disk, an external signal. `files` counts records that EXIST, so skipping quietly
  # would drop the file from the report and still exit 0, certifying a suite that ran fewer files
  # than it was asked to. Name it here and answer it below; the run is not trustworthy.
  if [ ! -f "$WORK/stat/$base" ]; then
    noresult=$((noresult + 1)); noresult_names="$noresult_names $base"
    printf '%-52s %4ss  rc=%-3s ok=%-5s notok=%-4s  NO RESULT (the job died before writing one)\n' \
      "$base" "?" "?" "?" "?"
    continue
  fi
  IFS=$'\t' read -r rc secs p f < "$WORK/stat/$base"
  # Test-only seam (change 0251): replace the measured duration with an injected one so the
  # budget state machine's tests are deterministic. Column 2 = parallel seconds.
  if [ -n "${DOCKET_RUNTESTS_TEST_DURATIONS:-}" ] && [ -f "${DOCKET_RUNTESTS_TEST_DURATIONS}" ]; then
    inj="$(awk -F'\t' -v b="${base}.sh" '$1==b{print $2; exit}' "$DOCKET_RUNTESTS_TEST_DURATIONS")"
    case "${inj:-}" in ''|*[!0-9]*) ;; *) secs="$inj" ;; esac
  fi
  files=$((files + 1)); asserts=$((asserts + p + f))
  ceil="$(ceiling_of "$t")"
  fmode="$(mode_of "$t")"
  over=0
  # Budget classification splits on JOBS (change 0251). The parallel path NEVER labels a crossing
  # OVER BUDGET — a contended measurement is only a screening observation (spec "Reporting"); it is
  # collected here and classified by the post-report state machine. The direct OVER BUDGET verdict
  # survives solely on the uncontended -j 1 path (Task 5 retunes its threshold to 3/2).
  if [ "$BUDGET_CHECK" = 1 ] && [ "$JOBS" -eq 1 ]; then
    if [ $((secs * SLACK_DEN)) -gt $((ceil * SLACK_NUM)) ]; then
      over=1; overbudget=$((overbudget + 1)); over_names="$over_names $base"
    fi
  elif [ "$BUDGET_CHECK" = 1 ] && [ "$JOBS" -gt 1 ] && [ "$fmode" = parallel ]; then
    PE_PATH+=("$t"); PE_CEIL+=("$ceil"); PE_SECS+=("$secs")
    if [ "$rc" = 0 ] && [ $((secs * SLACK_DEN)) -gt $((ceil * SLACK_NUM)) ]; then PE_OVER+=(1); else PE_OVER+=(0); fi
  fi
  if [ "$rc" = 0 ]; then passed=$((passed + 1)); else failed=$((failed + 1)); failed_names="$failed_names $base"; fi
  printf '%-52s %4ss  rc=%s  ok=%-5s notok=%-4s%s\n' "$base" "$secs" "$rc" "$p" "$f" \
    "$([ "$over" = 1 ] && printf '  OVER BUDGET (ceiling %ss)' "$ceil")"
  if [ "$VERBOSE" = 1 ] || [ "$rc" != 0 ]; then
    printf -- '---- %s ----\n' "$base"; cat "$WORK/logs/$base.log"; printf -- '---- end %s ----\n' "$base"
  fi
  [ -n "$TIMINGS" ] && printf '%s\t%s\t%s\t%s\t%s\n' "$t" "$secs" "$rc" "$p" "$f" >> "$TIMINGS"
done

printf 'SUITE files=%s passed=%s failed=%s asserts=%s wall=%ss\n' "$files" "$passed" "$failed" "$asserts" "$SUITE_WALL"
[ -n "$failed_names" ] && printf 'FAILED:%s\n' "$failed_names"
if [ "$noresult" -gt 0 ]; then
  # Named, not counted: a bare count sends the reader hunting through 80-odd basenames for the one
  # that is absent from a report that is sorted but no longer complete.
  printf 'NO RESULT:%s\n' "$noresult_names"
  printf '%s of %s test files produced no result — those jobs died before recording one (OOM kill under -j, a full disk, an external signal).\n' \
    "$noresult" "${#TARGETS[@]}"
  printf 'This run certified nothing about them: re-run, and if it recurs lower -j.\n'
fi
if [ -n "$over_names" ]; then
  printf 'OVER BUDGET:%s\n' "$over_names"
  # The remedy leads with the substantive fix. It must NOT suggest raising the ceiling — a budget
  # guard whose remedy is "raise the number" teaches the evasion it exists to catch.
  printf 'Remedy: shard this file or extend an existing shard so each part stays under its ceiling.\n'
  # Say the posture out loud in the same breath, both ways round. A reader who sees a breach and an
  # exit of 0 must not conclude the check is broken, and a caller that wants to be gated on this
  # must be told the flag exists here, where the evidence for using it is on screen.
  #
  # The red branch comes FIRST because the other two would otherwise state something false: a
  # breach is reported even when the run is red, and "the tests all passed" is exactly the sentence
  # a reader of a failing run must not be handed.
  if [ "$failed" -gt 0 ]; then
    printf 'Note: this run already fails on test failures (exit 1). The breach above is a separate finding.\n'
  elif [ "$noresult" -gt 0 ]; then
    printf 'Note: this run already fails on missing results (exit 3). The breach above is a separate finding.\n'
  elif [ "$BUDGET_STRICT" = 1 ]; then
    printf 'Strict: --strict-budget was given, so this breach fails the run (exit 4). The tests themselves passed.\n'
  else
    printf 'Advisory: the tests all passed, so this run does not fail on the breach (exit 0).\n'
    printf 'Pass --strict-budget to gate on it — but see scripts/run-tests.md first: the slack factor is calibrated to one machine (change 0229).\n'
  fi
fi

# ---- budget-state application (change 0251) -------------------------------------------------
# Only a QUALIFYING parallel overrun advances persistent state (spec "Qualifying parallel
# overrun"): budget checking on, the default discovered corpus (not a targeted subset), JOBS > 1,
# the whole suite green (no failed file), and every requested test produced a result. Red,
# incomplete, interrupted (which exits above, before ever reaching here), targeted, and
# --no-budget-check runs neither advance nor reset counters — and --no-budget-check does not even
# READ history, since the whole block is gated below its BUDGET_CHECK=1 conjunct.
#
# Fail-open throughout — an empty or unusable path, an unacquirable lock, or a failed write never
# changes the run's verdict, and this block sits BELOW the exit-affecting report so it cannot.
RUN_QUALIFYING=0
if [ "$DEFAULT_CORPUS" = 1 ] && [ "$JOBS" -gt 1 ] && [ "$BUDGET_CHECK" = 1 ] \
   && [ "$failed" -eq 0 ] && [ "$noresult" -eq 0 ]; then RUN_QUALIFYING=1; fi

if [ "$RUN_QUALIFYING" = 1 ]; then
  if [ -z "$STATE_FILE" ] || ! mkdir -p "$(dirname "$STATE_FILE")" 2>/dev/null; then
    STATE_USABLE=0
    printf 'run-tests: budget state unavailable — running without budget history.\n' >&2
  elif state_lock; then
    state_load
    apply_screen_observations
    state_write
    state_unlock
    emit_screen_report
  fi
fi

[ "$failed" -gt 0 ] && exit 1
# 3, not 1: no test file failed here, so a caller that answers 1 by dispatching a repair agent to
# root-cause failing tests would send it hunting for something that is not in any log. It is still
# non-zero, which is the only part every caller reads — a run missing a file's verdict must never
# certify the suite. It ranks BELOW a real failure: when a run is both red and incomplete, exit 1
# is the more actionable signal and the NO RESULT block above is printed either way.
[ "$noresult" -gt 0 ] && exit 3
[ "$overbudget" -gt 0 ] && [ "$BUDGET_STRICT" = 1 ] && exit 4
exit 0
