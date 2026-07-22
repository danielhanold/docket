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

exit $fail
