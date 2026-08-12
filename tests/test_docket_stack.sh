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
#
# The stub CONSUMES the `-C DIR` prefix rather than ignoring it, and records the directory it was
# addressed at, so the addressing is observable to an assert instead of being silently equivalent
# to its absence — which is exactly how an un-addressed call survived this file before.
GIT_STUB="$tmp/bin"; mkdir -p "$GIT_STUB"
cat > "$GIT_STUB/git" <<'EOF'
#!/usr/bin/env bash
# stub: `git [-C DIR] show-ref --verify --quiet refs/remotes/origin/<b>` succeeds only for listed
# branches. The DIR of every invocation is appended to $DOCKET_TEST_GIT_LOG, `-` when unaddressed.
if [ "${1:-}" = -C ]; then
  printf '%s\n' "$2" >> "${DOCKET_TEST_GIT_LOG:-/dev/null}"; shift 2
else
  printf '%s\n' '-' >> "${DOCKET_TEST_GIT_LOG:-/dev/null}"
fi
if [ "${1:-}" = show-ref ]; then
  for b in $DOCKET_TEST_REMOTE_BRANCHES; do
    case " $* " in (*" refs/remotes/origin/$b "*) exit 0 ;; esac
  done
  exit 1
fi
exit 0
EOF
chmod +x "$GIT_STUB/git"
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha"
DOCKET_TEST_GIT_LOG="$tmp/git-argv.log"; export DOCKET_TEST_GIT_LOG
: > "$DOCKET_TEST_GIT_LOG"
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

# --- WHICH repo answers rule 1: the changes-dir's, never the caller's cwd ---
# Two independent legs. First, over the stub: the ref lookup must be ADDRESSED at the changes dir
# it was handed, which the stub's argv log makes visible.
: > "$DOCKET_TEST_GIT_LOG"
stack_effective_base "$tmp" 2 main >/dev/null 2>&1
argv_dir="$(cat "$DOCKET_TEST_GIT_LOG")"
assert "the ref lookup is addressed at the changes dir it was handed" \
  '[ "$argv_dir" = "$tmp" ]'
# The argv assert proves the flag is SPELLED; it cannot prove real git honours it end to end, and a
# stub is free to disagree with git about what `-C` means. So the second leg drops the stub
# entirely: a REAL git over two REAL repos. The fixture repo carries
# `refs/remotes/origin/feat/real`, the decoy repo (the cwd the resolver is run from) does not. Drop
# the `-C` and the lookup fails against the decoy, which is indistinguishable from "not pushed" —
# rule 1's positive conjunct — so the resolver answers exit 4 and the resolved-name assert reddens.
REAL_REPO="$tmp/realrepo"; DECOY_REPO="$tmp/decoyrepo"
REAL_CD="$REAL_REPO/docs/changes"
mkdir -p "$REAL_CD/active" "$REAL_CD/archive"
git init -q "$REAL_REPO" >/dev/null 2>&1
git init -q "$DECOY_REPO" >/dev/null 2>&1
git -C "$REAL_REPO" -c user.email=t@t -c user.name=t commit -q --allow-empty -m base >/dev/null 2>&1
git -C "$REAL_REPO" update-ref refs/remotes/origin/feat/real "$(git -C "$REAL_REPO" rev-parse HEAD)"
mkchange_in(){ # mkchange_in <changes-dir> <id> <slug> <status> [stacked_on] [branch]
  local d="$1"; shift
  cat > "$d/active/$(printf '%04d' "$1")-$2.md" <<EOF
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
mkchange_in "$REAL_CD" 40 real-parent in-progress "" feat/real
mkchange_in "$REAL_CD" 41 real-child proposed 40
# Non-vacuity: the decoy must genuinely lack the ref, or the assert below would pass for a resolver
# that reads the cwd's repo as well as for one that reads the changes-dir's.
assert "the decoy repo does not carry the parent's remote ref" \
  '! git -C "$DECOY_REPO" show-ref --verify --quiet refs/remotes/origin/feat/real'
cwd_base="$( cd "$DECOY_REPO" && GIT=git stack_effective_base "$REAL_CD" 41 main 2>/dev/null )"
assert "rule 1 is answered by the changes-dir's repo, not the caller's cwd" \
  '[ "$cwd_base" = "feat/real" ]'

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

# --- the descendant CLI (stack-children.sh) ------------------------------------------------------
# WHY THIS EXISTS AT ALL: spec §11 says the finalize open-children gate "derives the child set by
# scanning … never by reading a parent-side list", and `stack_descendants` was reachable only from
# inside stack-closeout.sh. The one parent-side artifact — the derived `## Stacked children` row —
# regenerates on a link-bearing write to THE PARENT, so a child stacked on an already-`implemented`
# parent (the spec's motivating case) is created after the parent's last such write and never
# appears in it. A gate keyed on that row is a gate that reads green while the branch it is about to
# delete carries open child PRs. These asserts pin the live scan the gate reads instead.
KIDS="$REPO/scripts/stack-children.sh"
ktmp="$(mktemp -d "${TMPDIR:-/tmp}/docket-stack-kids.XXXXXX")"
trap 'rm -rf "$tmp" "$ktmp"' EXIT
mkdir -p "$ktmp/active" "$ktmp/archive"
mkkid(){ # mkkid <id> <slug> <status> [stacked_on] [pr]
  cat > "$ktmp/active/$(printf '%04d' "$1")-$2.md" <<EOF
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
branch: feat/$2
pr: ${5:-}
---

## Why

Fixture.
EOF
}
mkkid 30 root implemented
mkkid 31 kid-open in-progress 30 101
mkkid 32 grandkid stacked-merged 31 102
mkkid 33 kid-done done 30 103
mkkid 34 kid-killed killed 30 104
mkkid 35 kid-nopr proposed 30
# The octal discriminator: `030` IS valid octal (24), so a boundary missing its `10#` resolves
# change TWENTY-FOUR — which exists here and is childless — and reports "no descendants" at exit 0
# for a root that has five. An id that merely fails to parse would be caught by any error path.
mkkid 24 decoy proposed
mkkid 36 prose-child proposed
# A body-prose `stacked_on:` line with a bare id: the shape the anchored read exists to reject, and
# the shape the whole-tree prefilter hands it. A phantom child here would hard-block a finalize that
# has nothing to block on.
cat >> "$ktmp/active/0036-prose-child.md" <<'EOF'

A stacked change writes

stacked_on: 30

into its frontmatter, never into its body.
EOF
cat > "$ktmp/archive/2026-08-12-0037-archived-kid.md" <<'EOF'
---
id: 37
slug: archived-kid
title: "Change 37"
status: done
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
stacked_on: 30
branch: feat/archived-kid
pr: 107
---

## Why

Fixture.
EOF

assert "the descendants CLI exists and is executable" '[ -x "$KIDS" ]'
kids_all="$("$KIDS" --changes-dir "$ktmp" --id 30 2>/dev/null)"
kids_ids="$(awk '{print $1}' <<<"$kids_all" | tr '\n' ' ')"
assert "it lists every transitive descendant, padded" \
  '[ "$kids_ids" = "0031 0033 0034 0035 0037 0032 " ]'
# Parents before children is the close-out's promotion order and the gate's reporting order; the
# grandchild must not surface before the child it hangs off.
assert "it emits parents before children" \
  '[ "$(grep -n -E -e "^0031 " <<<"$kids_all" | cut -d: -f1)" -lt "$(grep -n -E -e "^0032 " <<<"$kids_all" | cut -d: -f1)" ]'
assert "each row carries the child's status and PR" \
  '[ -n "$(grep -xF "0031 in-progress 101" <<<"$kids_all")" ]'
assert "a child with no pr: renders the placeholder, keeping the row three-column" \
  '[ -n "$(grep -xF "0035 proposed -" <<<"$kids_all")" ]'
assert "a body-prose stacked_on line does not invent a child" \
  '[ -z "$(grep -E -e "^0036 " <<<"$kids_all")" ]'
assert "an archived descendant is found by the scan too" \
  '[ -n "$(grep -E -e "^0037 " <<<"$kids_all")" ]'
# --open-only is spec §8's gate set: everything a merge would strand. Terminal statuses come from
# the shared DOCKET_STATUSES_TERMINAL vocabulary, plus `stacked-merged`, which rides the merge.
kids_open="$("$KIDS" --changes-dir "$ktmp" --id 30 --open-only 2>/dev/null)"
assert "--open-only keeps a non-terminal child" '[ -n "$(grep -E -e "^0031 " <<<"$kids_open")" ]'
assert "--open-only drops a done child" '[ -z "$(grep -E -e "^0033 " <<<"$kids_open")" ]'
assert "--open-only drops a killed child" '[ -z "$(grep -E -e "^0034 " <<<"$kids_open")" ]'
# The one that is not a terminal status: `stacked-merged` is ACTIVE, so a filter written as
# "drop the terminal ones" alone leaves it in and hard-blocks every finalize of a stack root — the
# exact merge the close-out exists to let through.
assert "--open-only drops a stacked-merged child, which rides the merge" \
  '[ -z "$(grep -E -e "^0032 " <<<"$kids_open")" ]'
assert "a childless change prints nothing at exit 0" \
  'out="$("$KIDS" --changes-dir "$ktmp" --id 24)"; rc=$?; [ "$rc" = 0 ] && [ -z "$out" ]'
# An id nobody has is the failure mode the library cannot report: stack_descendants answers an
# unknown root with the same empty stdout as a childless one, so a typo'd id would read as
# "nothing to block on" — the gate passing for the wrong reason.
assert "an id that names no change exits 4, never a silent empty answer" \
  '"$KIDS" --changes-dir "$ktmp" --id 999 >/dev/null 2>&1; [ "$?" = 4 ]'
assert "the exit-4 diagnostic names the change and prints nothing on stdout" \
  'err="$("$KIDS" --changes-dir "$ktmp" --id 999 2>&1 >/dev/null)"; \
   out="$("$KIDS" --changes-dir "$ktmp" --id 999 2>/dev/null)"; \
   [ -n "$(grep -F "0999" <<<"$err")" ] && [ -z "$out" ]'
assert "it resolves a padded id by decimal, not octal" \
  '[ "$("$KIDS" --changes-dir "$ktmp" --id 0030 2>/dev/null | awk "{print \$1}" | tr "\n" " ")" = "$kids_ids" ]'
assert "it exits 2 on a missing required flag" \
  '"$KIDS" --changes-dir "$ktmp" >/dev/null 2>&1; [ "$?" = 2 ]'
assert "it exits 2 on an unknown flag" \
  '"$KIDS" --changes-dir "$ktmp" --id 30 --nope >/dev/null 2>&1; [ "$?" = 2 ]'
assert "it exits 2 on a non-numeric id" \
  '"$KIDS" --changes-dir "$ktmp" --id not-a-number >/dev/null 2>&1; [ "$?" = 2 ]'
assert "it exits 2 on a changes dir that does not exist" \
  '"$KIDS" --changes-dir "$ktmp/nope" --id 30 >/dev/null 2>&1; [ "$?" = 2 ]'
kids_help="$("$KIDS" --help 2>/dev/null)"; kids_help_rc=$?
assert "its --help exits 0" '[ "$kids_help_rc" = 0 ]'
assert "its --help prints its own header" '[ -n "$(grep -F stack-children.sh <<<"$kids_help")" ]'

# --- the gate that consumes it -------------------------------------------------------------------
# The open-children gate and the child-PR retarget are PROSE — nothing else in the suite reads them,
# and prose that names no oracle is what let the gate key on a rendered row. So: the reference
# section that owns the gate must carry a fenced facade invocation of the op, and the required-flag
# set is DERIVED from the script's own validation block, so a flag added there without reaching the
# documented invocation reddens on arrival.
STACKREF="$REPO/skills/docket-convention/references/stacked-changes.md"
FINSK="$REPO/skills/docket-finalize-change/SKILL.md"
kids_validation="$(grep -E 'die "missing --' "$KIDS")"
KIDS_REQUIRED="$(grep -oE -e '--[a-z][a-z-]+' <<<"$kids_validation" | sort -u)"
assert "the required-flag derivation found a plausible flag set" \
  '[ "$(grep -c . <<<"$KIDS_REQUIRED")" -ge 2 ]'
gate_section="$(awk '/^## Finalizing a parent that has open children/{s=1;next} s&&/^## /{exit} s' "$STACKREF")"
assert "the reference's open-children gate section exists" '[ -n "${gate_section// /}" ]'
# The fence literal lives in a SINGLE-quoted awk variable: no backtick may sit inside double quotes
# in test source (change 0221, scripts/check-test-source-hygiene.sh).
gate_block="$(awk -v f='```' 'index($0,f)==1{b=!b;next} b' <<<"$gate_section")"
assert "the gate section carries a fenced facade invocation of the descendants op" \
  'grep -qF "docket.sh stack-children" <<<"$gate_block"'
for flag in $KIDS_REQUIRED; do
  assert "the gate command names $flag" 'grep -qF -- "$flag" <<<"$gate_block"'
done
assert "the gate command asks for the OPEN subset, not the whole graph" \
  'grep -qF -- "--open-only" <<<"$gate_block"'
# Finalize's own body must reach the op too: its step 3.5 close-out gate ("does this change have
# stacked descendants") had the same rendered-row oracle, and a trigger nobody can evaluate is a
# step nobody runs.
fin_flat="$(tr -s '[:space:]' ' ' < "$FINSK")"
assert "docket-finalize-change names the descendants op" \
  'grep -qF "docket.sh stack-children" <<<"$fin_flat"'

printf '%s\n' "--- done"
exit "$fail"
