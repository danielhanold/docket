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
  # change 0127 made --type required. Inject a neutral default so the pre-0127 fixtures below keep
  # exercising what they were written to exercise; a test that cares about the type passes its own
  # --type, and the required-ness itself is proven by calling "$SCRIPT" directly (block T).
  local has_type=0 a
  for a in "$@"; do [ "$a" = --type ] && { has_type=1; break; }; done
  [ "$has_type" = 1 ] || set -- "$@" --type chore
  TODAY="$FIXED_DAY" "$SCRIPT" --changes-dir "$work/docs/changes" --template "$TEMPLATE" "$@"
}

mint_raw(){ # mint_raw WORK [args...] -> the script with NO injected --type, for arg-parsing and
            # required-ness fixtures that the injection would otherwise mask.
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

# --- (F) titles containing sed/awk metacharacters round-trip verbatim (B1) ---------------------
# set_field must never re-interpret '|', '&', or '\' in a model-authored title. Same repo across
# F1-F3 (each a fresh sequential mint), mirroring fixture C's chaining pattern.
WF="$(new_repo)"
BF1="$(body meta1)"
outF1="$(run_mint "$WF" --title 'Fix the thing | and the other' --body-file "$BF1" --discovered-from 91 2>&1)"; rcF1=$?
NEWF1="$WF/docs/changes/active/0001-fix-the-thing-and-the-other.md"
assert "F1: exit 0 with a pipe in the title"      '[ "$rcF1" -eq 0 ]'
assert "F1: minted line reports"                  '[ "$outF1" = "minted 1 fix-the-thing-and-the-other" ]'
assert "F1: title field exact (pipe)"             '[ "$(field "$NEWF1" title)" = "Fix the thing | and the other" ]'

BF2="$(body meta2)"
outF2="$(run_mint "$WF" --title 'Tabs & spaces' --body-file "$BF2" --discovered-from 91 2>&1)"; rcF2=$?
NEWF2="$WF/docs/changes/active/0002-tabs-spaces.md"
assert "F2: exit 0 with an ampersand in the title" '[ "$rcF2" -eq 0 ]'
assert "F2: minted line reports"                   '[ "$outF2" = "minted 2 tabs-spaces" ]'
assert "F2: title field exact (ampersand)"         '[ "$(field "$NEWF2" title)" = "Tabs & spaces" ]'

BF3="$(body meta3)"
outF3="$(run_mint "$WF" --title 'Handle back\slash paths' --body-file "$BF3" --discovered-from 91 2>&1)"; rcF3=$?
NEWF3="$WF/docs/changes/active/0003-handle-back-slash-paths.md"
assert "F3: exit 0 with a backslash in the title"  '[ "$rcF3" -eq 0 ]'
assert "F3: minted line reports"                   '[ "$outF3" = "minted 3 handle-back-slash-paths" ]'
assert "F3: title field exact (backslash)"         '[ "$(field "$NEWF3" title)" = "Handle back\slash paths" ]'

# --- (G) an EMPTY active/ plus a forced CAS retry must recreate the dir, not die (B2) -----------
# active/ starts with ZERO tracked files (git tracks no empty dirs); the competitor pushes only to
# archive/, so origin's tip still has no active/ file when our retry's reset --hard lands — pruning
# our own just-written stub's directory along with it, exactly like the reviewer's live repro.
WG="$(new_repo)"
OTHERG="$(mktemp -d)/otherg"; git_quiet clone "$(git -C "$WG" remote get-url origin)" "$OTHERG"
git -C "$OTHERG" config user.email o@o; git -C "$OTHERG" config user.name o
git_quiet -C "$OTHERG" checkout docket
mkdir -p "$OTHERG/docs/changes/archive"
mkchange "$OTHERG" archive 2026-01-01-0001-other.md 1 other-thing "Other thing"
git -C "$OTHERG" add -A; git_quiet -C "$OTHERG" commit -m "competing archive add"; git_quiet -C "$OTHERG" push origin docket
BG="$(body 'empty active dir race')"
outG="$(run_mint "$WG" --title "Empty dir race" --body-file "$BG" --discovered-from 91 2>&1)"; rcG=$?
assert "G: exit 0 despite active/ being pruned mid-retry" '[ "$rcG" -eq 0 ]'
assert "G: minted with re-derived id"                     '[ "$outG" = "minted 2 empty-dir-race" ]'
assert "G: stub file present after mkdir-recovery" \
  '[ -f "$WG/docs/changes/active/0002-empty-dir-race.md" ]'
assert "G: converged with origin" \
  '[ "$(git -C "$WG" rev-parse HEAD)" = "$(git -C "$WG" rev-parse origin/docket)" ]'

# --- (H) a real (non-race) push failure dies immediately, no unpushed commit left (B4) ----------
WH="$(new_repo)"
git -C "$WH" remote set-url origin /nonexistent/path/does-not-exist.git
BH="$(body 'real failure not a race')"
outH="$(run_mint "$WH" --title "Real failure" --body-file "$BH" --discovered-from 91 2>&1)"; rcH=$?
assert "H: exits nonzero on a real push failure" '[ "$rcH" -ne 0 ]'
assert "H: diagnosed as not-a-race, not exhaustion" \
  'case "$outH" in *"not a lost race"*) true ;; *) false ;; esac'
assert "H: no unpushed commit left behind (HEAD == origin/docket)" \
  '[ "$(git -C "$WH" rev-parse HEAD)" = "$(git -C "$WH" rev-parse origin/docket)" ]'
assert "H: working tree clean after the failed mint" '[ -z "$(git -C "$WH" status --porcelain)" ]'

# --- (I) unrelated uncommitted work blocks the CAS reset instead of being wiped (B3) ------------
WI="$(new_repo)"
mkchange "$WI" active 0005-other.md 5 other-thing "Other thing"
git -C "$WI" add -A; git_quiet -C "$WI" commit -m seed; git_quiet -C "$WI" push origin docket
OTHERI="$(mktemp -d)/otheri"; git_quiet clone "$(git -C "$WI" remote get-url origin)" "$OTHERI"
git -C "$OTHERI" config user.email o@o; git -C "$OTHERI" config user.name o
git_quiet -C "$OTHERI" checkout docket
mkchange "$OTHERI" active 0006-competitor.md 6 competitor-i "Competitor I"
git -C "$OTHERI" add -A; git_quiet -C "$OTHERI" commit -m "competing mint"; git_quiet -C "$OTHERI" push origin docket
# simulate another agent mid-write in the SAME shared worktree: an uncommitted edit to an
# unrelated change file, present before mint-stub ever runs.
printf '\nextra uncommitted line\n' >> "$WI/docs/changes/active/0005-other.md"
BI="$(body 'dirty tree race')"
outI="$(run_mint "$WI" --title "Dirty tree race" --body-file "$BI" --discovered-from 91 2>&1)"; rcI=$?
assert "I: refuses rather than silently succeeding" '[ "$rcI" -ne 0 ]'
assert "I: diagnosed as a dirty-worktree refusal" \
  'case "$outI" in *"uncommitted changes from another writer"*) true ;; *) false ;; esac'
assert "I: unrelated uncommitted edit survives" \
  'grep -qF "extra uncommitted line" "$WI/docs/changes/active/0005-other.md"'
assert "I: unrelated file still shows as modified (not wiped)" \
  '[ -n "$(git -C "$WI" status --porcelain -- docs/changes/active/0005-other.md)" ]'

# --- (J) a long title's slug never ends in a trailing hyphen after truncation (B5) ---------------
# 12x "abcd-" is exactly 60 chars ending in '-'; a 13th word ("efgh") follows so the FULL
# (untruncated) slug does not end in '-' — only cut -c1-60 introduces the trailing hyphen.
WJ="$(new_repo)"
BJ="$(body 'long title truncation')"
LONGTITLE="Abcd Abcd Abcd Abcd Abcd Abcd Abcd Abcd Abcd Abcd Abcd Abcd Efgh"
outJ="$(run_mint "$WJ" --title "$LONGTITLE" --body-file "$BJ" --discovered-from 91 2>&1)"; rcJ=$?
EXPECT_SLUG="abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd"
assert "J: exit 0 for a long title"     '[ "$rcJ" -eq 0 ]'
assert "J: slug has no trailing hyphen" '[ "$outJ" = "minted 1 $EXPECT_SLUG" ]'
assert "J: file uses the trimmed slug"  '[ -f "$WJ/docs/changes/active/0001-$EXPECT_SLUG.md" ]'

# --- (K0) NEW-1: a plain multi-line --title (no injection payload) is rejected too ---------------
# The guard fires on the control character itself, not on recognizing an injection payload.
WK0="$(new_repo)"
BEFORE_K0="$(git -C "$WK0" rev-parse HEAD)"
BK0="$(body 'plain newline guard')"
PLAIN_MULTI_TITLE="$(printf 'Line one\nLine two')"
outK0="$(run_mint "$WK0" --title "$PLAIN_MULTI_TITLE" --body-file "$BK0" --discovered-from 91 2>&1)"; rcK0=$?
assert "K0: exit nonzero on a plain multi-line --title" '[ "$rcK0" -ne 0 ]'
assert "K0: nothing written to active/"         '[ "$(ls "$WK0/docs/changes/active" | grep -c .)" -eq 0 ]'
assert "K0: no commit created (HEAD unchanged)" '[ "$(git -C "$WK0" rev-parse HEAD)" = "$BEFORE_K0" ]'

# --- (K) NEW-1: a --title newline carrying a `trivial: true` payload is rejected before any write -
# A newline in --title would otherwise let set_field's ENVIRON write inject an arbitrary extra
# frontmatter line (e.g. `trivial: true`) ahead of the template's real fields — field() returns the
# FIRST match, so the injected line would win and readiness() would treat an ungroomed stub as
# build-ready. Reject before any write.
WK="$(new_repo)"
BEFORE_K="$(git -C "$WK" rev-parse HEAD)"
BK="$(body 'newline injection guard')"
MULTI_TITLE="$(printf 'Line one\ntrivial: true')"
outK="$(run_mint "$WK" --title "$MULTI_TITLE" --body-file "$BK" --discovered-from 91 2>&1)"; rcK=$?
assert "K: exit nonzero on multi-line --title" '[ "$rcK" -ne 0 ]'
assert "K: exactly one diagnostic line"        '[ "$(printf "%s" "$outK" | grep -c .)" -eq 1 ]'
assert "K: diagnostic mentions newline"        'case "$outK" in *newline*) true ;; *) false ;; esac'
assert "K: no 'minted' line printed"           'case "$outK" in *"minted "*) false ;; *) true ;; esac'
assert "K: nothing written to active/"         '[ "$(ls "$WK/docs/changes/active" | grep -c .)" -eq 0 ]'
assert "K: no commit created (HEAD unchanged)" '[ "$(git -C "$WK" rev-parse HEAD)" = "$BEFORE_K" ]'
assert "K: nothing pushed (origin unchanged)"  '[ "$(git -C "$WK" rev-parse origin/docket)" = "$BEFORE_K" ]'
assert "K: working tree clean"                 '[ -z "$(git -C "$WK" status --porcelain)" ]'
assert "K: the injected trivial:true never lands in any stub" \
  '! grep -rq "^trivial: true$" "$WK/docs/changes/active" "$WK/docs/changes/archive" 2>/dev/null'

# --- (K2) NEW-1: an explicit multi-line --slug is rejected the same way -------------------------
WK2="$(new_repo)"
BEFORE_K2="$(git -C "$WK2" rev-parse HEAD)"
BK2="$(body 'slug newline guard')"
MULTI_SLUG="$(printf 'line-one\ntrivial: true')"
outK2="$(run_mint "$WK2" --title "Fine title" --slug "$MULTI_SLUG" --body-file "$BK2" --discovered-from 91 2>&1)"; rcK2=$?
assert "K2: exit nonzero on multi-line --slug" '[ "$rcK2" -ne 0 ]'
assert "K2: exactly one diagnostic line"       '[ "$(printf "%s" "$outK2" | grep -c .)" -eq 1 ]'
assert "K2: diagnostic mentions newline"       'case "$outK2" in *newline*) true ;; *) false ;; esac'
assert "K2: nothing written to active/"        '[ "$(ls "$WK2/docs/changes/active" | grep -c .)" -eq 0 ]'
assert "K2: no commit created (HEAD unchanged)" '[ "$(git -C "$WK2" rev-parse HEAD)" = "$BEFORE_K2" ]'
assert "K2: nothing pushed (origin unchanged)"  '[ "$(git -C "$WK2" rev-parse origin/docket)" = "$BEFORE_K2" ]'

# --- (L) NEW-2: a contended race with a stray UNTRACKED file present still mints (availability) --
# reset --hard never removes untracked files, so their mere presence must not block the CAS retry.
# Companion negative control already lives at (I): a dirty TRACKED file still refuses and preserves
# the edit — that safety property must be unchanged by this fix.
WL="$(new_repo)"
mkchange "$WL" active 0007-alpha.md 7 alpha "Alpha"
git -C "$WL" add -A; git_quiet -C "$WL" commit -m seed; git_quiet -C "$WL" push origin docket
OTHERL="$(mktemp -d)/otherl"; git_quiet clone "$(git -C "$WL" remote get-url origin)" "$OTHERL"
git -C "$OTHERL" config user.email o@o; git -C "$OTHERL" config user.name o
git_quiet -C "$OTHERL" checkout docket
mkchange "$OTHERL" active 0008-competitor.md 8 competitor "Competitor"
git -C "$OTHERL" add -A; git_quiet -C "$OTHERL" commit -m "competing mint"; git_quiet -C "$OTHERL" push origin docket
# a stray untracked file (e.g. another agent's scratch note, or an editor swap file) sitting in the
# shared metadata worktree — present BEFORE mint-stub ever runs, same shape as fixture I's dirty
# tracked file, but untracked.
printf 'scratch\n' > "$WL/docs/changes/notes.tmp"
BL="$(body 'untracked file must not block reset --hard')"
outL="$(run_mint "$WL" --title "Untracked race" --body-file "$BL" --discovered-from 91 2>&1)"; rcL=$?
assert "L: exit 0 despite a stray untracked file"    '[ "$rcL" -eq 0 ]'
assert "L: re-allocated to 9, not 8"                 '[ "$outL" = "minted 9 untracked-race" ]'
assert "L: file is 0009-untracked-race"              '[ -f "$WL/docs/changes/active/0009-untracked-race.md" ]'
assert "L: untracked scratch file survives the reset" '[ -f "$WL/docs/changes/notes.tmp" ]'
assert "L: converged with origin"                    '[ "$(git -C "$WL" rev-parse HEAD)" = "$(git -C "$WL" rev-parse origin/docket)" ]'

# --- (M) --metadata-branch guard: a mismatched branch refuses before any write (data safety) ----
# A mis-pointed --changes-dir must not silently commit+push a stub onto the wrong branch.
WM="$(new_repo)"
BEFORE_M="$(git -C "$WM" rev-parse HEAD)"
BM="$(body 'metadata branch guard mismatch')"
outM="$(run_mint "$WM" --title "Guarded thing" --body-file "$BM" --discovered-from 91 \
  --metadata-branch not-docket 2>&1)"; rcM=$?
assert "M: exit nonzero on metadata-branch mismatch"    '[ "$rcM" -ne 0 ]'
assert "M: diagnostic names actual and expected branch" \
  'case "$outM" in *"docket"*"not-docket"*) true ;; *) false ;; esac'
assert "M: nothing written to active/"          '[ "$(ls "$WM/docs/changes/active" | grep -c .)" -eq 0 ]'
assert "M: no commit created (HEAD unchanged)"  '[ "$(git -C "$WM" rev-parse HEAD)" = "$BEFORE_M" ]'
assert "M: nothing pushed (origin unchanged)"   '[ "$(git -C "$WM" rev-parse origin/docket)" = "$BEFORE_M" ]'

# --- (M2) --metadata-branch positive control: a MATCHING branch still mints normally -------------
WM2="$(new_repo)"
BM2="$(body 'metadata branch guard match')"
outM2="$(run_mint "$WM2" --title "Allowed thing" --body-file "$BM2" --discovered-from 91 \
  --metadata-branch docket 2>&1)"; rcM2=$?
assert "M2: exit 0 when --metadata-branch matches" '[ "$rcM2" -eq 0 ]'
assert "M2: minted normally"                       '[ "$outM2" = "minted 1 allowed-thing" ]'

# --- (N) a detached HEAD refuses regardless of --metadata-branch (data safety) -------------------
WN="$(new_repo)"
git_quiet -C "$WN" checkout --detach HEAD
BEFORE_N="$(git -C "$WN" rev-parse HEAD)"
BN="$(body 'detached head guard')"
outN="$(run_mint "$WN" --title "Detached thing" --body-file "$BN" --discovered-from 91 2>&1)"; rcN=$?
assert "N: exit nonzero on detached HEAD"       '[ "$rcN" -ne 0 ]'
assert "N: diagnostic mentions detached"        'case "$outN" in *detached*) true ;; *) false ;; esac'
assert "N: nothing written to active/"          '[ "$(ls "$WN/docs/changes/active" | grep -c .)" -eq 0 ]'
assert "N: HEAD unchanged"                      '[ "$(git -C "$WN" rev-parse HEAD)" = "$BEFORE_N" ]'

# --- (O) a flag as the FINAL argument with no value dies cleanly through `die`, never a raw bash
# "unbound variable" trace -------------------------------------------------------------------------
WO="$(new_repo)"
outO="$(mint_raw "$WO" --title 2>&1)"; rcO=$?
assert "O: exit nonzero on a trailing flag with no value" '[ "$rcO" -ne 0 ]'
assert "O: diagnostic names the flag" \
  'case "$outO" in *"--title requires a value"*) true ;; *) false ;; esac'
assert "O: not a raw bash unbound-variable trace" \
  'case "$outO" in *"unbound variable"*) false ;; *) true ;; esac'
assert "O: nothing written to active/" '[ "$(ls "$WO/docs/changes/active" | grep -c .)" -eq 0 ]'

# --- (T) change 0127: --type ------------------------------------------------------------------
WT="$(new_repo)"
BT="$(body 'typed capture')"
outT="$(run_mint "$WT" --title "Typed thing" --body-file "$BT" --discovered-from 127 --type fix 2>&1)"
NEWT="$WT/docs/changes/active/0001-typed-thing.md"
assert "T: mint with --type succeeds" '[ -f "$NEWT" ]'
assert "T: type field carries the requested value" '[ "$(field "$NEWT" type)" = "fix" ]'
assert "T: type lands INSIDE the first frontmatter block" \
  '[ "$(awk "/^---\$/{n++; next} n==1 && /^type:/{print \"in\"; exit}" "$NEWT")" = in ]'
assert "T: the injected type does not disturb discovered_from" \
  '[ "$(list_field "$NEWT" discovered_from)" = "127" ]'

# Required-ness and shape gates below call mint_raw (defined beside run_mint) so the injected
# default cannot mask them.
WT2="$(new_repo)"; BT2="$(body 'x')"
assert "T: missing --type is a hard error" \
  '! mint_raw "$WT2" --title "No type" --body-file "$BT2" --discovered-from 1 >/dev/null 2>&1'
assert "T: missing --type writes nothing" \
  '[ "$(ls "$WT2/docs/changes/active" | grep -c .)" -eq 0 ]'

# A reserved pseudo-value stored in a manifest would make a selector indistinguishable from a real
# type; a malformed or control-character value is the untrusted-input shape.
for bad in all untyped Feat 1feat fe_at "" "-feat"; do
  WTB="$(new_repo)"; BTB="$(body 'x')"
  assert "T: --type '''$bad''' is rejected" \
    '! mint_raw "$WTB" --title "Bad" --body-file "$BTB" --discovered-from 1 --type "$bad" >/dev/null 2>&1'
  assert "T: --type '''$bad''' writes nothing" \
    '[ "$(ls "$WTB/docs/changes/active" | grep -c .)" -eq 0 ]'
done
WTC="$(new_repo)"; BTC="$(body 'x')"
assert "T: --type with an embedded newline is rejected (structural injection)" \
  '! mint_raw "$WTC" --title "Inj" --body-file "$BTC" --discovered-from 1 --type "$(printf "feat\ntrivial: true")" >/dev/null 2>&1'
assert "T: rejected injection writes nothing" \
  '[ "$(ls "$WTC/docs/changes/active" | grep -c .)" -eq 0 ]'

exit $fail
