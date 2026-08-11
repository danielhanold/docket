#!/usr/bin/env bash
# tests/test_readme_skill_catalog.sh — run: bash tests/test_readme_skill_catalog.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

README="$REPO/README.md"

# The heading carries NO count (change 0168): a counted heading needs an edit per new skill and
# teaches the next author to bump the number instead of adding the row ([[guard-remedy-must-not-teach-the-evasion]]).
assert "catalog heading is count-free '## Skills'" 'grep -qx "## Skills" "$README"'
assert "no counted skills heading survives" \
  '! grep -qiE "^## The [a-z]+ skills" "$README"'
assert "no stale anchor link to a counted heading" \
  '! grep -qF "#the-eight-skills" "$README"'

# The rows named in the catalog section.
section="$(awk '/^## Skills[[:space:]]*$/{f=1;next} f&&/^## /{exit} f{print}' "$README")"
listed="$(printf '%s\n' "$section" | sed -nE 's/^\|[[:space:]]*`?(docket-[a-z-]+)`?[[:space:]]*\|.*/\1/p' | sort -u)"
live="$(cd "$REPO/skills" && for d in */SKILL.md; do echo "${d%/SKILL.md}"; done | sort -u)"

# forward: every live skill package is documented
while IFS= read -r s; do
  [ -n "$s" ] || continue
  assert "skills/$s is listed in the README catalog" 'grep -qx "'"$s"'" <<<"$listed"'
done <<<"$live"
# reverse: every listed row names a real skill package (anchored on the dirs, not an allowlist)
while IFS= read -r s; do
  [ -n "$s" ] || continue
  assert "catalog row $s names a real skills/ package" '[ -f "$REPO/skills/'"$s"'/SKILL.md" ]'
done <<<"$listed"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
