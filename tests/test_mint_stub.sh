#!/usr/bin/env bash
# tests/test_mint_stub.sh — verifies change 0091: scripts/mint-stub.sh, the deterministic
# discovered-work stub mint. Hermetic: a temp repo with a local *bare* origin parked on the docket
# branch so the CAS push actually lands; TODAY is mocked so the created/updated stamps are stable.
# Run: bash tests/test_mint_stub.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/mint-stub.sh"
TEMPLATE="$REPO/skills/docket-new-change/change-template.md"
# shellcheck source=/dev/null
. "$REPO/scripts/lib/docket-frontmatter.sh"   # field / list_field / int_field for the assertions
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
git_quiet(){ git "$@" >/dev/null 2>&1; }
FIXED_DAY=2026-07-18

new_repo(){
  local root origin work
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git_quiet -C "$work" checkout --orphan docket
  git_quiet -C "$work" rm -rf . || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive"
  echo baseline > "$work/docs/changes/.keep"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket baseline"
  git_quiet -C "$work" push -u origin docket
  printf '%s\n' "$work"
}

# mkchange WORK DIR FILE ID SLUG TITLE — write a minimal change file into active/ or archive/.
mkchange(){
  local work="$1" dir="$2" file="$3" id="$4" slug="$5" title="$6"
  cat > "$work/docs/changes/$dir/$file" <<EOF
---
id: $id
slug: $slug
title: $title
status: proposed
---

## Why
seed
EOF
}

body(){ local f; f="$(mktemp)"; printf '## Why\n\n%s\n\n## What changes\n\n- thing\n' "$1" > "$f"; printf '%s' "$f"; }

run_mint(){ # run_mint WORK [extra args...]
  local work="$1"; shift
  TODAY="$FIXED_DAY" "$SCRIPT" --changes-dir "$work/docs/changes" --template "$TEMPLATE" "$@"
}

# --- (A) happy path: allocates max+1, writes the stub, pushes, reports ------------------------
W="$(new_repo)"
mkchange "$W" active  0007-alpha.md 7 alpha "Alpha"
mkchange "$W" archive 2026-07-01-0012-beta.md 12 beta "Beta"
git -C "$W" add -A; git_quiet -C "$W" commit -m seed; git_quiet -C "$W" push origin docket
B="$(body 'discovered while building #91')"
outA="$(run_mint "$W" --title "Cap the widget" --body-file "$B" --discovered-from 91 2>&1)"; rcA=$?
NEW="$W/docs/changes/active/0013-cap-the-widget.md"
assert "A: exit 0"                 '[ "$rcA" -eq 0 ]'
assert "A: reports minted 13"      '[ "$outA" = "minted 13 cap-the-widget" ]'
assert "A: file created at max+1"  '[ -f "$NEW" ]'
assert "A: id field is 13"         '[ "$(int_field "$NEW" id)" = "13" ]'
assert "A: slug field"             '[ "$(field "$NEW" slug)" = "cap-the-widget" ]'
assert "A: title field"            '[ "$(field "$NEW" title)" = "Cap the widget" ]'
assert "A: status proposed"        '[ "$(field "$NEW" status)" = "proposed" ]'
assert "A: discovered_from set"    '[ "$(list_field "$NEW" discovered_from)" = "91" ]'
assert "A: created stamped"        '[ "$(field "$NEW" created)" = "$FIXED_DAY" ]'
assert "A: updated stamped"        '[ "$(field "$NEW" updated)" = "$FIXED_DAY" ]'
assert "A: needs-brainstorm (no spec)" '[ -z "$(field "$NEW" spec)" ]'
assert "A: not trivial"            '[ "$(field "$NEW" trivial)" = "false" ]'
assert "A: auto_groomable left unset" '[ -z "$(field "$NEW" auto_groomable)" ]'
assert "A: body carried through"   'grep -qF "discovered while building #91" "$NEW"'
assert "A: Artifacts markers present" \
  'grep -qF "docket:artifacts:start" "$NEW" && grep -qF "docket:artifacts:end" "$NEW"'
assert "A: pushed to origin" \
  '[ "$(git -C "$W" rev-parse HEAD)" = "$(git -C "$W" rev-parse origin/docket)" ]'
assert "A: working tree clean after mint" '[ -z "$(git -C "$W" status --porcelain)" ]'
assert "A: commit touched ONLY the new change file" \
  '[ "$(git -C "$W" show --name-only --format= HEAD | grep -c .)" -eq 1 ]'

# --- (B) dedup against an ACTIVE slug (case-insensitive) --------------------------------------
W2="$(new_repo)"
mkchange "$W2" active 0004-cap-the-widget.md 4 cap-the-widget "Cap The Widget"
git -C "$W2" add -A; git_quiet -C "$W2" commit -m seed; git_quiet -C "$W2" push origin docket
B2="$(body dup)"
outB="$(run_mint "$W2" --title "CAP the WIDGET" --body-file "$B2" --discovered-from 91 2>&1)"; rcB=$?
assert "B: exit 3 on duplicate"    '[ "$rcB" -eq 3 ]'
assert "B: reports the match"      '[ "$outB" = "skipped duplicate cap-the-widget (matches #4)" ]'
assert "B: no new file"            '[ "$(ls "$W2/docs/changes/active" | grep -c .)" -eq 1 ]'
assert "B: no new commit"          '[ -z "$(git -C "$W2" status --porcelain)" ]'

# --- (B2) an ARCHIVED slug is NOT a duplicate (dedup is active-only, by spec §5) ---------------
W2b="$(new_repo)"
mkchange "$W2b" archive 2026-07-01-0004-cap-the-widget.md 4 cap-the-widget "Cap The Widget"
git -C "$W2b" add -A; git_quiet -C "$W2b" commit -m seed; git_quiet -C "$W2b" push origin docket
B2b="$(body notdup)"
outB2="$(run_mint "$W2b" --title "Cap the widget" --body-file "$B2b" --discovered-from 91 2>&1)"; rcB2=$?
assert "B2: archived slug does not block the mint" '[ "$rcB2" -eq 0 ]'
assert "B2: minted 5"                              '[ "$outB2" = "minted 5 cap-the-widget" ]'

# --- (C) cap ---------------------------------------------------------------------------------
W3="$(new_repo)"
B3="$(body capped)"
outC="$(run_mint "$W3" --title "Fourth thing" --body-file "$B3" --discovered-from 91 --minted 3 2>&1)"; rcC=$?
assert "C: exit 4 at cap"        '[ "$rcC" -eq 4 ]'
assert "C: reports cap-reached"  '[ "$outC" = "skipped cap-reached (cap 3, minted 3)" ]'
assert "C: nothing written"      '[ "$(ls "$W3/docs/changes/active" | grep -c .)" -eq 0 ]'
outC2="$(run_mint "$W3" --title "Third thing" --body-file "$B3" --discovered-from 91 --minted 2 2>&1)"; rcC2=$?
assert "C: under the cap still mints" '[ "$rcC2" -eq 0 ]'

# --- (D) CAS: a competing writer takes the id first; the retry RE-ALLOCATES from fresh origin ---
# The competing commit must DIVERGE the same contended path (an id), or the retry branch is never
# exercised (learnings: green-suite-untested-branch).
W4="$(new_repo)"
mkchange "$W4" active 0007-alpha.md 7 alpha "Alpha"
git -C "$W4" add -A; git_quiet -C "$W4" commit -m seed; git_quiet -C "$W4" push origin docket
OTHER="$(mktemp -d)/other"; git_quiet clone "$(git -C "$W4" remote get-url origin)" "$OTHER"
git -C "$OTHER" config user.email o@o; git -C "$OTHER" config user.name o
git_quiet -C "$OTHER" checkout docket
mkchange "$OTHER" active 0008-competitor.md 8 competitor "Competitor"
git -C "$OTHER" add -A; git_quiet -C "$OTHER" commit -m "competing mint"; git_quiet -C "$OTHER" push origin docket
B4="$(body race)"
outD="$(run_mint "$W4" --title "Raced thing" --body-file "$B4" --discovered-from 91 2>&1)"; rcD=$?
assert "D: exit 0 after the race"      '[ "$rcD" -eq 0 ]'
assert "D: re-allocated to 9, not 8"   '[ "$outD" = "minted 9 raced-thing" ]'
assert "D: file is 0009-raced-thing"   '[ -f "$W4/docs/changes/active/0009-raced-thing.md" ]'
assert "D: stale 0008 name not left behind" '[ ! -f "$W4/docs/changes/active/0008-raced-thing.md" ]'
assert "D: competitor survived"        '[ -f "$W4/docs/changes/active/0008-competitor.md" ]'
assert "D: converged with origin" \
  '[ "$(git -C "$W4" rev-parse HEAD)" = "$(git -C "$W4" rev-parse origin/docket)" ]'

# --- (E) argument validation ------------------------------------------------------------------
W5="$(new_repo)"
assert "E: missing --title fails"       '! run_mint "$W5" --body-file "$(body x)" --discovered-from 91 >/dev/null 2>&1'
assert "E: missing --body-file fails"   '! run_mint "$W5" --title T --discovered-from 91 >/dev/null 2>&1'
assert "E: missing --discovered-from fails" '! run_mint "$W5" --title T --body-file "$(body x)" >/dev/null 2>&1'
assert "E: non-numeric --discovered-from fails" \
  '! run_mint "$W5" --title T --body-file "$(body x)" --discovered-from nine >/dev/null 2>&1'
BAD="$(mktemp)"; printf 'no heading here\n' > "$BAD"
assert "E: body not starting with ## Why fails" \
  '! run_mint "$W5" --title T --body-file "$BAD" --discovered-from 91 >/dev/null 2>&1'
assert "E: a rejected run writes nothing" '[ "$(ls "$W5/docs/changes/active" | grep -c .)" -eq 0 ]'

exit $fail
