#!/usr/bin/env bash
# tests/test_docket_runtime_lib.sh — run: bash tests/test_docket_runtime_lib.sh
# Unit-tests scripts/lib/docket-runtime.sh, the single implementation of runtime.bash
# block traversal, scalar decoding, counting, serializability, and GNU Bash 4+ validation.
# The library is BOOTSTRAP-COMPATIBLE: a dedicated case re-runs the core assertions under a
# real Bash 3.2 so a Bash-4-only construct cannot reach install.sh or ensure-global-config.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO/scripts/lib/docket-runtime.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# shellcheck source=/dev/null
. "$LIB"

MARK_OPEN='# >>> docket (runtime.bash) >>>'
MARK_CLOSE='# <<< docket (runtime.bash) <<<'

# --- sourcing is side-effect free -------------------------------------------
side="$(bash -c '. "$1" ; printf DONE' _ "$LIB" 2>&1)"
assert "sourcing produces no output and no error" '[ "$side" = "DONE" ]'

# --- scalar decoding --------------------------------------------------------
# Each case: <name>|<yaml scalar as written>|<expected decoded value>
scalar_case(){ # scalar_case <file> <written-scalar>
  printf 'runtime:\n  bash: %s\n' "$2" > "$1"
}
f="$tmp/scalar.yml"

scalar_case "$f" '/usr/local/bin/bash'
assert "scalar bare" '[ "$(docket_runtime_first "$f")" = "/usr/local/bin/bash" ]'

scalar_case "$f" "'/usr/local/bin/bash'"
assert "scalar single-quoted" '[ "$(docket_runtime_first "$f")" = "/usr/local/bin/bash" ]'

scalar_case "$f" '"/usr/local/bin/bash"'
assert "scalar double-quoted" '[ "$(docket_runtime_first "$f")" = "/usr/local/bin/bash" ]'

scalar_case "$f" '/usr/local/bin/bash   # chosen by hand'
assert "scalar bare strips inline comment" '[ "$(docket_runtime_first "$f")" = "/usr/local/bin/bash" ]'

scalar_case "$f" "'/opt/od''d/bash'  # doubled apostrophe"
assert "scalar single-quoted undoubles apostrophe and strips comment" \
  '[ "$(docket_runtime_first "$f")" = "/opt/od'\''d/bash" ]'

scalar_case "$f" "'/opt/back\\slash/bash'"
assert "scalar single-quoted keeps backslash literal" \
  '[ "$(docket_runtime_first "$f")" = "/opt/back\\slash/bash" ]'

scalar_case "$f" "'/opt/has#hash/bash'"
assert "scalar single-quoted keeps an inner hash" \
  '[ "$(docket_runtime_first "$f")" = "/opt/has#hash/bash" ]'

# Empty is a PRESENT declaration with an empty value — never "absent".
for empty in '' "''" '# pick one'; do
  scalar_case "$f" "$empty"
  assert "empty spelling [$empty]: value is empty" '[ -z "$(docket_runtime_first "$f")" ]'
  assert "empty spelling [$empty]: count is 1" '[ "$(docket_runtime_count "$f")" = 1 ]'
done

# --- runtime-block constraint (mutation target M1) --------------------------
printf 'bash: /decoy/top-level\nother:\n  bash: /decoy/nested\n' > "$tmp/decoy.yml"
assert "M1 a bash: leaf outside a runtime: block is never read" \
  '[ -z "$(docket_runtime_first "$tmp/decoy.yml")" ] && [ "$(docket_runtime_count "$tmp/decoy.yml")" = 0 ]'
printf 'other:\n  bash: /decoy/nested\nruntime:\n  bash: /real/bash\n' > "$tmp/decoy2.yml"
assert "M1 a decoy block does not shadow the real runtime block" \
  '[ "$(docket_runtime_first "$tmp/decoy2.yml")" = "/real/bash" ]'

# A dedent closes the block.
printf 'runtime:\n  other: x\nnext:\n  bash: /decoy/after-dedent\n' > "$tmp/dedent.yml"
assert "M1 dedent terminates the runtime block" \
  '[ "$(docket_runtime_count "$tmp/dedent.yml")" = 0 ]'

# Tab indentation counts as indentation (AGENTS.md: awk indent classes are [^[:space:]]).
printf 'runtime:\n\tbash: /tab/bash\n' > "$tmp/tab.yml"
assert "M1 tab-indented leaf is read" '[ "$(docket_runtime_first "$tmp/tab.yml")" = "/tab/bash" ]'

# --- counting + uniqueness (mutation target M2) -----------------------------
printf 'runtime:\n  bash: /one\n' > "$tmp/one.yml"
printf 'runtime:\n  bash: /one\n  bash: /two\n' > "$tmp/dup-in-block.yml"
printf 'runtime:\n  bash: /one\nruntime:\n  bash: /two\n' > "$tmp/dup-two-blocks.yml"

assert "count: absent file is 0" '[ "$(docket_runtime_count "$tmp/nope.yml")" = 0 ]'
assert "count: single is 1" '[ "$(docket_runtime_count "$tmp/one.yml")" = 1 ]'
assert "count: two leaves in one block is 2" '[ "$(docket_runtime_count "$tmp/dup-in-block.yml")" = 2 ]'
assert "count: two separate runtime blocks is 2" '[ "$(docket_runtime_count "$tmp/dup-two-blocks.yml")" = 2 ]'

u="$(docket_runtime_unique "$tmp/one.yml")"; urc=$?
assert "M2 unique: single declaration returns 0 with the value" '[ "$urc" -eq 0 ] && [ "$u" = "/one" ]'
u2="$(docket_runtime_unique "$tmp/dup-in-block.yml")"; urc2=$?
assert "M2 unique: duplicate leaves return 2" '[ "$urc2" -eq 2 ]'
assert "M2 unique: duplicate leaves print nothing" '[ -z "$u2" ]'
u3="$(docket_runtime_unique "$tmp/dup-two-blocks.yml")"; urc3=$?
assert "M2 unique: duplicate blocks return 2" '[ "$urc3" -eq 2 ]'
u4="$(docket_runtime_unique "$tmp/nope.yml")"; urc4=$?
assert "M2 unique: absent file returns 0 and empty" '[ "$urc4" -eq 0 ] && [ -z "$u4" ]'

# --- marker exclusion (mutation target M3) ----------------------------------
printf '%s\nruntime:\n  bash: /managed/bash\n%s\nruntime:\n  bash: /explicit/bash\n' \
  "$MARK_OPEN" "$MARK_CLOSE" > "$tmp/both.yml"
assert "M3 with markers: the managed block is excluded" \
  '[ "$(docket_runtime_first "$tmp/both.yml" "$MARK_OPEN" "$MARK_CLOSE")" = "/explicit/bash" ]'
assert "M3 with markers: only the explicit declaration is counted" \
  '[ "$(docket_runtime_count "$tmp/both.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 1 ]'
assert "M3 without markers: both declarations are visible" \
  '[ "$(docket_runtime_count "$tmp/both.yml")" = 2 ]'
assert "M3 without markers: the managed value is first" \
  '[ "$(docket_runtime_first "$tmp/both.yml")" = "/managed/bash" ]'

printf '%s\nruntime:\n  bash: /managed/bash\n%s\n' "$MARK_OPEN" "$MARK_CLOSE" > "$tmp/managed-only.yml"
assert "M3 managed-only file has no explicit declaration" \
  '[ "$(docket_runtime_count "$tmp/managed-only.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 0 ]'

# Empty marker arguments must not match blank lines — the resolver passes none.
printf '\nruntime:\n\n  bash: /blank/bash\n\n' > "$tmp/blank.yml"
assert "empty markers do not match blank lines" \
  '[ "$(docket_runtime_first "$tmp/blank.yml" "" "")" = "/blank/bash" ]'
assert "omitted markers behave like empty markers" \
  '[ "$(docket_runtime_first "$tmp/blank.yml")" = "/blank/bash" ]'

# --- bootstrap compatibility: the real Bash 3.2 witness ---------------------
# The library is sourced by install.sh and ensure-global-config.sh BEFORE a configured Bash 4+
# exists, so it must run under the system Bash. macOS ships 3.2.57 at /bin/bash.
LEGACY=""
if [ -x /bin/bash ]; then
  legacy_major="$(LC_ALL=C /bin/bash --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')"
  [ "${legacy_major:-9}" -lt 4 ] && LEGACY=/bin/bash
fi
if [ -n "$LEGACY" ]; then
  export LEGACY_SELF="$LEGACY"
  legacy_out="$("$LEGACY" -c '
    . "$1" || exit 90
    printf "%s|" "$(docket_runtime_first "$2")"
    printf "%s|" "$(docket_runtime_count "$3")"
    docket_runtime_unique "$3" >/dev/null 2>&1; printf "%s|" "$?"
    printf "%s|" "$(docket_runtime_first "$4" "$5" "$6")"
    docket_runtime_serializable "/plain/path" && printf "ser-ok|"
    docket_runtime_validate_bash "$LEGACY_SELF" >/dev/null 2>&1; printf "%s" "$?"
  ' _ "$LIB" "$tmp/one.yml" "$tmp/dup-in-block.yml" "$tmp/both.yml" "$MARK_OPEN" "$MARK_CLOSE" 2>&1)"
  assert "bash 3.2: library parses and every read shape works" \
    '[ "$legacy_out" = "/one|2|2|/explicit/bash|ser-ok|1" ]'
else
  echo "ok - bash 3.2: SKIPPED (no sub-4 /bin/bash on this host)"
fi

exit $fail
