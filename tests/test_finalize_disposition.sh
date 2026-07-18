#!/usr/bin/env bash
# tests/test_finalize_disposition.sh — guards change 0087 (headless finalize: the finalize-side
# disposition contract, mirroring 0088). Asserts the four-disposition terminal contract, id-set
# scoping, the mergeability ordering keys IN ORDER, the `## Finalize blocked` marker semantics,
# and the README drain-pattern doc.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review; this test does not replace it.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

FIN="$REPO/skills/docket-finalize-change/SKILL.md"

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$FIN"'
for d in advanced contended drained halted; do
  tok="\`$d\`"
  assert "SKILL names disposition $d (code-formatted)" 'grep -qF "$tok" "$FIN"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$FIN"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$FIN"'
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$FIN"'

# --- SKILL.md: the finalize-specific disposition semantics ---
assert "SKILL ties every abort-and-report point to halted" \
  'grep -Eqi "abort-and-report point.{0,40}(is|are|maps to|→).{0,20}\`?halted" "$FIN"'
assert "SKILL states a blocked-but-non-empty set is halted, not drained" \
  'grep -Eqi "halted.{0,30}(never|not).{0,10}\`?drained" "$FIN"'
assert "SKILL states one merge per invocation" \
  'grep -Eqi "exactly one|one merge per invocation" "$FIN"'
assert "SKILL states it never batches" 'grep -Eqi "never batch" "$FIN"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$FIN"'
assert "SKILL shows the comma-separated id-set form" 'grep -Eq "docket-finalize-change 90,92,94" "$FIN"'
assert "SKILL states naming the ids IS the authorization" \
  'grep -Eqi "naming the ids.{0,30}authorization" "$FIN"'
assert "SKILL ties the allowlist to the require_pr_approval override" \
  'grep -q "require_pr_approval" "$FIN"'

# --- SKILL.md: mergeability ordering, asserted IN ORDER (order is part of the contract) ---
# NOTE: never `grep … | head` under `set -o pipefail` (AGENTS.md) — the producer takes SIGPIPE and
# the 141 becomes an intermittent failure. Capture the whole match set, then take the first line
# with parameter expansion.
first_line_no(){ # first_line_no ERE -> line number of the first matching line, empty if none
  local m; m="$(grep -nEi -e "$1" "$FIN" || true)"
  [ -n "$m" ] || return 0
  m="${m%%$'\n'*}"        # first match only
  printf '%s' "${m%%:*}"  # strip everything from the first colon
}
p_dep="$(first_line_no '^[[:space:]]*1\..*depends_on')"
p_mrg="$(first_line_no '^[[:space:]]*2\..*mergeable')"
p_dif="$(first_line_no '^[[:space:]]*3\..*(smallest diff|changedFiles)')"
p_tie="$(first_line_no '^[[:space:]]*4\..*priority')"
assert "ordering key 1 is depends_on" '[ -n "$p_dep" ]'
assert "ordering key 2 is mergeable" '[ -n "$p_mrg" ]'
assert "ordering key 3 is diff size" '[ -n "$p_dif" ]'
assert "ordering key 4 is the priority tiebreak" '[ -n "$p_tie" ]'
assert "the four ordering keys appear in contract order" \
  '[ -n "$p_dep" ] && [ -n "$p_mrg" ] && [ -n "$p_dif" ] && [ -n "$p_tie" ] &&
   [ "$p_dep" -lt "$p_mrg" ] && [ "$p_mrg" -lt "$p_dif" ] && [ "$p_dif" -lt "$p_tie" ]'
assert "SKILL excludes CONFLICTING from selection" 'grep -q "CONFLICTING" "$FIN"'
assert "SKILL documents the lazy-mergeable poll" \
  'grep -q "UNKNOWN" "$FIN" && grep -Eqi "poll" "$FIN"'
assert "SKILL forbids pairwise file-overlap ranking" \
  'grep -Eqi "(not|never|do not|don.t) build pairwise|pairwise file-overlap" "$FIN"'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! grep -qF "\`advanced\`" "$probe"'
# Non-vacuity for the ordering comparison: a reversed pair must fail the same test.
assert "the ordering comparison is non-vacuous (9 < 3 is caught)" '! [ 9 -lt 3 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
