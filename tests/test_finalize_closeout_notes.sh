#!/usr/bin/env bash
# tests/test_finalize_closeout_notes.sh — the closeout-notes handoff contract.
#
# Guards the producer-to-consumer shape change 0330 added: the finalize skill's
# closeout step transforms invocation-supplied verification/findings into the
# two named request fields and passes the request to `docket finalize closeout
# --input`, with no post-merge pause; and the convention documents the terminal
# `## Closeout notes` section. The handoff checker is exercised by a mutation
# probe on a temp COPY of the skill (never the working file).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SKILL="skills/docket-finalize-change/SKILL.md"
CONV="skills/docket-convention/SKILL.md"

# --- section slice: named terminator, existence-asserted (never a bare /^### /) ---
closeout_section(){ # closeout_section FILE -> the "### 9. Closeout" section text
  awk '/^### 9\. Closeout/{f=1} /^### 10\. Cleanup/{f=0} f' "$1"
}
assert "closeout heading present in the skill" 'grep -q "^### 9\. Closeout" "$SKILL"'
assert "named terminator (### 10. Cleanup) present in the skill" 'grep -q "^### 10\. Cleanup" "$SKILL"'

# --- the handoff checker: run against the real skill AND the mutated copy ---
# The prose is hard-wrapped, so the checker reads a whitespace-collapsed slice.
check_notes_handoff(){ # check_notes_handoff FILE -> 0 when the handoff contract holds
  local sec flat
  sec="$(closeout_section "$1")"
  [ -n "$sec" ] || return 1                       # extractor non-vacuity (see below too)
  flat="$(tr '[:space:]' ' ' <<<"$sec" | tr -s ' ')"
  grep -qF -- "verification_outcomes" <<<"$flat" || return 1
  grep -qF -- "late_findings" <<<"$flat" || return 1
  # Producer->consumer: the request file is PASSED to the closeout invocation.
  grep -qE 'docket finalize closeout --id <id> \[--input <request-file>\]' <<<"$flat" || return 1
  grep -qF -- "pass it via \`--input\`" <<<"$flat" || return 1
  # No new pause: the unchanged no-input default is stated in the same step.
  grep -qF -- "no post-merge pause" <<<"$flat" || return 1
  return 0
}

# Extractor non-vacuity: the slice is non-empty and carries the invocation line.
SEC="$(closeout_section "$SKILL")"
assert "closeout section extractor returns a non-empty slice" '[ -n "$SEC" ]'
assert "closeout section carries the closeout invocation" \
  'grep -qF -- "docket finalize closeout" <<<"$SEC"'

assert "finalize skill routes invocation notes into the closeout --input handoff" \
  'check_notes_handoff "$SKILL"'

# The invocation contract is stated up front, before the mechanical flow.
OVERVIEW_FLAT="$(awk '/^## Overview/{f=1} /^## When to use/{f=0} f' "$SKILL" | tr '[:space:]' ' ' | tr -s ' ')"
assert "overview names the no-pause invocation contract" \
  'grep -qF -- "never pauses after merge" <<<"$OVERVIEW_FLAT"'

# --- convention: the terminal section is documented, bound to its claims ---
CONV_FLAT="$(tr '[:space:]' ' ' < "$CONV" | tr -s ' ')"
assert "convention documents ## Closeout notes as terminal-only and closeout-written" \
  'grep -qE -- "\`## Closeout notes\`[^|]{0,220}Written solely by \`docket finalize closeout\`" <<<"$CONV_FLAT"'
assert "convention keeps the results freeze rule beside the new section" \
  'grep -qF -- "the freeze rule above is unchanged" <<<"$CONV_FLAT"'

# --- mutation probe: prove the checker detects the handoff's removal ---
# Copy the skill, verify the handoff is present, strip it, verify the mutation
# LANDED, then require the same checker to reject the copy.
TMP="$(mktemp "${TMPDIR:-/tmp}/closeout-notes-skill.XXXXXX")"
trap 'rm -f "$TMP"' EXIT
cp "$SKILL" "$TMP"
assert "mutation baseline: checker passes on the pristine copy" 'check_notes_handoff "$TMP"'
sed -i '' -e 's/--input//g' "$TMP" 2>/dev/null || sed -i -e 's/--input//g' "$TMP"
assert "mutation landed: --input is gone from the copy" '! grep -qF -- "--input" "$TMP"'
assert "checker rejects the skill with the handoff stripped (mutation-proven)" \
  '! check_notes_handoff "$TMP"'

exit $fail
