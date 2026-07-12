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

# --- change 0065: doc sentinels -----------------------------------------------------------------
# Positive anchors on the MEANINGFUL FRAMING of the invocation-path / model-pinning docs, not on
# incidental wording (LEARNINGS #36/#37). Each assert owns exactly ONE clause in ONE file, so it can
# be mutation-tested in isolation (LEARNINGS #21) and the prose stays freely rewritable.
README="$REPO/README.md"
AGENT_LAYER="$REPO/skills/docket-convention/references/agent-layer.md"

assert "README names both invocation paths into the pinned wrapper" \
  'grep -qF "| **Skill-invoke** |" "$README" && grep -qF "| **Agent-dispatch** |" "$README"'
assert "README contrasts them by observability (forked run opaque, dispatch drillable)" \
  'grep -qiF "completed (forked execution)" "$README" && grep -qi "drillable" "$README"'
assert "README names the fork transcript path as the escape hatch" \
  'grep -qF "subagents/agent-" "$README"'
assert "README carries the process-start registration caveat" \
  'grep -qiE "register(ed)? at .{0,4}process start" "$README" && grep -qi "restart" "$README"'
assert "README teaches model-per-task over model-per-session" \
  'grep -qiE "one session, one model" "$README" && grep -qi "cheap tier" "$README"'
assert "README's What you get list surfaces per-agent model pinning" \
  'grep -qF "**The right model for each step.**" "$README"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
