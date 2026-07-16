#!/usr/bin/env bash
# tests/test_terminal_publish.sh — arg-validation guards for terminal-publish.sh. The --id/--adr
# integer guard fires at parse time, before any git work, so these need no repo. (change 0032)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/terminal-publish.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

err="$(bash "$SCRIPT" --id abc 2>&1)"; rc=$?
assert "--id abc exits non-zero"        '[ "$rc" -ne 0 ]'
assert "--id abc diagnostic names id"   'printf "%s" "$err" | grep -qiE "id"'

err="$(bash "$SCRIPT" --adr 1.5 2>&1)"; rc=$?
assert "--adr 1.5 exits non-zero"       '[ "$rc" -ne 0 ]'

# a valid integer id passes the int-guard (it dies later on a DIFFERENT, missing-arg error)
err="$(bash "$SCRIPT" --id 5 2>&1)"; rc=$?
assert "--id 5 passes the int guard"    '[ "$rc" -ne 0 ] && ! printf "%s" "$err" | grep -qi "non-integer"'

# --- change 0084: the --enabled contract ------------------------------------------------------
# Publish is opt-in. An OMITTED --enabled is a caller bug rather than a decision, so it no-ops
# LOUDLY; an explicit `--enabled false` is a decision, so it stays silent. Exit 0 on both paths:
# callers trust the exit code and a missing flag must never abort a close-out — the WARNING, not a
# non-zero exit, is what keeps a skipped publish from hiding (the #0043 silent-gap failure mode).
# The arg/mode/knob guards all run before any git work, so these need no repo fixture.
pub_args=(--id 5 --outcome done --integration-branch main --metadata-branch docket
          --changes-dir docs/changes --adrs-dir docs/adrs)

err="$(bash "$SCRIPT" "${pub_args[@]}" 2>&1)"; rc=$?
assert "omitted --enabled exits zero (never aborts a close-out)" '[ "$rc" -eq 0 ]'
assert "omitted --enabled warns on stderr"                       'printf "%s" "$err" | grep -q "WARNING"'
assert "omitted --enabled says NOTHING was published"            'printf "%s" "$err" | grep -qi "nothing was published"'
assert "omitted --enabled names the fix (--enabled true)"        'printf "%s" "$err" | grep -q -- "--enabled true"'

err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled false 2>&1)"; rc=$?
assert "explicit --enabled false exits zero"                     '[ "$rc" -eq 0 ]'
assert "explicit --enabled false is SILENT (no WARNING)"          '! printf "%s" "$err" | grep -q "WARNING"'
assert "explicit --enabled false logs the suppression"           'printf "%s" "$err" | grep -q "terminal_publish: false"'

err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled maybe 2>&1)"; rc=$?
assert "invalid --enabled exits non-zero"                        '[ "$rc" -ne 0 ]'
assert "invalid --enabled diagnostic names the value"            'printf "%s" "$err" | grep -q "maybe"'

# an explicit EMPTY value stays fail-closed — it must not be mistaken for an omitted flag
err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled "" 2>&1)"; rc=$?
assert "empty --enabled exits non-zero (not treated as omitted)" '[ "$rc" -ne 0 ]'
assert "empty --enabled does NOT warn"                           '! printf "%s" "$err" | grep -q "WARNING"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
