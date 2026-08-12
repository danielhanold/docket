#!/usr/bin/env bash
# scripts/render-board.sh — deterministic, idempotent renderer for docket's `inline` board
# surface (change 0022). Reads the change files (active/ + archive/) and emits BOARD.md to STDOUT
# byte-for-byte per docket-status's *Board -> Structure*. No git writes (the caller redirects +
# commits), offline (no gh, no network). Same change files => identical bytes.
#
# Usage: render-board.sh --changes-dir DIR [--repo OWNER/REPO] [--format markdown|digest]
#                        [--type TYPE|untyped|all] [--priority PRIORITY|all]
#   --repo builds pr: hyperlinks; defaults to deriving OWNER/REPO from the origin remote of
#   --changes-dir. Mock seam: GIT="${GIT:-git}".
#   --format markdown (default) emits the BOARD.md markdown; --format digest emits the
#   line-oriented backlog digest (`backlog <status> <count>` + `change <id> <status> <readiness>
#   <slug>` + a final `ready <id> …` queue line, change 0094) — a second projection of the same
#   dependency/readiness pass, consumed by docket-status.sh's report. The digest is REPORT OUTPUT,
#   NOT a board surface: it is never persisted, committed, or written to BOARD.md.
set -uo pipefail

GIT="${GIT:-git}"

# Count-based recency window over the archive: the archive table lists every killed entry plus the
# ARCHIVE_RECENT most-recent `done` entries verbatim; older `done` entries collapse into a per-month digest.
# Count-based (not time-based) keeps the renderer deterministic — same change files, identical bytes.
ARCHIVE_RECENT=15

CHANGES_DIR=""
REPO=""
FORMAT="markdown"
FILTER_TYPE="all"
FILTER_PRIORITY="all"
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --repo) REPO="$2"; shift ;;
    --format) FORMAT="$2"; shift ;;
    --type|--priority)
      # Guard the arity: without it a trailing `--type` reads an unset $2 under `set -u` and dies
      # with a raw "unbound variable" trace instead of the documented exit-2 argument error.
      [ $# -ge 2 ] || { printf 'render-board: %s requires a value\n' "$1" >&2; exit 2; }
      case "$1" in
        --type)     FILTER_TYPE="$2" ;;
        --priority) FILTER_PRIORITY="$2" ;;
      esac
      shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-board: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || { printf 'render-board: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ] || { printf 'render-board: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }
case "$FORMAT" in
  markdown|digest) : ;;
  *) printf 'render-board: unknown --format value: %s (expected markdown|digest)\n' "$FORMAT" >&2; exit 2 ;;
esac

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

# --- report filter validation (change 0127) ---------------------------------------------------
# `all` is the wildcard and is exactly equivalent to omitting the option. A --type filter accepts
# any WELL-FORMED token rather than only the effective change_types: a repository legitimately
# contains types written under another machine's configuration, and a query for one of those must
# work. `untyped` is the query token for a change carrying no type: at all.
if [ "$FILTER_TYPE" != all ] && [ "$FILTER_TYPE" != untyped ]; then
  docket_change_type_is_wellformed "$FILTER_TYPE" || {
    printf 'render-board: unknown --type value: %s (expected all, untyped, or a [a-z][a-z0-9-]* token)\n' \
      "$FILTER_TYPE" >&2
    exit 2
  }
fi
if [ "$FILTER_PRIORITY" != all ]; then
  docket_priority_is_member "$FILTER_PRIORITY" || {
    printf 'render-board: unknown --priority value: %s (expected all, %s)\n' \
      "$FILTER_PRIORITY" "${DOCKET_PRIORITIES[*]}" >&2
    exit 2
  }
fi

# digest_admits FILE — the report-only projection filter. Consulted ONLY inside the digest block:
# the markdown writer must never call it, or a filtered --board-only run would commit a TRUNCATED
# BOARD.md. That boundary is asserted in tests/test_render_board.sh.
digest_admits(){
  local t p
  if [ "$FILTER_TYPE" != all ]; then
    t="$(fm_field "$1" type)"; t="${t:-untyped}"
    [ "$t" = "$FILTER_TYPE" ] || return 1
  fi
  if [ "$FILTER_PRIORITY" != all ]; then
    p="$(field "$1" priority)"
    # Substitute the default for an UNRECOGNIZED value, not merely an absent one, because
    # docket_priority_rank does exactly that when it sorts. Admitting only on an exact string
    # match made the two disagree: a change carrying `priority: Medium` sorts into the medium band
    # in the unfiltered queue, yet matched NO --priority value — not even `medium` — so it was
    # reachable only via `--priority all`, and any narrowed selection silently dropped it.
    docket_priority_is_member "$p" || p="$DOCKET_PRIORITY_DEFAULT"
    [ "$p" = "$FILTER_PRIORITY" ] || return 1
  fi
  return 0
}

# type_cell FILE — the stored value verbatim, or `untyped` when absent. Deliberately NOT validated
# against the effective change_types: configuration governs CREATION, never the readability of
# shared history, so a type this machine does not configure still renders. Row visibility never
# depends on the type — a type problem must not drop a row (change 0127).
type_cell(){
  local t; t="$(fm_field "$1" type)"
  printf '%s' "${t:-untyped}"
}

# Derive OWNER/REPO from the origin remote when --repo is unset (best-effort, offline). Skipped
# entirely for --format digest (change 0094): REPO is consumed only by the markdown renderer's
# pr: hyperlinks (below), never by the digest projection — the digest needs no remote, so THIS
# RENDERER derives none. Scoped deliberately to a claim about render-board.sh alone: its caller,
# docket-status.sh's --digest-only, still invokes git elsewhere (docket_metadata_worktree's
# read-only `worktree list` anchor call) — see tests/test_docket_status.sh's --digest-only fixture
# comment for that half. "No remote derivation here" and "the --digest-only pass makes no git call
# at all" are different claims; only the first is true, and this comment no longer overstates it.
if [ -z "$REPO" ] && [ "$FORMAT" != digest ]; then
  url="$("$GIT" -C "$CHANGES_DIR" remote get-url origin 2>/dev/null || true)"
  if [ -n "$url" ]; then
    REPO="${url%.git}"; REPO="${REPO#git@github.com:}"; REPO="${REPO#https://github.com/}"
  fi
fi

pad(){ printf '%04d' "$1"; }                  # bare id -> 4-digit
emoji_for(){ case "$1" in
  in-progress) printf '🟢';; proposed) printf '🟡';; blocked) printf '🔴';;
  deferred) printf '⚪';; implemented) printf '🔵';; stacked-merged) printf '🪆';;
  done) printf '✅';; killed) printf '🗑️';;
esac; }
label_for(){ case "$1" in in-progress) printf 'in progress';; *) printf '%s' "$1";; esac; }
spec_link(){ printf '../%s' "${1#docs/}"; }   # docs/superpowers/specs/X -> ../superpowers/specs/X

resolve_deps "$CHANGES_DIR"

# sanitize VALUE — render TAB and CR as the visible two-character escapes \t and \r. Duplicated
# from board-checks.sh's `sanitize VALUE — render TAB and CR as the visible two-character escapes`
# rather than hoisted to the shared lib: hoisting would edit board-checks.sh, which change 0259
# holds explicitly out of scope for a three-line dedupe.
# Pure bash parameter expansion — BSD sed does not interpret \t in a pattern, so a sed form would
# be silently wrong. Used ONLY in diagnostics: a malformed value is REJECTED before it can reach a
# feeder, never laundered into one.
sanitize(){ local v="$1"; v="${v//$'\t'/\\t}"; v="${v//$'\r'/\\r}"; printf '%s' "$v"; }

# --- malformed-input validation (change 0259) -------------------------------------------------
# One upfront pass over every change file, BEFORE the section/tally loops read anything. The
# renderer's contract is "render complete output modulo skipped rows, then fail loud": a malformed
# file is excluded from every projection, diagnosed on stderr, and makes the whole run exit 3.
#
# "Malformed" is a CLOSED enumeration, deliberately narrow:
#   M1 unusable id     — int_field yields nothing (absent, empty, or non-integer).
#   M2 empty status    — field yields nothing.
#   M3 status outside DOCKET_STATUSES — which SUBSUMES any status carrying an interior TAB or CR,
#      because a control-char value can never match one of the closed vocabulary names. Rejection by
#      vocabulary IS the sanitization for `status:`, applied before the value ever reaches the
#      archive TAB join or an ARC_COUNT/SECTION array subscript.
#   M5 path carrying a TAB or CR — the one control-character source that never passes through a
#      frontmatter read, so no value check can see it. It is checked HERE, upfront over both
#      directories, rather than at a feeder: the ACTIVE side joins `id<TAB>file` into SECTION and
#      splits it back at FOUR consumers (print_section, the digest `change` loop, the mermaid loop,
#      the ready-queue loop), so a per-consumer read-back guard would have to be written four times
#      and would still miss the fifth consumer somebody adds. One upfront path check covers every
#      active consumer AND the archive feeder at once.
# M5 is numbered after M4 because M4 shipped first; it nevertheless runs in THIS pass, above it.
# Deliberately NOT malformed: a vocabulary-valid status in the "wrong" directory (`done` in
# active/ is legitimate mid-sweep state — failing on it would make docket-status's own sweep window
# a renderer failure); an unknown or absent type: (change 0127 — a type problem must never affect a
# row); a malformed created: (change 0094's 9999-99-99 sentinel already owns it).
declare -A BAD       # file path -> 1 when excluded from every projection
# The upfront pass already reads `id:` and `status:` out of every file, so it CACHES both rather
# than discarding them: every later consumer (the SECTION loop, the archive feeder, ARC_COUNT, the
# digest `change` loop, the done-node loop) reads the cache instead of re-parsing the file. That is
# a measured saving on a several-hundred-file backlog, and it also removes the possibility of two
# passes disagreeing if a file changes mid-run. Keys are the VERBATIM path, exactly as `BAD` is
# keyed, so a cache read and a BAD gate always speak about the same file. Only a file that survived
# every class is cached — a `${VALID_ID[$f]:-}` miss and a BAD hit are the same set.
declare -A VALID_ID  # file path -> validated integer id  (populated only for non-BAD files)
declare -A VALID_ST  # file path -> validated lifecycle status
MALFORMED=0
# The PATH is sanitized too, not only the offending value: a control character can live in a
# filename (M4's case), and the diagnostic stream must never carry a raw one. `BAD` is still keyed
# on the verbatim path, because that is what every consumer's gate compares against.
mark_malformed(){    # mark_malformed FILE REASON
  BAD["$1"]=1
  MALFORMED=$(( MALFORMED + 1 ))
  printf 'render-board: malformed change file: %s: %s\n' "$(sanitize "$1")" "$2" >&2
}

# Collect active files by status (ascending id), and archive rows.
declare -A SECTION         # status -> newline-separated "id\tfile"
mapfile -t AFILES < <(find "$CHANGES_DIR/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
mapfile -t ARCFILES < <(find "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)

# --- count line ---
# `total` deliberately still counts malformed files — an unaccounted-for file is exactly what
# board-checks.sh's `board-row-dropped` reports, and change 0115 owns that.
total=${#AFILES[@]}
total=$(( total + ${#ARCFILES[@]} ))

# Both directories, one pass. Iterates FILES, never the status array: render-board.sh's
# full-vocabulary iteration count is pinned at 2 by tests/test_render_board.sh, and membership here
# goes through docket_status_is_member so the vocabulary keeps exactly one source.
for f in ${AFILES[@]+"${AFILES[@]}"} ${ARCFILES[@]+"${ARCFILES[@]}"}; do
  # M5 first: a control character in the PATH corrupts every downstream TAB join and would be
  # rendered raw into a markdown link, whatever the frontmatter says.
  if [ "$f" != "${f//[$'\t\r']/}" ]; then
    mark_malformed "$f" "path contains a TAB or CR"
    continue
  fi
  v_id="$(int_field "$f" id)"
  if [ -z "$v_id" ]; then
    mark_malformed "$f" "unusable id (absent, empty, or non-integer)"
    continue
  fi
  v_st="$(field "$f" status)"
  if [ -z "$v_st" ]; then
    mark_malformed "$f" "empty status"
    continue
  fi
  if ! docket_status_is_member "$v_st"; then
    # Count-free wording: a hardcoded cardinality re-arms the same trap the vocabulary array exists
    # to disarm, so the diagnostic interpolates the array instead — the shape board-checks.sh's
    # `field-domain` diagnostic already uses.
    mark_malformed "$f" "status '$(sanitize "$v_st")' is not one of the lifecycle statuses: ${DOCKET_STATUSES[*]}"
    continue
  fi
  VALID_ID["$f"]="$v_id"
  VALID_ST["$f"]="$v_st"
done

# The BAD gate here is DEFENCE IN DEPTH and is currently unobservable — deliberately recorded so a
# reader does not mistake it for tested behaviour. M1/M2 are each already caught by this loop's own
# `[ -n … ]` guards — which now read the cache, so a miss and a BAD hit coincide by construction —
# and an M3 status can only ever become a SECTION key nobody reads (every
# consumer iterates DOCKET_STATUSES_ACTIVE). Removing this line reddens no assert. It stays so that
# `BAD` means exactly one thing — "excluded from every projection" — at all three consumers, which
# is what keeps a future consumer from inheriting the hole.
for f in "${AFILES[@]}"; do
  [ -z "${BAD[$f]:-}" ] || continue
  id="${VALID_ID[$f]:-}"; [ -n "$id" ] || continue
  st="${VALID_ST[$f]:-}"; [ -n "$st" ] || continue
  SECTION["$st"]+="$id"$'\t'"$f"$'\n'
done

# rows_sorted STATUS -> emits "id<TAB>file" lines for that status, ascending id
rows_sorted(){ printf '%s' "${SECTION[$1]:-}" | sed '/^$/d' | sort -t$'\t' -k1,1n; }
count_of(){ rows_sorted "$1" | grep -c . ; }

# --- archive feeder + read-back validation, M4 (change 0259) -----------------------------------
# Build the archive table's `date<TAB>id<TAB>status<TAB>path` rows ONCE, here, ABOVE every
# projection — and validate each tuple as it is built. Both halves of that placement are
# load-bearing, and each fixes a hole this check had while it lived at the render-time consumer:
#   * it sat inside the markdown archive block, BELOW the digest projection's own exit, so
#     `--format digest` never evaluated M4 at all — the two enumerated consumers then disagreed
#     over identical input, which is exactly the two-cases-one-signal collapse this change exists
#     to close;
#   * it fired after the tally loops, so a rejected file still counted in ARC_COUNT, in the
#     `<summary>` header, and in the digest rollup — the header claimed two rows above one.
# Running the check here restores ONE meaning for `BAD` at every consumer: "excluded from every
# projection".
#
# The check is a genuine ROUND TRIP, not a re-derivation: join the tuple, split it back with the
# consumer's own `IFS=$'\t' read -r`, and require every field to arrive unchanged. That catches any
# split that slid left or right, whatever field the stray TAB came from, and it keys rejection on
# the AUTHORITATIVE path — a shifted tuple's final field is a fragment, and marking a fragment BAD
# would leave the real file counted. The second conjunct then rejects a residual TAB or CR inside
# the path, which round-trips intact precisely because `read` assigns the unsplit remainder to the
# final field: such a path resolves on disk and carries a valid status, yet emitting it would write
# a raw control character into the rendered link. That is the conjunct a TAB in a FILENAME trips,
# and it is why M4 is not a duplicate of M1-M3 — a filename never passes through a frontmatter read.
# Since M5 ("path contains a TAB or CR") now rejects such a path UPFRONT over both directories,
# this conjunct is DEFENCE IN DEPTH rather than the sole owner of that case: unobservable today,
# kept deliberately so a future edit feeding this loop from somewhere other than the validated
# ARCFILES still fails loud.
ARC_ROWS=()
for f in "${ARCFILES[@]}"; do
  [ -z "${BAD[$f]:-}" ] || continue
  base="$(basename "$f")"; d="${base:0:10}"; id="${VALID_ID[$f]:-}"; st="${VALID_ST[$f]:-}"
  [ -n "$id" ] && [ -n "$st" ] || continue
  tuple="$d"$'\t'"$id"$'\t'"$st"$'\t'"$f"
  IFS=$'\t' read -r r_d r_id r_st r_f <<<"$tuple"
  if [ "$r_d" != "$d" ] || [ "$r_id" != "$id" ] || [ "$r_st" != "$st" ] || [ "$r_f" != "$f" ] \
     || [ "$r_f" != "${r_f//[$'\t\r']/}" ]; then
    mark_malformed "$f" "archive feeder tuple did not read back cleanly (status '$(sanitize "$st")')"
    continue
  fi
  ARC_ROWS+=("$tuple")
done

declare -A ARC_COUNT  # terminal-status counts (archive)
# Runs AFTER the feeder pass so `BAD` already carries M4's rejections. The BAD gate is what narrows
# the 0143-era "header counts what the table drops" mismatch: a file excluded from the table is now
# excluded from the tally too, so the remaining mismatch can only come from PLACEMENT divergence —
# board-row-dropped's territory.
for f in "${ARCFILES[@]}"; do
  [ -z "${BAD[$f]:-}" ] || continue
  st="${VALID_ST[$f]:-}"; [ -n "$st" ] || continue
  ARC_COUNT["$st"]=$(( ${ARC_COUNT[$st]:-0} + 1 ))
done

# --- digest projection (change 0069) --------------------------------------------------------
# A second, line-oriented projection of the SAME dependency-resolution/readiness pass the board
# renders from — so readiness has exactly one owner (readiness(), in lib/docket-frontmatter.sh)
# and the digest can never disagree with the board's Readiness cell. Emitted for the report;
# never persisted. Exits before the markdown emission, which stays byte-identical.
digest_readiness(){ # digest_readiness FILE ID STATUS -> machine-parseable readiness token
  local f="$1" id="$2" st="$3" tok
  # readiness() (in the shared lib) is meaningful only for a `proposed` change; `implemented`
  # carries its own presence-encoded readiness via finalize_blocked() (change 0087). Every other
  # status has none and reports `-` rather than a token that would not mean anything. Readiness
  # still has exactly one owner per status, so the digest can never disagree with the board.
  if [ "$st" = implemented ]; then
    if finalize_blocked "$f"; then printf 'finalize-blocked'; else printf '%s' '-'; fi
    return
  fi
  [ "$st" = proposed ] || { printf '%s' '-'; return; }
  tok="$(readiness "$f")"
  case "$tok" in
    waiting)
      # readiness() collapses both flavors to `waiting`; the flavor + blocking id live in the
      # resolve_deps globals, exactly as the board's readiness_cell reads them.
      case "${DEP_REASON[$id]:-}" in
        "needs your merge") printf 'waiting-on-%s-needs-merge' "${DEP_ON[$id]}" ;;
        *)                  printf 'waiting-on-%s-unbuilt' "${DEP_ON[$id]}" ;;
      esac ;;
    *) printf '%s' "$tok" ;;
  esac
}

if [ "$FORMAT" = digest ]; then
  for st in "${DOCKET_STATUSES[@]}"; do
    if docket_status_is_terminal "$st"; then n=${ARC_COUNT[$st]:-0}
    else n="$(count_of "$st")"
    fi
    [ "$n" -gt 0 ] || continue
    printf 'backlog %s %s\n' "$st" "$n"
  done
  while IFS=$'\t' read -r id f; do
    [ -n "$id" ] || continue
    digest_admits "$f" || continue
    st="${VALID_ST[$f]:-}"
    printf 'change %s %s %s %s\n' \
      "$id" "$st" "$(digest_readiness "$f" "$id" "$st")" "$(field "$f" slug)"
  done < <(
    for st in "${DOCKET_STATUSES_ACTIVE[@]}"; do
      rows_sorted "$st"
    done | sort -t$'\t' -k1,1n
  )
  # --- the `ready` line (change 0094) -------------------------------------------------------
  # The build-ready QUEUE, in the convention's deterministic selection order: priority
  # (critical > high > medium > low) -> created (ascending) -> id (ascending). An unset or
  # unrecognized priority defaults to medium; an unset, empty, or malformed created: sorts LAST
  # within its priority band, never first — an unstamped or unparseable change must never preempt
  # dated work. Membership is exactly the set digest_readiness() already reported as `build-ready`:
  # this is a SECOND call to that same pure function with identical arguments (not a reuse of the
  # earlier loop's result), so this line can never disagree with the `change` lines above — parity
  # rests on digest_readiness being pure and the DEP_* globals staying unmutated between the two
  # loops. What it adds is ORDER, which those id-ascending lines deliberately do not carry. Both
  # sort keys are STATIC frontmatter — no wall-clock read — so the renderer stays deterministic
  # and the golden byte-compare holds.
  #
  # ALWAYS EMITTED, bare when the queue is empty: absence of this line means NO QUEUE WAS PRODUCED
  # (an older render-board, or a render failure), never "nothing is ready". A consumer that cannot
  # tell those apart has merely moved the silence somewhere quieter.
  ready_ids=""
  while IFS=$'\t' read -r rid; do
    [ -n "$rid" ] || continue
    ready_ids="$ready_ids $rid"
  done < <(
    while IFS=$'\t' read -r id f; do
      [ -n "$id" ] || continue
      digest_admits "$f" || continue
      [ "$(digest_readiness "$f" "$id" proposed)" = build-ready ] || continue
      # An unset or unrecognized priority is `medium` — the convention's documented default.
      prank="$(docket_priority_rank "$(field "$f" priority)")"
      # An unset, empty, or malformed `created:` sorts LAST within its priority band, never first.
      # `field` returns empty when the key is absent, and nothing else validates `created:` — a
      # non-empty but non-date value (e.g. the docket-new-change template's unfilled
      # `created:                  # YYYY-MM-DD (UTC)` line, or any value collating below '0')
      # would otherwise pass through verbatim and sort BEFORE every real date, since `#`/`-`
      # collate below every digit. Anything that isn't a well-formed YYYY-MM-DD is unknown age, so
      # substitute the sentinel date whenever the shape check fails — not only when it's empty.
      cr="$(field "$f" created)"
      case "$cr" in
        [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) : ;;
        *) cr="9999-99-99" ;;   # unknown age -> last within the band
      esac
      printf '%s\t%s\t%s\n' "$prank" "$cr" "$id"
    done < <(rows_sorted proposed) | sort -t$'\t' -k1,1n -k2,2 -k3,3n | cut -f3
  )
  printf 'ready%s\n' "$ready_ids"
  [ "$MALFORMED" -eq 0 ] || exit 3
  exit 0
fi

printf '# Backlog\n\n'
seg=""
for st in "${DOCKET_STATUSES[@]}"; do
  if docket_status_is_terminal "$st"; then n=${ARC_COUNT[$st]:-0}
  else n="$(count_of "$st")"
  fi
  [ "$n" -gt 0 ] || continue
  seg+="$(emoji_for "$st") $n $(label_for "$st") · "
done
seg="${seg% · }"
printf '**%d changes** — %s\n' "$total" "$seg"

# --- active sections ---
label_for_title(){ case "$1" in
  in-progress) printf 'In progress';; proposed) printf 'Proposed';; blocked) printf 'Blocked';;
  deferred) printf 'Deferred';; implemented) printf 'Implemented';;
  stacked-merged) printf 'Stacked-merged';;
esac; }
readiness_cell(){ # readiness_cell FILE ID  (proposed)
  local f="$1" id="$2" tok; tok="$(readiness "$f")"
  case "$tok" in
    waiting) printf '⏳ waiting on #%s — %s' "${DEP_ON[$id]}" "${DEP_REASON[$id]}" ;;
    auto-groom-blocked) printf 'auto-groom blocked — needs you' ;;
    needs-brainstorm) printf 'needs-brainstorm' ;;
    build-ready) printf 'build-ready' ;;
  esac
}
implemented_cell(){ # implemented_cell FILE  (implemented)
  if finalize_blocked "$1"; then printf 'finalize blocked — needs you'; fi
}
stack_cell(){ # stack_cell FILE  (stacked-merged) — the parent whose branch this change merged into
  # Anchored accessor: stacked_on: is an OPTIONAL key (ADR-0057; rule table in
  # lib/docket-frontmatter.sh). `10#` at the id boundary because docket ids arrive zero-padded and
  # bash reads a leading `0` as octal. A stacked-merged change with no usable parent is a data
  # defect, but the renderer is not its reporter: the cell degrades to an em dash rather than
  # failing the render or emitting `#0000`.
  local parent; parent="$(fm_field "$1" stacked_on)"
  case "$parent" in
    ''|*[!0-9]*) printf '—' ;;
    *) printf 'merged into #%s' "$(pad "$(( 10#$parent ))")" ;;
  esac
}
pr_cell(){ local f="$1" pr num; pr="$(fm_field "$f" pr)"   # anchored: pr: is optional (ADR-0057)
  [ -n "$pr" ] || { printf ''; return; }
  num="${pr##*/}"                                   # trailing PR number, whether pr: is a full URL or bare
  case "$pr" in
    http*) printf '[#%s](%s)' "$num" "$pr" ;;       # the docket convention: pr: holds the full URL
    *) if [ -n "$REPO" ]; then printf '[#%s](https://github.com/%s/pull/%s)' "$num" "$REPO" "$num"
       else printf '#%s' "$num"; fi ;;              # bare-number fallback
  esac
}
table_header_for(){ case "$1" in
  in-progress) printf '| # | Title | Priority | Type | Spec | Branch |\n|---|-------|----------|------|------|--------|\n' ;;
  proposed)    printf '| # | Title | Priority | Type | Readiness |\n|---|-------|----------|------|-----------|\n' ;;
  blocked)     printf '| # | Title | Priority | Type | Blocked by |\n|---|-------|----------|------|------------|\n' ;;
  deferred)    printf '| # | Title | Priority | Type |\n|---|-------|----------|------|\n' ;;
  implemented) printf '| # | Title | Priority | Type | PR | Readiness |\n|---|-------|----------|------|----|-----------|\n' ;;
  # The implemented columns with the readiness column replaced by Stack: the useful fact about a
  # stacked-merged change is which parent it merged into, and readiness is spent by then.
  stacked-merged) printf '| # | Title | Priority | Type | PR | Stack |\n|---|-------|----------|------|----|-------|\n' ;;
esac; }
print_section(){ # print_section STATUS HEADER_SUFFIX
  local st="$1" suffix="$2" n; n="$(count_of "$st")"
  [ "$n" -gt 0 ] || return 0
  printf '\n## %s %s%s (%d)\n\n' "$(emoji_for "$st")" "$(label_for_title "$st")" "$suffix" "$n"
  local id f
  table_header_for "$st"
  while IFS=$'\t' read -r id f; do
    [ -n "$id" ] || continue
    local title priority ctype; title="$(field "$f" title)"; priority="$(field "$f" priority)"
    ctype="$(type_cell "$f")"
    local base; base="$(basename "$f")"
    # row_format_mapping
    # Optional keys go through the ANCHORED accessors (ADR-0057; rule table in
    # lib/docket-frontmatter.sh). blocked_by uses the VERBATIM shape because its ` #…` is data.
    case "$st" in
      in-progress)
        printf '| [%s](active/%s) | %s | `%s` | `%s` | [spec](%s) | `%s` |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(spec_link "$(fm_field "$f" spec)")" "$(fm_field "$f" branch)" ;;
      proposed)
        printf '| [%s](active/%s) | %s | `%s` | `%s` | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(readiness_cell "$f" "$id")" ;;
      blocked)
        printf '| [%s](active/%s) | %s | `%s` | `%s` | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(fm_field_verbatim "$f" blocked_by)" ;;
      deferred)
        printf '| [%s](active/%s) | %s | `%s` | `%s` |\n' "$(pad "$id")" "$base" "$title" "$priority" "$ctype" ;;
      implemented)
        printf '| [%s](active/%s) | %s | `%s` | `%s` | %s | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(pr_cell "$f")" "$(implemented_cell "$f")" ;;
      stacked-merged)
        printf '| [%s](active/%s) | %s | `%s` | `%s` | %s | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(pr_cell "$f")" "$(stack_cell "$f")" ;;
    esac
  done < <(rows_sorted "$st")
}

suffix_for(){ case "$1" in implemented) printf ' — awaiting merge' ;; esac; }
for st in "${DOCKET_STATUSES_ACTIVE[@]}"; do
  print_section "$st" "$(suffix_for "$st")"
done

# --- mermaid ---
printf '\n```mermaid\ngraph TD\n'
# Emit all active changes in ascending numeric id order; record every id referenced by an active
# change's depends_on (padded form as the key) so done nodes can be pruned to referenced-only
# below. A DONE dependency is *satisfied* in resolve_deps and skipped there, so the referenced set
# must be collected here, in the loop that already reads every depends_on value. (change 0093)
declare -A REFERENCED
while IFS=$'\t' read -r id f; do
  [ -n "$id" ] || continue
  local_deps="$(list_field "$f" depends_on)"
  if [ -n "$local_deps" ]; then
    for dep in $local_deps; do
      REFERENCED["$(pad "$dep")"]=1
      printf '  %s --> %s\n' "$(pad "$dep")" "$(pad "$id")"
    done
  else
    printf '  %s\n' "$(pad "$id")"
  fi
done < <(
  for st in "${DOCKET_STATUSES_ACTIVE[@]}"; do
    rows_sorted "$st"
  done | sort -t$'\t' -k1,1n
)
# Done nodes (ascending id): style :::done ONLY for a done id an active change depends on;
# unreferenced done ids carry no edge and are dropped. Killed omitted entirely. Emit the classDef
# line only when at least one :::done node remains (no dangling def). (change 0093)
# The BAD gate carries "excluded from every projection" into the graph as well: the M1/M2/M3 cases
# were already unreachable here (no usable id, or a status that cannot be `done`), but an M4
# rejection has a usable id AND `status: done`, so without this gate a rejected file would still be
# stylable as a node while being absent from the table, the tally, and the digest.
mapfile -t DONE_IDS < <(for f in "${ARCFILES[@]}"; do
  [ -z "${BAD[$f]:-}" ] || continue
  [ "${VALID_ST[$f]:-}" = "done" ] && { v="${VALID_ID[$f]:-}"; [ -n "$v" ] && printf '%s\n' "$v"; }; done | sort -n)
done_shown=0
for id in "${DONE_IDS[@]}"; do
  [ -n "$id" ] || continue
  [ -n "${REFERENCED["$(pad "$id")"]:-}" ] || continue
  printf '  %s:::done\n' "$(pad "$id")"; done_shown=1
done
[ "$done_shown" -eq 1 ] && printf '  classDef done fill:#d3f9d8;\n'
printf '```\n'

# --- archive ---
archive_count=0; em=""; lbl=""
for st in "${DOCKET_STATUSES_TERMINAL[@]}"; do
  n=${ARC_COUNT[$st]:-0}
  archive_count=$(( archive_count + n ))
  [ "$n" -gt 0 ] || continue
  em+="$(emoji_for "$st")"
  [ -n "$lbl" ] && lbl+=" + $st" || lbl="$st"
done
if [ "$archive_count" -gt 0 ]; then
  printf '\n<details><summary>%s Archive — %s (%d)</summary>\n\n' "$em" "$lbl" "$archive_count"
  printf '| # | Title | Merged |\n|---|-------|--------|\n'
  # Partition the date-desc / id-desc sorted rows: the verbatim window = every killed row (any age)
  # plus the first ARCHIVE_RECENT done rows in sort order (killed and recent done interleave by
  # date, unchanged shape); the collapsed set = older done rows, tallied into a per-YYYY-MM digest.
  # Killed never collapses. The status is carried in the sort tuple so the loop can partition
  # without re-reading the file. Sort keys (date field 1 desc, id field 2 num desc) are unchanged.
  # (change 0093)
  done_seen=0
  declare -A MONTH_DONE; month_order=()
  while IFS=$'\t' read -r date id st f; do
    [ -n "$id" ] || continue
    # Pure DEFENCE IN DEPTH, and unreachable by construction: every tuple in ARC_ROWS already
    # survived M4's round trip in the feeder pass above ("The check is a genuine ROUND TRIP, not a
    # re-derivation"). It stays so that a future edit which feeds this loop from somewhere else
    # fails LOUD rather than emitting a corrupt row — hence mark_malformed, not a bare `continue`.
    # Removing this line reddens no assert; M4's own coverage lives on the feeder pass.
    if ! docket_status_is_member "$st" || [ ! -e "$f" ] || [ "$f" != "${f//[$'\t\r']/}" ]; then
      mark_malformed "${f:-<unresolvable>}" "archive feeder tuple did not read back cleanly (status '$(sanitize "$st")')"
      continue
    fi
    if [ "$st" = "done" ]; then
      done_seen=$(( done_seen + 1 ))
      if [ "$done_seen" -gt "$ARCHIVE_RECENT" ]; then
        ym="${date:0:7}"
        [ -n "${MONTH_DONE[$ym]:-}" ] || month_order+=("$ym")
        MONTH_DONE["$ym"]=$(( ${MONTH_DONE[$ym]:-0} + 1 ))
        continue
      fi
    fi
    printf '| [%s](archive/%s) | %s | %s |\n' "$(pad "$id")" "$(basename "$f")" "$(field "$f" title)" "$date"
  done < <(printf '%s\n' ${ARC_ROWS[@]+"${ARC_ROWS[@]}"} | sort -t$'\t' -k1,1r -k2,2nr)
  if [ "${#month_order[@]}" -gt 0 ]; then
    printf '\n**Older done (collapsed)**\n\n'
    printf '| Month | Done |\n|-------|------|\n'
    for ym in "${month_order[@]}"; do
      printf '| [%s](archive/) | %d done |\n' "$ym" "${MONTH_DONE[$ym]}"
    done
  fi
  printf '\n</details>\n'
fi

# Render-complete, then fail loud (change 0259). stdout above is the full projection modulo the
# skipped rows; a non-zero malformed count makes the RUN non-zero so every caller gates honestly.
# BOTH emission paths reach this test over the same MALFORMED count, because every check that can
# raise it — M1-M3's upfront pass and M4's feeder pass — runs above the digest projection's own
# exit. Neither format can report success over input the other rejects.
# Callers need no new branching, and this was verified against each of them rather than assumed:
#   board-refresh.sh    — captures the render into a temp file and, on any non-zero rc, leaves
#                         BOARD.md byte-identical and propagates the code (its
#                         `render-board.sh failed (exit %d); BOARD.md left untouched` branch).
#   docket-status.sh    — backlog_pass tests `if ! out="$(… 2>&2)"`, a BARE non-zero check, so it
#                         reports "backlog digest failed; continuing without it" and emits no
#                         digest lines; digest_only_pass then fails closed on that empty capture.
# The consequence is deliberate and is the accepted cost of this change (spec assumption 1): ONE
# malformed file anywhere freezes BOARD.md and halts autonomous selection until a human fixes the
# named file. That is the trade — corruption-adjacent state stops the loop loudly rather than
# letting it run beside an unrenderable backlog. Exit 3, not 1 or 2: 2 is the established
# CLI-argument-error code and 1 is indistinguishable from an unexpected crash.
[ "$MALFORMED" -eq 0 ] || exit 3
exit 0
