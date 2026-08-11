#!/usr/bin/env bash
# tests/test_bsd_tool_defaults.sh — a BSD tool's DEFAULT behavior may not silently defeat a guard
# (change 0254). Two shapes, one class.
#
# WHY (mv): BSD `mv` with an unwritable destination and a tty on stdin PROMPTS, self-answers `n`
# at EOF, prints "not overwritten", and EXITS 0. Every `|| die` guard on such a call is therefore
# unreachable and the write is silently discarded. `-f` is what makes an install non-interactive;
# it is load-bearing, not style.
#
# WHY (mktemp): a bare `mktemp`, with or without `-d`, ignores TMPDIR on macOS and lands the temp
# file outside any redirect. A fixture that redirects TMPDIR to contain a script's scratch dir is
# then a no-op, and undeletable debris accumulates outside the fixture forever.
#
# SHAPE-KEYED, NOT FILE-KEYED: both halves are repo-wide policy asserted over a call shape. There
# is no allowlist of exempt files (ADR-0050) — exclusions are by walk scope only.
#
# PINNED GREP: the scan runs /usr/bin/grep by absolute path, never PATH grep. Probed during design:
# the single combined ERE `(^|[^-[:alnum:]])mv "` matches NOTHING under this machine's PATH grep
# (ugrep) while matching correctly under /usr/bin/grep — the combined spelling would make this
# guard vacuous exactly where the suite usually runs. Two split patterns agree under both engines,
# and pinning the binary removes the engine variable entirely. Engine agreement is never assumed.
#
# POPULATION FLOORS: a negative grep passes both when the tree is clean and when the scan has
# silently collapsed. Each half therefore also asserts a floor on the POSITIVE population it
# expects to find. The floors are literals both engines handle; they cannot detect a vacuous
# negative predicate, which is what the pinned binary and the mutation tests cover instead.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# The scan binary, pinned. On Linux this is GNU grep; on macOS, BSD grep. Both agree on the split
# patterns below. PATH grep is deliberately not trusted.
GREP=/usr/bin/grep
assert "the pinned scan binary exists" '[ -x "$GREP" ]'

# In-scope population floor: exists to catch a collapsed walk (a broken glob scanning nothing),
# not to track the current population. Set well below the measured count so deleting a normal
# handful of in-scope files never reddens this for a reason unrelated to a collapse.
MIN_FILES=20
# Floor on the agent-executed markdown half of the walk, asserted separately (see md_scope_files).
# Same collapse-detection intent as MIN_FILES, not a population tracker.
MIN_MD_FILES=20
# Post-sweep floor on atomic-replace/rename call sites written the non-interactive way.
MIN_MV_F=16

# The guarded surface: shipped shell, entry scripts included. tests/ is excluded — test-side
# hygiene is owned elsewhere, and this file's own patterns would match themselves.
#
# MARKDOWN THAT AN AGENT EXECUTES IS SHELL. A `.sh`-only walk cannot see the copies of these same
# operations that live as literal bash inside agent-run markdown — a skill body's atomic index
# replace is the identical `mktemp`/`mv` pair as the script's, and prompts identically. Three
# markdown surfaces are therefore in scope, chosen because an agent runs the bash in them
# verbatim: the script contracts (`scripts/*.md`), the skill procedures (`skills/*/SKILL.md`), and
# the procedure references those bodies delegate to (`skills/*/references/*.md`).
#
# ALL markdown is deliberately NOT in scope, and the exclusion is a correctness requirement, not a
# convenience. `docs/` is dominated by point-in-time records — archived changes, results files,
# historical plans — which this repo's rules forbid rewriting: they must keep whatever was true
# when written, and several of them quote bare `mv`/untemplated `mktemp` precisely BECAUSE those
# are the defect under discussion. A whole-markdown walk would demand edits that falsify history,
# and the only way back to green would be an allowlist — which ADR-0050 forbids.
#
# A DEFECTIVE FORM QUOTED AS AN EXAMPLE IS TOLERATED BY SHAPE. Prose naming the bad call writes it
# as a bare code span — `` `mv` `` with no operands — which the invocation predicate (command plus
# whitespace) never matches. Prose naming the GOOD call writes `` `mv -f` ``, whose closing
# backtick is why the exemption below is boundary-terminated rather than requiring a literal
# trailing space. Neither needed a file listed anywhere.
scope_files(){
  local f
  for f in "$ROOT"/scripts/*.sh "$ROOT"/scripts/lib/*.sh "$ROOT"/scripts/runners/*.sh \
           "$ROOT"/install.sh "$ROOT"/sync-agents.sh "$ROOT"/migrate-to-docket.sh "$ROOT"/link-skills.sh; do
    [ -f "$f" ] && printf '%s\n' "$f"
  done
  md_scope_files
}

# Split out from scope_files so the markdown half carries its own population floor: folded into one
# list, a glob that stopped matching would hide inside the combined count and the markdown scan
# would go silently vacuous while the total still cleared MIN_FILES.
md_scope_files(){
  local f
  for f in "$ROOT"/scripts/*.md "$ROOT"/skills/*/SKILL.md "$ROOT"/skills/*/references/*.md; do
    [ -f "$f" ] && printf '%s\n' "$f"
  done
}

n_files="$(scope_files | wc -l | tr -d ' ')"
assert "the scan reaches at least $MIN_FILES in-scope files (it reached $n_files)" \
  '[ "$n_files" -ge "$MIN_FILES" ]'

n_md="$(md_scope_files | wc -l | tr -d ' ')"
assert "the scan reaches at least $MIN_MD_FILES agent-executed markdown files (it reached $n_md)" \
  '[ "$n_md" -ge "$MIN_MD_FILES" ]'

# Every `mv` invocation, minus the allowances. Two patterns, never one combined alternation:
# column-0 `mv`, and `mv` preceded by any character that is not part of a longer word, a path
# segment, or a trailing option cluster.
#
# KEYED ON THE COMMAND, NOT ON ITS FIRST ARGUMENT. An earlier spelling required the literal `mv "`,
# which saw only the quoted-first-argument form. It was blind to `mv -- "$t" "$f"` (the portable
# spelling a future author reaches for), to unquoted `mv $t $f`, and — the sharpest miss — to
# `mv -i "$t" "$f"`, the exact interactive call this rule exists to forbid. It also made the
# `mv -f ` filter dead code, since no line ending the pattern in `"` could carry `-f`. The
# predicate now matches any `mv` followed by whitespace, so the filters below are load-bearing.
#
# COMMENTS ARE NOT INVOCATIONS. Matching the bare command means the scan also reaches prose that
# names `mv` — including the rationale comments this very rule motivated authors to write. A
# comment cannot prompt on a tty, so reddening on one would punish documenting the rule. Whole-line
# comments are therefore dropped before the allowances run. This is a filter on shape, not an
# allowlist of files (ADR-0050), and it cannot hollow the guard out: a comment marker can only ever
# hide a line that is already inert, and the `mv -f ` population floor below plus the mutation
# tests pin the predicate against collapsing to nothing.
#
# `-i`/`-n` ARE DENIED OUTRIGHT, not merely unexempted. They are the interactive and no-clobber
# defaults themselves, so a line carrying one is an offender even if some later `mv -f ` on the
# same line would otherwise satisfy the exemption filter.
#
# ALLOWANCES MATCH CONTENT, NOT LOCATION. The allowance filters run BEFORE the `$f:` path prefix is
# prepended, so only the source line itself can satisfy them. Prefixing first would let a *path*
# decide the verdict: `[^|]*` spans `/` and `:`, so any checkout or filename containing `git` — a
# repo cloned to ~/git, or docket's own gitignore-block library — would exempt every bare `mv` in
# it, and the guard would evaporate for a whole tree. `grep -n` still leaves a `LINENO:` prefix on
# the matched text, which carries no path and cannot bridge the allowance ERE.
#
# The `git`/`$GIT` allowance is keyed on the invocation shape actually present in the tree:
# archive-change.sh spells its rename `$GIT -C "$WT" mv …`, so a literal `git mv` allowance would
# never fire and the guard would redden on the carve-out. `git mv` is a different tool with
# different prompting semantics, and `-f` there means force-overwrite a tracked target — a
# semantics change, not a hardening. Its gap class excludes every shell command separator, not just
# `|`: `$GIT rev-parse …; mv "$t" "$f"` is two commands on one line, and only the first is git.
offenders_mv(){
  local f lines
  while IFS= read -r f; do
    # `grep -n` prefixes each hit with `LINENO:`; the whole-line-comment drop is anchored to that.
    lines="$({ "$GREP" -nE '^mv[[:space:]]' "$f"; "$GREP" -nE '[^-[:alnum:]_./]mv[[:space:]]' "$f"; } \
      | "$GREP" -vE '^[0-9][0-9]*:[[:space:]]*#')"
    [ -n "$lines" ] || continue
    # The exemption ends on a word boundary, not a literal space: in markdown the hardened form is
    # quoted as a code span and closes on a backtick, and `mv -f ` would redden on prose stating
    # the very rule. The class still refuses `-f` glued to a longer word.
    { printf '%s\n' "$lines" | "$GREP" -vE 'mv -f([^-[:alnum:]_]|$)' | "$GREP" -vE '(git|\$GIT)[^|;&]* mv '
      printf '%s\n' "$lines" | "$GREP" -E 'mv -[in][[:space:]]'
    } | sort -u | sed "s|^|$f:|"
  done < <(scope_files)
}

bad_mv="$(offenders_mv)"
assert "every mv that replaces a file passes -f, so it cannot prompt on a tty" \
  '[ -z "$bad_mv" ] || { echo "$bad_mv" | sed "s|^$ROOT/|  |" >&2; echo "  RULE: bare mv prompts on an unwritable destination with a tty, self-answers n, and exits 0 — so the || die never fires and the write is lost. Write these as: mv -f SRC DEST. git mv is exempt (different tool)." >&2; false; }'

# Counted through the same read loop the hit-scanning helpers use, never by word-splitting an
# unquoted `$(scope_files)` into grep's operands. Splitting undercounts silently on a checkout path
# containing whitespace, and — if every glob in the walk stopped matching — leaves grep with no file
# operands at all, at which point it reads STDIN and this floor becomes a hang or a bogus zero.
# Errors are deliberately not redirected to /dev/null: a scan that cannot read a file must say so.
n_mv_f_sites="$({ while IFS= read -r f; do "$GREP" -hcE 'mv -f ' "$f"; done < <(scope_files); } \
  | awk '{s+=$1} END{print s+0}')"
assert "at least $MIN_MV_F non-interactive mv sites exist, so the check above is not vacuous (found $n_mv_f_sites)" \
  '[ "$n_mv_f_sites" -ge "$MIN_MV_F" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or an install path stopped replacing files — check whether the scan broke or the population legitimately shrank." >&2; false; }'

# Post-sweep floor on templated mktemp calls: 23 swept here plus 6 pre-existing beside-destination
# sites. Same reason as the mv floor — a negative grep is also green when it scans nothing.
MIN_MKTEMP_TEMPLATED=29

# Every line invoking mktemp through `$(…)` command substitution, with or without inner padding.
# One predicate for BOTH the -d and the file form, and it does no option parsing — so a future FLAG
# cannot slip a site past the check. That flag claim is the whole of the coverage: this is a
# command-substitution predicate, not a general `mktemp` one. A backticked `` `mktemp` `` and a
# `mktemp` invoked outside a substitution are NOT matched, and the backtick form deliberately so —
# markdown is in scope, and a backticked bare `mktemp` in prose is byte-identical to a backticked
# invocation, so matching it would redden on every sentence that names the defect. `$(` has no such
# prose twin. The residual gap is a real one and is stated here rather than papered over.
hits_mktemp(){
  local f
  while IFS= read -r f; do
    "$GREP" -nE '\$\([[:space:]]*mktemp' "$f" | sed "s|^|$f:|"
  done < <(scope_files)
}

# TEMPLATE-required, deliberately NOT TMPDIR-required. Six in-scope sites are templated BESIDE
# their destination so the following mv is a same-filesystem atomic rename — a documented
# guarantee. A TMPDIR-required predicate would redden on those correct sites and push the next
# author into breaking that atomicity to get back to green.
offenders_mktemp(){ hits_mktemp | "$GREP" -vF 'XXXXXX'; }

bad_mktemp="$(offenders_mktemp)"
assert "every mktemp call passes a template, so TMPDIR is honored" \
  '[ -z "$bad_mktemp" ] || { echo "$bad_mktemp" | sed "s|^$ROOT/|  |" >&2; echo "  RULE: bare mktemp ignores TMPDIR on macOS, so a redirect meant to contain the scratch dir is a no-op and the debris lands outside it. Write: mktemp [-d] \"\${TMPDIR:-/tmp}/<script-name>.XXXXXX\" — or, when the temp file must sit beside its destination for an atomic rename, template it there instead." >&2; false; }'

n_tmpl="$(hits_mktemp | "$GREP" -cF 'XXXXXX')"
assert "at least $MIN_MKTEMP_TEMPLATED templated mktemp sites exist, so the check above is not vacuous (found $n_tmpl)" \
  '[ "$n_tmpl" -ge "$MIN_MKTEMP_TEMPLATED" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or scratch files stopped being created — check whether the scan broke or the population legitimately shrank." >&2; false; }'

exit "$fail"
