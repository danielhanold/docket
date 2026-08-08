#!/usr/bin/env bash
# scripts/lib/harness-defaults.sh — reader + structural validator for agents/harness-defaults.yml,
# docket's SHIPPED per-harness agent model/effort default layer (change 0168).
#
# Sourced by sync-agents.sh. Validation runs BEFORE any wrapper is written, so a malformed sidecar
# can never leave a half-regenerated agent directory.
#
# The file's shape is fixed and shallow (agents: -> <harness>: -> <agent>: { model: , effort: }),
# so these readers parse it directly rather than pulling in a YAML dependency. They deliberately do
# NOT reuse sync-agents.sh's section_body/field_of: those read USER config, whose shape is looser,
# and coupling the shipped-data reader to them would let a user-config change silently reshape
# program data.
#
# Every pipeline here ends in a consumer that drains its input. Under `set -o pipefail` (which
# sync-agents.sh sets) a `producer | head -n1` would take SIGPIPE and turn into an intermittent
# 141; single-line selection is done with shell parameter expansion instead.

# Known harness tokens — what may APPEAR in the sidecar at all. Adding one here is not enough to
# ship defaults for it; the emitter must also know how to write the harness's wrapper.
HD_KNOWN_HARNESSES="claude cursor codex opencode"

# Harnesses docket actually SHIPS defaults for. Each must carry a COMPLETE block (every
# agents/docket-*.md). A known-but-unshipped harness — one listed in HD_KNOWN_HARNESSES but absent
# here — may hold no block at all; the moment it holds one, listing it here is what makes partial
# coverage an error rather than a silent half-pinned harness. Sparseness is a property of WHICH
# harnesses appear, never of how much of one appears. All four known harnesses ship complete
# blocks today.
HD_SHIPPED_HARNESSES="claude cursor codex opencode"

# Print the body lines under `  <harness>:` (four-space-indented entries), comments stripped.
_hd_block(){ # $1=file $2=harness
  [ -f "$1" ] || return 0
  awk -v h="$2" '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ "^  "h"[[:space:]]*:[[:space:]]*$" { inb=1; next }
    inb && nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:/ { inb=0 }
    inb && nc ~ /^    [A-Za-z0-9._-]+[[:space:]]*:/ { print nc }
  ' "$1"
}

# Print the harness keys (two-space-indented, bare) present under agents:, in FILE ORDER, with
# duplicates INTACT. Only the duplicate-block guard wants this view; everything else wants the set.
_hd_harness_keys(){ # $1=file
  [ -f "$1" ] || return 0
  awk '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ {
      k=nc; sub(/^  /,"",k); sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }' "$1"
}

# Print the harness keys present under agents: as a SET (sorted, unique).
# Counting repeats against this is meaningless — `sort -u` collapses them — so the
# duplicate-harness-block guard in hd_validate counts against _hd_harness_keys instead.
hd_harnesses(){ # $1=file
  _hd_harness_keys "$1" | sort -u
}

# Print the agent short-names under <harness>, in FILE ORDER. Callers that need a set comparison
# sort explicitly.
hd_agents(){ # $1=file $2=harness
  _hd_block "$1" "$2" | sed -e 's/^    //' -e 's/[[:space:]]*:.*//'
}

# Print the entry line for (harness, agent), or nothing. Shared by the two field readers so they
# can never disagree about WHICH line they are reading.
_hd_entry_line(){ # $1=file $2=harness $3=agent
  local block line
  block="$(_hd_block "$1" "$2")"
  [ -n "$block" ] || return 0
  line="$(grep -E "^    $3[[:space:]]*:" <<<"$block" || true)"
  [ -n "$line" ] || return 0
  printf '%s' "${line%%$'\n'*}"                # first match only; no `| head` under pipefail
}

# Print the value of <field> for (harness, agent), or nothing.
#
# The value class is "everything up to the flow-map delimiters" — NOT a character allowlist.
# ADR-0015 makes model IDs opaque passthrough with no vendor allowlist, and provider-prefixed IDs
# (`anthropic/claude-opus-5`, `openai:gpt-5.6-sol`) are ordinary. A narrower class does not reject
# them, which would at least be honest; it TRUNCATES them to a prefix that then satisfies the
# completeness check in hd_validate and generates a wrong pin (0168 whole-branch review). Anything
# the class cannot express — a quoted scalar, an embedded space — is caught by hd_validate's
# bare-scalar check rather than silently clipped.
hd_field(){ # $1=file $2=harness $3=agent $4=model|effort
  local line val
  line="$(_hd_entry_line "$1" "$2" "$3")"
  [ -n "$line" ] || return 0
  val="$(sed -nE "s/.*[{,[:space:]]$4[[:space:]]*:[[:space:]]*([^,}[:space:]]+).*/\1/p" <<<"$line")"
  [ -n "$val" ] || return 0
  printf '%s' "${val%%$'\n'*}"
}

# Print the RAW field text for (harness, agent): everything between the colon and the next flow-map
# delimiter (`,` or `}`), trailing whitespace trimmed. This is what a YAML parser would see;
# hd_field is what DOCKET's reader consumes. hd_validate rejects any entry where the two differ, so
# a value the reader cannot consume whole fails loudly instead of shipping as a truncated prefix.
# The `_raw` tier follows the naming of docket-frontmatter.sh's field/field_raw pair (ADR-0058),
# though the split here is reader-capability, not quote-style.
hd_field_raw(){ # $1=file $2=harness $3=agent $4=model|effort
  local line val
  line="$(_hd_entry_line "$1" "$2" "$3")"
  [ -n "$line" ] || return 0
  val="$(sed -nE "s/.*[{,[:space:]]$4[[:space:]]*:[[:space:]]*([^,}]*).*/\1/p" <<<"$line")"
  val="${val%%$'\n'*}"
  printf '%s' "$(sed -E 's/[[:space:]]+$//' <<<"$val")"
}

# Validate the sidecar against <sources-dir> (agents/). Exit 1 with diagnostics on stderr.
hd_validate(){ # $1=file $2=sources-dir
  local f="$1" src="$2" rc=0 h a line k v raw n fields
  # This library is SOURCED, so it inherits whatever IFS the caller left behind. Several loops
  # below word-split an unquoted expansion; under a clobbered IFS ("") the field-name loop stops
  # splitting, which both fails the pristine sidecar and silently disarms the `runner` guard.
  # `local IFS` restores the caller's value on return.
  local IFS=$' \t\n'
  if [ ! -f "$f" ] || [ ! -r "$f" ]; then
    echo "harness-defaults: missing or unreadable: $f" >&2; return 1
  fi
  if ! grep -qE '^agents:[[:space:]]*$' "$f"; then
    echo "harness-defaults: no top-level 'agents:' block" >&2; rc=1
  fi
  for h in $(hd_harnesses "$f"); do
    if [ "$h" = "default" ]; then
      echo "harness-defaults: a harness-neutral 'default:' block is forbidden — every entry must name a concrete harness" >&2; rc=1; continue
    fi
    case " $HD_KNOWN_HARNESSES " in *" $h "*) : ;; *)
      echo "harness-defaults: unknown harness '$h' (known: $HD_KNOWN_HARNESSES)" >&2; rc=1; continue ;;
    esac
    # duplicate harness block. Counted against the RAW key listing: hd_harnesses ends in `sort -u`,
    # so counting against it can never exceed 1 and the guard was dead (0168 whole-branch review).
    if [ "$(_hd_harness_keys "$f" | grep -cxF "$h")" -gt 1 ]; then
      echo "harness-defaults: duplicate harness block '$h'" >&2; rc=1
    fi
    while IFS= read -r line; do
      [ -n "$line" ] || continue
      a="$(printf '%s' "$line" | sed -e 's/^    //' -e 's/[[:space:]]*:.*//')"
      [ -f "$src/docket-$a.md" ] || {
        echo "harness-defaults: $h/$a names no wrapper source ($src/docket-$a.md)" >&2; rc=1; }
      # exactly the allowed fields
      fields="$(printf '%s' "$line" | sed -nE 's/.*\{(.*)\}.*/\1/p' | tr ',' '\n' | sed -nE 's/^[[:space:]]*([A-Za-z0-9._-]+)[[:space:]]*:.*/\1/p')"
      for k in $fields; do
        case "$k" in
          model|effort) : ;;
          runner) echo "harness-defaults: $h/$a sets 'runner' — delegation is user policy, never a shipped default" >&2; rc=1 ;;
          *) echo "harness-defaults: $h/$a has unknown field '$k' (allowed: model, effort)" >&2; rc=1 ;;
        esac
      done
      for k in model effort; do
        v="$(hd_field "$f" "$h" "$a" "$k")"
        raw="$(hd_field_raw "$f" "$h" "$a" "$k")"
        if [ -z "$v" ]; then
          echo "harness-defaults: $h/$a is missing a non-empty '$k'" >&2; rc=1
        elif [ "$v" != "$raw" ] || case "$raw" in '"'*|"'"*) true;; *) false;; esac; then
          # The reader consumes bare scalars only. Without the `!=` leg a quoted or space-bearing
          # value is silently clipped to a prefix that still passes the non-empty check above — and
          # when the clip is empty, the diagnostic blames ABSENCE for what is really a quoting
          # problem.
          #
          # Two legs (ADR-0065). The `!=` leg catches anything the value class cannot express (an
          # embedded space). The quote leg catches what `!=` structurally CANNOT see: a quoted but
          # space-free value has v == raw, so the quotes would ride into the emitted pin verbatim
          # while this diagnostic's own remedy text tells the user to write them unquoted. Single
          # quotes included — the remedy says "unquoted", not "double-unquoted". Copied by value
          # from the reference implementation in sync-agents.sh's `validate_user_agent_values`;
          # this library's header forbids coupling the shipped-data reader to the user-config
          # readers, and extracting a shared helper is change #0256's scope, not this one's.
          echo "harness-defaults: $h/$a '$k' value '$raw' is not a bare scalar — the reader consumes only '$v'; write model/effort values unquoted and space-free" >&2; rc=1
        fi
      done
      # duplicate agent entry within the block
      if [ "$(hd_agents "$f" "$h" | grep -cx "$a")" -gt 1 ]; then
        echo "harness-defaults: duplicate entry '$a' under '$h'" >&2; rc=1
      fi
    done < <(_hd_block "$f" "$h")
  done
  # Completeness: every harness docket ships defaults for must cover the WHOLE wrapper set, so a
  # new agents/docket-*.md cannot land pinned on one harness and unpinned on another. The reverse
  # direction — an entry naming no wrapper source — is already enforced per-entry above (line
  # 131), for every harness including ones docket ships nothing for.
  for h in $HD_SHIPPED_HARNESSES; do
    for n in "$src"/docket-*.md; do
      [ -e "$n" ] || continue
      a="$(basename "$n" .md)"; a="${a#docket-}"
      [ -n "$(hd_field "$f" "$h" "$a" model)" ] || {
        echo "harness-defaults: $h block is incomplete — no entry for '$a'" >&2; rc=1; }
    done
  done
  return $rc
}
