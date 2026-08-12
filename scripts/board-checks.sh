#!/usr/bin/env bash
# scripts/board-checks.sh — the mechanical docket-status health checks (change 0023). Sources the
# shared frontmatter/dependency-resolution helper (change 0022) and walks the change files, emitting
# one finding per line on stdout. Git-only (no gh, no network) and warn-only (never auto-fixes); the
# caller (docket-status) surfaces the lines. The one judgment-bearing check (blocked_by: re-examination)
# stays model-driven in the skill — it is NOT here.
#
# Usage: board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
#                         [--lease-ttl-hours N] [--adrs-dir DIR] [--terminal-publish]
#                         [--results-dir REPO-RELATIVE-DIR]
#   Findings: TAB-separated  <check-id>\t<change-id>\t<message>  on stdout, sorted by (check-id, change-id).
#     check-id ∈ {aborted-run, adr-unpublished, board-row-dropped, broken-spec, broken-plan-results,
#                 dep-cycle, field-domain, malformed-id, publish-deferred, scalar-form,
#                 stack-invalid, stack-parent-killed,
#                 stale-in-progress, merge-gate-stall, stale-finalize-blocked, merged-orphan,
#                 unknown-commit-ref}
#     The set above is declared in lib/docket-frontmatter.sh as BOARD_CHECK_IDS and pinned to it,
#     to board-checks.md, and to docket-status.md by tests/test_board_checks.sh — edit all four.
#   Clean tree ⇒ no output, exit 0. --strict ⇒ exit 1 if any finding (for a future CI gate).
#   Branch args are passed to `git cat-file -e <ref>:<path>` verbatim; in main-mode the two refs
#   coincide and both link checks resolve on the same branch with no special-casing.
#   --lease-ttl-hours N defaults to 72 when absent (standalone use stays sane); a non-numeric or
#   negative N is rejected up front (exit 2), never crashed into the staleness arithmetic. It sets the
#   claim-lease TTL for stale-in-progress (change 0089): claimed_at + TTL expiry, on top of the
#   pre-existing branch-idle >3d signal. See that check's block below for the two trigger messages.
#   Mock seams: GIT="${GIT:-git}"  (the only external dependency); NOW="${NOW:-$(date +%s)}" (staleness clock).
set -uo pipefail

GIT="${GIT:-git}"
NOW="${NOW:-$(date +%s)}"
CHANGES_DIR=""; METADATA_BRANCH=""; INTEGRATION_BRANCH=""; STRICT=0
ADRS_DIR=""; ADR_GATE=0
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --metadata-branch) METADATA_BRANCH="$2"; shift ;;
    --integration-branch) INTEGRATION_BRANCH="$2"; shift ;;
    --strict) STRICT=1 ;;
    --lease-ttl-hours) LEASE_TTL_HOURS="$2"; shift ;;
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --results-dir) RESULTS_DIR_REL="$2"; shift ;;
    --terminal-publish) ADR_GATE=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'board-checks: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
LEASE_TTL_HOURS="${LEASE_TTL_HOURS:-72}"   # default when --lease-ttl-hours is absent (standalone use)
# Repo-RELATIVE artifact directories for the aborted-run leg-A probe (change 0113). Unlike
# --changes-dir/--adrs-dir (filesystem paths), these are addressed as `<ref>:<path>` and
# `ls-tree --full-tree`, which are always worktree-root-relative. --results-dir defaults to the
# convention's own default so a standalone hand-run stays sane; docket-status.sh passes the
# resolved RESULTS_DIR. The plans dir has no config knob in the convention (the plan path is fixed
# by the plan role's own default), so it is a constant here rather than a flag nobody would set.
RESULTS_DIR_REL="${RESULTS_DIR_REL:-docs/results}"
PLANS_DIR_REL="docs/superpowers/plans"
# Validate the resolved TTL UNCONDITIONALLY (mirrors reclaim-claims.sh's own guard). A non-numeric or
# negative value must fail here, cleanly — not crash the staleness arithmetic (`$(( LEASE_TTL_HOURS *
# 3600 ))`) unbound, which would otherwise only surface on repos that carry an in-progress change.
case "$LEASE_TTL_HOURS" in
  ''|*[!0-9]*) printf 'board-checks: invalid --lease-ttl-hours: %s (must be a non-negative integer, hours)\n' "$LEASE_TTL_HOURS" >&2; exit 2 ;;
esac
[ -n "$CHANGES_DIR" ]        || { printf 'board-checks: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ]        || { printf 'board-checks: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }
[ -n "$METADATA_BRANCH" ]    || { printf 'board-checks: missing --metadata-branch\n' >&2; exit 2; }
[ -n "$INTEGRATION_BRANCH" ] || { printf 'board-checks: missing --integration-branch\n' >&2; exit 2; }
# --adrs-dir is OPTIONAL (the check is opt-in), but a path that was SUPPLIED and does not exist is
# a caller error, never a silent skip: a typo'd dir would make the check vacuously green forever.
if [ -n "$ADRS_DIR" ] && [ ! -d "$ADRS_DIR" ]; then
  printf 'board-checks: adrs dir not found: %s\n' "$ADRS_DIR" >&2; exit 2
fi

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-stack.sh"   # stack_effective_base, for the two stack checks

resolve_deps "$CHANGES_DIR"            # populates STATUS_OF / DEP_STATE / DEP_REASON / DEP_ON

# git_has REF PATH — exit 0 iff REF:PATH resolves in the changes-dir's repo (no network).
git_has(){ "$GIT" -C "$CHANGES_DIR" cat-file -e "$1:$2" 2>/dev/null; }

# branch_ref BRANCH — print the first ref name that resolves for BRANCH (local first, then the
# origin remote-tracking ref) and exit 0; exit 1 with empty stdout when neither resolves or BRANCH
# is empty. Single source for "does this change's feature branch exist at all", shared by
# stale-in-progress's has_branch test and aborted-run's leg A.
branch_ref(){
  local br="$1"
  [ -n "$br" ] || return 1
  if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/heads/$br"; then
    printf '%s' "refs/heads/$br"; return 0
  fi
  if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$br"; then
    printf '%s' "refs/remotes/origin/$br"; return 0
  fi
  return 1
}

# branch_only_artifact REF DIR — print the first path under DIR that exists on REF but NOT on
# INTEGRATION_BRANCH, and exit 0; exit 1 with empty stdout when DIR is empty on REF or every path
# under it is already on the integration branch (inherited, i.e. already-merged work).
# --full-tree makes DIR worktree-root-relative regardless of the `-C "$CHANGES_DIR"` cwd, which is
# a subdirectory.
#
# -z is REQUIRED, not a style choice (change 0202). Plain `--name-only` C-quotes any path holding a
# quote, a backslash, a control character, or — under the default core.quotePath=true — any
# non-ASCII byte. git_has would then look up the literal quoted string, fail, and this function
# would report an INHERITED file as branch-only: a false positive in a check whose whole value is
# that it is believable. -z suppresses quoting and delimits with NUL.
#
# The NUL listing CANNOT be captured into a variable first: `$(…)` strips NUL bytes, so the
# delimiters would vanish and every path would concatenate into one string. Hence the
# process-substituted redirect. Do not "simplify" this back to a capture with a here-string.
#
# It is also not a pipeline, which matters for the early `return 0`. The hazard the previous
# comment called a race was really the SUBSHELL of `producer | while … done`: there, the in-loop
# `return 0` exits only the subshell, the function falls through to `return 1`, and the caller's
# `if` fails even though the path was printed. A redirect runs the loop in THIS shell, so the
# return is real. On that early return the process-substituted producer is orphaned and reaped with
# its remaining output discarded — harmless for a pure reader like ls-tree, and the reason never to
# swap in a producer with side effects.
#
# No emptiness guard is needed at EITHER level, and change 0200 removed the one that was here.
# Whole listing: an empty listing yields zero loop iterations and falls through to `return 1`.
# Per record: under -z, ls-tree never emits an empty record, and at EOF `read -d ''` returns
# nonzero with an empty accumulator, ending the loop before the body runs — so a
# `[ -n "$boa_p" ] || continue` inside the loop was unreachable. It is not re-added: by this repo's
# own rule an unenforced comment is decoration, and dead code invites "what does this protect?"
# archaeology.
branch_only_artifact(){
  local boa_ref="$1" boa_dir="$2" boa_p
  while IFS= read -r -d '' boa_p; do
    git_has "$INTEGRATION_BRANCH" "$boa_p" || { printf '%s' "$boa_p"; return 0; }
  done < <("$GIT" -C "$CHANGES_DIR" ls-tree -r -z --name-only --full-tree "$boa_ref" -- "$boa_dir" 2>/dev/null)
  return 1
}

declare -A ID_ACTIVE ID_EXISTS                # id -> 1; populated in the FILES walk below
declare -A EXPLAINED DROPPED DROPPED_DIR      # change-id -> 1 / -> dir kind; drive board-row-dropped
FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end

# sanitize VALUE — render TAB, CR, and LF as the visible two-character escapes \t, \r and \n
# (change 0104; the LF leg added by change 0200).
#
# Findings are TAB-separated and the caller splits them with `IFS=$'\t' read -r check_id change_id
# message` (docket-status.sh's `health_checks`). An interior TAB in ANY embedded value shifts every
# later field; an interior LF is worse — it splits one finding into TWO records, and the caller
# reads the orphaned tail as a finding in its own right, with the trailing `sort` free to move it
# anywhere in the output.
#
# Do NOT re-justify the escape set by where the values come from. That premise is what went stale:
# it once read "every embedded value arrives via field()/fm_field(), which truncate at the first
# newline". Since change 0202, leg A's $ar_hit is a GIT PATH read NUL-delimited
# (`ls-tree -r -z`) — and a git path may hold any byte but NUL, newline included — so it reaches
# emit raw. The escape therefore lives HERE, wrapping both embedded columns of every emit, rather
# than at that one call site: every current and future caller is covered without an audit.
#
# The LF coverage is deliberately partial and record-shaped, not a completeness guarantee: every
# LF is escaped wherever it sits; leg A's $(…) capture happens to strip a trailing one earlier, so
# a truncated-path message is a fidelity limit of that call site, not of this escape. That is the
# whole job — keep one finding on one record.
#
# Pure bash parameter expansion: BSD sed does not interpret \t in a pattern, so a sed form would be
# silently wrong.
sanitize(){ local v="$1"; v="${v//$'\t'/\\t}"; v="${v//$'\r'/\\r}"; v="${v//$'\n'/\\n}"; printf '%s' "$v"; }

emit(){ FINDINGS+="$1"$'\t'"$(sanitize "$2")"$'\t'"$(sanitize "$3")"$'\n'; }

# padded_id_from_file FILE — the zero-padded id encoded in the BASENAME (`0104-slug.md`, or
# `2026-07-20-0104-slug.md` in archive/), or `?` when the filename yields none. Used for the
# change-id column whenever the frontmatter id is unusable: that column is what the caller splits
# on, so it must never carry a raw frontmatter value. A validated int_field id is ^[0-9]+$ and
# cannot shift a field, so checks that have one keep emitting it verbatim (unpadded, as before).
padded_id_from_file(){
  local b; b="$(basename "$1")"
  b="${b#[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-}"   # strip an archive/ date prefix, if any
  case "$b" in
    [0-9][0-9][0-9][0-9]-*) printf '%s' "${b%%-*}" ;;
    *) printf '?' ;;
  esac
}

# Staleness horizon for the stale-finalize-blocked check (change 0098): an 'implemented' change's
# `## Finalize blocked` marker older than this fires the advisory. Hardcoded, no config knob —
# mirrors stale-in-progress's own hardcoded 3*86400 branch-idle horizon; 72h matches the lease-TTL
# default's sense of "a few days is normal, longer is suspicious". Promote to a flag only if
# independent tuning is ever wanted.
FINALIZE_BLOCKED_STALE_SECS=$(( 72 * 3600 ))

# Run-scale staleness horizon for aborted-run's leg B (change 0113). Hardcoded, no config knob —
# same precedent as FINALIZE_BLOCKED_STALE_SECS above and stale-in-progress's 3*86400 branch-idle
# threshold. 12h is six times tighter than the 72h lease default: tight enough that a /loop drain
# trips over an abort on its next iteration, loose enough to leave room for a marathon build. When
# a genuinely long build does trip it the finding is free, self-clearing, and worth a glance.
ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))

# Branch-idle floor for aborted-run's leg C (change 0211). Hardcoded, no config knob — the same
# precedent as ABORTED_RUN_STALE_SECS above, FINALIZE_BLOCKED_STALE_SECS, and stale-in-progress's
# 3*86400. Keyed on the branch's newest COMMIT, never on claimed_at: the heartbeat rider re-stamps
# claimed_at at every phase boundary, which is exactly why leg B is blind to this signature.
#
# 2h, derived: after the last build commit a healthy run still has review, any ADR, the ~10-minute
# suite, and the push to get through — and a review-driven fix COMMITS, resetting this clock, so the
# real exposure is that tail, not the whole build span. 2h covers it with room and is 6x tighter
# than leg B's 12h (the same ratio leg B took against the 72h lease).
#
# The residual, stated rather than hidden: the floor keys on the branch TIP, so ANY gap longer than
# this between commits on a live run fires leg C — a marathon tail with no post-review commit, but
# equally a single long build task sitting between two per-task commits. That finding is free,
# advisory, and self-clearing the moment the next commit lands or the PR is recorded — and a floor
# loose enough never to misfire would be loose enough to stop detecting.
ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))

# renders_row DIR_KIND ID STATUS — exit 0 iff render-board.sh would account for a file in DIR_KIND
# ('active' or 'archive') carrying this (int_field-validated) ID and this raw STATUS. This is the
# COMPUTED half of the board-row-dropped invariant (change 0104; widened to archive/ by 0115): it
# mirrors the renderer's own bucketing rather than re-enumerating the conditions the other checks
# already name, so a drop path ADDED TO THE RENDERER is noticed here without anyone editing this
# script. Three clauses, each anchored to real renderer behavior:
#   1. The id gate, hoisted because it is ONE condition holding in BOTH directories: the renderer
#      requires a usable integer id to emit an identifying row on either side. A file without one
#      is still counted in `total`, so it is unaccounted for.
#   2. active/  -> the renderer calls print_section once per DOCKET_STATUSES_ACTIVE member and
#      buckets on the RAW `status:` read, so a status outside that set lands in a bucket nothing
#      iterates. The live case is a TERMINAL status sitting in active/ — legal status, wrong
#      directory (the `sweep-failed <id> archive <reason>` state: status flipped, archive move
#      failed).
#   3. archive/ -> the archive block's open gate and its <summary> count both come from the
#      per-status archive tally read over DOCKET_STATUSES_TERMINAL, so a NON-terminal status is
#      counted in `total` and joins no summary. The live case is the mirror image: an interrupted
#      archive-change.sh, whose `git mv` precedes its status flip.
# Membership is read from the SHARED arrays via docket_status_is_active / docket_status_is_terminal,
# never a list restated here. That matters twice over: the active set and the full
# vocabulary are DIFFERENT sets and the difference IS the drop path, and since change 0116
# single-sourced the renderer's own vocabularies the renderer reads these very arrays too — so both
# arms are backed by the same source the consumer reads, not by a comment-asserted correspondence.
renders_row(){
  local rr_dir="$1" rr_id="$2" rr_st="$3"
  [ -n "$rr_id" ] || return 1
  case "$rr_dir" in
    active)  docket_status_is_active   "$rr_st" ;;
    archive) docket_status_is_terminal "$rr_st" ;;
    *) return 1 ;;
  esac
}

# --- scalar-form: an unquoted frontmatter scalar that is not well-formed YAML (change 0191).
# The well-formedness leg of the house yaml-scalar rule (AGENTS.md + ADR-0065): a BARE scalar
# carrying a ': ' (colon-space) or exactly matching a YAML 1.1 boolean keyword is read
# ambiguously by any YAML consumer, so it must be quoted or reworded. Covers the only two
# free-text string scalars docket reads that are not already shape/domain-gated — title and the
# optional blocked_by. The natively-boolean fields (trivial, auto_groomable, reconciled) hold a
# bare true/false BY DESIGN and are not scanned. Reads the RAW token (field_raw for title,
# fm_field_verbatim for blocked_by): field()/fm_field() unwrap surrounding quotes, which would make
# a quoted colon-space look exactly like a bad bare one. blocked_by goes through the ANCHORED
# accessor so an ABSENT blocked_by does not fall through to a body-prose line, and through the
# VERBATIM one so the value arrives as authored: fm_field_raw strips a whitespace-preceded `#...`
# before returning, which IS the truncation the comment-introducer leg exists to report, so reading
# through it would make that leg unreachable for this field (the real `blocked_by: PR #69 is stale
# ...` on the metadata branch would arrive as `PR` and pass silently). title needs no such twin —
# field_raw has no comment strip. At most ONE finding per field — the predicate
# reports the FIRST matching leg and stops, since one reason is enough to demand a quote; warn-only;
# never marks EXPLAINED (a malformed scalar does not drop a board row). One skip leg here, then
# docket_scalar_quote_reason's own empty-value early return, then its five syntax legs — in
# evaluation order:
#   skip               — empty, or the raw value opens with " or ' (quoted is well-formed by
#                        definition; never inspect the interior). This leg lives here, not in the
#                        predicate.
#   empty              — the predicate's own first act is an early return on an empty value (an
#                        empty scalar is well-formed bare); the skip leg above already caught it.
#   colon-space        — the unquoted raw value contains a colon followed by a space.
#   trailing-colon     — the unquoted raw value ends in a colon (the shape that let change 0173
#                        through unreported, change 0235).
#   bare-boolean       — the unquoted raw value is exactly one of on off yes no true false,
#                        whole-value and case-insensitive (YAML 1.1).
#   comment-introducer — the unquoted raw value contains whitespace (a space OR a tab, the
#                        [[:space:]] class) followed by a hash, which opens a YAML comment and
#                        silently TRUNCATES the value rather than aborting the parse.
#   indicator          — the unquoted raw value opens with a YAML indicator character. A leading
#                        hash is one of them, and is the maximal form of the leg above: the
#                        comment opens at character one, so the WHOLE value parses to null.
scalar_form_check(){ # scalar_form_check FIELD RAW CID
  local sfc_field="$1" sfc_raw="$2" sfc_cid="$3" sfc_reason
  case "$sfc_raw" in
    ''|\"*|\'*) return 0 ;;   # skip leg: empty, or opens with a quote -> well-formed, never inspected
  esac
  # The syntax legs live in ONE place — lib/docket-frontmatter.sh's docket_scalar_quote_reason —
  # so the checker and any future consumer cannot drift into two copies of the same rule
  # (change 0235). The skip arm above stays here: it is the only leg that needs the RAW token,
  # and pushing it into the predicate would wrongly skip a value that logically STARTS with a
  # quote character. The messages stay here too, because a finding is this script's output shape.
  #
  # The capture below FORKS — twice per change file, once for title and once for blocked_by.
  # Deliberate, and recorded here rather than left silent (change 0235 review). The house
  # "no subshell, no fork" property lib/docket-frontmatter.sh states for _docket_unwrap_quotes is
  # a property of a helper's BODY — use bash parameter expansion, not sed/tr — and
  # docket_scalar_quote_reason honours it exactly: its body is nothing but `case` arms. Neither
  # helper avoids the caller's capture subshell; field() spends the identical one on
  # $(_docket_unwrap_quotes …). Making the predicate assign a global instead would buy back two of
  # the ~38 command substitutions this per-file loop already spends — most of them forking sed or
  # awk — in exchange for an order-dependent out-parameter that every assert and the
  # docket_scalar_needs_quoting wrapper would have to read. Stdout stays the contract.
  sfc_reason="$(docket_scalar_quote_reason "$sfc_raw")"
  case "$sfc_reason" in
    colon-space)
      emit scalar-form "$sfc_cid" "$sfc_field: unquoted scalar contains ': ' — quote it or reword (well-formed YAML)"
      ;;
    trailing-colon)
      emit scalar-form "$sfc_cid" "$sfc_field: unquoted scalar ends with ':' — quote it or reword (well-formed YAML)"
      ;;
    bare-boolean)
      emit scalar-form "$sfc_cid" "$sfc_field: unquoted bare YAML boolean ($sfc_raw) — quote it or reword (well-formed YAML)"
      ;;
    comment-introducer)
      emit scalar-form "$sfc_cid" "$sfc_field: unquoted scalar contains whitespace followed by '#', a YAML comment introducer that silently truncates it — quote it or reword (well-formed YAML)"
      ;;
    indicator)
      emit scalar-form "$sfc_cid" "$sfc_field: unquoted scalar opens with a YAML indicator character — quote it or reword (well-formed YAML)"
      ;;
  esac
}
# --- end scalar-form helper ---
# The definition lives at TOP LEVEL (hoisted by change 0200) instead of inside the per-file walk,
# where it was redefined once per change file. The change-id every finding is attributed to arrives
# as the THIRD parameter (FIELD RAW CID, per the usage note on the definition line above), so the
# function depends on nothing in its caller's scope: a call from anywhere — including the later
# walk in this file that also assigns `cid`, or a future call at top level where no `cid` exists —
# reports the id the caller named rather than whichever `cid` happens to be live.
# The end marker above is NOT decoration. It is the named terminator mutation 4's first region
# delete bounds on — without it the range would run past this point into the walk and produce a
# syntactically dead copy that still passes every assert.

# Walk every change file (active + archive); per-check filters apply inside.
mapfile -t FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  pid="$(padded_id_from_file "$f")"
  # cid — the change-id column for every finding about this file: the validated integer id when
  # there is one, else the filename-derived padded id. NEVER the raw frontmatter value.
  cid="${id:-$pid}"
  # Anchored on "$CHANGES_DIR" rather than a bare */active/* glob: an unanchored pattern
  # misclassifies every file when CHANGES_DIR itself contains an `active` path component.
  dir_kind=archive; case "$f" in "$CHANGES_DIR"/active/*) dir_kind=active ;; esac
  status="$(field "$f" status)"

  # --- board-row-dropped, computed (change 0104; widened to archive/ by 0115). THE ONLY site that
  # populates DROPPED: the invariant is evaluated once, from renders_row's mirror of the renderer,
  # for every file in EITHER directory — never re-derived per drop CAUSE at the checks that happen
  # to name one.
  if ! renders_row "$dir_kind" "$id" "$status"; then DROPPED["$cid"]=1; DROPPED_DIR["$cid"]="$dir_kind"; fi

  if [ -z "$id" ]; then
    if [ -n "$raw" ]; then
      emit malformed-id "$cid" "non-integer id '$raw' in $(basename "$f")"
      # EXPLAINED: a non-integer id is a genuine drop CAUSE (render-board.sh's `[ -n "$id" ] ||
      # continue` skips the row), so
      # this finding accounts for the DROPPED entry above and the backstop stays quiet.
      EXPLAINED["$cid"]=1
    fi
    continue
  fi
  ID_EXISTS["$id"]=1
  case "$f" in
    */active/*)  ID_ACTIVE["$id"]=1 ;;
  esac
  # anchored: spec:/trivial: are optional (ADR-0057) — and a body-prose `spec:` makes a
  # needs-brainstorm change read as build-ready, which is the autonomous builder claiming an
  # undesigned change.
  spec="$(fm_field "$f" spec)"; trivial="$(fm_field "$f" trivial)"

  # --- field-domain: a value that is well-formed TEXT but outside its field's DOMAIN (change 0104).
  # These four fields are what the board renderers consume. A value outside the domain does not
  # error — it silently drops the row from every surface (status, slug) or injects columns into it
  # (title), and since change 0094 the digest's `ready` line is the machine-parsed selection channel
  # for docket-implement-next, so a stray inline comment can remove a change from the autonomous
  # build queue while the board still reports a healthy count. One finding per violated field.
  # Every domain is a SHAPE or MEMBERSHIP test — never an enumeration of bad values.
  # `id` is deliberately absent: malformed-id already covers it (no double-reporting).
  #
  # EXPLAINED (the board-row-dropped suppressor) is marked by the `status` arm ONLY. Suppression
  # means "a finding already accounts for this row's DISAPPEARANCE", and only status can make a row
  # disappear: `slug` is not read by the markdown renderer at all, `priority` renders raw into its
  # own cell, and a piped `title` INJECTS columns into a row that is still emitted. Marking those
  # arms would mean an unrelated pipe in a change's title silences the backstop on a row that
  # genuinely vanished for some other reason — the false-suppression failure the design warns about.
  fd_slug="$(field "$f" slug)"; fd_priority="$(field "$f" priority)"; fd_title="$(field "$f" title)"
  fd_type="$(fm_field "$f" type)"   # anchored: type may be ABSENT, so field() would read body prose

  status_ok=0
  for fd_s in "${DOCKET_STATUSES[@]}"; do
    if [ "$status" = "$fd_s" ]; then status_ok=1; break; fi
  done
  if [ "$status_ok" != 1 ]; then
    emit field-domain "$cid" "status '$status' is not one of: ${DOCKET_STATUSES[*]}"
    # A status outside the DOCKET_STATUSES vocabulary is outside the ACTIVE set too, so on a
    # file in either directory renders_row has already recorded the drop; this finding names its cause.
    EXPLAINED["$cid"]=1
  fi

  # slugify's own alphabet (mint-stub.sh's `slugify()`). Empty fails — slug has no documented
  # default.
  case "$fd_slug" in
    ''|*[!a-z0-9-]*) emit field-domain "$cid" "slug '$fd_slug' is not ^[a-z0-9-]+\$" ;;
  esac

  # Empty priority is LEGAL: the convention documents `medium` as the default and render-board.sh's
  # sort already implements it. Flagging it here would make the guard the noise source.
  if [ -n "$fd_priority" ] && ! docket_priority_is_member "$fd_priority"; then
    emit field-domain "$cid" "priority '$fd_priority' is not one of: ${DOCKET_PRIORITIES[*]} (empty = $DOCKET_PRIORITY_DEFAULT)"
  fi

  case "$fd_title" in
    *'|'*) emit field-domain "$cid" "title contains '|', which injects columns into the board row: $fd_title" ;;
  esac

  # `type` is rendered into every active board row (change 0127), so it needs the same
  # column-injection guard `title` has — without it a `|`-bearing value silently widened that row
  # of BOARD.md and nothing flagged it. The domain half checks SHAPE, not membership in the
  # configured taxonomy: render-board deliberately renders a type this machine does not configure
  # ("configuration governs CREATION, never the readability of shared history") and the --type
  # filter validates the same way, so a membership check here would report another machine's
  # legitimate type as a finding — the guard would become the noise source. Empty is LEGAL: it
  # renders as `untyped`, which is the state the migration exists to drain.
  if [ -n "$fd_type" ]; then
    case "$fd_type" in
      *'|'*) emit field-domain "$cid" "type contains '|', which injects columns into the board row: $fd_type" ;;
      *) docket_change_type_is_wellformed "$fd_type" \
           || emit field-domain "$cid" "type '$fd_type' is not ^[a-z][a-z0-9-]*\$ (empty = untyped)" ;;
    esac
  fi

  # --- scalar-form call sites (the definition is hoisted to top level; change 0200). Mutation 4
  # deletes these four lines as its SECOND region, matched individually.
  sf_title="$(field_raw "$f" title)"
  sf_blocked_by="$(fm_field_verbatim "$f" blocked_by)"
  scalar_form_check title "$sf_title" "$cid"
  scalar_form_check blocked_by "$sf_blocked_by" "$cid"

  # --- broken-spec: spec set, not trivial, path absent on the metadata branch ---
  if [ -n "$spec" ] && [ "$trivial" != "true" ]; then
    git_has "$METADATA_BRANCH" "$spec" || emit broken-spec "$id" "spec not found on $METADATA_BRANCH: $spec"
  fi

  # --- broken-plan-results: a done change's set plan:/results: must resolve on the integration branch ---
  # Carve-out: never flag an 'implemented' change — those files still live on the unmerged feature branch.
  if [ "$status" = "done" ]; then
    for key in plan results; do
      val="$(fm_field "$f" "$key")"   # anchored; $key is plan|results, both optional
      [ -n "$val" ] || continue
      git_has "$INTEGRATION_BRANCH" "$val" || emit broken-plan-results "$id" "$key not found on $INTEGRATION_BRANCH: $val"
    done
  fi

  # --- stale-in-progress: lease expired (claimed_at+TTL) OR branch idle >3 days ---
  # Complements the branch-age signal with a claimed_at signal that catches the crashed-BEFORE-branch
  # blind spot (branch ref absent). The reclaimable subset (expired AND no branch ref) carries the
  # trailing [reclaimable] marker — the machine contract docket-status keys on for its remedy print.
  if [ "$status" = "in-progress" ]; then
    branch="$(fm_field "$f" branch)"        # anchored: optional keys (ADR-0057)
    claimed="$(fm_field "$f" claimed_at)"
    has_branch=0
    if branch_ref "$branch" >/dev/null; then has_branch=1; fi
    lease_secs="$(( LEASE_TTL_HOURS * 3600 ))"
    expired=0; age_h=""
    if [ -n "$claimed" ]; then
      cepoch="$(iso_to_epoch "$claimed")" || cepoch=""
      if [ -n "$cepoch" ] && [ "$(( NOW - cepoch ))" -gt "$lease_secs" ]; then
        expired=1; age_h="$(( (NOW - cepoch) / 3600 ))"
      fi
    fi
    if [ "$has_branch" = 1 ]; then
      ts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$branch" 2>/dev/null)"
      if [ -n "$ts" ] && [ "$(( NOW - ts ))" -gt "$(( 3*86400 ))" ]; then
        emit stale-in-progress "$id" "branch $branch idle >3 days (last commit $(( (NOW - ts) / 86400 ))d ago)"
      elif [ "$expired" = 1 ]; then
        emit stale-in-progress "$id" "claim lease expired ${age_h}h ago; branch $branch exists — needs your review (not auto-reclaimable)"
      fi
    elif [ "$expired" = 1 ]; then
      emit stale-in-progress "$id" "claim lease expired ${age_h}h ago; no feature branch — self-heal with docket.sh reclaim-claims [reclaimable]"
    fi
  fi

  # --- aborted-run: an in-progress change whose autonomous run stopped mid-step (change 0113).
  # An agent that dropped its bookkeeping write is the least reliable narrator of whether it
  # dropped it — both observed incidents produced confident, specific, WRONG completion reports —
  # so the oracle has to be external and mechanical. FOUR INDEPENDENT legs; any emits, and more than
  # one may emit on one change (they describe different evidence, not several views of one).
  #
  # Advisory only. It flips no status, releases no claim, and touches no file: the originating
  # incident left a real written plan a naive claim release would have stranded, and this script is
  # a pure reader by contract. Never marks EXPLAINED and never feeds board-row-dropped — a dropped
  # metadata write does not drop a board row.
  #
  # Every field here is read with the ANCHORED fm_field, never field(): plan/results/branch/
  # claimed_at are all OPTIONAL, and an unanchored read falls through the closing --- into body
  # prose (ADR-0057). In THIS repo that is not a contrived hazard — a change file whose body
  # discusses `plan:` is ordinary content, and the failure it would cause is a silent FALSE
  # NEGATIVE: prose read as a set plan: makes the check certify the exact abort it exists to catch.
  if [ "$status" = "in-progress" ]; then
    ar_branch="$(fm_field "$f" branch)"

    # Leg A — manifest/git incoherence, time-free. The feature branch carries an artifact file the
    # integration branch does not have, while the manifest field that should record it is empty.
    # The exact INVERSE of broken-plan-results (field set, file missing on the integration branch):
    # same two fields, same two trees, opposite direction. Its only false-positive window is the
    # seconds between an artifact commit and its field write, and since the finding is advisory and
    # self-clearing that race costs nothing.
    if ar_ref="$(branch_ref "$ar_branch")"; then
      if [ -z "$(fm_field "$f" plan)" ] && ar_hit="$(branch_only_artifact "$ar_ref" "$PLANS_DIR_REL")"; then
        emit aborted-run "$id" "plan committed on $ar_branch ($ar_hit) but plan: is unset — the run stopped before its metadata write; record it or re-run the step"
      fi
      if [ -z "$(fm_field "$f" results)" ] && ar_hit="$(branch_only_artifact "$ar_ref" "$RESULTS_DIR_REL")"; then
        emit aborted-run "$id" "results committed on $ar_branch ($ar_hit) but results: is unset — the run stopped before its metadata write; record it or re-run the step"
      fi
    fi

    # Leg B — run-scale stale claim. Catches the abort that leaves NOTHING in git at all: the plan
    # written to the worktree but never committed, so leg A has no artifact to see. An absent or
    # unparseable claimed_at is NO POSITIVE EVIDENCE and stays silent — never treated as expired
    # (the same posture iso_to_epoch's contract states and stale-in-progress already takes).
    ar_claimed="$(fm_field "$f" claimed_at)"
    if [ -n "$ar_claimed" ]; then
      ar_epoch="$(iso_to_epoch "$ar_claimed")" || ar_epoch=""
      if [ -n "$ar_epoch" ] && [ "$(( NOW - ar_epoch ))" -gt "$ABORTED_RUN_STALE_SECS" ]; then
        emit aborted-run "$id" "claim stamped $(( (NOW - ar_epoch) / 3600 ))h ago, past the 12h run-scale window — a run may have stopped mid-step; verify it reached its PR"
      fi
    fi

    # ar_pr is read ONCE here and shared by legs D and C below, whose gates are exact
    # complements. This path is cost-sensitive (change 0176) and a second read of the same field
    # is a real regression.
    ar_pr="$(fm_field "$f" pr)"

    # Leg D — THE STEP 7 SEAM: pr: recorded, status: never advanced (change 0219).
    # docket-implement-next writes `status: implemented` and `pr:` in ONE field-write, and no script
    # under scripts/ writes pr:. A manifest showing pr: set while status: is still in-progress is
    # therefore an anomaly BY CONSTRUCTION, not a run in flight — which is why this leg is TIME-FREE
    # with no idle floor. Leg A is the precedent and the reasoning is identical: there is no healthy
    # window to wait out, so a floor would only delay a finding that is already certain. The other
    # three legs are all blind here: leg A finds no incoherence (plan: and results: are both recorded
    # by then), leg C SHORT-CIRCUITS on a non-empty pr: by deliberate design, and leg B catches it
    # only at 12h — the same lag change 0211 exists to close.
    #
    # KNOWN RESIDUAL, and it is narrower than it looks: this script reads change files off the
    # FILESYSTEM, not out of a git blob. Combined with the single-stroke field-write, leg D's honest
    # yield is uncommitted partial edits in the shared .docket worktree, plus non-compliant drivers
    # that write the two fields separately. It is worth having as a cheap, additive completeness
    # guarantee over the Step 7 seam, NOT because it is a frequent signature. No idle floor is
    # constructible for an uncommitted edit, so no floor is correct here — but for that reason, not
    # for the reason a first draft reaches for.
    if [ -n "$ar_pr" ]; then
      emit aborted-run "$id" "pr: records $ar_pr but status: is still in-progress — the run stopped before its final status write; verify the PR and set status: implemented"
    fi

    # Leg C — BUILT BUT NOT DELIVERED (change 0211). The run finished its build and stopped before
    # delivering it: commits on the feature branch, no PR recorded. Legs A and B are both
    # STRUCTURALLY blind to this, which is why it is a third leg and not a widening:
    #   - leg A keys on manifest/git INCOHERENCE, and here every field is coherent (plan: recorded,
    #     no results file written yet). The run dropped no bookkeeping write — it dropped two steps.
    #   - leg B keys on claimed_at, which the heartbeat rider re-stamps at every phase boundary, so
    #     a run that dies just AFTER a metadata commit starts leg B's countdown from the freshest
    #     possible stamp. Leg B is at its blindest exactly when a run has just completed a step.
    # Same check-id: this is more evidence for the same conclusion ("this run stopped mid-step"), so
    # a new id would buy a four-place BOARD_CHECK_IDS edit and a second remedy vocabulary for nothing.
    #
    # Gates are ordered CHEAPEST FIRST, and the ordering is a cost contract, not a style choice:
    # the FREE frontmatter read decides the common case (a change with a recorded PR costs ZERO git
    # calls), a non-firing path costs at most four (`log -1`, the two base `show-ref`s, and
    # `rev-list -n 1`), and the remote-ref probe runs only once the leg has already decided to fire:
    # at most six in total, and five on the pushed arm, which skips the `rev-list --count`. This
    # path is cost-sensitive (change 0176).
    #
    # A non-empty pr: short-circuits the WHOLE leg. A change whose PR is recorded has delivered;
    # "unpushed branch with a recorded PR" means the PR record and the remote disagree, which is a
    # different defect with a different remedy that leg C would be a misleading oracle for.
    if [ -z "$ar_pr" ] && [ -n "$ar_ref" ]; then
      # ar_ref is REUSED from leg A but RE-GUARDED: leg C runs OUTSIDE leg A's
      # `if ar_ref="$(branch_ref …)"`, and a failed branch_ref leaves ar_ref SET BUT EMPTY. The
      # guard is a COST guard, not a correctness one, and the distinction is worth stating so
      # nobody deletes it on a false premise: `git log -1 --format=%ct ""` exits 128 with empty
      # stdout, so the `[ -n "$ar_tip" ]` test on the very next line already keeps the idle floor
      # unreachable for a change with no branch. The -n test is kept because it skips that
      # pointless git call on every branchless in-progress change. Delete it and the leg still
      # behaves correctly — it just pays for a git invocation that can only fail.
      ar_tip="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$ar_ref" 2>/dev/null)"
      if [ -n "$ar_tip" ] && [ "$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS" ]; then
        # Ahead of BOTH bases. Feature branches are cut from origin/<integration_branch> while
        # INTEGRATION_BRANCH names the LOCAL ref, and a local integration ref routinely LAGS origin
        # (sync-integration-branch.sh is FF-only and best-effort). Comparing against the local ref
        # alone makes a freshly-cut, NOTHING-BUILT branch look arbitrarily far ahead with
        # arbitrarily old commits — it would sail through the idle floor and fire leg C on the
        # exact signature (0109: stopped with nothing built) that belongs to leg B.
        #
        # BOTH bases are show-ref-verified, symmetrically. An absent refs/heads/<integration> makes
        # rev-list exit 128 with EMPTY stdout, and since the predicate reads "empty => not ahead",
        # guarding only the remote one would silently turn the whole leg into a no-op with no
        # diagnostic. No base resolving at all is SILENCE (no positive evidence) — the same posture
        # leg B takes for an unparseable claimed_at — never "ahead of nothing".
        ar_bases=()
        for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do
          "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "$ar_b" && ar_bases+=( "$ar_b" )
        done
        # The count gate must come FIRST and short-circuit. It is load-bearing in TWO different
        # ways, and neither is hypothetical (mutation M in tests/test_board_checks.sh deletes it
        # and watches both):
        #   - bash >= 4.4 expands "${ar_bases[@]}" on an empty array happily, so the rev-list below
        #     would exclude NO bases at all, list the branch's WHOLE history, and fire the leg with
        #     an empty base label — "ahead of nothing", the exact reading this leg's design forbids.
        #     (Spelled without the rev-list's own literal text: the mutation that deletes THAT
        #     predicate counts its occurrences, and a comment quoting it verbatim would break the
        #     count.)
        #   - bash before 4.4 raises `ar_bases[@]: unbound variable` under set -u. Measured, not
        #     assumed: that error kills only the command-substitution SUBSHELL, not this script, so
        #     the damage there is a diagnostic leaking onto stderr for every no-base change rather
        #     than a crash. (The reachable window is 4.0-4.3 — this script needs `mapfile` and
        #     `declare -g`, so bash 3.2 cannot run it at all.)
        if [ "${#ar_bases[@]}" -gt 0 ] && \
           [ -n "$("$GIT" -C "$CHANGES_DIR" rev-list -n 1 "$ar_ref" --not "${ar_bases[@]}" 2>/dev/null)" ]; then
          # Display values only, computed ONLY on the firing path where their cost is irrelevant and
          # they are what make the finding actionable.
          ar_idle_h=$(( (NOW - ar_tip) / 3600 ))
          # One emit site, two mutually-exclusive messages. `origin` is hardcoded, inheriting
          # branch_ref's existing convention rather than inventing a second one. A STALE
          # remote-tracking ref left by a remote-side branch deletion reads as "pushed" and yields
          # the other message — acceptable for an advisory finding whose remedy in both cases is
          # "go look at this run".
          #
          # Both messages HEDGE, matching leg B's "a run may have stopped mid-step; verify it
          # reached its PR". The predicate fires on healthy runs by construction (see the idle
          # floor's known residual in board-checks.md), so asserting the abort as fact would be a
          # claim the check cannot support — and the remedy stays a VERIFICATION, never "push it"
          # or "open the PR", which acted on against a live run races the running agent on its own
          # branch. Neither message reuses leg B's "mid-step": the hedge names leg C's own seam
          # (the push, and the gap between the push and the PR record) so a message-shape assert
          # can still tell the legs apart.
          if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then
            emit aborted-run "$id" "$ar_branch is pushed but pr: is unset (last commit ${ar_idle_h}h ago) — a run may have stopped between its push and its PR record; verify the PR exists"
          else
            # The count and its base label are built HERE, on the only arm that prints them: the
            # pushed arm above would pay for a second rev-list traversal it never reads.
            ar_ahead="$("$GIT" -C "$CHANGES_DIR" rev-list --count "$ar_ref" --not "${ar_bases[@]}" 2>/dev/null)"
            # Name the bases the count ACTUALLY excluded, read back off ar_bases rather than
            # hardcoded. With a stale local integration ref the two bases differ, and a message
            # naming only "$INTEGRATION_BRANCH" cannot be reconciled with
            # `git rev-list --count <integration>..<branch>`; naming BOTH unconditionally would be a
            # fresh lie whenever only one base resolved. The ref prefixes are stripped so it reads
            # as the names a human types (`main and origin/main`).
            ar_base_label=""
            for ar_bl in "${ar_bases[@]}"; do
              ar_bl="${ar_bl#refs/heads/}"; ar_bl="${ar_bl#refs/remotes/}"
              ar_base_label="${ar_base_label:+$ar_base_label and }$ar_bl"
            done
            # "1 commits" reads as a bug in the check itself, which is corrosive for an advisory.
            if [ "$ar_ahead" = 1 ]; then ar_noun=commit; else ar_noun=commits; fi
            emit aborted-run "$id" "$ar_ahead $ar_noun on $ar_branch ahead of $ar_base_label, branch never pushed and pr: is unset (last commit ${ar_idle_h}h ago) — a run may have stopped before it pushed; verify it is not still building"
          fi
        fi
      fi
    fi
  fi

  # --- merge-gate-stall: build-ready, but its worst-unmet dep is stuck at 'implemented' ---
  if [ "$status" = "proposed" ] && { [ -n "$spec" ] || [ "$trivial" = "true" ]; }; then
    if [ "${DEP_REASON[$id]:-}" = "needs your merge" ]; then
      emit merge-gate-stall "$id" "build-ready but waiting on #${DEP_ON[$id]} — needs your merge"
    fi
  fi

  # --- stale-finalize-blocked: an 'implemented' change carrying the `## Finalize blocked` marker
  # whose marker has outlived FINALIZE_BLOCKED_STALE_SECS (change 0098). The marker's only clearing
  # path is a docket-finalize-change run; when a human resolves the underlying cause out of band
  # (without re-running finalize with the id named) the marker sits on the board indefinitely. This
  # is a git-only, time-based advisory: it cannot know whether the cause still holds (that needs a
  # network probe this script forbids), so it fires on ANY marker past the horizon — a marker still
  # genuinely blocked that long is itself worth a human glance. Marker age = the change file's
  # last-commit timestamp (git ct is tamper-proof; the in-body date is model-authored prose). Never
  # mutates the file / auto-clears the marker — that stays docket-finalize-change's job.
  if [ "$status" = "implemented" ] && finalize_blocked "$f"; then
    # "$f" is a pathspec here (unlike the ref args elsewhere in this script). It comes from
    # `find "$CHANGES_DIR/..."`, so it is absolute whenever --changes-dir is absolute — which the
    # real docket-status invocation and the tests always are, and git resolves an absolute pathspec
    # against the worktree root regardless of the -C cwd. A RELATIVE --changes-dir would make this
    # resolve against the changed cwd and return empty (→ silently never fires); pass an absolute dir.
    fbts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct -- "$f" 2>/dev/null)"
    if [ -n "$fbts" ] && [ "$(( NOW - fbts ))" -gt "$FINALIZE_BLOCKED_STALE_SECS" ]; then
      emit stale-finalize-blocked "$id" "## Finalize blocked marker set $(( (NOW - fbts) / 3600 ))h ago — resolve the cause and re-run finalize $id, or it will sit on the board"
    fi
  fi

  # --- publish-deferred: the change carries the `## Publish deferred` marker (change 0083).
  # A terminal close-out's publish step was EXPECTED (terminal_publish: true, docket-mode) but
  # deferred or blocked, so the archived record never reached the integration branch. Before this
  # check, board-checks.sh had NO terminal-record check at all and certified exactly this gap
  # clean for eight days (#0043).
  #
  # NO status gate and NO directory gate, both deliberate: the marker is written on the ARCHIVED
  # file (terminal status), and a status gate would make it unreadable where it is written. The
  # marker's PRESENCE is the entire state — mark-publish-deferred.sh writes it only on the defer
  # path and terminal-publish.sh removes it on success, so a marker in the tree always means a
  # pending deferral. An `active/` file carrying one (a close-out interrupted before archiving)
  # reports the same way, harmlessly.
  #
  # Reads the marker in the change file — NOT a `git cat-file -e origin/<integration>:<path>`
  # set-diff. That would reintroduce the detector this change deliberately declined (spec §1a),
  # fire forever under `terminal_publish: false`, and break the script's git-only/offline
  # invariant. This check neither marks EXPLAINED nor feeds board-row-dropped: a body section
  # cannot drop a board row.
  if publish_deferred "$f"; then
    emit publish-deferred "$cid" "terminal-publish to $INTEGRATION_BRANCH not completed — record on $METADATA_BRANCH only; complete the publish or record a decision not to"
  fi

  # --- stack-invalid / stack-parent-killed: a stacked change whose EFFECTIVE BASE does not resolve
  # (change 0298). A change carrying `stacked_on:` is built on its parent's feature branch, and
  # every consumer of that answer — the branch cut, the PR base, the rebase target — asks
  # stack_effective_base for it. When the resolver cannot answer, the failure surfaces at the moment
  # someone tries to build or finalize the change; these two checks move it onto the board instead.
  #
  # TWO checks, not one, because the REMEDIES differ and a finding a human cannot act on is noise:
  # exit 3 (the chain reaches a KILLED parent) is a scoping decision only a human makes — spec §9
  # forbids silently falling back to the integration branch — while exit 4 (missing parent, cycle,
  # or a parent branch with no remote ref) is a data or sequencing repair. Collapsing them is what
  # tests/test_board_checks_stack.sh's separateness asserts redden on.
  #
  # ONE call, then a branch on its status. Calling the resolver once per check would let the two
  # legs disagree about the same file: the resolver reads the tree and the remote refs, both of
  # which a concurrent run can move between two calls, and the second answer would silently win.
  #
  # `stacked_on` is an OPTIONAL key, so it is read with the ANCHORED accessor (ADR-0057): an
  # unanchored read runs past the closing `---` and would pick up a body-prose line, which in THIS
  # repo's change files is ordinary content.
  #
  # Scoped to NON-TERMINAL changes: a `done` or `killed` change's chain is history, and neither
  # re-parenting nor pushing a branch is something anyone can still do about it.
  if ! docket_status_is_terminal "$status"; then
    sc_parent="$(fm_field "$f" stacked_on)"
    if [ -n "$sc_parent" ]; then
      # Padded for the message only. A non-numeric value never reaches here with a finding attached
      # — stack_parent_id declines it quietly and the resolver then answers "unstacked", exit 0 —
      # so the arm exists to keep the interpolation total rather than to report that case.
      case "$sc_parent" in
        *[!0-9]*) sc_parent_label="$sc_parent" ;;
        *) sc_parent_label="$(printf '%04d' "$(( 10#$sc_parent ))")" ;;
      esac
      # NO `-C` wrapper over the GIT seam here: the resolver addresses its own `show-ref` at the
      # changes dir it is handed, and a second `-C` would compose relatively against the first.
      # Called DIRECTLY, never inside `$(…)`: exit 3's killed-ancestor id comes back in the global
      # STACK_KILLED_ANCESTOR, and a command substitution would discard it with the subshell.
      stack_effective_base "$CHANGES_DIR" "$id" "$INTEGRATION_BRANCH" >/dev/null 2>&1
      sc_rc=$?
      case "$sc_rc" in
        3)
          # WHICH change is killed is not knowable from this change's own `stacked_on:`: the
          # resolver recurses through a `stacked-merged` parent whose branch is gone, so exit 3 can
          # name an ancestor several hops up. Asserting the immediate parent is killed would point
          # the human at a change that is not, so the id decides the phrasing rather than the
          # phrasing assuming the id. The third arm keeps the message honest if the channel ever
          # arrives empty — an id the resolver did not publish is one this check must not invent.
          sc_killed_label=""
          case "${STACK_KILLED_ANCESTOR:-}" in
            ''|*[!0-9]*) ;;
            *) sc_killed_label="$(printf '%04d' "$(( 10#$STACK_KILLED_ANCESTOR ))")" ;;
          esac
          if [ -n "$sc_killed_label" ] && [ "$sc_killed_label" = "$sc_parent_label" ]; then
            sc_killed_clause="stacked on #$sc_parent_label, which is killed"
          elif [ -n "$sc_killed_label" ]; then
            sc_killed_clause="stacked on #$sc_parent_label, whose stacked_on chain reaches killed change #$sc_killed_label"
          else
            sc_killed_clause="stacked on #$sc_parent_label, whose stacked_on chain reaches a killed change"
          fi
          emit stack-parent-killed "$cid" "$sc_killed_clause — rescope this change onto $INTEGRATION_BRANCH, re-parent it onto a live change, or kill it too; there is no safe automatic fallback" ;;
        4) emit stack-invalid "$cid" "stacked_on chain does not resolve to a base branch (parent #$sc_parent_label) — repair the stacked_on id if the parent is missing, break the cycle if it closes one, or push the parent's branch to origin if it was never pushed" ;;
      esac
    fi
  fi
done

# --- adr-unpublished: an ADR whose publish onto the integration branch is DUE but did not happen,
# or that drifted after publication (change 0117). Computed, not marked: unlike publish-deferred,
# this needs nothing at all from the run that went wrong — which is the whole point, since the
# failure mode being closed is that NOBODY NOTICED. The ADR corpus has no marker seam to hang a
# marker on anyway: an ADR file is never moved (no archive moment) and an Accepted ADR is immutable
# except its status: line. See ADR-0051's boundary — that decision declined a detector-AND-HEALER
# over CHANGE records; this is a read-only report over ADRs and reverses nothing.
#
# Gated twice, both legs required (spec §4.4): --adrs-dir supplied AND --terminal-publish passed.
# The caller passes --terminal-publish only under `terminal_publish: true` AND docket-mode; under
# the default `false` the ledger deliberately lives on the metadata branch only, so an ungated
# check would fire on every ADR forever, and in main-mode the two refs coincide so the comparison
# is vacuous.
if [ -n "$ADRS_DIR" ] && [ "$ADR_GATE" = 1 ]; then
  # Repo-relative path prefix for the ADR dir, derived from git itself rather than from the
  # config value: the script is handed a FILESYSTEM path (as with --changes-dir) but must probe
  # refs, which are addressed repo-relative. --show-prefix is worktree-root-relative, which is
  # exactly what `<ref>:<path>` wants, and it needs no network.
  if ! adr_prefix="$("$GIT" -C "$ADRS_DIR" rev-parse --show-prefix 2>/dev/null)"; then
    printf 'board-checks: adrs dir is not inside a git worktree: %s\n' "$ADRS_DIR" >&2; exit 2
  fi
  mapfile -t ADR_FILES < <(find "$ADRS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
  # Guarded expansion (same idiom as commit 0695b921): an ADR dir that EXISTS but is EMPTY — the
  # normal state of a repo that opted in before writing its first ADR — leaves ADR_FILES declared
  # but empty, and "${ADR_FILES[@]}" throws "unbound variable" under set -u on bash 4.0-4.3 (this
  # repo's floor), aborting the script before FINDINGS prints and losing every other check's output.
  for af in ${ADR_FILES[@]+"${ADR_FILES[@]}"}; do
    a_num="$(padded_id_from_file "$af")"
    [ "$a_num" = '?' ] && continue          # not a numbered ADR file; adr-checks.sh owns naming hygiene
    a_rel="${adr_prefix}$(basename "$af")"
    # fm_field, never field: `change:` is legitimately ABSENT on a standalone ADR, and field()
    # would fall through and read body prose as its value.
    a_status="$(fm_field "$af" status)"
    a_change="$(fm_field "$af" change)"
    a_change_id=""
    case "$a_change" in
      ''|*[!0-9]*) ;;                        # absent, or not a bare integer -> unresolvable
      *) a_change_id="$(( 10#$a_change ))" ;;
    esac
    # ADR-0049: the change-id column carries only script-derived or shape-validated values. The
    # validated change id when there is one; otherwise `?`, the same fallback padded_id_from_file
    # already uses for a file whose id is unusable. The ADR number rides the MESSAGE column, which
    # is the last field of the caller's `read` and cannot shift a field.
    a_cid="${a_change_id:-?}"
    m_blob="$("$GIT" -C "$ADRS_DIR" rev-parse --verify -q "$METADATA_BRANCH:$a_rel" 2>/dev/null)"
    i_blob="$("$GIT" -C "$ADRS_DIR" rev-parse --verify -q "$INTEGRATION_BRANCH:$a_rel" 2>/dev/null)"

    if [ -n "$i_blob" ]; then
      # Present on the integration branch => due FOREVER, whatever its status. Deliberately
      # status-blind: an ADR published while Accepted must keep tracking its bytes after it is
      # Superseded or Reversed, and an Accepted-only gate here would silence exactly the
      # un-re-published status flip this arm exists to catch — the case a marker structurally
      # cannot see, because nothing FAILED at publish time.
      #
      # Blob-SHA equality, not a byte-by-byte diff: git already content-addresses both sides, so
      # the compare is one rev-parse each and needs no working-tree read. A missing m_blob (the
      # ADR is on the integration branch but not committed on the metadata branch) has nothing to
      # compare against, so it stays silent rather than guessing.
      if [ -n "$m_blob" ] && [ "$m_blob" != "$i_blob" ]; then
        emit adr-unpublished "$a_cid" "ADR-$a_num differs between $METADATA_BRANCH and $INTEGRATION_BRANCH — re-publish it (docket.sh terminal-publish --adr $a_num)"
      fi
      continue
    fi

    # Absent on the integration branch. Never expected there unless the publish trigger has fired.
    # An ADR not on the metadata branch at all (m_blob empty — working-tree-only, uncommitted) is
    # not yet a publish obligation: nothing to publish FROM, and the remedy this arm would print
    # (docket.sh terminal-publish --adr) reads its copy-set from the metadata branch and would
    # fail outright. Mirrors the stale arm's own "nothing to compare against" reasoning above.
    [ -n "$m_blob" ] || continue
    [ "$a_status" = "Accepted" ] || continue
    if [ -n "$a_change" ]; then
      # Change-tied: due only once its change reached a TERMINAL status. An unresolvable
      # change: value stays silent — absence of a resolvable link is not evidence of a gap.
      [ -n "$a_change_id" ] || continue
      docket_status_is_terminal "${STATUS_OF[$a_change_id]:-}" || continue
    fi
    emit adr-unpublished "$a_cid" "ADR-$a_num is due on $INTEGRATION_BRANCH but absent — publish it (docket.sh terminal-publish --adr $a_num)"
  done
fi

# --- board-row-dropped: an ACTIVE-or-ARCHIVE file counted in the board's total but not accounted
# --- for by its directory's pass ---
# The membership test is renders_row() (above), computed from the renderer's own bucketing — NOT a
# restatement of the causes the enumerated checks name. SUPPRESSED when a finding already accounts
# for the drop: `malformed-id` (non-integer id) or a `field-domain` **status** finding. Those are the
# only two arms that mark EXPLAINED, because they are the only two that describe a row DISAPPEARING;
# a bad slug/priority/title deliberately does not suppress (see the field-domain block).
# Unsuppressed, this finding says exactly one thing: a row vanished and nothing enumerated explains
# why. Four live triggers today, one set per directory —
#   active/:
#     (a) a file with NO `id:` field at all (malformed-id needs a non-empty raw value to fire), and
#     (b) an `active/` file carrying a TERMINAL status (`done`/`killed`): a legal status in the wrong
#         directory, so every enumerated check is correctly silent and only the computed invariant
#         sees it (the `sweep-failed <id> archive <reason>` state — status flipped, archive move
#         failed).
#   archive/:
#     (c) an `archive/` file carrying a NON-TERMINAL status: the symmetric case, a legal status in
#         the wrong directory, reachable from the same interrupted operation as (b) but on the other
#         side of the move.
#     (d) a terminal `archive/` file with NO usable id: the same "no `id:` field at all" shape as
#         (a), evaluated after the file has already moved.
# Beyond those, the remaining trigger on either side is a future renderer-added drop path: because
# renders_row reads DOCKET_STATUSES_ACTIVE and DOCKET_STATUSES_TERMINAL — the two arrays
# render-board.sh's own section iteration uses — a status either side of the renderer stops
# rendering starts reporting here with no edit to this script.
for drop_id in "${!DROPPED[@]}"; do
  [ -n "${EXPLAINED[$drop_id]:-}" ] && continue
  # The message names the two SUPPRESSING arms specifically, not "field-domain" wholesale: a change
  # can legitimately carry a field-domain finding (a piped title, say) AND this one, because that
  # finding does not account for a dropped row. Saying "no field-domain finding explains it" next to
  # a visible field-domain finding on the same id would read as a contradiction.
  # Two strings, one per direction: the direction is what tells the reader which way the file is
  # misfiled, and it is the reason this is a widened check rather than a second check-id.
  if [ "${DROPPED_DIR[$drop_id]:-active}" = archive ]; then
    emit board-row-dropped "$drop_id" "counted in the board total but not accounted for by the archive pass (no row identifying it, or a summary count that excludes it); no malformed-id or field-domain status finding accounts for the drop"
  else
    emit board-row-dropped "$drop_id" "counted in the board total but rendered in no section; no malformed-id or field-domain status finding accounts for the drop"
  fi
done

# --- dep-cycle: DFS over depends_on; mark every node that lies on a cycle ---
declare -A ADJ COLOR INSTACK ONCYCLE
for f in "${FILES[@]}"; do
  cid="$(int_field "$f" id)"; [ -n "$cid" ] || continue
  ADJ["$cid"]="$(list_field "$f" depends_on)"
done
PATH_STACK=()
dfs(){ # dfs NODE — colors: white(unset) / gray(on stack) / black(done)
  local node="$1" dep i seen
  COLOR["$node"]=gray; INSTACK["$node"]=1; PATH_STACK+=("$node")
  for dep in ${ADJ["$node"]:-}; do
    [ -n "${ADJ[$dep]+x}" ] || continue            # dep is not a known change ⇒ not a graph edge
    if [ "${INSTACK[$dep]:-0}" = 1 ]; then
      seen=0                                        # back edge: mark dep..top-of-stack
      for i in "${PATH_STACK[@]}"; do
        [ "$i" = "$dep" ] && seen=1
        [ "$seen" = 1 ] && ONCYCLE["$i"]=1
      done
    elif [ "${COLOR[$dep]:-white}" = white ]; then
      dfs "$dep"
    fi
  done
  COLOR["$node"]=black; INSTACK["$node"]=0
  PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")   # pop (bash-4.0-safe; no unset arr[-1])
}
for node in "${!ADJ[@]}"; do
  [ "${COLOR[$node]:-white}" = white ] && dfs "$node"
done
for node in "${!ONCYCLE[@]}"; do
  emit dep-cycle "$node" "participates in a depends_on cycle"
done

# --- merged-orphan / unknown-commit-ref: cross-reference integration-branch commit subjects
#     against the active/archive change set. Git-only, subjects only, conservative grammar
#     (numeric conventional-commit scope + a "(change N)" tag (conventionally trailing;
#     matched anywhere in the subject)); bare #N and bodies excluded to bound PR-number
#     false positives. Zero-padding tolerated (10# strips it). Full history.
declare -A REF_EVIDENCE                       # id -> "<short-sha> <subject>" (first commit seen)
re_scope='^[a-zA-Z]+\(0*([0-9]{1,4})\):'      # docket(0085): … / results(0085): …
re_trailing='\(change 0*([0-9]{1,4})\)'       # … (change 0085)
while IFS=$'\t' read -r ev_sha ev_subject; do
  [ -n "$ev_subject" ] || continue
  refs=""
  [[ "$ev_subject" =~ $re_scope ]]    && refs+=" $(( 10#${BASH_REMATCH[1]} ))"
  [[ "$ev_subject" =~ $re_trailing ]] && refs+=" $(( 10#${BASH_REMATCH[1]} ))"
  for rid in $refs; do
    [ -n "${REF_EVIDENCE[$rid]:-}" ] || REF_EVIDENCE["$rid"]="$ev_sha $ev_subject"
  done
done < <("$GIT" -C "$CHANGES_DIR" log --format='%h%x09%s' "$INTEGRATION_BRANCH" 2>/dev/null)

for rid in "${!REF_EVIDENCE[@]}"; do
  ev="${REF_EVIDENCE[$rid]}"
  if [ -n "${ID_ACTIVE[$rid]:-}" ]; then
    emit merged-orphan "$rid" "merged on $INTEGRATION_BRANCH ($ev) but still active (not archived)"
  elif [ -z "${ID_EXISTS[$rid]:-}" ]; then
    emit unknown-commit-ref "$rid" "referenced by $INTEGRATION_BRANCH commit ($ev) but no change file exists"
  fi
  # archived (terminal) ⇒ properly closed out ⇒ no finding
done

# Emit findings sorted by (check-id asc, change-id numeric asc) for determinism.
if [ -n "$FINDINGS" ]; then
  printf '%s' "$FINDINGS" | sort -t"$(printf '\t')" -k1,1 -k2,2n
fi

if [ "$STRICT" = 1 ] && [ -n "$FINDINGS" ]; then exit 1; fi
exit 0
