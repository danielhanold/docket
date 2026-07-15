#!/usr/bin/env bash
# tests/test_link_skills.sh — run: bash tests/test_link_skills.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected_skills="$(find "$REPO/skills" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Fake harness root: three states on purpose.
mkdir -p "$tmp/.claude/skills" "$tmp/.agents/skills"   # present WITH skills subdir
mkdir -p "$tmp/.cursor"                                 # harness present, skills subdir ABSENT
# .codex/.kiro/.windsurf fully absent (no parent dir at all)

DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null

assert "links into present .claude/skills"  '[ -L "$tmp/.claude/skills/docket-status" ]'
assert "links into present .agents/skills"  '[ -L "$tmp/.agents/skills/docket-status" ]'
assert "symlink target is absolute repo path" '[ "$(readlink "$tmp/.claude/skills/docket-status")" = "$REPO/skills/docket-status" ]'
assert "creates missing skills subdir under a present harness" '[ -d "$tmp/.cursor/skills" ]'
assert "links into the created .cursor/skills"                 '[ -L "$tmp/.cursor/skills/docket-status" ]'
assert "all skills linked into created .cursor/skills" '[ "$(find "$tmp/.cursor/skills" -maxdepth 1 -type l | wc -l | tr -d " ")" = "$expected_skills" ]'
assert "does NOT create a fully-absent harness dir" '[ ! -e "$tmp/.codex" ] && [ ! -e "$tmp/.codex/skills" ]'
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

# A present harness whose skills PATH is a non-directory must NOT abort the whole run
# (set -e + mkdir -p would kill every remaining harness/skill); it is skipped, later harnesses still link.
mkdir -p "$tmp/.kiro"; : > "$tmp/.kiro/skills"        # .kiro present, skills path is a regular FILE
rm -rf "$tmp/.windsurf"; mkdir -p "$tmp/.windsurf"    # .windsurf present, skills subdir missing (comes AFTER .kiro)
if DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null 2>&1; then rc=0; else rc=$?; fi
assert "non-dir skills path does not abort the run (exit 0)"      '[ "$rc" = 0 ]'
assert "run continued past the bad harness to a later one"        '[ -L "$tmp/.windsurf/skills/docket-status" ]'

exit $fail
