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
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The scan binary, pinned. On Linux this is GNU grep; on macOS, BSD grep. Both agree on the split
# patterns below. PATH grep is deliberately not trusted.
GREP=/usr/bin/grep
assert "the pinned scan binary exists" '[ -x "$GREP" ]'

# In-scope population floor: if the walk collapses, every negative assert below passes vacuously.
MIN_FILES=40
# Post-sweep floor on atomic-replace/rename call sites written the non-interactive way.
MIN_MV_F=16

# The guarded surface: shipped shell, entry scripts included. tests/ is excluded — test-side
# hygiene is owned elsewhere, and this file's own patterns would match themselves.
scope_files(){
  local f
  for f in "$ROOT"/scripts/*.sh "$ROOT"/scripts/lib/*.sh "$ROOT"/scripts/runners/*.sh \
           "$ROOT"/install.sh "$ROOT"/sync-agents.sh "$ROOT"/migrate-to-docket.sh; do
    [ -f "$f" ] && printf '%s\n' "$f"
  done
}

n_files="$(scope_files | wc -l | tr -d ' ')"
assert "the scan reaches at least $MIN_FILES in-scope files (it reached $n_files)" \
  '[ "$n_files" -ge "$MIN_FILES" ]'

# Every `mv` invocation whose first argument is a double-quoted word, minus the allowances. Two
# patterns, never one combined alternation: column-0 `mv`, and `mv` preceded by any character that
# is not part of a longer word or a trailing option cluster. They are disjoint, so no line is
# reported twice.
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
  local f
  while IFS= read -r f; do
    { "$GREP" -nE '^mv "' "$f"; "$GREP" -nE '[^-[:alnum:]]mv "' "$f"; } \
      | "$GREP" -vE 'mv -f ' \
      | "$GREP" -vE '(git|\$GIT)[^|;&]* mv ' \
      | sed "s|^|$f:|"
  done < <(scope_files)
}

bad_mv="$(offenders_mv)"
assert "every mv that replaces a file passes -f, so it cannot prompt on a tty" \
  '[ -z "$bad_mv" ] || { echo "$bad_mv" | sed "s|^$ROOT/|  |" >&2; echo "  RULE: bare mv prompts on an unwritable destination with a tty, self-answers n, and exits 0 — so the || die never fires and the write is lost. Write these as: mv -f SRC DEST. git mv is exempt (different tool)." >&2; false; }'

n_mv_f="$("$GREP" -rlE 'mv -f ' $(scope_files) 2>/dev/null | wc -l | tr -d " ")"
n_mv_f_sites="$("$GREP" -hcE 'mv -f ' $(scope_files) 2>/dev/null | awk '{s+=$1} END{print s+0}')"
assert "at least $MIN_MV_F non-interactive mv sites exist, so the check above is not vacuous (found $n_mv_f_sites)" \
  '[ "$n_mv_f_sites" -ge "$MIN_MV_F" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or an install path stopped replacing files — check which before touching this number." >&2; false; }'

# Post-sweep floor on templated mktemp calls: 23 swept here plus 6 pre-existing beside-destination
# sites. Same reason as the mv floor — a negative grep is also green when it scans nothing.
MIN_MKTEMP_TEMPLATED=29

# Every line invoking mktemp through command substitution. One predicate for BOTH the -d and the
# file form: no option parsing, so a future flag cannot slip a site past the check.
hits_mktemp(){
  local f
  while IFS= read -r f; do
    "$GREP" -nF '$(mktemp' "$f" | sed "s|^|$f:|"
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
  '[ "$n_tmpl" -ge "$MIN_MKTEMP_TEMPLATED" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or scratch files stopped being created — check which before touching this number." >&2; false; }'

exit "$fail"
