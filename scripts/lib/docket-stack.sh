#!/usr/bin/env bash
# scripts/lib/docket-stack.sh — the stacked-changes library (change 0298). SOURCE this; it is
# never executed directly. It declares functions only: no side effects on source, no git, no
# network, no writes of any kind. Every routine here is a pure read over a changes directory.
#
# REQUIRES scripts/lib/docket-frontmatter.sh to have been sourced FIRST — this file consumes its
# `fm_field` accessor and does not source it itself, because the two libraries have distinct
# lifetimes: every consumer of docket-stack.sh already sources the frontmatter library for its own
# reasons, and a second source would re-declare the vocabulary globals under whatever the consumer
# had already set.
#
# WHY `fm_field` AND NEVER `field`: `stacked_on:` is an OPTIONAL key — the change template ships it
# empty and a hand-authored or older file may omit the line entirely. An unanchored read of an
# absent key does not return empty; it runs past the closing `---` and returns whatever body line
# opens with that word, and in THIS repo a change file's body discussing `stacked_on:` is normal
# content, not a contrived fixture. See the selection rule in scripts/lib/docket-frontmatter.sh.
#
# WHY `10#` AT EVERY ID BOUNDARY: docket displays zero-padded 4-digit ids, so ids arrive padded.
# Bash reads a leading `0` as an octal prefix, which makes `0237` silently 159 and `0008` an
# outright parse error. Every id crossing into arithmetic here is canonicalized with `10#` first,
# matching the precedent in scripts/board-checks.sh and scripts/adr-checks.sh.

# stack_find_file CHANGES_DIR ID -> absolute path on stdout, or nothing + exit 1.
# Searches active/ first, then archive/ — a change is in exactly one of them, and the active copy
# is the live one whenever both somehow exist.
stack_find_file(){
  local dir="$1" id padded f
  case "$2" in (''|*[!0-9]*) return 1 ;; esac
  id=$(( 10#$2 )) || return 1
  padded="$(printf '%04d' "$id")"
  for f in "$dir"/active/"$padded"-*.md "$dir"/archive/*-"$padded"-*.md; do
    [ -f "$f" ] || continue
    printf '%s\n' "$f"
    return 0
  done
  return 1
}

# stack_parent_id CHANGES_DIR ID -> the parent change id, canonical and unpadded, or nothing.
# ALWAYS exits 0: "this change has no parent" and "this change does not exist" are both answered by
# an empty stdout, so a caller can read the value without a status dance. A malformed (non-numeric)
# `stacked_on:` is likewise no parent here — naming it as a defect belongs to the health checks,
# not to a resolver every renderer calls on every file.
stack_parent_id(){
  local f raw
  f="$(stack_find_file "$1" "$2")" || return 0
  raw="$(fm_field "$f" stacked_on)"
  [ -n "$raw" ] || return 0
  case "$raw" in (*[!0-9]*) return 0 ;; esac
  printf '%d\n' "$(( 10#$raw ))"
}

# stack_chain CHANGES_DIR ID -> the ancestor chain, nearest parent first, one canonical id per
# line. Exit 0 when the chain is well-formed (including the empty chain of an unstacked change),
# exit 1 with a diagnostic on stderr when it names a missing parent or closes a cycle.
#
# The visited set is a space-delimited string rather than an associative array so the routine is
# usable from any sourced context without declaring a global; it is seeded with the STARTING id, so
# a change stacked on itself is caught by the same arm as a longer cycle.
stack_chain(){
  local dir="$1" cur seen parent
  case "$2" in (''|*[!0-9]*) printf 'stack: not a change id: %s\n' "$2" >&2; return 1 ;; esac
  cur=$(( 10#$2 ))
  seen=" $cur "
  while :; do
    parent="$(stack_parent_id "$dir" "$cur")"
    [ -n "$parent" ] || return 0
    if ! stack_find_file "$dir" "$parent" >/dev/null; then
      printf 'stack: change %s names a missing stacked_on parent %s\n' "$cur" "$parent" >&2
      return 1
    fi
    case "$seen" in (*" $parent "*)
      printf 'stack: cycle in the stacked_on chain at change %s\n' "$parent" >&2
      return 1 ;;
    esac
    printf '%s\n' "$parent"
    seen="$seen$parent "
    cur="$parent"
  done
}
