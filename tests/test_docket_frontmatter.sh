#!/usr/bin/env bash
# tests/test_docket_frontmatter.sh — unit tests for the shared frontmatter/dependency helper
# (change 0022). Sources the library directly and asserts the accessors, resolve_deps arrays,
# and the readiness classifier. Run: bash tests/test_docket_frontmatter.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "library exists" '[ -f "$LIB" ]'
# shellcheck source=/dev/null
source "$LIB"

# --- int_field: integer-only accessor ---
ifd="$(mktemp -d)"
printf -- '---\nid: 7\n---\n'    > "$ifd/ok.md"
printf -- '---\nid: 007\n---\n'  > "$ifd/pad.md"
printf -- '---\nid: 0\n---\n'    > "$ifd/zero.md"
printf -- '---\nid: abc\n---\n'  > "$ifd/abc.md"
printf -- '---\nid: 1.5\n---\n'  > "$ifd/dot.md"
printf -- '---\nid: 7x\n---\n'   > "$ifd/trail.md"
printf -- '---\nid: -3\n---\n'   > "$ifd/neg.md"
printf -- '---\nslug: x\n---\n'  > "$ifd/none.md"
assert "int_field accepts 7"        '[ "$(int_field "$ifd/ok.md" id)" = "7" ]'
assert "int_field accepts 007"      '[ "$(int_field "$ifd/pad.md" id)" = "007" ]'
assert "int_field accepts 0"        '[ "$(int_field "$ifd/zero.md" id)" = "0" ]'
assert "int_field rejects abc"      '[ -z "$(int_field "$ifd/abc.md" id)" ]'
assert "int_field rejects 1.5"      '[ -z "$(int_field "$ifd/dot.md" id)" ]'
assert "int_field rejects 7x"       '[ -z "$(int_field "$ifd/trail.md" id)" ]'
assert "int_field rejects -3"       '[ -z "$(int_field "$ifd/neg.md" id)" ]'
assert "int_field empty when unset" '[ -z "$(int_field "$ifd/none.md" id)" ]'
rm -rf "$ifd"

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

# 10 done (satisfies a dep); 8 implemented (needs your merge); 3 proposed (not yet built)
cat > "$tmp/archive/2026-06-15-0010-juliet.md" <<'EOF'
---
id: 10
slug: juliet
title: Juliet feature
status: done
priority: medium
depends_on: []
EOF
cat > "$tmp/active/0008-hotel.md" <<'EOF'
---
id: 8
slug: hotel
title: Hotel feature
status: implemented
priority: high
depends_on: []
EOF
cat > "$tmp/active/0003-charlie.md" <<'EOF'
---
id: 3
slug: charlie
title: Charlie feature
status: proposed
priority: medium
depends_on: []
spec:
EOF
# 2: build-ready, dep on a done change (satisfied) + has spec
cat > "$tmp/active/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo feature
status: proposed
priority: medium
depends_on: [10]
spec: docs/superpowers/specs/2026-06-10-bravo.md
EOF
# 5: waiting / not yet built (dep 3 is proposed)
cat > "$tmp/active/0005-echo.md" <<'EOF'
---
id: 5
slug: echo
title: Echo feature
status: proposed
priority: medium
depends_on: [3]
spec: docs/superpowers/specs/2026-06-10-echo.md
EOF
# 6: waiting / needs your merge (dep 8 is implemented)
cat > "$tmp/active/0006-foxtrot.md" <<'EOF'
---
id: 6
slug: foxtrot
title: Foxtrot feature
status: proposed
priority: medium
depends_on: [8]
spec: docs/superpowers/specs/2026-06-10-foxtrot.md
EOF
# 4: needs-brainstorm has the auto-groom-blocked body section
cat > "$tmp/active/0004-delta.md" <<'EOF'
---
id: 4
slug: delta
title: Delta feature
status: proposed
priority: low
depends_on: []
spec:
---

## Auto-groom blocked

2026-06-12 — abstained: needs a human call on scope.
EOF
# 14: proposed, no spec, and it only *talks about* the marker in prose — must NOT be blocked.
cat > "$tmp/active/0014-november.md" <<'EOF'
---
id: 14
slug: november
title: November feature
status: proposed
priority: low
depends_on: []
spec:
---

## Design

- A stub the groomer abstains on gets a dated `## Auto-groom blocked` body section
  (see change 0014) so the abstention is self-describing at the change.
EOF
# 13: implemented and genuinely carrying the finalize-blocked section (the true positive).
cat > "$tmp/active/0013-mike.md" <<'EOF'
---
id: 13
slug: mike
title: Mike feature
status: implemented
priority: high
depends_on: []
pr: https://github.com/o/r/pull/151
---

## Finalize blocked

2026-07-18 — ambiguous rebase conflict; resolve by hand and re-run.
EOF
# 15: implemented, and it only *talks about* the marker in prose — must NOT be blocked.
cat > "$tmp/active/0015-papa.md" <<'EOF'
---
id: 15
slug: papa
title: Papa feature
status: implemented
priority: low
depends_on: []
pr: https://github.com/o/r/pull/153
---

## Design

- A gate failure is marked with a dated `## Finalize blocked` section mirroring the
  proven `## Auto-groom blocked` marker — presence-encoded, cleared by hand.
EOF

# --- accessors ---
assert "field reads a scalar" '[ "$(field "$tmp/active/0008-hotel.md" status)" = "implemented" ]'
assert "field trims trailing space" '[ "$(field "$tmp/active/0008-hotel.md" priority)" = "high" ]'
assert "list_field expands a flow list" '[ "$(list_field "$tmp/active/0002-bravo.md" depends_on)" = "10" ]'
assert "list_field empty for []" '[ -z "$(list_field "$tmp/active/0008-hotel.md" depends_on)" ]'
assert "has_section finds a body line" 'has_section "$tmp/active/0004-delta.md" "## Auto-groom blocked"'
assert "has_section absent returns nonzero" '! has_section "$tmp/active/0003-charlie.md" "## Auto-groom blocked"'
# has_section is a WHOLE-LINE match. These markers are presence-encoded state, and change files
# routinely mention them inline in prose; an unanchored substring match (`grep -qF`) turned every
# such mention into a false "blocked" cell on the board. Both marker strings, both directions.
assert "has_section ignores an inline prose mention (auto-groom)" \
  '! has_section "$tmp/active/0014-november.md" "## Auto-groom blocked"'
assert "has_section ignores an inline prose mention (finalize)" \
  '! has_section "$tmp/active/0015-papa.md" "## Finalize blocked"'
assert "has_section still matches the real section it was pointed at" \
  'has_section "$tmp/active/0004-delta.md" "## Auto-groom blocked"'

# --- resolve_deps ---
resolve_deps "$tmp"
assert "STATUS_OF records own status" '[ "${STATUS_OF[10]}" = "done" ]'
assert "dep on done is clear" '[ "${DEP_STATE[2]}" = "clear" ] && [ -z "${DEP_REASON[2]}" ] && [ -z "${DEP_ON[2]}" ]'
assert "dep on proposed is waiting / not yet built" \
  '[ "${DEP_STATE[5]}" = "waiting" ] && [ "${DEP_REASON[5]}" = "not yet built" ] && [ "${DEP_ON[5]}" = "3" ]'
assert "dep on implemented is waiting / needs your merge" \
  '[ "${DEP_STATE[6]}" = "waiting" ] && [ "${DEP_REASON[6]}" = "needs your merge" ] && [ "${DEP_ON[6]}" = "8" ]'
assert "no deps is clear" '[ "${DEP_STATE[8]}" = "clear" ]'

# --- readiness ---
assert "readiness build-ready (spec + satisfied dep)" '[ "$(readiness "$tmp/active/0002-bravo.md")" = "build-ready" ]'
assert "readiness needs-brainstorm (no spec, not trivial)" '[ "$(readiness "$tmp/active/0003-charlie.md")" = "needs-brainstorm" ]'
assert "readiness auto-groom-blocked (no spec + blocked section)" '[ "$(readiness "$tmp/active/0004-delta.md")" = "auto-groom-blocked" ]'
assert "readiness waiting takes precedence over missing spec" '[ "$(readiness "$tmp/active/0005-echo.md")" = "waiting" ]'
assert "readiness waiting (needs-your-merge dep) returns waiting" '[ "$(readiness "$tmp/active/0006-foxtrot.md")" = "waiting" ]'
assert "readiness needs-brainstorm when the marker is only a prose mention" \
  '[ "$(readiness "$tmp/active/0014-november.md")" = "needs-brainstorm" ]'

# --- finalize_blocked (change 0087) ---
assert "finalize_blocked true for a real section" 'finalize_blocked "$tmp/active/0013-mike.md"'
assert "finalize_blocked false for a prose mention" '! finalize_blocked "$tmp/active/0015-papa.md"'
assert "finalize_blocked false when the section is absent" '! finalize_blocked "$tmp/active/0008-hotel.md"'

# --- iso_to_epoch: portable UTC ISO-8601 -> epoch ---
# Derive the oracle from the host's own date (GNU or BSD) so the test is host-portable —
# compare iso_to_epoch against that, never against a hardcoded epoch constant.
known="2026-07-17T12:00:00Z"
oracle="$(TZ=UTC date -u -d "$known" +%s 2>/dev/null || date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$known" +%s 2>/dev/null)"
got="$(iso_to_epoch "$known")"
assert "iso_to_epoch parses a UTC ISO-8601 timestamp" '[ -n "$got" ] && [ "$got" = "$oracle" ]'
assert "iso_to_epoch returns nonzero + empty on garbage" '! iso_to_epoch "not-a-timestamp" >/dev/null 2>&1'
assert "iso_to_epoch returns empty string on garbage" '[ -z "$(iso_to_epoch "not-a-timestamp" 2>/dev/null)" ]'

# --- shared board vocabularies (change 0116) ---
assert "DOCKET_PRIORITIES is rank-ordered critical > high > medium > low" \
  '[ "${DOCKET_PRIORITIES[*]:-}" = "critical high medium low" ]'
assert "DOCKET_PRIORITIES has exactly four members" '[ "${#DOCKET_PRIORITIES[@]}" = 4 ]' 2>/dev/null
assert "active-status helper is defined" 'declare -F docket_status_is_active >/dev/null'
assert "terminal-status helper is defined" 'declare -F docket_status_is_terminal >/dev/null'
assert "priority-membership helper is defined" 'declare -F docket_priority_is_member >/dev/null'
assert "priority-rank helper is defined" 'declare -F docket_priority_rank >/dev/null'
assert "DOCKET_PRIORITY_DEFAULT is a declared priority" \
  'docket_priority_is_member "${DOCKET_PRIORITY_DEFAULT:-}"'
assert "active helper accepts proposed" 'docket_status_is_active proposed'
assert "active helper rejects terminal done" '! docket_status_is_active done'
assert "active helper rejects empty" '! docket_status_is_active ""'
assert "terminal helper accepts killed" 'docket_status_is_terminal killed'
assert "terminal helper rejects active implemented" '! docket_status_is_terminal implemented'
assert "terminal helper rejects empty" '! docket_status_is_terminal ""'
assert "member-status helper is defined" 'declare -F docket_status_is_member >/dev/null'
assert "member helper accepts an active status" 'docket_status_is_member proposed'
assert "member helper accepts a terminal status" 'docket_status_is_member done'
assert "member helper rejects a status outside the vocabulary" '! docket_status_is_member bogus'
assert "member helper rejects empty" '! docket_status_is_member ""'
# An interior TAB can never match a closed-vocabulary name — this is the rejection that IS the
# sanitization for `status:` in render-board.sh (change 0259, spec assumption 4).
assert "member helper rejects a vocabulary name carrying an interior TAB" \
  '! docket_status_is_member "$(printf "done\tx")"'
assert "member helper accepts every DOCKET_STATUSES entry" \
  'for _m_s in "${DOCKET_STATUSES[@]}"; do docket_status_is_member "$_m_s" || exit 1; done'
assert "priority membership accepts high" 'docket_priority_is_member high'
assert "priority membership rejects empty" '! docket_priority_is_member ""'
assert "priority membership rejects unknown" '! docket_priority_is_member urgent'
assert "priority rank derives critical as zero" '[ "$(docket_priority_rank critical)" = 0 ]'
assert "priority rank derives low as three" '[ "$(docket_priority_rank low)" = 3 ]'
assert "priority rank defaults empty to medium's index" '[ "$(docket_priority_rank "")" = 2 ]'
assert "priority rank defaults unknown to medium's index" '[ "$(docket_priority_rank urgent)" = 2 ]'

# --- matched-quote unwrap: readers return the LOGICAL scalar (change 0138) ---
qd="$(mktemp -d)"
printf -- '---\ntitle: "Comma, title"\n---\n'   > "$qd/dq.md"
printf -- "---\ntitle: 'Comma, title'\n---\n"   > "$qd/sq.md"
printf -- '---\ntitle: Bare title\n---\n'       > "$qd/bare.md"
printf -- '---\ntitle: Say "hi" now\n---\n'     > "$qd/interior.md"
printf -- '---\ntitle: "unterminated\n---\n'    > "$qd/untl.md"
printf -- '---\ntitle: foo"\n---\n'             > "$qd/trailq.md"
printf -- '---\ntitle: "\n---\n'                > "$qd/onechar.md"
printf -- '---\ntitle: ""\n---\n'               > "$qd/empty2.md"

# field(): strip a matched surrounding pair, leave everything else byte-for-byte
assert "field strips a double-quoted value"       '[ "$(field "$qd/dq.md" title)" = "Comma, title" ]'
assert "field strips a single-quoted value"       '[ "$(field "$qd/sq.md" title)" = "Comma, title" ]'
assert "field leaves a bare value unchanged"      '[ "$(field "$qd/bare.md" title)" = "Bare title" ]'
assert "field leaves an interior quote untouched" '[ "$(field "$qd/interior.md" title)" = "Say \"hi\" now" ]'
assert "field leaves an unterminated open quote"  '[ "$(field "$qd/untl.md" title)" = "\"unterminated" ]'
assert "field leaves a trailing-only quote"       '[ "$(field "$qd/trailq.md" title)" = "foo\"" ]'
assert "field leaves a lone single quote char"    '[ "$(field "$qd/onechar.md" title)" = "\"" ]'
assert "field reduces an empty quoted value"      '[ -z "$(field "$qd/empty2.md" title)" ]'
# field() MUST keep its single trailing newline (piped-consumer contract, e.g. mermaid done-id list)
assert "field emits exactly one trailing newline" '[ "$(field "$qd/dq.md" title | wc -l | tr -d " ")" = "1" ]'

# fm_field(): mirror-image cases through the anchored twin (shares the helper)
assert "fm_field strips a double-quoted value"       '[ "$(fm_field "$qd/dq.md" title)" = "Comma, title" ]'
assert "fm_field strips a single-quoted value"       '[ "$(fm_field "$qd/sq.md" title)" = "Comma, title" ]'
assert "fm_field leaves a bare value unchanged"      '[ "$(fm_field "$qd/bare.md" title)" = "Bare title" ]'
assert "fm_field leaves an interior quote untouched" '[ "$(fm_field "$qd/interior.md" title)" = "Say \"hi\" now" ]'
assert "fm_field leaves an unterminated open quote"  '[ "$(fm_field "$qd/untl.md" title)" = "\"unterminated" ]'
assert "fm_field empty when the key is absent"       '[ -z "$(fm_field "$qd/dq.md" nonesuch)" ]'

# fm_field_raw(): the RAW reader through the anchored twin — surrounding quotes LEFT INTACT, same
# ---...--- scope and inline-comment strip as fm_field (change 0191).
assert "fm_field_raw preserves a double-quoted value" '[ "$(fm_field_raw "$qd/dq.md" title)" = "\"Comma, title\"" ]'
assert "fm_field_raw preserves a single-quoted value" '[ "$(fm_field_raw "$qd/sq.md" title)" = "'"'"'Comma, title'"'"'" ]'
assert "fm_field_raw leaves a bare value unchanged"   '[ "$(fm_field_raw "$qd/bare.md" title)" = "Bare title" ]'
assert "fm_field_raw leaves an interior quote untouched" '[ "$(fm_field_raw "$qd/interior.md" title)" = "Say \"hi\" now" ]'
assert "fm_field_raw leaves an unterminated open quote"  '[ "$(fm_field_raw "$qd/untl.md" title)" = "\"unterminated" ]'
assert "fm_field_raw empty when the key is absent"    '[ -z "$(fm_field_raw "$qd/dq.md" nonesuch)" ]'

# The anchored-read proof: fm_field_raw must NOT fall through to a body line that opens with the
# key when the frontmatter omits it (the ADR-0057 fixture discipline, same shape as fm_field's).
fr="$(mktemp -d)"
printf -- '---\nid: 1\n---\n\n## Why\nblocked_by: this is body prose, not frontmatter\n' > "$fr/body.md"
assert "fm_field_raw is empty when the key is absent but the body opens with it" \
  '[ -z "$(fm_field_raw "$fr/body.md" blocked_by)" ]'

# Same inline-comment strip as fm_field: whitespace-preceded `#` to EOL is dropped, a `#` not
# preceded by whitespace stays part of the value.
printf -- '---\ntype: feat   # chosen at creation\n---\n' > "$fr/commented.md"
printf -- '---\ntype: feat#1\n---\n' > "$fr/hash.md"
assert "fm_field_raw strips the same inline-comment shape fm_field strips" \
  '[ "$(fm_field_raw "$fr/commented.md" type)" = feat ]'
assert "fm_field_raw keeps a hash not preceded by whitespace in the value" \
  '[ "$(fm_field_raw "$fr/hash.md" type)" = "feat#1" ]'

# ...but the strip is BARE-value territory only: a QUOTED scalar's interior is not comment territory,
# so a ' #' inside the quotes must survive whole (change 0235). Every minted title is now
# single-quoted, so without this skip the stray-quote truncation ('clear finding) is the ROUTINE
# outcome for any '#'-bearing title — and render-artifact-backlink.sh stamps exactly that value into
# specs, plans, results files and PR bodies. Mirrors the skip leg board-checks's scalar_form_check
# already uses on the raw token.
printf -- "---\ntitle: 'clear finding #3 from review'\n---\n" > "$fr/qhash.md"
assert "fm_field returns a quoted '#'-bearing title whole" \
  '[ "$(fm_field "$fr/qhash.md" title)" = "clear finding #3 from review" ]'
assert "fm_field_raw keeps the quoted '#'-bearing token intact" \
  '[ "$(fm_field_raw "$fr/qhash.md" title)" = "'"'"'clear finding #3 from review'"'"'" ]'

# fm_field_verbatim(): the third tier — same anchored ---...--- scope, NEITHER strip. The value
# arrives exactly as authored, because a consumer JUDGING a scalar's YAML form cannot be handed a
# value the reader already repaired: the comment strip IS the truncation board-checks's
# comment-introducer leg exists to report (change 0235). The contrast pair against fm_field_raw on
# the same fixture is the whole point of the tier.
printf -- '---\nblocked_by: PR #69 is stale, predating the rework\n---\n' > "$fr/hashval.md"
assert "fm_field_verbatim keeps a whitespace-preceded '#...' in the value" \
  '[ "$(fm_field_verbatim "$fr/hashval.md" blocked_by)" = "PR #69 is stale, predating the rework" ]'
assert "fm_field_raw TRUNCATES that same value at the ' #' (the contrast the tier exists for)" \
  '[ "$(fm_field_raw "$fr/hashval.md" blocked_by)" = "PR" ]'
assert "fm_field_verbatim keeps the template's inline comment too (no strip at all)" \
  '[ "$(fm_field_verbatim "$fr/commented.md" type)" = "feat   # chosen at creation" ]'
assert "fm_field_verbatim preserves surrounding quotes" \
  '[ "$(fm_field_verbatim "$qd/dq.md" title)" = "\"Comma, title\"" ]'
assert "fm_field_verbatim leaves a bare value unchanged" \
  '[ "$(fm_field_verbatim "$qd/bare.md" title)" = "Bare title" ]'
assert "fm_field_verbatim empty when the key is absent" \
  '[ -z "$(fm_field_verbatim "$qd/dq.md" nonesuch)" ]'
assert "fm_field_verbatim is anchored: empty when the key is absent but the body opens with it" \
  '[ -z "$(fm_field_verbatim "$fr/body.md" blocked_by)" ]'
rm -rf "$fr"

# field_raw() is the RAW reader — surrounding quotes are LEFT INTACT (change 0138)
assert "field_raw preserves a double-quoted value"  '[ "$(field_raw "$qd/dq.md" title)" = "\"Comma, title\"" ]'
assert "field_raw preserves a single-quoted value"  '[ "$(field_raw "$qd/sq.md" title)" = "'"'"'Comma, title'"'"'" ]'
assert "field_raw leaves a bare value unchanged"    '[ "$(field_raw "$qd/bare.md" title)" = "Bare title" ]'
assert "field_raw emits exactly one trailing newline" '[ "$(field_raw "$qd/dq.md" title | wc -l | tr -d " ")" = "1" ]'
rm -rf "$qd"

# --- _docket_unwrap_quotes: the single-quote escape inverse (change 0235, ADR-0071) -------------
# Single-quoted YAML interprets no escapes and has exactly ONE rule: an embedded ' is written ''.
# The reader must be the exact inverse of docket_yaml_single_quote's doubling, or every
# apostrophe-bearing title renders as manifest''s in BOARD.md and mis-compares in dup_of.
uq="$(mktemp -d)"
printf -- "---\ntitle: 'The manifest''s elsewhere: check'\n---\n" > "$uq/sq-apos.md"
printf -- "---\ntitle: 'plain single quoted'\n---\n"              > "$uq/sq-plain.md"
printf -- '---\ntitle: "a b'"''"'c"\n---\n'                       > "$uq/dq-doubled.md"
printf -- "---\ntitle: it''s bare\n---\n"                         > "$uq/bare-doubled.md"
printf -- "---\ntitle: ''\n---\n"                                 > "$uq/sq-empty.md"
printf -- "---\ntitle: ''''''''\n---\n"                           > "$uq/sq-only-quotes.md"

assert "unwrap undoubles '' inside a single-quoted token" \
  '[ "$(field "$uq/sq-apos.md" title)" = "The manifest'"'"'s elsewhere: check" ]'
assert "unwrap leaves a single-quoted token with no doubling unchanged" \
  '[ "$(field "$uq/sq-plain.md" title)" = "plain single quoted" ]'
# The double-quoted arm adds NO escape interpretation: a literal '' inside "..." stays two bytes.
assert "unwrap does NOT undouble '' inside a DOUBLE-quoted token" \
  '[ "$(field "$uq/dq-doubled.md" title)" = "a b'"''"'c" ]'
# Undoubling is scoped to a token that was actually quoted — a BARE value is never rewritten.
assert "unwrap does NOT undouble '' in a BARE (unquoted) value" \
  '[ "$(field "$uq/bare-doubled.md" title)" = "it'"''"'s bare" ]'
assert "unwrap maps the empty single-quoted scalar '"'"''"'"' to the empty string" \
  '[ -z "$(field "$uq/sq-empty.md" title)" ]'
# Interior of '''''''' (8 quotes stripped to 6) is three doubled pairs -> three apostrophes.
assert "unwrap collapses every doubled pair, not just the first" \
  '[ "$(field "$uq/sq-only-quotes.md" title)" = "'"'''"'" ]'
assert "field_raw still preserves the doubling (the RAW token is not decoded)" \
  '[ "$(field_raw "$uq/sq-apos.md" title)" = "'"'"'The manifest'"''"'s elsewhere: check'"'"'" ]'
rm -rf "$uq"

# --- docket_scalar_quote_reason: the CHECKER's needs-quoting predicate (change 0235) ------------
# Scoped to the logical value and to SYNTAX only. The writer does not consume this (it quotes
# unconditionally, ADR-0071); board-checks does, because it judges scalars it did not write.
reason(){ docket_scalar_quote_reason "$1"; }

assert "leg colon-space"        '[ "$(reason "a: b")" = colon-space ]'
assert "leg trailing-colon"     '[ "$(reason "ends in a colon:")" = trailing-colon ]'
assert "leg bare-boolean yes"   '[ "$(reason "yes")" = bare-boolean ]'
assert "leg bare-boolean TRUE"  '[ "$(reason "TRUE")" = bare-boolean ]'
assert "leg bare-boolean off"   '[ "$(reason "off")" = bare-boolean ]'
assert "leg comment-introducer" '[ "$(reason "clear finding #3")" = comment-introducer ]'
# YAML opens a comment on ANY whitespace before the '#', not on a literal space alone — a TAB does
# it just as silently. The leg must be at least as wide as the READER it warns about: fm_field_raw's
# own inline-comment strip is `[[:space:]]+#`, so a tab-preceded '#' WOULD be truncated on the read
# path while a space-only detector stayed silent about it. mint-stub's control-character gate keeps
# tabs off the WRITE path, but scalar_form_check judges hand-authored files, which have no such gate.
assert "leg comment-introducer with a TAB before the hash" \
  '[ "$(reason "$(printf "a\t#b")")" = comment-introducer ]'
assert "leg indicator bracket"  '[ "$(reason "[WIP] rework")" = indicator ]'
assert "leg indicator anchor"   '[ "$(reason "&anchor thing")" = indicator ]'
assert "leg indicator star"     '[ "$(reason "*star* emphasis")" = indicator ]'
assert "leg indicator at"       '[ "$(reason "@mention someone")" = indicator ]'
assert "leg indicator quote"    '[ "$(reason "\"quoted\" start")" = indicator ]'
assert "leg indicator dash-sp"  '[ "$(reason "- a list-looking title")" = indicator ]'
# A LEADING '#' is the maximal case of the failure the comment-introducer leg exists to catch: the
# comment opens at character one, so the ENTIRE value parses to null. It reaches this leg only when
# the value carries no ': ' and no ' #' of its own — hence the second fixture, which must land on
# comment-introducer instead (first leg matched wins) and not on indicator.
assert "leg indicator leading-hash" '[ "$(reason "#235 follow-up work")" = indicator ]'
assert "a leading hash with a later ' #' takes the earlier leg" \
  '[ "$(reason "#235 clears finding #3")" = comment-introducer ]'

# Near-misses: each is well-formed bare YAML and must stay SILENT.
assert "silent: a colon with no following space" '[ -z "$(reason "a:b ratio")" ]'
assert "silent: offset is not off"               '[ -z "$(reason "offset")" ]'
assert "silent: nobody is not no"                '[ -z "$(reason "nobody")" ]'
assert "silent: a hash not preceded by space"    '[ -z "$(reason "issue#3 reopened")" ]'
assert "silent: an interior dash"                '[ -z "$(reason "a well-formed title")" ]'
assert "silent: an interior asterisk"            '[ -z "$(reason "a b*c title")" ]'
assert "silent: an ordinary prose title"         '[ -z "$(reason "Cap the widget")" ]'
assert "silent: the EMPTY value is exempt"       '[ -z "$(reason "")" ]'

# There is NO flow-collection exemption. The predicate answers one question — "is this value
# well-formed as a BARE SCALAR" — and a flow collection is not a scalar at all, so `[234]` gets the
# honest answer for the question asked: bare, it does not read back as the string `[234]`. A caller
# that means a value as a sequence or a map must not route it through a scalar predicate. An
# exemption here bought nothing (no consumer reads a collection-valued field) and cost the four
# legs it shadowed, since it was necessarily evaluated ahead of them.
assert "a flow sequence is not a scalar: indicator"      '[ "$(reason "[234]")" = indicator ]'
assert "a flow sequence with items: indicator"           '[ "$(reason "[4, 6]")" = indicator ]'
assert "a flow map hits the FIRST leg it matches"        '[ "$(reason "{owner: x}")" = colon-space ]'
# The legs an exemption placed ahead of them made unreachable. Both fired under the pre-0235
# checker; neither may go silent again.
assert "a bracketed value with a colon-space still fires" '[ "$(reason "[a title: with colon]")" = colon-space ]'
assert "a bracketed value ending in a colon still fires"  '[ "$(reason "[see the plan]:")" = trailing-colon ]'
# An UNCLOSED bracket was never a collection under any reading — it must still fire.
assert "leg indicator: an UNCLOSED bracket still fires" '[ "$(reason "[WIP")" = indicator ]'

# The boolean wrapper agrees with the reason function in both directions.
assert "needs_quoting true for a colon-space value" 'docket_scalar_needs_quoting "a: b"'
assert "needs_quoting false for a clean value"      '! docket_scalar_needs_quoting "Cap the widget"'
assert "needs_quoting false for the empty value"    '! docket_scalar_needs_quoting ""'

# --- readiness(): spec:/trivial: are OPTIONAL, so the reads must be anchored (change 0244) ---
# Without anchoring, a needs-brainstorm change whose BODY opens a `spec:` line reports build-ready
# and the autonomous builder claims an undesigned change. The fixture must OMIT the key: a change
# that HAS spec: in frontmatter reports build-ready under both implementations.
rdy="$(mktemp -d)"
mkdir -p "$rdy/active" "$rdy/archive"
cat > "$rdy/active/0902-prose-spec.md" <<'EOF'
---
id: 902
slug: prose-spec
title: Prose spec
status: proposed
priority: medium
created: 2026-08-08
updated: 2026-08-08
depends_on: []
---

## Why

The design will live at
spec: docs/superpowers/specs/2026-08-08-not-a-real-value-design.md
once someone writes it.
EOF
cat > "$rdy/active/0903-prose-trivial.md" <<'EOF'
---
id: 903
slug: prose-trivial
title: Prose trivial
status: proposed
priority: medium
created: 2026-08-08
updated: 2026-08-08
depends_on: []
---

## Why

Whether this is
trivial: true
is exactly the open question.
EOF
resolve_deps "$rdy"
assert "readiness: body-prose spec: does not make a change build-ready" \
  '[ "$(readiness "$rdy/active/0902-prose-spec.md")" = "needs-brainstorm" ]'
assert "readiness: body-prose trivial: does not make a change build-ready" \
  '[ "$(readiness "$rdy/active/0903-prose-trivial.md")" = "needs-brainstorm" ]'
rm -rf "$rdy"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
