#!/usr/bin/env bash
# tests/test_pipe_shapes.sh — run: bash tests/test_pipe_shapes.sh
#
# WHOLE-REPO SHELL-SHAPE GUARD: no producer piped into an early-exiting consumer (AGENTS.md
# § Shell). Under `set -o pipefail` — which every shell file in this repo sets — a consumer that
# exits before EOF (`grep -q`/`-m`, `head`, `awk … exit`, `sed … q`, `read`) SIGPIPEs its producer,
# and the 141 surfaces as an
# INTERMITTENT failure that only fires when the payload outruns the pipe buffer under load.
# The change-0276 build gate went red twice on exactly this class, in two different files:
#   1. `git show … | grep -q …` over the ~40KB example (tests/test_docket_example_yml.sh, which
#      now carries the measurement: 70/70 failures under pipe-buffer pressure, 0/70 unloaded);
#   2. `printf "%s\n" "$(cat …)" | grep -qF …` over a captured docket-status transcript.
# Failure 2 is why this guard has NO printf/echo producer exemption: the file-scoped guard that
# preceded it (test_docket_example_yml.sh, "SHELL-SHAPE SELF-GUARD") exempted printf of an
# already-materialized payload as bounded, and the very next gate run failed on a site inside
# that exemption — a materialized 64KB payload SIGPIPEs exactly like a streamed one.
#
# The remedies this repo standardized in the same sweep (change 0276 integration repair):
#   - producer is printf/echo of one payload  ->  drop the pipe: `grep <flags> <pat> <<<"$var"`
#     (spelled `grep <<<"$var" <flags> <pat>` where the rewrite was mechanical — same command,
#     the redirection word is position-independent);
#   - consumer is a mid-/end-of-pipeline quiet grep  ->  de-quiet it: `| grep >/dev/null <flags>`
#     reads to EOF, so the producer can never take SIGPIPE, and every stage's exit status is
#     unchanged;
#   - consumer is `head -N`  ->  `| sed -n 1,Np` (sed reads to EOF; same output, same status).
#
# SHAPE, NOT SPELLINGS: "early-exiting consumer" is a MECHANISM, not a command name — anything that
# closes stdin before EOF SIGPIPEs its producer. So the consumer half covers, each with its own
# `predicate fires:` fixture below:
#   - any grep flag bundle carrying `q` in ANY position (`-q`, `-qF`, `-Eqi`, `-vqE`, …), the long
#     spellings, and `-m`/`--max-count`. The file-scoped predecessor required `q` to CLOSE the
#     bundle and so never saw `-qF` — the exact spelling of the second gate failure;
#   - `head` (flag-or-EOL, so prose "| head)" is not a consumer);
#   - `awk` whose program contains a bare `exit` — the house idiom of the very file this sweep was
#     written for (`awk '$1==p{print $2; exit}'`), and the spelling an enumerate-two-commands
#     predicate missed at three live sites;
#   - `sed` whose script contains a `q` command (`sed 1q`, `sed -n '1p;q'`, `sed '2q;d'`);
#   - `read` as the direct consumer (`| read -r x`, `| IFS= read -r x`) — one line, then close.
# Each of those also matches through an optional PATH PREFIX (`| /usr/bin/grep -q …`): this repo
# already pipes into `/usr/bin/grep` elsewhere, so keying on the bare word would have been the same
# enumerate-the-spelling mistake one level down.
#
# DELIBERATELY NOT CONSUMERS, on evidence rather than convenience:
#   - `sed` with NO `q` — `sed -n 1,Np`, this sweep's own replacement for `head -N`, and
#     `sed -n 's/^worktree //p'`. GNU and BSD sed both read to EOF unless a `q`/`Q` command fires,
#     so the producer never sees SIGPIPE. These are correct code and the predicate abstains on them.
#   - `awk`'s `exit` inside an `END` action (`awk '… END{exit !ok}'`, tests/test_render_adr_index.sh).
#     END runs only AFTER input is exhausted, so that `exit` cannot preempt the producer. scan()
#     blanks `END{…}` actions before testing the line, so the exit inside one is not seen.
#   - `| while read …` loops, which consume to EOF.
#
# KNOWN IMPRECISION, stated rather than hidden — the assert title claims only what this list leaves:
#   - the scan is LINE-BASED, so a pipeline whose consumer opens a continuation line is not seen;
#   - a quoted DATA string on an executable line that spells the shape is reported as if live. The
#     one such data site (tests/test_render_board.sh's fd-dup fixtures) assembles its pipe from a
#     variable for exactly this reason — the same dodge this file's own fixtures use. Prose in an
#     assert title reading "| read the file" would report the same way;
#   - the `END{…}` blanking above uses a brace-balanced-by-one match, so an END action with a
#     NESTED brace block keeps its `exit` and would be reported as a false positive;
#   - a consumer reached indirectly — through a function, `xargs`, or a variable holding the
#     command word — is invisible to a textual predicate and is not claimed.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d "${TMPDIR:-/tmp}/pipe-shapes.XXXXXX")"; trap 'rm -rf "$tmp"' EXIT

# The ONE predicate, used verbatim on the live tree and on every fixture below. `[|]` not `\|`
# (POSIX ERE leaves `\|` undefined); the leading `(^|[^|])` keeps `||`-chained greps (not
# pipelines) out; `head` must be followed by an ASCII flag or end the line, so prose like
# "(2>&1 | head)" in an assert title does not read as a consumer. Composed from named parts so each
# consumer shape is readable on its own; NO bounded repetition anywhere (`{n,m}`), because BSD grep
# caps a bound at 255 and PATH grep here is ugrep, which would mask the error (AGENTS.md).
# Optional path prefix — piping into an absolute-path grep is the same defect as the bare word.
# (Spelled here on a COMMENT line, not a trailing one: this file is inside its own scan population.)
_C_PFX='([^[:space:]|]*/)?'
_C_GREP='grep[[:space:]]+(-[A-Za-z]+[[:space:]]+)*(-[A-Za-z]*q|-m|--quiet|--silent|--max-count)'
_C_HEAD='head([[:space:]]+-|[[:space:]]*$)'
# awk/sed: `[^|]*` confines the program text to THIS pipeline stage, so a later stage's text cannot
# satisfy an earlier stage's branch. sed's pre-`q` class excludes letters, `_`, `/`, `^` and `|` so
# that a `q` inside an s/// pattern or replacement (`sed 's/quiet/loud/'`) is not read as a command,
# while `sed 1q`, `sed q`, `;q` and `{q` all are.
_C_AWK='awk[[:space:]][^|]*[^[:alnum:]_]exit([^[:alnum:]_]|$)'
_C_SED='sed[[:space:]][^|]*[^[:alpha:]_/^|-]q([^[:alnum:]_]|$)'
_C_READ='([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*[[:space:]]+)*read([[:space:]]|$)'
BAD_SHAPE="(^|[^|])[|][[:space:]]*${_C_PFX}(${_C_GREP}|${_C_HEAD}|${_C_AWK}|${_C_SED}|${_C_READ})"

# An `exit` inside an awk END action fires only after input is exhausted, so it is NOT early-exiting
# (see DELIBERATELY NOT CONSUMERS above). Blank those actions out before testing a candidate line —
# blanking rather than dropping the line, so a line carrying BOTH an END exit and a real early exit
# is still reported on the real one.
# Braces bracketed (`[{]`, not `{`) — a bare `{` outside a valid interval is undefined in POSIX ERE
# and BSD sed -E rejects some spellings of it.
END_ACTION_RE='END[[:space:]]*[{][^{}]*[}]'

scan(){ # scan <file>... -> "file:line:content" per offending EXECUTABLE line
  local raw line body
  raw="$(grep -HnE "$BAD_SHAPE" "$@" 2>/dev/null)" || true
  raw="$(grep -vE '^[^:]*:[0-9]+:[[:space:]]*#' <<<"$raw")" || true
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    body="$(sed -E "s/$END_ACTION_RE/ENDACTION/g" <<<"$line")"
    if grep -qE "$BAD_SHAPE" <<<"$body"; then printf '%s\n' "$line"; fi
  done <<<"$raw"
}

# --- the live tree -------------------------------------------------------------------------------
# Derived, never hand-listed (AGENTS.md § Guards): every tracked shell file under tests/ and
# scripts/ at any depth, plus the repo-root scripts.
repo_sh_files(){
  find "$REPO/tests" "$REPO/scripts" -type f -name '*.sh'
  find "$REPO" -maxdepth 1 -type f -name '*.sh'
}
files="$(repo_sh_files)"
assert "the scan population is non-vacuous (>=100 shell files)" \
  '[ "$(wc -l <<<"$files")" -ge 100 ]'
hits="$(scan $files)"
# Title states the DELIVERED scope, not the aspiration: single-line pipelines, and the five consumer
# shapes enumerated in the header. Indirect consumers and continuation-line pipelines are disclosed
# under KNOWN IMPRECISION and are deliberately not claimed here.
assert "no single-line pipeline feeds a producer into an early-exiting consumer — grep -q/-m, head, awk exit, sed q, read (AGENTS.md § Shell)" \
  '[ -z "$hits" ]'
if [ -n "$hits" ]; then
  echo "--- pipelines whose producer can take SIGPIPE under pipefail ---"
  printf '%s\n' "$hits"
  echo "--- remedies: <consumer> <<<\"\$var\"; de-quiet to '| grep >/dev/null'; head -N -> sed -n 1,Np ---"
fi

# --- guard-the-guard -----------------------------------------------------------------------------
# The assert above is green when the tree is clean AND when the predicate matches nothing at all,
# so prove the predicate fires on each shape it exists to catch and abstains on each remedy.
# Offending fixtures are ASSEMBLED from $_bar: this file is inside its own scan population, so a
# literal spelling of the defect here would redden the live-tree assert it supports.
_bar='|'

printf '%s\n' "git -C \"\$d\" show origin/main:.docket.yml $_bar grep -q '^k:'" > "$tmp/bad-git.sh"
assert "predicate fires: git-show producer into quiet grep" '[ -n "$(scan "$tmp/bad-git.sh")" ]'

printf '%s\n' "printf '%s\\n' \"\$(cat \"\$out\")\" $_bar grep -qF 'needle'" > "$tmp/bad-printf.sh"
assert "predicate fires: printf of a materialized payload is NOT exempt (the run-2 failure class)" \
  '[ -n "$(scan "$tmp/bad-printf.sh")" ]'

printf '%s\n' "printf '%s' \"\$sout\" $_bar grep -Eqi 'pat'" > "$tmp/bad-bundle.sh"
assert "predicate fires: q buried mid-bundle (-Eqi), the file-scoped guard's blind spot" \
  '[ -n "$(scan "$tmp/bad-bundle.sh")" ]'

printf '%s\n' "git log --format=%s $_bar head -n1" > "$tmp/bad-head.sh"
assert "predicate fires: head as the early-exiting consumer" '[ -n "$(scan "$tmp/bad-head.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar grep --max-count=1 'x'" > "$tmp/bad-maxcount.sh"
assert "predicate fires: --max-count long spelling" '[ -n "$(scan "$tmp/bad-maxcount.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar /usr/bin/grep -q 'x'" > "$tmp/bad-pathgrep.sh"
assert "predicate fires: path-prefixed consumer (/usr/bin/grep -q), already an idiom in this repo" \
  '[ -n "$(scan "$tmp/bad-pathgrep.sh")" ]'

printf '%s\n' "\"\$GIT\" worktree list --porcelain $_bar awk '/^worktree /{print \$2; exit}'" \
  > "$tmp/bad-awk.sh"
assert "predicate fires: awk program with a bare exit (the sweep's own house idiom)" \
  '[ -n "$(scan "$tmp/bad-awk.sh")" ]'

printf '%s\n' "printf '%s\\n' \"\$flat\" $_bar awk -F'\\t' -v p=\"\$k\" '\$1==p{print \$2; exit}'" \
  > "$tmp/bad-awk-field.sh"
assert "predicate fires: awk exit inside a field-match action (the two example-yml sites)" \
  '[ -n "$(scan "$tmp/bad-awk-field.sh")" ]'

printf '%s\n' "git log --format=%s $_bar sed 1q" > "$tmp/bad-sed-1q.sh"
assert "predicate fires: sed 1q" '[ -n "$(scan "$tmp/bad-sed-1q.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar sed -n '1p;q'" > "$tmp/bad-sed-pq.sh"
assert "predicate fires: sed -n '1p;q'" '[ -n "$(scan "$tmp/bad-sed-pq.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar read -r first" > "$tmp/bad-read.sh"
assert "predicate fires: read as the direct consumer" '[ -n "$(scan "$tmp/bad-read.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar IFS= read -r first" > "$tmp/bad-read-ifs.sh"
assert "predicate fires: read behind an inline assignment (IFS= read -r)" \
  '[ -n "$(scan "$tmp/bad-read-ifs.sh")" ]'

printf '%s\n' 'grep -qF "needle" <<<"$var"' > "$tmp/ok-herestring.sh"
assert "predicate abstains: the here-string remedy" '[ -z "$(scan "$tmp/ok-herestring.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar grep >/dev/null -F 'needle'" > "$tmp/ok-dequiet.sh"
assert "predicate abstains: the de-quieted '| grep >/dev/null' remedy" \
  '[ -z "$(scan "$tmp/ok-dequiet.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar sed -n 1p" > "$tmp/ok-sed.sh"
assert "predicate abstains: the sed -n 1,Np remedy for head" '[ -z "$(scan "$tmp/ok-sed.sh")" ]'

printf '%s\n' "\"\$git\" worktree list --porcelain $_bar sed -n '1s/^worktree //p'" > "$tmp/ok-sed-sub.sh"
assert "predicate abstains: q-less sed reads to EOF (docket-root.sh's first-worktree idiom)" \
  '[ -z "$(scan "$tmp/ok-sed-sub.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar sed -e 's/quiet/loud/'" > "$tmp/ok-sed-qdata.sh"
assert "predicate abstains: a 'q' inside an s/// pattern is data, not a sed command" \
  '[ -z "$(scan "$tmp/ok-sed-qdata.sh")" ]'

printf '%s\n' "printf '%s\\n' \"\$out\" $_bar awk '/## A/{f=1} f&&/_None._/{ok=1} END{exit !ok}'" \
  > "$tmp/ok-awk-end.sh"
assert "predicate abstains: awk exit inside an END action runs after EOF" \
  '[ -z "$(scan "$tmp/ok-awk-end.sh")" ]'

printf '%s\n' "printf '%s\\n' \"\$out\" $_bar awk 'NR==2{print; exit} END{exit 1}'" \
  > "$tmp/mixed-awk-end.sh"
assert "predicate still fires: a real early exit on a line that ALSO carries an END exit" \
  '[ -n "$(scan "$tmp/mixed-awk-end.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar while IFS= read -r l; do echo \"\$l\"; done" > "$tmp/ok-whileread.sh"
assert "predicate abstains: a 'while read' loop consumes to EOF" \
  '[ -z "$(scan "$tmp/ok-whileread.sh")" ]'

printf '%s\n' 'grep -qF "a" "$f" || grep -qi "b" "$f"' > "$tmp/ok-orchain.sh"
assert "predicate abstains: ||-chained file greps are not pipelines" \
  '[ -z "$(scan "$tmp/ok-orchain.sh")" ]'

printf '%s\n' "echo \"reader shape (2>&1 $_bar head)\"" > "$tmp/ok-prose.sh"
assert "predicate abstains: prose 'head' with no flag after it (assert-title shape)" \
  '[ -z "$(scan "$tmp/ok-prose.sh")" ]'

printf '%s\n' "# a comment: cat f $_bar grep -q x" > "$tmp/ok-comment.sh"
assert "predicate abstains: comment lines" '[ -z "$(scan "$tmp/ok-comment.sh")" ]'

exit $fail
