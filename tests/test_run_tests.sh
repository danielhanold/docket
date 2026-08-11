#!/usr/bin/env bash
# tests/test_run_tests.sh — guard for scripts/run-tests.sh (change 0227). Every fixture test this
# builds is deliberately trivial; this file must never invoke the real suite (it is itself IN the
# real suite, so doing so would recurse).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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
# Captured first, then matched from a here-string: `producer | grep -q` under pipefail lets the
# producer take SIGPIPE and turn a real result into an intermittent 141 (AGENTS.md, Shell).
vout="$(bash "$RT" -j 2 --verbose "$T/tests/test_alpha.sh" 2>&1)"
assert "passing log shown with --verbose"     'grep -q "ok - alpha one" <<<"$vout"'

# A file with ZERO `ok` lines must still produce a well-formed stat record. `grep -c` prints 0 AND
# exits 1 on no match, so the obvious `grep -c ... || echo 0` yields a TWO-LINE count field that
# truncates the record and drops a column — green tests hide it, this is where it shows.
out="$(bash "$RT" -j 1 "$T/tests/test_red.sh" 2>&1)"
assert "a file with no ok lines still counts its NOT OKs" \
  'grep -qE "^SUITE files=1 passed=0 failed=1 asserts=1 " <<<"$out"'

# (3) -j 1 and -j 4 agree on the aggregate — parallelism changes wall time, never the verdict
s1="$(bash "$RT" -j 1 "$T"/tests/test_alpha.sh "$T"/tests/test_beta.sh 2>&1 | grep -E "^SUITE ")"
s4="$(bash "$RT" -j 4 "$T"/tests/test_alpha.sh "$T"/tests/test_beta.sh 2>&1 | grep -E "^SUITE " | sed -E "s/ wall=[0-9]+s$//")"
assert "-j1 and -j4 agree on the aggregate" '[ "${s1% wall=*}" = "${s4% wall=*}" ] || [ "$(sed -E "s/ wall=[0-9]+s$//" <<<"$s1")" = "$s4" ]'

# (4) per-file output is emitted in a deterministic (sorted) order regardless of -j.
# Reads STDOUT only, deliberately: the deterministic report is what stdout carries. Stderr carries
# the live progress ticker, which is emitted in COMPLETION order and is racy by construction —
# folding it in with 2>&1 would test the ticker's order, which the runner does not promise.
# `awk NR<=2`, never `head -2`: head exits early and SIGPIPEs the producer (AGENTS.md, Shell).
ord(){ bash "$RT" -j "$1" "$T"/tests/test_beta.sh "$T"/tests/test_alpha.sh 2>/dev/null | grep -oE "test_(alpha|beta)" | awk 'NR<=2' | tr "\n" " "; }
assert "per-file order is deterministic across -j" '[ "$(ord 1)" = "$(ord 4)" ]'
assert "per-file order is sorted, not argv order"  '[ "$(ord 4)" = "test_alpha test_beta " ]'

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

# (7) BUDGETS — a breach is REPORTED loudly, and it is fatal only for a caller that asked
# (`--strict-budget`). The default posture is the load-bearing part: a non-zero exit is read as
# "the suite is red" by every caller that only knows the universal `non-zero = failed` rule, which
# is all three of this runner's — finalize's `configured-bash-finalize` block, docket-build's build
# gate, and a human or agent following AGENTS.md. None of them can tell 4 from 1, and the first two
# answer red by dispatching a repair agent to root-cause failing tests that do not exist. So a
# green-but-slow run exits 0 while saying so out loud. See scripts/run-tests.md, "Budget
# enforcement", for the full argument and for what is deferred to change 0229.
cat > "$T/tests/test_slow.sh" <<'EOF'
#!/usr/bin/env bash
sleep 3
echo "ok - slow one"
exit 0
EOF
chmod +x "$T/tests/test_slow.sh"
printf 'tests/test_slow.sh\t1\tparallel\n' > "$T/budgets.tsv"

# A second, CHEAPER breach fixture for the asserts below that are about posture and reporting
# rather than about the slack arithmetic. `test_slow` at a 1s ceiling is the one that exercises
# the 5/2 boundary (3s breaches, 2s would not); these only need *a* breach, and a 1s sleeper at a
# 0s ceiling is the shortest one that exists. Cost matters here: this file is itself in the budget
# table, and padding it out is the regrowth the table exists to catch.
cat > "$T/tests/test_slowish.sh" <<'EOF'
#!/usr/bin/env bash
sleep 1
echo "ok - slowish one"
exit 0
EOF
chmod +x "$T/tests/test_slowish.sh"
printf 'tests/test_slowish.sh\t0\tparallel\n' > "$T/budgets_cheap.tsv"
bout="$( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets.tsv" "$T/tests/test_slow.sh" 2>&1 )"; brc=$?
assert "over-budget green suite exits 0 by default" '[ "$brc" = "0" ]'
assert "over-budget file is named"                 'grep -q "test_slow" <<<"$bout"'
assert "over-budget breach is still reported"      'grep -q "OVER BUDGET" <<<"$bout"'
assert "advisory breach says the run did not fail" 'grep -qi "does not fail" <<<"$bout"'
assert "advisory breach names the strict opt-in"   'grep -q -- "--strict-budget" <<<"$bout"'
assert "over-budget remedy says shard, not raise"  'grep -qi "shard this file or extend an existing shard" <<<"$bout"'
assert "over-budget remedy does NOT say raise the budget" '! grep -qiE "raise (the )?(budget|ceiling|number)" <<<"$bout"'

# The strict path is what a budget-aware caller opts into, and it still yields 4: the code that
# tells "green but slow" apart from "red" has not gone anywhere — only its default audience has.
sout7="$( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets_cheap.tsv" --strict-budget "$T/tests/test_slowish.sh" 2>&1 )"; src7=$?
assert "--strict-budget makes an over-budget file exit 4" '[ "$src7" = "4" ]'
assert "--strict-budget still reports the breach"         'grep -q "OVER BUDGET" <<<"$sout7"'

# --no-budget-check is strictly stronger than the advisory default: it suppresses the comparison
# itself, so the breach is not even reported. That gap is the whole reason advisory exists as a
# third state rather than being spelled `--no-budget-check` at the merge gate.
nout="$( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets_cheap.tsv" --no-budget-check "$T/tests/test_slowish.sh" 2>&1 )"; nrc=$?
assert "--no-budget-check exits 0"           '[ "$nrc" = "0" ]'
assert "--no-budget-check reports no breach" '! grep -q "OVER BUDGET" <<<"$nout"'

# Asking for both is a contradiction, and a silently disarmed guard is the exact failure this
# section exists to prevent — so it is a usage error, not a winner-takes-all.
( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets_cheap.tsv" --no-budget-check --strict-budget "$T/tests/test_slowish.sh" >/dev/null 2>&1 ); crc=$?
assert "--no-budget-check with --strict-budget is a usage error" '[ "$crc" = "2" ]'

# Failures still win, and the advisory text must not contradict them. A run that is BOTH red and
# over budget exits 1 — and must not print "the tests all passed" at a reader whose tests did not.
printf 'tests/test_slowish.sh\t0\tparallel\ntests/test_red.sh\t60\tparallel\n' > "$T/budgets_mixed.tsv"
mout="$( cd "$T" && bash "$RT" -j 2 --budgets "$T/budgets_mixed.tsv" "$T/tests/test_slowish.sh" "$T/tests/test_red.sh" 2>&1 )"; mrc=$?
assert "red + over budget exits 1, not 0 or 4" '[ "$mrc" = "1" ]'
assert "red + over budget still reports the breach" 'grep -q "OVER BUDGET" <<<"$mout"'
assert "red run is NOT told its tests all passed" '! grep -qi "tests all passed" <<<"$mout"'

# The check must not fire on a file comfortably inside its ceiling — otherwise the strict assert
# above would pass for the wrong reason (a check that always fires). Run it STRICT: under the
# advisory default an rc of 0 here would prove nothing at all.
printf 'tests/test_slow.sh\t60\tparallel\n' > "$T/budgets_ok.tsv"
iout="$( cd "$T" && bash "$RT" -j 1 --budgets "$T/budgets_ok.tsv" --strict-budget "$T/tests/test_slow.sh" 2>&1 )"; irc=$?
assert "in-budget file does NOT trip the check, even strict" '[ "$irc" = "0" ]'
assert "in-budget file reports no breach"                    '! grep -q "OVER BUDGET" <<<"$iout"'

# (8) SERIAL mode — a file pinned serial still runs and is still reported.
printf 'tests/test_alpha.sh\t60\tserial\n' > "$T/budgets_serial.tsv"
sout="$( cd "$T" && bash "$RT" -j 4 --budgets "$T/budgets_serial.tsv" "$T/tests/test_alpha.sh" "$T/tests/test_beta.sh" 2>&1 )"
assert "serial-pinned file still runs"  'grep -qE "^SUITE files=2 passed=2 " <<<"$sout"'

# A set that is ENTIRELY serial must still run — the parallel phase is then empty, and an empty
# array under `set -u` is exactly where a slot scheduler falls over.
printf 'tests/test_alpha.sh\t60\tserial\ntests/test_beta.sh\t60\tserial\n' > "$T/budgets_allser.tsv"
aout="$( cd "$T" && bash "$RT" -j 4 --budgets "$T/budgets_allser.tsv" "$T/tests/test_alpha.sh" "$T/tests/test_beta.sh" 2>&1 )"
assert "an all-serial set still runs"   'grep -qE "^SUITE files=2 passed=2 failed=0 asserts=3 " <<<"$aout"'

# (9) SLOT ACCOUNTING — never more than -j N jobs in flight, and no fewer when work is waiting.
# An off-by-one in the slot loop is invisible in the verdict (every file still passes and the
# aggregate is identical), so overlap has to be observed directly. Each fixture appends a token on
# entry and another on exit; O_APPEND makes those single-byte writes atomic, so the file is a true
# interleaving and the running prefix sum is the concurrency at each instant.
for n in 1 2 3 4; do
  cat > "$T/tests/test_slot$n.sh" <<'EOF'
#!/usr/bin/env bash
printf '+\n' >> "$SLOTLOG"
sleep 1
printf -- '-\n' >> "$SLOTLOG"
echo "ok - slot"
exit 0
EOF
done
chmod +x "$T"/tests/test_slot*.sh
: > "$T/slotlog"
SLOTLOG="$T/slotlog" bash "$RT" -j 2 "$T"/tests/test_slot1.sh "$T"/tests/test_slot2.sh \
  "$T"/tests/test_slot3.sh "$T"/tests/test_slot4.sh >/dev/null 2>&1
peak="$(awk '/^\+/{c++; if (c > m) m = c} /^-/{c--} END{print m + 0}' "$T/slotlog")"
# Exactly 2 is two-sided on purpose: 3 means the slot loop leaks a job, 1 means it never actually
# ran anything in parallel.
assert "-j 2 holds exactly 2 jobs in flight" '[ "$peak" = "2" ]'
: > "$T/slotlog"
SLOTLOG="$T/slotlog" bash "$RT" -j 1 "$T"/tests/test_slot1.sh "$T"/tests/test_slot2.sh >/dev/null 2>&1
peak1="$(awk '/^\+/{c++; if (c > m) m = c} /^-/{c--} END{print m + 0}' "$T/slotlog")"
assert "-j 1 never overlaps two jobs" '[ "$peak1" = "1" ]'

# (10) A JOB THAT PRODUCES NO RESULT AT ALL must be loud, not silently dropped. Per-file verdicts
# are stat records the job's own subshell writes AFTER the test exits; if that subshell dies first
# (OOM kill under -j, a full disk, an external signal) no record is written, and a report loop that
# skips a missing record counts `files` from what survived — an incomplete run that still exits 0.
# The fixture reproduces exactly that: the runner's per-job subshell is this script's parent, so
# SIGKILLing $PPID drops the record between launch and its write. No sleep is needed — the parent
# is blocked waiting on this process, so the signal lands before it can reach the printf.
cat > "$T/tests/test_vanish.sh" <<'EOF'
#!/usr/bin/env bash
echo "ok - vanish ran"
kill -KILL "$PPID" 2>/dev/null
exit 0
EOF
chmod +x "$T/tests/test_vanish.sh"
gout="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/tests/test_vanish.sh" 2>&1)"; grc=$?
assert "a job with no stat record does NOT exit 0"   '[ "$grc" != "0" ]'
assert "a job with no stat record exits 3"           '[ "$grc" = "3" ]'
assert "the file with no result is NAMED, not just counted" 'grep -qE "^NO RESULT:.* test_vanish" <<<"$gout"'
assert "the no-result report gives the counts"       'grep -q "1 of 2 test files produced no result" <<<"$gout"'
assert "the surviving file is still reported"        'grep -qE "^SUITE files=1 passed=1 failed=0 " <<<"$gout"'
# Negative control: the check must key on an ABSENT record, not fire on every run — otherwise the
# asserts above would pass for the wrong reason.
cout="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/tests/test_beta.sh" 2>&1)"; crc10=$?
assert "a complete run reports no missing result"    '! grep -q "NO RESULT" <<<"$cout"'
assert "a complete run still exits 0"                '[ "$crc10" = "0" ]'

# It must also not leave the budget block telling the reader something false about the exit: the
# advisory sentence claims exit 0, which a run missing a result is not.
vbout="$( cd "$T" && bash "$RT" -j 2 --budgets "$T/budgets_cheap.tsv" "$T/tests/test_slowish.sh" "$T/tests/test_vanish.sh" 2>&1 )"; vbrc=$?
assert "missing result outranks an advisory breach"  '[ "$vbrc" = "3" ]'
assert "incomplete run is NOT told it exits 0"       '! grep -qi "does not fail" <<<"$vbout"'

# (11) usage error
bash "$RT" --bogus-flag >/dev/null 2>&1
assert "unknown flag exits 2" '[ "$?" = "2" ]'

# (12) COLLIDING BASENAMES are a usage error, for the same reason a nonexistent target is one.
# Logs, stat records and budget rows are keyed on basename, so two targets sharing one basename put
# two concurrent jobs on the same $WORK/logs/<base>.log and $WORK/stat/<base>: interleaved logs,
# doubled assert counts, a double-printed row. Unlike a mistyped path, nothing about that LOOKS
# wrong in the output — which is why it has to be refused before any job launches.
mkdir -p "$T/other"
cp "$T/tests/test_alpha.sh" "$T/other/test_alpha.sh"
dout="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/other/test_alpha.sh" 2>&1)"; drc=$?
assert "colliding basenames are a usage error (exit 2)" '[ "$drc" = "2" ]'
assert "the collision names the colliding basename"     'grep -q "duplicate test basename: test_alpha.sh" <<<"$dout"'
assert "the collision names the FIRST path"             'grep -q "tests/test_alpha.sh" <<<"$dout"'
assert "the collision names the SECOND path"            'grep -q "other/test_alpha.sh" <<<"$dout"'
assert "the collision launches no job at all"           '! grep -q "^SUITE " <<<"$dout"'
# The same path passed twice is the same collision, and is the likelier way to hit it.
bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/tests/test_alpha.sh" >/dev/null 2>&1; drc2=$?
assert "the same path passed twice is a usage error"    '[ "$drc2" = "2" ]'
# Negative control: the guard must key on the BASENAME colliding, not on two targets sharing a
# directory or on any two-target run — otherwise the asserts above would pass for the wrong reason.
cp "$T/tests/test_beta.sh" "$T/other/test_gamma.sh"
nout12="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" "$T/other/test_gamma.sh" 2>&1)"; nrc12=$?
assert "distinct basenames across directories still run" '[ "$nrc12" = "0" ]'
assert "distinct basenames report both files"            'grep -qE "^SUITE files=2 passed=2 " <<<"$nout12"'

# (13) INTERRUPTION must reap its jobs and say what was lost, not orphan them and delete $WORK.
# Output is buffered until every job finishes, so an interrupted run has printed nothing; the EXIT
# trap alone would remove $WORK while live jobs are still writing into it, and the jobs themselves
# SURVIVE — a runner with no job control gives every async child SIGINT ignored, so Ctrl-C reaches
# the runner and nothing else. The fixture publishes its own pid and then blocks; after the signal
# that pid must be gone.
cat > "$T/tests/test_hang.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$$" > "$HANGPID"
sleep 30
echo "ok - hang finished"
exit 0
EOF
chmod +x "$T/tests/test_hang.sh"
: > "$T/hangpid"
HANGPID="$T/hangpid" bash "$RT" -j 2 "$T/tests/test_hang.sh" > "$T/int.out" 2> "$T/int.err" &
rtpid=$!
# Poll rather than sleep a fixed interval: the wait is a few milliseconds in practice, and this
# file is itself in the budget table.
n=0; while [ ! -s "$T/hangpid" ] && [ "$n" -lt 100 ]; do sleep 0.1; n=$((n + 1)); done
hpid="$(cat "$T/hangpid" 2>/dev/null)"
kill -TERM "$rtpid" 2>/dev/null
wait "$rtpid"; ircx=$?
n=0; while kill -0 "$hpid" 2>/dev/null && [ "$n" -lt 100 ]; do sleep 0.1; n=$((n + 1)); done
assert "an interrupted run leaves no orphaned test process" '[ -n "$hpid" ] && ! kill -0 "$hpid" 2>/dev/null'
assert "an interrupted run says it was interrupted"         'grep -qi "interrupted" "$T/int.err"'
assert "the interrupt report says what was lost"            'grep -qE "0 of 1 test files had finished" "$T/int.err"'
assert "an interrupted run exits non-zero"                  '[ "$ircx" != "0" ]'
assert "an interrupted run prints no SUITE line"            '! grep -q "^SUITE " "$T/int.out"'
# Negative control: the handler must fire on a SIGNAL, not on every run — an uninterrupted run of
# the same shape still reports normally and exits 0.
cout13="$(bash "$RT" -j 2 "$T/tests/test_alpha.sh" 2>&1)"; crc13=$?
assert "an uninterrupted run is not told it was interrupted" '! grep -qi "interrupted" <<<"$cout13"'
assert "an uninterrupted run still exits 0"                  '[ "$crc13" = "0" ]'

rm -rf "$T"
exit $fail
