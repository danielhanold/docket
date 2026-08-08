#!/usr/bin/env bash
# tests/test_runtime_budgets.sh — regrowth guard (change 0227): every tests/test_*.sh carries a
# wall-clock budget row, no row exceeds the 60s ceiling, the serial pin-list is budgeted with its
# own counter, the ceilings sum to a pinned total, and the merge gate still reports breaches.
#
# WHY SEPARATE COUNTERS. A count-based guard whose remedy is "make the number agree" teaches the
# evasion it exists to catch (repo learning: guard-remedy-must-not-teach-the-evasion). Four paths
# grant a file relief from the discipline, and each is asserted on its own, so that taking any one
# of them leaves the completeness assertion green and reddens only its own counter:
#
#   A  a ceiling ABOVE the hard 60s limit                       — assertion (3)
#   B  a `serial` pin, removing the file from the parallel phase — assertion (4)
#   C  a ceiling raised anywhere BELOW the limit — the quiet one: a row moved 35 -> 60 breaks no
#      ceiling and pins nothing serial, yet buys 62s MORE measured slack under the 5/2 factor.
#      The table's TOTAL is pinned instead, so any raise moves it                — assertion (5)
#   D  the wiring site: disarming the budget report at the merge gate, which leaves every
#      assertion about the table green and the table itself decoration            — assertion (7)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

TBL="$REPO/tests/runtime-budgets.tsv"
CEILING=60          # the hard ceiling; no row may exceed it
EXPECTED_SERIAL=0   # files pinned serial by the change-0227 audit. RAISING THIS IS A FINDING:
                    # a serial pin removes a file from the parallel phase, so it must be justified
                    # in the same diff with the shared state that forced it.
EXPECTED_TOTAL=1415 # the sum of every ceiling, seeded with the table from the measured serial run.
                    # 1405 -> 1415 (change 0244): the new-test-file case named below —
                    # tests/test_frontmatter_read_shapes.sh brings its own row, floored to the
                    # table's 10s minimum.
                    # 1365 -> 1405 (change 0255): the new-test-file case named below —
                    # tests/test_harness_defaults_flow_map.sh brings its own row, measured
                    # standalone at 9.8s and sized to 40s to cover the `#`-leg probes task 2
                    # appends to the same file.
                    # 1355 -> 1365 (change 0254): the new-test-file case named below —
                    # tests/test_bsd_tool_defaults.sh brings its own row, floored to the table's
                    # 10s minimum.
                    # 1345 -> 1355 (change 0237): the new-test-file case named below —
                    # tests/test_verify_run.sh brings its own row, measured serially at 1.4s,
                    # floored to the table's 10s minimum.
                    # 1325 -> 1335 (change 0223): the new-test-file case named below —
                    # tests/test_gate_execution_posture.sh brings its own row, measured serially at
                    # 1s (scripts/run-tests.sh -j 1), floored to the table's 10s minimum.
                    # 1335 -> 1345 (change 0190): the same case again —
                    # tests/test_skip_allowlist_invisibility.sh brings its own row, measured at
                    # 0.4s standalone, floored to the table's 10s minimum.
                    # It is the whole suite's budget, and it moves only for a reason that is stated
                    # in the diff that moves it: a new test file brings its own row, and re-cutting
                    # a sharded file's rows redistributes cost that has to be re-measured.

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
# file's ceiling upward leaves (2) green and reddens only this.
over="$(awk -F'\t' -v c="$CEILING" '$2 > c {print $1}' <<<"$rows")"
assert "no budget row exceeds the ${CEILING}s ceiling" \
  '[ -z "$over" ] || { echo "  over ceiling: $over" >&2; echo "  Shard the file or move its new assertions into a shard with room. Raising the ceiling is not the remedy." >&2; false; }'

# (4) RELIEF COUNTER B — files pinned serial, budgeted exactly. Also independent of (2).
serial_n="$(awk -F'\t' '$3 == "serial" {n++} END{print n+0}' <<<"$rows")"
assert "exactly $EXPECTED_SERIAL files are pinned serial" \
  '[ "$serial_n" = "$EXPECTED_SERIAL" ] || { echo "  serial rows: $(awk -F"\t" "\$3==\"serial\"{print \$1}" <<<"$rows" | tr "\n" " ")" >&2; echo "  A serial pin removes a file from the parallel phase. Name the shared state that forces it, in this diff." >&2; false; }'

# (5) RELIEF COUNTER C — the table's TOTAL. Counters A and B only see the two loud reliefs; a row
# raised from 35 to 60 trips neither, and completeness never looks at column 2 at all. The sum sees
# every raise, however small, and reddens on it alone.
total="$(awk -F'\t' '{s += $2} END {print s+0}' <<<"$rows")"
assert "the table's budgeted total is $EXPECTED_TOTAL seconds" \
  '[ "$total" = "$EXPECTED_TOTAL" ] || { echo "  budgeted total: ${total}s across $(wc -l <<<"$rows" | tr -d " ") rows, pinned at ${EXPECTED_TOTAL}s" >&2; echo "  A ceiling describes what a file already costs, so it moves when the file is re-shaped, not when it gets slower: shard the file, or move its new assertions into a shard with room." >&2; echo "  The total legitimately moves in two cases, and both belong in this diff: a new test file brings its own row, and re-cutting a sharded file redistributes its cost. Re-seed from a measured serial run (scripts/run-tests.sh -j 1 --timings PATH) and say which case it was." >&2; false; }'

# (6) the runner actually READS this table by default — otherwise the whole table is decoration
assert "run-tests.sh defaults to tests/runtime-budgets.tsv" \
  'grep -q "tests/runtime-budgets.tsv" "$REPO/scripts/run-tests.sh"'

# (7) RELIEF PATH D — THE WIRING SITE. (6) proves the runner reads the table when it runs; it says
# nothing about the command the merge gate actually runs. Since a breach became advisory, that gate
# no longer turns red on one — the `OVER BUDGET:` REPORT is the whole remaining channel (see
# scripts/run-tests.md, "Why a breach is advisory by default"). So what has to hold is that the
# configured command still emits that report: `--no-budget-check` measures no breach and prints
# none, and would leave every assertion above green over a table nothing reads.
#
# Resolution mirrors scripts/docket-config.sh's precedence — repo-local `.docket.local.yml` over
# committed `.docket.yml`. The machine-global layer is deliberately NOT consulted: a test that
# reads the developer's $HOME is the shape the change-0227 parallel-safety audit forbids.
test_command_of(){   # block-scoped awk, the tests/test_finalize_gate.sh idiom
  awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f && /^[^[:space:]#]/{f=0}
    f && /^[[:space:]]+test_command[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*test_command[[:space:]]*:[[:space:]]*/,"",line);
      sub(/[[:space:]]+$/,"",line); print line; exit
    }' "$1" 2>/dev/null
}
TESTCMD="$(test_command_of "$REPO/.docket.local.yml")"
[ -n "$TESTCMD" ] || TESTCMD="$(test_command_of "$REPO/.docket.yml")"

assert "the resolved finalize.test_command runs the budget-reporting runner" \
  'case "$TESTCMD" in *run-tests.sh*) true ;; *) echo "  resolved finalize.test_command: ${TESTCMD:-<unset>}" >&2; echo "  The merge gate is the one run whose output is always read, so it is where a budget breach gets seen. A command that does not run scripts/run-tests.sh reports no budgets at all; point the gate back at the runner, or record in this diff what carries the report instead." >&2; false ;; esac'

assert "the resolved finalize.test_command does not suppress the budget report" \
  'case "$TESTCMD" in *--no-budget-check*) echo "  resolved finalize.test_command: $TESTCMD" >&2; echo "  --no-budget-check measures no breach and prints none, which retires this whole table at the one run everybody reads. If a file is breaching, shard it or move its new assertions into a shard with room; --no-budget-check belongs in measurement runs, where enforcing a ceiling against the number being measured is circular." >&2; false ;; *) true ;; esac'

# (8) tests/README.md exists and tells the reader where new tests go
assert "tests/README.md exists"          '[ -f "$REPO/tests/README.md" ]'
assert "tests/README.md says where new tests go" \
  'grep -qi "where new tests go" "$REPO/tests/README.md"'

exit $fail
