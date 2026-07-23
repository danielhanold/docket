#!/usr/bin/env bash
# tests/test_backfill_change_types.sh — the one-time active-backlog categorization helper
# (change 0127). Hermetic: plain temp trees, no git, no network — the helper only rewrites files;
# committing the result is the caller's job.
# Run: bash tests/test_backfill_change_types.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/backfill-change-types.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# mkfix <dir> — active/0001,0002 untyped; 0003 already typed; 0004 has an EMPTY type: placeholder
# (the template's own shape); archive/0009 untyped and must never be touched.
# 0001's body deliberately contains a prose line starting `type:` so an unanchored edit is caught.
mkfix(){
  rm -rf "$1"; mkdir -p "$1/active" "$1/archive"
  printf -- '---\nid: 1\nslug: a\ntitle: A\nstatus: proposed\npriority: high\n---\n\n## Why\ntype: this is prose, not frontmatter\n' > "$1/active/0001-a.md"
  printf -- '---\nid: 2\nslug: b\ntitle: B\nstatus: proposed\npriority: low\n---\n\n## Why\nx\n' > "$1/active/0002-b.md"
  printf -- '---\nid: 3\nslug: c\ntitle: C\nstatus: proposed\npriority: low\ntype: feat\n---\n\n## Why\nx\n' > "$1/active/0003-c.md"
  printf -- '---\nid: 4\nslug: d\ntitle: D\nstatus: proposed\npriority: low\ntype:\n---\n\n## Why\nx\n' > "$1/active/0004-d.md"
  printf -- '---\nid: 9\nslug: z\ntitle: Z\nstatus: done\npriority: low\n---\n\n## Why\nx\n' > "$1/archive/2026-01-01-0009-z.md"
}
arc_hash(){ find "$1/archive" -type f -exec cat {} + | shasum | cut -d' ' -f1; }
fm_type(){ # fm_type FILE -> the type: value from the FIRST frontmatter block only
  awk '/^---[[:space:]]*$/{n++; next} n==1 && /^type:[[:space:]]*/{sub(/^type:[[:space:]]*/,""); print; exit}' "$1"
}

# --- happy path --------------------------------------------------------------
d="$tmp/ok"; mkfix "$d"; before_arc="$(arc_hash "$d")"
out="$(bash "$SCRIPT" --changes-dir "$d" --map "1=fix,2=docs,4=chore" 2>&1)"; rc=$?
assert "apply: a complete valid mapping exits 0" '[ "$rc" -eq 0 ]'
assert "apply: id 1 typed"                       '[ "$(fm_type "$d/active/0001-a.md")" = fix ]'
assert "apply: id 2 typed"                       '[ "$(fm_type "$d/active/0002-b.md")" = docs ]'
assert "apply: an EMPTY type: placeholder is filled in place" \
  '[ "$(fm_type "$d/active/0004-d.md")" = chore ]'
assert "apply: the empty placeholder is not duplicated" \
  '[ "$(grep -c "^type:" "$d/active/0004-d.md")" -eq 1 ]'
assert "apply: an already-typed change is left alone" \
  '[ "$(fm_type "$d/active/0003-c.md")" = feat ]'
assert "apply: archive is byte-identical"        '[ "$(arc_hash "$d")" = "'"$before_arc"'" ]'
assert "apply: reports how many files changed"   'grep -q "3" <<<"$out"'

# The prose `type:` line in 0001's BODY must survive untouched — the frontmatter-anchor rule.
assert "anchor: a body line starting 'type:' is never rewritten" \
  'grep -qx "type: this is prose, not frontmatter" "$d/active/0001-a.md"'
assert "anchor: the written type is inside the FIRST frontmatter block" \
  '[ "$(fm_type "$d/active/0001-a.md")" = fix ]'
assert "anchor: exactly one frontmatter type: line was added to 0001" \
  '[ "$(awk "/^---[[:space:]]*\$/{n++; next} n==1 && /^type:/{c++} END{print c+0}" "$d/active/0001-a.md")" -eq 1 ]'

# --- idempotent --------------------------------------------------------------
snap="$(cat "$d/active/0001-a.md")"
bash "$SCRIPT" --changes-dir "$d" --map "1=fix,2=docs,4=chore" >/dev/null 2>&1
assert "idempotent: rerunning the applied mapping changes nothing" \
  '[ "$(cat "$d/active/0001-a.md")" = "'"$snap"'" ]'
assert "idempotent: rerun still exits 0" \
  'bash "$SCRIPT" --changes-dir "$d" --map "1=fix,2=docs,4=chore" >/dev/null 2>&1'

# --- all-or-nothing refusals -------------------------------------------------
# Each case asserts BOTH a non-zero exit and that not one file moved: a helper that validated
# lazily would fail on entry N having already written entries 1..N-1.
refuses(){ # refuses <label> <map>
  local d2="$tmp/r${RANDOM}${RANDOM}" h1 h2 h4 ha
  mkfix "$d2"
  h1="$(cat "$d2/active/0001-a.md")"; h2="$(cat "$d2/active/0002-b.md")"
  h4="$(cat "$d2/active/0004-d.md")"; ha="$(arc_hash "$d2")"
  assert "refuse: $1 exits non-zero" \
    '! bash "$SCRIPT" --changes-dir "'"$d2"'" --map "'"$2"'" >/dev/null 2>&1'
  assert "refuse: $1 leaves every active file untouched" \
    '[ "$(cat "'"$d2"'/active/0001-a.md")" = "'"$h1"'" ] && [ "$(cat "'"$d2"'/active/0002-b.md")" = "'"$h2"'" ] && [ "$(cat "'"$d2"'/active/0004-d.md")" = "'"$h4"'" ]'
  assert "refuse: $1 leaves the archive byte-identical" \
    '[ "$(arc_hash "'"$d2"'")" = "'"$ha"'" ]'
}
refuses "unknown id"                "1=fix,2=docs,4=chore,77=docs"
refuses "duplicate assignment"      "1=fix,1=docs,2=docs,4=chore"
refuses "malformed type"            "1=Fix,2=docs,4=chore"
refuses "reserved type all"         "1=all,2=docs,4=chore"
refuses "reserved type untyped"     "1=untyped,2=docs,4=chore"
refuses "partial mapping"           "1=fix"
refuses "conflicting overwrite"     "1=fix,2=docs,4=chore,3=chore"
refuses "an archived id"            "1=fix,2=docs,4=chore,9=chore"
refuses "malformed entry, no ="     "1=fix,2docs,4=chore"
refuses "empty type"                "1=,2=docs,4=chore"
# --- control characters in --map ---------------------------------------------
# The mapping is ONE physical line by contract: `IFS=',' read -r -a` consumes a SINGLE line, so an
# embedded newline silently discards every assignment after it — the validation for those entries
# included. Rejecting the shape before the split is what makes the loss visible.
#
# The old test here passed for the wrong reason: `1=$(printf 'fix\ntrivial: true'),2=docs,4=chore`
# was truncated by `read` down to `1=fix`, so it died on `incomplete mapping` and never reached any
# control-character guard at all. It is now split into the three shapes below, each pinned to the
# diagnostic it actually produces.
refuses "a newline between --map entries"   "$(printf '1=fix,\n2=docs,4=chore')"
refuses "a newline inside a type token"     "$(printf '1=fix,2=docs,4=ch\nore')"
refuses "a tab inside a type token"         "$(printf '1=fix\ttrivial: true,2=docs,4=chore')"

# Each refusal above asserts only "non-zero exit + nothing written", which several INDEPENDENT
# mechanisms can satisfy — the conflicting-overwrite case, for instance, is also caught downstream
# by the post-write verification, so removing the explicit guard left every assert above green
# (mutation-tested). Pin each guard to its OWN diagnostic so the guard, not merely some backstop,
# is what the test holds in place.
diag(){ # diag <label> <map> <expected-substring>
  local d2="$tmp/dg${RANDOM}${RANDOM}" err
  mkfix "$d2"
  err="$(bash "$SCRIPT" --changes-dir "$d2" --map "$2" 2>&1 >/dev/null)"
  assert "diagnostic: $1 names its own cause" 'grep -qF -- "'"$3"'" <<<"'"$err"'"'
}
diag "conflicting overwrite" "1=fix,2=docs,4=chore,3=chore" "already has type"
diag "unknown id"            "1=fix,2=docs,4=chore,77=docs" "not an active change"
diag "archived id"           "1=fix,2=docs,4=chore,9=chore" "not an active change"
diag "partial mapping"       "1=fix"                        "incomplete mapping"
diag "duplicate assignment"  "1=fix,1=docs,2=docs,4=chore"  "duplicate assignment"
diag "reserved type"         "1=all,2=docs,4=chore"         "reserved value"
diag "malformed type"        "1=Fix,2=docs,4=chore"         "must match"
# The map-level control-character guard, pinned to its OWN message so the newline cases above can
# never again be satisfied by `incomplete mapping` (which is what the truncation itself produced).
diag "a newline between --map entries" "$(printf '1=fix,\n2=docs,4=chore')" \
  "--map contains a control character"
diag "a newline inside a type token"   "$(printf '1=fix,2=docs,4=ch\nore')" \
  "--map contains a control character"
# NOTE (verified against this script, 2026-07-22): a NON-newline control character cannot reach the
# per-type `type for id N contains control characters` guard either. Every type token is a substring
# of $MAP, so any control character in a type is by construction a control character in $MAP and is
# caught by the map-level guard first — a tab, vertical tab, CR, FF and BEL were all checked. That
# per-type guard is therefore unreachable defence-in-depth (and doubly redundant: the `[a-z][a-z0-9-]*`
# shape gate rejects the same tokens). It is pinned here to the diagnostic that DOES fire; deleting
# the per-type guard is not observable from outside the script, so no assert can hold it in place.
diag "a tab inside a type token"       "$(printf '1=fix\ttrivial: true,2=docs,4=chore')" \
  "--map contains a control character"

# The WORST case the map-level guard exists for: the break lands INSIDE the final type token. Before
# the guard this exited 0 having written `type: ch` — the completeness check still passed (id 4 WAS
# assigned, just to a truncated value), --dry-run reported it identically to a correct run, and the
# overwrite guard then refused to repair the wrong value. Pin both halves: the refusal, and the
# absence of the truncated value.
dnl="$tmp/newline-worst"; mkfix "$dnl"
bash "$SCRIPT" --changes-dir "$dnl" --map "$(printf '1=fix,2=docs,4=ch\nore')" >/dev/null 2>&1
nlrc=$?
assert "newline: a break inside the FINAL type token is refused, not silently truncated" \
  '[ "$nlrc" -ne 0 ]'
assert "newline: the truncated type 'ch' is never written" \
  '[ -z "$(fm_type "$dnl/active/0004-d.md")" ]'
assert "newline: the assignments before the break are not written either" \
  '[ -z "$(fm_type "$dnl/active/0001-a.md")" ] && [ -z "$(fm_type "$dnl/active/0002-b.md")" ]'

# --- zero-padded ids ---------------------------------------------------------
# `0004` and `4` name the same change. Filenames (`0004-d.md`) and BOARD.md rows both show the
# PADDED form, so that is what an operator or an agent composing the map copies, while `id:`
# frontmatter carries the bare integer. Keying the two sides differently made a live active change
# report as "id 0001 is not an active change (archived records are never reclassified)" — a
# diagnostic that names the one thing that was NOT wrong. Both sides canonicalize now.
dpad="$tmp/padded"; mkfix "$dpad"
dbare="$tmp/bare";  mkfix "$dbare"
bash "$SCRIPT" --changes-dir "$dbare" --map "1=fix,2=docs,4=chore" >/dev/null 2>&1
padout="$(bash "$SCRIPT" --changes-dir "$dpad" --map "0001=fix,0002=docs,0004=chore" 2>&1)"
padrc=$?
assert "padded ids: a zero-padded map exits 0"   '[ "$padrc" -eq 0 ]'
assert "padded ids: it is not misreported as inactive" \
  '! grep -q "not an active change" <<<"$padout"'
assert "padded ids: id 0001 is typed"            '[ "$(fm_type "$dpad/active/0001-a.md")" = fix ]'
assert "padded ids: id 0002 is typed"            '[ "$(fm_type "$dpad/active/0002-b.md")" = docs ]'
assert "padded ids: 0004's empty placeholder is filled" \
  '[ "$(fm_type "$dpad/active/0004-d.md")" = chore ]'
assert "padded ids: the padded map lands exactly what the bare map lands" \
  'diff -r "$dpad/active" "$dbare/active" >/dev/null'
# The other side of the same coin: a change file whose OWN frontmatter carries a zero-padded `id:`
# (a shape board-checks.sh tolerates) must still be addressable by the bare id. Canonicalizing only
# the --map side would just move the mismatch, not remove it.
dfmp="$tmp/fm-padded"; mkdir -p "$dfmp/active" "$dfmp/archive"
printf -- '---\nid: 0007\nslug: g\ntitle: G\nstatus: proposed\npriority: low\n---\n\n## Why\nx\n' \
  > "$dfmp/active/0007-g.md"
assert "padded ids: a padded frontmatter id: is addressable by its bare id" \
  'bash "$SCRIPT" --changes-dir "$dfmp" --map "7=feat" >/dev/null 2>&1'
assert "padded ids: ...and the type lands in that file" \
  '[ "$(fm_type "$dfmp/active/0007-g.md")" = feat ]'

# Canonicalizing must not blanket-suppress the guard: a padded id that is genuinely absent from
# active/ still gets the real diagnostic (reported in its canonical bare form).
refuses "a padded but absent id" "1=fix,2=docs,4=chore,0077=docs"
diag    "a padded but absent id" "1=fix,2=docs,4=chore,0077=docs" "77 is not an active change"

# --- install-phase rollback --------------------------------------------------
# Staging protects the REWRITE phase only. The install itself is a loop of `mv`, and a bare loop is
# NOT all-or-none: a failure at file k leaves 1..k-1 installed and k..N not — exactly the
# half-migrated backlog the contract, and the documented exit-code semantics ("install failure.
# Nothing was installed."), promise is impossible. So the install carries its own undo, and the
# property to pin is the strong one: after a MID-LOOP failure every active file is byte-identical
# to its pre-run state.
#
# The failure is forced from outside the script, on the real `mv`: the stage installs in glob order
# (0001, 0002, 0004), so making 0002's destination immutable fails the SECOND mv — after 0001 has
# already landed and before 0004 has. TMPDIR is redirected under the fixture so the script's own
# scratch dir (whose rollback copies inherit the immutable flag) is cleaned up with it.
drb="$tmp/rollback"; mkfix "$drb"; mkdir -p "$drb/tmpdir"
rb1="$(cat "$drb/active/0001-a.md")"; rb2="$(cat "$drb/active/0002-b.md")"
rb3="$(cat "$drb/active/0003-c.md")"; rb4="$(cat "$drb/active/0004-d.md")"
rbarc="$(arc_hash "$drb")"
if chflags uchg "$drb/active/0002-b.md" 2>/dev/null; then
  # Clear the flag before the tree is removed, however this script exits.
  trap 'chflags -R nouchg "$drb" 2>/dev/null; rm -rf "$tmp"' EXIT
  rberr="$(TMPDIR="$drb/tmpdir" bash "$SCRIPT" --changes-dir "$drb" \
             --map "1=fix,2=docs,4=chore" 2>&1 >/dev/null)"
  rbrc=$?
  chflags nouchg "$drb/active/0002-b.md" 2>/dev/null
  assert "rollback: a mid-install failure exits non-zero" '[ "$rbrc" -ne 0 ]'
  assert "rollback: the failure names the file and says it rolled back" \
    'grep -q "install failed for 0002-b.md" <<<"$rberr" && grep -q "rolled back" <<<"$rberr"'
  assert "rollback: the file installed BEFORE the failure is restored to its pre-run bytes" \
    '[ "$(cat "$drb/active/0001-a.md")" = "$rb1" ]'
  assert "rollback: the file the install failed ON is unchanged" \
    '[ "$(cat "$drb/active/0002-b.md")" = "$rb2" ]'
  assert "rollback: the files AFTER the failure are unchanged" \
    '[ "$(cat "$drb/active/0003-c.md")" = "$rb3" ] && [ "$(cat "$drb/active/0004-d.md")" = "$rb4" ]'
  assert "rollback: the archive is byte-identical" '[ "$(arc_hash "$drb")" = "$rbarc" ]'
  assert "rollback: no rollback-failure warning was emitted" \
    '! grep -q "rollback failed" <<<"$rberr"'
else
  # No way to make one destination unwritable here (root, or a filesystem without chflags).
  echo "skip - rollback: cannot make an install destination unwritable in this environment"
fi

# --- dry run -----------------------------------------------------------------
d3="$tmp/dry"; mkfix "$d3"; snap3="$(cat "$d3/active/0001-a.md")"
dry="$(bash "$SCRIPT" --changes-dir "$d3" --map "1=fix,2=docs,4=chore" --dry-run 2>&1)"
assert "dry-run: exits 0"              '[ "$?" -eq 0 ]'
assert "dry-run: writes nothing"       '[ "$(cat "$d3/active/0001-a.md")" = "'"$snap3"'" ]'
assert "dry-run: says what would change" 'grep -qi "dry-run" <<<"$dry"'

# --- argument handling -------------------------------------------------------
assert "args: missing --changes-dir is an error" '! bash "$SCRIPT" --map "1=fix" >/dev/null 2>&1'
assert "args: missing --map is an error"         '! bash "$SCRIPT" --changes-dir "$tmp/ok" >/dev/null 2>&1'
assert "args: a nonexistent changes dir is an error" \
  '! bash "$SCRIPT" --changes-dir "$tmp/nope" --map "1=fix" >/dev/null 2>&1'
assert "args: an unknown flag is an error" \
  '! bash "$SCRIPT" --changes-dir "$tmp/ok" --map "1=fix" --bogus >/dev/null 2>&1'
assert "args: --help exits 0" 'bash "$SCRIPT" --help >/dev/null 2>&1'

# --- an all-typed backlog is a clean no-op -----------------------------------
d4="$tmp/none"; mkdir -p "$d4/active" "$d4/archive"
printf -- '---\nid: 5\nslug: e\ntitle: E\nstatus: proposed\npriority: low\ntype: feat\n---\n\n## Why\nx\n' > "$d4/active/0005-e.md"
assert "no-op: an already-fully-typed backlog accepts an empty-effect mapping" \
  'bash "$SCRIPT" --changes-dir "$d4" --map "5=feat" >/dev/null 2>&1'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit $fail
