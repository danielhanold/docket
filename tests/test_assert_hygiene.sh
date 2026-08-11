#!/usr/bin/env bash
# tests/test_assert_hygiene.sh — the regression test for scripts/check-test-source-hygiene.sh
# (change 0221). It drives the checker against the committed fixtures under tests/fixtures/hygiene/
# in BOTH directions: every red fixture must be flagged WITH ITS EXPECTED CLASS TOKEN, and every
# green fixture must go unflagged. Zero false positives establishes nothing about false negatives,
# which is why both halves are here.
#
# THREE DISCIPLINES THIS FILE IS BUILT AROUND:
#
#  1. ASSERT THE CLASS, NOT THE EXIT CODE. "It failed" is satisfied by every unrelated way of
#     failing — a typo in a path, an awk error, an unrelated file's drift. tests/fixtures/hygiene/
#     red/heredoc_plain.sh must report HEREDOC-BACKTICK specifically, so the mechanism is pinned
#     and not merely the verdict.
#
#  2. KEY EVERY PER-FIXTURE VERDICT ON LINES NAMING THAT FIXTURE, never on the bare exit code.
#     Rule (a) sweeps the whole tests/ tree on EVERY invocation, independent of the paths handed in
#     (scripts/check-test-source-hygiene.md, "Rule (a) makes every invocation report the whole
#     tree's definition drift"). A green fixture's verdict read off rc alone would therefore redden
#     the moment any unrelated test file drifts, and would silently pass for the wrong reason if a
#     red fixture stopped being detected while some other file kept the rc at 1.
#
#  3. NON-VACUITY FIRST. The fixture roster is a GLOB, never a hand-list — a hand-list goes stale
#     the day a fixture is added, and the new fixture is then unguarded (AGENTS.md § Guards). But a
#     glob that matches nothing expands to nothing, every loop below runs zero times, and the file
#     prints a confident all-green. So the discovered counts are asserted non-zero, every red
#     fixture must carry a declared expected class (an undeclared one reddens rather than being
#     skipped), and the visit counters are asserted against the discovered counts.
#
#  4. A HAZARD IS PINNED IN THE SPELLING THE SUITE ACTUALLY WRITES, not only the compact one. Every
#     red fixture here once put its hazard on ONE physical line, while backslash-continuation was
#     covered by tests/fixtures/hygiene/green/continuation.sh alone — and a GREEN fixture proves
#     only that the machine does not false-positive across a continuation, never that it still
#     DETECTS across one. It did not: the scanner read the spliced newline as an escape of the next
#     character, which swallowed the house indent and disarmed the eval rule on the majority of the
#     suite, and at column zero swallowed an opening quote and ran the lexer inverted to end of
#     file. Both went green through a documented mutation probe. The red half that asymmetry was
#     missing is now tests/fixtures/hygiene/red/continuation_eval.sh (indented, the 55% case) and
#     tests/fixtures/hygiene/red/continuation_dq.sh (column zero, the inversion case) — the missed
#     spelling was the target tree's own house idiom (AGENTS.md § Guards and tests).
#
# The fixtures are hazardous BY CONSTRUCTION and live outside the runner's tests/test_*.sh glob so
# they are never launched as tests. Nothing in this file sources or executes one; the checker reads
# their bytes, and tests/fixtures/hygiene/red/sentinel.sh pins that — it writes a marker file if
# anything ever runs it, and the sentinel block below asserts the marker is absent after a scan
# that flagged the file. Detection that requires execution is not prevention.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

CHECKER="$REPO/scripts/check-test-source-hygiene.sh"
FIXTURE_ROOT="$REPO/tests/fixtures/hygiene"

# Absolute, and derived from mktemp with a TEMPLATE: bare mktemp ignores TMPDIR on macOS
# (AGENTS.md § Shell), and an rm -rf of a relative path is a data-loss shape.
tmp="$(mktemp -d "${TMPDIR:-/tmp}/assert-hygiene.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

# The expected violation class per red fixture. This table is a VERDICT declaration, not the
# roster: the roster is the glob below, and a red fixture missing from this table reddens (the
# "declares an expected class" assert). So adding a fixture cannot silently skip it.
EXPECTED_CLASSES='
brace_length_expansion.sh DQ-BACKTICK
continuation_dq.sh DQ-BACKTICK
continuation_eval.sh EVAL-BACKTICK
defn_echo.sh DEFN-DRIFT
defn_function_kw.sh DEFN-DRIFT
defn_multiline.sh DEFN-DRIFT
defn_spaced.sh DEFN-DRIFT
dq_condition_escaped.sh DQ-BACKTICK
dq_description.sh DQ-BACKTICK
dq_sites_block.sh DQ-BACKTICK
heredoc_dash.sh HEREDOC-BACKTICK
heredoc_plain.sh HEREDOC-BACKTICK
normal_substitution.sh NORMAL-BACKTICK
sentinel.sh DQ-BACKTICK
sq_condition_unescaped.sh EVAL-BACKTICK
'

assert "the checker exists and is readable" '[ -r "$CHECKER" ]'

# --- the roster ----------------------------------------------------------------------------------
# nullglob so an empty directory yields an EMPTY array rather than the literal pattern: the count
# assertions below are the non-vacuity floor, and they can only do their job if a miss is a zero.
shopt -s nullglob
red_files=("$FIXTURE_ROOT"/red/*.sh)
green_files=("$FIXTURE_ROOT"/green/*.sh)
shopt -u nullglob

assert "the red fixture glob discovered at least one file (a vacuous glob greens every assert below)" \
  '[ "${#red_files[@]}" -gt 0 ]'
assert "the green fixture glob discovered at least one file" \
  '[ "${#green_files[@]}" -gt 0 ]'

# --- one scan for all reds, one for all greens ---------------------------------------------------
# The checker takes many paths and prefixes every violation with the path it came from, so batching
# costs one process instead of eighteen WITHOUT weakening any per-fixture verdict: each verdict
# below still reads only the lines naming its own fixture.
red_out="$(bash "$CHECKER" "${red_files[@]}" 2>"$tmp/red.err")"
red_rc=$?
green_out="$(bash "$CHECKER" "${green_files[@]}" 2>"$tmp/green.err")"
green_rc=$?

assert "the red batch exits 1 (violations found)" '[ "$red_rc" -eq 1 ]'
assert "the red batch wrote nothing to stderr" '[ ! -s "$tmp/red.err" ]'
assert "the green batch wrote nothing to stderr" '[ ! -s "$tmp/green.err" ]'
# rc on the green batch is deliberately NOT asserted: rule (a) sweeps tests/ on every invocation,
# so this rc reports the whole tree, not these six files. The per-fixture asserts below carry the
# verdict. The rc is still recorded, so a run that dies before printing is not read as clean.
assert "the green batch ran to completion (rc is 0 or 1, never a usage or crash code)" \
  '[ "$green_rc" -eq 0 ] || [ "$green_rc" -eq 1 ]'

# --- red half: flagged, and flagged with the RIGHT class ------------------------------------------
red_visited=0
for f in "${red_files[@]}"; do
  base="${f##*/}"
  want="$(awk -v n="$base" '$1 == n { print $2 }' <<<"$EXPECTED_CLASSES")"
  assert "red fixture $base declares an expected class (an undeclared fixture is not silently skipped)" \
    '[ -n "$want" ]'
  [ -n "$want" ] || continue
  hit_lines="$(awk -v p="$f:" 'index($0, p) == 1' <<<"$red_out")"
  class_lines="$(awk -v p="$f:" -v c=" $want: " 'index($0, p) == 1 && index($0, c) > 0' <<<"$red_out")"
  assert "red fixture $base is flagged, on a line naming the file itself" '[ -n "$hit_lines" ]'
  assert "red fixture $base reports $want specifically, not merely a nonzero exit" \
    '[ -n "$class_lines" ]'
  red_visited=$((red_visited + 1))
done
assert "every discovered red fixture was visited (${#red_files[@]} of them)" \
  '[ "$red_visited" -eq "${#red_files[@]}" ]'

# --- green half: not a single line names them -----------------------------------------------------
green_visited=0
for f in "${green_files[@]}"; do
  base="${f##*/}"
  hit_lines="$(awk -v p="$f:" 'index($0, p) == 1' <<<"$green_out")"
  assert "green fixture $base is not flagged by any class" '[ -z "$hit_lines" ]'
  green_visited=$((green_visited + 1))
done
assert "every discovered green fixture was visited (${#green_files[@]} of them)" \
  '[ "$green_visited" -eq "${#green_files[@]}" ]'

# --- the side-effect sentinel ---------------------------------------------------------------------
# Both halves matter and neither alone proves anything: a checker that executed the fixture would
# ALSO flag it, and a checker that read nothing at all would also leave the marker absent.
SENTINEL_DIR="$tmp/sentinel"
mkdir -p "$SENTINEL_DIR"
sentinel="$FIXTURE_ROOT/red/sentinel.sh"
assert "the sentinel fixture is present" '[ -r "$sentinel" ]'
sentinel_out="$(HYGIENE_SENTINEL_DIR="$SENTINEL_DIR" bash "$CHECKER" "$sentinel" 2>"$tmp/sentinel.err")"
sentinel_lines="$(awk -v p="$sentinel:" 'index($0, p) == 1' <<<"$sentinel_out")"
assert "the sentinel fixture is flagged when scanned by name" '[ -n "$sentinel_lines" ]'
assert "scanning the sentinel wrote no marker file — detection did not require execution" \
  '[ ! -e "$SENTINEL_DIR/EXECUTED" ]'
assert "scanning the sentinel left its scratch dir completely empty" \
  '[ -z "$(ls -A "$SENTINEL_DIR")" ]'

# --- the usage contract ----------------------------------------------------------------------------
bash "$CHECKER" >/dev/null 2>&1
noargs_rc=$?
bash "$CHECKER" "$tmp/does-not-exist.sh" >/dev/null 2>&1
missing_rc=$?
assert "the checker exits 2 on a usage error (no paths given)" '[ "$noargs_rc" -eq 2 ]'
assert "the checker exits 2 on an unreadable path" '[ "$missing_rc" -eq 2 ]'

exit $fail
