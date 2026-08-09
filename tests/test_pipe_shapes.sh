#!/usr/bin/env bash
# tests/test_pipe_shapes.sh — run: bash tests/test_pipe_shapes.sh
#
# WHOLE-REPO SHELL-SHAPE GUARD: no producer piped into an early-exiting consumer (AGENTS.md
# § Shell). Under `set -o pipefail` — which every shell file in this repo sets — a consumer that
# exits before EOF (`grep -q`/`-m`, `head`) SIGPIPEs its producer, and the 141 surfaces as an
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
# SHAPE, NOT SPELLINGS: the consumer half keys on any grep flag bundle carrying `q` in ANY
# position (`-q`, `-qF`, `-Eqi`, `-vqE`, …), the long spellings, `-m`/`--max-count`, and `head`.
# The file-scoped predecessor required `q` to CLOSE the bundle and so never saw `-qF` — the exact
# spelling of the second gate failure. That gap is the other reason this guard exists.
#
# KNOWN IMPRECISION, stated rather than hidden: the scan is line-based, so a pipeline whose
# consumer opens a continuation line is not seen, and a quoted DATA string on an executable line
# that spells the shape would be reported as if live. The one such data site
# (tests/test_render_board.sh's fd-dup fixtures) assembles its pipe from a variable for exactly
# this reason — the same dodge this file's own fixtures use.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d "${TMPDIR:-/tmp}/pipe-shapes.XXXXXX")"; trap 'rm -rf "$tmp"' EXIT

# The ONE predicate, used verbatim on the live tree and on every fixture below. `[|]` not `\|`
# (POSIX ERE leaves `\|` undefined); the leading `(^|[^|])` keeps `||`-chained greps (not
# pipelines) out; `head` must be followed by an ASCII flag or end the line, so prose like
# "(2>&1 | head)" in an assert title does not read as a consumer.
BAD_SHAPE='(^|[^|])[|][[:space:]]*(grep[[:space:]]+(-[A-Za-z]+[[:space:]]+)*(-[A-Za-z]*q|-m|--quiet|--silent|--max-count)|head([[:space:]]+-|[[:space:]]*$))'

scan(){ # scan <file>... -> "file:line:content" per offending EXECUTABLE line
  local raw
  raw="$(grep -HnE "$BAD_SHAPE" "$@" 2>/dev/null)" || true
  grep -vE '^[^:]*:[0-9]+:[[:space:]]*#' <<<"$raw" || true
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
assert "no shell file pipes a producer into an early-exiting consumer (AGENTS.md § Shell)" \
  '[ -z "$hits" ]'
if [ -n "$hits" ]; then
  echo "--- pipelines whose producer can take SIGPIPE under pipefail ---"
  printf '%s\n' "$hits"
  echo "--- remedies: grep <<<\"\$var\"; de-quiet to '| grep >/dev/null'; head -N -> sed -n 1,Np ---"
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

printf '%s\n' 'grep -qF "needle" <<<"$var"' > "$tmp/ok-herestring.sh"
assert "predicate abstains: the here-string remedy" '[ -z "$(scan "$tmp/ok-herestring.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar grep >/dev/null -F 'needle'" > "$tmp/ok-dequiet.sh"
assert "predicate abstains: the de-quieted '| grep >/dev/null' remedy" \
  '[ -z "$(scan "$tmp/ok-dequiet.sh")" ]'

printf '%s\n' "cat \"\$f\" $_bar sed -n 1p" > "$tmp/ok-sed.sh"
assert "predicate abstains: the sed -n 1,Np remedy for head" '[ -z "$(scan "$tmp/ok-sed.sh")" ]'

printf '%s\n' 'grep -qF "a" "$f" || grep -qi "b" "$f"' > "$tmp/ok-orchain.sh"
assert "predicate abstains: ||-chained file greps are not pipelines" \
  '[ -z "$(scan "$tmp/ok-orchain.sh")" ]'

printf '%s\n' "echo \"reader shape (2>&1 $_bar head)\"" > "$tmp/ok-prose.sh"
assert "predicate abstains: prose 'head' with no flag after it (assert-title shape)" \
  '[ -z "$(scan "$tmp/ok-prose.sh")" ]'

printf '%s\n' "# a comment: cat f $_bar grep -q x" > "$tmp/ok-comment.sh"
assert "predicate abstains: comment lines" '[ -z "$(scan "$tmp/ok-comment.sh")" ]'

exit $fail
