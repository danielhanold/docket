#!/usr/bin/env bash
# tests/test_sync_agents.sh — run: bash tests/test_sync_agents.sh
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: the script reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}; pin global to the sandbox
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Extract a single-line frontmatter scalar value from a markdown file.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }

# ---- Task 1: built-in wrapper source files ---------------------------------
AGENTS="$REPO/agents"
AUTONOMOUS="docket-implement-next docket-auto-groom docket-finalize-change docket-status docket-adr"

assert "agents/ source dir exists" '[ -d "$AGENTS" ]'
assert "exactly 5 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "5" ]'

for w in $AUTONOMOUS; do
  f="$AGENTS/$w.md"
  assert "$w: file exists" '[ -f "$f" ]'
  assert "$w: name matches file" '[ "$(fm "$f" name)" = "$w" ]'
  assert "$w: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$w: description matches the skill (single source)" \
    '[ "$(fm "$f" description)" = "$(fm "$REPO/skills/$w/SKILL.md" description)" ]'
  assert "$w: model is a known alias" '[[ "$(fm "$f" model)" =~ ^(opus|sonnet|haiku|fable)$ ]]'
  assert "$w: effort in allowed set" '[[ "$(fm "$f" effort)" =~ ^(low|medium|high|xhigh|max)$ ]]'
  assert "$w: skills: injects the skill itself" 'grep -Eq "^skills:.*\b'"$w"'\b" "$f"'
  assert "$w: skills: injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  assert "$w: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Built-in model/effort match the §4 default table.
assert "implement-next built-in = opus/xhigh" \
  '[ "$(fm "$AGENTS/docket-implement-next.md" model)/$(fm "$AGENTS/docket-implement-next.md" effort)" = "opus/xhigh" ]'
assert "auto-groom built-in = opus/xhigh" \
  '[ "$(fm "$AGENTS/docket-auto-groom.md" model)/$(fm "$AGENTS/docket-auto-groom.md" effort)" = "opus/xhigh" ]'
assert "finalize-change built-in = sonnet/medium" \
  '[ "$(fm "$AGENTS/docket-finalize-change.md" model)/$(fm "$AGENTS/docket-finalize-change.md" effort)" = "sonnet/medium" ]'
assert "status built-in = sonnet/medium" \
  '[ "$(fm "$AGENTS/docket-status.md" model)/$(fm "$AGENTS/docket-status.md" effort)" = "sonnet/medium" ]'
assert "adr built-in = sonnet/medium" \
  '[ "$(fm "$AGENTS/docket-adr.md" model)/$(fm "$AGENTS/docket-adr.md" effort)" = "sonnet/medium" ]'

# Advisory/interactive skills must NOT have a wrapper file.
assert "no wrapper for new-change (advisory)" '[ ! -f "$AGENTS/docket-new-change.md" ]'
assert "no wrapper for groom-next (advisory)" '[ ! -f "$AGENTS/docket-groom-next.md" ]'

exit $fail
