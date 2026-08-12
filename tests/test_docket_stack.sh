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

printf '%s\n' "--- done"
exit "$fail"
