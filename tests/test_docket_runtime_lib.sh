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
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
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

# The managed block's own `runtime:` header must not leak in_runtime state past the close
# marker: a bare indented `bash:` leaf with NO runtime: header of its own, sitting right after
# the close marker, must not be picked up.
printf '%s\nruntime:\n  bash: /managed/bash\n%s\n  bash: /leaked/bash\n' \
  "$MARK_OPEN" "$MARK_CLOSE" > "$tmp/leak-after-close.yml"
assert "M3 the managed runtime: header does not leak past the close marker" \
  '[ "$(docket_runtime_count "$tmp/leak-after-close.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 0 ]'

# Empty marker arguments must not match blank lines — the resolver passes none.
printf '\nruntime:\n\n  bash: /blank/bash\n\n' > "$tmp/blank.yml"
assert "empty markers do not match blank lines" \
  '[ "$(docket_runtime_first "$tmp/blank.yml" "" "")" = "/blank/bash" ]'
assert "omitted markers behave like empty markers" \
  '[ "$(docket_runtime_first "$tmp/blank.yml")" = "/blank/bash" ]'

# --- serializability (mutation target M5) -----------------------------------
assert "M5 serializable: a plain path is accepted" 'docket_runtime_serializable "/opt/homebrew/bin/bash"'
assert "M5 serializable: an empty value is accepted" 'docket_runtime_serializable ""'
assert "M5 serializable: an apostrophe is accepted" 'docket_runtime_serializable "/opt/od'\''d/bash"'
assert "M5 serializable: a backslash is accepted" 'docket_runtime_serializable "/opt/back\\slash/bash"'
assert "M5 serializable: a newline is rejected" '! docket_runtime_serializable "/opt/two"$'\''\n'\''"lines"'
assert "M5 serializable: a carriage return is rejected" '! docket_runtime_serializable "/opt/cr"$'\''\r'\''"bash"'

# --- Bash 4+ validation (mutation target M4) --------------------------------
fake_bash(){ # fake_bash <path> <first --version line> [noexec]
  mkdir -p "$(dirname "$1")"
  cat > "$1" <<EOF
#!/bin/sh
[ "\$#" -eq 1 ] && [ "\$1" = --version ] || exit 42
printf '%s\n' '$2'
EOF
  if [ "${3-}" = noexec ]; then chmod -x "$1"; else chmod +x "$1"; fi
}
BIN="$tmp/vbin"; mkdir -p "$BIN"
fake_bash "$BIN/good"      'GNU bash, version 5.2.0(1)-release (test)'
fake_bash "$BIN/exactly4"  'GNU bash, version 4.0.0(1)-release (test)'
fake_bash "$BIN/legacy"    'GNU bash, version 3.2.57(1)-release (test)'
fake_bash "$BIN/notbash"   'zsh 5.9 (arm64-apple-darwin)'
fake_bash "$BIN/weird"     'GNU bash, version X.Y-release (test)'
fake_bash "$BIN/noexec"    'GNU bash, version 5.2.0(1)-release (test)' noexec
printf '#!/bin/sh\nexit 7\n' > "$BIN/novers"; chmod +x "$BIN/novers"

probe(){ # probe <path> -> "<rc>|<reason>|<version line>"
  local out rc reason rest
  out="$(docket_runtime_validate_bash "$1"; printf 'x')"
  docket_runtime_validate_bash "$1" >/dev/null 2>&1; rc=$?
  out="${out%x}"
  reason="${out%%$'\n'*}"; rest="${out#*$'\n'}"
  printf '%s|%s|%s' "$rc" "$reason" "${rest%$'\n'}"
}

assert "validate: a GNU Bash 5 executable is ok" \
  '[ "$(probe "$BIN/good")" = "0|ok|GNU bash, version 5.2.0(1)-release (test)" ]'
assert "M4 validate: major exactly 4 is accepted" \
  '[ "$(probe "$BIN/exactly4")" = "0|ok|GNU bash, version 4.0.0(1)-release (test)" ]'
assert "M4 validate: Bash 3.2 is rejected as old-major" \
  '[ "$(probe "$BIN/legacy")" = "1|old-major|GNU bash, version 3.2.57(1)-release (test)" ]'
assert "M4 validate: an unparseable major is rejected as old-major" \
  '[ "$(probe "$BIN/weird")" = "1|old-major|GNU bash, version X.Y-release (test)" ]'
assert "validate: a non-GNU-Bash binary is rejected with its banner" \
  '[ "$(probe "$BIN/notbash")" = "1|not-gnu-bash|zsh 5.9 (arm64-apple-darwin)" ]'
assert "validate: a relative path is rejected before any exec" \
  '[ "$(probe "bash")" = "1|not-absolute|" ]'
assert "validate: a missing file is rejected as not-executable" \
  '[ "$(probe "$BIN/does-not-exist")" = "1|not-executable|" ]'
assert "validate: a non-executable file is rejected as not-executable" \
  '[ "$(probe "$BIN/noexec")" = "1|not-executable|" ]'
assert "validate: a binary that cannot report a version is rejected" \
  '[ "$(probe "$BIN/novers")" = "1|no-version|" ]'

# --- depth anchoring of the runtime leaf (mutation target M6) -----------------
# The pre-0153 pattern `in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/` matched ANY
# indentation depth under the header, and in_runtime is cleared only by a column-0 non-space line.
# Measured pre-fix: `runtime:` -> `codex:` -> `bash: /opt/weird/bash` resolved to count=1,
# value=/opt/weird/bash. A user writing that means "the bash for the codex runner"; docket adopted
# it as the machine's Bash runtime for every operation, with no diagnostic.
printf 'runtime:\n  nested:\n    bash: /some/path\n' > "$tmp/deep.yml"
assert "M6 a leaf deeper than the block's shallowest child is not counted" \
  '[ "$(docket_runtime_count "$tmp/deep.yml")" = 0 ]'
assert "M6 a too-deep leaf yields no value" \
  '[ -z "$(docket_runtime_first "$tmp/deep.yml")" ]'
docket_runtime_unique "$tmp/deep.yml" >/dev/null 2>&1; deep_rc=$?
assert "M6 docket_runtime_unique reports a too-deep leaf with rc 3" '[ "$deep_rc" -eq 3 ]'

# The motivating hazard: a SIBLING key, not a decorative nesting.
printf 'runtime:\n  codex:\n    bash: /opt/weird/bash\n' > "$tmp/sibling.yml"
assert "M6 a bash: under a sibling nested key is not adopted" \
  '[ "$(docket_runtime_count "$tmp/sibling.yml")" = 0 ]'
docket_runtime_unique "$tmp/sibling.yml" >/dev/null 2>&1; sib_rc=$?
assert "M6 the sibling-key shape is reported with rc 3" '[ "$sib_rc" -eq 3 ]'

# DEPTH-RELATIVE, NOT TWO-SPACE: a four-space canonical file resolved before this change and must
# keep resolving. Hard-coding two spaces would be a second, unannounced tightening.
printf 'runtime:\n    bash: /four/space/bash\n' > "$tmp/four.yml"
assert "M6 a four-space one-level leaf still resolves" \
  '[ "$(docket_runtime_first "$tmp/four.yml")" = "/four/space/bash" ]'
assert "M6 the four-space file is counted once" \
  '[ "$(docket_runtime_count "$tmp/four.yml")" = 1 ]'

# The anchor is the SHALLOWEST structural child, not the FIRST: when the first child is the nested
# key, a first-child anchor lands too deep and would wrongly reject a later legitimate leaf.
printf 'runtime:\n    deep_first:\n      x: 1\n  bash: /correct/bash\n' > "$tmp/shallow-later.yml"
assert "M6 a one-level leaf after a deeper first child still resolves" \
  '[ "$(docket_runtime_first "$tmp/shallow-later.yml")" = "/correct/bash" ]'

# PER-BLOCK RESET: without it, block 2 inherits block 1's anchor.
printf 'runtime:\n  nested:\n    bash: /deep/one\nruntime:\n  bash: /good/two\n' > "$tmp/two-blocks.yml"
assert "M6 the depth anchor resets when in_runtime clears" \
  '[ "$(docket_runtime_first "$tmp/two-blocks.yml")" = "/good/two" ]'
assert "M6 block 2 is counted once despite block 1's deep leaf" \
  '[ "$(docket_runtime_count "$tmp/two-blocks.yml")" = 1 ]'

# UNCHANGED SHAPES — regression pins, expected green both ways.
assert "M6 the canonical two-space file is unchanged" \
  '[ "$(docket_runtime_first "$tmp/one.yml")" = "/one" ]'
assert "M6 a tab-indented leaf is still read" \
  '[ "$(docket_runtime_first "$tmp/tab.yml")" = "/tab/bash" ]'
assert "M6 a bash: at column 0 outside any runtime: block is still ignored" \
  '[ "$(docket_runtime_count "$tmp/decoy.yml")" = 0 ]'
assert "M6 the managed-block file is unchanged" \
  '[ "$(docket_runtime_count "$tmp/managed-only.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 0 ]'

# A deep leaf INSIDE a managed block must not be seen at all: managed lines `next` out before
# `structural` is computed, so depth tracking never sees them.
printf '%s\nruntime:\n  nested:\n    bash: /managed/deep\n%s\nruntime:\n  bash: /explicit/ok\n' \
  "$MARK_OPEN" "$MARK_CLOSE" > "$tmp/managed-deep.yml"
assert "M6 a deep leaf inside the managed block does not leak a DEEP report" \
  '[ "$(docket_runtime_count "$tmp/managed-deep.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 1 ]'

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
