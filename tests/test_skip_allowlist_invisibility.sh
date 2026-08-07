#!/usr/bin/env bash
# tests/test_skip_allowlist_invisibility.sh — the allowlisted results tree is INVISIBLE to this
# suite (change 0190). Run: bash tests/test_skip_allowlist_invisibility.sh
#
# WHAT THIS PROVES. docket-finalize-change may skip its post-rebase suite run when the only
# post-gate delta lies under the repo's <results_dir> tree — the tree docket-implement-next
# Step 6.5 commits AFTER the build gate has already run green. That skip is safe for exactly one
# reason, and it is a property of this repo rather than of the word "docs": no executable
# component of this suite reads <results_dir> as a CONTENT SOURCE, so a commit that only adds
# files there cannot move any assertion's verdict. The distinction matters here more than most
# places — this repo's own guards grep README.md, skills/**/*.md and agents/**/*.md, none of which
# are allowlisted. This file is what stops that property rotting silently: if a suite component
# ever starts reading the allowlisted tree, the skip becomes a hole, and this guard reddens the
# build gate before the extension can ship on a stale justification.
#
# HOW IT WORKS. It scans the COMMITTED tree (git grep over a rev — committed tracked blobs, the
# tree the suite actually runs against, so an uncommitted scratch file cannot pad or fake the
# corpus) for the <results_dir> literal and classifies EVERY occurrence it finds. Classification
# runs over LOGICAL lines, with backslash continuations joined first: this repo's reads spell the
# command on one physical line and its path operands on the next, so a raw per-line predicate
# reports "no reading command here" on precisely the occurrences that are reads.
#
#   INERT    the occurrence is in a file the suite cannot execute. Derived from the committed
#            blob itself (no .sh suffix AND no shebang), never from a list of filenames.
#   COMMENT  the occurrence sits on a comment line of an executable file.
#   EXCL     the occurrence is the operand of an EXCLUSION construct — a `:!` pathspec exclusion,
#            a `!` glob negation, or a `^`-anchored pattern on an invert-match line. These are the
#            suite's own protective mechanisms, and they are separately asserted to survive.
#   ASSIGN   the occurrence is a VALUE, not a path operand: it sits right of an `=` inside its own
#            shell token (`RESULTS_DIR=<results_dir>`, `X="${X:-<results_dir>}"`).
#   NOVERB   an executable, un-negated line that carries the path but runs no reading command.
#   READ     an executable, un-negated occurrence CO-LOCATED WITH A READING COMMAND. This is the
#            hazard predicate. It is derived from what the consuming line actually does — the
#            verbs are the reading commands, matched as commands — never from a hand-written list
#            of the causes an author happened to think of, which is exactly the list the specific
#            checks already cover (repo learning: backstop-must-compute-not-reenumerate).
#
# A READ occurrence fails the guard unless it carries a curated exemption naming why that
# particular consumer cannot be moved by an addition under the tree. Exemptions are keyed on a
# file plus a VERBATIM SOURCE SLICE, never a line number (AGENTS.md), they are counted by their
# own independent counter, and a declared-but-unmatched exemption is itself a failure.
#
# HONEST LIMITS — this file claims no more than it mutates.
#   (a) The HAZARD predicate is derived from the consuming code. The BENIGN residue is
#       necessarily CURATED: a whole-tree grep can neither name a fixture path benign nor tell an
#       expected-value string from a path operand. The curation is bounded by its own exact
#       counter, so laundering a real read through a new exemption moves a number that no other
#       assertion moves.
#   (b) A co-located-verb detector cannot see an INDIRECT read — `r="$ROOT/docs/results"` on one
#       logical line and `grep x "$r"` on another. The mutation evidence for this guard proves the
#       DIRECT form reddens; the indirect form is out of reach of any single-line predicate and is
#       left to review. This is a priced limitation, not an oversight, and there is a live
#       instance of the shape: scripts/board-checks.sh defaults RESULTS_DIR_REL to the tree and
#       later reads it through that variable (its aborted-run leg A). It is safe for a reason this
#       guard cannot express — every test invocation points board-checks.sh at a fixture repo in a
#       tmpdir, never at this repo's own tree — so what protects it is fixture hygiene and review,
#       not this predicate. A change that ran board-checks.sh against the live repo would need
#       that reasoning redone.
#   (c) SELF-MEMBERSHIP. This file is excluded from the corpus scan by pathspec, because it must
#       name the very exclusion tokens it keys on and would otherwise inject its own occurrences
#       into the population it measures. The exclusion is not a blind spot: its prose says
#       <results_dir> rather than the literal, and the same classifier is run over this file's own
#       working-tree text with a zero-hazard assertion, below.
#   (d) SCOPE. The walk is every tracked path except docs/ — exclusion by walk scope, never by
#       exception entry (ADR-0050), so a new top-level directory is in scope automatically. docs/
#       holds point-in-time records (results, specs, plans, archived changes, ADRs) that the
#       convention forbids rewriting, and the metadata branch is absent from this checkout
#       entirely. That the exclusion is safe is asserted mechanically rather than claimed: no
#       tracked path under docs/ is an executable script.
#   (e) TRACKED-AND-COMMITTED ONLY. A brand-new consumer is invisible here until it is committed.
#       Accepted: this guard runs at the build gate over committed work, which is the same tree
#       finalize's skip decision is made against.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SELF_REL="tests/$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){  printf 'ok   - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }
assert(){ if eval "$2"; then ok "$1"; else nok "$1"; fi; }

# The allowlisted tree, as the convention's default spells it. This assignment is the only place
# this file writes the bare literal outside a data slice, and it carries no reading verb, so the
# self-scan below classifies it NOVERB. Every other use interpolates it.
RESULTS_DIR_REL="docs/results"

# The rev to scan. HEAD is the committed tree the suite runs against.
REV="HEAD"

# POPULATION FLOOR — measured live at build time (change 0190, base f7fb123f), never copied from a
# plan or a spec. It is a floor, not an equality: benign occurrences legitimately accrue.
FLOOR=56
# COVERAGE-GRANTING PATH, BUDGETED. Every curated exemption is a hand-granted pass, so the count of
# them is pinned exactly and independently of the floor: laundering a real content read through a
# new exemption moves this number even though the floor and the hazard count both stay put.
EXPECTED_EXEMPT=2

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
NO_EXEMPTIONS="$TMP/no-exemptions.tsv"
: > "$NO_EXEMPTIONS"

# ---------------------------------------------------------------------------
# Helpers — ONE implementation of each, shared by the live scan and the controls, so neutering a
# scan path anywhere neuters it everywhere and a control cannot stay green while the loop goes
# blind.
# ---------------------------------------------------------------------------

# Join backslash-continued shell lines into one LOGICAL line, keeping the physical line number the
# logical line starts on. This is load-bearing twice over: the reading command and its path
# operands routinely sit on different physical lines of one multi-line git/rg invocation, so a
# raw per-line predicate reports "no verb here" on exactly the occurrences that are reads, and a
# per-line token check cannot tell whether an exclusion token belongs to the probe it arms.
logical_lines(){
  awk '{
    start = FNR
    cur = $0
    while (cur ~ /\\$/) {
      sub(/\\$/, "", cur)
      if ((getline nx) <= 0) break
      cur = cur nx
    }
    printf "%s\t%s\n", start, cur
  }' "$1"
}
flatten(){ logical_lines "$1" | cut -f2-; }

# Emit `<isexec>TAB<path>TAB<lineno>TAB<logical-line>` records for every committed logical line
# carrying the literal. isexec is derived from the blob itself — a .sh suffix or a shebang first
# line — never from a list of filenames.
corpus_records(){
  local files f isexec first blob
  blob="$TMP/blob"
  files="$(git -C "$ROOT" grep -l -F -e "$RESULTS_DIR_REL" "$REV" -- ':!docs/' ":!$SELF_REL" || true)"
  [ -n "$files" ] || return 0
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    f="${f#"$REV":}"
    git -C "$ROOT" show "$REV:$f" > "$blob" 2>/dev/null || continue
    isexec=0
    case "$f" in
      *.sh) isexec=1 ;;
      *)    first="$(sed -n '1p' "$blob")"; case "$first" in '#!'*) isexec=1 ;; esac ;;
    esac
    logical_lines "$blob" | awk -F'\t' -v x="$isexec" -v p="$f" -v lit="$RESULTS_DIR_REL" \
      'index($0, lit) > 0 { ln = $1; sub(/^[^\t]*\t/, ""); printf "%s\t%s\t%s\t%s\n", x, p, ln, $0 }'
  done <<<"$files"
}

# The classifier. stdin: records as emitted above. stdout: `<class>TAB<path>TAB<lineno>TAB<text>`,
# one row per OCCURRENCE (a line carrying the literal twice yields two rows), plus one ORPHAN row
# per declared exemption that matched nothing.
classify(){
  awk -F'\t' -v lit="$RESULTS_DIR_REL" -v exf="$1" '
    # The shell token the occurrence sits in: the run of non-whitespace around it.
    function token_prefix(text, pos,   i, c, out) {
      out = ""
      for (i = pos - 1; i >= 1; i--) {
        c = substr(text, i, 1)
        if (c == " " || c == "\t") break
        out = c out
      }
      return out
    }
    function class_of(isexec, text, pos,   pre, l1, l2) {
      if (isexec != "1")            return "INERT"
      if (text ~ /^[[:space:]]*#/)  return "COMMENT"
      pre = token_prefix(text, pos)
      l1  = substr(pre, length(pre), 1)
      l2  = substr(pre, length(pre) - 1, 2)
      if (l2 == ":!")               return "EXCL"
      if (l1 == "!")                return "EXCL"
      if (l1 == "^" && text ~ inv)  return "EXCL"
      # An assignment or env-prefix VALUE, not a path operand: the occurrence sits to the right of
      # an = inside its own shell token. A genuine read spells the tree as a bare operand, so this
      # cannot hide one; what it does hide is the INDIRECT form named in HONEST LIMITS (b) above,
      # which no single-line predicate reaches in either direction.
      if (index(pre, "=") > 0)      return "ASSIGN"
      if (text ~ verb)              return "READ"
      return "NOVERB"
    }
    BEGIN {
      # A reading command in command position: preceded by something that cannot be part of a
      # name, followed by whitespace (it takes arguments). POSIX ERE only — no \b, no \<, which
      # BSD grep and git grep silently fail to honor.
      verb = "(^|[^[:alnum:]_./-])(cat|grep|egrep|fgrep|rg|awk|sed|head|tail|wc|sort|uniq|cut|nl|od|xxd|find|diff|cmp|ls-tree|ls-files|xargs)[[:space:]]"
      inv  = "(^|[^[:alnum:]_-])(-v|--invert-match)([^[:alnum:]_-]|$)"
      ne = 0
      while ((getline el < exf) > 0) {
        if (el == "") continue
        t = index(el, "\t")
        if (t == 0) continue
        ne++
        ex_path[ne]  = substr(el, 1, t - 1)
        ex_slice[ne] = substr(el, t + 1)
        ex_hit[ne]   = 0
      }
    }
    {
      isexec = $1; path = $2; ln = $3
      text = $4
      for (k = 5; k <= NF; k++) text = text "\t" $k
      start = 1
      while (1) {
        q = index(substr(text, start), lit)
        if (q == 0) break
        pos = start + q - 1
        c = class_of(isexec, text, pos)
        if (c == "READ") {
          c = "HAZARD"
          for (k = 1; k <= ne; k++) {
            if (ex_path[k] == path && index(text, ex_slice[k]) > 0) { ex_hit[k] = 1; c = "EXEMPT"; break }
          }
        }
        printf "%s\t%s\t%s\t%s\n", c, path, ln, substr(text, 1, 200)
        start = pos + length(lit)
      }
    }
    END { for (k = 1; k <= ne; k++) if (!ex_hit[k]) printf "ORPHAN\t%s\t-\t%s\n", ex_path[k], ex_slice[k] }
  '
}

count_class(){ awk -F'\t' -v c="$1" '$1 == c {n++} END {print n+0}' <<<"$2"; }

# ---------------------------------------------------------------------------
# The curated exemptions. Each is a READ-classified occurrence that is genuinely incapable of
# being moved by an addition under the allowlisted tree, with the reason stated. Keyed on a
# verbatim source slice, so drift shows up as an ORPHAN rather than as silence.
# ---------------------------------------------------------------------------
EXEMPTIONS="$TMP/exemptions.tsv"
: > "$EXEMPTIONS"
exempt(){ printf '%s\t%s\n' "$1" "$2" >> "$EXEMPTIONS"; }

# test_docket_build.sh's armed-probe companion greps the EXEMPT HISTORY (four directories at once)
# for a retired profile token and asserts the result is NON-EMPTY. It is the one deliberate read
# of the tree in the suite, and it is monotone in additions: a new file under the results tree can
# only leave a non-empty result non-empty, never flip the assertion. Its verdict does not depend
# on the results tree at all — the other three directories carry the tokens.
exempt tests/test_docket_build.sh \
  "'docs/changes/archive' '$RESULTS_DIR_REL' 'docs/superpowers' 'docs/adrs'"

# test_render_change_links.sh greps a FIXTURE change file for the rendered Results row. The
# literal here is inside the grep PATTERN — a URL in the expected markdown — and the haystack is
# the fixture, not the tree.
exempt tests/test_render_change_links.sh '| Results | ['

# ---------------------------------------------------------------------------
# 1. Scope soundness — the docs/ exclusion is asserted, not claimed.
# ---------------------------------------------------------------------------
docs_tree="$(git -C "$ROOT" ls-tree -r "$REV" -- 'docs/')"
docs_exec="$(awk '{ mode = $1; p = $0; sub(/^[^\t]*\t/, "", p); if (mode == "100755" || p ~ /\.sh$/) print p }' <<<"$docs_tree")"
assert "the skipped docs/ prefix is walked by nothing executable (mode 100755 or .sh)" \
  '[ -n "$docs_tree" ] && [ -z "$docs_exec" ] || { printf "  executable blobs under docs/: %s\n" "$docs_exec" >&2; false; }'

# ---------------------------------------------------------------------------
# 2. The live corpus, classified.
# ---------------------------------------------------------------------------
RECORDS="$(corpus_records)"
RESULT="$(classify "$EXEMPTIONS" <<<"$RECORDS")"

# ORPHAN rows are diagnostics about the exemption table, not occurrences — they are subtracted out
# of the population, so a pair of stale exemptions cannot pad the floor a shrinking corpus fails.
n_rows="$(grep -c . <<<"$RESULT" || true)"
n_orphan="$(count_class ORPHAN  "$RESULT")"
n_total="$((n_rows - n_orphan))"
n_inert="$(count_class INERT    "$RESULT")"
n_comment="$(count_class COMMENT "$RESULT")"
n_excl="$(count_class EXCL      "$RESULT")"
n_assign="$(count_class ASSIGN  "$RESULT")"
n_noverb="$(count_class NOVERB  "$RESULT")"
n_exempt="$(count_class EXEMPT  "$RESULT")"
n_hazard="$(count_class HAZARD  "$RESULT")"
printf '  corpus: %s occurrences — inert %s, comment %s, exclusion %s, assignment %s, no-verb %s, exempt %s, hazard %s\n' \
  "$n_total" "$n_inert" "$n_comment" "$n_excl" "$n_assign" "$n_noverb" "$n_exempt" "$n_hazard"

# ---------------------------------------------------------------------------
# 3. POPULATION FLOOR — the scan still reaches the corpus it was measured against.
# ---------------------------------------------------------------------------
assert "the scan reaches at least $FLOOR committed occurrences of the results-dir literal" \
  '[ "$n_total" -ge "$FLOOR" ] || { echo "  found $n_total, floor $FLOOR." >&2; echo "  A DROP means the scan went blind, not that the repo got safer: check that the rev, the pathspec and the literal still resolve, and that the consumers really were deleted rather than renamed. Only once that is confirmed does the floor move." >&2; false; }'

# Non-vacuity companions through the SAME extractor: an empty or collapsed corpus must not read as
# a clean one, and the classifier must actually be reaching executable files.
assert "the corpus contains executable-file occurrences, not only inert documentation" \
  '[ "$((n_total - n_inert))" -ge 40 ]'
assert "every corpus occurrence carries a class" \
  '[ "$((n_inert + n_comment + n_excl + n_assign + n_noverb + n_exempt + n_hazard))" = "$n_total" ]'

# ---------------------------------------------------------------------------
# 4. THE PROPERTY — no unexplained content read of the allowlisted tree.
# ---------------------------------------------------------------------------
assert "no executable suite component reads the allowlisted results tree as a content source" \
  '[ "$n_hazard" = 0 ] || { grep "^HAZARD" <<<"$RESULT" >&2; echo "  Each line above is an executable, un-negated occurrence sitting on a line that also runs a reading command." >&2; echo "  FIRST establish whether it genuinely reads the tree as a content source. If it does, finalize'"'"'s post-gate skip is no longer sound for this repo and the READ must be removed or excluded (a :! pathspec, a ! glob) — that is the fix." >&2; echo "  Only when the occurrence provably cannot be moved by an addition under the tree does it earn a curated exemption, and adding one is a budgeted event: EXPECTED_EXEMPT moves with it, in the same diff, with the reason written down." >&2; false; }'

# ---------------------------------------------------------------------------
# 5. THE COVERAGE-GRANTING PATH, with its own counter.
# ---------------------------------------------------------------------------
assert "exactly $EXPECTED_EXEMPT occurrences are covered by a curated exemption" \
  '[ "$n_exempt" = "$EXPECTED_EXEMPT" ] || { grep "^EXEMPT" <<<"$RESULT" >&2; echo "  An exemption is a hand-granted pass around the hazard predicate, so its count is pinned on its own: a real content read laundered through a new exemption moves THIS number while the hazard count and the floor both stay green." >&2; false; }'
assert "no declared exemption is stale (every one still matches a live occurrence)" \
  '[ "$n_orphan" = 0 ] || { grep "^ORPHAN" <<<"$RESULT" >&2; echo "  The exempted consumer moved or was rewritten. Re-read it and re-decide, rather than re-pointing the slice." >&2; false; }'

# ---------------------------------------------------------------------------
# 6. THE POSITIVE CLAIM — the suite's real exclusion mechanisms survive, keyed on their magic
#    token and bound to the probe that uses it. A bare-path assert is NOT enough here: both files
#    carry the bare literal elsewhere (a comment and an armed probe in one, nothing but the escape
#    in the other), so the real exclusion can be deleted with the bare path untouched.
# ---------------------------------------------------------------------------
TDB="$ROOT/tests/test_docket_build.sh"
RMF="$ROOT/tests/test_readme_finalize_docs.sh"

# Returns 0 armed, 1 token missing, 2 anchor missing (the extractor itself went blind).
tdb_exclusion_armed(){
  local probe
  probe="$(flatten "$1" | grep -F 'live_hits=' || true)"
  [ -n "$probe" ] || return 2
  case "$probe" in *"grep"*) ;; *) return 2 ;; esac
  grep -qF -e ":!$RESULTS_DIR_REL" <<<"$probe"
}
rmf_exclusion_armed(){
  local probe
  probe="$(flatten "$1" | grep -F -e '--glob' || true)"
  [ -n "$probe" ] || return 2
  case "$probe" in *"rg "*) ;; *) return 2 ;; esac
  grep -qF -e "--glob \"!$RESULTS_DIR_REL/**\"" <<<"$probe"
}

assert "test_docket_build.sh's live-tree probe still excludes the results tree by pathspec" \
  'tdb_exclusion_armed "$TDB"'
assert "test_readme_finalize_docs.sh's doc-content search still escapes the results tree by glob" \
  'rmf_exclusion_armed "$RMF"'

# Attachment: the one exempted read is still the armed-history probe it was exempted as, not some
# other consumer that inherited the exemption slice.
armed_probe="$(flatten "$TDB" | grep -F 'armed_hits=' || true)"
assert "the exempted read is still test_docket_build.sh's armed-history probe" \
  '[ -n "$armed_probe" ] && grep -qF -e "$RESULTS_DIR_REL" <<<"$armed_probe" && grep -qF -e "docs/adrs" <<<"$armed_probe"'

# ---------------------------------------------------------------------------
# 7. POSITIVE CONTROLS. Everything above is an assertion that a state is ABSENT, and an absence
#    assertion is green for free when the machinery is broken. These run the SAME classifier and
#    the SAME token extractors against throwaway copies mutated so the drift really is present,
#    and require them to REPORT it. Each mutation is confirmed to have landed before its result is
#    believed (repo learning: assert-detects-removal-not-replacement).
# ---------------------------------------------------------------------------

# 7a. The classifier assigns each class for the right reason, and CAN emit HAZARD.
synth(){ printf '%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4"; }
{
  synth 1 tests/test_synthetic_probe.sh 1 "  hits=\"\$(grep -rlF token \"\$ROOT/$RESULTS_DIR_REL\")\""
  synth 1 tests/test_synthetic_probe.sh 2 "  git grep -n token -- ':!$RESULTS_DIR_REL'"
  synth 1 tests/test_synthetic_probe.sh 3 "  ! rg -q --glob \"!$RESULTS_DIR_REL/**\" token \"\$ROOT\""
  synth 1 tests/test_synthetic_probe.sh 4 "  run_thing 'RESULTS_DIR=$RESULTS_DIR_REL' --flag"
  synth 1 tests/test_synthetic_probe.sh 5 "  # grep the $RESULTS_DIR_REL tree"
  synth 0 README.md                     6 "run grep over $RESULTS_DIR_REL to see"
  synth 1 tests/test_synthetic_probe.sh 7 "  mkdir -p \"\$work/$RESULTS_DIR_REL\""
} > "$TMP/synth.tsv"
SYNTH="$(classify "$NO_EXEMPTIONS" < "$TMP/synth.tsv")"
synth_class(){ awk -F'\t' -v l="$1" '$3 == l {print $1}' <<<"$SYNTH"; }
assert "control: a direct read of the tree classifies HAZARD" '[ "$(synth_class 1)" = HAZARD ]'
assert "control: a :! pathspec operand classifies EXCL"        '[ "$(synth_class 2)" = EXCL ]'
assert "control: a ! glob operand classifies EXCL"             '[ "$(synth_class 3)" = EXCL ]'
assert "control: an env-prefix value classifies ASSIGN"        '[ "$(synth_class 4)" = ASSIGN ]'
assert "control: a comment line classifies COMMENT"            '[ "$(synth_class 5)" = COMMENT ]'
assert "control: a non-executable file classifies INERT"       '[ "$(synth_class 6)" = INERT ]'
assert "control: a bare operand with no reading command classifies NOVERB" \
  '[ "$(synth_class 7)" = NOVERB ]'

# 7b. Deleting the pathspec exclusion from a throwaway copy of test_docket_build.sh is REPORTED.
MUT_TDB="$TMP/mutated_docket_build.sh"
awk -v tok=":!$RESULTS_DIR_REL" '{ while ((p = index($0, tok)) > 0) $0 = substr($0, 1, p - 1) substr($0, p + length(tok)); print }' \
  "$TDB" > "$MUT_TDB"
tdb_before="$(grep -c -F -e ":!$RESULTS_DIR_REL" "$TDB" || true)"
tdb_after="$(grep -c -F -e ":!$RESULTS_DIR_REL" "$MUT_TDB" || true)"
assert "mutation landed: the pathspec exclusion token is present before and gone after" \
  '[ "$tdb_before" -ge 1 ] && [ "$tdb_after" = 0 ]'
assert "control: the bare literal SURVIVES that mutation, so a bare-path assert would stay green" \
  '[ "$(grep -c -F -e "$RESULTS_DIR_REL" "$MUT_TDB" || true)" -ge 2 ]'
tdb_exclusion_armed "$MUT_TDB"; tdb_mut_rc=$?
assert "control: the exclusion check REPORTS the deleted pathspec exclusion" '[ "$tdb_mut_rc" = 1 ]'

# 7c. The same, for the rg glob escape.
MUT_RMF="$TMP/mutated_readme_finalize.sh"
awk -v tok="--glob \"!$RESULTS_DIR_REL/**\"" '{ while ((p = index($0, tok)) > 0) $0 = substr($0, 1, p - 1) substr($0, p + length(tok)); print }' \
  "$RMF" > "$MUT_RMF"
rmf_before="$(grep -c -F -e "--glob \"!$RESULTS_DIR_REL/**\"" "$RMF" || true)"
rmf_after="$(grep -c -F -e "--glob \"!$RESULTS_DIR_REL/**\"" "$MUT_RMF" || true)"
assert "mutation landed: the glob escape is present before and gone after" \
  '[ "$rmf_before" -ge 1 ] && [ "$rmf_after" = 0 ]'
rmf_exclusion_armed "$MUT_RMF"; rmf_mut_rc=$?
assert "control: the glob-escape check REPORTS the deleted escape" '[ "$rmf_mut_rc" = 1 ]'

# 7d. Vacuity control — if the anchor the extractor keys on disappears, that is reported as a
# BROKEN EXTRACTOR (2), never as an armed guard. An absence assert whose extractor silently
# returns nothing is green for the wrong reason.
BLIND_TDB="$TMP/blind_docket_build.sh"
sed 's/live_hits=/renamed_hits=/g' "$TDB" > "$BLIND_TDB"
assert "mutation landed: the probe anchor is gone from the blinded copy" \
  '[ "$(grep -c -F -e "live_hits=" "$BLIND_TDB" || true)" = 0 ]'
tdb_exclusion_armed "$BLIND_TDB"; tdb_blind_rc=$?
assert "control: a renamed probe anchor is reported as a broken extractor, not as armed" \
  '[ "$tdb_blind_rc" = 2 ]'

# ---------------------------------------------------------------------------
# 8. SELF-MEMBERSHIP. This file is out of the corpus scan by pathspec, so its own text is scanned
#    here instead, through the same classifier. Its policy prose says <results_dir>; the literal
#    appears only in the assignment above and inside data slices, none of them next to a reading
#    verb. Nothing here may read the allowlisted tree either.
# ---------------------------------------------------------------------------
SELF_RECORDS="$TMP/self.tsv"
: > "$SELF_RECORDS"
self_ln=0
while IFS= read -r sline; do
  self_ln=$((self_ln + 1))
  case "$sline" in
    *"$RESULTS_DIR_REL"*) printf '%s\t%s\t%s\t%s\n' 1 "$SELF_REL" "$self_ln" "$sline" >> "$SELF_RECORDS" ;;
  esac
done < "${BASH_SOURCE[0]}"
SELF_RESULT="$(classify "$NO_EXEMPTIONS" < "$SELF_RECORDS")"
self_total="$(grep -c . <<<"$SELF_RESULT" || true)"
self_hazard="$(count_class HAZARD "$SELF_RESULT")"
assert "self-scan is armed (this file does carry the literal it excludes itself for)" \
  '[ "$self_total" -ge 1 ]'
assert "this guard does not itself read the allowlisted results tree" \
  '[ "$self_hazard" = 0 ] || { grep "^HAZARD" <<<"$SELF_RESULT" >&2; false; }'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
