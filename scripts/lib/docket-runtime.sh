#!/usr/bin/env bash
# scripts/lib/docket-runtime.sh — the ONE implementation of docket's `runtime.bash` mechanics
# for install.sh, scripts/ensure-global-config.sh, and scripts/docket-config.sh (change 0133).
# SOURCE this; declaring functions is its only effect — no writes, no git, no network, no output
# at source time.
#
# After change 0152 (Task 7 routed scripts/ensure-docket-env.sh through this library), the ONLY
# independent Bash-version check left is scripts/docket.sh's POSIX `sh` bootstrap prologue, which
# runs before a Bash interpreter is even chosen. That duplication is PERMANENT BY DESIGN, not a
# folding-in backlog item — see that prologue's own comment block for why it cannot source this
# library. MAINTENANCE OBLIGATION: any change to the version grammar (the banner match or the
# major-version floor) made to docket_runtime_validate_bash below MUST also be applied to that
# prologue's copy, or vice versa — tests/test_bash_runtime_routing.sh's change-0152 equivalence
# guard drives both implementations with fake fixtures and reddens if they diverge.
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
