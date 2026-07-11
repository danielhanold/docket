#!/usr/bin/env bash
# tests/test_consultant_brainstorm.sh — verifies change 0056 Task 2: the docket-brainstorm skill
# implements the single-dispatch consultant-author flow (spec §3) with a degrade rule (ADR-0018).
# Run: bash tests/test_consultant_brainstorm.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$REPO/skills/docket-brainstorm/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "docket-brainstorm skill exists" '[ -f "$SKILL" ]'
assert "dispatches the pinned consultant" 'grep -q "docket-brainstorm-consultant" "$SKILL"'
assert "single dispatch — no SendMessage/continuation" '! grep -qi "SendMessage" "$SKILL" && ! grep -qi "continuation" "$SKILL"'
assert "author-or-critique gate documented" 'grep -qi "critique" "$SKILL" && grep -qi "author" "$SKILL"'
assert "in-context return contract" 'grep -qiE "in-context|in context" "$SKILL"'
assert "stops at the spec (0049 role contract)" 'grep -qi "stop" "$SKILL" && grep -qi "spec" "$SKILL"'
assert "degrade rule: inline + warn when undispatchable" 'grep -qiE "degrade|undispatchable|cannot be dispatched" "$SKILL" && grep -qi "warn" "$SKILL"'
assert "respects ADR-0006 — no simulated human / real dialogue inline" 'grep -qiE "real human|no.{0,4}simulat|inline" "$SKILL"'
assert "Convention load-first block present" 'grep -qF "## Convention (load first — blocking)" "$SKILL" || grep -qi "docket-convention" "$SKILL"'

NC="$REPO/skills/docket-new-change/SKILL.md"; GN="$REPO/skills/docket-groom-next/SKILL.md"
assert "new-change notes the consultant verbal opt-in" 'grep -q "docket-brainstorm" "$NC"'
assert "groom-next notes the consultant verbal opt-in" 'grep -q "docket-brainstorm" "$GN"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
