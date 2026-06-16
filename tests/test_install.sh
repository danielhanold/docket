#!/usr/bin/env bash
# tests/test_install.sh — run: bash tests/test_install.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# A present harness root. link-skills.sh leaf-checks <root>/.claude/skills (so create it);
# sync-agents.sh parent-checks <root>/.claude and creates agents/ itself.
mkdir -p "$tmp/.claude/skills"

# Run the umbrella installer against the sandbox harness, from a repo-less cwd (so sync-agents.sh's
# per-repo pass is a no-op — we are only exercising the user-level install both primitives perform).
out="$(cd "$tmp" && DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/install.sh" 2>&1)"; rc=$?
assert "install.sh exits 0" '[ "$rc" = "0" ]'
assert "install.sh ran link-skills.sh (skill symlinked)" '[ -L "$tmp/.claude/skills/docket-status" ]'
assert "install.sh ran sync-agents.sh (agent generated)" '[ -f "$tmp/.claude/agents/docket-status.md" ]'

# Idempotent: a second run still succeeds.
out2="$(cd "$tmp" && DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/install.sh" 2>&1)"; rc2=$?
assert "install.sh idempotent (second run exits 0)" '[ "$rc2" = "0" ]'

exit $fail
