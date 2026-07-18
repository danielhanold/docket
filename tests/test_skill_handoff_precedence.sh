#!/usr/bin/env bash
# tests/test_skill_handoff_precedence.sh — change 0096.
# Guards the autonomy-precedence contract: an invoked role skill's interactive hand-off must never
# halt an autonomous run. Two groups:
#   (1) docket-convention's *Skill layer* states the precedence rule and names the call-site marker.
#   (2) COVERAGE — every autonomous invocation of a resolved role skill pre-specifies its outcome
#       (`DIRECTED to:`), with docket-finalize-change's human-present close-out the one exception.
# Group (2) is the load-bearing one: a presence-only check would test the durability prose (which
# demonstrably does not win at the moment of invocation) while leaving the mechanism unguarded.
# Sites are DERIVED from a whole-repo grep, never hand-listed (AGENTS.md: enumerated floor).
# Run: bash tests/test_skill_handoff_precedence.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The literal marker a call site uses to pre-specify its outcome. One house token, documented in the
# convention — a shape the guard can actually check, not an open-ended paraphrase.
MARKER='DIRECTED to:'

# --- group 1: the convention states the rule ----------------------------------------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention SKILL.md exists" '[ -f "$CONV" ]'
# Scope to the *Skill layer* section so a stray mention elsewhere cannot satisfy these.
LAYER="$(awk '/^### Skill layer/{f=1;next} f&&/^### /{exit} f' "$CONV")"
assert "the Skill layer section is non-empty" '[ -n "$LAYER" ]'
assert "Skill layer states an invoked skill never outranks the caller" 'grep -qi "never outranks" <<<"$LAYER"'
assert "Skill layer names pre-specification as the mechanism" 'grep -qi "pre-specif" <<<"$LAYER"'
assert "Skill layer names the call-site marker" 'grep -qF -- "$MARKER" <<<"$LAYER"'

# --- group 2: coverage over every autonomous role invocation ------------------------------------
# A skill is AUTONOMOUS iff sync-agents.sh generates a wrapper for it (agents/<skill>.md) — the same
# wrapper that carries the abort-and-report rule. Interactive skills have no wrapper and are skipped
# by construction, never by name: their prompts are the product.
SITES="$(grep -rn -e '\$SKILL_[A-Z]\{4,\}' "$REPO/skills" 2>/dev/null)"
assert "role-skill invocation sites were discovered" '[ -n "$SITES" ]'

checked=0
exceptions=0
while IFS= read -r entry; do
  [ -n "$entry" ] || continue
  file="${entry%%:*}"; rest="${entry#*:}"; lno="${rest%%:*}"; text="${rest#*:}"
  skill="$(basename "$(dirname "$file")")"
  # Skip interactive skills (no generated wrapper).
  [ -f "$REPO/agents/$skill.md" ] || continue
  rel="${file#"$REPO"/}"
  checked=$((checked+1))
  if grep -qi "human is present" <<<"$text"; then
    exceptions=$((exceptions+1))
    assert "$rel:$lno human-present exception belongs to docket-finalize-change" \
      '[ "$skill" = "docket-finalize-change" ]'
    continue
  fi
  assert "$rel:$lno autonomous role invocation pre-specifies its outcome" \
    'grep -qF -- "$MARKER" <<<"$text"'
done <<<"$SITES"

# The classifier must not go vacuous: if wrapper detection broke, every site would be skipped and
# every assert above would silently vanish.
assert "autonomous role invocations were actually checked (checked=$checked >= 5)" '[ "$checked" -ge 5 ]'
assert "exactly one human-present exception exists (found $exceptions)" '[ "$exceptions" -eq 1 ]'

# --- non-vacuity / mutation proof ---------------------------------------------------------------
# The marker check must reject an unmarked invocation line — the exact shape of today's defective §4.
UNMARKED='Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export.'
assert "the marker check is non-vacuous (an unmarked invocation is caught)" \
  '! grep -qF -- "$MARKER" <<<"$UNMARKED"'
# The exception classifier must not match an ordinary invocation line.
assert "the exception classifier is non-vacuous (a plain line is not an exception)" \
  '! grep -qi "human is present" <<<"$UNMARKED"'

[ "$fail" -eq 0 ] && echo "ALL OK" || echo "FAILURES"
exit "$fail"
