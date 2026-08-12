#!/usr/bin/env bash
# tests/test_docket_stack.sh — unit tests for the stacked-changes library and the stack-base CLI
# (change 0298). Sources scripts/lib/docket-stack.sh directly and drives scripts/stack-base.sh
# against hermetic fixture trees. Run: bash tests/test_docket_stack.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO/scripts/lib/docket-stack.sh"
SCRIPT="$REPO/scripts/stack-base.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "library exists" '[ -f "$LIB" ]'
[ -f "$LIB" ] || { printf '%s\n' "--- done"; exit "$fail"; }
# shellcheck source=/dev/null
source "$REPO/scripts/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$LIB"

tmp="$(mktemp -d "${TMPDIR:-/tmp}/docket-stack.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

mkchange(){ # mkchange <id> <slug> <status> [stacked_on] [branch]
  cat > "$tmp/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: "Change $1"
status: $3
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
stacked_on: ${4:-}
branch: ${5:-}
---

## Why

Fixture.
EOF
}

mkchange 1 alpha implemented "" feat/alpha
mkchange 2 beta proposed 1
mkchange 3 gamma proposed 2

assert "an unstacked change has no parent" '[ -z "$(stack_parent_id "$tmp" 1)" ]'
assert "a stacked change names its parent" '[ "$(stack_parent_id "$tmp" 2)" = 1 ]'
assert "a padded id resolves to the same change" '[ "$(stack_parent_id "$tmp" 0002)" = 1 ]'
assert "a nested chain lists nearest parent first" '[ "$(stack_chain "$tmp" 3 | tr "\n" " ")" = "2 1 " ]'
assert "a well-formed chain exits 0" 'stack_chain "$tmp" 3 >/dev/null 2>&1'

# A padded id whose digits reach above 7 is where the octal trap actually bites: `0008` is not a
# valid octal literal, so a boundary missing its `10#` fails outright rather than off-by-a-value.
mkchange 8 theta proposed 1
assert "a padded id with a digit above 7 resolves" '[ "$(stack_parent_id "$tmp" 0008)" = 1 ]'
mkchange 9 iota proposed 0008
assert "a padded stacked_on value is canonicalized" '[ "$(stack_parent_id "$tmp" 9)" = 8 ]'
assert "a chain through a padded parent is well-formed" '[ "$(stack_chain "$tmp" 9 | tr "\n" " ")" = "8 1 " ]'

# a cycle: 4 -> 5 -> 4
mkchange 4 delta proposed 5
mkchange 5 epsilon proposed 4
assert "a cycle is refused" '! stack_chain "$tmp" 4 >/dev/null 2>&1'
cyc="$(stack_chain "$tmp" 4 2>&1 >/dev/null)"
assert "a cycle names the cycle on stderr" '[ -n "$(grep -F cycle <<<"$cyc")" ]'

# a self-cycle: a change stacked on itself
mkchange 18 sigma proposed 18
assert "a self-cycle is refused" '! stack_chain "$tmp" 18 >/dev/null 2>&1'

# A THREE-cycle: 21 -> 22 -> 23 -> 21. This is the leg that makes the visited-set ACCUMULATION
# load-bearing. The two-cycle above is caught by the seed alone (the walk returns to the starting
# id on its second hop), so a walk that recorded only its start would still refuse it; only a cycle
# that closes on a MIDDLE link needs each visited parent to have been recorded. Without the
# accumulation this walk does not terminate.
mkchange 21 phi proposed 22
mkchange 22 chi proposed 23
mkchange 23 psi proposed 21
assert "a three-cycle is refused" '! stack_chain "$tmp" 21 >/dev/null 2>&1'
assert "a three-cycle entered mid-ring is refused" '! stack_chain "$tmp" 22 >/dev/null 2>&1'

# A RHO-shaped chain — a tail leading into a ring the start is NOT part of: 24 -> 25 -> 26 -> 25.
# Every cycle above closes back on the STARTING id, so the seeded entry alone refuses all of them
# and the accumulation is never consulted. This is the only shape that consults it: the walk must
# remember 25 from its first hop to recognize it on its third. A walk that recorded only its start
# never terminates here, so this assert's mutation evidence is the run being killed by a timeout,
# not a NOT OK line.
mkchange 24 omega proposed 25
mkchange 25 aleph proposed 26
mkchange 26 beth proposed 25
assert "a cycle the start is not part of is refused" '! stack_chain "$tmp" 24 >/dev/null 2>&1'

# a missing parent
mkchange 6 zeta proposed 999
assert "a missing parent is refused" '! stack_chain "$tmp" 6 >/dev/null 2>&1'
miss="$(stack_chain "$tmp" 6 2>&1 >/dev/null)"
assert "a missing parent is named on stderr" '[ -n "$(grep -F "missing stacked_on parent" <<<"$miss")" ]'

# stack_find_file
assert "stack_find_file finds an active change" '[ "$(stack_find_file "$tmp" 1)" = "$tmp/active/0001-alpha.md" ]'
assert "stack_find_file accepts a padded id" '[ "$(stack_find_file "$tmp" 0008)" = "$tmp/active/0008-theta.md" ]'
cp "$tmp/active/0001-alpha.md" "$tmp/archive/2026-08-12-0019-tau.md"
assert "stack_find_file searches the archive" '[ "$(stack_find_file "$tmp" 19)" = "$tmp/archive/2026-08-12-0019-tau.md" ]'
assert "stack_find_file exits 1 on an absent id" '! stack_find_file "$tmp" 999 >/dev/null 2>&1'
assert "stack_find_file prints nothing on an absent id" '[ -z "$(stack_find_file "$tmp" 999 2>/dev/null)" ]'

# The absent-key hazard: the frontmatter omits stacked_on entirely and the BODY opens a line with
# it. The body value is a BARE ID and nothing else — the discriminating shape. A body line reading
# `stacked_on: 42 is discussed as prose` would be rejected downstream by the numeric guard and would
# therefore leave an unanchored read looking correct; only a value that survives every later check
# proves the read itself is anchored.
cat > "$tmp/active/0007-eta.md" <<'EOF'
---
id: 7
slug: eta
title: "Change 7"
status: proposed
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
---

## Why

A stacked change writes

stacked_on: 42

into its frontmatter, never into its body.
EOF
assert "an absent stacked_on does not fall through to body prose" '[ -z "$(stack_parent_id "$tmp" 7)" ]'
assert "an absent stacked_on leaves the chain well-formed" 'stack_chain "$tmp" 7 >/dev/null 2>&1'

# a non-numeric stacked_on is not a parent
cat > "$tmp/active/0020-upsilon.md" <<'EOF'
---
id: 20
slug: upsilon
title: "Change 20"
status: proposed
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
stacked_on: "not-an-id"
branch:
---

## Why

Fixture.
EOF
assert "a non-numeric stacked_on names no parent" '[ -z "$(stack_parent_id "$tmp" 20)" ]'
# Emptiness alone does not prove the value was REJECTED — an arithmetic error also produces empty
# stdout, so the stdout assert above passes just as well when the numeric guard is gone. Stderr is
# what separates "declined the value" from "tripped over it": the shape a caller must not see is a
# raw bash arithmetic diagnostic leaking out of a routine every renderer calls on every file.
mal_err="$(stack_parent_id "$tmp" 20 2>&1 >/dev/null)"
assert "a non-numeric stacked_on is declined quietly, not an arithmetic error" '[ -z "$mal_err" ]'
assert "a non-numeric stacked_on still exits 0" 'stack_parent_id "$tmp" 20 >/dev/null 2>&1'
assert "a non-numeric stacked_on leaves the chain well-formed" 'stack_chain "$tmp" 20 >/dev/null 2>&1'

# --- effective base resolution (spec §3) ---
# The stub knows ORIGIN refs only, and only the branches named in DOCKET_TEST_REMOTE_BRANCHES. That
# narrowness is deliberate twice over: it makes rule 1's remote-ref conjunct observable (a branch
# name alone never satisfies it), and it makes the --remote flag observable (a lookup under any
# other remote finds nothing, so a flag that is not threaded through to the ref path reddens).
GIT_STUB="$tmp/bin"; mkdir -p "$GIT_STUB"
cat > "$GIT_STUB/git" <<'EOF'
#!/usr/bin/env bash
# stub: `git show-ref --verify --quiet refs/remotes/origin/<b>` succeeds only for listed branches
if [ "$1" = show-ref ]; then
  for b in $DOCKET_TEST_REMOTE_BRANCHES; do
    case " $* " in (*" refs/remotes/origin/$b "*) exit 0 ;; esac
  done
  exit 1
fi
exit 0
EOF
chmod +x "$GIT_STUB/git"
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha"
GIT="$GIT_STUB/git"; export GIT

assert "rule 1: a live parent with a pushed branch resolves to that branch" \
  '[ "$(stack_effective_base "$tmp" 2 main)" = "feat/alpha" ]'
assert "an unstacked change resolves to the integration branch" \
  '[ "$(stack_effective_base "$tmp" 1 main)" = "main" ]'
# The integration branch is a PARAMETER, not the literal `main`: a consuming repo on `develop` must
# get `develop` back, and an assert that only ever passes `main` cannot tell the two apart.
assert "the integration branch is taken from the argument, not hardcoded" \
  '[ "$(stack_effective_base "$tmp" 1 develop)" = "develop" ]'

# rule 1 is remote-ref gated: an in-progress parent whose branch was never pushed is NOT a base
mkchange 10 iota in-progress "" feat/iota
mkchange 11 kappa proposed 10
assert "rule 4: a branch with no remote ref is invalid" \
  'stack_effective_base "$tmp" 11 main >/dev/null 2>&1; [ "$?" = 4 ]'

# rule 2: a merged parent resolves upward
mkchange 12 lambda done "" feat/lambda
mkchange 13 mu proposed 12
assert "rule 2: a done parent resolves to the integration branch" \
  '[ "$(stack_effective_base "$tmp" 13 main)" = "main" ]'

# rule 2, nested: grandparent still live
mkchange 14 nu stacked-merged 1 feat/nu
mkchange 15 xi proposed 14
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha"
assert "rule 2 nested: a stacked-merged parent whose branch is gone resolves to the grandparent" \
  '[ "$(stack_effective_base "$tmp" 15 main)" = "feat/alpha" ]'
# …and while that branch IS still pushed, the stacked-merged parent is itself the base: the fallback
# is a fallback, not the rule. Without this leg the arm above passes just as well for a resolver
# that ignores a stacked-merged parent's branch entirely.
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha feat/nu"
assert "a stacked-merged parent whose branch is still pushed is itself the base" \
  '[ "$(stack_effective_base "$tmp" 15 main)" = "feat/nu" ]'
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha"

# rule 3: killed parent
mkchange 16 omicron killed "" feat/omicron
mkchange 17 pi proposed 16
assert "rule 3: a killed parent stops with exit 3" \
  'stack_effective_base "$tmp" 17 main >/dev/null 2>&1; [ "$?" = 3 ]'

assert "rule 4: a cycle is invalid" \
  'stack_effective_base "$tmp" 4 main >/dev/null 2>&1; [ "$?" = 4 ]'
assert "rule 4: a missing parent is invalid" \
  'stack_effective_base "$tmp" 6 main >/dev/null 2>&1; [ "$?" = 4 ]'
# A cycle whose members have ALREADY MERGED is the shape that makes the up-front stack_chain refusal
# load-bearing. Every cycle fixture above terminates by accident: an empty `branch:` makes rule 4
# fire on the first hop, before any recursion begins. Here rule 2 applies at every hop instead, so a
# resolver that skipped the validation would walk the ring forever. As with the rho-chain leg above,
# this assert's mutation evidence is the run failing to terminate, not a NOT OK line.
mkchange 27 gimel done 28
mkchange 28 dalet done 27
assert "a cycle of merged parents is refused rather than walked forever" \
  'stack_effective_base "$tmp" 27 main >/dev/null 2>&1; [ "$?" = 4 ]'
# An invalid resolution prints NO branch: a caller that reads stdout and ignores the status must not
# be handed a plausible-looking base.
assert "an invalid resolution prints no base on stdout" \
  '[ -z "$(stack_effective_base "$tmp" 11 main 2>/dev/null)" ]'
assert "a killed parent prints no base on stdout" \
  '[ -z "$(stack_effective_base "$tmp" 17 main 2>/dev/null)" ]'

# The remote is a parameter too — under a remote the stub does not know, rule 1 cannot be satisfied.
assert "the remote argument reaches the ref lookup" \
  'stack_effective_base "$tmp" 2 main upstream >/dev/null 2>&1; [ "$?" = 4 ]'

# --- the CLI ---
assert "the CLI exists and is executable" '[ -x "$SCRIPT" ]'
assert "CLI prints the resolved base" \
  '[ "$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 2 --integration-branch main)" = "feat/alpha" ]'
assert "CLI accepts a padded id" \
  '[ "$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 0002 --integration-branch main)" = "feat/alpha" ]'
# `0008` is where the octal trap actually bites: it is not a valid octal literal, so a boundary
# missing its `10#` fails outright rather than resolving to a merely wrong value.
assert "CLI accepts a padded id with a digit above 7" \
  '[ "$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 0008 --integration-branch main)" = "feat/alpha" ]'
assert "CLI exits 3 on a killed parent" \
  'GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 17 --integration-branch main >/dev/null 2>&1; [ "$?" = 3 ]'
# `0008` above proves the CLI does not CRASH on a padded id, but not that it resolved the id it was
# handed: the library canonicalizes again downstream, so a CLI boundary missing its `10#` still
# reaches the right change for a value bash happens to parse. `0017` is the shape that separates
# them — it IS valid octal, so an uncanonicalized boundary silently resolves change FIFTEEN and
# reports its base at exit 0 instead of stopping on seventeen's killed parent.
assert "CLI resolves a padded id by decimal, not octal" \
  'GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 0017 --integration-branch main >/dev/null 2>&1; [ "$?" = 3 ]'
# The diagnostic names the change the caller asked about, padded — a stderr line that read `0015`
# would send a human to the wrong file.
killed_err="$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 0017 --integration-branch main 2>&1 >/dev/null)"
assert "the exit-3 diagnostic names the change and its remedy" \
  '[ -n "$(grep -F "0017" <<<"$killed_err")" ] && [ -n "$(grep -F "KILLED" <<<"$killed_err")" ]'
assert "CLI exits 4 on an unresolvable branch" \
  'GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 11 --integration-branch main >/dev/null 2>&1; [ "$?" = 4 ]'
assert "CLI passes --remote through to the ref lookup" \
  'GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 2 --integration-branch main --remote upstream >/dev/null 2>&1; [ "$?" = 4 ]'
assert "CLI exits 2 on a missing required flag" \
  '"$SCRIPT" --changes-dir "$tmp" >/dev/null 2>&1; [ "$?" = 2 ]'
assert "CLI exits 2 on an unknown flag" \
  '"$SCRIPT" --changes-dir "$tmp" --id 2 --integration-branch main --nope >/dev/null 2>&1; [ "$?" = 2 ]'
assert "CLI exits 2 on a non-numeric id" \
  '"$SCRIPT" --changes-dir "$tmp" --id not-a-number --integration-branch main >/dev/null 2>&1; [ "$?" = 2 ]'
assert "CLI exits 2 on a changes dir that does not exist" \
  '"$SCRIPT" --changes-dir "$tmp/nope" --id 2 --integration-branch main >/dev/null 2>&1; [ "$?" = 2 ]'
help_txt="$("$SCRIPT" --help 2>/dev/null)"; help_rc=$?
assert "CLI --help exits 0" '[ "$help_rc" = 0 ]'
assert "CLI --help prints its own header" '[ -n "$(grep -F stack-base.sh <<<"$help_txt")" ]'

printf '%s\n' "--- done"
exit "$fail"
