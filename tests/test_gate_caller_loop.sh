#!/usr/bin/env bash
# tests/test_gate_caller_loop.sh — guards the caller-side gate-DRIVER contract published in
# skills/docket-build/references/gate-caller-loop.md.
#
# WHAT THIS FILE IS FOR: change 0342 retired the executable Bash observe loop this reference used to
# publish (a `while` loop that slept and re-parsed `docket gate observe --json` with jq). A workflow
# caller no longer runs a shell poll loop; it makes short, slice-bounded, synchronous
# `docket gate drive start|advance|handoff|claim` calls to the native gate driver. So this file no
# longer EXTRACTS AND EXECUTES a fence — there is none. It is a prose/structure guard on the typed
# driver contract: the four driver operations are documented, the four dispositions and their
# semantics are stated, WAITING requires an explicit handoff, the raw verbs are demoted to
# primitives, and NO Bash gate loop / jq workflow parsing survives. Each assert is mutation-aware:
# deleting the clause or the table row it keys on reddens it.
#
# The assert helper is the tree's canonical one byte for byte (scripts/check-test-source-hygiene.sh
# rule (a) is a byte-exact allowlist); scripts/run-tests.sh accounts results on the `ok - ` /
# `NOT OK - ` markers it prints.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
# Whitespace-flatten a hard-wrapped markdown slice: grep matches within a line, so a phrase-spanning
# pattern over wrapped prose otherwise doubles as a re-flow guard.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

REF="$REPO/skills/docket-build/references/gate-caller-loop.md"
assert "the gate-driver reference exists" '[ -f "$REF" ]'
ref="$(cat "$REF" 2>/dev/null || true)"
# Population floor FIRST: an unreadable page has to redden here, or every assert below passes by
# default and reports a contract it never actually looked for.
assert "the reference is non-vacuous (>= 40 lines)" '[ "$(grep -c . <<<"$ref")" -ge 40 ]'
ref_flat="$(flat "$ref")"

# --- (1) THE BASH GATE LOOP AND jq WORKFLOW PARSING ARE RETIRED -----------------------------------
# The core of change 0342 for this file: the executable observe loop and its jq extraction are gone.
# Keyed on SHAPE (a ```bash fence, the jq tool, the sleep-poll idiom), never on an enumerated
# spelling — the shape you miss is the one that regrows. Mutation: paste back the old fence -> the
# fence/jq/sleep asserts redden together.
assert "no executable bash fence survives (the caller loop is retired)" \
  '! grep -qE "^\`\`\`bash$" <<<"$ref"'
assert "jq is no longer a dependency of any gate sequencing here" \
  '! grep -qiw "jq" <<<"$ref"'
assert "no sleep/poll loop over the raw observe verb survives" \
  '! grep -qiE "sleep[[:space:]]+[0-9]" <<<"$ref" && ! grep -qiE "while[[:space:]]*:" <<<"$ref"'

# --- (2) THE FOUR DRIVER OPERATIONS ARE DOCUMENTED ------------------------------------------------
# DERIVED from the "## The driver's operations" table's first-column code-span token, never
# hand-listed: a row dropped from the table reddens the set-equality below automatically. The four
# operations are the workflow surface `docket gate drive <op>` this contract publishes.
assert "the driver-operations section carries its own heading" \
  'grep -qxF -- "## The driver'"'"'s operations" <<<"$ref"'
ops_blk="$(awk '/^## The driver'"'"'s operations$/{f=1;next} f && /^## /{f=0} f' <<<"$ref")"
assert "the driver-operations section is non-vacuous (>= 6 lines)" \
  '[ "$(grep -c . <<<"$ops_blk")" -ge 6 ]'
ops="$(awk -F'|' '
  /^\| *Operation *\|/ {f=1; next}
  !f {next}
  /^\|[ -]*\|[ -]*\|?/ {next}
  /^\|/ {n=split($2, a, "`"); if (n>=2) {t=a[2]; gsub(/[^a-z-]/, "", t); if (t != "") print t}; next}
  {f=0}' <<<"$ops_blk" | sort -u)"
assert "the operations table lists exactly start/advance/handoff/claim" \
  '[ "$(printf "%s\n" $ops)" = "$(printf "%s\n" advance claim handoff start | sort)" ]'
for op in $ops; do
  assert "the contract names the '$op' driver operation as \`docket gate drive $op\`" \
    'grep -qF -- "docket gate drive '"$op"'" <<<"$ref"'
done

# --- (3) THE FOUR DISPOSITIONS AND THEIR SEMANTICS ------------------------------------------------
# Also DERIVED from the disposition table's first column: the four typed outcomes a caller keys on.
assert "the disposition-vocabulary section carries a heading" \
  'grep -qE "^## The disposition vocabulary" <<<"$ref"'
disp_blk="$(awk '/^## The disposition vocabulary/{f=1;next} f && /^## /{f=0} f' <<<"$ref")"
assert "the disposition section is non-vacuous (>= 6 lines)" \
  '[ "$(grep -c . <<<"$disp_blk")" -ge 6 ]'
disp="$(awk -F'|' '
  /^\| *Disposition *\|/ {f=1; next}
  !f {next}
  /^\|[ -]*\|[ -]*\|/ {next}
  /^\|/ {n=split($2, a, "`"); if (n>=2) {t=a[2]; gsub(/[^A-Z]/, "", t); if (t != "") print t}; next}
  {f=0}' <<<"$disp_blk" | sort -u)"
assert "the disposition table lists exactly WAITING/PASSED/FAILED/HALTED" \
  '[ "$(printf "%s\n" $disp)" = "$(printf "%s\n" FAILED HALTED PASSED WAITING | sort)" ]'
# The three load-bearing semantics, each keyed on its rule with a bounded window so a bare token
# mention cannot stand in. Mutation: delete the sentence -> red.
assert "WAITING is the only nonterminal disposition (advances again)" \
  'grep -qiE "WAITING[^.]{0,120}(only nonterminal|advance)" <<<"$ref_flat"'
assert "only FAILED feeds repair; death/drift/deadline are HALTED, never red" \
  'grep -qiE "FAILED[^.]{0,40}ONLY[^.]{0,60}(feed|repair)|only[^.]{0,20}FAILED[^.]{0,60}(feed|repair)" <<<"$ref_flat" &&
   grep -qiE "(death|drift|deadline|uncertain)[^.]{0,120}HALTED" <<<"$ref_flat"'
assert "only PASSED exposes the raw run dir for evidence" \
  'grep -qiE "only[^.]{0,20}PASSED[^.]{0,60}raw run dir" <<<"$ref_flat"'

# --- (4) HANDOFF-REQUIRED WAITING ----------------------------------------------------------------
# The heart of the ownership contract: a departing owner must perform an explicit handoff and name
# the handoff token; a bare "still waiting" is not a valid departure. Both poles are pinned so a
# rewrite that keeps the word "handoff" while dropping the requirement reddens.
assert "the handoff section carries a heading" \
  'grep -qE "^## Handoff" <<<"$ref"'
assert "a departing owner must call handoff before returning control" \
  'grep -qiE "(before[^.]{0,40}return|departing owner)[^.]{0,120}handoff|handoff[^.]{0,120}(before[^.]{0,40}return)" <<<"$ref_flat"'
assert "a WAITING departure MUST name the handoff token" \
  'grep -qiE "handoff token" <<<"$ref_flat"'
assert "a bare still-waiting with no handoff token is invalid" \
  'grep -qiE "(bare|no handoff)[^.]{0,120}(not[^.]{0,20}valid|invalid|stranded)" <<<"$ref_flat"'
assert "claim consumes the single-use receipt with a compare-and-swap, one winner" \
  'grep -qiE "single-use" <<<"$ref_flat" && grep -qiE "compare-and-swap|only one" <<<"$ref_flat"'

# --- (5) THE RAW VERBS ARE PRIMITIVE/OPERATOR APIs, NOT WORKFLOW VERBS ----------------------------
# The five raw verbs retain their narrow meaning but a workflow caller never composes them. This is
# the demotion that keeps the driver the sole workflow surface.
assert "the primitives section carries a heading" \
  'grep -qiE "^## The raw verbs are primitive" <<<"$ref"'
prim_blk="$(awk '/^## The raw verbs are primitive/{f=1;next} f && /^## /{f=0} f' <<<"$ref")"
assert "the primitives section is non-vacuous (>= 4 lines)" \
  '[ "$(grep -c . <<<"$prim_blk")" -ge 4 ]'
prim_flat="$(flat "$prim_blk")"
assert "the raw launch/observe/stop verbs are named as primitives" \
  'grep -qF -- "docket gate launch" <<<"$prim_blk" && grep -qF -- "docket gate observe" <<<"$prim_blk" && grep -qF -- "docket gate stop" <<<"$prim_blk"'
assert "a workflow caller never composes the raw verbs directly" \
  'grep -qiE "workflow[^.]{0,120}(never|not)[^.]{0,60}(compose|call|directly)" <<<"$prim_flat"'
assert "the raw verbs are not high-level workflow APIs" \
  'grep -qiE "not[^.]{0,40}(high-level )?workflow API" <<<"$prim_flat"'

exit "$fail"
