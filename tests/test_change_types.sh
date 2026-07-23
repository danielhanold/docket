#!/usr/bin/env bash
# tests/test_change_types.sh — the change-type vocabulary (change 0127).
# Run: bash tests/test_change_types.sh   (no network, no fixtures — pure library unit test)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# shellcheck disable=SC1090
. "$LIB"

# --- the array exists, is non-empty, ordered, duplicate-free, well-formed ----
assert "DOCKET_CHANGE_TYPES_DEFAULT is declared and non-empty" \
  '[ "${#DOCKET_CHANGE_TYPES_DEFAULT[@]}" -gt 0 ]'
assert "default taxonomy is exactly the six spec'd tokens in order" \
  '[ "${DOCKET_CHANGE_TYPES_DEFAULT[*]}" = "chore docs feat fix refactor perf" ]'
dups="$(printf '%s\n' "${DOCKET_CHANGE_TYPES_DEFAULT[@]}" | sort | uniq -d)"
assert "default taxonomy is duplicate-free (${dups:-none})" '[ -z "$dups" ]'
for t in "${DOCKET_CHANGE_TYPES_DEFAULT[@]}"; do
  assert "default type '$t' is well-formed" 'docket_change_type_is_wellformed "$t"'
done

# --- membership is over the CALLER's list, not the default -------------------
# The effective list is config-resolved, so a helper that silently consulted the built-in array
# would ignore a repo's configured taxonomy entirely.
assert "member: feat is in the default list" \
  'docket_change_type_is_member feat "${DOCKET_CHANGE_TYPES_DEFAULT[@]}"'
assert "member: spike is NOT in the default list" \
  '! docket_change_type_is_member spike "${DOCKET_CHANGE_TYPES_DEFAULT[@]}"'
assert "member: honors a caller-supplied effective list (spike admitted)" \
  'docket_change_type_is_member spike chore spike'
assert "member: honors a caller-supplied effective list (feat excluded)" \
  '! docket_change_type_is_member feat chore spike'

# --- reserved pseudo-values --------------------------------------------------
assert "reserved: all" 'docket_change_type_is_reserved all'
assert "reserved: untyped" 'docket_change_type_is_reserved untyped'
assert "not reserved: feat" '! docket_change_type_is_reserved feat'

# --- well-formedness rejects the shapes the spec forbids ---------------------
for bad in "Feat" "1feat" "fe_at" "feat " "" "fe at" "-feat"; do
  assert "well-formed rejects '$bad'" '! docket_change_type_is_wellformed "$bad"'
done
assert "well-formed accepts a hyphenated token" 'docket_change_type_is_wellformed multi-word'
# A newline-bearing value must not slip through on the strength of its first line: this is the
# structural-injection shape (model-authored-values-are-untrusted-input), and a regex anchored
# with ^…$ matches line-wise under grep, so the guard must reject the whole value.
assert "well-formed rejects an embedded newline" \
  '! docket_change_type_is_wellformed "$(printf "feat\ntrivial: true")"'

# --- ADR-0055: reserved set is pinned by set equality, with a cardinality floor
assert "reserved array has exactly 2 members" \
  '[ "${#DOCKET_CHANGE_TYPE_RESERVED[@]}" = 2 ]'
assert "reserved array is exactly {all, untyped}" \
  '[ "$(printf "%s\n" "${DOCKET_CHANGE_TYPE_RESERVED[@]}" | sort | tr "\n" " ")" = "all untyped " ]'

# --- fm_field: frontmatter-anchored reads (change 0127) ----------------------
# The failure this exists to prevent: a change with NO frontmatter type: whose BODY opens a line
# with `type:` — the exact state of every un-backfilled change during the migration window.
fmtmp="$(mktemp -d)"; trap 'rm -rf "$fmtmp"' EXIT
printf -- '---\nid: 1\nstatus: proposed\n---\n\n## Why\ntype: this is prose, not frontmatter\n' > "$fmtmp/no-type.md"
printf -- '---\nid: 2\nstatus: proposed\ntype: feat\n---\n\n## Why\ntype: prose again\n' > "$fmtmp/typed.md"
printf -- '---\nid: 3\nstatus: proposed\ntype:\n---\n\n## Why\nx\n' > "$fmtmp/empty.md"

assert "fm_field: a body-only key reads EMPTY, never the prose" \
  '[ -z "$(fm_field "$fmtmp/no-type.md" type)" ]'
assert "fm_field: reads a real frontmatter value" \
  '[ "$(fm_field "$fmtmp/typed.md" type)" = feat ]'
assert "fm_field: an empty placeholder reads empty" \
  '[ -z "$(fm_field "$fmtmp/empty.md" type)" ]'
assert "fm_field: an absent key reads empty" \
  '[ -z "$(fm_field "$fmtmp/typed.md" nosuchkey)" ]'
assert "fm_field: still reads the other frontmatter keys" \
  '[ "$(fm_field "$fmtmp/typed.md" status)" = proposed ]'
# The contrast that proves the anchor is load-bearing: unanchored field() DOES return the prose.
assert "fm_field: unanchored field() would have returned the prose (the bug this prevents)" \
  '[ "$(field "$fmtmp/no-type.md" type)" = "this is prose, not frontmatter" ]'

# --- fm_field: a YAML inline comment is stripped BEFORE the key prefix -------
# The failure this exists to prevent: change-template.md ships `type:` WITH a trailing comment, so
# an unfilled template line read without the strip returns the COMMENT TEXT rather than empty. The
# change is then neither `untyped` nor a real type: it escapes `docket-status --type untyped` (the
# documented migration inventory), backfill-change-types.sh refuses to assign it ("already has
# type ..."), and the comment's pipe characters inject phantom columns into its board row.
#
# The fixture takes the template's type: line VERBATIM from the shipped template rather than
# restating it here, so the pin tracks the real artifact instead of a copy that can drift from it.
TEMPLATE="$REPO/skills/docket-new-change/change-template.md"
tmpl_line="$(grep -m1 '^type:' "$TEMPLATE" || true)"
assert "fixture source: change-template.md still ships a commented type: placeholder" \
  '[ -n "$tmpl_line" ] && [ "$tmpl_line" != "${tmpl_line%%#*}" ]'
{ printf -- '---\nid: 4\nstatus: proposed\n'
  printf '%s\n' "$tmpl_line"
  printf -- '---\n\n## Why\nx\n'; } > "$fmtmp/template-line.md"
printf -- '---\nid: 5\nstatus: proposed  # set at creation\ntype: feat   # chosen at creation\n---\n\n## Why\nx\n' > "$fmtmp/commented.md"
printf -- '---\nid: 6\nstatus: proposed\ntype: feat#1\n---\n\n## Why\nx\n' > "$fmtmp/hash-value.md"

assert "fm_field: the template's own commented type: placeholder reads EMPTY (not the comment)" \
  '[ -z "$(fm_field "$fmtmp/template-line.md" type)" ]'
assert "fm_field: a real value with a trailing comment reads as just the value" \
  '[ "$(fm_field "$fmtmp/commented.md" type)" = feat ]'
# YAML only treats `#` as a comment when whitespace precedes it; anything else is part of the value.
assert "fm_field: a hash with no preceding whitespace stays in the value" \
  '[ "$(fm_field "$fmtmp/hash-value.md" type)" = "feat#1" ]'
# The strip must be scoped to the matched line's own value — sibling keys still read normally.
assert "fm_field: a commented sibling key is unharmed by the strip" \
  '[ "$(fm_field "$fmtmp/commented.md" status)" = proposed ]'
# The contrast that proves the strip is load-bearing: comment-blind field() DOES return the
# comment text — comment-shaped (leading #) and pipe-bearing (the phantom board columns).
tmpl_unstripped="$(field "$fmtmp/template-line.md" type)"
assert "fm_field: comment-blind field() would have returned the template comment (the bug this prevents)" \
  '[ "${tmpl_unstripped#\#}" != "$tmpl_unstripped" ] && [ "${tmpl_unstripped%%|*}" != "$tmpl_unstripped" ]'

exit $fail
