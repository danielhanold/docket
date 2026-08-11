#!/usr/bin/env bash
# tests/test_grep_portability.sh — no maintained source file may carry an ERE repetition bound
# above 255 (change 0130).
#
# WHY: BSD grep rejects a repetition bound above 255 with "maximum repetition exceeds 255". A test
# written with an over-threshold bound therefore errors out before it examines anything, and on a
# machine whose PATH grep is GNU grep or ugrep it passes anyway — the suite runs green while the
# bug is real. This guard is a STATIC SOURCE scan, deliberately not a runtime probe of the local
# grep's behavior: on Linux /usr/bin/grep is GNU grep and accepts the bound, so a behavioral
# assertion would be a platform-dependent false failure. The property wanted is source
# portability, which is true or false independent of the machine running the suite.
#
# SCOPE: every tracked path (git ls-files, anchored on the repo root resolved from BASH_SOURCE),
# minus the docs/ prefix. NO extension filter — an extension list is the same re-enumeration on a
# different axis (.mdc, .py, an extensionless hook) and buys nothing, because no false positive is
# possible at a >255 threshold. Binary safety comes from grep -I.
#
# docs/ IS THE ONE EXCLUSION, AND IT IS A DECISION: archived change files, historical plans,
# published terminal records and design specs legitimately quote defective patterns verbatim, and
# they are immutable point-in-time records the convention forbids rewriting (AGENTS.md, "Comments
# and cross-references"). Four such occurrences exist today, and terminal_publish: true will add
# this change's own file and spec — which quote the historical over-threshold bound verbatim —
# at close-out. The guard must not demand a repair it cannot legally have. Every OTHER tracked
# surface is in scope automatically, including any new top-level directory added later.
# NO ALLOWLIST: exclusions are by walk scope, never by exception entry (ADR-0050).
#
# SELF-MEMBERSHIP: this file is NOT self-excluded. It is asserted to be in the scanned population
# and clean, which is why every >255 literal it needs is assembled at runtime rather than written.
#
# TRACKED-FILES-ONLY: a brand-new file is invisible here until it is staged. Accepted — the guard
# runs at the build gate over committed work — and the self-membership assert makes the gap loud.
#
# TWO CLASSES, ONE WALK (change 0246): this file scans for two independent portability defects over
# the same population. (1) an ERE repetition bound above 255. (2) the TWO-BACKSLASH SOURCE SPELLING
# of the word-boundary form — two source backslashes before b, < or >. BSD grep's and git-grep's
# ERE do not support \b/\</\>; they return zero matches SILENTLY, so such a guard goes blind rather
# than red, which is the same fail-open shape as the interval class. Change 0246 converted the only
# three two-backslash sites (all in tests/test_docket_example_yml.sh), so the tree is provably clean
# for this SPELLING as the class lands.
#
# WHAT THIS CLASS DOES **NOT** COVER — READ BEFORE TRUSTING IT. The scanner is a pure byte pattern
# with no shell-quote awareness, so what it gates is a SPELLING, not the defect class. A
# double-quoted SINGLE-backslash "\b" delivers the very same two bytes to grep — b is not a
# recognised double-quote escape in bash, so the one- and two-backslash source spellings are
# byte-identical at the grep boundary — and this class matches only the two-backslash one. The
# one-backslash spelling is therefore UNGUARDED, and the residual gap is
# real, not theoretical: tests/test_docket_metadata_branch.sh ('"--yes\b|\b-y\b"') and
# tests/test_cursor_dispatch_rule.sh ('"\b(Task|Agent)\b"') both carry it today and pass this class
# clean. The single-backslash form is left unmatched DELIBERATELY on this branch, not because it is
# safe: a substantial tracked population uses it, much of it as deliberate comment-blessed
# PATH-grep idiom (see tests/test_docket_build.sh, which blesses them explicitly), and separating
# the blessed ones from the defective ones is its own change. HOW MANY is deliberately NOT written
# here or in any assert message: a hand-written figure is exactly the staleness this suite exists
# to catch, and a prior one (~26) was already wrong by the time it was read. The count is COMPUTED
# from the same walk (see ONE_BACKSLASH below) and reported informationally, so it is
# self-maintaining. The negative control below ASSERTS this limitation rather than merely
# describing it, so nobody can mistake a green run for full coverage of the defect.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELF_REL="tests/$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# The maximum repetition bound BSD grep accepts. A bound EQUAL to this is legal; above it is not.
MAX_BOUND=255
# The walk must reach at least this many tracked files, or it has silently collapsed.
MIN_FILES=100

# A brace interval literal in EITHER regex flavor: ERE {m}, {m,} or {m,n}, and BRE \{m,n\} (a
# backslash immediately before the brace on either or both sides). Against a BRE-quoted
# over-threshold bound this pattern consumes the leading backslash, the brace, the digits, and
# the trailing backslash-brace, so it DOES match the BRE form — earlier wording claiming the plain
# ERE pattern matched BRE text without the optional backslashes was wrong; the backslashes are
# matched explicitly here, not incidentally.
# NOTE: no \b / \< anywhere — git grep's and BSD grep's ERE do not support them and return zero
# silently. This pattern carries no bound of its own above MAX_BOUND, so the guard stays clean
# under its own scan.
INTERVAL='\\?\{[0-9]+(,[0-9]*)?\\?\}'

# ONE scan implementation, used by the main loop AND both controls below — never a second,
# independently-written grep call. Routing everything through this function means neutering the
# scan path anywhere neuters it everywhere, so a control cannot stay green while the loop goes
# blind. -I skips binaries; -o emits one interval per line; -n prefixes the source line number.
scan_file(){ grep -InoE "$INTERVAL" "$1" 2>/dev/null; }

# The TWO-BACKSLASH source spelling of the word-boundary form. Written with an assembled backslash
# literal for the same self-membership reason as the fixtures below: this pattern is itself
# scanned. Two source backslashes, then b, < or >. It is byte-level only: a double-quoted
# single-backslash "\b" reaches grep identically and is NOT matched here (see the header).
WORD_BOUNDARY="$(printf '%s%s[b<>]' '\\' '\\')"

# Human-readable spellings of the two forms, for assert messages. ASSEMBLED for the same reason the
# pattern is: a message that spelled the banned form literally would make this guard flag itself.
BS='\'
WB_ESC="$(printf '%s%sb / %s%s< / %s%s>' "$BS" "$BS" "$BS" "$BS" "$BS" "$BS")"
WB_ONE="$(printf '%sb' "$BS")"

# ONE scan implementation for this class, used by the main loop AND both controls — same discipline
# as scan_file: routing every caller through one function means neutering the scan anywhere neuters
# it everywhere, so a control cannot stay green while the loop goes blind.
scan_word_boundary(){ grep -InoE "$WORD_BOUNDARY" "$1" 2>/dev/null; }

# The ONE-backslash source spelling — a single source backslash before b, < or >. This is NOT a
# gate: it is the population the header calls unguarded, measured instead of guessed. Assembled at
# runtime for the same self-membership reason as WORD_BOUNDARY. Note the two-backslash spelling is
# a superset match of this one (the second backslash + b matches), so the sites unique to the
# one-backslash form are the SET DIFFERENCE of the two per-line hit sets, computed below.
ONE_BACKSLASH="$(printf '%s[b<>]' '\\')"
scan_one_backslash(){ grep -InoE "$ONE_BACKSLASH" "$1" 2>/dev/null; }

# Report every bound above MAX_BOUND in "lineno:interval" input. Reads scan_file output on stdin.
# Pure text + arithmetic; no regex expresses "greater than 255" readably, and attempting one would
# embed brace intervals into this guard.
offenders(){
  local line lineno interval nums n
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    lineno="${line%%:*}"
    interval="${line#*:}"
    interval="${interval//\\/}"                  # drop the BRE flavor backslashes so the digits below are bare
    nums="${interval#\{}"; nums="${nums%\}}"     # peel the braces, leaving just the comma-joined numbers
    while [ -n "$nums" ]; do
      n="${nums%%,*}"
      if [ -n "$n" ] && [ "$n" -gt "$MAX_BOUND" ] 2>/dev/null; then
        printf '%s:%s\n' "$lineno" "$interval"
        break
      fi
      [ "$nums" = "${nums#*,}" ] && break
      nums="${nums#*,}"
    done
  done
}

# --- informational, NON-GATING: which grep did this run actually exercise? -----------------------
# A portability guard that silently tests a different tool than the one it targets is the trap this
# change exists to close. This line does not gate anything — the scan is static — but it lets a
# reader see the toolchain. Captured into a variable first, then read with a here-string: a
# producer feeding an early-exiting consumer under pipefail can take SIGPIPE and become an
# intermittent 141 (AGENTS.md, Shell).
grep_path="$(command -v grep 2>/dev/null || true)"
grep_ver_all="$(grep --version 2>/dev/null || true)"
grep_ver="$(sed -n '1p' <<<"$grep_ver_all")"
printf '#    - resolved grep: %s (%s)\n' "${grep_path:-unknown}" "${grep_ver:-version unavailable}"

# --- collect the in-scope population -------------------------------------------------------------
# Computed from tracked files, never a hand-enumerated directory list: a hand list leaves any new
# top-level source directory silently unguarded, which is precisely what ADR-0050 rules out.
# NUL-delimited (git ls-files -z + mapfile -d '') so a tracked path with a special character (a
# quote, non-ASCII bytes, an embedded newline) survives the walk instead of being silently quoted
# by git and then vanishing at the [ -f ] test below. The docs/ exclusion is applied as a bash
# pattern match on each NUL-delimited entry, not via `grep -z` (not portable to BSD grep).
mapfile -d '' -t ALL_FILES < <(cd "$ROOT" && git ls-files -z)
FILES=()
for f in "${ALL_FILES[@]}"; do
  case "$f" in
    docs/*) continue ;;
  esac
  FILES+=("$f")
done

# --- population floor: the walk must actually reach files ----------------------------------------
# A guard iterating an empty list is green and proves nothing.
n_files=${#FILES[@]}
[ "$n_files" -ge "$MIN_FILES" ] \
  && ok "walk population is non-trivial ($n_files files)" \
  || nok "walk population collapsed to $n_files files (expected >= $MIN_FILES) — ls-files or the filter broke"

files_joined=""
[ "$n_files" -gt 0 ] && files_joined="$(printf '%s\n' "${FILES[@]}")"

# Named probes across distinct surfaces, including two NON-.sh files: the walk has no extension
# filter, and these are what prove it.
for probe in tests/test_finalize_disposition.sh scripts/board-checks.sh AGENTS.md \
             .docket.example.yml skills/docket-adr/SKILL.md agents/docket-adr.md \
             cursor-rules/dispatch/docket-adr.md migrate-to-docket.sh; do
  grep -qxF "$probe" <<<"$files_joined" \
    && ok "walk includes $probe" \
    || nok "walk MISSES $probe — the in-scope surface is not fully covered"
done

# The docs/ exclusion must actually exclude.
grep -qE '^docs/' <<<"$files_joined" \
  && nok "walk leaked a docs/ path — the exclusion is not applied" \
  || ok "walk excludes docs/"

# --- self-membership: this guard is scanned like everything else ---------------------------------
# Stays RED until this file is git-added. That is the tracked-only edge made loud, not a bug.
grep -qxF "$SELF_REL" <<<"$files_joined" \
  && ok "guard is itself in the scanned population ($SELF_REL)" \
  || nok "guard is NOT in the scanned population — git add $SELF_REL (tracked-files-only walk)"

# --- the check -----------------------------------------------------------------------------------
violations=""
wb_violations=""
# Informational populations for the computed single-backslash count (never a gate).
ob_sites=""   # file:lineno carrying a one-backslash \b / \< / \>
wb_sites=""   # file:lineno carrying the two-backslash spelling (a subset of the above)
scanned=0
skipped=""
if [ "$n_files" -gt 0 ]; then
  for f in "${FILES[@]}"; do
    if [ ! -f "$ROOT/$f" ]; then
      skipped+="$f"$'\n'
      continue
    fi
    scanned=$(( scanned + 1 ))
    wb_hits="$(scan_word_boundary "$ROOT/$f")"
    if [ -n "$wb_hits" ]; then
      while IFS= read -r l; do
        wb_violations+="$f:$l"$'\n'
        wb_sites+="$f:${l%%:*}"$'\n'
      done <<<"$wb_hits"
    fi
    ob_hits="$(scan_one_backslash "$ROOT/$f")"
    if [ -n "$ob_hits" ]; then
      while IFS= read -r l; do
        ob_sites+="$f:${l%%:*}"$'\n'
      done <<<"$ob_hits"
    fi
    hits="$(scan_file "$ROOT/$f")"
    [ -n "$hits" ] || continue
    bad="$(offenders <<<"$hits")"
    if [ -n "$bad" ]; then
      while IFS= read -r l; do
        violations+="$f:$l"$'\n'
      done <<<"$bad"
    fi
  done
fi

# Equality, not a floor: a floor of MIN_FILES would still pass on a sparse checkout that silently
# dropped a large fraction of the population (fail-open). Every file the walk found must actually
# be scanned, or the guard names exactly which tracked paths were skipped.
if [ "$scanned" -eq "$n_files" ]; then
  ok "scanned $scanned files"
else
  nok "scanned $scanned of $n_files tracked files — the scan loop is not reaching the population; skipped:"
  printf '%s' "$skipped" | sed 's/^/       /'
fi

if [ -z "$violations" ]; then
  ok "no ERE repetition bound above $MAX_BOUND in maintained source"
else
  nok "ERE repetition bound above $MAX_BOUND found — BSD grep rejects these; rewrite the pattern:"
  printf '%s' "$violations" | sed 's/^/       /'
fi

if [ -z "$wb_violations" ]; then
  ok "no two-backslash $WB_ESC spelling in maintained source (single-backslash spelling NOT covered)"
else
  nok "two-backslash word-boundary spelling found — BSD grep and git-grep ERE return zero for these SILENTLY; use an explicit [^[:alnum:]_] class instead:"
  printf '%s' "$wb_violations" | sed 's/^/       /'
fi

# --- COMPUTED, NON-GATING: how large is the unguarded single-backslash population? ---------------
# The header explains that the one-backslash spelling is deliberately left unmatched, and the size
# of that population is the justification for not widening the class here. That figure used to be
# hand-written (in two places, both stale). It is derived instead: every file:line carrying a
# one-backslash form, minus the file:line sites that carry the two-backslash spelling (already
# gated above, and a superset match of the one-backslash pattern). Sorted -u first because comm
# requires sorted input and a single source line can yield several -o matches.
_ob_sorted="$(printf '%s' "$ob_sites" | sort -u)"
_wb_sorted="$(printf '%s' "$wb_sites" | sort -u)"
one_bs_sites="$(comm -23 <(printf '%s\n' "$_ob_sorted") <(printf '%s\n' "$_wb_sorted") | grep -c ':' || true)"
printf '#    - unguarded single-backslash %s sites in scope: %s (computed, not gating)\n' \
  "$WB_ONE" "$one_bs_sites"

# --- controls: prove the predicate FIRES and where its boundary sits ------------------------------
# Without these, every assert above is consistent with a pattern that can never match anything.
# Routed through the SAME scan_file + offenders the loop uses, so a control can only stay green if
# the exact path the loop runs is still capable of firing.
#
# The over-threshold fixtures are ASSEMBLED AT RUNTIME. Writing an over-threshold bound literally
# here would make this guard fail its own scan — the self-membership assert above is what makes
# that a real constraint rather than a stylistic one.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

over="$(( MAX_BOUND + 345 ))"       # 600 — the real-world bound, never written literally
edge="$(( MAX_BOUND + 1 ))"         # 256 — one past the boundary
printf 'x.%s%s,%s%sy\n' '{' 0 "$over" '}' > "$tmp/over.txt"
printf 'x.%s%s,%s%sy\n' '{' 0 "$edge" '}' > "$tmp/edge.txt"
printf 'x.%s%s,%s%sy\n' '\{' 0 "$over" '\}' > "$tmp/bre.txt"

pos="$(offenders <<<"$(scan_file "$tmp/over.txt")")"
[ -n "$pos" ] \
  && ok "positive control: an ERE bound of $over is reported" \
  || nok "positive control FAILED: an ERE bound of $over is not reported — the guard is vacuous"

edge_hit="$(offenders <<<"$(scan_file "$tmp/edge.txt")")"
[ -n "$edge_hit" ] \
  && ok "boundary control: a bound of $edge (one past $MAX_BOUND) is reported" \
  || nok "boundary control FAILED: $edge slipped through — the threshold is off by at least one"

bre_hit="$(offenders <<<"$(scan_file "$tmp/bre.txt")")"
[ -n "$bre_hit" ] \
  && ok "BRE positive control: a bound of \\{0,$over\\} is reported" \
  || nok "BRE positive control FAILED: \\{0,$over\\} is not reported — the BRE interval form is not covered"

# Negative control. This 255 bound is written LITERALLY on purpose: it is legal under this guard's
# own rule, so it doubles as a demonstration that the boundary is inclusive. The BRE line does the
# same for the BRE form: \{0,1\} is a real, legal interval — 21 tracked files use this construct
# today in sed/grep invocations — and must not be flagged either.
printf 'x.{0,255}y\n' > "$tmp/clean.txt"
printf 'a{b}c and ${VAR} and awk "{print}"\n' >> "$tmp/clean.txt"
printf 'sed -E "s/a\\{0,1\\}/X/"\n' >> "$tmp/clean.txt"
neg="$(offenders <<<"$(scan_file "$tmp/clean.txt")")"
[ -n "$neg" ] \
  && nok "negative control FAILED: a legal bound (ERE $MAX_BOUND, BRE 0,1) or a non-regex brace was flagged" \
  || ok "negative control: legal ERE/BRE bounds and non-regex braces are not flagged"

# --- word-boundary class controls (change 0246) --------------------------------------------------
wb_over="$tmp/wb-bad.txt"
wb_clean="$tmp/wb-ok.txt"
# ASSEMBLED AT RUNTIME, exactly like the MAX_BOUND fixtures above and for the same reason: this
# guard is in its own scanned population (see the self-membership assert), so writing the banned
# form literally here would make the guard fail its own scan. That applies to the ASSERT MESSAGES
# too, not just the fixtures — hence WB_ESC/WB_ONE beside the pattern above, which spell the two
# forms for a human reader without ever putting the banned byte pair in this file's source.
printf 'grep -qE "%s%sb$k%s%sb" "$f"\n' "$BS" "$BS" "$BS" "$BS" > "$wb_over"
printf 'grep -qE "%s%s<$k" "$f"\n'      "$BS" "$BS"             >> "$wb_over"
# Neighbours that must NOT be flagged: the blessed single-quoted form, a literal backslash-b in a
# printf, and a doubled backslash not followed by b/</>.
printf "grep -qE '%sb\$k%sb' \"\$f\"\n" "$BS" "$BS" > "$wb_clean"
printf 'printf "%s%sn"\n'               "$BS" "$BS" >> "$wb_clean"
# ...and the KNOWN GAP, asserted rather than merely commented: a DOUBLE-quoted single-backslash
# "\b" delivers bytes identical to the two-backslash spelling at the grep boundary, yet this class
# does not match it. Kept in the clean fixture so the limitation is pinned — if someone later
# widens the pattern to catch the real defect class, this control reddens and forces them to
# revisit the tracked single-backslash sites deliberately instead of by accident — the computed
# count printed above says how many there are, so no figure is restated here.
printf 'grep -qE "%sb(Task|Agent)%sb" "$f"\n' "$BS" "$BS" >> "$wb_clean"

wb_pos="$(scan_word_boundary "$wb_over")"
[ -n "$wb_pos" ] \
  && ok "word-boundary positive control: the two-backslash $WB_ESC spelling is reported" \
  || nok "word-boundary positive control FAILED: the two-backslash spelling is not reported — the class is vacuous"

wb_neg="$(scan_word_boundary "$wb_clean")"
[ -z "$wb_neg" ] \
  && ok "word-boundary negative control: single- AND double-quoted $WB_ONE (the known, unguarded gap) and a plain backslash are not flagged" \
  || nok "word-boundary negative control FAILED: a single-backslash form was flagged — this class gates a SPELLING, not the defect class; widening it reddens $one_bs_sites tracked sites and belongs in its own change"

exit "$fail"
