#!/usr/bin/env bash
# scripts/render-board.sh — deterministic, idempotent renderer for docket's `inline` board
# surface (change 0022). Reads the change files (active/ + archive/) and emits BOARD.md to STDOUT
# byte-for-byte per docket-status's *Board -> Structure*. No git writes (the caller redirects +
# commits), offline (no gh, no network). Same change files => identical bytes.
#
# Usage: render-board.sh --changes-dir DIR [--repo OWNER/REPO]
#   --repo builds pr: hyperlinks; defaults to deriving OWNER/REPO from the origin remote of
#   --changes-dir. Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
CHANGES_DIR=""
REPO=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --repo) REPO="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-board: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || { printf 'render-board: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ] || { printf 'render-board: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

# Derive OWNER/REPO from the origin remote when --repo is unset (best-effort, offline).
if [ -z "$REPO" ]; then
  url="$("$GIT" -C "$CHANGES_DIR" remote get-url origin 2>/dev/null || true)"
  if [ -n "$url" ]; then
    REPO="${url%.git}"; REPO="${REPO#git@github.com:}"; REPO="${REPO#https://github.com/}"
  fi
fi

pad(){ printf '%04d' "$1"; }                  # bare id -> 4-digit
emoji_for(){ case "$1" in
  in-progress) printf '🟢';; proposed) printf '🟡';; blocked) printf '🔴';;
  deferred) printf '⚪';; implemented) printf '🔵';; done) printf '✅';; killed) printf '🗑️';;
esac; }
label_for(){ case "$1" in in-progress) printf 'in progress';; *) printf '%s' "$1";; esac; }
spec_link(){ printf '../%s' "${1#docs/}"; }   # docs/superpowers/specs/X -> ../superpowers/specs/X

resolve_deps "$CHANGES_DIR"

# Collect active files by status (ascending id), and archive rows.
declare -A SECTION         # status -> newline-separated "id\tfile"
mapfile -t AFILES < <(find "$CHANGES_DIR/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${AFILES[@]}"; do
  id="$(int_field "$f" id)"; [ -n "$id" ] || continue
  st="$(field "$f" status)"
  SECTION["$st"]+="$id"$'\t'"$f"$'\n'
done

# rows_sorted STATUS -> emits "id<TAB>file" lines for that status, ascending id
rows_sorted(){ printf '%s' "${SECTION[$1]:-}" | sed '/^$/d' | sort -t$'\t' -k1,1n; }
count_of(){ rows_sorted "$1" | grep -c . ; }

# --- count line ---
total=${#AFILES[@]}
mapfile -t ARCFILES < <(find "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
total=$(( total + ${#ARCFILES[@]} ))

declare -A ARC_COUNT  # done/killed counts (archive)
for f in "${ARCFILES[@]}"; do st="$(field "$f" status)"; ARC_COUNT["$st"]=$(( ${ARC_COUNT[$st]:-0} + 1 )); done

printf '# Backlog\n\n'
seg=""
for st in in-progress proposed blocked deferred implemented done killed; do
  case "$st" in
    done|killed) n=${ARC_COUNT[$st]:-0} ;;
    *) n="$(count_of "$st")" ;;
  esac
  [ "$n" -gt 0 ] || continue
  seg+="$(emoji_for "$st") $n $(label_for "$st") · "
done
seg="${seg% · }"
printf '**%d changes** — %s\n' "$total" "$seg"

# --- active sections ---
label_for_title(){ case "$1" in
  in-progress) printf 'In progress';; proposed) printf 'Proposed';; blocked) printf 'Blocked';;
  deferred) printf 'Deferred';; implemented) printf 'Implemented';;
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
pr_cell(){ local f="$1" pr num; pr="$(field "$f" pr)"
  [ -n "$pr" ] || { printf ''; return; }
  num="${pr##*/}"                                   # trailing PR number, whether pr: is a full URL or bare
  case "$pr" in
    http*) printf '[#%s](%s)' "$num" "$pr" ;;       # the docket convention: pr: holds the full URL
    *) if [ -n "$REPO" ]; then printf '[#%s](https://github.com/%s/pull/%s)' "$num" "$REPO" "$num"
       else printf '#%s' "$num"; fi ;;              # bare-number fallback
  esac
}
print_section(){ # print_section STATUS HEADER_SUFFIX
  local st="$1" suffix="$2" n; n="$(count_of "$st")"
  [ "$n" -gt 0 ] || return 0
  printf '\n## %s %s%s (%d)\n\n' "$(emoji_for "$st")" "$(label_for_title "$st")" "$suffix" "$n"
  local id f
  case "$st" in
    in-progress) printf '| # | Title | Priority | Spec | Branch |\n|---|-------|----------|------|--------|\n' ;;
    proposed)    printf '| # | Title | Priority | Readiness |\n|---|-------|----------|-----------|\n' ;;
    blocked)     printf '| # | Title | Priority | Blocked by |\n|---|-------|----------|------------|\n' ;;
    deferred)    printf '| # | Title | Priority |\n|---|-------|----------|\n' ;;
    implemented) printf '| # | Title | Priority | PR |\n|---|-------|----------|----|\n' ;;
  esac
  while IFS=$'\t' read -r id f; do
    [ -n "$id" ] || continue
    local title priority; title="$(field "$f" title)"; priority="$(field "$f" priority)"
    local base; base="$(basename "$f")"
    case "$st" in
      in-progress)
        printf '| [%s](active/%s) | %s | `%s` | [spec](%s) | `%s` |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$(spec_link "$(field "$f" spec)")" "$(field "$f" branch)" ;;
      proposed)
        printf '| [%s](active/%s) | %s | `%s` | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$(readiness_cell "$f" "$id")" ;;
      blocked)
        printf '| [%s](active/%s) | %s | `%s` | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$(field "$f" blocked_by)" ;;
      deferred)
        printf '| [%s](active/%s) | %s | `%s` |\n' "$(pad "$id")" "$base" "$title" "$priority" ;;
      implemented)
        printf '| [%s](active/%s) | %s | `%s` | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$(pr_cell "$f")" ;;
    esac
  done < <(rows_sorted "$st")
}

print_section in-progress ""
print_section proposed ""
print_section blocked ""
print_section deferred ""
print_section implemented " — awaiting merge"

# --- mermaid ---
printf '\n```mermaid\ngraph TD\n'
# emit all active changes in ascending numeric id order
while IFS=$'\t' read -r id f; do
  [ -n "$id" ] || continue
  local_deps="$(list_field "$f" depends_on)"
  if [ -n "$local_deps" ]; then
    for dep in $local_deps; do printf '  %s --> %s\n' "$(pad "$dep")" "$(pad "$id")"; done
  else
    printf '  %s\n' "$(pad "$id")"
  fi
done < <(
  for st in in-progress proposed blocked deferred implemented; do
    rows_sorted "$st"
  done | sort -t$'\t' -k1,1n
)
# done nodes (ascending id); killed omitted
mapfile -t DONE_IDS < <(for f in "${ARCFILES[@]}"; do
  [ "$(field "$f" status)" = "done" ] && { v="$(int_field "$f" id)"; [ -n "$v" ] && printf '%s\n' "$v"; }; done | sort -n)
for id in "${DONE_IDS[@]}"; do [ -n "$id" ] && printf '  %s:::done\n' "$(pad "$id")"; done
printf '  classDef done fill:#d3f9d8;\n```\n'

# --- archive ---
ndone=${ARC_COUNT[done]:-0}; nkilled=${ARC_COUNT[killed]:-0}
if [ $(( ndone + nkilled )) -gt 0 ]; then
  em=""; lbl=""
  [ "$ndone" -gt 0 ] && { em+="✅"; lbl="done"; }
  [ "$nkilled" -gt 0 ] && { em+="🗑️"; [ -n "$lbl" ] && lbl="$lbl + killed" || lbl="killed"; }
  printf '\n<details><summary>%s Archive — %s (%d)</summary>\n\n' "$em" "$lbl" "$(( ndone + nkilled ))"
  printf '| # | Title | Merged |\n|---|-------|--------|\n'
  # sort archive rows: date desc, then id desc. Key = "<date>\t<id>\t<file>".
  while IFS=$'\t' read -r date id f; do
    [ -n "$id" ] || continue
    printf '| [%s](archive/%s) | %s | %s |\n' "$(pad "$id")" "$(basename "$f")" "$(field "$f" title)" "$date"
  done < <(
    for f in "${ARCFILES[@]}"; do
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"
      printf '%s\t%s\t%s\n' "$d" "$id" "$f"
    done | sort -t$'\t' -k1,1r -k2,2nr
  )
  printf '\n</details>\n'
fi
