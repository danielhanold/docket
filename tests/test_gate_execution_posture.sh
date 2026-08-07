#!/usr/bin/env bash
# tests/test_gate_execution_posture.sh — change 0223. Guards for the build gate's EXECUTION
# posture: the contract clauses in docket-build, finalize's citation-by-reference, the
# per-harness reference, and the default budget's agreement across every surface that states it.
# Run: bash tests/test_gate_execution_posture.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BUILD="$REPO/skills/docket-build/SKILL.md"
REF="$REPO/skills/docket-build/references/gate-execution.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Phrase asserts read a whitespace-FLATTENED haystack. grep matches within a line, so a
# phrase-spanning assert over hard-wrapped markdown silently doubles as a line-wrap guard: a pure
# re-flow reddens it with a message about a policy nobody changed. `-s` (squeeze) is load-bearing,
# not tidiness — a wrapped list item indents its continuation, so a plain newline-to-space swap
# leaves words four spaces apart and a single-space pattern misses.
flatten(){ tr -s '[:space:]' ' '; }

assert "build: SKILL.md exists" '[ -f "$BUILD" ]'
build_body="$(cat "$BUILD" 2>/dev/null)"
build_flat="$(flatten <<<"$build_body")"
# Non-vacuity floor: every assert below reads these variables, so an unreadable file must redden
# HERE rather than passing every negative grep by default.
assert "build: body is non-vacuous (>= 150 lines)" \
  '[ "$(printf "%s\n" "$build_body" | grep -c .)" -ge 150 ]'

# --- (1) the posture subsection exists, INSIDE the build gate -------------------
# LINE-anchored on purpose: the heading level is the signal — a `##` here would make the posture a
# sibling of the gate rather than part of it, and flattening erases the line start.
assert "posture: is a subsection heading" \
  'grep -qE "^### Gate execution posture" <<<"$build_body"'

# Groups (2)-(4) read a SECTION SLICE, not the whole file. The plan drafted them file-wide; that is
# too weak for at least one of them, and weak in the same way for the rest. The repair-task clause
# is the proof: `## Halting conditions`' pre-existing "Never convert this into a repair task"
# (the undetectable-suite case) already satisfies a file-wide match, so deleting the posture's own
# refusal would have reddened nothing — a guard that cannot fail on the mutation it exists to catch.
# This is the same reasoning tests/test_docket_build.sh records for its `halt_blk` slice: "a
# whole-file grep would be satisfied by the in-place rule that points here". Consequence, recorded
# rather than discovered later: deleting or re-levelling the heading reddens (1) AND every scoped
# assert together. That cascade is correct — a posture with no section states none of these rules —
# and it is why each clause is still mutation-tested individually, one clause at a time.
posture_blk="$(awk '/^### Gate execution posture$/{f=1;next} f && /^#+ /{f=0} f' <<<"$build_body")"
posture_flat="$(flatten <<<"$posture_blk")"
assert "posture: the subsection body is non-vacuous (>= 20 lines)" \
  '[ "$(grep -c . <<<"$posture_blk")" -ge 20 ]'

# --- (2) the load-bearing clauses ----------------------------------------------
# Keyed on the RULE, not on any wording introduced by this change alone, so a faithful rewrite
# stays green and a rewrite that drops the rule reddens.
assert "posture: must not depend on a single foreground call" \
  'grep -qiE "(not|never)[^.]{0,120}single foreground" <<<"$posture_flat"'
assert "posture: requires a durable result artifact" \
  'grep -qiE "durable[^.]{0,60}(result|artifact)" <<<"$posture_flat"'
assert "posture: completion is established from the artifact, not the caller signal" \
  'grep -qiE "(never|not)[^.]{0,140}completion signal" <<<"$posture_flat"'
assert "posture: observation is bounded by a finite budget" \
  'grep -qiE "(bounded|finite)[^.]{0,80}budget" <<<"$posture_flat"'
assert "posture: names the resolved budget export" \
  'grep -qF "GATE_OBSERVATION_BUDGET" <<<"$posture_blk"'
assert "posture: exhausting the budget FAILS CLOSED" \
  'grep -qiE "fail[s]? closed" <<<"$posture_flat"'
# Fail-closed is a HALT, never a red — the distinction that keeps an unfinished run from
# manufacturing an integration-repair task.
assert "posture: fail-closed must not mint a repair task" \
  'grep -qiE "(never|not|no)[^.]{0,120}repair task" <<<"$posture_flat"'

# --- (3) the false-completion rule ---------------------------------------------
assert "posture: a stale pre-yield report is not evidence of a crash" \
  'grep -qiE "stale[^.]{0,120}(crash|not evidence)" <<<"$posture_flat"'

# --- (4) the ADR-0024 boundary is NOT relaxed ----------------------------------
assert "posture: distinguishes a dispatched subagent from an external gate process" \
  'grep -qiE "dispatched subagent" <<<"$posture_flat"'

# --- (5) HARNESS NEUTRALITY: the body names no mechanism -----------------------
# This is the negative that keeps the quarantine real, and it is deliberately WHOLE-FILE: the
# neutrality invariant binds the skill body everywhere, not only inside the posture section.
# Anchored word-wise so ordinary prose ("background" as an English word) cannot trip it while a
# real mechanic does.
for mech in nohup setsid "&>" "process group" "shell id"; do
  assert "neutrality: build body does not name the mechanism '$mech'" \
    '! grep -qiF -- "'"$mech"'" <<<"$build_body"'
done
# No harness FIGURE either. The one number the body may carry is docket's own default budget, so
# the pattern deliberately excludes a bare "30" and targets duration-shaped literals.
assert "neutrality: build body states no harness timeout figure" \
  '! grep -qiE "[0-9]+[[:space:]]*(ms|milliseconds|000 ms|minute foreground|s timeout)" <<<"$build_body"'

# --- (6) the blocking pointer to the quarantine --------------------------------
assert "posture: points at the per-harness reference" \
  'grep -qF "references/gate-execution.md" <<<"$posture_blk"'
assert "reference: the file exists" '[ -f "$REF" ]'
ref_body="$(cat "$REF" 2>/dev/null)"
assert "reference: is non-vacuous (>= 40 lines)" \
  '[ "$(printf "%s\n" "$ref_body" | grep -c .)" -ge 40 ]'
# Six capabilities, counted rather than spot-checked: dropping one is the drift that matters.
assert "reference: enumerates exactly 6 required capabilities" \
  '[ "$(grep -cE "^[0-9]+\. " <<<"$ref_body")" -eq 6 ]'

# --- (7) the halting condition --------------------------------------------------
# LINE-anchored, and scoped to the section that owns the enumeration: `## Halting conditions` is
# where a halt becomes a disposition rather than a mention, so a budget bullet anywhere else is
# not this rule. The `^- **` shape is the section's own bullet form.
halt_blk="$(awk '/^## Halting conditions$/{f=1;next} f && /^## /{f=0} f' <<<"$build_body")"
assert "build: the Halting conditions section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$halt_blk")" -ge 15 ]'
assert "build: Halting conditions carries the exhausted-budget bullet" \
  'grep -qiE "^- \*\*.*budget" <<<"$halt_blk"'

# --- (8) finalize CITES the posture, never restates it -------------------------
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize: SKILL.md exists" '[ -f "$FIN" ]'
fin_body="$(cat "$FIN" 2>/dev/null)"
fin_flat="$(flatten <<<"$fin_body")"
assert "finalize: body is non-vacuous (>= 100 lines)" \
  '[ "$(printf "%s\n" "$fin_body" | grep -c .)" -ge 100 ]'
# The POSITIVES read the gate flow's item-5 `local` bullet, not the whole file. That bullet IS the
# run this posture governs, and the plan's file-wide draft would be satisfied by the phrase turning
# up anywhere in the file — including a mention that leaves the gate's own run uncited, which is the
# exact mutation these asserts exist to catch. Same reasoning as groups (2)-(4)'s section slices.
# A bullet-level anchor is deliberately tighter than a `## The rebase-retest merge gate` section
# slice: the section also contains items 4, 6 and 7, and a citation parked beside the CI leg would
# satisfy a section-scoped grep while leaving the local run — the only leg that runs a suite here —
# uncited.
fin_local="$(grep -E '^[[:space:]]*- `local` runs the suite' <<<"$fin_body")"
local_flat="$(flatten <<<"$fin_local")"
assert "finalize: the item-5 local-gate bullet was located (non-vacuity anchor)" \
  '[ -n "$fin_local" ]'
# The citation names the OWNER, so a reader lands on the single source.
assert "finalize: local gate cites the gate execution posture" \
  'grep -qiE "gate execution posture" <<<"$local_flat"'
assert "finalize: the citation names docket-build as the owner" \
  'grep -qiE "gate execution posture[^.]{0,120}docket-build|docket-build[^.]{0,120}gate execution posture" <<<"$local_flat"'
# ...and does NOT restate it. Restatement accumulates its own guards and then goes stale; these
# negatives are what keep the single source single. Deliberately WHOLE-FILE, unlike the positives
# above: the no-restatement rule binds finalize everywhere, not only inside the bullet that cites.
# Both phrases are absent from the pre-change file (measured: zero matches), so neither negative
# arrives already red — each is mutation-tested by PLANTING the restatement it forbids. The file's
# pre-existing "durable root" prose (change 0075) does not reach an "artifact" inside the window.
# (The plan drafted the first as `durable[^.]{0,60}(result artifact|artifact)`; the first branch is
# subsumed by the second, so this keys on `artifact` alone and matches strictly more.)
assert "finalize: does not restate the durable-artifact clause" \
  '! grep -qiE "durable[^.]{0,60}artifact" <<<"$fin_flat"'
assert "finalize: does not restate the fail-closed clause" \
  '! grep -qiE "fail[s]? closed" <<<"$fin_flat"'

exit $fail
