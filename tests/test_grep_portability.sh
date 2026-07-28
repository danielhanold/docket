#!/usr/bin/env bash
# tests/test_grep_portability.sh — no maintained source file may carry an ERE repetition bound
# above 255 (change 0130).
#
# WHY: BSD grep rejects a repetition bound above 255 with "maximum repetition exceeds 255". A test
# written with an over-threshold bound therefore errors out before it examines anything, and on a machine whose
# PATH grep is GNU grep or ugrep it passes anyway — the suite runs green while the bug is real.
# This guard is a STATIC SOURCE scan, deliberately not a runtime probe of the local grep's
# behavior: on Linux /usr/bin/grep is GNU grep and accepts the bound, so a behavioral assertion
# would be a platform-dependent false failure. The property wanted is source portability, which is
# true or false independent of the machine running the suite.
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
# this change's own file and spec — which quote the historical over-threshold bound verbatim — at close-out. The guard must not
# demand a repair it cannot legally have. Every OTHER tracked surface is in scope automatically,
# including any new top-level directory added later.
# NO ALLOWLIST: exclusions are by walk scope, never by exception entry (ADR-0050).
#
# SELF-MEMBERSHIP: this file is NOT self-excluded. It is asserted to be in the scanned population
# and clean, which is why every >255 literal it needs is assembled at runtime rather than written.
#
# TRACKED-FILES-ONLY: a brand-new file is invisible here until it is staged. Accepted — the guard
# runs at the build gate over committed work — and the self-membership assert makes the gap loud.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELF_REL="tests/$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){   printf 'ok   - %s\n' "$1"; }
nok(){  printf 'NOT OK - %s\n' "$1"; fail=1; }

# The maximum repetition bound BSD grep accepts. A bound EQUAL to this is legal; above it is not.
MAX_BOUND=255

# A brace interval literal: {m}, {m,} or {m,n}. \{ is a literal brace in ERE on GNU and BSD alike.
# This also matches the BRE form \{m,n\} — the backslash merely precedes the matched text.
# NOTE: no \b / \< anywhere — git grep's and BSD grep's ERE do not support them and return zero
# silently. This pattern carries no bound of its own above MAX_BOUND, so the guard stays clean
# under its own scan.
INTERVAL='\{[0-9]+(,[0-9]*)?\}'

# ONE scan implementation, used by the main loop AND both controls below — never a second,
# independently-written grep call. Routing everything through this function means neutering the
# scan path anywhere neuters it everywhere, so a control cannot stay green while the loop goes
# blind. -I skips binaries; -o emits one interval per line; -n prefixes the source line number.
scan_file(){ grep -InoE "$INTERVAL" "$1" 2>/dev/null; }

# Report every bound above MAX_BOUND in "lineno:interval" input. Reads scan_file output on stdin.
# Pure text + arithmetic; no regex expresses "greater than 255" readably, and attempting one would
# embed brace intervals into this guard.
offenders(){
  local line lineno interval nums n
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    lineno="${line%%:*}"
    interval="${line#*:}"
    nums="${interval#\{}"; nums="${nums%\}}"     # {0,N} -> 0,N
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
mapfile -t FILES < <(
  cd "$ROOT" || exit 1
  git ls-files | grep -v '^docs/'
)

# --- population floor: the walk must actually reach files ----------------------------------------
# A guard iterating an empty list is green and proves nothing.
n_files=${#FILES[@]}
[ "$n_files" -ge 100 ] \
  && ok "walk population is non-trivial ($n_files files)" \
  || nok "walk population collapsed to $n_files files (expected >= 100) — ls-files or the filter broke"

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
scanned=0
if [ "$n_files" -gt 0 ]; then
  for f in "${FILES[@]}"; do
    [ -f "$ROOT/$f" ] || continue
    scanned=$(( scanned + 1 ))
    hits="$(scan_file "$ROOT/$f")"
    [ -n "$hits" ] || continue
    bad="$(offenders <<<"$hits")"
    [ -n "$bad" ] && violations+="$(sed "s|^|$f:|" <<<"$bad")"$'\n'
  done
fi

[ "$scanned" -ge 100 ] \
  && ok "scanned $scanned files" \
  || nok "scanned only $scanned files — the scan loop is not reaching the population"

if [ -z "$violations" ]; then
  ok "no ERE repetition bound above $MAX_BOUND in maintained source"
else
  nok "ERE repetition bound above $MAX_BOUND found — BSD grep rejects these; rewrite the pattern:"
  printf '%s' "$violations" | sed 's/^/       /'
fi

# --- controls: prove the predicate FIRES and where its boundary sits ------------------------------
# Without these, every assert above is consistent with a pattern that can never match anything.
# Routed through the SAME scan_file + offenders the loop uses, so a control can only stay green if
# the exact path the loop runs is still capable of firing.
#
# The over-threshold fixtures are ASSEMBLED AT RUNTIME. Writing an over-threshold bound literally here would make
# this guard fail its own scan — the self-membership assert above is what makes that a real
# constraint rather than a stylistic one.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

over="$(( MAX_BOUND + 345 ))"       # 600 — the real-world bound, never written literally
edge="$(( MAX_BOUND + 1 ))"         # 256 — one past the boundary
printf 'x.%s%s,%s%sy\n' '{' 0 "$over" '}' > "$tmp/over.txt"
printf 'x.%s%s,%s%sy\n' '{' 0 "$edge" '}' > "$tmp/edge.txt"

pos="$(offenders <<<"$(scan_file "$tmp/over.txt")")"
[ -n "$pos" ] \
  && ok "positive control: a bound of $over is reported" \
  || nok "positive control FAILED: a bound of $over is not reported — the guard is vacuous"

edge_hit="$(offenders <<<"$(scan_file "$tmp/edge.txt")")"
[ -n "$edge_hit" ] \
  && ok "boundary control: a bound of $edge (one past $MAX_BOUND) is reported" \
  || nok "boundary control FAILED: $edge slipped through — the threshold is off by at least one"

# Negative control. This 255 bound is written LITERALLY on purpose: it is legal under this guard's
# own rule, so it doubles as a demonstration that the boundary is inclusive.
printf 'x.{0,255}y\n' > "$tmp/clean.txt"
printf 'a{b}c and ${VAR} and awk "{print}"\n' >> "$tmp/clean.txt"
neg="$(offenders <<<"$(scan_file "$tmp/clean.txt")")"
[ -n "$neg" ] \
  && nok "negative control FAILED: a legal bound of $MAX_BOUND or a non-regex brace was flagged" \
  || ok "negative control: a bound of exactly $MAX_BOUND and non-regex braces are not flagged"

exit "$fail"
