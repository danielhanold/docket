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
# file's ceiling upward leaves (2) green and reddens only this.
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
