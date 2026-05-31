#!/usr/bin/env bash
# tests/test_link_skills.sh — run: bash tests/test_link_skills.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected_skills="$(find "$REPO/skills" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Fake harness root: SOME dirs present, some absent on purpose.
mkdir -p "$tmp/.claude/skills" "$tmp/.agents/skills"   # present
# .cursor/.codex/.kiro/.windsurf intentionally absent

DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null

assert "links into present .claude/skills"  '[ -L "$tmp/.claude/skills/docket-status" ]'
assert "links into present .agents/skills"  '[ -L "$tmp/.agents/skills/docket-status" ]'
assert "symlink target is absolute repo path" '[ "$(readlink "$tmp/.claude/skills/docket-status")" = "$REPO/skills/docket-status" ]'
assert "does NOT create an absent harness dir" '[ ! -d "$tmp/.cursor/skills" ]'
assert "all skills linked" '[ "$(find "$tmp/.claude/skills" -maxdepth 1 -type l | wc -l | tr -d " ")" = "$expected_skills" ]'

# Idempotency: a second run creates nothing new.
out="$(DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh")"
assert "second run idempotent (Created: 0)" 'echo "$out" | grep -q "Created: 0"'

# A pre-existing entry at a link path is left untouched (not clobbered).
rm "$tmp/.agents/skills/docket-adr"; echo "do not touch" > "$tmp/.agents/skills/docket-adr"
DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null
assert "pre-existing file preserved" 'grep -q "do not touch" "$tmp/.agents/skills/docket-adr"'

# A pre-existing DANGLING symlink at a link path is left untouched (not clobbered).
rm -f "$tmp/.claude/skills/docket-status"
ln -s /nonexistent-docket-target "$tmp/.claude/skills/docket-status"
DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null
assert "dangling symlink left alone" '[ -L "$tmp/.claude/skills/docket-status" ] && [ ! -e "$tmp/.claude/skills/docket-status" ]'

exit $fail
