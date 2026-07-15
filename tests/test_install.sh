#!/usr/bin/env bash
# tests/test_install.sh — run: bash tests/test_install.sh
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: sync-agents.sh reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}; a set XDG would leak (and since 0050, MIGRATE) the dev's real global config
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# A present harness root. link-skills.sh parent-checks <root>/.claude and creates skills/ itself;
# sync-agents.sh parent-checks <root>/.claude and creates agents/ itself.
mkdir -p "$tmp/.claude/skills"

# Run the umbrella installer against the sandbox harness, from a repo-less cwd (so sync-agents.sh's
# per-repo pass is a no-op — we are only exercising the user-level install both primitives perform).
out="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh bash "$REPO/install.sh" 2>&1)"; rc=$?
assert "install.sh exits 0" '[ "$rc" = "0" ]'
assert "install.sh ran link-skills.sh (skill symlinked)" '[ -L "$tmp/.claude/skills/docket-status" ]'
assert "install.sh ran sync-agents.sh (agent generated)" '[ -f "$tmp/.claude/agents/docket-status.md" ]'
assert "install.sh injected DOCKET_SCRIPTS_DIR into the shell profile" \
  'grep -qF "export DOCKET_SCRIPTS_DIR=\"$REPO/scripts\"" "$tmp/.zshenv"'
assert "install.sh injected env.DOCKET_SCRIPTS_DIR into settings.json" \
  'jq -e --arg v "$REPO/scripts" ".env.DOCKET_SCRIPTS_DIR == \$v" "$tmp/.claude/settings.json" >/dev/null'

# Idempotent: a second run still succeeds.
out2="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh bash "$REPO/install.sh" 2>&1)"; rc2=$?
assert "install.sh idempotent (second run exits 0)" '[ "$rc2" = "0" ]'

exit $fail
