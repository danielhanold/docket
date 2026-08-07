<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0227 — Parallel test-suite runner — 4x+ wall-clock speedup](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0227-parallel-test-suite-runner.md)**
<!-- docket:backlink:end -->

# Parallel Test-Suite Runner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut docket's own test-suite wall-clock from ~629s to under 157s by running the 79 hermetic `tests/test_*.sh` files in parallel under a new `scripts/run-tests.sh`, sharding the four dominant files so no shard exceeds ~60s, and guarding the result with a runtime-budget table so the tail cannot regrow.

**Architecture:** One new dev-tooling script, `scripts/run-tests.sh`, discovers `tests/test_*.sh`, orders them longest-budget-first, and runs N at a time under the configured Bash with per-job `HOME` / `TMPDIR` / git-config isolation, buffering each file's output to a per-file log and reporting a deterministic aggregate. A companion table `tests/runtime-budgets.tsv` gives every test file a wall-clock ceiling and a `parallel`/`serial` mode; the runner enforces the ceilings and a new guard test enforces the table's completeness. The four slowest files are then mechanically split along their existing section banners into shards that share a sourced prologue at `tests/lib/sync_agents_common.sh`, with assertion counts proven equal before and after.

**Tech Stack:** GNU Bash 4.3+ (`wait -n`), POSIX coreutils, git. No new dependencies, no CI service. Test discovery stays the `tests/test_*.sh` glob, so new shards self-register.

## Global Constraints

- **Zero assertion changes.** No assertion's content, name, or coverage may change. The only permitted edits to existing test files are moving assertion blocks between files and replacing a duplicated prologue with a `source` of a shared helper. Verbatim moves — do not "improve" an assertion you are relocating.
- **Assertion count before == after.** Every sharding task proves it by running the pre-split file and the post-split shards and comparing counted `ok`/`NOT OK` lines.
- **Bash floor.** Repo runtime floor is Bash 4+ (`docket_runtime_validate_bash` in `scripts/docket.sh`). `scripts/run-tests.sh` needs 4.3+ for `wait -n`, so it re-execs under `$DOCKET_BASH_PATH` exactly the way `scripts/profile-asserts.sh` re-execs for its Bash 5 requirement. Test files themselves gain no new floor.
- **Every top-level `scripts/<name>.sh` needs a co-located `scripts/<name>.md`.** Enforced by `tests/test_script_contracts_coverage.sh`. `scripts/run-tests.md` is therefore mandatory, not optional.
- **`run-tests` is NOT a facade op.** It is dev tooling for this repo, like `profile-asserts.sh` / `profile-one-test.sh` — it must not be added to `WRAPPED_OPS` in `scripts/docket.sh`. `tests/test_docket_facade.sh` asserts the dispatch `case` has only the known arms.
- **`.docket.yml` stays ≤ 40 lines.** `tests/test_docket_example_yml.sh:1161`. It is 31 lines today; Task 7 adds one.
- **Exit-code semantics are out of scope.** Change 0224 owns "green/red is the exit code". This runner keys success on each test file's own exit status, which is what the suite already does. Do not add `NOT OK`-scraping as the pass/fail authority — scraped counts are reporting only.
- **`scripts/profile-asserts.sh` and `scripts/profile-one-test.sh` are untouched.** Profiling stays serial by design.
- **House test style.** `set -uo pipefail` (never `-e`), an `assert(){ ... }` or `ok`/`no` pair that accumulates into `fail`, `exit $fail` at the end. Follow it in new test files.
- **Guard remedies must not teach the evasion.** Any count- or budget-based assertion added here must be paired with a second, independent counter over the coverage-granting path, and its failure message must lead with the substantive fix, not "raise the number". (Repo learning: `guard-remedy-must-not-teach-the-evasion`.)
- **A performance change has no oracle in the correctness suite.** Accept each task on measured wall clock, recorded in the task's verification step. (Repo learning: `optimization-needs-a-measured-oracle`.)
- **Agent-authored sweep commands run under the harness shell, not bash.** Run every multi-file sweep or verification loop under an explicit `bash -c`, and verify greps with `command grep` / `git grep`. Zero iterations and zero matches read exactly like success. (Repo learning: `agent-shell-noop-reads-as-success`.)

---

## File Structure

**Created:**

| Path | Responsibility |
|---|---|
| `scripts/run-tests.sh` | The parallel runner: discovery, ordering, per-job isolation, buffered output, aggregate summary, budget enforcement. |
| `scripts/run-tests.md` | Its contract (Purpose / Usage / Behavior / Exit codes / Invariants), per house convention. |
| `tests/test_run_tests.sh` | Guard for the runner itself — isolation, `-j 1` equivalence, failure propagation, budget breach, deterministic output. |
| `tests/runtime-budgets.tsv` | One row per test file: `<relpath>\t<max_seconds>\t<parallel\|serial>`. |
| `tests/test_runtime_budgets.sh` | Guard for the table: completeness both directions, well-formedness, the over-ceiling counter, the serial-pin counter. |
| `tests/README.md` | "Where new tests go" — extend a topical shard, never grow a file past its budget. |
| `tests/lib/sync_agents_common.sh` | The prologue the `test_sync_agents*` shards share (helpers, `$SYNC`, `$AGENTS`, sandbox builders). Not matched by the `tests/test_*.sh` glob, so it never runs as a test. |
| `tests/test_sync_agents_drift_docs.sh` | Shard 2 of `test_sync_agents.sh` — `--check` drift gate + doc/README sentinels. |
| `tests/test_sync_agents_defaults.sh` | Shard 3 — shipped-sidecar layering (changes 0051, 0168). |
| `tests/test_sync_agents_runners.sh` | Shard 4 — runner shims, atomic generation, pin injection. |
| `tests/test_sync_agents_validator.sh` | Shard 5 — the change-0173 generation validator. |
| `tests/test_harness_defaults_validator.sh` | Shard 2 of `test_harness_defaults.sh` — the malformed-shape validator. Boundary chosen by measured cost; see Task 4. |

**Modified:**

| Path | Change |
|---|---|
| `tests/test_sync_agents.sh` | Keeps shard 1 (wrapper sources + generator); prologue replaced by a `source` of `tests/lib/sync_agents_common.sh`. |
| `tests/test_harness_defaults.sh` | Keeps shard 1. |
| `tests/test_sync_agents_codex.sh` | Isolation-audit fixes only if the audit finds any; otherwise untouched. |
| `.docket.yml` | `finalize.test_command: scripts/run-tests.sh` under the existing `finalize:` block. |
| `AGENTS.md` | The "Guards and tests" bullet naming how the whole suite is run. |

---

### Task 1: The parallel runner

**Files:**
- Create: `scripts/run-tests.sh`
- Create: `scripts/run-tests.md`
- Test: `tests/test_run_tests.sh`

**Interfaces:**
- Consumes: `$DOCKET_BASH_PATH` (the configured runtime, exported by `docket.sh preflight`/`env`); `tests/runtime-budgets.tsv` if present (Task 6 creates it — the runner must work without it).
- Produces, and relied on by every later task:
  - CLI: `scripts/run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check] [TEST ...]`
  - A tab-separated timings record per file: `<relpath>\t<seconds>\t<rc>\t<passes>\t<failures>`
  - Exit codes: `0` all green and in budget; `1` one or more test files exited non-zero; `4` all test files green but at least one exceeded its budget; `2` usage error or unmet Bash floor.
  - Summary line, grepped by later tasks: `SUITE files=<n> passed=<n> failed=<n> asserts=<n> wall=<n>s`

- [ ] **Step 1: Write the failing test**

Create `tests/test_run_tests.sh`. It builds its own throwaway fixture tests so it stays sub-second — it must never invoke the real suite.

```bash
#!/usr/bin/env bash
# tests/test_run_tests.sh — guard for scripts/run-tests.sh (change 0227). Every fixture test this
# builds is deliberately trivial; this file must never invoke the real suite (it is itself IN the
# real suite, so doing so would recurse).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

RT="$REPO/scripts/run-tests.sh"
assert "runner exists"                '[ -f "$RT" ]'
assert "runner is executable"         '[ -x "$RT" ]'
assert "runner has a contract"        '[ -f "$REPO/scripts/run-tests.md" ]'
assert "runner is not a facade op"    '! grep -q "run-tests" "$REPO/scripts/docket.sh"'

T="$(mktemp -d)"; mkdir -p "$T/tests"

# Three fixture tests: two green, one red.
cat > "$T/tests/test_alpha.sh" <<'EOF'
#!/usr/bin/env bash
echo "ok - alpha one"
echo "ok - alpha two"
exit 0
EOF
cat > "$T/tests/test_beta.sh" <<'EOF'
#!/usr/bin/env bash
echo "ok - beta one"
exit 0
EOF
cat > "$T/tests/test_red.sh" <<'EOF'
#!/usr/bin/env bash
echo "NOT OK - red one"
exit 1
EOF
chmod +x "$T"/tests/test_*.sh

# (1) all-green set exits 0 and reports the aggregate
out="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/tests/test_beta.sh" 2>&1)"; rc=$?
assert "green set exits 0"                    '[ "$rc" = "0" ]'
assert "green set reports files=2 passed=2"   'grep -qE "^SUITE files=2 passed=2 failed=0 " <<<"$out"'
assert "green set counts 3 assertions"        'grep -qE "^SUITE .* asserts=3 " <<<"$out"'

# (2) a failing file propagates rc=1 and its log is printed even without --verbose
out="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/tests/test_red.sh" 2>&1)"; rc=$?
assert "failing file exits 1"                 '[ "$rc" = "1" ]'
assert "failing file named in summary"        'grep -q "test_red" <<<"$out"'
assert "failing file log is shown by default" 'grep -q "NOT OK - red one" <<<"$out"'
assert "passing log hidden without --verbose" '! grep -q "ok - alpha one" <<<"$out"'
assert "passing log shown with --verbose"     'bash "$RT" -j 2 --verbose "$T/tests/test_alpha.sh" 2>&1 | grep -q "ok - alpha one"'

# (3) -j 1 and -j 4 agree on the aggregate — parallelism changes wall time, never the verdict
s1="$(bash "$RT" -j 1 "$T"/tests/test_alpha.sh "$T"/tests/test_beta.sh 2>&1 | grep -E "^SUITE ")"
s4="$(bash "$RT" -j 4 "$T"/tests/test_alpha.sh "$T"/tests/test_beta.sh 2>&1 | grep -E "^SUITE " | sed -E "s/ wall=[0-9]+s$//")"
assert "-j1 and -j4 agree on the aggregate" '[ "${s1% wall=*}" = "${s4% wall=*}" ] || [ "$(sed -E "s/ wall=[0-9]+s$//" <<<"$s1")" = "$s4" ]'

# (4) per-file output is emitted in a deterministic (sorted) order regardless of -j
ord(){ bash "$RT" -j "$1" "$T"/tests/test_beta.sh "$T"/tests/test_alpha.sh 2>&1 | grep -oE "test_(alpha|beta)" | head -2 | tr "\n" " "; }
assert "per-file order is deterministic across -j" '[ "$(ord 1)" = "$(ord 4)" ]'

# (5) ISOLATION — a test cannot see the invoker's HOME, TMPDIR, or global git config.
cat > "$T/tests/test_iso.sh" <<'EOF'
#!/usr/bin/env bash
[ "$HOME" != "$OUTER_HOME" ] && echo "ok - HOME is isolated" || echo "NOT OK - HOME leaked"
[ "${TMPDIR%/}" != "${OUTER_TMPDIR%/}" ] && echo "ok - TMPDIR is isolated" || echo "NOT OK - TMPDIR leaked"
[ "$(git config --get user.email)" = "test@docket.invalid" ] && echo "ok - git identity is synthetic" || echo "NOT OK - real git identity leaked"
[ -w "$HOME" ] && echo "ok - HOME is writable" || echo "NOT OK - HOME not writable"
exit 0
EOF
chmod +x "$T/tests/test_iso.sh"
iso="$(OUTER_HOME="$HOME" OUTER_TMPDIR="${TMPDIR:-/tmp}" bash "$RT" -j 1 --verbose "$T/tests/test_iso.sh" 2>&1)"
assert "job HOME is isolated"          'grep -q "ok - HOME is isolated" <<<"$iso"'
assert "job TMPDIR is isolated"        'grep -q "ok - TMPDIR is isolated" <<<"$iso"'
assert "job git identity is synthetic" 'grep -q "ok - git identity is synthetic" <<<"$iso"'
assert "job HOME is writable"          'grep -q "ok - HOME is writable" <<<"$iso"'

# Two jobs must not share a HOME — a shared shim is isolation from the developer but not from
# each other, which is the race this runner exists to avoid.
cat > "$T/tests/test_home_a.sh" <<'EOF'
#!/usr/bin/env bash
echo "ok - home_a $HOME"
exit 0
EOF
cp "$T/tests/test_home_a.sh" "$T/tests/test_home_b.sh"
sed -i.bak 's/home_a/home_b/' "$T/tests/test_home_b.sh"; rm -f "$T/tests/test_home_b.sh.bak"
homes="$(bash "$RT" -j 2 --verbose "$T/tests/test_home_a.sh" "$T/tests/test_home_b.sh" 2>&1 | grep -oE "ok - home_[ab] .*" | sed -E "s/^ok - home_[ab] //" | sort -u | wc -l | tr -d " ")"
assert "each job gets its OWN HOME" '[ "$homes" = "2" ]'

# (6) TIMINGS record
tf="$T/timings.tsv"
bash "$RT" -j 2 --timings "$tf" "$T/tests/test_alpha.sh" "$T/tests/test_beta.sh" >/dev/null 2>&1
assert "timings file written"              '[ -s "$tf" ]'
assert "timings has one row per file"      '[ "$(wc -l < "$tf" | tr -d " ")" = "2" ]'
assert "timings rows are 5 tab fields"     '[ "$(awk -F"\t" "{print NF}" "$tf" | sort -u)" = "5" ]'
assert "timings carries the assert counts" 'awk -F"\t" "\$1 ~ /test_alpha/ && \$4 == 2 {found=1} END{exit !found}" "$tf"'

# (7) BUDGETS — an over-budget file reddens with exit 4, and the message says how to FIX it.
cat > "$T/tests/test_slow.sh" <<'EOF'
#!/usr/bin/env bash
sleep 3
echo "ok - slow one"
exit 0
EOF
chmod +x "$T/tests/test_slow.sh"
printf 'tests/test_slow.sh\t1\tparallel\n' > "$T/budgets.tsv"
bout="$( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets.tsv" "$T/tests/test_slow.sh" 2>&1 )"; brc=$?
assert "over-budget file exits 4 (green but slow)" '[ "$brc" = "4" ]'
assert "over-budget file is named"                 'grep -q "test_slow" <<<"$bout"'
assert "over-budget remedy says shard, not raise"  'grep -qi "shard this file or extend an existing shard" <<<"$bout"'
assert "over-budget remedy does NOT say raise the budget" '! grep -qiE "raise (the )?(budget|ceiling|number)" <<<"$bout"'
assert "--no-budget-check suppresses the breach" \
  '( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets.tsv" --no-budget-check "$T/tests/test_slow.sh" >/dev/null 2>&1 )'

# The budget check must not fire on a file that is comfortably inside its ceiling — otherwise the
# assert above would pass for the wrong reason (a check that always fires).
printf 'tests/test_slow.sh\t60\tparallel\n' > "$T/budgets_ok.tsv"
( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets_ok.tsv" "$T/tests/test_slow.sh" >/dev/null 2>&1 )
assert "in-budget file does NOT trip the check" '[ "$?" = "0" ]'

# (8) SERIAL mode — a file pinned serial still runs and is still reported.
printf 'tests/test_alpha.sh\t60\tserial\n' > "$T/budgets_serial.tsv"
sout="$( cd "$T" && bash "$RT" -j 4 --budgets "$T/budgets_serial.tsv" "$T/tests/test_alpha.sh" "$T/tests/test_beta.sh" 2>&1 )"
assert "serial-pinned file still runs"  'grep -qE "^SUITE files=2 passed=2 " <<<"$sout"'

# (9) usage error
bash "$RT" --bogus-flag >/dev/null 2>&1
assert "unknown flag exits 2" '[ "$?" = "2" ]'

rm -rf "$T"
exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_run_tests.sh`
Expected: FAIL — many `NOT OK` lines, starting with `NOT OK - runner exists`.

- [ ] **Step 3: Write the runner**

Create `scripts/run-tests.sh`:

```bash
#!/usr/bin/env bash
# scripts/run-tests.sh — parallel runner for docket's OWN test suite (change 0227).
#
# The suite is 79 hermetic per-file scripts with no ordering dependencies, so serial execution buys
# nothing and costs ~10 minutes. This runs N at a time, each job in its own HOME/TMPDIR/git-config
# sandbox, buffering per-file output and reporting a deterministic aggregate.
#
# WHAT ISOLATION MEANS HERE: a job cannot see the developer's home directory, the developer's global
# git config, another job's temp files, or an interactive prompt. It is NOT a container — a test that
# writes inside the repo still writes inside the repo, which is why tests/runtime-budgets.tsv carries
# a `serial` mode for files that legitimately cannot share the tree.
#
# Usage: run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check] [TEST ...]
#   -j N               parallel jobs (default: CPU count; -j 1 is serial)
#   --verbose          print every file's output, not only failing files'
#   --timings PATH     write <relpath>\t<seconds>\t<rc>\t<passes>\t<failures> per file
#   --budgets PATH     budget table (default: tests/runtime-budgets.tsv when present)
#   --no-budget-check  run the tests, report the times, never fail on a breach
#   TEST ...           test files to run (default: tests/test_*.sh)
# Exit: 0 green and in budget; 1 a test file failed; 4 all green but a budget was exceeded;
#       2 usage error or unmet Bash floor.
#
# Dev tooling for THIS repo's suite — deliberately NOT a docket.sh facade op, like profile-asserts.sh.
set -uo pipefail

# `wait -n` is Bash 4.3+, above docket's own 4+ runtime floor. Re-exec under the configured runtime
# when the invoking interpreter is older (macOS still ships 3.2 as /bin/bash); the sentinel keeps a
# runtime that is itself pre-4.3 from re-exec'ing forever. Mirrors scripts/profile-asserts.sh.
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
# that teaches people to pass --no-budget-check. Breach = measured > ceiling * 3/2.
SLACK_NUM=3; SLACK_DEN=2

cpu_count(){
  if command -v nproc >/dev/null 2>&1; then nproc
  elif command -v sysctl >/dev/null 2>&1; then sysctl -n hw.ncpu 2>/dev/null || echo 4
  else echo 4; fi
}

JOBS=""; VERBOSE=0; TIMINGS=""; BUDGETS=""; BUDGET_CHECK=1; TARGETS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -j) JOBS="${2:-}"; shift 2 || exit 2 ;;
    -j*) JOBS="${1#-j}"; shift ;;
    --verbose) VERBOSE=1; shift ;;
    --timings) TIMINGS="${2:-}"; shift 2 || exit 2 ;;
    --budgets) BUDGETS="${2:-}"; shift 2 || exit 2 ;;
    --no-budget-check) BUDGET_CHECK=0; shift ;;
    -h|--help) sed -n '/^# Usage:/,/^# Exit:/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    --) shift; TARGETS+=("$@"); break ;;
    -*) printf 'run-tests: unknown option: %s\n' "$1" >&2; exit 2 ;;
    *) TARGETS+=("$1"); shift ;;
  esac
done
JOBS="${JOBS:-$(cpu_count)}"
case "$JOBS" in ''|*[!0-9]*|0) printf 'run-tests: -j needs a positive integer, got "%s"\n' "$JOBS" >&2; exit 2 ;; esac

if [ "${#TARGETS[@]}" -eq 0 ]; then
  while IFS= read -r f; do TARGETS+=("$f"); done < <(find "$REPO/tests" -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)
fi
[ "${#TARGETS[@]}" -gt 0 ] || { printf 'run-tests: no test files to run\n' >&2; exit 2; }

[ -n "$BUDGETS" ] || { [ -f "$REPO/tests/runtime-budgets.tsv" ] && BUDGETS="$REPO/tests/runtime-budgets.tsv"; }

# ---- budget table -----------------------------------------------------------------------------
# Keyed by BASENAME so a table row written repo-relative matches a target given as an absolute path.
declare -A CEILING=() MODE=()
if [ -n "$BUDGETS" ] && [ -f "$BUDGETS" ]; then
  while IFS=$'\t' read -r bfile bsec bmode; do
    case "$bfile" in ''|'#'*) continue ;; esac
    CEILING["$(basename "$bfile")"]="$bsec"
    MODE["$(basename "$bfile")"]="${bmode:-parallel}"
  done < "$BUDGETS"
fi

ceiling_of(){ printf '%s' "${CEILING[$(basename "$1")]:-$DEFAULT_CEILING}"; }
mode_of(){    printf '%s' "${MODE[$(basename "$1")]:-parallel}"; }

# ---- ordering: longest budget first, so the tail starts immediately ----------------------------
PAR=(); SER=()
for t in "${TARGETS[@]}"; do
  if [ "$(mode_of "$t")" = serial ]; then SER+=("$t"); else PAR+=("$t"); fi
done
if [ "${#PAR[@]}" -gt 1 ]; then
  mapfile -t PAR < <(
    for t in "${PAR[@]}"; do printf '%s\t%s\n' "$(ceiling_of "$t")" "$t"; done |
      LC_ALL=C sort -k1,1nr -k2,2 | cut -f2-
  )
fi

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/logs" "$WORK/stat" "$WORK/jobs"

launch(){  # launch <test-path>
  local t="$1" base; base="$(basename "$t" .sh)"
  (
    jobdir="$WORK/jobs/$base"
    mkdir -p "$jobdir/home/.config" "$jobdir/tmp"
    export HOME="$jobdir/home"
    export TMPDIR="$jobdir/tmp"
    export XDG_CONFIG_HOME="$jobdir/home/.config"
    export GIT_CONFIG_GLOBAL="$jobdir/home/.gitconfig"
    export GIT_CONFIG_SYSTEM="$jobdir/home/.gitconfig-system"
    : > "$GIT_CONFIG_GLOBAL"; : > "$GIT_CONFIG_SYSTEM"
    # A synthetic identity, not an absent one: a test that commits must still be able to.
    git config --file "$GIT_CONFIG_GLOBAL" user.name  "docket test" 2>/dev/null
    git config --file "$GIT_CONFIG_GLOBAL" user.email "test@docket.invalid" 2>/dev/null
    git config --file "$GIT_CONFIG_GLOBAL" init.defaultBranch main 2>/dev/null
    # Nothing may block on a human: a hung prompt in a background job is invisible.
    export GIT_TERMINAL_PROMPT=0 GIT_ASKPASS=true GIT_EDITOR=true EDITOR=true VISUAL=true
    export GIT_PAGER=cat PAGER=cat GIT_MERGE_AUTOEDIT=no
    start=$(date +%s)
    "$TEST_BASH" "$t" > "$WORK/logs/$base.log" 2>&1
    rc=$?
    end=$(date +%s)
    p="$(grep -cE '^ok[[:space:]]*-' "$WORK/logs/$base.log" 2>/dev/null || echo 0)"
    f="$(grep -cE '^NOT OK' "$WORK/logs/$base.log" 2>/dev/null || echo 0)"
    printf '%s\t%s\t%s\t%s\n' "$rc" "$((end - start))" "$p" "$f" > "$WORK/stat/$base"
    printf '  %-52s %s\n' "$base" "$([ "$rc" = 0 ] && echo PASS || echo FAIL)" >&2
  ) &
}

SUITE_START=$(date +%s)
running=0
for t in "${PAR[@]}"; do
  while [ "$running" -ge "$JOBS" ]; do wait -n 2>/dev/null; running=$((running - 1)); done
  launch "$t"; running=$((running + 1))
done
wait
for t in "${SER[@]}"; do launch "$t"; wait; done
SUITE_WALL=$(( $(date +%s) - SUITE_START ))

# ---- report: deterministic, sorted by basename, independent of completion order ----------------
files=0; passed=0; failed=0; asserts=0; overbudget=0
failed_names=""; over_names=""
[ -n "$TIMINGS" ] && : > "$TIMINGS"

for t in $(printf '%s\n' "${TARGETS[@]}" | LC_ALL=C sort); do
  base="$(basename "$t" .sh)"
  [ -f "$WORK/stat/$base" ] || continue
  IFS=$'\t' read -r rc secs p f < "$WORK/stat/$base"
  files=$((files + 1)); asserts=$((asserts + p + f))
  ceil="$(ceiling_of "$t")"
  over=0
  if [ "$BUDGET_CHECK" = 1 ] && [ $((secs * SLACK_DEN)) -gt $((ceil * SLACK_NUM)) ]; then
    over=1; overbudget=$((overbudget + 1)); over_names="$over_names $base"
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
if [ -n "$over_names" ]; then
  printf 'OVER BUDGET:%s\n' "$over_names"
  # The remedy leads with the substantive fix. It must NOT suggest raising the ceiling — a budget
  # guard whose remedy is "raise the number" teaches the evasion it exists to catch.
  printf 'Remedy: shard this file or extend an existing shard so each part stays under its ceiling.\n'
fi

[ "$failed" -gt 0 ] && exit 1
[ "$overbudget" -gt 0 ] && exit 4
exit 0
```

Then `chmod +x scripts/run-tests.sh`.

- [ ] **Step 4: Write the contract**

Create `scripts/run-tests.md` following the house Purpose / Usage / Behavior / Exit codes / Invariants shape used by `scripts/profile-asserts.md`. It must state, at minimum:

- **Purpose** — parallel execution of docket's own `tests/test_*.sh`; dev tooling for this repo, not part of the convention a consuming repo adopts.
- **Usage** — the full flag list from the header block, verbatim.
- **Behavior** — discovery (`tests/test_*.sh` glob or explicit args); ordering (descending budget, then basename; unbudgeted files take `DEFAULT_CEILING`); per-job isolation (`HOME`, `TMPDIR`, `XDG_CONFIG_HOME`, `GIT_CONFIG_GLOBAL`/`SYSTEM` with a synthetic identity, prompt/editor/pager pinned non-interactive); serial-mode files run after the parallel phase, one at a time; output buffered per file and emitted sorted by basename, so the report is byte-stable across `-j`; failing files' logs always shown, passing files' only with `--verbose`.
- **Exit codes** — `0` / `1` / `4` / `2` exactly as in the header.
- **Invariants** — parallelism never changes a verdict, only wall time; the runner never edits a test file; `-j 1` is the serial reference; the budget check is advisory-by-flag but on by default; the runner is **not** a `docket.sh` facade op.

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_run_tests.sh`
Expected: PASS — no `NOT OK` lines, exit 0.

- [ ] **Step 6: Measure the baseline, with the runner as the instrument**

This is the task's acceptance evidence — the correctness suite cannot judge a performance change.

```bash
bash -c '
  time scripts/run-tests.sh -j 1 --no-budget-check --timings /tmp/docket-serial.tsv > /tmp/docket-serial.log 2>&1
  time scripts/run-tests.sh --no-budget-check --timings /tmp/docket-par.tsv > /tmp/docket-par.log 2>&1
  grep -E "^SUITE " /tmp/docket-serial.log /tmp/docket-par.log
  LC_ALL=C sort -k2,2nr /tmp/docket-serial.tsv | head -10
'
```

Record in the commit message: serial wall time, parallel wall time, and the two `SUITE` lines. **Both `SUITE` lines must report the same `files=`, `failed=`, and `asserts=` values** — a differing assert count means a test raced, and that is a Task 2 finding, not an acceptable result. Keep `/tmp/docket-serial.tsv`; Task 6 seeds the budget table from it.

- [ ] **Step 7: Commit**

```bash
git add scripts/run-tests.sh scripts/run-tests.md tests/test_run_tests.sh
git commit -m "feat(0227): parallel test-suite runner with per-job isolation and budgets"
```

---

### Task 2: Parallel-safety audit

**Files:**
- Modify: any `tests/test_*.sh` the audit proves unsafe (expected: few or none)
- Modify: `scripts/run-tests.md` (record the audit's outcome under Invariants)

**Interfaces:**
- Consumes: `scripts/run-tests.sh` and the two timings files from Task 1 Step 6.
- Produces: the definitive list of files that must carry `serial` mode in Task 6's table. If the list is empty, say so explicitly — an empty result is the good outcome, and Task 6's serial-pin counter is then asserted at `0`.

- [ ] **Step 1: Find the candidate offenders by inspection**

A file is a *candidate* if it reads or writes state outside its own fixture. Run each sweep under an explicit `bash -c` and use `git grep`, never the harness shell's `grep`:

```bash
bash -c '
  echo "== real HOME =="
  git grep -nE "\\\$HOME|~/" -- "tests/test_*.sh" | grep -v "HOME=" || echo "  none"
  echo "== global/system git config =="
  git grep -nE "git config --global|git config --system" -- "tests/test_*.sh" || echo "  none"
  echo "== the repo`s own worktrees or .docket =="
  git grep -nE "\.worktrees|\.docket/|worktree add" -- "tests/test_*.sh" || echo "  none"
  echo "== fixed paths under /tmp (not mktemp) =="
  git grep -nE "/tmp/[a-zA-Z0-9_.-]+" -- "tests/test_*.sh" || echo "  none"
  echo "== network =="
  git grep -nE "curl |wget |gh api|git (clone|fetch|push) (https|git@)" -- "tests/test_*.sh" || echo "  none"
  echo "== fixed ports =="
  git grep -nE "localhost:[0-9]+|127\.0\.0\.1:[0-9]+" -- "tests/test_*.sh" || echo "  none"
'
```

Record every hit and its verdict. Most `$HOME` hits will be *assignments* into a sandbox (safe); the unsafe shape is a **read** of the ambient `$HOME` or a write through it.

- [ ] **Step 2: Prove or disprove each candidate empirically**

Inspection finds the shapes; only a run finds the races. Run the full suite in parallel three times and diff the per-file verdicts against the `-j 1` baseline:

```bash
bash -c '
  for i in 1 2 3; do
    scripts/run-tests.sh --no-budget-check --timings "/tmp/docket-par-$i.tsv" > "/tmp/docket-par-$i.log" 2>&1
    echo "run $i: $(grep -E "^SUITE " "/tmp/docket-par-$i.log")"
  done
  for i in 1 2 3; do cut -f1,3 "/tmp/docket-par-$i.tsv" | LC_ALL=C sort > "/tmp/rc-$i"; done
  cut -f1,3 /tmp/docket-serial.tsv | LC_ALL=C sort > /tmp/rc-serial
  for i in 1 2 3; do echo "== diff serial vs par-$i =="; diff /tmp/rc-serial "/tmp/rc-$i" || true; done
'
```

Expected: three empty diffs. **A non-empty diff names the offender** — and a file that fails only sometimes is the same finding as one that fails always.

- [ ] **Step 3: Fix or pin each proven offender**

For each file the diff named, in order of preference:

1. **Fix the leak** — replace an ambient `$HOME`/`/tmp/fixed-name` read with a fixture-local path. This is a fixture change, not an assertion change, so it is inside the Global Constraints.
2. **Pin it serial** — only when the file legitimately needs the real repo tree (e.g. it creates a git worktree inside this repo). Record the reason in a comment at the top of that test file, naming what is shared. Task 6's table then carries `serial` for it, and its serial-pin counter is raised in the same diff with that reason quoted.

Never "fix" a race by loosening an assertion.

- [ ] **Step 4: Re-run to verify**

```bash
bash -c '
  scripts/run-tests.sh --no-budget-check --timings /tmp/docket-par-final.tsv > /tmp/docket-par-final.log 2>&1
  grep -E "^SUITE " /tmp/docket-par-final.log
  diff <(cut -f1,3 /tmp/docket-serial.tsv | LC_ALL=C sort) <(cut -f1,3 /tmp/docket-par-final.tsv | LC_ALL=C sort) && echo "PARALLEL == SERIAL"
'
```

Expected: `PARALLEL == SERIAL`, and the `SUITE` line matching the serial baseline on `files=`, `failed=`, `asserts=`.

- [ ] **Step 5: Record the outcome in the contract**

Add to `scripts/run-tests.md` under Invariants: the date of the audit, the number of files inspected (79), the offenders found, and what was done with each. If none were found, state that too — "audited, none found" is the load-bearing sentence a future reader needs.

- [ ] **Step 6: Commit**

```bash
git add -A tests scripts/run-tests.md
git commit -m "test(0227): audit the suite for parallel-execution races"
```

---

### Task 3: Shard `test_sync_agents.sh`

**Files:**
- Create: `tests/lib/sync_agents_common.sh`
- Create: `tests/test_sync_agents_drift_docs.sh`, `tests/test_sync_agents_defaults.sh`, `tests/test_sync_agents_runners.sh`, `tests/test_sync_agents_validator.sh`
- Modify: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `scripts/run-tests.sh` (for the parity measurement).
- Produces: `tests/lib/sync_agents_common.sh`, which every `test_sync_agents*.sh` shard sources as its first real statement. It defines exactly: `fail`, `assert`, `within`, `fm`, `body_of`, `mkgitrepo`, `make_sandbox`, `parser_subprocess_count`, `fm_key_count`, `fm_anchored`, and the variables `REPO`, `HD`, `AGENTS`, `AUTONOMOUS`, `SYNC`. Later shards rely on those names being unchanged.

This is the 225s file — 66% of the sharding win lives here. The split points below are the file's own `# ----` section banners, chosen so each shard carries a near-equal count of the expensive `"$SYNC"` invocations (159 total, ~1.4s each):

| Shard file | Source lines | `$SYNC` calls | est. |
|---|---|---|---|
| `tests/test_sync_agents.sh` (kept) | 40–533 | 33 | ~47s |
| `tests/test_sync_agents_drift_docs.sh` | 534–1002 | 41 | ~58s |
| `tests/test_sync_agents_defaults.sh` | 1003–1368 | 35 | ~50s |
| `tests/test_sync_agents_runners.sh` | 1369–1993 | 34 | ~48s |
| `tests/test_sync_agents_validator.sh` | 1994–end | 16 | ~23s |

- [ ] **Step 1: Capture the pre-split baseline (do this FIRST — it is unrecoverable afterwards)**

```bash
bash -c '
  scripts/run-tests.sh -j 1 --no-budget-check --verbose --timings /tmp/sa-before.tsv tests/test_sync_agents.sh > /tmp/sa-before.log 2>&1
  echo "rc/secs/ok/notok:"; cat /tmp/sa-before.tsv
  grep -cE "^ok[[:space:]]*-" /tmp/sa-before.log
'
```

Write the two numbers down — the `ok` count and the `NOT OK` count from `/tmp/sa-before.tsv` fields 4 and 5. They are this task's oracle.

- [ ] **Step 2: Extract the shared prologue**

Create `tests/lib/sync_agents_common.sh` by moving — **verbatim, no edits** — lines 1–38 of `tests/test_sync_agents.sh` (the `set -uo pipefail`, the `unset XDG_CONFIG_HOME`, `REPO`, `fail`, `assert`, `within`, `fm`, `body_of`, the `harness-defaults.sh` source, `HD`), plus `AGENTS`/`AUTONOMOUS` (41–42), `SYNC` (86), `make_sandbox` (104), `parser_subprocess_count` (108–…), `mkgitrepo` (179–…), `fm_key_count` (1646–…), and `fm_anchored` (1874–…).

**One edit is required and only one.** `REPO` is derived from `BASH_SOURCE`, and this file sits one directory deeper than the tests that source it:

```bash
# tests/lib/sync_agents_common.sh — the prologue every tests/test_sync_agents*.sh shard sources
# (change 0227). NOT matched by the tests/test_*.sh discovery glob, so it never runs as a test.
#
# This file is sourced, so BASH_SOURCE points at tests/lib/ — REPO needs TWO levels up, where the
# unsharded test needed one. That is the ONLY line that differs from the prologue it replaces.
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: the script reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
# ... within(), fm(), body_of(), the harness-defaults source, HD, AGENTS, AUTONOMOUS, SYNC,
# ... make_sandbox(), parser_subprocess_count(), mkgitrepo(), fm_key_count(), fm_anchored()
```

Each shard then opens with exactly:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_<part>.sh — <what this shard covers> (shard of test_sync_agents.sh,
# change 0227). Run: bash tests/test_sync_agents_<part>.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"
```

and closes with exactly `exit $fail`.

- [ ] **Step 3: Move the assertion blocks**

Cut lines 534–1002, 1003–1368, 1369–1993, 1994–(end minus the trailing `exit $fail`) out of `tests/test_sync_agents.sh` and paste each block, unmodified, into its shard between the prologue source and the closing `exit $fail`. `test_sync_agents.sh` keeps 40–533 and gains the prologue source in place of its old inline prologue.

Two things to check as you move, because both are silent when wrong:

- A shard that uses `fm_key_count` or `fm_anchored` no longer has them defined mid-file — they come from the common lib now. That is why Step 2 hoists them.
- A block that references `$SBX` relies on a `make_sandbox` call earlier in *its own* shard. Every one of the five blocks calls `make_sandbox` before its first `$SBX` use; confirm per shard with `grep -n 'make_sandbox\|\$SBX' <shard>` and check the first `make_sandbox` precedes the first `$SBX`.

- [ ] **Step 4: Verify parity — the count is the whole point**

```bash
bash -c '
  scripts/run-tests.sh --no-budget-check --timings /tmp/sa-after.tsv \
    tests/test_sync_agents.sh tests/test_sync_agents_drift_docs.sh tests/test_sync_agents_defaults.sh \
    tests/test_sync_agents_runners.sh tests/test_sync_agents_validator.sh > /tmp/sa-after.log 2>&1
  cat /tmp/sa-after.tsv
  echo "TOTAL ok    after: $(awk -F"\t" "{s+=\$4} END{print s}" /tmp/sa-after.tsv)"
  echo "TOTAL notok after: $(awk -F"\t" "{s+=\$5} END{print s}" /tmp/sa-after.tsv)"
  echo "BEFORE was:        $(cut -f4,5 /tmp/sa-before.tsv)"
'
```

Expected: summed `ok` after == `ok` before; summed `NOT OK` after == `NOT OK` before (which should be `0`); every shard `rc=0`; no shard over ~60s. **If a shard exceeds ~60s, split that shard again at its next `# ----` banner** — the ceiling is the constraint, the part count is not.

- [ ] **Step 5: Verify nothing references the moved lines by position**

```bash
bash -c 'git grep -nE "test_sync_agents\.sh:[0-9]+" -- . || echo "no line-anchored references — good"'
```

Expected: no hits. The repo bans filename-plus-line-number anchors (`tests/test_comment_anchor_style.sh`), so this should come back clean; if it does not, update the prose reference to name the shard instead.

- [ ] **Step 6: Commit**

```bash
git add tests/lib/sync_agents_common.sh tests/test_sync_agents*.sh
git commit -m "test(0227): shard test_sync_agents.sh into five parts behind a shared prologue"
```

---

### Task 4: Shard `test_harness_defaults.sh`

> **Amended 2026-08-07, mid-build.** This task originally also split `test_docket_config.sh`, and
> named line 169 as the `test_harness_defaults.sh` boundary. A worker executed both and returned
> `BLOCKED` with evidence; both instructions were wrong and are corrected below. The two findings
> are recorded here rather than silently dropped, because each is a durable fact about the file.
>
> **`test_docket_config.sh` is no longer split — it cannot be.** It carries the change-0126
> prelude-correspondence guard, which scans `${BASH_SOURCE[0]}` — its own file — and asserts a
> whole-file population floor:
>
> ```bash
> assert "0126 T: guard reached a real population (>= 60 sites)" '[ "$t_sites" -ge 60 ]'
> ```
>
> against a 64-site corpus, plus a derived cross-check `t_sites == t_raw - t_helper - t_comments -
> t_selflit` over the same `BASH_SOURCE`. Any two-way split leaves both halves near 32 and
> falsifies it; the measured split produced `sites=31` and one `NOT OK`. Making it pass means
> lowering the floor or teaching the guard a multi-file corpus — both assertion changes, which the
> Global Constraints forbid. The spec's own rule already covers this case: *"`test_docket_config.sh`
> and `test_sync_agents_codex.sh` → 2 parts each **if section boundaries allow; otherwise leave and
> accept the ~60s floor**."* Section boundaries do not allow it. The file measures **50s**, already
> inside the ~60s ceiling, so leaving it whole costs the change nothing. Record the reason in
> `tests/README.md` (Task 6) so the next person does not re-attempt it.
>
> **The line-169 boundary does not achieve the objective.** Measured, it yields shards of **4s and
> 76s** against an 80s original — it buys 4s and leaves a shard over the ceiling. Essentially all
> the cost sits in ~20 `hd_validate` calls inside the validator section. The boundary must be
> chosen by measured cost, not by section title; Step 2 below now says so.

**Files:**
- Create: `tests/test_harness_defaults_validator.sh` (and a third part if the measurement calls for one)
- Modify: `tests/test_harness_defaults.sh`

**Interfaces:**
- Consumes: nothing from Task 3 — this file has its own small prologue and does not share `sync_agents_common.sh`.
- Produces: one (or two) new test files that self-register via the discovery glob.

The split duplicates the short prologue rather than extracting a lib: it is a small self-contained helper set, and a lib for two consumers buys less than it costs in indirection. (`test_sync_agents.sh` earned its lib at five consumers.)

**Prologue the moved block needs** — verified by walking every reference in the moved range, not guessed: `set -uo pipefail`, `REPO`, `fail`, `assert`, the `scripts/lib/harness-defaults.sh` source, `HD`, and **`SRC`**. `T`, `mut`, `del_entry`, and `fm_has` are defined *inside* the moved block, so they travel with it — do not also put them in the prologue.

**`tests/test_sync_agents_codex.sh`** (296 lines, ~55s) is **not** split, under the same spec rule: it has no `# ----` section banners, so there is no mechanical boundary. Its budget row in Task 6 is set from its measured time with the standard margin. Record this decision in `tests/README.md` too.

- [ ] **Step 1: Capture the baseline (unrecoverable afterwards — do it first)**

```bash
bash -c '
  scripts/run-tests.sh -j 1 --no-budget-check --timings /tmp/hd-before.tsv tests/test_harness_defaults.sh >/dev/null 2>&1
  echo "harness_defaults before (path secs rc ok notok):"; cat /tmp/hd-before.tsv
'
```

Expect roughly `80s`, `rc=0`, `223` ok, `0` NOT OK.

- [ ] **Step 2: Locate the boundary by measured cost, then cut there**

The expensive unit in this file is a `hd_validate` call over the full sidecar. Count them and find the line that halves them, rather than trusting a section title:

```bash
bash -c '
  echo "== hd_validate call sites =="
  command grep -n "hd_validate" tests/test_harness_defaults.sh
  echo "== section banners =="
  command grep -nE "^# ----" tests/test_harness_defaults.sh
  echo "== total =="; command grep -c "hd_validate" tests/test_harness_defaults.sh
'
```

Choose the `# ----` banner nearest to the median call site — **not** line 169, which sits before
essentially all of them. If no single banner splits the calls acceptably, cut into **three** parts
at two banners; the ceiling is the constraint and the part count is not. Name the third part
`tests/test_harness_defaults_shapes.sh` if you need it.

Each new part opens with the prologue named above and this header:

```bash
#!/usr/bin/env bash
# tests/test_harness_defaults_<part>.sh — <what this part covers> (shard of
# test_harness_defaults.sh, change 0227). Run: bash tests/test_harness_defaults_<part>.sh
set -uo pipefail
```

Every part keeps the original's trailing report shape:

```bash
[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
```

- [ ] **Step 3: Verify parity and the ceiling**

```bash
bash -c '
  scripts/run-tests.sh -j 1 --no-budget-check --timings /tmp/hd-after.tsv tests/test_harness_defaults*.sh >/dev/null 2>&1
  b="$(cut -f4 /tmp/hd-before.tsv)"; a="$(awk -F"\t" "{s+=\$4} END{print s}" /tmp/hd-after.tsv)"
  bn="$(cut -f5 /tmp/hd-before.tsv)"; an="$(awk -F"\t" "{s+=\$5} END{print s}" /tmp/hd-after.tsv)"
  echo "ok before=$b after=$a  |  notok before=$bn after=$an"
  [ "$b" = "$a" ] && [ "$bn" = "$an" ] && echo "PARITY OK" || echo "PARITY FAILED"
  cat /tmp/hd-after.tsv
'
```

Expected: `PARITY OK` (223 ok, 0 NOT OK), every part `rc=0`, and **no part over ~60s measured
serially**. A part still over the ceiling means the boundary is wrong — re-cut it; do not accept it.

Then confirm nothing else went red:

```bash
scripts/run-tests.sh --no-budget-check
```

Expected: `failed=0`, and `files=` one or two higher than before this task.

- [ ] **Step 4: Commit**

```bash
git add tests/test_harness_defaults*.sh
git commit -m "test(0227): split the harness-defaults tail file by measured validator cost"
```

---

### Task 5: Full-suite acceptance measurement

**Files:**
- No files change. This task exists because the deliverable of Tasks 1–4 is a *number*, and no assertion in the suite can report it.

**Interfaces:**
- Consumes: everything from Tasks 1–4.
- Produces: `/tmp/docket-final.tsv` — the measured per-file timings Task 6 seeds the budget table from. This is the authoritative measurement; the Task 1 baseline was taken pre-shard.

- [ ] **Step 1: Measure the sharded suite, serial and parallel**

```bash
bash -c '
  echo "== serial reference =="
  time scripts/run-tests.sh -j 1 --no-budget-check --timings /tmp/docket-final-serial.tsv > /tmp/final-serial.log 2>&1
  grep -E "^SUITE " /tmp/final-serial.log
  echo "== parallel (default -j) =="
  time scripts/run-tests.sh --no-budget-check --timings /tmp/docket-final.tsv > /tmp/final-par.log 2>&1
  grep -E "^SUITE " /tmp/final-par.log
  echo "== slowest ten files =="
  LC_ALL=C sort -k2,2nr /tmp/docket-final.tsv | head -10
'
```

- [ ] **Step 2: Check the acceptance criteria**

Assert all four by reading the output above:

1. **Wall time < 157s** on the parallel run (the spec's ≥4x goal against the 629s baseline).
2. **`asserts=` equals the pre-change total** — compare against `/tmp/docket-serial.tsv` from Task 1 Step 6: `awk -F'\t' '{s+=$4+$5} END{print s}' /tmp/docket-serial.tsv`. Sharding moves assertions between files; it must not lose one.
3. **`failed=0`** on both runs.
4. **No single file over 60s** in `/tmp/docket-final.tsv` except a file consciously accepted in Task 4 (`test_sync_agents_codex.sh`).

**If wall time is above 157s**, the fix is to split the file at the top of the slowest-ten list one level further, not to raise the target. **If the assert count moved**, stop and find the lost assertions before continuing — a green suite with fewer assertions is the failure mode this whole change must not ship.

- [ ] **Step 3: Commit the measurement as the record**

Nothing to add to the index; record the numbers in the commit message so they survive in history:

```bash
git commit --allow-empty -m "perf(0227): suite wall time <serial>s -> <parallel>s, <n> assertions unchanged"
```

---

### Task 6: The runtime-budget guard

> **Amended 2026-08-07, mid-build (second correction).** A worker built this task's three files
> green and then returned `BLOCKED`: with the table seeded from serial times and no row above 60,
> the *enforced* full-suite run still exits 4, because 11 files breach. The cause is that this
> task's three requirements — seed from serial, no row above 60, enforced suite green — are
> jointly unsatisfiable at the slack factor Task 1 shipped. **The runner's `SLACK_NUM=3;
> SLACK_DEN=2` (1.5x) is too tight: measured contention inflation reaches 2.22x**, reproduced
> across two independent full-run pairs:
>
> | file | serial | parallel | ratio |
> |---|---|---|---|
> | `test_render_board.sh` | 18s | 40s | 2.22 |
> | `test_harness_defaults.sh` | 39s | 86s | 2.21 |
> | `test_harness_defaults_validator.sh` | 42s | 91s | 2.17 |
> | `test_board_checks.sh` | 48s | 101s | 2.10 |
>
> This is oversubscription inherent to `-j <CPU count>`, not a loaded machine. Every other lever is
> forbidden by design: raising the table's numbers is the exact evasion the guard exists to catch,
> and weakening the guard defeats the task. **Step 0 below is therefore added to this task** — a
> single, scoped constant change in `scripts/run-tests.sh`, which supersedes the `SLACK_NUM=3;
> SLACK_DEN=2` line in Task 1's code block.

**Files:**
- Modify: `scripts/run-tests.sh` and `scripts/run-tests.md` (the slack constant only — Step 0)
- Create: `tests/runtime-budgets.tsv`
- Create: `tests/test_runtime_budgets.sh`
- Create: `tests/README.md`

**Interfaces:**
- Consumes: `/tmp/docket-final-serial.tsv` from Task 5 (see Step 1 on why the serial file, not the parallel one); `scripts/run-tests.sh`'s `--budgets` reader and `DEFAULT_CEILING`.
- Produces: the table the runner reads by default at `tests/runtime-budgets.tsv`, and the guard that keeps it complete.

- [ ] **Step 0: Widen the runner's slack factor to 5/2**

In `scripts/run-tests.sh`, replace the slack constants and their comment:

```bash
# A wall-clock assertion on a shared developer machine must tolerate load, or it becomes a flake
# that teaches people to pass --no-budget-check. It must ALSO tolerate this runner's own
# contention: a budget row is a claim about a file's cost measured SERIALLY, but enforcement
# happens during a parallel run where every job competes. Measured inflation on the change-0227
# hardware reached 2.22x (test_render_board.sh 18s -> 40s; test_harness_defaults.sh 39s -> 86s),
# so 3/2 rejected 11 healthy files. 5/2 covers the measured worst case with margin while still
# catching the regrowth this table exists to prevent — a file that doubles its OWN serial cost
# breaches, because the ceiling it is measured against did not move.
# Breach = measured > ceiling * 5/2.
SLACK_NUM=5; SLACK_DEN=2
```

Update the corresponding sentence in `scripts/run-tests.md` under Behavior/Invariants so the
contract states 5/2 and why. This is the whole of Step 0 — change nothing else in either file.

Verify the constant is live rather than assumed:

```bash
bash -c '
  command grep -n "SLACK_NUM\|SLACK_DEN\|5/2" scripts/run-tests.sh scripts/run-tests.md
  bash tests/test_run_tests.sh | command grep -cE "^NOT OK"
'
```

Expected: the constants read `5` and `2`, the contract mentions 5/2, and `tests/test_run_tests.sh`
still reports `0` failures — its budget-breach fixture uses a 1s ceiling against a 3s sleep, which
breaches at either slack value, and its in-budget control uses 60s against 3s, which passes at
either. If that test goes red, the fixtures were tuned to the old constant and that is a finding to
report, not something to silently retune.

- [ ] **Step 1: Seed the table from the measurement**

**Seed from the SERIAL timings — `/tmp/docket-final-serial.tsv`, not `/tmp/docket-final.tsv`.**
(Corrected 2026-08-07 against Task 5's evidence: this step originally named the parallel file. Under
full-suite contention per-file times inflate substantially — nine files exceed 60s in the parallel
run, peaking at 101s for `test_board_checks.sh`, against a 53s serial maximum. Seeding from those
numbers would write rows the plan's own "no row may exceed 60" rule forbids, and would encode this
machine's core count into the table. A budget is a claim about a file's *own* cost, which is what
the serial number measures; the runner's 3/2 slack factor is what absorbs contention at enforcement
time.)

Generate the rows from the measured times, never by hand. Round each measured time up to the next multiple of 5 and add a working margin of 5s, floored at 10s — near-zero headroom is a known failure mode in this repo's budget tables:

```bash
bash -c '
  {
    echo "# tests/runtime-budgets.tsv — per-file wall-clock ceilings (change 0227)."
    echo "# Format: <repo-relative test path><TAB><max seconds><TAB><parallel|serial>"
    echo "#"
    echo "# WHY: the suite was 629s because four files grew to 66% of it. A one-time split decays;"
    echo "# this table is the discipline made durable. Seeded from the measured post-shard run,"
    echo "# rounded up to the next multiple of 5 plus a 5s margin (min 10s)."
    echo "#"
    echo "# A new tests/test_*.sh with NO row here fails tests/test_runtime_budgets.sh. That is"
    echo "# deliberate: adding a test file is a conscious placement decision. Read tests/README.md"
    echo "# before adding one — the usual right answer is to EXTEND an existing shard."
    echo "#"
    echo "# NO row may exceed 60 seconds. If a file outgrows its ceiling, shard it or move the new"
    echo "# assertions into a shard with room. Raising a number here is not the remedy: the guard"
    echo "# counts over-ceiling rows separately, so laundering one still reddens."
    LC_ALL=C sort -k1,1 /tmp/docket-final-serial.tsv | while IFS=$'"'"'\t'"'"' read -r p secs rc ok notok; do
      ceil=$(( ((secs + 4) / 5) * 5 + 5 )); [ "$ceil" -lt 10 ] && ceil=10
      printf "%s\t%s\tparallel\n" "tests/$(basename "$p")" "$ceil"
    done
  } > tests/runtime-budgets.tsv
  wc -l tests/runtime-budgets.tsv
  awk -F"\t" "!/^#/ && \$2 > 60 {print \"OVER 60:\", \$0}" tests/runtime-budgets.tsv
'
```

Then hand-edit **only** these two things:
- Set `serial` on any file Task 2 pinned, and add a comment line directly above it quoting Task 2's recorded reason.
- If the `OVER 60` scan printed anything, that file was not sharded far enough. Go back and shard it — do not write a row above 60.

- [ ] **Step 2: Write the failing guard test**

Create `tests/test_runtime_budgets.sh`:

```bash
#!/usr/bin/env bash
# tests/test_runtime_budgets.sh — regrowth guard (change 0227): every tests/test_*.sh carries a
# wall-clock budget row, no row exceeds the 60s ceiling, and the serial pin-list is budgeted with
# its own counter.
#
# WHY A SECOND COUNTER. A count-based guard whose remedy is "make the number agree" teaches the
# evasion it exists to catch (repo learning: guard-remedy-must-not-teach-the-evasion). Two paths
# here grant a file relief from the discipline — a ceiling above the default, and a `serial` pin
# that removes it from the parallel phase entirely. Each is counted and asserted independently, so
# taking either one silently is impossible: the completeness assertion would still pass, and the
# relief counter reddens on its own.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

TBL="$REPO/tests/runtime-budgets.tsv"
CEILING=60          # the hard ceiling; no row may exceed it
EXPECTED_SERIAL=0   # files pinned serial by the change-0227 audit. RAISING THIS IS A FINDING:
                    # a serial pin removes a file from the parallel phase, so it must be justified
                    # in the same diff with the shared state that forced it.

assert "budget table exists" '[ -f "$TBL" ]'

rows="$(grep -vE '^[[:space:]]*(#|$)' "$TBL")"

# (1) well-formedness — three tab fields, integer seconds, known mode
bad=0
while IFS=$'\t' read -r p secs mode; do
  [ -n "$p" ] || continue
  case "$secs" in ''|*[!0-9]*) echo "  malformed seconds: $p [$secs]" >&2; bad=1 ;; esac
  case "$mode" in parallel|serial) ;; *) echo "  malformed mode: $p [$mode]" >&2; bad=1 ;; esac
  [ "$(awk -F'\t' -v k="$p" '$1==k{n++} END{print n+0}' <<<"$rows")" = 1 ] || { echo "  duplicate row: $p" >&2; bad=1; }
done <<<"$rows"
assert "every row is <path><TAB><integer seconds><TAB><parallel|serial>, no duplicates" '[ "$bad" = 0 ]'

# (2) completeness, BOTH directions — a new test file with no row fails here
listed="$(awk -F'\t' '{print $1}' <<<"$rows" | LC_ALL=C sort)"
actual="$(cd "$REPO" && find tests -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)"
assert "every tests/test_*.sh has a budget row, and every row has a live file" \
  '[ "$listed" = "$actual" ] || { diff <(echo "$listed") <(echo "$actual") >&2; false; }'

# (3) RELIEF COUNTER A — rows above the hard ceiling. Independent of (2): laundering a single
# file`s ceiling upward leaves (2) green and reddens only this.
over="$(awk -F'\t' -v c="$CEILING" '$2 > c {print $1}' <<<"$rows")"
assert "no budget row exceeds the ${CEILING}s ceiling" \
  '[ -z "$over" ] || { echo "  over ceiling: $over" >&2; echo "  Shard the file or move its new assertions into a shard with room. Raising the ceiling is not the remedy." >&2; false; }'

# (4) RELIEF COUNTER B — files pinned serial, budgeted exactly. Also independent of (2).
serial_n="$(awk -F'\t' '$3 == "serial" {n++} END{print n+0}' <<<"$rows")"
assert "exactly $EXPECTED_SERIAL files are pinned serial" \
  '[ "$serial_n" = "$EXPECTED_SERIAL" ] || { echo "  serial rows: $(awk -F"\t" "\$3==\"serial\"{print \$1}" <<<"$rows" | tr "\n" " ")" >&2; echo "  A serial pin removes a file from the parallel phase. Name the shared state that forces it, in this diff." >&2; false; }'

# (5) the runner actually READS this table by default — otherwise the whole table is decoration
assert "run-tests.sh defaults to tests/runtime-budgets.tsv" \
  'grep -q "tests/runtime-budgets.tsv" "$REPO/scripts/run-tests.sh"'

# (6) tests/README.md exists and tells the reader where new tests go
assert "tests/README.md exists"          '[ -f "$REPO/tests/README.md" ]'
assert "tests/README.md says where new tests go" \
  'grep -qi "where new tests go" "$REPO/tests/README.md"'

exit $fail
```

- [ ] **Step 3: Run it to verify it fails, then passes**

Run: `bash tests/test_runtime_budgets.sh`
Expected before `tests/README.md` exists: FAIL on the last two assertions only. Every other assertion should already pass against the table generated in Step 1 — if completeness fails, the table generation missed a file.

- [ ] **Step 4: Mutation-test the guard — three mutations, each proving one assertion fires alone**

A guard is code; strip what it guards and watch it redden, or it is decoration.

```bash
bash -c '
  cp tests/runtime-budgets.tsv /tmp/rb.bak
  echo "-- M1: delete a row (completeness)"; sed -i.bak "/test_adr_checks/d" tests/runtime-budgets.tsv
  bash tests/test_runtime_budgets.sh | grep -E "^NOT OK"; cp /tmp/rb.bak tests/runtime-budgets.tsv
  echo "-- M2: launder ONE ceiling to 90 (relief counter A, with completeness still green)"
  awk -F"\t" -v OFS="\t" "/^#/{print;next} \$1 ~ /test_sync_agents.sh/ {\$2=90} {print}" /tmp/rb.bak > tests/runtime-budgets.tsv
  bash tests/test_runtime_budgets.sh | grep -E "^(ok - every tests|NOT OK)"; cp /tmp/rb.bak tests/runtime-budgets.tsv
  echo "-- M3: pin a file serial (relief counter B, with completeness still green)"
  awk -F"\t" -v OFS="\t" "/^#/{print;next} \$1 ~ /test_adr_checks/ {\$3=\"serial\"} {print}" /tmp/rb.bak > tests/runtime-budgets.tsv
  bash tests/test_runtime_budgets.sh | grep -E "^(ok - every tests|NOT OK)"; cp /tmp/rb.bak tests/runtime-budgets.tsv
  rm -f tests/runtime-budgets.tsv.bak
  echo "-- restored"; bash tests/test_runtime_budgets.sh | grep -cE "^NOT OK"
'
```

Expected: M1 reddens the completeness assertion. **M2 and M3 each redden their own relief counter while the completeness assertion stays `ok`** — that is the property the second counter exists to provide, and if either mutation leaves the suite green the counter is not independent and must be fixed. The final line prints `0`.

- [ ] **Step 5: Write `tests/README.md`**

```markdown
# docket's test suite

79-plus standalone Bash files, discovered by the `tests/test_*.sh` glob — there is no registry, so
a new file self-registers. Each file is hermetic: `set -uo pipefail`, its own tmpdir fixtures, no
ordering dependencies, runnable on its own as `bash tests/test_X.sh`.

## Running it

```
scripts/run-tests.sh             # parallel, all files, budgets enforced
scripts/run-tests.sh -j 1        # serial reference
scripts/run-tests.sh --verbose tests/test_docket_config.sh   # one file, full output
```

`scripts/run-tests.md` is the contract. Exit `0` green, `1` a test failed, `4` green but a file
blew its wall-clock budget, `2` usage error.

## Where new tests go

The suite is parallel, and its wall-clock floor is its **slowest single file** — not its total. A
file that grows past its budget slows every future build, so placement is a real decision:

1. **Extend the topical shard your assertion belongs to.** This is almost always right. Find the
   file already covering that subsystem and add to it — if it has room in `tests/runtime-budgets.tsv`.
2. **If that shard has no room, extend a sibling shard or start a new one.** `test_sync_agents*.sh`
   and `test_docket_config*.sh` are already split this way; adding `_<topic>` to the family is
   cheap and keeps every part under its ceiling.
3. **A brand-new file is for a brand-new subsystem** — a new script, a new surface. It needs a row
   in `tests/runtime-budgets.tsv`, or `tests/test_runtime_budgets.sh` fails.

**Never grow a file past its budget and raise the number.** The budget guard counts over-ceiling
rows separately from row completeness, so that edit reddens on its own. If a file legitimately
cannot be split, that is a decision to argue in the diff, not a number to bump. Two files were
argued that way at change 0227 and both are still whole:

- `test_sync_agents_codex.sh` — no internal section banners, so there is no mechanical boundary.
- `test_docket_config.sh` — it carries the change-0126 prelude-correspondence guard, which scans
  its own `${BASH_SOURCE[0]}` and asserts a whole-file floor of **≥60 `eval` sites** against a
  64-site corpus, with a derived cross-check over the same file. Any split halves the corpus and
  falsifies both. Splitting it means changing that assertion — so do not re-attempt the split
  without deciding, deliberately, what the guard's population should be across several files.

## Parallel-safety

`scripts/run-tests.sh` gives every job its own `HOME`, `TMPDIR`, `XDG_CONFIG_HOME`, and git config
(with a synthetic identity), and pins git non-interactive. A test must not read the ambient `$HOME`,
write global git config, use a fixed `/tmp/<name>` path, touch this repo's own worktrees, or reach
the network. A file that genuinely must share the real tree carries `serial` in the budget table —
and that pin is counted by the guard, so it has to be justified.
```

- [ ] **Step 6: Verify and commit**

```bash
bash -c 'bash tests/test_runtime_budgets.sh; echo "rc=$?"; scripts/run-tests.sh | tail -5'
```

Expected: the guard exits 0; the full suite exits 0 with a `SUITE` line under 157s and no `OVER BUDGET` block.

```bash
git add tests/runtime-budgets.tsv tests/test_runtime_budgets.sh tests/README.md
git commit -m "test(0227): runtime-budget table and regrowth guard"
```

---

### Task 7: Wire the runner into the merge gate

**Files:**
- Modify: `.docket.yml`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: `scripts/run-tests.sh`.
- Produces: `FINALIZE_TEST_COMMAND=scripts/run-tests.sh` from the config resolver, which is what `docket-finalize-change`'s rebase-retest gate executes.

- [ ] **Step 1: Set `finalize.test_command`**

The change's Open Questions resolved this on 2026-08-06: name the runner explicitly rather than relying on auto-detect, so the merge gate deterministically runs the parallel suite. Add one line under the existing `finalize:` block in `.docket.yml`:

```yaml
finalize:
  gate: local                  # rebase onto main + re-run the suite locally before merging
  test_command: scripts/run-tests.sh   # the parallel runner (change 0227), not auto-detect
```

- [ ] **Step 2: Verify the resolver picks it up and the file is still slim**

```bash
bash -c '
  "$DOCKET_SCRIPTS_DIR"/docket.sh env | grep -E "^FINALIZE_(GATE|TEST_COMMAND)="
  echo ".docket.yml lines: $(wc -l < .docket.yml) (budget 40)"
  bash tests/test_docket_example_yml.sh | grep -E "slim|NOT OK" | head
'
```

Expected: `FINALIZE_TEST_COMMAND=scripts/run-tests.sh`, `FINALIZE_GATE=local`, line count ≤ 40, and the example-yml guard green.

- [ ] **Step 3: Point `AGENTS.md` at the runner**

`AGENTS.md`'s "Guards and tests" section says to run the whole suite at the build gate but not how. Extend that bullet:

```markdown
- Run the whole suite at the build gate, never only the tests the spec enumerated. Use
  `scripts/run-tests.sh` — it runs the files in parallel with per-job isolation and enforces each
  file's wall-clock budget. `tests/README.md` covers where a new test belongs.
```

Keep the existing bullet's first sentence verbatim; only the "Use ..." sentence is new.

- [ ] **Step 4: Full-suite verification**

```bash
bash -c 'time scripts/run-tests.sh; echo "rc=$?"'
```

Expected: `rc=0`, `failed=0`, `asserts=` matching Task 5's number, wall under 157s, no `OVER BUDGET`.

- [ ] **Step 5: Commit**

```bash
git add .docket.yml AGENTS.md
git commit -m "chore(0227): point the merge gate and AGENTS.md at the parallel runner"
```

---

## Self-review

**Spec coverage.** Spec §1 (`scripts/run-tests.sh` + contract) → Task 1. Spec §1's isolation and hidden-shared-state audit → Tasks 1 and 2. Spec §2 (shard the tail) → Tasks 3 and 4, with the spec's "otherwise leave and accept the ~60s floor" branch taken explicitly for **two** files and the reason recorded for each — `test_sync_agents_codex.sh` (no section boundaries) and `test_docket_config.sh` (a self-scanning population-floor guard no split can satisfy; see Task 4's amendment note). Spec §3 (budget table, guard test, `tests/README.md`) → Task 6. Spec "Integration" (`finalize.test_command`, `profile-asserts.sh` untouched) → Task 7 and the Global Constraints. Spec "Verification" (5954 assertions, 0 failures, <157s; `-j 1` matches serial; assertion-count parity per split) → Task 5 Step 2, plus the parity steps inside Tasks 3 and 4 and the `-j1`/`-j4` equivalence assertion in Task 1.

**Placeholder scan.** No `TBD`/`TODO`; the runner, the guard, both test files, and the README are given in full. The two places that legitimately cannot be literal — which files the audit will find (Task 2) and the measured seconds (Tasks 5–6) — are handled by generating the artifact from the measurement rather than by predicting a value, which is the correct shape for a number that does not exist until it is measured.

**Type consistency.** The runner's timings record is `<relpath>\t<seconds>\t<rc>\t<passes>\t<failures>` in Task 1's implementation, Task 1's test (`5 tab fields`, field 4 as the pass count), Tasks 3/4's `awk -F'\t' '{s+=$4}'` parity math, and Task 6's seeding loop. The budget row is `<path>\t<seconds>\t<parallel|serial>` in the runner's reader, Task 1's fixture tables, Task 6's generator, and Task 6's guard. Exit codes `0/1/2/4` are identical in the runner, its contract, its test, and `tests/README.md`. `DEFAULT_CEILING=60` in the runner matches `CEILING=60` in the guard and the "no row may exceed 60" rule in the table header and README.
