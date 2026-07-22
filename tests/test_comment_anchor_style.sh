#!/usr/bin/env bash
# tests/test_comment_anchor_style.sh — cross-references in MAINTAINED SOURCE anchor on a symbol
# name or a verbatim-quoted clause, never on a line number (change 0114, ADR-0054).
#
# PARTIAL BY DESIGN. This guard enforces exactly ONE anchor form: the explicit-file form, a
# filename with a source extension followed by a colon and a line number. That is the only
# predicate measurable without false positives (26/26 true anchors at conversion time). The two
# other forms were converted by hand and are deliberately NOT guarded, because neither can be
# matched cleanly:
#   - the bare colon-number form measured ~38% false positives (bash array slices such as
#     "${PATH_STACK[@]:0:...}" and JSON fixtures such as "p10":{...} are indistinguishable
#     without parsing), and tightening it introduces a false NEGATIVE on real anchors;
#   - the prose "line N" form measured 60% false positives (test fixtures legitimately discuss
#     "line 2" of a constructed input) and would additionally have to match an en-dash range.
# Those rest on the AGENTS.md authoring rule plus review — where this repo already puts claims it
# cannot mechanically check (ADR-0031 bounds source-syntax scanning; ADR-0050 shapes this guard).
#
# SCOPE: maintained source only. docs/adrs/ is excluded because an Accepted ADR is immutable
# except its status: line, so a guard cannot demand a repair the convention forbids;
# docs/results/, docs/changes/archive/ and docs/superpowers/specs/ are immutable point-in-time
# records; docs/changes/active/ lives on the docket metadata branch and is absent from the
# integration-branch checkout this suite runs in, so there is no such path to walk.
# NO ALLOWLIST: exclusions are by walk scope, never by exception entry (ADR-0050, enumerated-floor).
#
# TRACKED-FILES-ONLY: the walk enumerates what the repository's version control tracks, not every
# byte on disk. A brand-new file carrying an anchor is invisible here until it is staged, so a
# newly added, unstaged file is NOT covered — accepted because this guard runs at the build gate
# over committed work, and an untracked file is not yet part of the repo the gate protects.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELF="$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){   printf 'ok   - %s\n' "$1"; }
nok(){  printf 'NOT OK - %s\n' "$1"; fail=1; }

# The explicit-file anchor: <name>.<source-ext> immediately followed by :<digits>.
# NOTE: no \b / \< anywhere — git grep's ERE does not support them and returns zero silently.
ANCHOR='[A-Za-z0-9_-]+\.(sh|md|yml|yaml|mdc):[0-9]+'

# --- collect the in-scope population ------------------------------------------------------------
# git ls-files, NOT git grep: git grep prefixes every hit with "path:lineno:", and that path ends
# in ".sh:"/".md:" — the exact shape ANCHOR matches. Filtering git grep output would therefore
# match the tool's own prefix on every line. Each file is scanned separately with grep -n, whose
# "lineno:content" prefix cannot collide with ANCHOR (no extension precedes the colon).
# docs/ needs no filter here: it is excluded structurally by simply never appearing in the
# pathspec below (the root globs cannot reach into a subdirectory) — not by a post-hoc filter.
mapfile -t FILES < <(
  cd "$ROOT" || exit 1
  git ls-files -- scripts tests skills agents cursor-rules ':(glob)*.md' ':(glob)*.yml'
)

# --- population floor: the walk must actually reach files ---------------------------------------
# A guard iterating an empty list is green and proves nothing. Assert a non-trivial count AND the
# presence of specific known in-scope files, so a broken pathspec or a bad ls-files invocation
# reddens instead of passing vacuously.
n_files=${#FILES[@]}
[ "$n_files" -ge 40 ] \
  && ok "walk population is non-trivial ($n_files files)" \
  || nok "walk population collapsed to $n_files files (expected >= 40) — pathspec or ls-files broke"

# Capture once into a variable rather than piping into grep -q per probe: a producer feeding an
# early-exiting consumer under pipefail can take SIGPIPE and turn into an intermittent 141
# (AGENTS.md, Shell). A here-string has no producer process to signal.
files_joined=""
[ "$n_files" -gt 0 ] && files_joined="$(printf '%s\n' "${FILES[@]}")"

for probe in scripts/board-checks.sh tests/test_board_checks.sh AGENTS.md .docket.example.yml; do
  grep -qxF "$probe" <<<"$files_joined" \
    && ok "walk includes $probe" \
    || nok "walk MISSES $probe — the in-scope surface is not fully covered"
done

# --- the check ----------------------------------------------------------------------------------
violations=""
scanned=0
# Guard the expansion: "${FILES[@]}" on a fully empty array raises "unbound variable" under
# set -u on bash 4.0-4.3, aborting instead of reporting a clean NOT OK. Skip the loop entirely
# when the population is empty; the floor check above already reddens that case.
if [ "$n_files" -gt 0 ]; then
  for f in "${FILES[@]}"; do
    [ "$(basename "$f")" = "$SELF" ] && continue   # structural self-exclusion; never an allowlist
    [ -f "$ROOT/$f" ] || continue
    scanned=$(( scanned + 1 ))
    hits="$(grep -nE "$ANCHOR" "$ROOT/$f" 2>/dev/null)"
    [ -n "$hits" ] && violations+="$(printf '%s\n' "$hits" | sed "s|^|$f:|")"$'\n'
  done
fi

[ "$scanned" -ge 40 ] \
  && ok "scanned $scanned files (guard self-excluded)" \
  || nok "scanned only $scanned files — the scan loop is not reaching the population"

if [ -z "$violations" ]; then
  ok "no line-number cross-reference anchors in maintained source"
else
  nok "line-number cross-reference anchors found — anchor on a symbol name or a quoted clause instead:"
  printf '%s' "$violations" | sed 's/^/       /'
fi

# --- positive control: prove the predicate FIRES ------------------------------------------------
# Without this, every assert above is consistent with a pattern that can never match anything.
# Mutate a throwaway copy so the drift is really present, and assert it is reported.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
printf '# see render-board.sh:76 for the id gate\n' > "$tmp/probe.sh"
grep -qE "$ANCHOR" "$tmp/probe.sh" \
  && ok "positive control: the anchor pattern reports a real explicit-file anchor" \
  || nok "positive control FAILED: the anchor pattern matches nothing — the guard is vacuous"

# Negative control: the forms this guard deliberately does NOT catch, plus the shapes
# that must never be flagged. Pins the FP-free property the partial scope rests on.
printf '%s\n' \
  'PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")' \
  '{"data":{"p10":{"number":101,"mergedAt":"2026-07-05T18:22:31Z"}}}' \
  '# the archive table renders from its own pass' \
  > "$tmp/clean.sh"
grep -qE "$ANCHOR" "$tmp/clean.sh" \
  && nok "negative control FAILED: the pattern flags a bash array slice or JSON timestamp" \
  || ok "negative control: array slices and JSON timestamps are not flagged"

exit "$fail"
