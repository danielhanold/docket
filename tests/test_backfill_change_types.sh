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
refuses "control character in type" "1=$(printf 'fix\ntrivial: true'),2=docs,4=chore"

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
