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
# ugrep and accepts a repetition bound of 600; BSD /usr/bin/grep rejects any bound above 255
# ("maximum repetition exceeds 255"), so a shape test that used one would pass locally and fail on
# a stock macOS host (change 0130). The example bound is spelled in words rather than written as a
# brace interval on purpose: tests/test_grep_portability.sh is a STATIC SOURCE scan over every
# tracked non-docs path, and it does not — must not — exempt comments, since an exemption a comment
# can claim is an exemption an offending pattern can claim by being moved into one.
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

# ---------------------------------------------------------------------------
# (2) Census: every `$(field ` / `$(field_raw ` read in scripts/ names a GUARANTEED-PRESENT key.
#
# Anchored on the CONSUMING CODE — grep the scripts — never on a list of already-migrated sites.
# An allowlist of migrated sites answers "is this one expected?", never "does this one exist?",
# and would age straight into the gap it was written to close. The allowlist below is instead the
# closed set of keys the TEMPLATES guarantee, so adding to it is a conscious one-line edit.
#
# The library itself is path-excluded: list_field/int_field legitimately delegate to field(), and
# field() delegates to field_raw(). Those are the shapes' own plumbing, not consumer call sites.
# ---------------------------------------------------------------------------
ALLOW_FIELD=' id status slug title priority created updated change date '
ALLOW_FIELD_RAW=' title hook '

# Split each line on `$(` and keep the fragments that OPEN with an accessor name, so several reads
# on one line are all seen (`spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"`).
# awk, not `sed 's/x/\n/g'` — BSD sed inserts a literal `n` there.
census_frags(){ awk '{ n = split($0, p, /\$\(/); for (i = 2; i <= n; i++) print p[i] }' "$1"; }

census_files=()
while IFS= read -r f; do
  case "$f" in "$ROOT/scripts/lib/docket-frontmatter.sh") continue ;; esac
  census_files+=("$f")
done < <(find "$ROOT/scripts" -name '*.sh' -type f | sort)
assert "census scanned a non-empty script population" '[ "${#census_files[@]}" -gt 10 ]'

census_violations=""
census_seen=0
for f in "${census_files[@]}"; do
  while IFS= read -r frag; do
    case "$frag" in
      'field '*)     acc=field ;;
      'field_raw '*) acc=field_raw ;;
      *) continue ;;
    esac
    census_seen=$((census_seen + 1))
    rest="${frag#* }"        # "$f" key)…      (drop the accessor)
    rest="${rest#* }"        # key)…           (drop the FILE argument)
    key="${rest%%)*}"        # key
    case "$acc" in
      field)     allow="$ALLOW_FIELD" ;;
      field_raw) allow="$ALLOW_FIELD_RAW" ;;
    esac
    case "$allow" in
      *" $key "*) ;;
      *) census_violations+="  ${f#$ROOT/}: $acc $key"$'\n' ;;
    esac
  done < <(census_frags "$f")
done

# Population floor: a guard that found nothing to check is not a passing guard.
assert "census found real field()/field_raw() reads to check" '[ "$census_seen" -gt 20 ]'

if [ -z "$census_violations" ]; then
  ok "census: every unanchored read names a guaranteed-present key"
else
  no "census: unanchored read of a key that may be ABSENT — use fm_field (or fm_field_verbatim for
free-prose values where a whitespace-preceded '#' is data). See the selection rule in
scripts/lib/docket-frontmatter.sh. If the key really is present in EVERY file the site reads,
add it to this test's ALLOW_FIELD/ALLOW_FIELD_RAW deliberately. Offending sites:
$census_violations"
fi

# A variable-key read (`field "$f" "$key"`) must FAIL by default, so the census cannot be routed
# around by hoisting the key into a variable. Prove that rather than asserting it in a comment.
vk_frag='field "$f" "$key")'
vk_rest="${vk_frag#* }"; vk_rest="${vk_rest#* }"; vk_key="${vk_rest%%)*}"
case "$ALLOW_FIELD" in
  *" $vk_key "*) no "census: a variable-key read would pass the allowlist" ;;
  *)             ok "census: a variable-key read fails the allowlist by default" ;;
esac

# ---------------------------------------------------------------------------
# (3) Orphan pin: fm_field_raw has ZERO production callers. Both directions matter — a silent
# adoption is a call site that never got the rule applied to it, and a silent deletion removes the
# documented raw twin the rule table's raw/anchored quadrant depends on.
# ---------------------------------------------------------------------------
orphan_hits=0
for f in "${census_files[@]}"; do
  while IFS= read -r frag; do
    case "$frag" in 'fm_field_raw '*) orphan_hits=$((orphan_hits + 1)) ;; esac
  done < <(census_frags "$f")
done
if [ "$orphan_hits" -eq 0 ]; then
  ok "orphan pin: fm_field_raw still has 0 production callers"
else
  no "orphan pin: fm_field_raw gained $orphan_hits production caller(s). That is not forbidden —
it is the documented raw twin for an optional key whose consumer decodes quotes itself — but it is
a deliberate adoption: update the rule table in scripts/lib/docket-frontmatter.sh (which currently
states the count is zero) and this pin together."
fi
# The pin is only meaningful if the accessor still EXISTS — a deletion must not read as "0 callers,
# all good".
assert "orphan pin: fm_field_raw is still defined in the library" \
  'grep -qF "fm_field_raw(){" "$LIB"'

# ---------------------------------------------------------------------------
# (4) Behavioral: absent optional keys must read EMPTY, not fall through to body prose.
#
# Driven through render-change-links.sh — the highest-blast-radius consumer, since the values it
# reads get stamped into specs, plans, results files and PR bodies. Two fixtures, because the five
# optional reads are not all observable in one file: fixture 1 discriminates spec/plan/results/pr
# (each renders its own row), fixture 2 discriminates branch (visible only via the plan row's ref).
# One absent-key fixture and one mutation arm PER READ — a fixture that supplies body prose for
# only one key proves exactly one read, and its mirrors can be unanchored later with a green suite.
# ---------------------------------------------------------------------------
CL="$ROOT/scripts/render-change-links.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/docket-config.sh" <<'CFG'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "INTEGRATION_BRANCH=main"
echo "ADRS_DIR=docs/adrs"
echo "METADATA_WORKTREE="
CFG
chmod +x "$tmp/docket-config.sh"

render_cl(){ # render_cl CHANGEFILE
  DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git \
    bash "$CL" --change-file "$1" --repo danielhanold/docket --adrs-dir "$tmp"
}

# --- Fixture 1: spec/plan/results/pr ABSENT from frontmatter, PRESENT as body prose ---
cf1="$tmp/0900-absent-keys.md"
cat > "$cf1" <<'EOF'
---
id: 900
slug: absent-keys
title: Absent optional keys
status: in-progress
priority: medium
created: 2026-08-08
updated: 2026-08-08
branch: feat/absent-keys
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Sentinel | FIXTURE-1-PRE-RENDER-SENTINEL |
<!-- docket:artifacts:end -->

## Why

This body deliberately opens lines with the optional keys, which is ordinary content for a repo
whose subject matter is the field names:

spec: docs/superpowers/specs/PROSE-NOT-A-VALUE.md
plan: docs/superpowers/plans/PROSE-NOT-A-VALUE.md
results: docs/results/PROSE-NOT-A-VALUE.md
pr: https://github.com/danielhanold/docket/pull/99999
EOF
render_cl "$cf1" >/dev/null 2>&1; rc1=$?
# Scope the assertion to the RENDERED block: the fixture's own body carries the prose lines by
# construction, so a whole-file grep can never distinguish a leak from the bait.
block(){ awk '/docket:artifacts:start/,/docket:artifacts:end/' "$1"; }
body1="$(block "$cf1")"
for leaked in \
  'PROSE-NOT-A-VALUE' \
  '99999' ; do
  if printf '%s\n' "$body1" | grep -qF -- "$leaked"; then
    no "absent-key read leaked body prose into the Artifacts block ($leaked)"
  else
    ok "absent-key read returned empty rather than body prose ($leaked)"
  fi
done
# Positive floor, in two halves that fail DIFFERENTLY so a reader can tell a crashed renderer from
# a no-op one. An end-marker grep alone would be vacuous here: with all five optional reads empty
# and `adrs: []` the emitted block is START+END with no rows, which is byte-identical to an
# untouched marker block — so it cannot distinguish "rendered" from "never ran".
#   (a) exit status — a render-change-links.sh that dies on cf1 (bad argument, config resolution
#       failure) must not leave the leak asserts above green by never writing anything;
#   (b) sentinel — the fixture ships a row INSIDE the marker block that only a real rewrite can
#       remove, so its absence is positive proof the block was regenerated.
assert "renderer exited 0 on fixture 1" '[ "$rc1" -eq 0 ]'
assert "renderer rewrote the marker-bounded block (pre-render sentinel row is gone)" \
  '! grep -qF "FIXTURE-1-PRE-RENDER-SENTINEL" <<<"$body1"'
assert "fixture 1 markers survived the rewrite" \
  'printf "%s\n" "$body1" | grep -qF "docket:artifacts:end"'

# --- Fixture 2: branch ABSENT from frontmatter, PRESENT as body prose, plan SET ---
# branch is invisible in fixture 1 (it only selects the blob ref for plan/results rows), so it
# needs its own fixture or its read can be unanchored later with the suite still green.
cf2="$tmp/0901-absent-branch.md"
cat > "$cf2" <<'EOF'
---
id: 901
slug: absent-branch
title: Absent branch key
status: in-progress
priority: medium
created: 2026-08-08
updated: 2026-08-08
plan: docs/superpowers/plans/2026-08-08-absent-branch.md
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

branch: feat/PROSE-NOT-A-BRANCH
EOF
render_cl "$cf2" >/dev/null 2>&1
body2="$(block "$cf2")"
if printf '%s\n' "$body2" | grep -qF -- 'PROSE-NOT-A-BRANCH'; then
  no "absent branch: read leaked body prose into the plan row's blob ref"
else
  ok "absent branch: read returned empty rather than body prose"
fi
assert "plan row rendered (fixture 2 is not vacuous)" \
  'printf "%s\n" "$body2" | grep -qF "2026-08-08-absent-branch.md"'

exit "$fail"
