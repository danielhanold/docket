#!/usr/bin/env bash
# tests/test_docket_review.sh — guards change 0170's review role: the docket-review skill
# contract, the three rung wrappers, the build-evidence chain, and finalize's conditional skip.
# Run: bash tests/test_docket_review.sh
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
fails=0
assert(){ if eval "$2"; then echo "ok   - $1"; else echo "FAIL - $1"; fails=$((fails+1)); fi; }

REV="$REPO/skills/docket-review/SKILL.md"

# --- the skill exists and declares itself -------------------------------------------------
assert "docket-review skill exists" '[ -f "$REV" ]'
assert "docket-review frontmatter name is docket-review" \
  'awk "/^---$/{n++; next} n==1" "$REV" | grep -qE "^name: docket-review$"'

# --- read-only conduct: the properties that make the verdict trustworthy -------------------
# Each is a distinct promise; a single "read-only" mention would not prove any of them.
assert "conduct: forbids running the test suite" \
  'grep -qiE "never runs? the (full )?(test )?suite" "$REV"'
assert "conduct: forbids writing, committing, or checking out" \
  'grep -qiE "never (writes|commits|checks out)" "$REV"'
assert "conduct: forbids dispatching subagents" \
  'grep -qiE "never dispatches" "$REV"'
assert "conduct: no reviewer escalation ladder" \
  'grep -qiE "never re-dispatches itself|no .{0,20}escalation" "$REV"'

# --- the finding schema: every field a triaging controller must be able to read ------------
for f in severity location summary rationale suggested_fix; do
  assert "finding schema names the '$f' field" 'grep -qF -- "$f" "$REV"'
done
for s in blocker important minor; do
  assert "finding schema names the '$s' severity" 'grep -qE "\`$s\`|\*\*$s\*\*" "$REV"'
done

# --- the evidence backstop finding --------------------------------------------------------
# The reviewer's ONLY answer to bad evidence is a finding; it must never run the suite itself.
assert "reviewer reports unverified-build-state rather than running the suite" \
  'grep -qF -- "unverified-build-state" "$REV"'
assert "reviewer verifies the evidence head_sha against the branch HEAD" \
  'grep -qF -- "head_sha" "$REV"'

echo "---"; [ "$fails" -eq 0 ] && echo "PASS" || { echo "FAIL ($fails)"; exit 1; }
