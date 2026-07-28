<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0133 — Centralize shared Bash runtime configuration helpers](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0133-centralize-runtime-config-helpers.md)**
<!-- docket:backlink:end -->

# Centralize Shared Bash Runtime Configuration Helpers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the duplicated `runtime.bash` parser, counter, serializability check, and GNU Bash 4+ validator in `install.sh`, `scripts/ensure-global-config.sh`, and `scripts/docket-config.sh` with one bootstrap-compatible shared library, preserving every caller's current authority, precedence, and diagnostics.

**Architecture:** A new source-only `scripts/lib/docket-runtime.sh` owns the reusable mechanics — one awk scanner for `runtime:`-block traversal and YAML scalar decoding, plus a validator and a serializability check. It exposes small namespaced functions shaped to each caller's existing semantics (first-value, count, unique-or-error, validate). Authority, discovery, marker rewriting, layer precedence, and all user-facing diagnostics stay in the callers. The library must parse and run under Bash 3.2, because two of its three callers run before a configured Bash 4+ runtime exists.

**Tech Stack:** POSIX-ish Bash (3.2-compatible in the library), `awk`, `sed`. No new dependencies.

## Global Constraints

Every task's requirements implicitly include this section.

- **The library is Bash 3.2-compatible.** `scripts/lib/docket-runtime.sh` must parse and execute under `/bin/bash` (3.2.57 on macOS). Forbidden: associative arrays (`declare -A`), `mapfile`/`readarray`, `${x^^}`/`${x,,}`, `declare -g`, `;;&` in `case`, negative array subscripts. Permitted and used: `local`, `[[ ]]`, `<<<`, `$'\n'`, `$( )`.
- **No top-level side effects in the library.** Sourcing it declares functions only — no writes, no git, no network, no output.
- **The configured-runtime contract is unchanged.** A configured `runtime.bash` value must still be an absolute, executable, GNU Bash **version 4 or newer**. Bash 3.2 is a bootstrap-execution allowance for the library, never an accepted configured runtime.
- **Caller policy is not absorbed into the library.** Discovery order, managed-marker validation/rewriting, explicit-vs-managed authority, repo-local > global precedence, the committed-key fence, and every user-facing message stay in the caller that owns them today.
- **No YAML-parser adoption.** Change 0018 remains the place to evaluate `yq`; this change only deduplicates the existing hand-rolled subset parser.
- **No new script contract file.** `tests/test_script_contracts_coverage.sh` scopes `scripts/lib/*.sh` out (its glob `scripts/*.sh` never matches `/`). Do **not** create `scripts/lib/docket-runtime.md`; the library is documented in its own header and in its callers' contracts, matching `scripts/lib/docket-root.sh`.
- **Comment-anchor rule (AGENTS.md, enforced by `tests/test_comment_anchor_style.sh`):** cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause — never `file:line`.
- **Whole-suite gate.** Run every test, not only the ones this plan names:
  `for t in tests/test_*.sh; do GIT_EDITOR=true bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || echo "FAIL: $t"; done`

---

## File Structure

| File | Disposition | Responsibility |
|---|---|---|
| `scripts/lib/docket-runtime.sh` | **Create** | Sole implementation of `runtime:`-block traversal, YAML scalar decoding, declaration counting, path serializability, and GNU Bash 4+ validation. Bootstrap-compatible. |
| `tests/test_docket_runtime_lib.sh` | **Create** | Direct unit coverage of the library, including a real Bash 3.2 execution witness and the mutation matrix. |
| `scripts/ensure-global-config.sh` | Modify | Keeps discovery, markers, authority checks, messages. Four helper bodies become library calls. |
| `scripts/docket-config.sh` | Modify | Keeps precedence, the committed fence, and all five runtime diagnostics. Parsing/counting/validation delegate. |
| `install.sh` | Modify | Its 32-line inline awk becomes one library call. |
| `scripts/docket-config.md`, `scripts/ensure-global-config.md` | Modify | Note the shared library as the parsing/validation source. |

---

### Task 1: The shared scanner

Creates the library with its traversal + scalar-decoding scanner and the three read shapes the callers need. No caller is rewired yet — this task lands the library and proves it standalone.

**Files:**
- Create: `scripts/lib/docket-runtime.sh`
- Test: `tests/test_docket_runtime_lib.sh` (create)

**Interfaces:**
- Consumes: nothing.
- Produces, for Tasks 2–5:
  - `_docket_runtime_scan <file> [open_marker] [close_marker]` — private; sets globals `DOCKET_RUNTIME_COUNT` (integer) and `DOCKET_RUNTIME_VALUE` (string, first declaration's decoded scalar). Returns 0 always. Absent file ⇒ `0` / empty.
  - `docket_runtime_count <file> [open_marker] [close_marker]` — prints the declaration count. Returns 0.
  - `docket_runtime_first <file> [open_marker] [close_marker]` — prints the first declaration's decoded value (empty if none). Returns 0.
  - `docket_runtime_unique <file> [open_marker] [close_marker]` — prints the value when count ≤ 1; **returns 2 and prints nothing** when count > 1.
  - Marker arguments are optional. **Empty marker strings disable marker handling** — they must never match a blank input line.

- [ ] **Step 1: Write the failing test**

Create `tests/test_docket_runtime_lib.sh`:

```bash
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
```

Note: the Bash 3.2 block calls `docket_runtime_serializable` and `docket_runtime_validate_bash`, which arrive in Task 2. Until then it will fail — that is expected; Task 1 Step 2 confirms the *scanner* assertions fail for the right reason (no library at all).

`LEGACY_SELF` is exported just before the legacy block; add this line immediately above `legacy_out=`:

```bash
  export LEGACY_SELF="$LEGACY"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_runtime_lib.sh`
Expected: FAIL — sourcing errors ("No such file or directory") and `NOT OK` lines, because `scripts/lib/docket-runtime.sh` does not exist. Exit non-zero.

- [ ] **Step 3: Write the library scanner**

Create `scripts/lib/docket-runtime.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/docket-runtime.sh — the ONE implementation of docket's `runtime.bash` mechanics
# (change 0133). SOURCE this; declaring functions is its only effect — no writes, no git, no
# network, no output at source time.
#
# BOOTSTRAP-COMPATIBLE BY REQUIREMENT. install.sh and scripts/ensure-global-config.sh source this
# BEFORE a configured GNU Bash 4+ runtime has been discovered or persisted, so every line here must
# parse and run under the system Bash (macOS ships 3.2.57). Forbidden: associative arrays,
# mapfile/readarray, ${x^^}/${x,,}, declare -g, `;;&`. This is an allowance for the library's own
# EXECUTION only — it does not relax docket's requirement that the CONFIGURED runtime be Bash 4+,
# which docket_runtime_validate_bash still enforces.
#
# The library owns reusable MECHANICS only. Authority, discovery order, managed-block rewriting,
# layer precedence, and every user-facing diagnostic stay in the caller — which is why the
# validator returns a machine-readable reason token instead of printing a message.
#
#   docket_runtime_count  <file> [open] [close]  -> declaration count on stdout (0 if absent)
#   docket_runtime_first  <file> [open] [close]  -> first declaration's decoded value (empty if none)
#   docket_runtime_unique <file> [open] [close]  -> the value; returns 2 (prints nothing) if count>1
#   docket_runtime_serializable <value>          -> 0 iff representable as a one-line YAML scalar
#   docket_runtime_validate_bash <path>          -> line1 reason token, line2 version line; 0 iff ok
#
# `open`/`close` are the caller's installer-managed marker strings; supplying them EXCLUDES that
# block from the scan. Empty or omitted markers disable marker handling entirely and must never
# match a blank line.
#
# This file is a sourced helper: it is documented in its callers' contracts (docket-config.md,
# ensure-global-config.md), not by a co-located .md (test_script_contracts_coverage.sh scopes
# scripts/lib/ out).

# Scan <file> for `runtime:` -> `bash:` declarations. Sets DOCKET_RUNTIME_COUNT and
# DOCKET_RUNTIME_VALUE (the FIRST declaration's decoded scalar). Always returns 0.
#
# The `printf x` guard is load-bearing: `$( )` strips ALL trailing newlines, so an empty value
# would collapse the two-line payload to one line and the count would be read back as the value.
_docket_runtime_scan(){ # _docket_runtime_scan <file> [open] [close]
  local _raw
  DOCKET_RUNTIME_COUNT=0
  DOCKET_RUNTIME_VALUE=""
  [ -f "$1" ] || return 0
  _raw="$(awk -v o="${2-}" -v c="${3-}" '
    function scalar(value, sq,out,i,ch,rest) {
      sq=sprintf("%c", 39)
      if (substr(value,1,1) == sq) {
        out=""
        for (i=2; i<=length(value); i++) {
          ch=substr(value,i,1)
          if (ch == sq) {
            if (substr(value,i+1,1) == sq) { out=out sq; i++; continue }
            rest=substr(value,i+1)
            if (rest ~ /^[[:space:]]*(#.*)?$/) return out
            return value
          }
          out=out ch
        }
        return value
      }
      if (value ~ /^"[^"]*"[[:space:]]*(#.*)?$/) {
        sub(/^"/, "", value); sub(/"[[:space:]]*(#.*)?$/, "", value)
      } else {
        sub(/[[:space:]]*#.*/, "", value); sub(/[[:space:]]+$/, "", value)
      }
      return value
    }
    # An EMPTY marker must never match a blank input line, so guard both marker rules on o/c
    # being non-empty. A managed line never touches in_runtime, so a managed `runtime:` header
    # cannot leak the block state past the closing marker.
    o != "" && $0==o { managed=1; next }
    c != "" && $0==c { managed=0; next }
    managed { next }
    { raw=$0; structural=$0; sub(/[[:space:]]*#.*/, "", structural) }
    structural ~ /^runtime[[:space:]]*:[[:space:]]*$/ { in_runtime=1; next }
    in_runtime && structural ~ /^[^[:space:]]/ { in_runtime=0 }
    in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/ {
      count++
      if (count == 1) {
        value=raw; sub(/^[[:space:]]+bash[[:space:]]*:[[:space:]]*/, "", value)
        first=scalar(value)
      }
    }
    END { printf "%d\n%s\n", count+0, first }
  ' "$1"; printf 'x')"
  _raw="${_raw%x}"
  DOCKET_RUNTIME_COUNT="${_raw%%$'\n'*}"
  _raw="${_raw#*$'\n'}"
  DOCKET_RUNTIME_VALUE="${_raw%$'\n'}"
  return 0
}

docket_runtime_count(){ # docket_runtime_count <file> [open] [close]
  _docket_runtime_scan "$@"
  printf '%s\n' "$DOCKET_RUNTIME_COUNT"
}

docket_runtime_first(){ # docket_runtime_first <file> [open] [close]
  _docket_runtime_scan "$@"
  printf '%s\n' "$DOCKET_RUNTIME_VALUE"
}

# Duplicates are an AMBIGUITY, not a precedence question: callers require exactly one authority
# per layer, so more than one declaration is reported (return 2) rather than resolved.
docket_runtime_unique(){ # docket_runtime_unique <file> [open] [close]
  _docket_runtime_scan "$@"
  [ "$DOCKET_RUNTIME_COUNT" -le 1 ] || return 2
  printf '%s\n' "$DOCKET_RUNTIME_VALUE"
}
```

- [ ] **Step 4: Run the test to verify the scanner assertions pass**

Run: `bash tests/test_docket_runtime_lib.sh`
Expected: every scalar, block-constraint, counting, uniqueness, and marker assertion prints `ok - …`. The bash-3.2 witness still prints `NOT OK` (it calls the Task 2 functions). Exit non-zero, from that one assertion only.

Verify explicitly that only the legacy assertion fails:

Run: `bash tests/test_docket_runtime_lib.sh | grep -c '^NOT OK'`
Expected: `1`

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/docket-runtime.sh tests/test_docket_runtime_lib.sh
git commit -m "refactor(0133): add the bootstrap-compatible runtime scanner library"
```

---

### Task 2: The validator and serializability check

Adds the two remaining primitives. The validator returns a **reason token** so both callers keep their own diagnostics — `ensure-global-config.sh` has one message for every failure mode, `docket-config.sh` has five distinct ones.

**Files:**
- Modify: `scripts/lib/docket-runtime.sh` (append after `docket_runtime_unique`)
- Test: `tests/test_docket_runtime_lib.sh` (extend)

**Interfaces:**
- Consumes: nothing from Task 1 at runtime; appends to the same file.
- Produces, for Tasks 3–5:
  - `docket_runtime_serializable <value>` — returns 0 iff the value contains no CR and no LF. No output.
  - `docket_runtime_validate_bash <path>` — prints the reason token on line 1 and the `--version` first line on line 2 (empty when unobtained); returns 0 iff the token is `ok`. Tokens, in evaluation order: `not-absolute`, `not-executable`, `no-version`, `not-gnu-bash`, `old-major`, `ok`. `old-major` covers both an unparseable major and a major below 4 — matching the resolver's existing single "must be Bash 4 or newer" die.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_runtime_lib.sh`, immediately **before** the `# --- bootstrap compatibility` block:

```bash
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
  out="$(docket_runtime_validate_bash "$1"; printf 'x')"; rc=$?
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_runtime_lib.sh`
Expected: FAIL — the new `M5 …` and `validate: …` assertions print `NOT OK` ("command not found" for both new functions).

- [ ] **Step 3: Write the implementation**

Append to `scripts/lib/docket-runtime.sh`:

```bash
# A runtime path is stored as a ONE-LINE YAML scalar. write_runtime_block doubles apostrophes and
# YAML single quotes keep backslashes literal, so only record separators are unrepresentable.
docket_runtime_serializable(){ # docket_runtime_serializable <value>
  case "$1" in *$'\n'*|*$'\r'*) return 1 ;; esac
  return 0
}

# Validate that <path> is an absolute, executable GNU Bash 4 or newer. Prints a machine-readable
# reason token on line 1 and the binary's `--version` first line on line 2 (empty when it was not
# obtained), so a caller can build its OWN diagnostic without re-running the binary. `old-major`
# deliberately covers both an unparseable major and a major below 4: the resolver has always
# collapsed those into one "must be Bash 4 or newer" failure, and splitting them here would invent
# a distinction no caller makes.
docket_runtime_validate_bash(){ # docket_runtime_validate_bash <path>
  local _p="$1" _version _first _major
  case "$_p" in /*) ;; *) printf '%s\n%s\n' not-absolute ""; return 1 ;; esac
  [ -x "$_p" ] || { printf '%s\n%s\n' not-executable ""; return 1; }
  _version="$(LC_ALL=C "$_p" --version 2>/dev/null)" \
    || { printf '%s\n%s\n' no-version ""; return 1; }
  _first="${_version%%$'\n'*}"
  case "$_first" in
    'GNU bash, version '*) ;;
    *) printf '%s\n%s\n' not-gnu-bash "$_first"; return 1 ;;
  esac
  _major="$(printf '%s\n' "$_first" | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')"
  case "$_major" in
    ''|*[!0-9]*) printf '%s\n%s\n' old-major "$_first"; return 1 ;;
  esac
  [ "$_major" -ge 4 ] || { printf '%s\n%s\n' old-major "$_first"; return 1; }
  printf '%s\n%s\n' ok "$_first"
  return 0
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_runtime_lib.sh`
Expected: every assertion `ok - …`, including `ok - bash 3.2: library parses and every read shape works`. Exit 0.

If the bash-3.2 assertion fails, the library used a Bash-4-only construct — fix the library, never the assertion.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/docket-runtime.sh tests/test_docket_runtime_lib.sh
git commit -m "refactor(0133): add the shared Bash 4+ validator and serializability check"
```

---

### Task 3: Route `ensure-global-config.sh` through the library

Replaces four helper **bodies** with library calls. Discovery, marker validation and rewriting, explicit-versus-managed authority, and every message stay exactly as they are.

**Files:**
- Modify: `scripts/ensure-global-config.sh` — replace `validate_runtime` (lines 17-27), `validate_serializable_path` (lines 29-34), `explicit_runtime` (lines 45-82), `explicit_runtime_count` (lines 85-97); add the library source after the constants block
- Test: `tests/test_ensure_global_config.sh`, `tests/test_bash_runtime_install.sh` (both must pass **unchanged** — that is the point)

**Interfaces:**
- Consumes: `docket_runtime_first`, `docket_runtime_count`, `docket_runtime_serializable`, `docket_runtime_validate_bash`.
- Produces: nothing new. `markers_valid`, `consider_candidate`, `discover_runtime`, `write_pointer`, `write_runtime_block`, `strip_runtime_block` and the whole main flow are untouched.

- [ ] **Step 1: Run the existing tests to record the green baseline**

Run: `bash tests/test_ensure_global_config.sh; echo "rc=$?"; bash tests/test_bash_runtime_install.sh; echo "rc=$?"`
Expected: both `rc=0`, no `NOT OK` lines. These are the regression oracle for this task — they must still be green afterwards with **zero edits**.

- [ ] **Step 2: Source the library**

In `scripts/ensure-global-config.sh`, immediately after the `die` / `file_mode` helper lines (after the `file_mode(){ … }` line), insert:

```bash
# The runtime.bash parser and the Bash 4+ validator are shared with scripts/docket-config.sh and
# install.sh (change 0133). The library is bootstrap-compatible for exactly this call site: it is
# sourced before a configured Bash 4+ runtime exists.
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/docket-runtime.sh
. "$SELF_DIR/lib/docket-runtime.sh"
```

- [ ] **Step 3: Replace the four helper bodies**

Replace `validate_runtime` and `validate_serializable_path` with:

```bash
# Policy stays here: this script has ONE diagnostic for every invalid-runtime mode, so it discards
# the library's reason token. scripts/docket-config.sh consumes the same token to build five.
validate_runtime(){ docket_runtime_validate_bash "$1" >/dev/null; }

validate_serializable_path(){ docket_runtime_serializable "$1"; }
```

Replace `explicit_runtime` and `explicit_runtime_count` with:

```bash
# "Explicit" means user-authored: the installer-managed block is excluded, so this script can tell
# its own previous write apart from a hand-authored authority. That exclusion is the only reason
# these two pass markers and the resolver's equivalents do not.
explicit_runtime(){ docket_runtime_first "$1" "$MARK_OPEN" "$MARK_CLOSE"; }

explicit_runtime_count(){ docket_runtime_count "$1" "$MARK_OPEN" "$MARK_CLOSE"; }
```

Delete the awk bodies these replace. Leave `markers_valid` alone — marker *validation* is installer-owned policy, not shared mechanics.

- [ ] **Step 4: Run the tests to verify they still pass**

Run: `bash tests/test_ensure_global_config.sh; echo "rc=$?"; bash tests/test_bash_runtime_install.sh; echo "rc=$?"`
Expected: both `rc=0`, byte-identical output to the Step 1 baseline.

Also confirm the duplication is actually gone:

Run: `grep -c 'function scalar' scripts/ensure-global-config.sh`
Expected: `0`

- [ ] **Step 5: Commit**

```bash
git add scripts/ensure-global-config.sh
git commit -m "refactor(0133): route ensure-global-config.sh through the shared runtime library"
```

---

### Task 4: Route `docket-config.sh` through the library

The resolver keeps repo-local > global precedence, the committed-key fence, its duplicate-declaration `die`s, and all five distinct runtime diagnostics — it only stops owning a second copy of the parser and validator.

**Files:**
- Modify: `scripts/docket-config.sh` — add the library source beside the existing three (after the `docket-frontmatter.sh` source, ~line 39); replace `runtime_get` (lines 129-165) and `runtime_count` (lines 170-179); replace the inline validation chain (lines 281-299, from `[ -n "$DOCKET_BASH_PATH" ]` through the `must be Bash 4 or newer` die)
- Test: `tests/test_docket_config.sh` (must pass **unchanged**), `tests/test_bash_runtime_routing.sh`

**Interfaces:**
- Consumes: `docket_runtime_unique`, `docket_runtime_count`, `docket_runtime_serializable`, `docket_runtime_validate_bash`.
- Produces: nothing new. `DOCKET_BASH_PATH` resolution, ordering in the export block, and every message string are unchanged.

- [ ] **Step 1: Run the existing tests to record the green baseline**

Run: `bash tests/test_docket_config.sh; echo "rc=$?"`
Expected: `rc=0`, no `NOT OK`. This suite carries the whole `0132 runtime …` block — global/local/committed precedence, the ambiguity abort, all five invalid cases, absence, odd paths, and the CR control byte. It is the oracle for this task.

- [ ] **Step 2: Source the library**

In `scripts/docket-config.sh`, after the `. "$SELF_DIR/lib/docket-frontmatter.sh"` line, insert:

```bash
# change 0133: the single implementation of runtime.bash parsing, counting, and Bash 4+
# validation, shared with install.sh and scripts/ensure-global-config.sh. Definitions only.
. "$SELF_DIR/lib/docket-runtime.sh"
```

- [ ] **Step 3: Replace the two parser helpers**

Replace the `runtime_get` and `runtime_count` definitions (keeping the explanatory comments above them) with:

```bash
# Read one scalar from one top-level block without treating `#` inside a quoted value as a
# comment. Duplicate leaves (including two separate runtime blocks) are an ambiguity, not
# precedence: callers require exactly one authority per layer. The resolver passes NO markers —
# it has no managed block; only the installer excludes one.
runtime_get() { # runtime_get <file>
  docket_runtime_unique "$1"
}

# Count runtime.bash declarations without parsing their values. The committed layer is fenced by
# key presence, so even empty, malformed, or duplicate committed values are warning-only and can
# never block a valid machine-local fallback.
runtime_count() { # runtime_count <file>
  docket_runtime_count "$1"
}
```

- [ ] **Step 4: Replace the inline validation chain**

Replace everything from `[ -n "$DOCKET_BASH_PATH" ] \` through the `must be Bash 4 or newer` die with:

```bash
[ -n "$DOCKET_BASH_PATH" ] \
  || die "runtime.bash is not configured — $_runtime_remedy"
docket_runtime_serializable "$DOCKET_BASH_PATH" \
  || die "runtime.bash must not contain carriage returns or newlines — $_runtime_remedy"
# The shared validator returns a reason token; the five diagnostics below are resolver-owned
# policy and are why it returns a token instead of printing a message. The `printf x` guard keeps
# an empty version line from collapsing the two-line payload.
_runtime_probe="$(docket_runtime_validate_bash "$DOCKET_BASH_PATH"; printf 'x')"
_runtime_probe="${_runtime_probe%x}"
_runtime_reason="${_runtime_probe%%$'\n'*}"
_runtime_probe="${_runtime_probe#*$'\n'}"
_runtime_first_line="${_runtime_probe%$'\n'}"
case "$_runtime_reason" in
  ok) ;;
  not-absolute)
    die "runtime.bash must be an absolute path, got '$DOCKET_BASH_PATH' — $_runtime_remedy" ;;
  not-executable)
    die "runtime.bash is not an executable file: $DOCKET_BASH_PATH — $_runtime_remedy" ;;
  no-version)
    die "runtime.bash could not report its version: $DOCKET_BASH_PATH — $_runtime_remedy" ;;
  not-gnu-bash)
    die "runtime.bash did not identify itself as GNU Bash: $DOCKET_BASH_PATH reported '${_runtime_first_line:-no version}' — $_runtime_remedy" ;;
  old-major)
    die "runtime.bash must be Bash 4 or newer, got '${_runtime_first_line:-unknown version}' from $DOCKET_BASH_PATH — $_runtime_remedy" ;;
  *)
    die "runtime.bash validation returned an unrecognized result '$_runtime_reason' for $DOCKET_BASH_PATH — $_runtime_remedy" ;;
esac
```

Note the order is preserved deliberately: configured → serializable → absolute → executable → version. `tests/test_docket_config.sh`'s `0132 runtime control byte` case depends on CR/LF being rejected **before** the binary is executed.

- [ ] **Step 5: Run the tests to verify they still pass**

Run: `bash tests/test_docket_config.sh; echo "rc=$?"; bash tests/test_bash_runtime_routing.sh; echo "rc=$?"`
Expected: both `rc=0`, output byte-identical to the Step 1 baseline.

Confirm the duplication is gone:

Run: `grep -c 'function scalar' scripts/docket-config.sh`
Expected: `0`

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-config.sh
git commit -m "refactor(0133): route docket-config.sh through the shared runtime library"
```

---

### Task 5: Route `install.sh` through the library

Deletes the third copy of the scalar parser. `install.sh` performs a plain post-bootstrap read of the value `ensure-global-config.sh` just wrote or validated — no markers, no duplicate handling, because the preceding stage already guaranteed exactly one authority or exited non-zero.

**Files:**
- Modify: `install.sh` — source the library after `SCRIPT_DIR`; replace the `DOCKET_BASH_PATH="$(awk …)"` block (lines 27-58)
- Test: `tests/test_install.sh`, `tests/test_bash_runtime_install.sh`

**Interfaces:**
- Consumes: `docket_runtime_first`.
- Produces: nothing new. `DOCKET_BASH_PATH` is still exported and still launches the three downstream scripts.

- [ ] **Step 1: Run the existing tests to record the green baseline**

Run: `bash tests/test_install.sh; echo "rc=$?"`
Expected: `rc=0`, no `NOT OK`.

- [ ] **Step 2: Write the implementation**

In `install.sh`, after the `SCRIPT_DIR=` line, insert:

```bash
# change 0133: the shared runtime.bash reader. Sourced here rather than re-implemented so a fix to
# the scalar grammar reaches install, the installer, and the resolver together. Bootstrap-
# compatible by requirement — this runs under the system Bash, before DOCKET_BASH_PATH exists.
# shellcheck source=scripts/lib/docket-runtime.sh
. "$SCRIPT_DIR/scripts/lib/docket-runtime.sh"
```

Then replace the whole `DOCKET_BASH_PATH="$(awk …)"` assignment (the 32-line awk block) with:

```bash
# No markers and no duplicate handling: ensure-global-config.sh has just guaranteed exactly one
# authoritative declaration or exited non-zero, so this reads the value it settled on — managed or
# hand-authored alike.
DOCKET_BASH_PATH="$(docket_runtime_first "$CONFIG_ROOT/docket/config.yml")"
```

Keep the `CONFIG_ROOT=` line and the `export DOCKET_BASH_PATH` line exactly as they are.

- [ ] **Step 3: Run the tests to verify they still pass**

Run: `bash tests/test_install.sh; echo "rc=$?"; bash tests/test_bash_runtime_install.sh; echo "rc=$?"`
Expected: both `rc=0`, output byte-identical to the Step 1 baseline.

- [ ] **Step 4: Verify no copy of the parser survives anywhere**

Run: `grep -rn 'function scalar' --include='*.sh' . | grep -v '^./.git'`
Expected: exactly one hit — `./scripts/lib/docket-runtime.sh`.

Also confirm `install.sh` still runs under the system Bash (it is invoked by users before any configured runtime exists):

Run: `/bin/bash -n install.sh && echo "parses under bash 3.2"`
Expected: `parses under bash 3.2`

Run: `/bin/bash -n scripts/lib/docket-runtime.sh && /bin/bash -n scripts/ensure-global-config.sh && echo "bootstrap chain parses under bash 3.2"`
Expected: `bootstrap chain parses under bash 3.2`

- [ ] **Step 5: Commit**

```bash
git add install.sh
git commit -m "refactor(0133): route install.sh through the shared runtime library"
```

---

### Task 6: Mutation matrix and contract docs

Proves the shared scanner and validator are load-bearing — deleting any one of the five properties they own must redden a focused test — and records the library in the two callers' contracts.

**Files:**
- Modify: `scripts/docket-config.md`, `scripts/ensure-global-config.md`
- Test: `tests/test_docket_runtime_lib.sh` (verification only, no new asserts unless a mutation survives)

**Interfaces:**
- Consumes: everything from Tasks 1–5.
- Produces: nothing.

- [ ] **Step 1: Run the mutation matrix**

For each row: apply the mutation to `scripts/lib/docket-runtime.sh`, run the named test, confirm it goes **red**, then `git checkout -- scripts/lib/docket-runtime.sh`.

| # | Property removed | Mutation | Must redden |
|---|---|---|---|
| M1 | runtime-block constraint | Delete the `structural ~ /^runtime…/ { in_runtime=1; next }` rule and change the leaf rule's guard from `in_runtime && structural ~ /^[[:space:]]+bash…/` to `structural ~ /bash[[:space:]]*:/` | `tests/test_docket_runtime_lib.sh` — the `M1 …` asserts |
| M2 | duplicate detection | In `docket_runtime_unique`, delete the `[ "$DOCKET_RUNTIME_COUNT" -le 1 ] \|\| return 2` line | `tests/test_docket_runtime_lib.sh` — the `M2 unique: …` asserts; **and** `tests/test_docket_config.sh` — `0132 runtime authority: duplicate global declarations abort` |
| M3 | marker exclusion | Delete the two `o != "" && $0==o` / `c != "" && $0==c` rules and the `managed { next }` rule | `tests/test_docket_runtime_lib.sh` — the `M3 …` asserts; **and** `tests/test_ensure_global_config.sh` |
| M4 | Bash-major check | In `docket_runtime_validate_bash`, change `[ "$_major" -ge 4 ]` to `[ "$_major" -ge 0 ]` | `tests/test_docket_runtime_lib.sh` — `M4 validate: Bash 3.2 is rejected as old-major`; **and** `tests/test_docket_config.sh` — `0132 runtime invalid legacy: resolver aborts` |
| M5 | serializability | In `docket_runtime_serializable`, delete the `case` line so it always returns 0 | `tests/test_docket_runtime_lib.sh` — the `M5 serializable: …` rejection asserts; **and** `tests/test_docket_config.sh` — `0132 runtime control byte: …` |

Record each cell's observed result. **A mutation that leaves everything green is a defect** — add a focused assert that catches it before continuing, per AGENTS.md ("a guard is code: mutation-test it… or it is decoration").

Also mutation-test the empty-marker guard specifically, since it is a new behavior with no predecessor:

| # | Property removed | Mutation | Must redden |
|---|---|---|---|
| M6 | empty-marker guard | Change `o != "" && $0==o` to `$0==o` and `c != "" && $0==c` to `$0==c` | `tests/test_docket_runtime_lib.sh` — `empty markers do not match blank lines` |

- [ ] **Step 2: Verify the working tree is clean after the matrix**

Run: `git status --porcelain scripts/lib/docket-runtime.sh`
Expected: no output — every mutation was reverted.

- [ ] **Step 3: Update the two script contracts**

In `scripts/docket-config.md`, add to its Behavior or Invariants section:

```markdown
- `runtime.bash` parsing, declaration counting, and GNU Bash 4+ validation are delegated to the
  shared `scripts/lib/docket-runtime.sh` (change 0133). The resolver keeps its own policy: repo-local
  > global precedence, the committed-key fence, the duplicate-declaration abort, and the five
  distinct runtime diagnostics it builds from the library's reason token.
```

In `scripts/ensure-global-config.md`, add to its Behavior or Invariants section:

```markdown
- The `runtime.bash` scalar parser, declaration counter, path-serializability check, and GNU Bash 4+
  validator come from the shared `scripts/lib/docket-runtime.sh` (change 0133). Discovery order,
  managed-marker validation and rewriting, explicit-versus-managed authority, and this script's
  single invalid-runtime diagnostic remain owned here. The library is bootstrap-compatible because
  this script runs before a configured Bash 4+ runtime exists.
```

- [ ] **Step 4: Run the whole suite**

Run as **one foreground command**:

```bash
fail=0; for t in tests/test_*.sh; do GIT_EDITOR=true bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
```

Expected: `suite fail=0`, with no `FAIL:` lines.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.md scripts/ensure-global-config.md
git commit -m "docs(0133): record the shared runtime library in the caller contracts"
```

---

## Self-Review

**Spec coverage.**

| Spec requirement | Task |
|---|---|
| Bootstrap-compatible `scripts/lib/docket-runtime.sh`, source-only, no side effects | 1 |
| Scan `runtime:` block, decode quoted/unquoted one-line `bash:` scalars | 1 |
| Count declarations; report ambiguity distinctly from absence | 1 (`docket_runtime_count` / `docket_runtime_unique` returning 2) |
| Optionally exclude a caller-supplied marker block | 1 |
| Validate one-line-YAML serializability | 2 |
| Validate absolute executable GNU Bash 4+ without caller-specific diagnostics | 2 (reason token) |
| Sole implementation of scalar decoding and block traversal | 1, verified in 5 Step 4 |
| Semantic shapes: unique-value (resolver), first-plus-count (installer), normal read (install) | 1 |
| `ensure-global-config.sh` keeps discovery/markers/authority/messages | 3 |
| `docket-config.sh` keeps precedence/fence/duplicate messages/diagnostics | 4 |
| `install.sh` uses the shared read, still routes through `DOCKET_BASH_PATH` | 5 |
| No caller imports discovery or managed-block mutation | 3 (`markers_valid`, `discover_runtime`, `write_runtime_block` untouched) |
| Unit coverage: decoding, duplicates, marker exclusion, blank values, quoted inline comments, apostrophes, backslashes | 1, 2 |
| Bootstrap invocation exercised under Bash 3.2 syntax | 1 (real `/bin/bash` witness), 5 Step 4 (`-n` parse checks) |
| Existing installer/install/resolver tests preserved and still proving authority, precedence, diagnostics, `DOCKET_BASH_PATH` routing | 3, 4, 5 — each runs its suite unchanged against a recorded baseline |
| Mutation-test block constraint, duplicate detection, marker exclusion, Bash-major check | 6 (M1–M4, plus M5/M6) |
| Full repository suite at the build gate | 6 Step 4 |
| Non-goal: no `yq` / no discovery-order change / no Bash 3.2 as configured runtime | Global Constraints |

No gaps.

**Placeholder scan.** No TBD/TODO, no "add appropriate error handling", no "similar to Task N", no test described without its code. Every code step carries the literal content.

**Type consistency.** Function names used identically across tasks: `_docket_runtime_scan`, `docket_runtime_count`, `docket_runtime_first`, `docket_runtime_unique`, `docket_runtime_serializable`, `docket_runtime_validate_bash`; globals `DOCKET_RUNTIME_COUNT` / `DOCKET_RUNTIME_VALUE`; reason tokens `ok` / `not-absolute` / `not-executable` / `no-version` / `not-gnu-bash` / `old-major` are spelled the same in Task 2's implementation, Task 2's asserts, and Task 4's `case`.

**Known cross-task ordering.** Task 1's test file includes the Bash 3.2 witness that calls Task 2's functions, so Task 1 Step 4 ends with exactly one failing assertion. This is called out at both steps and resolves at Task 2 Step 4.
