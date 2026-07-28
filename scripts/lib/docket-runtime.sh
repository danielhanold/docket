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
