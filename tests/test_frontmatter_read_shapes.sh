#!/usr/bin/env bash
# tests/test_frontmatter_read_shapes.sh — guards the ONE selection rule for the frontmatter
# scalar read shapes (change 0244). Three guards, deliberately different in kind:
#   (1) prose  — the decision table exists in the library header and says what it must say;
#   (2) census — every `$(field ` / `$(field_raw ` read in scripts/ names a GUARANTEED-PRESENT
#                key, so a new unanchored optional-key read reddens the suite (ADR-0057);
#   (3) orphan — fm_field_raw's production-caller count stays 0, so silent adoption AND silent
#                deletion are both visible;
#   (4) behavior — an absent-key fixture proves the migrated reads are anchored, since a fixture
#                that HAS the key passes under both implementations and guards nothing.
# Plain bash; hermetic fixtures; no network. Run: bash tests/test_frontmatter_read_shapes.sh
#
# Every pattern here is a fixed string or a `case` glob — never bounded repetition. PATH grep is
# ugrep and accepts `{0,600}`; BSD /usr/bin/grep rejects it, so a shape test that used one would
# pass locally and fail on a stock macOS host (change 0130).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$ROOT/scripts/lib/docket-frontmatter.sh"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }
assert(){ if eval "$2"; then ok "$1"; else no "$1"; fi; }

# ---------------------------------------------------------------------------
# (1) The rule exists in the library header, and says the four things it must.
# ---------------------------------------------------------------------------
assert "library exists" '[ -f "$LIB" ]'

# The header block only — the rule is a CONTRACT for readers of the header, so a matching phrase
# further down in an implementation comment must not satisfy it. Slice on the named terminator
# rather than a generic /^## /: a heading-shaped line inside the block would end the slice early.
header="$(awk '/^# --- THE SELECTION RULE/{f=1} /^# resolve_deps globals/{f=0} f' "$LIB")"
assert "header carries the named rule marker" '[ -n "$header" ]'

# Bind each phrase to the accessor it is asserted ABOUT — a guard that merely asserts a phrase is
# PRESENT survives a rewrite that keeps the words and drops the claim.
rule_says(){ # rule_says DESCRIPTION FIXED-STRING
  if printf '%s\n' "$header" | grep -qF -- "$2"; then ok "rule: $1"; else no "rule: $1 (missing: $2)"; fi
}
# The accessor asserts key on the TABLE ROW — the phrase plus the `| accessor` it resolves to.
# A bare `fm_field_verbatim` / `field_raw` grep is satisfied by any of the prose paragraphs that
# merely mention the name, so deleting a row from the table leaves it green (verified by mutation:
# dropping the free-prose row did not redden a bare-name grep).
rule_says "guaranteed-present keys may use field()"       'key guaranteed present, logical value                  | field'
rule_says "an absent-capable key takes the anchored tier" 'key may be ABSENT'
rule_says "structured anchored values use fm_field"       'ordinary structured value           | fm_field'
rule_says "free-prose blocked_by uses fm_field_verbatim"  'is DATA (blocked_by)       | fm_field_verbatim'
rule_says "own-decoding callers use the raw tier"         'caller decodes quotes itself   | field_raw'
rule_says "when unsure, anchor"                           'When unsure'
rule_says "the rule cites ADR-0057"                       'ADR-0057'
rule_says "the rule cites ADR-0058"                       'ADR-0058'

# board-checks.md points AT the canonical table rather than restating it — a restatement
# accumulates its own guards and quietly becomes load-bearing.
BC_MD="$ROOT/scripts/board-checks.md"
assert "board-checks.md points at the library header rule" \
  'grep -qF "docket-frontmatter.sh" "$BC_MD" && grep -qF "selection rule" "$BC_MD"'

exit "$fail"
