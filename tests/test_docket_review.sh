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

# --- the three rung wrappers ---------------------------------------------------------------
HD="$REPO/agents/harness-defaults.yml"
REV_DESC="$(awk "/^---$/{n++; next} n==1" "$REV" | sed -n 's/^description: //p')"
assert "the skill's description is non-empty (anchor for the wrapper compare)" '[ -n "$REV_DESC" ]'

for rung in lean standard deep; do
  W="$REPO/agents/docket-review-$rung.md"
  assert "wrapper exists: docket-review-$rung" '[ -f "$W" ]'
  [ -f "$W" ] || continue
  assert "docket-review-$rung: name matches its filename" \
    'grep -qE "^name: docket-review-'"$rung"'$" "$W"'
  # Byte-equality with the skill's own description is the house rule for wrappers.
  assert "docket-review-$rung: description matches the skill's" \
    'wd="$(sed -n "s/^description: //p" "$W")"; [ "$wd" = "$REV_DESC" ]'
  # The wrapper wraps the review skill ONLY — no docket-convention, mirroring the build workers.
  assert "docket-review-$rung: injects docket-review" 'grep -qF -- "skills: [docket-review]" "$W"'
  assert "docket-review-$rung: does NOT inject docket-convention" \
    '! grep -qF -- "docket-convention" "$W"'
  assert "docket-review-$rung: carries the abort-and-report posture" \
    'grep -qF -- "abort-and-report" "$W"'
  # No pins live in wrapper files since change 0168 — they live in the sidecar.
  assert "docket-review-$rung: carries no model/effort pin" \
    '! grep -qE "^(model|effort):" "$W"'
  # Every shipped harness must supply a pair, or generation fails outright.
  for h in claude cursor codex; do
    assert "harness-defaults: $h supplies a pair for review-$rung" \
      'grep -qE "^ *review-'"$rung"': *\{ *model: *[^ ,}]+, *effort: *[^ ,}]+ *\}" "$HD"'
  done
  F="$REPO/cursor-rules/dispatch/docket-review-$rung.md"
  assert "cursor dispatch fragment exists: docket-review-$rung" '[ -f "$F" ]'
done

# The cap-rung invariant, asserted per harness rather than asserted in prose: review-deep is
# pinned exactly where build-max is, so the cap rung never reviews below the strength the
# riskiest build work was built with.
for h in claude cursor codex; do
  assert "$h: the review-deep pin equals the build-max pin" \
    'blk="$(awk "/^  '"$h"':/{f=1;next} /^  [a-z]+:/{f=0} f" "$HD")";
     d="$(printf "%s" "$blk" | sed -n "s/^ *review-deep: *//p")";
     m="$(printf "%s" "$blk" | sed -n "s/^ *build-max: *//p")";
     [ -n "$d" ] && [ "$d" = "$m" ]'
done

# The rung wrappers must NOT introduce a new dispatch site into test_dispatch_capability.sh's
# reverse-correspondence population. The four docket-build-* workers set the precedent: they are
# referred to as profile agents, never in the `name`-near-"subagent" shape that guard derives on.
assert "rung dispatch prose avoids the derived-dispatch-site shape" \
  '! grep -rohE --include="*.md" "\`docket-review-[a-z]+\`[^\`]{0,20}subagent" "$REPO/skills/" | grep -q .'

# --- the build-evidence chain: producer ----------------------------------------------------
# Per the learnings finding `specified-but-unreachable`: a contract with a producer and a consumer
# needs at least one assert anchored on the paragraph that PERFORMS the write, not only on the
# section that defines what the write means. The producer is docket-build's gate.
BUILD="$REPO/skills/docket-build/SKILL.md"
assert "producer: docket-build names the build-evidence record" \
  'grep -qF -- "build-evidence" "$BUILD"'
assert "producer: the evidence markers are defined where the gate emits them" \
  'grep -qF -- "docket:build-evidence:start" "$BUILD"'
for f in command result head_sha ran_at; do
  assert "producer: the evidence record carries the '$f' field" 'grep -qF -- "$f" "$BUILD"'
done
# The emission must be attached to the GREEN path of the gate, never to the section that merely
# describes the record: scope the search to the gate section's own text.
gate_sec="$(awk "/^## The build gate/{f=1;next} /^## /{f=0} f" "$BUILD")"
assert "producer: the gate section itself emits the evidence (not just a definition elsewhere)" \
  'grep -qF -- "build-evidence" <<<"$gate_sec"'
assert "producer: a red suite mints no evidence record" \
  'grep -qiE "red .{0,60}(never|no) .{0,30}evidence|evidence .{0,40}only .{0,20}green" "$BUILD"'

echo "---"; [ "$fails" -eq 0 ] && echo "PASS" || { echo "FAIL ($fails)"; exit 1; }
