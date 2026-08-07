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
# command on one physical line and its path operands on the next, so the words that give an
# occurrence its role — the command that opens the line, the option or redirection in front of the
# path — are routinely on a different physical line from the path itself.
#
# THE PREDICATE IS INVERTED, and that inversion is the design. The obvious guard asks "does this
# line run a reading command?" and answers it from a list of verbs — but the list an author can
# write is never the list of read forms the shell offers: `< redirection`, `source`, `.`, an
# interpreter (`bash`, `python3`, `jq`), `ls`, `[ -f ]` each open a file and none of them looks
# like `grep`. Every form missing from such a list passes SILENTLY, which is exactly the failure
# backstop-must-compute-not-reenumerate names — a hand-written list of the causes an author
# happened to think of, wearing the word invariant. So this file asks the complementary question,
# "is this occurrence PROVABLY not consumed?", and treats every occurrence that is not as a
# HAZARD. The enumeration still exists; it now sits on the BENIGN side, where a missing entry
# fails CLOSED (a loud false positive) instead of open, and where every entry is budgeted.
#
#   INERT    the occurrence is in a file the suite cannot execute. Derived from the committed
#            blob itself (no .sh suffix AND no shebang), never from a list of filenames.
#   COMMENT  the occurrence sits on a comment line of an executable file.
#   EXCL     the occurrence is the operand of an EXCLUSION construct — a `:!` pathspec exclusion,
#            a `!` glob negation, or a `^`-anchored pattern on an invert-match line. These are the
#            suite's own protective mechanisms, and they are separately asserted to survive.
#   ASSIGN   the occurrence is a VALUE, not a path operand: it sits right of an `=` inside its own
#            shell token (`RESULTS_DIR=<results_dir>`, `X="${X:-<results_dir>}"`).
#   DATA     the logical line is a data line, not a command — `key: <results_dir>/…` with no
#            command separator or substitution anywhere on it. It invokes nothing, so it opens
#            nothing.
#   WRITE    the occurrence is CREATED, not read: the line's FIRST word is `mkdir`/`touch`/`rmdir`
#            (again with no separator on the line), or the occurrence is the target of `>`/`>>`.
#   OPTVAL   the occurrence is the value of a space-separated LONG option (`--results-dir <path>`)
#            — a parameter handed to another program, not an operand this line opens.
#   EXPECT   the occurrence is the right-hand side of a shell comparison word (`=`, `==`, `!=`):
#            an expected value being compared against, not a path being opened.
#   CURATED  a hand-declared occurrence that opens nothing but whose shape none of the above
#            expresses — a path inside an assert DESCRIPTION, inside a regex pattern, or in a
#            `for … in` word list.
#   EXEMPT   a hand-declared genuine read that cannot be MOVED by an addition under the tree.
#   HAZARD   THE DEFAULT. Every other executable, un-negated occurrence fails the guard.
#
# Both hand-declared classes are keyed on a file plus a VERBATIM SOURCE SLICE, never a line number
# (AGENTS.md), and a declared-but-unmatched entry is itself a failure. Every route past the HAZARD
# default carries its own independent counter: the four shape classes in aggregate, OPTVAL again
# on its own, CURATED, and EXEMPT.
#
# HONEST LIMITS — this file claims no more than it mutates.
#   (a) HAZARD is the DEFAULT, so it is the BENIGN side that is curated — and therefore the side
#       that is counted. Six routes lead past the default: four shape classes and two hand-
#       declared tables. The two tables are pinned individually; the four shape classes are pinned
#       in AGGREGATE, so any ADDITION moves a number no other assertion moves, but a pure SWAP
#       between two shape classes does not. The one swap worth pricing is priced: OPTVAL is the
#       widest of the six — `--file`, `--include` and their kin are also long options and the
#       receiving command READS what they name — so the space-separated spellings of those are
#       denied the pass by name, and OPTVAL carries a second counter of its own. A swap among
#       DATA, WRITE and EXPECT is left unpriced deliberately: none of the three can name a path a
#       command opens (a data line invokes nothing, a WRITE line creates, an EXPECT operand is a
#       comparison right-hand side), so the swap has nowhere to hide a read.
#   (b) A single-logical-line predicate cannot see an INDIRECT read — `r="$ROOT/docs/results"` on
#       one logical line and `grep x "$r"` on another. The mutation evidence for this guard proves
#       the DIRECT forms redden; the indirect form is out of reach of any single-line predicate
#       and is left to review. This is a priced limitation, not an oversight, and it has live
#       instances: the `for p in … <results_dir>/; do` word list in tests/test_codex_runbook.sh is
#       declared CURATED for precisely this reason (its loop body greps the runbook for the path
#       as a citation and never opens the tree, but no line-scoped rule can see that), and
#       scripts/board-checks.sh defaults RESULTS_DIR_REL to the tree and
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
# this file writes the bare literal, and it sits right of an `=` inside its own token, so the
# self-scan below classifies it ASSIGN. Every other use interpolates it.
RESULTS_DIR_REL="docs/results"

# The rev to scan. HEAD is the committed tree the suite runs against.
REV="HEAD"

# POPULATION FLOOR — measured live at build time (change 0190, base f7fb123f), never copied from a
# plan or a spec. It is a floor, not an equality: benign occurrences legitimately accrue.
FLOOR=56
# COVERAGE-GRANTING PATHS, EACH BUDGETED. Every route past the HAZARD default is a pass, so each
# count is pinned exactly and independently of the floor: laundering a real content read through
# any of them moves a number even though the floor and the hazard count both stay green.
#   BENIGN   the four SHAPE classes in aggregate (DATA + WRITE + OPTVAL + EXPECT).
#   OPTVAL   pinned a second time on its own — it is the widest shape pass, so a swap that keeps
#            the aggregate steady must still redden something.
#   CURATED  hand-declared occurrences that open nothing but whose shape no rule expresses.
#   EXEMPT   hand-declared genuine reads that an addition under the tree cannot move.
# All four measured live at build time (change 0190, base f7fb123f), never copied from a plan.
EXPECTED_BENIGN=15
EXPECTED_OPTVAL=2
EXPECTED_CURATED=3
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
# per declared table entry that matched nothing.
#
# HAZARD is the fall-through. Every rule below is a BENIGN rule — a reason this occurrence cannot
# be a content read — so a shape nobody anticipated lands on HAZARD and is reported, which is the
# opposite of what a verb enumeration does with a read form nobody anticipated.
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
    # The whitespace-delimited word immediately LEFT of the token holding the occurrence — the
    # option, the redirection or the comparison operator that gives the occurrence its role.
    function prev_word(text, pos,   i, c, out) {
      i = pos - 1
      while (i >= 1) { c = substr(text, i, 1); if (c == " " || c == "\t") break; i-- }
      while (i >= 1) { c = substr(text, i, 1); if (c != " " && c != "\t") break; i-- }
      out = ""
      while (i >= 1) { c = substr(text, i, 1); if (c == " " || c == "\t") break; out = c out; i-- }
      return out
    }
    function class_of(isexec, text, pos,   pre, l1, l2, pw) {
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
      pw = prev_word(text, pos)
      # A data line invokes nothing. The no-separator clause is what makes that true rather than
      # merely likely: `key: <lit>` with a pipe, a `;`, a redirect or a $( ) on it could still run
      # something, so it is denied the pass and falls through to HAZARD.
      if (text ~ datal && text !~ sepr)          return "DATA"
      if (pw == ">" || pw == ">>")               return "WRITE"
      if (text ~ writel && text !~ sepr)         return "WRITE"
      # A long option value. SHORT options are deliberately excluded — `-f`, `-T`, `[ -f` and
      # `stat -f%z` all name a path the command then opens, and there is no way to tell those from
      # a benign short flag. Long options that are themselves file-consuming are denied by name.
      if (pw ~ /^--[[:alpha:]]/ && pw !~ optread) return "OPTVAL"
      if (pw == "=" || pw == "==" || pw == "!=") return "EXPECT"
      return "HAZARD"
    }
    BEGIN {
      inv    = "(^|[^[:alnum:]_-])(-v|--invert-match)([^[:alnum:]_-]|$)"
      # POSIX ERE only throughout — no \b, no \<, which BSD grep and git grep silently ignore.
      datal  = "^[[:space:]]*[[:alpha:]_][[:alnum:]_-]*:[[:space:]]"
      writel = "^[[:space:]]*(mkdir|touch|rmdir)([[:space:]]|$)"
      sepr   = "[|;<>`]|\\$\\(|&&"
      optread = "^--(file|include|exclude|exclude-from|from-file|pathspec-from-file|glob)$"
      ne = 0
      while ((getline el < exf) > 0) {
        if (el == "") continue
        if (split(el, a, "\t") < 3) continue
        ne++
        ex_class[ne] = a[1]
        ex_path[ne]  = a[2]
        ex_slice[ne] = a[3]
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
        if (c == "HAZARD") {
          for (k = 1; k <= ne; k++) {
            if (ex_path[k] == path && index(text, ex_slice[k]) > 0) { ex_hit[k] = 1; c = ex_class[k]; break }
          }
        }
        printf "%s\t%s\t%s\t%s\n", c, path, ln, substr(text, 1, 200)
        start = pos + length(lit)
      }
    }
    END { for (k = 1; k <= ne; k++) if (!ex_hit[k]) printf "ORPHAN\t%s\t-\t%s %s\n", ex_class[k], ex_path[k], ex_slice[k] }
  '
}

count_class(){ awk -F'\t' -v c="$1" '$1 == c {n++} END {print n+0}' <<<"$2"; }

# ---------------------------------------------------------------------------
# The two hand-declared tables, both keyed on a verbatim source slice so drift shows up as an
# ORPHAN rather than as silence, and both budgeted by their own counter.
#
#   exempt — a genuine READ of the tree that is incapable of being MOVED by an addition under it.
#   benign — an occurrence that opens nothing at all, but whose shape no mechanical rule expresses.
#
# They are kept apart on purpose: mixing them would let a real read be admitted under the softer
# claim, and would blur the one number whose job is to price a read.
# ---------------------------------------------------------------------------
EXEMPTIONS="$TMP/exemptions.tsv"
: > "$EXEMPTIONS"
exempt(){ printf 'EXEMPT\t%s\t%s\n'  "$1" "$2" >> "$EXEMPTIONS"; }
benign(){ printf 'CURATED\t%s\t%s\n' "$1" "$2" >> "$EXEMPTIONS"; }

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

# The three occurrences the HAZARD default catches that open nothing. Each is a shape a
# line-scoped rule cannot express without opening a hole wider than the case it closes.
#
# An assert DESCRIPTION string. The assertion body beside it is `! has_finding "$ar8_default" …`,
# which reads a captured variable; the path here is prose naming board-checks.sh's default.
benign tests/test_board_checks.sh "(default $RESULTS_DIR_REL)"
# A `for … in` WORD LIST. The loop body greps $RUNBOOK for each path as a cited string — the
# runbook is the haystack, the path is the needle, and the tree is never opened. This is the
# indirect shape of HONEST LIMITS (b), which is why it is declared rather than ruled.
benign tests/test_codex_runbook.sh "scripts/runners/codex.sh $RESULTS_DIR_REL/; do"
# A regex PATTERN emitted by `echo` for a later match against .docket.example.yml. The literal is
# inside the needle, not the haystack.
benign tests/test_docket_example_yml.sh "^results_dir:[[:space:]]*$RESULTS_DIR_REL"

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

# ORPHAN rows are diagnostics about the declared tables, not occurrences — they are subtracted out
# of the population, so a pair of stale entries cannot pad the floor a shrinking corpus fails.
n_rows="$(grep -c . <<<"$RESULT" || true)"
n_orphan="$(count_class ORPHAN  "$RESULT")"
n_total="$((n_rows - n_orphan))"
n_inert="$(count_class INERT    "$RESULT")"
n_comment="$(count_class COMMENT "$RESULT")"
n_excl="$(count_class EXCL      "$RESULT")"
n_assign="$(count_class ASSIGN  "$RESULT")"
n_data="$(count_class DATA      "$RESULT")"
n_write="$(count_class WRITE    "$RESULT")"
n_optval="$(count_class OPTVAL  "$RESULT")"
n_expect="$(count_class EXPECT  "$RESULT")"
n_curated="$(count_class CURATED "$RESULT")"
n_exempt="$(count_class EXEMPT  "$RESULT")"
n_hazard="$(count_class HAZARD  "$RESULT")"
n_benign="$((n_data + n_write + n_optval + n_expect))"
printf '  corpus: %s occurrences — inert %s, comment %s, exclusion %s, assignment %s | benign shapes %s (data %s, write %s, optval %s, expect %s), curated %s, exempt %s | hazard %s\n' \
  "$n_total" "$n_inert" "$n_comment" "$n_excl" "$n_assign" \
  "$n_benign" "$n_data" "$n_write" "$n_optval" "$n_expect" "$n_curated" "$n_exempt" "$n_hazard"

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
  '[ "$((n_inert + n_comment + n_excl + n_assign + n_benign + n_curated + n_exempt + n_hazard))" = "$n_total" ]'

# ---------------------------------------------------------------------------
# 4. THE PROPERTY — no unexplained content read of the allowlisted tree.
# ---------------------------------------------------------------------------
assert "no executable suite component reads the allowlisted results tree as a content source" \
  '[ "$n_hazard" = 0 ] || { grep "^HAZARD" <<<"$RESULT" >&2; echo "  Each line above is an executable, un-negated occurrence that no benign rule could prove harmless. HAZARD is the DEFAULT here, so this is also what an unanticipated read form looks like." >&2; echo "  FIRST establish what command, if any, opens that path — including the forms that do not look like reads: < redirection, source, ., an interpreter, ls, [ -f ]. If the tree is genuinely read as a content source, finalize'"'"'s post-gate skip is no longer sound for this repo and the read must be removed or excluded (a :! pathspec, a ! glob) — that is the fix." >&2; echo "  If instead the occurrence opens nothing, it belongs in the benign table (EXPECTED_CURATED); if it opens the tree but an addition under it cannot change the verdict, it belongs in the exemption table (EXPECTED_EXEMPT). Either way the budget moves in the same diff, with the reason written down." >&2; false; }'

# ---------------------------------------------------------------------------
# 5. THE COVERAGE-GRANTING PATHS, each with its own counter. Every one of these is a route past
#    the HAZARD default, so every one is priced. Pinning only the hazard count would let a real
#    read be laundered into any of them for free (repo learning: guard-remedy-must-not-teach-the-
#    evasion) — and pinning only the aggregate would let one shape absorb another, which is why
#    OPTVAL, the widest of them, is pinned a second time on its own.
# ---------------------------------------------------------------------------
assert "exactly $EXPECTED_BENIGN occurrences take a benign SHAPE pass (data/write/optval/expect)" \
  '[ "$n_benign" = "$EXPECTED_BENIGN" ] || { grep -E "^(DATA|WRITE|OPTVAL|EXPECT)" <<<"$RESULT" >&2; echo "  found $n_benign (data $n_data, write $n_write, optval $n_optval, expect $n_expect), budget $EXPECTED_BENIGN." >&2; echo "  A shape class is a pass around the HAZARD default, so its total is pinned. FIRST confirm the new occurrence is genuinely not a content read: open the line and name the command that would open that path, then check that the rule which claimed it really rules it out — a DATA line that gained a \$( ), a WRITE line that gained a pipe, an OPTVAL whose option is one the receiving program reads from." >&2; echo "  Only once it provably opens nothing does the budget move, in the same diff." >&2; false; }'
assert "exactly $EXPECTED_OPTVAL occurrences take the long-option pass" \
  '[ "$n_optval" = "$EXPECTED_OPTVAL" ] || { grep "^OPTVAL" <<<"$RESULT" >&2; echo "  found $n_optval, budget $EXPECTED_OPTVAL." >&2; echo "  OPTVAL is the widest pass in this file: an option value LOOKS like a parameter handed onward, but --file, --include and their kin name a path the receiving command then READS. FIRST identify which program receives this option and whether it opens the path; if it does, the option belongs in the classifier'"'"'s optread denial list, not in this budget." >&2; false; }'
assert "exactly $EXPECTED_CURATED occurrences are hand-declared benign" \
  '[ "$n_curated" = "$EXPECTED_CURATED" ] || { grep "^CURATED" <<<"$RESULT" >&2; echo "  found $n_curated, budget $EXPECTED_CURATED." >&2; echo "  A benign declaration asserts the occurrence opens NOTHING — not that its read is harmless. FIRST re-read the line and confirm no command receives that path as a file operand. A read that cannot be moved by an addition is a different claim and belongs in the exemption table instead." >&2; false; }'
assert "exactly $EXPECTED_EXEMPT occurrences are covered by a curated exemption" \
  '[ "$n_exempt" = "$EXPECTED_EXEMPT" ] || { grep "^EXEMPT" <<<"$RESULT" >&2; echo "  found $n_exempt, budget $EXPECTED_EXEMPT." >&2; echo "  An exemption admits a REAL read and claims an addition under the tree cannot change its verdict. FIRST prove that claim — name the assertion the read feeds and show why adding a file under the tree leaves the verdict identical. Only then does this budget move, and it moves alone: the hazard count and the floor both stay green either way." >&2; false; }'
assert "no declared table entry is stale (every one still matches a live occurrence)" \
  '[ "$n_orphan" = 0 ] || { grep "^ORPHAN" <<<"$RESULT" >&2; echo "  The declared consumer moved or was rewritten. Re-read it and re-decide, rather than re-pointing the slice." >&2; false; }'

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
  synth 1 tests/test_synthetic_probe.sh 8 "    results: $RESULTS_DIR_REL/2026-06-01-x-results.md"
  synth 1 tests/test_synthetic_probe.sh 9 "  run_thing --results-dir $RESULTS_DIR_REL --flag"
  synth 1 tests/test_synthetic_probe.sh 10 "  assert x '[ \"\$RESULTS_DIR\" = $RESULTS_DIR_REL ]'"
  # THE STRUCTURAL CONTROL. A bare operand carrying no recognised reading command used to be the
  # unbudgeted free pass this file shipped with; under the inverted predicate it is a HAZARD.
  synth 1 tests/test_synthetic_probe.sh 11 "  some_helper \"\$ROOT/$RESULTS_DIR_REL\""
  # A file-consuming LONG option must not buy the OPTVAL pass.
  synth 1 tests/test_synthetic_probe.sh 12 "  grep --file $RESULTS_DIR_REL/pats \"\$F\""
} > "$TMP/synth.tsv"
SYNTH="$(classify "$NO_EXEMPTIONS" < "$TMP/synth.tsv")"
synth_class(){ awk -F'\t' -v l="$1" '$3 == l {print $1}' <<<"$SYNTH"; }
assert "control: a direct read of the tree classifies HAZARD" '[ "$(synth_class 1)" = HAZARD ]'
assert "control: a :! pathspec operand classifies EXCL"        '[ "$(synth_class 2)" = EXCL ]'
assert "control: a ! glob operand classifies EXCL"             '[ "$(synth_class 3)" = EXCL ]'
assert "control: an env-prefix value classifies ASSIGN"        '[ "$(synth_class 4)" = ASSIGN ]'
assert "control: a comment line classifies COMMENT"            '[ "$(synth_class 5)" = COMMENT ]'
assert "control: a non-executable file classifies INERT"       '[ "$(synth_class 6)" = INERT ]'
assert "control: a mkdir operand classifies WRITE"             '[ "$(synth_class 7)" = WRITE ]'
assert "control: a frontmatter data line classifies DATA"      '[ "$(synth_class 8)" = DATA ]'
assert "control: a long-option value classifies OPTVAL"        '[ "$(synth_class 9)" = OPTVAL ]'
assert "control: a comparison right-hand side classifies EXPECT" '[ "$(synth_class 10)" = EXPECT ]'
assert "control: a bare operand with no recognised reading command classifies HAZARD, not a free pass" \
  '[ "$(synth_class 11)" = HAZARD ]'
assert "control: a file-consuming long option is DENIED the OPTVAL pass and classifies HAZARD" \
  '[ "$(synth_class 12)" = HAZARD ]'

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

# 7e. THE READ FORMS THE OLD VERB LIST MISSED. Each of these opens a file and none of them looks
# like `grep`; under a verb enumeration every one classified as an unbudgeted free pass. They are
# introduced into a THROWAWAY COPY of a real corpus file and run through the SAME logical_lines
# extractor and the SAME classifier the live scan uses, so a control cannot stay green while the
# real path goes blind. The mutation is proved to have landed by occurrence count first.
NEW_FORMS="$TMP/new-read-forms.tsv"
: > "$NEW_FORMS"
form(){ printf '%s\t%s\n' "$1" "$2" >> "$NEW_FORMS"; }
form '< redirection feeding a read loop' "done < \"\$ROOT/$RESULTS_DIR_REL/x\""
form 'mapfile from a < redirection'      "mapfile -t X < \"\$ROOT/$RESULTS_DIR_REL/x\""
form 'read from a < redirection'         "read -r a < \"\$ROOT/$RESULTS_DIR_REL/x\""
form 'source'                            "source \"\$ROOT/$RESULTS_DIR_REL/lib.sh\""
form 'dot-source'                        ". \"\$ROOT/$RESULTS_DIR_REL/lib.sh\""
form 'a bash interpreter'                "bash \"\$ROOT/$RESULTS_DIR_REL/x.sh\""
form 'an sh interpreter'                 "sh \"\$ROOT/$RESULTS_DIR_REL/x.sh\""
form 'a python interpreter'              "python3 \"\$ROOT/$RESULTS_DIR_REL/x.py\""
form 'a perl interpreter'                "perl -ne 'print' \"\$ROOT/$RESULTS_DIR_REL/x\""
form 'a jq filter'                       "jq -r .a \"\$ROOT/$RESULTS_DIR_REL/x.json\""
form 'a yq filter'                       "yq '.a' \"\$ROOT/$RESULTS_DIR_REL/x.yml\""
form 'ls'                                "ls \"\$ROOT/$RESULTS_DIR_REL\""
form 'stat'                              "stat -f%z \"\$ROOT/$RESULTS_DIR_REL/x\""
form 'an existence test'                 "[ -f \"\$ROOT/$RESULTS_DIR_REL/x\" ] || fail"
form 'a short-option file argument'      "grep -f \"\$ROOT/$RESULTS_DIR_REL/pats\" \"\$F\""
n_forms="$(grep -c . "$NEW_FORMS" || true)"

MUT_READS="$TMP/mutated_read_forms.sh"
cp "$TDB" "$MUT_READS"
printf '\n' >> "$MUT_READS"   # the appended forms must start on their own physical line
reads_before="$(grep -c -F -e "$RESULTS_DIR_REL" "$MUT_READS" || true)"
base_lines="$(awk 'END {print NR}' "$MUT_READS")"
cut -f2 "$NEW_FORMS" >> "$MUT_READS"
reads_after="$(grep -c -F -e "$RESULTS_DIR_REL" "$MUT_READS" || true)"
assert "mutation landed: the throwaway copy gained one line per new read form" \
  '[ "$((reads_after - reads_before))" = "$n_forms" ]'

MUT_RECORDS="$TMP/mutated_read_forms.tsv"
logical_lines "$MUT_READS" | awk -F'\t' -v p=tests/mutated_read_forms.sh -v lit="$RESULTS_DIR_REL" \
  'index($0, lit) > 0 { ln = $1; sub(/^[^\t]*\t/, ""); printf "1\t%s\t%s\t%s\n", p, ln, $0 }' > "$MUT_RECORDS"
MUT_RESULT="$(classify "$NO_EXEMPTIONS" < "$MUT_RECORDS")"
form_i=0
while IFS=$'\t' read -r form_label _; do
  [ -n "$form_label" ] || continue
  form_i=$((form_i + 1))
  form_class="$(awk -F'\t' -v l="$((base_lines + form_i))" '$3 == l {print $1}' <<<"$MUT_RESULT")"
  assert "control: $form_label reads the tree and is REPORTED as HAZARD" \
    '[ "$form_class" = HAZARD ]'
done < "$NEW_FORMS"
assert "control: every declared new read form was actually classified" \
  '[ "$form_i" = "$n_forms" ]'

# ---------------------------------------------------------------------------
# 8. SELF-MEMBERSHIP. This file is out of the corpus scan by pathspec, so its own text is scanned
#    here instead, through the same classifier. Its policy prose says <results_dir>; the literal
#    appears only in the assignment above and on comment lines, so under the inverted predicate
#    every occurrence must land on ASSIGN or COMMENT. Nothing here may read the tree either.
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
