#!/usr/bin/env bash
# link-skills.sh — symlink docket's skills into each PRESENT agent-harness GLOBAL skill dir.
#
# Absolute symlinks point back to this repo's skills/<name>, so the source of truth stays
# in this clone (default ~/dev/docket): edit once, picked up everywhere, no copying.
# Idempotent: only creates MISSING links, and only into harnesses that ALREADY EXIST —
# the parent harness dir must be present (we never create a harness you don't use); a
# present harness whose skills/ subdir is absent gets that subdir created. Verify each
# harness's exact skills dir if this list drifts.
#
# Usage: bash link-skills.sh
# Test seam: set DOCKET_HARNESS_ROOT to override $HOME (used by tests/test_link_skills.sh).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="$SCRIPT_DIR/skills"

# Tests override the harness root; real runs use $HOME.
HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"

HARNESS_SKILL_DIRS=(
  "$HARNESS_ROOT/.claude/skills"
  "$HARNESS_ROOT/.codex/skills"
  "$HARNESS_ROOT/.cursor/skills"
  "$HARNESS_ROOT/.agents/skills"
  "$HARNESS_ROOT/.kiro/skills"
  "$HARNESS_ROOT/.windsurf/skills"
)

created=0
skipped=0

for skill_path in "$SKILLS_DIR"/*/; do
  [ -d "$skill_path" ] || continue
  name="$(basename "$skill_path")"
  target="$SKILLS_DIR/$name"            # absolute
  for dir in "${HARNESS_SKILL_DIRS[@]}"; do
    [ -d "$(dirname "$dir")" ] || continue   # only into harnesses the user actually uses (parent present)
    [ -d "$dir" ] || mkdir -p "$dir" || continue   # present harness, skills/ subdir missing → create it; skip if the path can't be made a dir
    link="$dir/$name"
    if [ -e "$link" ] || [ -L "$link" ]; then
      skipped=$((skipped + 1))
      continue
    fi
    ln -s "$target" "$link"
    echo "linked $link -> $target"
    created=$((created + 1))
  done
done

echo ""
echo "Created: $created   Skipped (already present): $skipped"
