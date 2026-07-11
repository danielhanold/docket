#!/usr/bin/env bash
# tests/test_skill_fork_dispatch.sh — change 0061.
# Asserts the fork-dispatch invariant: the four headless-safe autonomous skills carry
# `context: fork` + a matching `agent: docket-<name>` in their SKILL.md frontmatter (so a
# direct Claude Code invocation forks into the pinned wrapper), and the three interactive/
# excluded skills carry neither. Run: bash tests/test_skill_fork_dispatch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILLS="$REPO/skills"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Print the YAML frontmatter block (between the first two --- fences) of a markdown file.
frontmatter(){ awk 'NR==1 && $0=="---"{f=1;next} f && $0=="---"{exit} f{print}' "$1"; }
# Extract a single-line frontmatter scalar value ("" if absent). Scoped to the frontmatter
# block so a body line like "agent:" can never satisfy the assertion.
fmval(){ frontmatter "$1" | sed -n "s/^$2:[[:space:]]*//p" | head -n1 | sed 's/[[:space:]]*$//'; }

# The four headless-safe autonomous skills that MUST fork into their pinned wrapper.
FORKED="docket-status docket-adr docket-implement-next docket-auto-groom"
# The three interactive/excluded skills that MUST NOT fork (no channel to the human, or a
# merge blocked by the auto-mode classifier — see change 0062).
EXCLUDED="docket-finalize-change docket-new-change docket-groom-next"

for s in $FORKED; do
  f="$SKILLS/$s/SKILL.md"
  assert "$s: SKILL.md exists" '[ -f "$f" ]'
  assert "$s: context is fork" '[ "$(fmval "$f" context)" = "fork" ]'
  assert "$s: agent routes to its own wrapper" '[ "$(fmval "$f" agent)" = "$s" ]'
done

for s in $EXCLUDED; do
  f="$SKILLS/$s/SKILL.md"
  assert "$s: SKILL.md exists" '[ -f "$f" ]'
  assert "$s: no context field (not forked)" '[ -z "$(fmval "$f" context)" ]'
  assert "$s: no agent field (not forked)" '[ -z "$(fmval "$f" agent)" ]'
done

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
