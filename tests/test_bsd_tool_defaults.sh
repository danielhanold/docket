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

# Every `mv` invocation whose first argument is a double-quoted word. Two patterns, never one
# combined alternation: column-0 `mv`, and `mv` preceded by any character that is not part of a
# longer word or a trailing option cluster. They are disjoint, so no line is reported twice.
hits_mv(){
  local f
  while IFS= read -r f; do
    "$GREP" -nE '^mv "' "$f" | sed "s|^|$f:|"
    "$GREP" -nE '[^-[:alnum:]]mv "' "$f" | sed "s|^|$f:|"
  done < <(scope_files)
}

# An allowance keyed on the invocation shape actually present in the tree: archive-change.sh spells
# its rename `$GIT -C "$WT" mv …`, so a literal `git mv` allowance would never fire and the guard
# would redden on the carve-out. `git mv` is a different tool with different prompting semantics,
# and `-f` there means force-overwrite a tracked target — a semantics change, not a hardening.
offenders_mv(){ hits_mv | "$GREP" -vE 'mv -f ' | "$GREP" -vE '(git|\$GIT)[^|]* mv '; }

bad_mv="$(offenders_mv)"
assert "every mv that replaces a file passes -f, so it cannot prompt on a tty" \
  '[ -z "$bad_mv" ] || { echo "$bad_mv" | sed "s|^$ROOT/|  |" >&2; echo "  RULE: bare mv prompts on an unwritable destination with a tty, self-answers n, and exits 0 — so the || die never fires and the write is lost. Write these as: mv -f SRC DEST. git mv is exempt (different tool)." >&2; false; }'

n_mv_f="$("$GREP" -rlE 'mv -f ' $(scope_files) 2>/dev/null | wc -l | tr -d " ")"
n_mv_f_sites="$("$GREP" -hcE 'mv -f ' $(scope_files) 2>/dev/null | awk '{s+=$1} END{print s+0}')"
assert "at least $MIN_MV_F non-interactive mv sites exist, so the check above is not vacuous (found $n_mv_f_sites)" \
  '[ "$n_mv_f_sites" -ge "$MIN_MV_F" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or an install path stopped replacing files — check which before touching this number." >&2; false; }'

exit "$fail"
