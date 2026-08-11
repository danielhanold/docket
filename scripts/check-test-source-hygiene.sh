#!/usr/bin/env bash
# scripts/check-test-source-hygiene.sh — refuse test source whose backticks the shell would
# EXECUTE (change 0221). Reads source. Never sources, runs, or otherwise evaluates a scanned file.
#
# WHY THIS EXISTS. A backtick in a test file runs when the SHELL READS THE LINE — before `assert`
# is ever called, before any helper prints anything. So a verbatim-quoted guard anchor pasted into
# a test description is not data, it is a command, and the test then reports green on output it
# produced itself. Change 0212 shipped exactly that: a multi-line double-quoted `SITES="…"` block
# whose anchor text carried `git checkout .`.
#
# The executing vector is SOURCE EVALUATION, plus a second one at `eval "$2"`. It is NOT the
# helper's print: parameter expansion does not re-trigger command substitution, so a backtick
# already sitting in a variable's value is inert through `printf '%s' "$1"`. Normalizing the
# helpers is drift control; this scanner is the safety mechanism. Do not write a comment or a test
# name claiming otherwise.
#
# ---- THE RULES ---------------------------------------------------------------------------------
#
# (b) QUOTING SCAN — a whole-file shell-quoting state machine over every path handed to this
#     script. State is carried ACROSS lines, not reset per line: the 0212 incident lived inside a
#     multi-line double-quoted assignment, which a line-local scanner cannot see. Four classes,
#     each with a probe or an incident behind it:
#
#       NORMAL-BACKTICK   legacy `cmd` substitution in unquoted code position. Runs at source
#                         evaluation. `$(cmd)` is the house style and the tree has no live use.
#       DQ-BACKTICK       any backtick inside double quotes — BARE or BACKSLASH-ESCAPED. Bare, it
#                         runs at source evaluation. Escaped, the escape is CONSUMED at source
#                         evaluation, so what reaches `$2` is a bare backtick and it runs one
#                         evaluation later, at `eval`. One class for both spellings deliberately:
#                         the escaped form is not an accepted residual, it is the same defect
#                         displaced in time.
#       HEREDOC-BACKTICK  a backtick in a heredoc body whose delimiter is UNQUOTED. `<<EOF` and
#                         `<<-EOF` substitute in the body; `<<'EOF'`, `<<"EOF"` and `<<\EOF` do not.
#       EVAL-BACKTICK     a backtick in the single-quoted CONDITION (argument 2) of an
#                         assert-family call that is not immediately preceded by a backslash.
#                         Source quoting protects the FIRST evaluation only; `eval "$2"` re-parses
#                         the value and runs it there. The house idiom — a backslash-escaped
#                         backtick inside the condition — survives `eval` as a literal and stays
#                         legal.
#
# (a) DEFINITION ALLOWLIST — one class, `DEFN-DRIFT`. Every assert-family definition must match one
#     of the canonical forms BYTE FOR BYTE. Normalizing the whole tests tree onto those bytes is
#     what buys an anchor this strict; softening the comparison into a fuzzy or a substring match
#     gives the strictness away for nothing.
#
#       DEFN-DRIFT        an assert-family definition that is not byte-for-byte canonical.
#
#     DISCOVERY AND VERDICT ARE TWO DIFFERENT MECHANISMS, deliberately. Discovery is SHAPE-tolerant
#     — one-line, spaced (`assert () {`), function-keyword (`function assert {`), multiline, and
#     brace-on-the-next-line all get found, and a definition counts as assert-family by what its
#     body DOES (evals argument 2, or prints a runner-contract result marker), never by a list of
#     helper names. The verdict is then byte-exact. Conflating the two defeats the guard: a drifted
#     spelling must not be able to dodge the allowlist by dodging the census. Narrowing the
#     declaration shape was probed — `function assert { … }` and `assert () { … }` both go silently
#     green the moment discovery stops tolerating their spelling.
#
#     ITS SCOPE IS THE TESTS TREE, NOT THE PATH LIST. Rule (a) additionally sweeps every
#     `tests/**/*.sh` the caller did NOT pass, because run-tests.sh hands the preflight only its
#     `tests/test_*.sh` targets while tests/lib/gate_run_common.sh,
#     tests/lib/runner_dispatch_detach_common.sh and tests/lib/sync_agents_common.sh each define an
#     assert helper outside that glob. Trusting the caller list would leave those permanently
#     unguarded. tests/fixtures/ is excluded from the sweep — its red half is drifted on purpose,
#     and is a verdict only when a caller names one of those files explicitly.
#
# ---- HOW THE HELPER NAMES ARE DERIVED, NOT ENUMERATED -------------------------------------------
#
# The EVAL-BACKTICK rule needs to know which command words take an eval-ed condition as argument 2.
# That set is DERIVED per file, keyed on shape: any function whose definition body contains
# `eval "$2"` contributes its own name, whatever it is spelled. `assert` is seeded as a floor,
# because a file may CALL the helper without defining it (every fixture under
# tests/fixtures/hygiene/ does). A hand-list of helper names would fail on the first file whose
# house idiom differs (AGENTS.md § Guards and tests).
#
# ---- WHAT IT PROTECTS, AND WHAT IT DOES NOT -----------------------------------------------------
#
#   - SUITE RUNS ONLY. `scripts/run-tests.sh` calls this synchronously over its targets before the
#     first job launches, so a violation aborts a suite run with zero test files executed. A file
#     run directly — `bash tests/test_x.sh` — BYPASSES the preflight entirely. Accepted residual,
#     taken rather than paying for a preamble in 100+ test files.
#   - SOURCE, NOT VALUES. A condition assembled at runtime (`cond="grep $pat"; assert "d" "$cond"`)
#     is not modeled and cannot be: the bytes `eval` finally sees do not exist until the test runs.
#     Same for a helper reached through a variable, an alias, or `$@`.
#   - `$(…)` IS a fresh quoting context, and modeling it is not optional polish. Without it the
#     everyday `var="$(awk -v q="X" …)"` shape reads the inner opening quote as the CLOSING quote
#     of the outer one, and the machine runs INVERTED from there to end of file — which loses real
#     violations exactly as readily as it invents false ones. It was measured, not theorized:
#     tests/test_sync_agents_runners.sh desynced at its `_between="$(awk -v q="…"` line and
#     reported 61 phantom heredoc hits over the following 300 lines.
#     Its residual is the reverse: a `)` that is not a real close-paren pops the frame early. A
#     `case` pattern label inside a command substitution is the shape that would do it; the tests
#     tree has none today, and one would surface as a burst of misclassified hits — loud — rather
#     than as a silent miss.
#   - `<<X` is read as a heredoc only in unquoted code position. `<<<X` is a here-string and is
#     never consumed as one (the suite has ~2000 of them). Arithmetic left-shift inside `$(( … ))`
#     is not modeled; the tests tree has no such site, and one would show up as a phantom heredoc
#     rather than as a silent miss.
#   - IT CANNOT FLAG ITSELF, by construction rather than by an exception entry: the scanner writes
#     no literal backtick in executable position anywhere — every one it needs is built with
#     `sprintf("%c", 96)` — and its awk program lives in a single-quoted literal.
#
# Usage: check-test-source-hygiene.sh <path>...
#   Prints one  <path>:<line>: <CLASS>: <message>  line per violation to stdout, at most one per
#   (line, class) pair, in line order within each file. The files come in the order given, followed
#   by rule (a)-only reports for the swept tests-tree files the caller did not pass, named by
#   absolute path.
#   Exit: 0 clean; 1 violations found; 2 usage error (no paths given, an unknown flag, or a path
#   that is not a readable regular file).
#
# Fixtures: tests/fixtures/hygiene/red/*.sh and tests/fixtures/hygiene/green/*.sh are the
# executable specification of these rules, and tests/test_assert_hygiene.sh drives them in both
# directions. They sit outside the runner's `tests/test_*.sh` glob deliberately, so a red fixture
# is never launched as a test — and a caller that scans the whole tests tree must exclude
# tests/fixtures/, whose red half is red on purpose.
set -uo pipefail

# Repo root, for rule (a)'s own sweep of the tests tree (see the sweep block near the bottom).
REPO_ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd -P)"

PATHS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    --) shift; break ;;
    -*) printf 'check-test-source-hygiene: unknown argument: %s\n' "$1" >&2; exit 2 ;;
    *) PATHS+=("$1"); shift ;;
  esac
done
while [ $# -gt 0 ]; do PATHS+=("$1"); shift; done

if [ "${#PATHS[@]}" -eq 0 ]; then
  printf 'check-test-source-hygiene: no paths given\n' >&2
  printf 'usage: check-test-source-hygiene.sh <path>...\n' >&2
  exit 2
fi

# ---- the state machine ---------------------------------------------------------------------
# SINGLE-QUOTED, and it must stay that way: this scanner scans a tree that includes itself, so any
# character it treats as special has to be inert where it is written. Two consequences a later
# editor has to honor:
#   - NO APOSTROPHE may appear anywhere in the program, not even in prose. One would close this
#     literal and hand the remainder to the shell. (An odd count fails loudly at parse time; an
#     even count would splice shell-interpreted text into the program, which would not.) The long
#     prose therefore lives in the header above, outside the literal.
#   - NO LITERAL BACKTICK either. `sprintf("%c", 96)` builds it. A backslash-escaped backtick in an
#     awk string is an undefined escape: gawk warns ("escape sequence \` treated as plain `"), and
#     POSIX does not define it.
HYG_AWK='
BEGIN {
  BT = sprintf("%c", 96); BS = sprintf("%c", 92)
  Q1 = sprintf("%c", 39); Q2 = sprintf("%c", 34); TB = sprintf("%c", 9)
  NORMAL = 0; SQ = 1; DQ = 2
  # Per-file state lives in file_reset(), which runs at every file boundary; only the constants
  # below are set once here.
  # Words after which the NEXT word is still a command word, so a helper name following one of
  # them is recognized rather than counted as an argument.
  kw["then"] = 1; kw["do"] = 1; kw["else"] = 1; kw["elif"] = 1; kw["if"] = 1
  kw["while"] = 1; kw["until"] = 1; kw["time"] = 1; kw["!"] = 1
  M_NORMAL = "legacy backtick substitution in code position - the shell runs it when it reads this line; write it as $(...)"
  M_DQ = "backtick inside double quotes - bare, it runs at source evaluation; backslash-escaped, the escape is consumed there and a BARE backtick reaches eval. Carry the text in single quotes or a quoted-delimiter heredoc"
  M_HD = "backtick in a heredoc body whose delimiter is unquoted - the body substitutes; quote the delimiter"
  M_EV = "unescaped backtick in an assert condition - single quotes protect the first evaluation only, eval re-parses the value and runs it; escape the backtick"
  M_DEFN = "assert-family definition is not byte-for-byte one of the canonical forms - see rule (a) in scripts/check-test-source-hygiene.md"
  build_allowlist()
}

# ---- file boundaries ---------------------------------------------------------------------------
# The program accepts MANY files in one invocation, and every piece of per-file state is torn down
# at each boundary, so an N-file run is N independent single-file runs concatenated. That is what
# lets the rule (a) sweep batch its whole file list into one process instead of forking awk once
# per tests-tree file. Two things this reset must keep true, or batching silently corrupts verdicts:
#   - EVERY global the passes touch is cleared here. The line buffer and its length, the report
#     buffer and its dedupe key set, the rule (b) lexer state, the heredoc and context stacks, and
#     the DERIVED helper-name set (which is per-file by construction - see derive_names).
#   - Reports are FLUSHED at the boundary, not at END, so output stays grouped and line-ordered per
#     file and the files print in the order they were given.
# split("", a) is the portable whole-array clear; delete a is not in every awk this has to run on.
FNR == 1 { if (NR > 1) file_end(); file_reset() }

{ L[FNR] = $0; nlines = FNR }

END { if (NR > 0) file_end() }

function file_reset() {
  # -v path=... names a single file explicitly; a batch run leaves it unset and takes FILENAME.
  cur_path = (path != "" ? path : FILENAME)
  nlines = 0
  split("", L)
  split("", seen); nrep = 0; split("", rep_l); split("", rep_t)
  st = NORMAL; esc = 0
  hn = 0; hact = 0; depth = 0
  split("", hd_delim); split("", hd_dash); split("", hd_live)
  split("", sv_st); split("", sv_widx); split("", sv_inword); split("", sv_armed); split("", sv_wtxt)
  widx = -1; inword = 0; armed = 0; wtxt = ""
  # Floor for a file that CALLS the helper without defining it. derive_names() adds the rest.
  split("", names); names["assert"] = 1
}

function file_end(   i) {
  if (do_defn) defn_pass()
  if (do_quote) {
    derive_names()
    for (i = 1; i <= nlines; i++) line_pass(L[i], i)
  }
  flush_reports()
}

# ---- rule (a): the canonical definition allowlist ----------------------------------------------
# The WHOLE legal set, in one block, so a reader sees it at once. Assembled rather than written
# out because the canonical forms carry apostrophes, which cannot appear in this literal - see the
# note above the program. The verdict against this set is BYTE-EXACT: normalizing 88 definitions
# in the tests tree is what buys a comparison this strict, so it must not be softened into a
# fuzzy or a substring match.
function build_allowlist(   NLE, P_OK, P_NOK, EV) {
  NLE = BS "n"
  P_OK  = "printf " Q1 "ok - %s" NLE Q1 " " Q2 "$1" Q2
  P_NOK = "printf " Q1 "NOT OK - %s" NLE Q1 " " Q2 "$1" Q2
  EV = "eval " Q2 "$2" Q2
  allow["assert(){ if " EV "; then " P_OK "; else " P_NOK "; fail=1; fi; }"] = 1
  allow["assert(){ if ( " EV " ); then " P_OK "; else " P_NOK "; fail=1; fi; }"] = 1
  allow["assert(){ if " EV "; then " P_OK "; else " P_NOK "; fails=$((fails+1)); fi; }"] = 1
  allow["ok(){ " P_OK "; }"] = 1
  allow["no(){ " P_NOK "; fail=1; }"] = 1
  allow["nok(){ " P_NOK "; fail=1; }"] = 1
}

# Rule (a). DISCOVERY is shape-tolerant and VERDICT is byte-exact, and they are deliberately two
# different mechanisms: a drifted spelling must not dodge the allowlist by dodging the census.
# So discovery keys on the syntax of a function declaration - one-line, spaced, function-keyword,
# multiline, or brace-on-the-next-line - and on what the body DOES (evals argument 2, or prints a
# runner-contract result marker), never on a list of helper names.
function defn_pass(   i, j, s, blk, d, t) {
  for (i = 1; i <= nlines; i++) {
    s = L[i]
    if (s ~ /^[[:space:]]*#/) continue
    j = i
    if (s ~ /^[[:space:]]*(function[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*[[:space:]]*([(][)])?[[:space:]]*[{]/) {
      blk = s
    } else if (i < nlines && L[i+1] ~ /^[[:space:]]*[{]/ &&
               s ~ /^[[:space:]]*(function[[:space:]]+[A-Za-z_][A-Za-z0-9_]*([[:space:]]*[(][)])?|[A-Za-z_][A-Za-z0-9_]*[[:space:]]*[(][)])[[:space:]]*$/) {
      j = i + 1; blk = s "\n" L[j]
    } else continue
    d = braces(blk)
    # A body that never balances is capped rather than run to end of file; the cap only ever makes
    # a definition LONGER than the allowlist entries, so it cannot turn a drifted body green.
    while (d > 0 && j < nlines && (j - i) < 40) { j++; blk = blk "\n" L[j]; d += braces(L[j]) }
    if (is_family(blk)) {
      t = blk
      sub(/^[[:space:]]+/, "", t)
      if (!(t in allow)) report(i, "DEFN-DRIFT", M_DEFN)
    }
    i = j
  }
}

# Assert-family by behavior, not by name: it evals argument 2, or it emits one of the runner
# result markers that scripts/run-tests.sh accounts on.
function is_family(b) {
  if (index(b, "eval " Q2 "$2" Q2) > 0) return 1
  if (index(b, "NOT OK") > 0) return 1
  if (b ~ /[^A-Za-z]ok[[:space:]]+-[[:space:]]/) return 1
  if (b ~ /[^A-Za-z]FAIL[[:space:]]+-[[:space:]]/) return 1
  return 0
}

function braces(s,   k, n, c, d) {
  n = length(s); d = 0
  for (k = 1; k <= n; k++) {
    c = substr(s, k, 1)
    if (c == "{") d++
    else if (c == "}") d--
  }
  return d
}

# Shape-keyed discovery of the eval-ed helpers defined IN THIS FILE: remember the name opened by
# any function-declaration shape, and claim it when a later line inside that body evals argument 2.
# Covers the one-line and the multi-line spellings, and the function-keyword and spaced ones.
function derive_names(   i, s, name, pend) {
  pend = ""
  for (i = 1; i <= nlines; i++) {
    s = L[i]
    if (s ~ /^[[:space:]]*#/) continue
    if (s ~ /^[[:space:]]*(function[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*[[:space:]]*([(][)])?[[:space:]]*[{]/) {
      name = s
      sub(/^[[:space:]]+/, "", name)
      sub(/^function[[:space:]]+/, "", name)
      sub(/[^A-Za-z0-9_].*$/, "", name)
      if (name != "") pend = name
    }
    if (pend != "" && index(s, "eval " Q2 "$2" Q2) > 0) { names[pend] = 1; pend = "" }
  }
}

function line_pass(s, lno,   t) {
  if (hact) {
    t = s
    if (hd_dash[1]) { while (substr(t, 1, 1) == TB) t = substr(t, 2) }
    if (t == hd_delim[1]) { hd_shift(); return }
    if (hd_live[1]) hd_scan(s, lno)
    return
  }
  scan(s, lno)
  # A heredoc body starts on the line AFTER the operator, so activation waits for the line to end.
  if (hn > 0) hact = 1
}

# A live heredoc body is not shell-quoted text: no quote transitions apply inside it, only
# backslash escaping.
function hd_scan(s, lno,   i, n, c) {
  n = length(s); i = 1
  while (i <= n) {
    c = substr(s, i, 1)
    if (c == BS) { i += 2; continue }
    if (c == BT) { report(lno, "HEREDOC-BACKTICK", M_HD); return }
    i++
  }
}

function hd_shift(   k) {
  for (k = 1; k < hn; k++) {
    hd_delim[k] = hd_delim[k+1]; hd_dash[k] = hd_dash[k+1]; hd_live[k] = hd_live[k+1]
  }
  delete hd_delim[hn]; delete hd_dash[hn]; delete hd_live[hn]
  hn--
  if (hn == 0) hact = 0
}

function scan(s, lno,   i, n, c, d) {
  n = length(s); i = 1
  while (i <= n) {
    c = substr(s, i, 1)
    # A backslash-newline is a SPLICE, not an escape: both characters are removed and the character
    # here is ORDINARY. So this clears the carried flag and consumes NOTHING - the character falls
    # through to the branches below and is lexed on its own merits. Consuming it instead swallows
    # the house indent (opening a spurious word, which pushes an assert condition from index 2 to
    # index 3 and disarms the eval rule) or, at column zero, swallows an opening quote - and then
    # the CLOSING quote opens a region and the machine runs inverted to end of file.
    if (esc) esc = 0

    if (st == SQ) {
      # Nothing escapes inside single quotes - not even a backslash. Only a closing quote leaves.
      if (c == Q1) { st = NORMAL; i++; continue }
      if (c == BT && armed && widx == 2 && substr(s, i-1, 1) != BS) report(lno, "EVAL-BACKTICK", M_EV)
      i++; continue
    }

    if (st == DQ) {
      if (c == BS) {
        d = substr(s, i+1, 1)
        if (d == "") { esc = 1; i++; continue }
        if (d == BT) report(lno, "DQ-BACKTICK", M_DQ)
        i += 2; continue
      }
      # A command substitution restarts quoting from scratch, even inside double quotes. Modeling
      # it is not precision for its own sake: without it, the very common
      # var="$(awk -v q="X" ...)" shape reads the inner opening quote as the CLOSING quote of the
      # outer one, and the machine then runs inverted for the rest of the file - which loses real
      # violations as readily as it invents false ones.
      if (c == "$" && substr(s, i+1, 1) == "(") { push_ctx(NORMAL); i += 2; continue }
      if (c == BT) { report(lno, "DQ-BACKTICK", M_DQ); i++; continue }
      if (c == Q2) { st = NORMAL; i++; continue }
      i++; continue
    }

    # NORMAL
    if (c == BS) {
      d = substr(s, i+1, 1)
      if (d == "") { esc = 1; i++; continue }
      word_char(""); i += 2; continue
    }
    # A hash starts a comment only where it BEGINS a word, which is exactly where no word is open.
    if (c == "#" && !inword) break
    if (c == Q1) { word_char(""); st = SQ; i++; continue }
    if (c == Q2) { word_char(""); st = DQ; i++; continue }
    if (c == BT) { report(lno, "NORMAL-BACKTICK", M_NORMAL); word_char(""); i++; continue }
    if (c == "$" && substr(s, i+1, 1) == "(") { word_char(""); push_ctx(NORMAL); i += 2; continue }
    if (c == "$" && substr(s, i+1, 1) == "{") { i = brace_expand(s, i, lno); continue }
    if (c == "<" && substr(s, i+1, 1) == "<") {
      if (substr(s, i+2, 1) == "<") { word_char(""); i += 3; continue }
      i = queue_heredoc(s, i); continue
    }
    if (c == " " || c == TB) { word_end(); i++; continue }
    # A plain parenthesis gets a frame of its own so that a subshell, a function header, or an
    # arithmetic (( )) nested inside a command substitution cannot pop that substitution early.
    if (c == "(") { word_end(); push_ctx(NORMAL); i++; continue }
    if (c == ")") {
      if (depth > 0) { pop_ctx(); i++; continue }
      word_end(); cmd_reset(); i++; continue
    }
    if (c == ";" || c == "|" || c == "&" || c == "{" || c == "}") {
      word_end(); cmd_reset(); i++; continue
    }
    word_char(c); i++
  }
  # End of the physical line. A line that ends inside a quote, or on an escaped newline, is one
  # logical line with the next - so the command, its word index, and the arm survive.
  if (st == NORMAL && !esc) { word_end(); cmd_reset() }
}

# A ${...} expansion is WORD TEXT, and consuming it here is what keeps its braces out of the
# separator branch. Without this, ${#FILES[@]} closes the word at its opening brace, the very next
# character is a hash, that reads as a comment start, and the REST OF THE PHYSICAL LINE goes
# unscanned - a silent miss of every hazard written after an unquoted length expansion, which is a
# shape the live suite already writes (tests/test_comment_anchor_style.sh,
# tests/test_grep_portability.sh). Its own brace counter tracks nesting, and quoted spans are
# skipped so a closing brace inside one does not end the expansion early. The quoting inside an
# expansion is real quoting, so a backtick unquoted or double-quoted in there is still reported;
# only a single-quoted one is inert, exactly as the shell reads it.
function brace_expand(s, i, lno,   n, c, q, bdepth) {
  n = length(s); word_char(""); i += 2; bdepth = 1
  while (i <= n) {
    c = substr(s, i, 1)
    if (c == BS) {
      if (substr(s, i+1, 1) == "") { esc = 1; return i + 1 }
      i += 2; continue
    }
    if (c == Q1 || c == Q2) {
      q = c; i++
      while (i <= n) {
        c = substr(s, i, 1)
        if (c == q) { i++; break }
        if (q == Q2 && c == BS) { i += 2; continue }
        if (q == Q2 && c == BT) report(lno, "DQ-BACKTICK", M_DQ)
        i++
      }
      continue
    }
    if (c == BT) { report(lno, "NORMAL-BACKTICK", M_NORMAL); i++; continue }
    if (c == "{") { bdepth++; i++; continue }
    if (c == "}") { bdepth--; i++; if (bdepth == 0) return i; continue }
    i++
  }
  return i
}

# Consumes the operator and its delimiter word, returning the index just past it. Any quoting
# anywhere in the delimiter - single, double, or a backslash - makes the body inert.
function queue_heredoc(s, i,   j, n, dash, c, delim, quoted) {
  word_end()
  n = length(s); j = i + 2; dash = 0; delim = ""; quoted = 0
  if (substr(s, j, 1) == "-") { dash = 1; j++ }
  while (j <= n && (substr(s, j, 1) == " " || substr(s, j, 1) == TB)) j++
  while (j <= n) {
    c = substr(s, j, 1)
    if (c == " " || c == TB || c == ";" || c == "&" || c == "|" || c == "<" || c == ">" || c == "(" || c == ")") break
    if (c == Q1) {
      quoted = 1; j++
      while (j <= n && substr(s, j, 1) != Q1) { delim = delim substr(s, j, 1); j++ }
      j++; continue
    }
    if (c == Q2) {
      quoted = 1; j++
      while (j <= n && substr(s, j, 1) != Q2) { delim = delim substr(s, j, 1); j++ }
      j++; continue
    }
    if (c == BS) {
      quoted = 1; j++
      if (j <= n) { delim = delim substr(s, j, 1); j++ }
      continue
    }
    delim = delim c; j++
  }
  if (delim == "") return j
  hn++
  hd_delim[hn] = delim; hd_dash[hn] = dash; hd_live[hn] = (quoted ? 0 : 1)
  return j
}

# Word index within the current simple command: 0 is the command word, 1 the description, 2 the
# condition. A word may be built from adjacent quoted and unquoted pieces, so only unquoted
# whitespace and command separators end one.
function word_char(c) {
  if (!inword) { inword = 1; widx++; if (widx == 0) wtxt = "" }
  if (widx == 0 && c != "") wtxt = wtxt c
}

function word_end() {
  if (!inword) return
  inword = 0
  if (widx == 0) {
    if (wtxt in kw) { widx = -1; return }
    armed = (wtxt in names)
  }
}

function cmd_reset() { widx = -1; inword = 0; armed = 0; wtxt = "" }

# A nested quoting context: a command substitution or a parenthesized group. The quote state AND
# the word/argument tracking are both saved, so a helper call inside a substitution cannot shift
# the argument index of the enclosing call, and the enclosing word resumes where it left off.
function push_ctx(newst) {
  depth++
  sv_st[depth] = st
  sv_widx[depth] = widx; sv_inword[depth] = inword
  sv_armed[depth] = armed; sv_wtxt[depth] = wtxt
  st = newst
  cmd_reset()
}

function pop_ctx() {
  if (depth <= 0) return
  st = sv_st[depth]
  widx = sv_widx[depth]; inword = sv_inword[depth]
  armed = sv_armed[depth]; wtxt = sv_wtxt[depth]
  depth--
}

# At most one report per (line, class): two backticks delimiting one substitution are one defect.
# Buffered rather than printed on sight, because rule (a) runs as its own pass over the file and
# would otherwise interleave out of line order with rule (b).
function report(lno, cls, msg,   key) {
  key = lno ":" cls
  if (key in seen) return
  seen[key] = 1
  nrep++
  rep_l[nrep] = lno
  rep_t[nrep] = sprintf("%s:%d: %s: %s", cur_path, lno, cls, msg)
}

# Insertion sort - stable, so two classes reported on the same line keep discovery order.
function flush_reports(   i, j, tl, tt) {
  for (i = 2; i <= nrep; i++) {
    tl = rep_l[i]; tt = rep_t[i]; j = i - 1
    while (j >= 1 && rep_l[j] > tl) { rep_l[j+1] = rep_l[j]; rep_t[j+1] = rep_t[j]; j-- }
    rep_l[j+1] = tl; rep_t[j+1] = tt
  }
  for (i = 1; i <= nrep; i++) print rep_t[i]
}
'

found=0

# One awk invocation per CALLER-NAMED path. The full rule (a) + rule (b) scan is what a caller asks
# for, the list is short, and a per-file process gives per-file error attribution for free. The
# tests-tree sweep below does NOT use this — see the batching note there.
# LC_ALL=C pins byte semantics, so substr() and length() agree regardless of the ambient locale and
# of which awk is installed.
hyg_scan() {
  _p="$1"; _q="$2"; _d="$3"
  if ! _out="$(LC_ALL=C awk -v path="$_p" -v do_quote="$_q" -v do_defn="$_d" "$HYG_AWK" "$_p")"; then
    printf 'check-test-source-hygiene: scanner failed on %s\n' "$_p" >&2
    exit 2
  fi
  if [ -n "$_out" ]; then printf '%s\n' "$_out"; found=1; fi
}

NL='
'
scanned=""
for p in "${PATHS[@]}"; do
  if [ ! -f "$p" ] || [ ! -r "$p" ]; then
    printf 'check-test-source-hygiene: not a readable file: %s\n' "$p" >&2
    exit 2
  fi
  case "$p" in /*) abs="$p" ;; *) abs="$PWD/${p#./}" ;; esac
  scanned="$scanned$abs$NL"
  hyg_scan "$p" 1 1
done

# ---- rule (a) discovers definitions on its own -------------------------------------------------
# Scope for rule (a) is the whole tests tree, NOT the path list the caller passed. run-tests.sh
# hands the preflight its target list, which is tests/test_*.sh — and tests/lib/gate_run_common.sh,
# tests/lib/runner_dispatch_detach_common.sh and tests/lib/sync_agents_common.sh each define an
# assert helper without matching that glob. Trusting the caller list would leave those three
# definitions permanently unguarded, so the sweep below covers whatever the list missed.
# tests/fixtures/ is excluded: its red half is drifted ON PURPOSE and is only ever a verdict when
# a caller names one of those files explicitly.
#
# IT IS ONE AWK PROCESS FOR THE WHOLE SWEEP, not one per file, and that is a cost decision with a
# safety argument behind it. The sweep is unconditional and its list is the whole tests tree, so a
# process per file made the scan O(#test files) on EVERY invocation — and tests/test_run_tests.sh
# invokes the runner ~35 times, so it paid that product. The argument that batching is safe: the
# per-file-process rule protects rule (b), whose heredoc and quote state is carried across lines
# and must not bleed across a file boundary. The sweep runs do_quote=0, and rule (a) reads only the
# buffered lines of the file it is scanning — it carries no cross-line lexer state at all — so a
# file-boundary reset is sufficient here. The program does that reset in file_reset(); if you ever
# give the sweep do_quote=1, that reset is what has to be right, not this loop.
if [ -d "$REPO_ROOT/tests" ]; then
  tree_list="$(find "$REPO_ROOT/tests" -type f -name '*.sh' | sort)"
  sweep=()
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    case "$f" in "$REPO_ROOT/tests/fixtures/"*) continue ;; esac
    case "$NL$scanned" in *"$NL$f$NL"*) continue ;; esac
    sweep+=("$f")
  done <<<"$tree_list"
  if [ "${#sweep[@]}" -gt 0 ]; then
    # No -v path: a batch run takes each report path from FILENAME, which is the absolute path
    # find produced — the same string the per-file call used to pass.
    if ! sweep_out="$(LC_ALL=C awk -v do_quote=0 -v do_defn=1 "$HYG_AWK" "${sweep[@]}")"; then
      printf 'check-test-source-hygiene: scanner failed during the tests-tree sweep\n' >&2
      exit 2
    fi
    if [ -n "$sweep_out" ]; then printf '%s\n' "$sweep_out"; found=1; fi
  fi
fi

exit "$found"
