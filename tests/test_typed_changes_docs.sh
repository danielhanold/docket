#!/usr/bin/env bash
# tests/test_typed_changes_docs.sh — doc-sentinel for the typed-changes / selective-auto-capture
# documentation (change 0127 and its follow-ups). Run: bash tests/test_typed_changes_docs.sh
#
# Three doc defects shipped alongside 0127; each one was a *copyable* instruction that broke the
# reader's repo (or their whole machine) rather than merely reading wrong. This file pins all
# three fixes so a later edit cannot quietly reintroduce them.
#
#   (A) NO CONFIG EXAMPLE SHIPS THE RETIRED SCALAR `auto_capture`.
#       0127 made `auto_capture` a MAP with no compatibility shim, so a bare scalar is a HARD
#       resolver failure in every layer — every docket skill's Step 0 refuses to start. README's
#       `~/.config/docket/config.yml` and `<repo>/.docket.local.yml` example blocks nevertheless
#       still showed `auto_capture: false`. Copying the documented global-layer example bricked
#       docket in EVERY repo on the machine.
#       Why this needs its own guard: tests/test_docket_example_yml.sh does scan every README
#       yaml fence, but its per-fence check is KEY-EXISTENCE-ONLY — value equality is opt-in per
#       fence via the `<!-- docket:config-fence: values -->` marker, which neither of these two
#       layered-config fences carries. It therefore never judges a value's SHAPE, and a scalar
#       standing where a map belongs sails through it green.
#
#   (B) THE MIGRATION RUNBOOK PRESCRIBES A WRITE-FREE INVENTORY.
#       The runbook told readers to inventory with a bare `docket-status --type untyped` — the
#       full MUTATING pass: it commits and pushes BOARD.md, sweeps merged changes, archives, and
#       harvests before it prints the digest. A command run to *look* was writing. Both call
#       sites now carry `--digest-only`.
#
#   (C) EVERY CREATION PATH WRITES A `type:`.
#       README states the invariant "every creation path writes a type from here on, so the
#       untyped set can only shrink". The primary HUMAN path, skills/docket-new-change, never
#       mentioned `type:` at all — not in its Brainstorm-mode draft step, not in Scan mode — so
#       the untyped set actually GREW with every hand-filed change. Both now write it, and the
#       draft step says to REPLACE the template's comment: an unfilled `type:` line is neither a
#       real type nor `untyped` (the fm_field hazard).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RM="$ROOT/README.md"
EX="$ROOT/.docket.example.yml"
BF="$ROOT/scripts/backfill-change-types.md"
NC="$ROOT/skills/docket-new-change/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Prose asserts run against a WHITESPACE-FLATTENED copy of the file: these docs are hard-wrapped,
# so a sentence a guard depends on can (and does) straddle a newline, and a line-anchored grep
# would go red purely because someone rewrapped a paragraph. Structural asserts below stay
# line-based — a command line is a line.
flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

assert "README exists" '[ -f "$RM" ]'
assert ".docket.example.yml exists" '[ -f "$EX" ]'
assert "backfill runbook exists" '[ -f "$BF" ]'
assert "docket-new-change SKILL.md exists" '[ -f "$NC" ]'

# ── (A) no shipped config example carries the retired scalar ───────────────────────────────────
#
# THE SURFACE IS DERIVED, NOT ENUMERATED: every `*.md` at the repo root plus .docket.example.yml.
# A new root-level doc that copies a config block is guarded the day it is written.
#
# THE REGEX. A directive line is matched ANCHORED — optional indent, then `auto_capture:`, then a
# value. The allowance is deliberate and narrow:
#   `auto_capture:`                  bare map header      => PASSES (nothing after the colon)
#   `auto_capture:   # a MAP since…`  map header + comment => PASSES (only a comment follows)
#   `auto_capture: false` / `: true`  retired scalar       => FAILS
# Lines where the token is INSIDE a comment (README's `# auto_capture: true` "before" sample, and
# the example's `# BREAKING …: the old scalar \`auto_capture: true|false\` …` note) do not match
# the anchor at all, so documenting the retired form as retired stays legal — which is the point,
# since the migration section must be able to show what it is replacing.
# Known limit, accepted: an inline flow map (`auto_capture: {enabled: false}`) would trip this.
# Nothing ships that form, and the message names the map form as the remedy.
a_files(){
  local f
  for f in "$ROOT"/*.md "$EX"; do [ -f "$f" ] && printf '%s\n' "$f"; done
}
scalar_hits="$(a_files | while read -r f; do
  grep -nE '^[[:space:]]*auto_capture:[[:space:]]*[^[:space:]#]' "$f" | sed "s|^|${f#"$ROOT"/}:|"
done)"
assert "(A) no config example ships the retired scalar auto_capture (0127 made it a MAP with no shim — a bare scalar is a hard resolver failure in every layer; use \`auto_capture:\` / \`enabled:\` / \`types:\`). Offenders: [${scalar_hits//$'\n'/ | }]" \
  '[ -z "$scalar_hits" ]'

# POSITIVE COUNTERPART — the map form must actually be present and well-formed, so the negative
# above cannot be satisfied by deleting the examples outright. Every `auto_capture:` header must
# be followed within two lines by an `enabled:` leaf.
map_bad="$(a_files | while read -r f; do
  awk -v rel="${f#"$ROOT"/}" '
    /^[[:space:]]*auto_capture:/ { hdr=NR; want=1; next }
    want && /^[[:space:]]*enabled:/ { want=0; next }
    want && NR > hdr+2 { printf "%s:%d\n", rel, hdr; want=0 }
    END { if (want) printf "%s:%d\n", rel, hdr }
  ' "$f"
done)"
a_headers="$(a_files | while read -r f; do grep -cE '^[[:space:]]*auto_capture:' "$f"; done | awk '{n+=$1} END{print n+0}')"
assert "(A) every auto_capture example is the MAP form — an enabled: leaf within two lines (found $a_headers header(s)); offenders: [${map_bad//$'\n'/ | }]" \
  '[ -z "$map_bad" ]'
assert "(A) non-vacuity floor: the auto_capture config examples still exist to be guarded (got $a_headers, want >= 3: README's two layered-config fences + the example yml)" \
  '[ "${a_headers:-0}" -ge 3 ]'

# ── (B) the migration runbook prescribes a WRITE-FREE inventory ────────────────────────────────
#
# Any documented `docket-status … --type untyped` INVOCATION must carry `--digest-only`.
#
# Scoped to lines that actually invoke docket-status, not to every appearance of the flag pair.
# The write hazard is a property of THAT pass — it commits and pushes BOARD.md, sweeps, archives,
# and harvests — not of the filter: `render-board --format digest --type untyped` is write-free
# and correctly needs no `--digest-only`. An earlier, broader spelling matched the flags alone and
# duly fired on a prose sentence explaining what the `untyped` selector means, which is the guard
# becoming the noise source. Keying on the pass costs a little rename-resistance and buys a rule
# that is true; the non-vacuity floor below is what catches a rename (it goes red if the
# documented invocation disappears entirely).
b_bad="$(for f in "$RM" "$BF"; do
  [ -f "$f" ] || continue
  grep -nE -- 'docket-status[^`]*--type[= ]+untyped' "$f" | grep -v -- '--digest-only' | sed "s|^|${f#"$ROOT"/}:|"
done)"
b_count="$(for f in "$RM" "$BF"; do
  [ -f "$f" ] && grep -cE -- 'docket-status[^`]*--type[= ]+untyped' "$f"
done | awk '{n+=$1} END{print n+0}')"
assert "(B) every documented --type untyped inventory is --digest-only (a bare docket-status pass commits and pushes BOARD.md, sweeps, archives, and harvests before printing the digest — a command run to *look* must not write). Offenders: [${b_bad//$'\n'/ | }]" \
  '[ -z "$b_bad" ]'
assert "(B) non-vacuity floor: the inventory command is still documented in both README and the backfill runbook (got $b_count occurrence(s), want >= 2)" \
  '[ "${b_count:-0}" -ge 2 ]'
assert "(B) the runbook explains WHY --digest-only is load-bearing, not just that it is there" \
  'flat "$BF" | grep -Eqi -- "load.bearing" && flat "$BF" | grep -Eqi "commits and pushes .?BOARD\.md"'
assert "(B) README's inventory step flags the read as write-free" \
  'flat "$RM" | grep -Eqi -- "write.free"'

# ── (C) every creation path — including the human one — writes a type: ─────────────────────────
#
# Block extraction rather than whole-file grep: a `type:` mention anywhere in the skill must not
# satisfy the draft step. Each block runs from its heading/marker line to the next blank line
# (Brainstorm-mode steps are one-line numbered items) or the next `## ` heading (Scan mode).
# Asserts match KEY TOKENS, not sentences, so light rewording stays green.
para(){ awk -v pat="$1" 'index($0,pat){f=1} f && /^[[:space:]]*$/{exit} f{print}' "$2"; }
section(){ awk -v pat="$1" 'index($0,pat){f=1;print;next} f && /^## /{exit} f{print}' "$2"; }

DRAFT="$(para 'Draft the change' "$NC")"
SCAN="$(section '## Scan mode' "$NC")"

assert "(C) the Brainstorm-mode draft step was found" '[ -n "$DRAFT" ]'
assert "(C) the Scan mode section was found" '[ -n "$SCAN" ]'
assert "(C) the draft step writes type: into the frontmatter" \
  'printf "%s\n" "$DRAFT" | grep -q "type:"'
assert "(C) the draft step binds type: to a configured change_type" \
  'printf "%s\n" "$DRAFT" | grep -q "change_type"'
assert "(C) the draft step says to REPLACE the template's comment (an unfilled type: line is neither a real type nor untyped — the fm_field hazard)" \
  'printf "%s\n" "$DRAFT" | grep -qi "replace" &&
   printf "%s\n" "$DRAFT" | grep -qi "template" &&
   printf "%s\n" "$DRAFT" | grep -Eqi "unfilled|blank|empty|left as|comment"'
assert "(C) the draft step names untyped as what an unfilled line is NOT" \
  'printf "%s\n" "$DRAFT" | grep -q "untyped"'
assert "(C) Scan-mode stubs carry a type: too (scan is the bulk creation path — the one that grew the untyped set fastest)" \
  'printf "%s\n" "$SCAN" | grep -q "type:"'
assert "(C) Scan mode restates the shrink-only invariant" \
  'printf "%s\n" "$SCAN" | grep -q "untyped" && printf "%s\n" "$SCAN" | grep -Eqi "shrink"'
assert "(C) README still states the invariant the skill has to uphold" \
  'flat "$RM" | grep -Eqi "creation path writes a type" && flat "$RM" | grep -Eqi "untyped set can only shrink"'

# --- (D) THE TAXONOMY ITSELF IS DOCUMENTED, NOT JUST NAMED -------------------
# The section heading promises `change_types`, but for a while the key appeared ONLY as a bare
# line inside a yaml example — nothing told a reader the default set, that a higher layer REPLACES
# the list rather than merging it (the only way to remove a built-in), what a valid token looks
# like, or that `all`/`untyped` are reserved. A config key you cannot discover how to change is
# undocumented no matter how often the word appears. Pinned against a whitespace-flattened README
# because these docs are hard-wrapped and every sentence below straddles a line break.
RMF="$(flat "$RM")"
assert "(D) README names the default taxonomy in full" \
  'printf "%s" "$RMF" | grep -Eq "chore.*docs.*feat.*fix.*refactor.*perf"'
assert "(D) README states the list is REPLACED, never merged" \
  'printf "%s" "$RMF" | grep -Eqi "replace" && printf "%s" "$RMF" | grep -Eqi "never merge|not merge|rather than merg"'
assert "(D) README explains that replacement is how you REMOVE a built-in" \
  'printf "%s" "$RMF" | grep -Eqi "unremovable|remove .*perf|never drop one"'
assert "(D) README gives the token grammar" \
  'printf "%s" "$RMF" | grep -q "a-z0-9-"'
assert "(D) README names all/untyped as reserved, never stored types" \
  'printf "%s" "$RMF" | grep -Eqi "reserved" && printf "%s" "$RMF" | grep -Eqi "all. and .untyped|untyped. are reserved|all./.untyped"'
assert "(D) README shows a CUSTOM taxonomy example, not only the default restated" \
  'grep -Eq "^change_types: \[.*(spike|[a-z-]+)\]" "$RM" && printf "%s" "$RMF" | grep -Eqi "spike"'
assert "(D) README documents the report-only --type/--priority filters" \
  'printf "%s" "$RMF" | grep -Eq "\-\-type" && printf "%s" "$RMF" | grep -Eq "\-\-priority" && printf "%s" "$RMF" | grep -Eqi "report.only|narrow the digest|digest only"'
assert "(D) README says the filters never narrow writes (the property that makes them safe)" \
  'printf "%s" "$RMF" | grep -Eqi "never .*(the board|narrow the board)" || printf "%s" "$RMF" | grep -Eqi "narrow the .{0,20}digest .{0,20}only"'
# The per-layer validation rule is user-visible semantics: it is why a machine-local narrowing no
# longer bricks a repo, so a reader who hits the same-layer error needs it stated somewhere.
assert "(D) README explains auto_capture.types is checked against the SETTING layer's taxonomy" \
  'printf "%s" "$RMF" | grep -Eqi "visible to the layer|layer that set it" && printf "%s" "$RMF" | grep -Eqi "independent"'

exit $fail
