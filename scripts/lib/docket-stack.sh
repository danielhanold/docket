#!/usr/bin/env bash
# scripts/lib/docket-stack.sh — the stacked-changes library (change 0298). SOURCE this; it is
# never executed directly. It declares functions only: no side effects on source, no writes of any
# kind, no network. `stack_find_file`, `stack_parent_id` and `stack_chain` are pure reads over a
# changes directory; `stack_effective_base` additionally makes ONE read-only git call
# (`show-ref --verify`) through the repo's standard `GIT="${GIT:-git}"` mock seam, because rule 1
# turns on whether a parent's branch actually exists on the remote. That call is addressed
# `-C <changes dir>` here, so every caller gets the changes-dir's repo answered regardless of its
# own cwd and none of them may add a `-C` of its own — see the note at the ref lookup.
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

# stack_descendants CHANGES_DIR ROOT_ID -> every TRANSITIVE descendant id, one canonical id per
# line, PARENTS BEFORE CHILDREN. Breadth-first, because that is the order the stack close-out's
# promotion loop needs: a child is promoted after the parent it merged into. Exit 1 on a
# non-numeric ROOT_ID; an unknown root and a childless root both print nothing and exit 0.
#
# THE PREFILTER IS NOT AN OPTIMIZATION FOOTNOTE — it is what makes this callable per pass. One
# `fm_field` subshell per change file costs ~1s over a 300-change tree; one `grep -l` over the same
# list, then `fm_field` only on the files it names, costs ~0.07s. It is safe because it keys on the
# KEY'S SHAPE (`^stacked_on:`), which is a strict SUPERSET of what `fm_field` can answer non-empty
# for: it also matches an empty value, a padded or inline-commented one, and a body-prose line the
# anchored read then discards. So it narrows the WORK and never the DECISION — every file it drops
# is one `fm_field` would have answered empty for. Narrowing it to a value shape (`^stacked_on: [0-9]`)
# would break exactly that property, and tests/test_stack_closeout.sh's padded, inline-commented
# fixture is the pin that reddens if anyone does.
#
# The `seen` set is seeded with the ROOT and grows with every emitted id, so a cyclic `stacked_on`
# graph terminates and each descendant is emitted once. A cycle is a data defect the health checks
# name; a graph walk that hangs the sweep on one is not an acceptable way to report it.
stack_descendants(){
  local dir="$1" frontier next seen f id raw_id parent hits
  case "$2" in (''|*[!0-9]*) return 1 ;; esac
  frontier=" $(( 10#$2 )) "
  seen="$frontier"
  local -a cands=()
  for f in "$dir"/active/*.md "$dir"/archive/*.md; do
    [ -f "$f" ] || continue
    cands+=("$f")
  done
  [ "${#cands[@]}" -gt 0 ] || return 0
  hits="$(grep -l -E -e '^stacked_on:' -- "${cands[@]}" 2>/dev/null)"
  [ -n "$hits" ] || return 0
  while [ -n "$frontier" ]; do
    next=""
    while IFS= read -r f; do
      [ -f "$f" ] || continue
      parent="$(fm_field "$f" stacked_on)"
      [ -n "$parent" ] || continue
      case "$parent" in (*[!0-9]*) continue ;; esac
      parent=$(( 10#$parent ))
      case "$frontier" in (*" $parent "*) ;; (*) continue ;; esac
      raw_id="$(field "$f" id)"
      case "$raw_id" in (''|*[!0-9]*) continue ;; esac
      id=$(( 10#$raw_id ))
      case "$seen" in (*" $id "*) continue ;; esac
      seen="$seen$id "
      printf '%s\n' "$id"
      next="$next$id "
    done <<<"$hits"
    if [ -n "$next" ]; then frontier=" $next"; else frontier=""; fi
  done
}

# stack_effective_base CHANGES_DIR ID INTEGRATION_BRANCH [REMOTE] -> the branch this change is
# built on, on stdout. Exit 0 resolved, 3 the chain reaches a KILLED parent, 4 the chain is invalid
# (missing parent, cycle, or a parent whose branch has no remote ref). Nothing is printed on 3 or 4:
# a caller that reads stdout and forgets the status must not be handed a plausible-looking base.
#
# This is the ONE place spec §3's four rules live. Walking upward rather than answering from the
# immediate parent alone is what makes rule 2 work at depth: a parent that already merged carries no
# branch worth basing on, so the answer is whatever ITS base resolves to, recursively, until the walk
# reaches an unstacked ancestor and lands on the integration branch.
#
# WHY THE REMOTE REF IS A CONJUNCT OF RULE 1, not a nicety: `branch:` is stamped into the manifest at
# CLAIM time, but the branch is not pushed until the PR step. So an `in-progress` parent routinely
# carries a valid-looking `branch:` with nothing behind it, and cutting a child from that name would
# silently produce a branch based on the integration branch while everyone believes it is stacked.
# Exit 4 — a data/sequencing problem a human resolves — is the only honest answer there.
#
# `field` (unanchored) is correct for `status`: the change template guarantees the key, so it is in
# the guaranteed-present tier of the selection rule. `branch:` and `stacked_on:` are absent-capable
# and take the anchored `fm_field`. See tests/test_frontmatter_read_shapes.sh, which censuses this.
stack_effective_base(){
  local dir="$1" id="$2" integration="$3" remote="${4:-origin}" parent f status branch
  local git="${GIT:-git}"
  stack_chain "$dir" "$id" >/dev/null 2>&1 || return 4
  parent="$(stack_parent_id "$dir" "$id")"
  [ -n "$parent" ] || { printf '%s\n' "$integration"; return 0; }
  f="$(stack_find_file "$dir" "$parent")" || return 4
  status="$(field "$f" status)"
  case "$status" in
    killed) return 3 ;;
    done)   stack_effective_base "$dir" "$parent" "$integration" "$remote"; return $? ;;
  esac
  branch="$(fm_field "$f" branch)"
  # ADDRESSED AT THE CHANGES DIR, never the caller's cwd: the resolver is called from renderers,
  # health checks and a CLI that all run from wherever their dispatcher left them, and a lookup
  # against the wrong repo simply finds no ref — indistinguishable here from "the parent's branch
  # was never pushed", so it silently becomes exit 4 and drops the change out of the ready queue.
  # A caller must therefore NOT add a `-C` of its own: `git -C a -C b` composes RELATIVELY, so a
  # second flag over a relative CHANGES_DIR resolves against the first and lands in neither repo.
  if [ -n "$branch" ] && "$git" -C "$dir" show-ref --verify --quiet "refs/remotes/$remote/$branch"; then
    printf '%s\n' "$branch"
    return 0
  fi
  # A stacked-merged parent has already merged into ITS parent, so its branch may legitimately be
  # gone; the base is then whatever the parent's own base resolves to (spec §3 rule 2). Any other
  # status with an unpushed branch is rule 4, not a fallback — see the conjunct note above.
  if [ "$status" = stacked-merged ]; then
    stack_effective_base "$dir" "$parent" "$integration" "$remote"
    return $?
  fi
  return 4
}
