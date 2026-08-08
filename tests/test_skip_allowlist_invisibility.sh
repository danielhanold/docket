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
# TWO LIMBS HOLD THAT CLAIM UP, because either one alone leaves a whole shape of consumer invisible.
# LIMB 1 (sections 1-8) is keyed on the results-dir LITERAL: it finds every component that NAMES the
# tree and proves none of them opens it. LIMB 2 (section 9) is keyed on the WALK: it finds every
# component that enumerates a tree at all — ls-files, ls-tree, find — and proves each one is scoped
# away from the results tree. The second exists because the first cannot see a whole-tree walker
# that drops the tree at a broader prefix and so never spells the literal: tests/
# test_grep_portability.sh enumerates every tracked path and excludes the tree only through
# `case "$f" in docs/*)`, and tests/test_comment_anchor_style.sh excludes it only by never listing
# docs/ in its pathspec. Narrow the first one component — to docs/changes/ — or widen the second to
# name docs/, and the results tree becomes a live content source for a suite scan while limb 1 goes
# on reporting hazard 0. Both mutations are run below, and both redden limb 2.
#
# HOW LIMB 1 WORKS. It scans the COMMITTED tree (git grep over a rev — committed tracked blobs, the
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
#   ASSIGN   the occurrence is a VALUE, not a path operand: its own shell token opens with a
#            genuine ASSIGNMENT shape, `NAME=` — directly (`RESULTS_DIR=<results_dir>`) or through
#            a default expansion on the right of one (`X="${X:-<results_dir>}"`). NOT any token
#            carrying an `=`: `--include=<results_dir>/*.md` is a read and classifies HAZARD.
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
# (AGENTS.md), and a declared-but-unmatched entry is itself a failure. The slice is matched against
# the OCCURRENCE, not the logical line: it must SPAN the matched position, so a read appended to an
# already-declared line cannot inherit the pass — which matters most for the read a line-scoped
# occurrence count cannot see at all, one spelled through a shell variable. Every route past the
# HAZARD default carries its own independent counter: the four shape classes in aggregate, OPTVAL
# again on its own, CURATED, and EXEMPT — and the two tables carry a SECOND, independent counter
# each, the number of entries DECLARED, so the corpus side and the author side of a declaration are
# budgeted separately.
#
# HOW LIMB 2 WORKS. Its population is DERIVED from the consuming code, never hand-listed: every
# ls-files / ls-tree / find invocation on a live (non-comment, non-string) logical line of an
# executable file under tests/ and scripts/. Each site is then asked one computed question — can
# its scope reach a file under <results_dir>/? — answered from the walk root, the pathspecs and the
# find depth bound, with the prefix relation computed from the literal rather than spelled out. As
# in limb 1 the predicate is INVERTED: reaching-and-unbounded is the DEFAULT, and every route past
# it is a named, budgeted class.
#
#   SCOPED    the walk PROVABLY cannot reach <results_dir>/… — rooted in another tree, or in a
#             subtree off the results chain, or every pathspec is off that chain, or its find
#             -maxdepth is shallower than the tree is deep. This is the by-construction shape, and
#             it is what test_comment_anchor_style.sh earns from its explicit non-docs pathspec
#             list — a recognised shape, not an exception entry.
#   EXCLUDED  the invocation itself carries an exclusion pathspec at or ABOVE <results_dir>/.
#   FILTERED  the invocation is unscoped, but its FILE applies a path-prefix exclusion at or above
#             <results_dir>/ to the walk output — test_grep_portability.sh's `case` arm. The widest
#             pass of the five, and pinned on its own.
#   WRAPPED   a git walk whose REPOSITORY is chosen by a wrapper rather than by `git -C` — which
#             tree it enumerates is decided by the wrapper definition, out of reach of any
#             line-scoped rule. Budgeted, so a second one cannot appear quietly.
#   DECLARED  a hand-declared reaching walk that an addition under the tree cannot move.
#   HAZARD    THE DEFAULT, exactly as in limb 1.
#
# HONEST LIMITS — this file claims no more than it mutates. (a)-(e) bound LIMB 1, (f) bounds LIMB 2.
#   (a) HAZARD is the DEFAULT, so it is the BENIGN side that is curated — and therefore the side
#       that is counted. Six routes lead past the default: four shape classes and two hand-
#       declared tables. The two tables are pinned individually; the four shape classes are pinned
#       in AGGREGATE, so any ADDITION moves a number no other assertion moves, but a pure SWAP
#       between two shape classes does not. The two tables carry an entry counter each on top of
#       their occurrence counter. The one swap worth pricing is priced: OPTVAL is the
#       widest of the six — `--file`, `--include` and their kin are also long options and the
#       receiving command READS what they name — so the space-separated spellings of those are
#       denied the pass by name, and the `=`-joined spellings (`--include=<tree>`) are denied it by
#       the ASSIGN shape being a real assignment rather than any token carrying an `=`. OPTVAL
#       carries a second counter of its own. A swap among
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
#   (d) SCOPE (limb 1). The corpus is every tracked path except docs/ — exclusion by scan scope,
#       never by exception entry (ADR-0050), so a new top-level directory is in scope. docs/
#       holds point-in-time records (results, specs, plans, archived changes, ADRs) that the
#       convention forbids rewriting, and the metadata branch is absent from this checkout
#       entirely. That the exclusion is safe is asserted mechanically rather than claimed: no
#       tracked path under docs/ is an executable script.
#   (e) TRACKED-AND-COMMITTED ONLY. A brand-new consumer is invisible here until it is committed.
#       Accepted: this guard runs at the build gate over committed work, which is the same tree
#       finalize's skip decision is made against.
#   (f) LIMB 2 has three named gaps, and all three fail in a direction the budgets can see.
#       INDIRECT SCOPE, the same family as (b): a walk root or pathspec held in a variable this
#       file cannot tie back to the repo root — `find "$SBX"`, `-- "$CHANGES_DIR/active"` — reads
#       as another tree, so a variable that happened to hold the results tree would pass. The
#       repo-root variables ARE derived rather than listed (any name assigned from BASH_SOURCE in
#       the same file), which is what makes `find "$ROOT/docs"` reaching and `find "$ROOT/skills"`
#       scoped, but a root computed some other way is out of reach. WRAPPED is the same gap for
#       git, isolated into its own budgeted class so it cannot spread silently.
#       FILE-SCOPED FILTERS: the FILTERED pass looks for the path-prefix exclusion anywhere in the
#       walker's own file, not bound to the walk it protects, so a file could in principle satisfy
#       it with an exclusion belonging to a different invocation. What prices that is the exact
#       FILTERED budget — a second unscoped walk in the same file moves the number even though the
#       hazard count stays at zero.
#       EXCLUSION SPELLINGS: three shapes are recognised as a path-prefix exclusion — a `:!`/
#       `:(exclude)` pathspec, a `!`-negated glob, and a `case` arm ending in `*)`. A fourth
#       spelling (a `[[ ]]` test, a `grep -v`) fails CLOSED, as a loud HAZARD, which is the same
#       direction limb 1 chose and the reason it is safe to recognise shapes at all.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SELF_REL="tests/$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){  printf 'ok   - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }
assert(){ if eval "$2"; then ok "$1"; else nok "$1"; fi; }

# The allowlisted tree. This assignment is the only place this file writes the bare literal, and it
# sits right of an `=` inside its own token, so the self-scan below classifies it ASSIGN. Every
# other use interpolates it — BOTH LIMBS, since limb 2's RESULTS_PARENT is derived from it too, so
# binding this one name binds the whole file. What makes the literal legitimate rather than a
# hard-coded guess is the CORRESPONDENCE section below, which ties it to the results_dir the
# scanned revision actually configures.
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
EXPECTED_BENIGN=16
# BENIGN moved 15 -> 16 in change 0244, DATA class. The added occurrence is
# tests/test_frontmatter_read_shapes.sh's `results: <results_dir>/PROSE-NOT-A-VALUE.md`, a line of a
# QUOTED heredoc (`<<'EOF'`) that writes a fixture change file into a mktemp dir. It provably opens
# nothing: no command word, no separator and no substitution anywhere on the logical line, so no
# program receives the path as an operand — the DATA rule that claimed it is the rule that applies.
# It is bait, not a value: the fixture omits `results:` from its frontmatter and puts this line in
# the BODY, and the test asserts the anchored read returns empty instead of picking the prose up.
# Nothing resolves or opens it even if that read regressed — render-change-links.sh renders the
# value into a link, it does not stat the tree. Same shape as the four sibling DATA occurrences in
# test_board_checks.sh, test_render_change_links.sh and test_terminal_publish.sh.
EXPECTED_OPTVAL=2
EXPECTED_CURATED=3
EXPECTED_EXEMPT=2
# ...and the DECLARATION counts, pinned separately from the occurrence counts above. The two are
# independent budgets on purpose: an occurrence count is moved by the corpus, a declaration count by
# the author of this file. Adding a table entry to cover a new read moves the entry budget even in
# the case where the occurrence budget would not — a slice widened to span a second occurrence, or
# an entry re-pointed at a different line — so neither number can absorb the other silently.
EXPECTED_EXEMPT_ENTRIES=2
EXPECTED_CURATED_ENTRIES=3

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
NO_EXEMPTIONS="$TMP/no-exemptions.tsv"
: > "$NO_EXEMPTIONS"

# ---------------------------------------------------------------------------
# 0. CORRESPONDENCE — the literal above IS the configured results_dir, asserted rather than assumed.
#
# docket-finalize-change is instructed to read the allowlisted tree from config and never to
# hard-code a path name, so a literal here is one half of a two-part correspondence with nothing
# binding it: retarget the config key and finalize allowlists the new tree while this guard goes on
# certifying the old one — green, and certifying the wrong property (repo learning:
# correspondence-guard-runs-one-way). This section is the binding. The literal is kept rather than
# derived because every budget in this file was measured against THIS tree; a retarget must stop the
# guard and force a recalibration, not silently re-aim a set of numbers that no longer describe
# anything.
#
# THE VALUE COMES FROM THE SCANNED REVISION. Everything below scans $REV, so the config that governs
# it is the one $REV carries — read straight out of the committed blob, never off the working tree
# and never through a layer a single machine can set. results_dir is a coordination-fenced key in
# scripts/docket-config.sh (its per-repo-only fence list), so a .docket.local.yml or a global
# config.yml retargeting it is warned-and-ignored there; reading only the committed blob here gives
# this guard the same immunity by construction rather than by trusting that fence to hold.
# The parse mirrors the resolver's committed-layer reader (config_line_scalar_get): strip a `#`
# comment, split on the first colon, trim, drop one layer of matching quotes, take the first match.
CFG_YML="$TMP/rev-config-yml"
git -C "$ROOT" show "$REV:.docket.yml" > "$CFG_YML" 2>/dev/null || : > "$CFG_YML"

# The configured results_dir, or empty when the key is absent. Empty is NOT silently defaulted:
# defaulting it to the built-in would make an unreadable or keyless config read as agreement, which
# is the vacuous-green shape this whole file exists to refuse.
configured_results_dir(){
  awk -v key=results_dir '
    BEGIN { sq = sprintf("%c", 39); dq = sprintf("%c", 34) }
    {
      line = $0
      sub(/#.*/, "", line)
      if (index(line, ":") == 0) next
      k = substr(line, 1, index(line, ":") - 1)
      v = substr(line, index(line, ":") + 1)
      gsub(/^[[:space:]]+/, "", k); gsub(/[[:space:]]+$/, "", k)
      gsub(/^[[:space:]]+/, "", v); gsub(/[[:space:]]+$/, "", v)
      if (k != key) next
      if (length(v) >= 2) {
        f = substr(v, 1, 1); l = substr(v, length(v), 1)
        if ((f == dq && l == dq) || (f == sq && l == sq)) v = substr(v, 2, length(v) - 2)
      }
      print v
      exit
    }
  ' "$1"
}

# ONE implementation, shared by the live assert and the controls below, exactly as with the two
# scanners: 0 bound, 1 retargeted (the literal is stale), 2 the key is absent (nothing to bind to,
# or the reader went blind).
results_dir_bound(){
  local v
  v="$(configured_results_dir "$1")"
  [ -n "$v" ] || return 2
  [ "$v" = "$RESULTS_DIR_REL" ]
}

results_dir_bound "$CFG_YML"; cfg_bind_rc=$?
assert "the scanned rev configures a results_dir at all (this binding is not vacuous)" \
  '[ "$cfg_bind_rc" != 2 ] || { echo "  no results_dir key in $REV:.docket.yml." >&2; echo "  Either the repo stopped configuring the tree — in which case finalize falls back to the built-in default and this guard must be re-pointed at that, deliberately — or this reader stopped matching how the key is spelled. Establish which before touching the literal." >&2; false; }'
assert "the tree this guard scans is the results_dir the scanned rev configures" \
  '[ "$cfg_bind_rc" = 0 ] || { printf "  configured %s, scanned %s.\n" "$(configured_results_dir "$CFG_YML")" "$RESULTS_DIR_REL" >&2; echo "  finalize resolves the allowlisted tree from config, so a retarget moves the tree whose invisibility has to hold. This guard does NOT follow it automatically on purpose: every budget below (the floor, the shape and table counts, the walk counts) was measured against the old tree and describes nothing about the new one. Re-point the literal AND re-measure every budget in the same diff." >&2; false; }'

# MUTATION CONTROL for the binding, run through the SAME predicate: a retargeted key must be
# REPORTED, and a deleted key must be reported as absent rather than as agreement. Both mutations
# are confirmed to have landed by occurrence count before their result is believed (repo learning:
# assert-detects-removal-not-replacement).
MUT_CFG="$TMP/mutated-config-yml"
BLIND_CFG="$TMP/keyless-config-yml"
sed 's|^results_dir:.*|results_dir: docs/elsewhere|' "$CFG_YML" > "$MUT_CFG"
sed '/^results_dir:/d' "$CFG_YML" > "$BLIND_CFG"
# Keyed on the CONFIGURED value, not on the literal: a control whose precondition is the very
# correspondence under test would go red as a duplicate of the assert above instead of reporting on
# the mutation it exists to price.
cfg_val="$(configured_results_dir "$CFG_YML")"
cfg_before="$(grep -c -E -e "^results_dir:[[:space:]]*$cfg_val" "$CFG_YML" || true)"
cfg_after="$(grep -c -E -e "^results_dir:[[:space:]]*$cfg_val" "$MUT_CFG" || true)"
cfg_new="$(grep -c -F -e 'results_dir: docs/elsewhere' "$MUT_CFG" || true)"
cfg_gone="$(grep -c -E -e '^results_dir:' "$BLIND_CFG" || true)"
assert "mutation landed: the configured key is present before, retargeted after, and deletable" \
  '[ "$cfg_before" = 1 ] && [ "$cfg_after" = 0 ] && [ "$cfg_new" = 1 ] && [ "$cfg_gone" = 0 ]'
results_dir_bound "$MUT_CFG"; cfg_mut_rc=$?
results_dir_bound "$BLIND_CFG"; cfg_blind_rc=$?
assert "control: a retargeted results_dir is REPORTED as a stale literal" \
  '[ "$cfg_mut_rc" = 1 ] && [ "$(configured_results_dir "$MUT_CFG")" = docs/elsewhere ]'
assert "control: a deleted results_dir is REPORTED as absent, not read as agreement" \
  '[ "$cfg_blind_rc" = 2 ]'

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
    # Does some occurrence of slice in text SPAN the occurrence at pos? A declared table entry
    # scopes to the OCCURRENCE, not to the logical line: a line-wide `index(text, slice) > 0` lets
    # a read appended to an already-declared line inherit the pass, and a read spelled through a
    # shell variable moves no counter at all while it does so.
    function slice_spans(text, slice, pos, plen,   s, off) {
      if (slice == "") return 0
      off = 1
      while (1) {
        s = index(substr(text, off), slice)
        if (s == 0) return 0
        s = off + s - 1
        if (s <= pos && pos + plen - 1 <= s + length(slice) - 1) return 1
        off = s + 1
      }
    }
    function class_of(isexec, text, pos,   pre, l1, l2, pw, ap) {
      if (isexec != "1")            return "INERT"
      if (text ~ /^[[:space:]]*#/)  return "COMMENT"
      pre = token_prefix(text, pos)
      l1  = substr(pre, length(pre), 1)
      l2  = substr(pre, length(pre) - 1, 2)
      if (l2 == ":!")               return "EXCL"
      if (l1 == "!")                return "EXCL"
      if (l1 == "^" && text ~ inv)  return "EXCL"
      # An assignment or env-prefix VALUE, not a path operand. The shape is a GENUINE SHELL
      # ASSIGNMENT and nothing looser: a `NAME=` token prefix (optionally opened by a quote, as in
      # a quoted env line inside a printf word list), and nothing else. A `${NAME:-<lit>}` default
      # expansion earns the class only through that same prefix — `X="${X:-<lit>}"` opens with
      # `X=` — and DELIBERATELY not on its own: a defaulted expansion sitting in OPERAND position,
      # `some_cmd "${DIR:-<lit>}"`, is a read, and the one live instance of the shape is an option
      # value that belongs in the budgeted OPTVAL class rather than in this unbudgeted one.
      # The looser "the token prefix contains an =" test this replaced also handed the pass to
      # `--include=<lit>` and `--file=<lit>` — long options whose RECEIVING COMMAND opens what they
      # name, i.e. genuine reads — and it was tested ahead of every verb rule, so those were
      # counted nowhere at all. They now fall through to HAZARD, and section 7a asserts exactly
      # that. What this class still cannot see is the INDIRECT form named in HONEST LIMITS (b)
      # above, which no single-line predicate reaches in either direction.
      ap = pre
      sub(qstrip, "", ap)
      if (ap ~ asgnl)               return "ASSIGN"
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
      # The ASSIGN shape, spelled without an apostrophe anywhere: this whole program is a
      # single-quoted shell word, so 39 has to be built with sprintf.
      qstrip = "^[" sprintf("%c", 39) "\"]+"
      asgnl  = "^[[:alpha:]_][[:alnum:]_]*="
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
            if (ex_path[k] == path && slice_spans(text, ex_slice[k], pos, length(lit))) { ex_hit[k] = 1; c = ex_class[k]; break }
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
# the fixture, not the tree. The slice SPANS the occurrence rather than merely preceding it,
# because the match is scoped to the occurrence: a bare `| Results | [` would cover the whole
# logical line and hand its pass to anything else that line later grew.
exempt tests/test_render_change_links.sh "blob/feat/build/$RESULTS_DIR_REL/2026-06-21-build-results.md) |"

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
table_entries(){ awk -F'\t' -v c="$1" '$1 == c {n++} END {print n+0}' "$2"; }
n_exempt_entries="$(table_entries EXEMPT "$EXEMPTIONS")"
n_curated_entries="$(table_entries CURATED "$EXEMPTIONS")"
assert "exactly $EXPECTED_EXEMPT_ENTRIES exemption entries are declared" \
  '[ "$n_exempt_entries" = "$EXPECTED_EXEMPT_ENTRIES" ] || { echo "  declared $n_exempt_entries, budget $EXPECTED_EXEMPT_ENTRIES." >&2; echo "  This counts TABLE ENTRIES, not occurrences: it is the budget a widened slice or a re-pointed entry moves even when the occurrence total holds. Every entry admits a real read of the tree, so a new one needs its claim written down beside it." >&2; false; }'
assert "exactly $EXPECTED_CURATED_ENTRIES benign entries are declared" \
  '[ "$n_curated_entries" = "$EXPECTED_CURATED_ENTRIES" ] || { echo "  declared $n_curated_entries, budget $EXPECTED_CURATED_ENTRIES." >&2; false; }'
assert "no declared table entry is stale (every one still matches a live occurrence)" \
  '[ "$n_orphan" = 0 ] || { grep "^ORPHAN" <<<"$RESULT" >&2; echo "  The declared consumer moved or was rewritten. Re-read it and re-decide, rather than re-pointing the slice." >&2; false; }'

# ---------------------------------------------------------------------------
# 6. THE POSITIVE CLAIM — the suite's real exclusion mechanisms survive, keyed on their magic
#    token and bound to the probe that uses it. A bare-path assert is NOT enough here: two of the
#    three files carry the bare literal elsewhere (a comment and an armed probe in one, nothing but
#    the escape in the other), so the real exclusion can be deleted with the bare path untouched.
#
#    THE LIST IS THE LIVE ONE, and it is three long. The classifier files each of these three under
#    EXCL and moves on, which is a statement that the occurrence is not a read — not a statement
#    that the exclusion still WORKS. So each gets a named extractor and a mutation control:
#    test_docket_build.sh's `:!` pathspec, test_readme_finalize_docs.sh's rg `--glob` escape, and
#    test_cursor_contract_docs.sh's `grep -v` prefix alternation. The third is the one shape the
#    walk limb (section 9) explicitly does NOT recognise — HONEST LIMITS (f), EXCLUSION SPELLINGS,
#    names `grep -v` as failing closed there — so this section is the only place it is asserted at
#    all, and it would otherwise rest on the population floor alone.
# ---------------------------------------------------------------------------
TDB="$ROOT/tests/test_docket_build.sh"
RMF="$ROOT/tests/test_readme_finalize_docs.sh"
CCD="$ROOT/tests/test_cursor_contract_docs.sh"

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
# Same contract, same three return codes, for the third mechanism: test_cursor_contract_docs.sh
# pipes a whole-repo `git grep -l` through a `grep -v` alternation of point-in-time prefixes. The
# anchor is the extractor function the file defines, and the invert-match pipe is what makes the
# probe the exclusion rather than a bare mention.
ccd_exclusion_armed(){
  local probe
  probe="$(flatten "$1" | grep -F -e 'stale_grep()' || true)"
  [ -n "$probe" ] || return 2
  case "$probe" in *"grep -v"*) ;; *) return 2 ;; esac
  grep -qF -e "^$RESULTS_DIR_REL/" <<<"$probe"
}

assert "test_docket_build.sh's live-tree probe still excludes the results tree by pathspec" \
  'tdb_exclusion_armed "$TDB"'
assert "test_readme_finalize_docs.sh's doc-content search still escapes the results tree by glob" \
  'rmf_exclusion_armed "$RMF"'
assert "test_cursor_contract_docs.sh's stale-claim sweep still drops the results tree by prefix" \
  'ccd_exclusion_armed "$CCD"'

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
  # ...nor may its `=`-JOINED spelling buy the ASSIGN pass. `--include=` and `--file=` put an `=`
  # in the token prefix, and ASSIGN is tested ahead of every verb rule, so under a bare
  # "the prefix contains an =" test these two genuine reads were classified benign and counted
  # nowhere at all. They are the reason ASSIGN keys on a real assignment shape.
  synth 1 tests/test_synthetic_probe.sh 13 "  grep --include=$RESULTS_DIR_REL/*.md -r \"\$ROOT\""
  synth 1 tests/test_synthetic_probe.sh 14 "  rg --file=$RESULTS_DIR_REL/pat \"\$ROOT\""
  # The positive half of the same rule, and its boundary. 15 is a default expansion whose token
  # DOES open with an assignment (`X=`), so it keeps the class; 16 is the same expansion in operand
  # position, which is a read and must not.
  synth 1 tests/test_synthetic_probe.sh 15 "  X=\"\${X:-$RESULTS_DIR_REL}\""
  synth 1 tests/test_synthetic_probe.sh 16 "  some_cmd \"\${DIR:-$RESULTS_DIR_REL}\""
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
assert "control: --include=<tree> is DENIED the ASSIGN pass and classifies HAZARD" \
  '[ "$(synth_class 13)" = HAZARD ]'
assert "control: --file=<tree> is DENIED the ASSIGN pass and classifies HAZARD" \
  '[ "$(synth_class 14)" = HAZARD ]'
assert "control: a \${NAME:-} default on the right of an assignment still classifies ASSIGN" \
  '[ "$(synth_class 15)" = ASSIGN ]'
assert "control: the same default expansion in OPERAND position classifies HAZARD" \
  '[ "$(synth_class 16)" = HAZARD ]'

# 7a-bis. THE EXEMPTION IS SCOPED TO THE OCCURRENCE, NOT THE LINE. The declared slice is run
# against a synthetic record that reproduces the exempted line and then APPENDS a second read to
# it — the shape a reflow or a one-line addition produces. The first occurrence must keep its
# EXEMPT; the second must be REPORTED as HAZARD rather than inheriting the line-wide pass. Both
# rows come out of the SAME classifier and the SAME live $EXEMPTIONS table the corpus scan uses.
EX_SLICE="'docs/changes/archive' '$RESULTS_DIR_REL' 'docs/superpowers' 'docs/adrs'"
{
  synth 1 tests/test_docket_build.sh 1 "  h=\"\$(git grep -Il x -- $EX_SLICE)\""
  synth 1 tests/test_docket_build.sh 2 "  h=\"\$(git grep -Il x -- $EX_SLICE)\"; some_helper \"\$ROOT/$RESULTS_DIR_REL\""
} > "$TMP/inherit.tsv"
INHERIT="$(classify "$EXEMPTIONS" < "$TMP/inherit.tsv")"
assert "mutation landed: the appended line carries the exempted slice AND a second occurrence" \
  '[ "$(awk -F"\t" '"'"'$3 == 2'"'"' <<<"$INHERIT" | wc -l | tr -d " ")" = 2 ]'
assert "control: the declared occurrence on the exempted line still classifies EXEMPT" \
  '[ "$(awk -F"\t" '"'"'$3 == 1 {print $1}'"'"' <<<"$INHERIT")" = EXEMPT ]'
assert "control: a second read appended to the exempted line does NOT inherit and classifies HAZARD" \
  '[ "$(awk -F"\t" '"'"'$3 == 2 {print $1}'"'"' <<<"$INHERIT" | tr "\n" " ")" = "EXEMPT HAZARD " ]'

# ...and the ENTRY counter is a second, independent budget: adding a table entry moves it even
# though the occurrence counts above are computed from the corpus.
MORE_EX="$TMP/more-exemptions.tsv"
cp "$EXEMPTIONS" "$MORE_EX"
printf 'EXEMPT\t%s\t%s\n' tests/test_synthetic_probe.sh "$RESULTS_DIR_REL/x" >> "$MORE_EX"
assert "control: declaring one more exemption moves the entry counter on its own" \
  '[ "$(table_entries EXEMPT "$MORE_EX")" = "$((EXPECTED_EXEMPT_ENTRIES + 1))" ] && [ "$(table_entries CURATED "$MORE_EX")" = "$EXPECTED_CURATED_ENTRIES" ]'

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

# 7c-bis. The same, for the grep -v prefix alternation. Deleting the results-tree alternative alone
# leaves the pipeline, the two sibling prefixes and the whole invocation shape intact — which is
# precisely why the extractor keys on the alternative and not on the pipe.
MUT_CCD="$TMP/mutated_cursor_contract.sh"
awk -v tok="^$RESULTS_DIR_REL/\\\\|" '{ while ((p = index($0, tok)) > 0) $0 = substr($0, 1, p - 1) substr($0, p + length(tok)); print }' \
  "$CCD" > "$MUT_CCD"
ccd_before="$(grep -c -F -e "^$RESULTS_DIR_REL/" "$CCD" || true)"
ccd_after="$(grep -c -F -e "^$RESULTS_DIR_REL/" "$MUT_CCD" || true)"
ccd_siblings="$(grep -c -F -e '^docs/superpowers/' "$MUT_CCD" || true)"
assert "mutation landed: the results-tree alternative is present before, gone after, siblings intact" \
  '[ "$ccd_before" -ge 1 ] && [ "$ccd_after" = 0 ] && [ "$ccd_siblings" -ge 1 ]'
ccd_exclusion_armed "$MUT_CCD"; ccd_mut_rc=$?
assert "control: the prefix-alternation check REPORTS the deleted alternative" '[ "$ccd_mut_rc" = 1 ]'
BLIND_CCD="$TMP/blind_cursor_contract.sh"
sed 's/stale_grep/renamed_grep/g' "$CCD" > "$BLIND_CCD"
assert "mutation landed: the stale-claim extractor anchor is gone from the blinded copy" \
  '[ "$(grep -c -F -e "stale_grep()" "$BLIND_CCD" || true)" = 0 ]'
ccd_exclusion_armed "$BLIND_CCD"; ccd_blind_rc=$?
assert "control: a renamed stale-claim extractor is reported as a broken extractor, not as armed" \
  '[ "$ccd_blind_rc" = 2 ]'

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
#
#    It runs over LOGICAL lines, through the same logical_lines join the corpus scan uses — not
#    over physical ones. A bespoke per-physical-line loop here would contradict the header's own
#    reason for the join: a read in THIS file spelled with the command on one physical line and
#    its path operand on the next would be handed a free-pass class for exactly the reason a raw
#    per-line predicate is wrong everywhere else.
# ---------------------------------------------------------------------------
SELF_RECORDS="$TMP/self.tsv"
: > "$SELF_RECORDS"
while IFS=$'\t' read -r self_ln sline; do
  [ -n "$self_ln" ] || continue
  case "$sline" in
    *"$RESULTS_DIR_REL"*) printf '%s\t%s\t%s\t%s\n' 1 "$SELF_REL" "$self_ln" "$sline" >> "$SELF_RECORDS" ;;
  esac
done < <(logical_lines "${BASH_SOURCE[0]}")
SELF_RESULT="$(classify "$NO_EXEMPTIONS" < "$SELF_RECORDS")"
self_total="$(grep -c . <<<"$SELF_RESULT" || true)"
self_hazard="$(count_class HAZARD "$SELF_RESULT")"
assert "self-scan is armed (this file does carry the literal it excludes itself for)" \
  '[ "$self_total" -ge 1 ]'
assert "this guard does not itself read the allowlisted results tree" \
  '[ "$self_hazard" = 0 ] || { grep "^HAZARD" <<<"$SELF_RESULT" >&2; false; }'

# ---------------------------------------------------------------------------
# 9. LIMB 2 — THE WHOLE-TREE WALKERS. Everything above is keyed on the results-dir LITERAL, so a
#    component that enumerates the whole repo and drops the tree at a BROADER prefix never enters
#    it. This limb closes that shape by deriving the walk sites from the consuming code and asking
#    each one whether its scope can reach a file under the results tree. See HOW LIMB 2 WORKS.
# ---------------------------------------------------------------------------

# Classify every tree-walk invocation in the files named. stdout is one row per site,
# `<class>TAB<path>TAB<lineno>TAB<text>`. ONE implementation, shared by the live scan, the synthetic
# controls and the mutation controls, so neutering the derivation anywhere neuters it everywhere.
#
# It takes MANY files in one awk process rather than one file per call: the live population is every
# executable blob under tests/ and scripts/, and a process pair per file costs more wall clock than
# this whole file is budgeted. That is also why the backslash-continuation join is inlined below
# instead of piping through logical_lines — a per-file pipeline is what the batching removes.
#
# $1 declarations table, $2 prefix to strip from each name to get its repo path, $3 label override
# ("" derives the label from the name), $4.. the files.
walk_classify(){
  local decf="$1" pre="$2" lbl="$3"; shift 3
  awk -v lit="$RESULTS_DIR_REL" -v decf="$decf" -v pre="$pre" -v lbl="$lbl" '
    # --- the prefix arithmetic, computed from the literal, never spelled out -------------------
    # covers(): pp is an exclusion prefix AT or ABOVE the results tree, so excluding it excludes
    # the tree. docs -> yes. docs/results -> yes. docs/changes -> NO, and that NO is the whole
    # point of this limb.
    function covers(pp) {
      if (pp == "") return 0
      if (pp == lit) return 1
      if (substr(lit, 1, length(pp) + 1) == pp "/") return 1
      return 0
    }
    # onchain(): s and the results tree lie on one root-to-leaf chain, so a walk scoped to s can
    # reach a file under the tree (or is inside it already).
    function onchain(s) {
      if (s == "") return 1
      if (s == lit) return 1
      if (substr(lit, 1, length(s) + 1) == s "/") return 1
      if (substr(s, 1, length(lit) + 1) == lit "/") return 1
      return 0
    }
    function comps(s,   a) { if (s == "") return 0; return split(s, a, "/") }
    function strip(t) { gsub(qcls, "", t); gsub(/^[({]+/, "", t); gsub(/[)}]+$/, "", t); sub(/\/+$/, "", t); return t }
    # Resolve a walk root or pathspec to a repo-relative path, or OTHER when it is rooted in a tree
    # this file cannot tie back to the repo root. The repo-root variable names are DERIVED from the
    # file being scanned (anything assigned from BASH_SOURCE), never enumerated here.
    function resolve(t,   v, rest) {
      t = strip(t)
      if (t == "" || t == ".") return ""
      if (substr(t, 1, 1) == "$") {
        rest = substr(t, 2)
        if (substr(rest, 1, 1) == "{") { sub(/^\{/, "", rest); sub(/}/, "", rest) }
        v = rest; sub(/\/.*$/, "", v)
        rest = (index(rest, "/") ? substr(rest, index(rest, "/") + 1) : "")
        if (!(v in repovar)) return "OTHER"
        if (index(rest, "$") > 0) return ""
        sub(/\/+$/, "", rest)
        return rest
      }
      if (index(t, "$") > 0) return ""
      return t
    }
    # --- is this occurrence CODE, or is it text? -----------------------------------------------
    # Mark every character that sits inside a double-quoted span carrying no command substitution:
    # such a span is data (a diagnostic message, a sed script, an expected string), and the walk
    # words inside it invoke nothing. A `$(` opens a FRESH quoting context, which is why the state
    # is stacked — `x="$(git -C "$ROOT" ls-tree …)"` is a real invocation even though it opens
    # inside a double quote, and a flat scanner phases the quotes wrong and calls it text.
    function build_dead(text,   i, n, c, insq, indq, st, sb, d, k) {
      n = length(text); insq = 0; indq = 0; st = 0; sb = 0; d = 0
      for (i = 1; i <= n; i++) dead[i] = 0
      for (i = 1; i <= n; i++) {
        c = substr(text, i, 1)
        if (c == "\\") { i++; continue }
        if (insq) { if (c == sq) insq = 0; continue }
        if (c == "$" && substr(text, i + 1, 1) == "(") {
          if (indq) sb = 1
          d++; sv_q[d] = indq; sv_s[d] = st; sv_b[d] = sb
          indq = 0; st = 0; sb = 0; i++
          continue
        }
        if (c == ")" && d > 0 && !indq) { indq = sv_q[d]; st = sv_s[d]; sb = sv_b[d]; d--; continue }
        if (indq) { if (c == dq) { if (!sb) for (k = st; k <= i; k++) dead[k] = 1; indq = 0 } ; continue }
        if (c == sq) { insq = 1; continue }
        if (c == dq) { indq = 1; st = i; sb = 0 }
      }
    }
    function tokenize(text,   i, n, c, cur, pos) {
      ntok = 0; n = length(text); cur = ""; pos = 0
      for (i = 1; i <= n; i++) {
        c = substr(text, i, 1)
        if (c == " " || c == "\t") { if (cur != "") { ntok++; tok[ntok] = cur; tpos[ntok] = pos; cur = "" } }
        else { if (cur == "") pos = i; cur = cur c }
      }
      if (cur != "") { ntok++; tok[ntok] = cur; tpos[ntok] = pos }
    }
    # The command word a token ends in, past any shell punctuation glued to its front:
    # `<(find` -> find, `x="$(git` -> git, `$($GIT` -> GIT.
    function cmdword(t,   i, c, last) {
      last = 0
      for (i = 1; i <= length(t); i++) { c = substr(t, i, 1); if (index(brk, c) > 0) last = i }
      return substr(t, last + 1)
    }
    function tail_strip(t) { gsub(tailcls, "", t); return t }
    function declared(t,   z) { for (z = 1; z <= nd; z++) if (d_path[z] == pathlabel && index(t, d_slice[z]) > 0) return 1; return 0 }
    # --- the verdict for one walk site ---------------------------------------------------------
    function site(kind, k, t,   j, m, r, md, reach, npos, anyx, e, cw, hasC, argC) {
      reach = 0; npos = 0; anyx = 0
      if (kind == "FIND") {
        md = -1
        for (j = k + 1; j <= ntok; j++) if (tok[j] == "-maxdepth" && j < ntok) md = tok[j + 1] + 0
        for (j = k + 1; j <= ntok; j++) {
          if (tok[j] ~ /^[-(!]/) break
          r = resolve(tok[j])
          if (r == "OTHER" || !onchain(r)) continue
          # A depth bound shallower than the tree is deep cannot reach a file inside it. The
          # threshold is computed from the literal, so a deeper results_dir relaxes it by itself.
          if (md >= 0 && md < (comps(lit) - comps(r) + 1)) continue
          reach = 1
        }
        if (!reach) return "SCOPED"
        if (fb) return "FILTERED"
        return declared(t) ? "DECLARED" : "HAZARD"
      }
      for (j = k + 1; j <= ntok; j++) {
        if (tok[j] != "--") continue
        for (m = j + 1; m <= ntok; m++) {
          if (tok[m] ~ /^[|;&]/ || tok[m] ~ /^[0-9]*[<>]/) break
          e = strip(tok[m]); if (e == "") continue
          if (e ~ /^:(!|\(exclude\))/) {
            sub(/^:(!|\(exclude\))/, "", e); sub(/\/?\*+$/, "", e); sub(/\/+$/, "", e)
            if (covers(e)) anyx = 1
            continue
          }
          npos++
          # A :(glob) pathspec with no slash matches the repo ROOT level only — git glob magic
          # stops * at a path separator — so it cannot descend into docs/ at all. Without the magic
          # the same pattern crosses separators, so it is treated as reaching.
          if (e ~ /^:\(glob\)/) { sub(/^:\(glob\)/, "", e); if (index(e, "/") == 0) continue }
          sub(/^:[^)]*\)/, "", e)
          r = resolve(e)
          if (r == "OTHER") continue
          if (index(e, "*") > 0) { reach = 1; continue }
          if (onchain(r)) reach = 1
        }
        break
      }
      if (anyx) return "EXCLUDED"
      if (npos > 0 && !reach) return "SCOPED"
      hasC = 0
      for (j = 1; j < k; j++) if (tok[j] == "-C" && j < ntok) { hasC = 1; argC = tok[j + 1] }
      if (hasC) { r = resolve(argC); if (r == "OTHER" || !onchain(r)) return "SCOPED" }
      else {
        cw = ""
        for (j = k - 1; j >= 1; j--) {
          if (tok[j] ~ /^-/) continue
          if (j > 1 && (tok[j - 1] == "-C" || tok[j - 1] == "-c")) continue
          cw = cmdword(tok[j]); break
        }
        if (cw != "git") return "WRAPPED"
      }
      if (declared(t)) return "DECLARED"
      # An EXPLICIT pathspec into the results chain is a deliberate walk of that tree, so the
      # file-scoped FILTERED fallback is denied it: a filter belonging to some other invocation
      # must not launder a walk that was aimed at the tree on purpose.
      if (npos > 0) return "HAZARD"
      return fb ? "FILTERED" : "HAZARD"
    }
    BEGIN {
      dq = "\""; sq = sprintf("%c", 39)
      qcls = "[" dq sq "]"
      brk  = "(){}|;&<>$=" dq sq
      tailcls = "[)}|;&<>" dq sq "]+$"
      # Characters that, sitting immediately left of the word, mean it is part of a larger literal
      # rather than a command: a quote opens a pattern, a slash is inside a sed script.
      bad = "[[:alnum:]_./=" dq sq "-]"
      nd = 0
      while ((getline dl < decf) > 0) {
        if (dl == "") continue
        if (split(dl, a, "\t") < 3) continue
        nd++; d_path[nd] = a[2]; d_slice[nd] = a[3]
      }
    }
    function rel(f) { if (pre != "" && index(f, pre) == 1) f = substr(f, length(pre) + 1); return f }
    # One file finished; classify what was accumulated for it, then reset. Called at the first line
    # of the NEXT file and at END, so pathlabel/fb/repovar still describe the file being flushed.
    function emit_file(   i, t, s, e, k, w, kind, q, b, v) {
      if (pend != "") { ln[++N] = pstart; tx[N] = pend; pend = "" }
      for (i = 1; i <= N; i++)
        if (tx[i] ~ /^[[:space:]]*[[:alpha:]_][[:alnum:]_]*=.*BASH_SOURCE/) {
          v = tx[i]; sub(/^[[:space:]]*/, "", v); sub(/=.*$/, "", v); repovar[v] = 1
        }
      # File-level path-prefix exclusions, in the three recognised shapes (HONEST LIMITS (f)).
      for (i = 1; i <= N; i++) {
        t = tx[i]
        if (t ~ /^[[:space:]]*#/) continue
        s = t
        while (match(s, /:(!|\(exclude\))[^[:space:]]+/)) {
          e = substr(s, RSTART, RLENGTH); s = substr(s, RSTART + RLENGTH)
          sub(/^:(!|\(exclude\))/, "", e); e = strip(e); sub(/\/?\*+$/, "", e); sub(/\/+$/, "", e)
          if (covers(e)) fb = 1
        }
        s = t
        while (match(s, /!\/?[[:alnum:]_.\/-]+/)) {
          e = substr(s, RSTART, RLENGTH); s = substr(s, RSTART + RLENGTH)
          sub(/^!/, "", e); sub(/\/?\*+$/, "", e); sub(/\/+$/, "", e)
          if (covers(e)) fb = 1
        }
        if (match(t, /^[[:space:]]*[[:alnum:]_.\/-]+\*+\)/)) {
          e = substr(t, RSTART, RLENGTH); sub(/^[[:space:]]*/, "", e); sub(/\*+\)$/, "", e); sub(/\/+$/, "", e)
          if (covers(e)) fb = 1
        }
      }
      for (i = 1; i <= N; i++) {
        t = tx[i]
        if (t ~ /^[[:space:]]*#/) continue
        build_dead(t); tokenize(t)
        for (k = 1; k <= ntok; k++) {
          w = tok[k]; kind = ""
          if (tail_strip(w) == "ls-files" || tail_strip(w) == "ls-tree") kind = "GIT"
          else if (cmdword(w) == "find") kind = "FIND"
          if (kind == "") continue
          q = tpos[k] + (kind == "FIND" ? length(w) - 4 : 0)
          if (dead[q]) continue
          b = (q > 1) ? substr(t, q - 1, 1) : " "
          if (b ~ bad) continue
          if (kind == "FIND" && k >= ntok) continue
          printf "%s\t%s\t%s\t%s\n", site(kind, k, t), pathlabel, ln[i], substr(t, 1, 190)
        }
      }
      N = 0
    }
    FNR == 1 {
      emit_file()
      pend = ""; pstart = 0; fb = 0
      split("", repovar)
      pathlabel = (lbl != "" ? lbl : rel(FILENAME))
      # Executability is derived from the blob itself — a .sh name or a shebang — never from a list
      # of filenames, exactly as the corpus scan derives INERT.
      keep = (FILENAME ~ /\.sh$/ || $0 ~ /^#!/)
    }
    keep {
      line = $0
      if (pend != "") { line = pend line; ls = pstart } else { ls = FNR }
      if (line ~ /\\$/) { sub(/\\$/, "", line); pend = line; pstart = ls; next }
      pend = ""
      ln[++N] = ls; tx[N] = line
    }
    END { emit_file() }
  '  "$@"
}

# The live population: every executable blob under tests/ and scripts/ at the scanned REV — the
# committed tree the suite runs against, so an uncommitted scratch file can neither pad the walk
# population nor hide a walker from it. THIS file is then overlaid from its working-tree text, the
# same treatment and the same reason as the self-membership section above.
WROOT="$TMP/walkroot"
walk_records(){
  local f; local -a WFILES
  mkdir -p "$WROOT"
  git -C "$ROOT" archive "$REV" tests scripts | tar -x -C "$WROOT" || return 0
  cp "${BASH_SOURCE[0]}" "$WROOT/$SELF_REL"
  WFILES=()
  while IFS= read -r f; do [ -n "$f" ] || continue; WFILES+=("$f"); done \
    < <(find "$WROOT" -type f | LC_ALL=C sort)
  [ "${#WFILES[@]}" -gt 0 ] || return 0
  walk_classify "$1" "$WROOT/" "" "${WFILES[@]}"
}

# The parent prefix the walkers actually exclude, derived from the literal rather than spelled.
RESULTS_PARENT="${RESULTS_DIR_REL%/*}"
WALK_DECL="$TMP/walk-declarations.tsv"
NO_WALK_DECL="$TMP/no-walk-declarations.tsv"
: > "$WALK_DECL"
: > "$NO_WALK_DECL"
walk_declare(){ printf 'DECLARED\t%s\t%s\n' "$1" "$2" >> "$WALK_DECL"; }

# The one declared reaching walk: section 1 above walks docs/ ON PURPOSE, to prove the prefix limb
# 1 skips holds nothing executable. It reads the TREE LISTING — mode and path — never file content,
# and its predicate is exactly the property whose violation would be a hazard. A results file added
# by Step 6.5 is mode 100644 with an .md suffix, so it is not listed and the verdict is unchanged;
# an EXECUTABLE appearing under the tree would redden it, and that is the detection working.
walk_declare "$SELF_REL" "-- '$RESULTS_PARENT/'"

WALK_RESULT="$(walk_records "$WALK_DECL")"

# POPULATION, measured live at build time (change 0190), never copied from a plan.
#   WALK_FLOOR       the whole derived population. A FLOOR, not an equality, and deliberately so:
#                    an exact total over every fixture walk in the suite would be bumped as
#                    bookkeeping by unrelated tests, which is the evasion this limb exists to deny
#                    (repo learning: guard-remedy-must-not-teach-the-evasion). Coverage is pinned
#                    instead by the REACHING count and by the named probes below.
#   WALK_REACHING    the sites whose scope touches the results chain at all — the population that
#                    actually needs a bound, pinned EXACTLY.
#   WALK_FILTERED / WALK_EXCLUDED / WALK_WRAPPED / WALK_DECLARED
#                    every individual route past the HAZARD default, each pinned on its own so a
#                    swap between two of them cannot keep the aggregate steady.
WALK_FLOOR=80
EXPECTED_WALK_REACHING=2
EXPECTED_WALK_FILTERED=1
EXPECTED_WALK_EXCLUDED=0
EXPECTED_WALK_WRAPPED=1
EXPECTED_WALK_DECLARED=1

n_walk_total="$(grep -c . <<<"$WALK_RESULT" || true)"
n_walk_scoped="$(count_class SCOPED   "$WALK_RESULT")"
n_walk_excluded="$(count_class EXCLUDED "$WALK_RESULT")"
n_walk_filtered="$(count_class FILTERED "$WALK_RESULT")"
n_walk_wrapped="$(count_class WRAPPED  "$WALK_RESULT")"
n_walk_declared="$(count_class DECLARED "$WALK_RESULT")"
n_walk_hazard="$(count_class HAZARD    "$WALK_RESULT")"
n_walk_reaching="$((n_walk_excluded + n_walk_filtered + n_walk_declared + n_walk_hazard))"
printf '  walks:  %s sites — scoped %s, wrapped %s | reaching %s (excluded %s, filtered %s, declared %s) | hazard %s\n' \
  "$n_walk_total" "$n_walk_scoped" "$n_walk_wrapped" \
  "$n_walk_reaching" "$n_walk_excluded" "$n_walk_filtered" "$n_walk_declared" "$n_walk_hazard"

walk_class_at(){ awk -F'\t' -v f="$1" -v l="$2" '$2 == f && $3 == l {print $1}' <<<"$3"; }

assert "the walk derivation reaches at least $WALK_FLOOR tree-walk sites under tests/ and scripts/" \
  '[ "$n_walk_total" -ge "$WALK_FLOOR" ]  || { echo "  found $n_walk_total, floor $WALK_FLOOR." >&2; echo "  A DROP means the derivation went blind, not that the suite stopped walking: check that the rev and the tests/scripts pathspec still resolve, and that the invocation shapes this file recognises still match how the suite spells a walk." >&2; false; }'
# Coverage, not just population: an exact total cannot promise that the two walkers this limb
# EXISTS for are among the sites reached, because the property migrates to whichever site satisfies
# it most cheaply (repo learning: marker-scoped-guard-needs-a-population-floor). So both are named,
# with the class each is supposed to earn.
assert "the derivation reaches test_grep_portability.sh's whole-index walk, and it is FILTERED" \
  '[ "$(walk_class_at tests/test_grep_portability.sh 102 "$WALK_RESULT")" = FILTERED ] || { grep "test_grep_portability" <<<"$WALK_RESULT" >&2; echo "  This is the walk that enumerates every tracked path and drops the results tree only through its docs/ case arm. If it moved, re-find it and re-point the probe; if it lost the filter, that is the finding." >&2; false; }'
assert "the derivation reaches test_comment_anchor_style.sh's pathspec walk, and it is SCOPED" \
  '[ -n "$(awk -F"\t" '"'"'$2 == "tests/test_comment_anchor_style.sh" && $1 == "SCOPED"'"'"' <<<"$WALK_RESULT")" ]'

assert "no whole-tree walk in tests/ or scripts/ can reach a file under the allowlisted results tree" \
  '[ "$n_walk_hazard" = 0 ] || { grep "^HAZARD" <<<"$WALK_RESULT" >&2; echo "  Each line above is a tree walk whose scope reaches the results tree with nothing bounding it — which makes the tree a live content source for a suite scan WITHOUT the scan ever naming it, the shape limb 1 cannot see." >&2; echo "  FIRST work out what the walk feeds: if its output is scanned, a results file added after the build gate can move a verdict and finalize'"'"'s post-gate skip is no longer sound. The fix is to bound the walk — a :! pathspec, a narrower root, a -maxdepth, or a docs-level filter on its output — not to widen this budget." >&2; false; }'

assert "exactly $EXPECTED_WALK_REACHING walk sites reach the results chain at all" \
  '[ "$n_walk_reaching" = "$EXPECTED_WALK_REACHING" ] || { grep -E "^(EXCLUDED|FILTERED|DECLARED|HAZARD)" <<<"$WALK_RESULT" >&2; echo "  found $n_walk_reaching (excluded $n_walk_excluded, filtered $n_walk_filtered, declared $n_walk_declared, hazard $n_walk_hazard), budget $EXPECTED_WALK_REACHING." >&2; echo "  A walk that reaches the tree is the population this limb exists to bound. FIRST establish what the new one enumerates and whether anything reads its output; a walk that does not need to see docs/ at all should be re-scoped rather than admitted." >&2; false; }'
assert "exactly $EXPECTED_WALK_FILTERED walk sites are bounded only by a downstream file-level filter" \
  '[ "$n_walk_filtered" = "$EXPECTED_WALK_FILTERED" ] || { grep "^FILTERED" <<<"$WALK_RESULT" >&2; echo "  found $n_walk_filtered, budget $EXPECTED_WALK_FILTERED." >&2; echo "  FILTERED is the widest pass here: the exclusion is found anywhere in the walker'"'"'s file, not bound to the walk it protects. FIRST open the file and confirm the prefix exclusion really is applied to THIS walk output, and that it sits at or above the results tree. Bounding the walk at its own invocation is the better fix." >&2; false; }'
assert "exactly $EXPECTED_WALK_EXCLUDED walk sites carry an exclusion pathspec at the invocation" \
  '[ "$n_walk_excluded" = "$EXPECTED_WALK_EXCLUDED" ]'
assert "exactly $EXPECTED_WALK_WRAPPED git walks select their repository through a wrapper" \
  '[ "$n_walk_wrapped" = "$EXPECTED_WALK_WRAPPED" ] || { grep "^WRAPPED" <<<"$WALK_RESULT" >&2; echo "  found $n_walk_wrapped, budget $EXPECTED_WALK_WRAPPED." >&2; echo "  A wrapper decides which repository is walked, and this file cannot resolve it. FIRST read the wrapper definition and establish whether the walk can ever run against this repo rather than a fixture; if it can, spell the repository as git -C so the derivation can see it." >&2; false; }'
assert "exactly $EXPECTED_WALK_DECLARED walk sites are hand-declared reaching walks" \
  '[ "$n_walk_declared" = "$EXPECTED_WALK_DECLARED" ] || { grep "^DECLARED" <<<"$WALK_RESULT" >&2; echo "  found $n_walk_declared, budget $EXPECTED_WALK_DECLARED." >&2; echo "  A declaration admits a walk that DOES reach the tree and claims an addition under it cannot change the verdict. FIRST prove that: name the assertion the walk feeds and show that a new results file leaves it identical." >&2; false; }'

walk_orphans=""
while IFS=$'\t' read -r _ dpath dslice; do
  [ -n "${dpath:-}" ] || continue
  awk -F'\t' -v pp="$dpath" '$1 == "DECLARED" && $2 == pp {n++} END {exit !(n > 0)}' <<<"$WALK_RESULT" \
    || walk_orphans="${walk_orphans}${dpath} ${dslice}"$'\n'
done < "$WALK_DECL"
assert "no declared walk is stale (every declaration still matches a live walk site)" \
  '[ -z "$walk_orphans" ] || { printf "  %s" "$walk_orphans" >&2; echo "  The declared walk moved or was rewritten. Re-read it and re-decide, rather than re-pointing the slice." >&2; false; }'

# ---------------------------------------------------------------------------
# 9b. POSITIVE CONTROLS for limb 2. Same shape as section 7: the asserts above are all absence
#     claims, so each class is exercised against throwaway input through the SAME classifier, and
#     each real-file mutation is confirmed to have landed by occurrence count before its result is
#     believed. The synthetic lines are written with printf rather than a heredoc on purpose — a
#     heredoc body is indistinguishable from live code to this file's own self-scan.
# ---------------------------------------------------------------------------
WS1="$TMP/walk-synth-1.sh"
: > "$WS1"
wsynth(){ printf '%s\n' "$2" >> "$1"; }
wsynth "$WS1" "ROOT=\"\$(cd \"\$(dirname \"\${BASH_SOURCE[0]}\")/..\" && pwd -P)\""
wsynth "$WS1" "  find \"\$ROOT\" -type f"
wsynth "$WS1" "  find \"\$ROOT\" -maxdepth 1 -type f"
wsynth "$WS1" "  find \"\$ROOT/$RESULTS_DIR_REL\" -type f"
wsynth "$WS1" "  find \"\$ROOT/skills\" -type f"
wsynth "$WS1" "  find \"\$SBX\" -type f"
wsynth "$WS1" "  git ls-files -z"
wsynth "$WS1" "  git ls-files -- scripts tests ':(glob)*.md'"
wsynth "$WS1" "  git -C \"\$ROOT\" ls-tree -r HEAD -- '$RESULTS_PARENT/'"
wsynth "$WS1" "  git -C \"\$W\" ls-tree -r --name-only origin/main"
wsynth "$WS1" "  landed=\"\$(\$GIT ls-files)\""
WS1_RESULT="$(walk_classify "$NO_WALK_DECL" "" tests/synthetic_walk_probe.sh "$WS1")"
ws1_class(){ awk -F'\t' -v l="$1" '$3 == l {print $1}' <<<"$WS1_RESULT"; }
assert "control: an unbounded find at the repo root classifies HAZARD"        '[ "$(ws1_class 2)" = HAZARD ]'
assert "control: a find bounded by -maxdepth above the tree classifies SCOPED" '[ "$(ws1_class 3)" = SCOPED ]'
assert "control: a find rooted at the results tree itself classifies HAZARD"   '[ "$(ws1_class 4)" = HAZARD ]'
assert "control: a find rooted off the chain classifies SCOPED"               '[ "$(ws1_class 5)" = SCOPED ]'
assert "control: a find rooted in another tree classifies SCOPED"             '[ "$(ws1_class 6)" = SCOPED ]'
assert "control: an unbounded whole-index ls-files classifies HAZARD"         '[ "$(ws1_class 7)" = HAZARD ]'
assert "control: an explicit non-docs pathspec list classifies SCOPED"        '[ "$(ws1_class 8)" = SCOPED ]'
assert "control: a pathspec aimed INTO the results chain classifies HAZARD"   '[ "$(ws1_class 9)" = HAZARD ]'
assert "control: a walk of another repository classifies SCOPED"             '[ "$(ws1_class 10)" = SCOPED ]'
assert "control: a git walk through a wrapper classifies WRAPPED"            '[ "$(ws1_class 11)" = WRAPPED ]'

# The declared route, and the proof that its SLICE is what binds it — a declaration for the right
# file with the wrong slice leaves the site HAZARD and reports itself as stale.
WD_OK="$TMP/walk-decl-ok.tsv"
WD_BAD="$TMP/walk-decl-bad.tsv"
printf 'DECLARED\t%s\t%s\n' tests/synthetic_walk_probe.sh "-- '$RESULTS_PARENT/'"  > "$WD_OK"
printf 'DECLARED\t%s\t%s\n' tests/synthetic_walk_probe.sh "-- 'nomatch/'"       > "$WD_BAD"
WS1_OK="$(walk_classify "$WD_OK" "" tests/synthetic_walk_probe.sh "$WS1")"
WS1_BAD="$(walk_classify "$WD_BAD" "" tests/synthetic_walk_probe.sh "$WS1")"
assert "control: a matching declaration turns that HAZARD into DECLARED" \
  '[ "$(awk -F"\t" '"'"'$3 == 9 {print $1}'"'"' <<<"$WS1_OK")" = DECLARED ]'
assert "control: a declaration whose slice does not match leaves the site HAZARD" \
  '[ "$(awk -F"\t" '"'"'$3 == 9 {print $1}'"'"' <<<"$WS1_BAD")" = HAZARD ] && [ "$(count_class DECLARED "$WS1_BAD")" = 0 ]'

# The exclusion routes, each in its own throwaway file: the file-level filter is file-scoped, so
# these cannot share one fixture without contaminating each other.
WS2="$TMP/walk-synth-2.sh"; : > "$WS2"
wsynth "$WS2" "  git ls-files -- ':!$RESULTS_DIR_REL'"
WS3="$TMP/walk-synth-3.sh"; : > "$WS3"
wsynth "$WS3" "  git ls-files -- ':!$RESULTS_DIR_REL/inner'"
WS4="$TMP/walk-synth-4.sh"; : > "$WS4"
wsynth "$WS4" "  git ls-files -z"
wsynth "$WS4" "    $RESULTS_PARENT/*) continue ;;"
assert "control: an exclusion pathspec at or above the tree classifies EXCLUDED" \
  '[ "$(walk_classify "$NO_WALK_DECL" "" tests/synthetic_walk_probe.sh "$WS2" | cut -f1)" = EXCLUDED ]'
assert "control: an exclusion BELOW the tree buys nothing and still classifies HAZARD" \
  '[ "$(walk_classify "$NO_WALK_DECL" "" tests/synthetic_walk_probe.sh "$WS3" | cut -f1)" = HAZARD ]'
assert "control: an unscoped walk with a docs-level output filter classifies FILTERED" \
  '[ "$(walk_classify "$NO_WALK_DECL" "" tests/synthetic_walk_probe.sh "$WS4" | cut -f1)" = FILTERED ]'

# THE MUTATION THIS LIMB EXISTS FOR. Narrowing test_grep_portability.sh's docs/ exclusion one
# component — to a prefix BELOW the results tree — turns its whole-index walk into a live reader of
# the tree, with limb 1 still reporting hazard 0. Run against a throwaway copy of the real file,
# through the same derivation, and confirmed to have landed by occurrence count first.
GP="$ROOT/tests/test_grep_portability.sh"
MUT_GP="$TMP/mutated_grep_portability.sh"
sed "s|$RESULTS_PARENT/\\*) continue|$RESULTS_PARENT/changes/*) continue|" "$GP" > "$MUT_GP"
gp_before="$(grep -c -F -e "$RESULTS_PARENT/*) continue" "$GP" || true)"
gp_after="$(grep -c -F -e "$RESULTS_PARENT/*) continue" "$MUT_GP" || true)"
gp_narrow="$(grep -c -F -e "$RESULTS_PARENT/changes/*) continue" "$MUT_GP" || true)"
assert "mutation landed: the docs/ walk filter is present before and narrowed after" \
  '[ "$gp_before" -ge 1 ] && [ "$gp_after" = 0 ] && [ "$gp_narrow" -ge 1 ]'
GP_CLEAN="$(walk_classify "$NO_WALK_DECL" "" tests/test_grep_portability.sh "$GP")"
GP_MUT="$(walk_classify "$NO_WALK_DECL" "" tests/test_grep_portability.sh "$MUT_GP")"
assert "control: the UNMUTATED file reports no hazard (the mutation is what reddens, not the extractor)" \
  '[ "$(count_class HAZARD "$GP_CLEAN")" = 0 ] && [ "$(count_class FILTERED "$GP_CLEAN")" -ge 1 ]'
assert "control: narrowing the filter below the results tree is REPORTED as a HAZARD walk" \
  '[ "$(count_class HAZARD "$GP_MUT")" -ge 1 ]'

# The same, for the other invisible shape: test_comment_anchor_style.sh excludes docs/ only by
# never naming it, so widening its pathspec is the way that walk goes bad.
CAS="$ROOT/tests/test_comment_anchor_style.sh"
MUT_CAS="$TMP/mutated_comment_anchor_style.sh"
sed "s|ls-files -- scripts|ls-files -- $RESULTS_PARENT scripts|" "$CAS" > "$MUT_CAS"
cas_before="$(grep -c -F -e "ls-files -- $RESULTS_PARENT scripts" "$CAS" || true)"
cas_after="$(grep -c -F -e "ls-files -- $RESULTS_PARENT scripts" "$MUT_CAS" || true)"
assert "mutation landed: the pathspec gained docs/ in the throwaway copy only" \
  '[ "$cas_before" = 0 ] && [ "$cas_after" -ge 1 ]'
CAS_MUT="$(walk_classify "$NO_WALK_DECL" "" tests/test_comment_anchor_style.sh "$MUT_CAS")"
assert "control: widening the pathspec to docs/ is REPORTED as a HAZARD walk" \
  '[ "$(count_class HAZARD "$CAS_MUT")" -ge 1 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
