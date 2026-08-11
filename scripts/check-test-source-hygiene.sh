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
# (a) DEFINITION ALLOWLIST — NOT IMPLEMENTED IN THIS FILE YET. It lands in the same change and adds
#     exactly one class, `DEFN-DRIFT`; nothing in the interface below changes when it does.
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
#   (line, class) pair, in line order.
#   Exit: 0 clean; 1 violations found; 2 usage error (no paths given, an unknown flag, or a path
#   that is not a readable regular file).
#
# Fixtures: tests/fixtures/hygiene/red/*.sh and tests/fixtures/hygiene/green/*.sh are the
# executable specification of these rules, and tests/test_assert_hygiene.sh drives them in both
# directions. They sit outside the runner's `tests/test_*.sh` glob deliberately, so a red fixture
# is never launched as a test — and a caller that scans the whole tests tree must exclude
# tests/fixtures/, whose red half is red on purpose.
set -uo pipefail

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
  st = NORMAL; esc = 0
  hn = 0; hact = 0; depth = 0
  widx = -1; inword = 0; armed = 0; wtxt = ""
  # Floor for a file that CALLS the helper without defining it. derive_names() adds the rest.
  names["assert"] = 1
  # Words after which the NEXT word is still a command word, so a helper name following one of
  # them is recognized rather than counted as an argument.
  kw["then"] = 1; kw["do"] = 1; kw["else"] = 1; kw["elif"] = 1; kw["if"] = 1
  kw["while"] = 1; kw["until"] = 1; kw["time"] = 1; kw["!"] = 1
  M_NORMAL = "legacy backtick substitution in code position - the shell runs it when it reads this line; write it as $(...)"
  M_DQ = "backtick inside double quotes - bare, it runs at source evaluation; backslash-escaped, the escape is consumed there and a BARE backtick reaches eval. Carry the text in single quotes or a quoted-delimiter heredoc"
  M_HD = "backtick in a heredoc body whose delimiter is unquoted - the body substitutes; quote the delimiter"
  M_EV = "unescaped backtick in an assert condition - single quotes protect the first evaluation only, eval re-parses the value and runs it; escape the backtick"
}

{ L[NR] = $0 }

END {
  derive_names()
  for (i = 1; i <= NR; i++) line_pass(L[i], i)
}

# Shape-keyed discovery of the eval-ed helpers defined IN THIS FILE: remember the name opened by
# any function-declaration shape, and claim it when a later line inside that body evals argument 2.
# Covers the one-line and the multi-line spellings, and the function-keyword and spaced ones.
function derive_names(   i, s, name, pend) {
  pend = ""
  for (i = 1; i <= NR; i++) {
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
    # A backslash at end of line escapes the newline; the escaped character is the first one here.
    if (esc) { esc = 0; if (st == NORMAL) word_char(""); i++; continue }

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
function report(lno, cls, msg,   key) {
  key = lno ":" cls
  if (key in seen) return
  seen[key] = 1
  printf "%s:%d: %s: %s\n", path, lno, cls, msg
}
'

found=0
for p in "${PATHS[@]}"; do
  if [ ! -f "$p" ] || [ ! -r "$p" ]; then
    printf 'check-test-source-hygiene: not a readable file: %s\n' "$p" >&2
    exit 2
  fi
  # One awk invocation per file: heredoc and quote state must not bleed across a file boundary,
  # and a per-file program cannot get that wrong. LC_ALL=C pins byte semantics, so substr() and
  # length() agree regardless of the ambient locale and of which awk is installed.
  if ! out="$(LC_ALL=C awk -v path="$p" "$HYG_AWK" "$p")"; then
    printf 'check-test-source-hygiene: scanner failed on %s\n' "$p" >&2
    exit 2
  fi
  if [ -n "$out" ]; then printf '%s\n' "$out"; found=1; fi
done

exit "$found"
