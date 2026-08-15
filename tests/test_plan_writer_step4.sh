#!/usr/bin/env bash
# tests/test_plan_writer_step4.sh — Step 4's plan-writer dispatch contract in
# skills/docket-implement-next (change 0324): foreground dispatch, PLAN_PATH-only success,
# git-side verification with no directory allowlist, local MUST-continue, Tier C posture,
# and the resume seam in edge-paths.md.
# run: bash tests/test_plan_writer_step4.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SK="$REPO/skills/docket-implement-next/SKILL.md"
EP="$REPO/skills/docket-implement-next/references/edge-paths.md"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
flat(){ tr '\n' ' ' < "$1" | tr -s ' '; }
S="$(flat "$SK")"; E="$(flat "$EP")"

# producer-side anchors (specified-but-unreachable learning: anchor on the paragraph that ACTS)
assert "step 4 dispatches docket-plan-writer foreground" \
  'grep -q "dispatch.*docket-plan-writer.*foreground\|docket-plan-writer.*(foreground" <<<"$S" || grep -qE "dispatches? the .docket-plan-writer. subagent \(foreground" <<<"$S"'
assert "success protocol is the PLAN_PATH line" 'grep -q "PLAN_PATH=" <<<"$S"'
assert "returned path is a claim, not proof" 'grep -qi "claim, not proof" <<<"$S"'
assert "verification: delta since pre-dispatch HEAD contains only the plan file" \
  'grep -qi "only the returned plan file\|only the plan file" <<<"$S"'
assert "verification: exactly one Docket-Plan-Path trailer equal to the returned path" \
  'grep -q "Docket-Plan-Path:" <<<"$S"'
assert "verification: backlink markers ordered, balanced, point home" \
  'grep -qi "backlink" <<<"$S"'
assert "no directory allowlist, deliberately" 'grep -qi "no directory allowlist" <<<"$S"'
assert "local MUST continue into Step 5" \
  'grep -qE "MUST (be verified and attached, then[^.]*proceed|proceed) into Step 5" <<<"$S"'
assert "PLAN_PATH is never a terminal disposition" \
  'grep -qi "PLAN_PATH.*(never|not).*(terminal disposition|advanced)" <<<"$S" || grep -qi "neither the child.s return nor Step 4" <<<"$S"'
assert "tier C: dispatch unavailable halts unless SKILL_PLAN=auto authorizes inline" \
  'grep -qi "Tier C" <<<"$S"'
assert "never adopt or commit the child.s uncommitted output" \
  'grep -qi "never adopt" <<<"$S"'

# resume seam (edge-paths.md)
assert "resume: plan: set + verified artifact -> reuse, continue at Step 5, no second planner" \
  'grep -qi "never dispatch a second planner" <<<"$E"'
assert "resume: trailer recovery when plan: is empty" 'grep -q "Docket-Plan-Path:" <<<"$E"'
assert "resume: ambiguity halts with the exact mismatch, never re-plans" \
  'grep -qi "never re-plan\|never guess a custom plan location" <<<"$E"'
assert "resume: attributed re-dispatch enters resume before selection" \
  'grep -qi "before ordinary ready-queue\|before selection" <<<"$E"'
assert "resume: ordinary allowlist still skips an in-progress id" \
  'grep -qi "still skips" <<<"$E"'

CV="$REPO/skills/docket-convention/SKILL.md"
C="$(tr '\n' ' ' < "$CV" | tr -s ' ')"
assert "convention: step-4 plan-writer composition dispatch is named" \
  'grep -q "docket-plan-writer.*(step 4)\|dispatches the .docket-plan-writer. subagent (step 4)" <<<"$C"'
assert "convention: plan-writer is in the Tier C dispatch cell" \
  'grep -qi "plan-writer dispatch.*build.*review" <<<"$C"'
assert "convention: seventeen wrappers, five wrap none" \
  'grep -qi "seventeen" <<<"$C" && grep -qiE "five.{0,40}wrap (no skill|none)" <<<"$C"'

exit "$fail"
