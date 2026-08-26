#!/usr/bin/env bash
# tests/test_branch_reconstruction_guard.sh — a branch name is minted through exactly ONE
# constructor (domain.MintBranch) and consumed from what the claim RECORDED; it is never
# reconstructed from a slug or hand-assembled from the "feat/" prefix. This guard pins that
# invariant statically, so a regression reintroducing a bare-slug branch name reddens here
# instead of at runtime.
#
# Three clauses, each keyed on syntactic SHAPE (never an enumerated list of spellings), each with a
# COMPUTED population floor and a positive control routed through the same scan the main check uses
# (guards-are-code; a scan that never fired against a real drift is decoration):
#
#   1. RETIRED SYMBOL — BranchForSlug was deleted when the recorded branch became authoritative.
#      It must match NOWHERE under internal/ in any .go file, tests included: a total ban, because
#      the symbol no longer exists and any occurrence — call, definition, or comment — is drift.
#
#   2. NO "feat/" RECONSTRUCTION IN EXECUTABLE GO — over non-test .go under internal/, comment-only
#      lines are stripped first, then the byte pattern must not appear at all.
#      KNOWN IMPRECISION (assert-adjacent, stated per byte-pattern-guard-matches-a-spelling): this
#      clause bans the QUOTED SPELLING only. The leading double quote bounds the literal's left
#      edge, so it catches "feat/" + x and fmt.Sprintf("feat/%s", ...) alike — but a branch name
#      assembled without that quoted prefix (a rune-built string, a constant indirection) is a
#      spelling this pattern does not reach. The quote is the shape that every observed
#      reconstruction has used; widening past it is left to a conscious future change.
#
#   3. MINT POPULATION FLOOR+CEILING — the set of non-test .go files that CALL MintBranch( (the
#      constructor's own definition excluded) must be EXACTLY the known-good set below. Both
#      directions: every expected file present (floor — a call deleted reddens), no unexpected file
#      (ceiling — a new call site reddens). A new legitimate mint site updates the list CONSCIOUSLY,
#      in this file, in the same diff — that deliberate edit is the point, not an obstacle.
#
# The expected mint set is COMPUTED from the tree, not copied from the plan: the plan that
# commissioned this guard named three call sites, but implementation_context.go is a genuine fourth
# — added in the same change that deleted BranchForSlug, where the pre-claim context bundle mints
# the branch the imminent claim will record. Enumerating from a stale belief rather than the tree is
# the exact failure marker-scoped-guard-needs-a-population-floor warns against; the set here is what
# git actually shows.
#
# TRACKED-FILES-ONLY: the walk enumerates what version control tracks, so a brand-new unstaged .go
# file is invisible until staged. Accepted: this guard runs at the build gate over committed work.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# --- ONE scan implementation per clause, shared by the main check AND its controls ---------------
# A separate grep at each call site would let a mutation to the loop's own invocation go untested.
# Routing the controls through these functions means neutering a scan anywhere neuters it
# everywhere, so a positive control cannot stay green while the loop goes blind.

# Clause 1: any occurrence of the retired symbol. Fixed-string; total ban, no comment stripping.
scan_retired(){ grep -nF 'BranchForSlug' "$1" 2>/dev/null; }

# Comment-only lines are those whose first non-space characters are //. ERE, POSIX class only (no
# repetition bounds), so /usr/bin/grep and PATH ugrep agree.
strip_comment_lines(){ grep -vE '^[[:space:]]*//' "$1" 2>/dev/null; }

# Clause 2: the quoted "feat/ byte pattern, in executable (comment-stripped) lines. Captured into a
# variable before the second grep — never producer|consumer under pipefail (a here-string has no
# producer to take SIGPIPE). The pattern's leading byte is a double quote, not --, so no -e guard.
scan_feat(){
  local stripped
  stripped="$(strip_comment_lines "$1")"
  grep -nF '"feat/' <<<"$stripped"
}

# Clause 3: MintBranch( CALL lines — comment-only lines stripped, and the constructor's own
# definition (func MintBranch() excluded so the defining file is not miscounted as a caller.
mint_call_hits(){
  local stripped withtoken
  stripped="$(strip_comment_lines "$1")"
  withtoken="$(grep -F 'MintBranch(' <<<"$stripped")"
  [ -z "$withtoken" ] && return 0
  grep -vF 'func MintBranch(' <<<"$withtoken"
}

# --- populations ---------------------------------------------------------------------------------
# git ls-files, so the walk is version-controlled reality, not on-disk debris. The pathspec is the
# directory; the extension filter is a shape test on the suffix, not an enumerated file list.
mapfile -t ALL_GO < <(cd "$ROOT" && git ls-files -- internal | grep -E '\.go$')
SRC_GO=()
for f in "${ALL_GO[@]:-}"; do
  [ -n "$f" ] || continue
  case "$f" in *_test.go) ;; *) SRC_GO+=("$f");; esac
done

n_all=${#ALL_GO[@]}
n_src=${#SRC_GO[@]}

# --- population floor: computed from real files, not a magic count -------------------------------
# A guard iterating an empty (or test-only) list is green and proves nothing. The floor is anchored
# on files that MUST exist for the invariant to be meaningful — the constructor's home, a known
# caller, and a known test — so a broken pathspec reddens instead of passing vacuously. n_all>n_src
# additionally proves the tests-included walk really includes tests.
all_joined=""
[ "$n_all" -gt 0 ] && all_joined="$(printf '%s\n' "${ALL_GO[@]}")"
src_joined=""
[ "$n_src" -gt 0 ] && src_joined="$(printf '%s\n' "${SRC_GO[@]}")"

[ "$n_src" -gt 0 ] \
  && ok "non-test .go population is non-empty ($n_src files)" \
  || nok "non-test .go population collapsed to $n_src — pathspec or ls-files broke"
[ "$n_all" -gt "$n_src" ] \
  && ok "tests-included walk is strictly larger ($n_all vs $n_src) — tests are in scope" \
  || nok "tests-included walk ($n_all) is not larger than non-test ($n_src) — tests missing from walk"

for probe in internal/domain/types.go internal/domain/actions.go internal/app/change_reclaim.go; do
  grep -qxF "$probe" <<<"$src_joined" \
    && ok "walk includes known non-test file $probe" \
    || nok "walk MISSES $probe — the in-scope surface is not fully covered"
done
# A known test file must be in the tests-included walk, or clause 1's "tests included" is a claim
# the population never backs.
probe_test="internal/app/branch_identity_test.go"
grep -qxF "$probe_test" <<<"$all_joined" \
  && ok "tests-included walk reaches a known _test.go ($probe_test)" \
  || nok "tests-included walk MISSES $probe_test — the test surface is not covered"

# ================================================================================================
# Clause 1 — the retired symbol matches nowhere under internal/, tests included.
# ================================================================================================
retired_violations=""
if [ "$n_all" -gt 0 ]; then
  for f in "${ALL_GO[@]}"; do
    [ -f "$ROOT/$f" ] || continue
    hits="$(scan_retired "$ROOT/$f")"
    [ -n "$hits" ] && retired_violations+="$f"$'\n'
  done
fi
if [ -z "$retired_violations" ]; then
  ok "clause 1: retired symbol BranchForSlug matches nowhere under internal/"
else
  nok "clause 1: retired symbol BranchForSlug reappeared — it was deleted; consume the recorded branch instead:"
  printf '%s' "$retired_violations" | sed 's/^/       /'
fi

# ================================================================================================
# Clause 2 — no quoted "feat/ reconstruction in executable (non-test) Go.
# KNOWN IMPRECISION: this assert bans the QUOTED SPELLING only (see header). A branch name built
# without the "feat/ literal is a spelling this clause does not reach.
# ================================================================================================
feat_violations=""
if [ "$n_src" -gt 0 ]; then
  for f in "${SRC_GO[@]}"; do
    [ -f "$ROOT/$f" ] || continue
    hits="$(scan_feat "$ROOT/$f")"
    [ -n "$hits" ] && feat_violations+="$(printf '%s\n' "$hits" | sed "s|^|$f:|")"$'\n'
  done
fi
if [ -z "$feat_violations" ]; then
  ok 'clause 2: no quoted "feat/ branch reconstruction in executable Go'
else
  nok 'clause 2: quoted "feat/ found in executable Go — mint via domain.MintBranch, consume the recorded branch:'
  printf '%s' "$feat_violations" | sed 's/^/       /'
fi

# ================================================================================================
# Clause 3 — mint call sites are EXACTLY the known-good set (floor + ceiling).
# ================================================================================================
# EXPECTED is the computed known-good set. Adding or removing a legitimate mint site is a conscious
# edit to THIS list, made in the same diff — that is the guard working, not fighting you.
EXPECTED=(
  internal/domain/actions.go
  internal/domain/lease.go
  internal/app/change_reclaim.go
  internal/app/implementation_context.go
)
actual_callers=""
if [ "$n_src" -gt 0 ]; then
  for f in "${SRC_GO[@]}"; do
    [ -f "$ROOT/$f" ] || continue
    hits="$(mint_call_hits "$ROOT/$f")"
    [ -n "$hits" ] && actual_callers+="$f"$'\n'
  done
fi
expected_joined="$(printf '%s\n' "${EXPECTED[@]}")"
# Floor: every expected caller is actually present.
missing=""
while IFS= read -r want; do
  [ -n "$want" ] || continue
  grep -qxF "$want" <<<"$actual_callers" || missing+="$want"$'\n'
done <<<"$expected_joined"
# Ceiling: no caller outside the expected set.
unexpected=""
while IFS= read -r got; do
  [ -n "$got" ] || continue
  grep -qxF "$got" <<<"$expected_joined" || unexpected+="$got"$'\n'
done <<<"$actual_callers"

if [ -z "$missing" ]; then
  ok "clause 3 floor: every expected MintBranch( caller is present"
else
  nok "clause 3 floor: an expected MintBranch( caller vanished — a mint site was removed or renamed:"
  printf '%s' "$missing" | sed 's/^/       /'
fi
if [ -z "$unexpected" ]; then
  ok "clause 3 ceiling: no MintBranch( caller outside the known-good set"
else
  nok "clause 3 ceiling: a NEW MintBranch( caller appeared — if legitimate, add it to EXPECTED in this file:"
  printf '%s' "$unexpected" | sed 's/^/       /'
fi

# ================================================================================================
# Controls — prove each predicate FIRES, through the SAME scan functions the loops use, and prove
# the comment-strip actually suppresses a commented occurrence (the negative direction).
# ================================================================================================
tmp="$(mktemp -d "${TMPDIR:-/tmp}/branch-guard.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

# Clause 1 positive control: a re-added definition is reported.
printf 'func BranchForSlug(slug string) string { return slug }\n' > "$tmp/retired.go"
[ -n "$(scan_retired "$tmp/retired.go")" ] \
  && ok "control: retired-symbol scan reports a re-added BranchForSlug" \
  || nok "control FAILED: retired-symbol scan matches nothing — clause 1 is vacuous"

# Clause 2 positive control: a quoted "feat/ on an executable line is reported.
printf '%s\n' 'func f() { x := "feat/" + "y"; _ = x }' > "$tmp/feat_exec.go"
[ -n "$(scan_feat "$tmp/feat_exec.go")" ] \
  && ok 'control: feat scan reports a quoted "feat/ on an executable line' \
  || nok 'control FAILED: feat scan matches an executable occurrence nowhere — clause 2 is vacuous'
# Clause 2 negative control: the SAME literal inside a comment-only line is suppressed by the strip.
printf '%s\n' '// x := "feat/" + "y" is how it used to be built' > "$tmp/feat_comment.go"
[ -z "$(scan_feat "$tmp/feat_comment.go")" ] \
  && ok 'control: feat scan suppresses a commented "feat/ (comment strip works)' \
  || nok 'control FAILED: comment strip does not suppress a commented "feat/ — clause 2 over-fires'

# Clause 3 positive control: a real call is detected.
printf '%s\n' 'func g() { _ = domain.MintBranch("a", domain.OptionalString{}, "b") }' > "$tmp/caller.go"
[ -n "$(mint_call_hits "$tmp/caller.go")" ] \
  && ok "control: mint-call scan reports a real MintBranch( call" \
  || nok "control FAILED: mint-call scan matches a real call nowhere — clause 3 ceiling is vacuous"
# Clause 3 negative control A: the constructor DEFINITION is not counted as a caller.
printf '%s\n' 'func MintBranch(typeToken string, prefix OptionalString, slug string) string { return "" }' > "$tmp/def.go"
[ -z "$(mint_call_hits "$tmp/def.go")" ] \
  && ok "control: mint-call scan excludes the constructor definition" \
  || nok "control FAILED: mint-call scan counts the func MintBranch definition as a caller"
# Clause 3 negative control B: a commented mention is not counted as a caller.
printf '%s\n' '// see domain.MintBranch( for the single constructor' > "$tmp/mention.go"
[ -z "$(mint_call_hits "$tmp/mention.go")" ] \
  && ok "control: mint-call scan excludes a commented MintBranch( mention" \
  || nok "control FAILED: mint-call scan counts a commented mention as a caller"

exit "$fail"
