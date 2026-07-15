#!/usr/bin/env bash
# tests/test_sync_agents_codex.sh — Codex harness TOML generation + AGENTS.md dispatch (change 0077).
# run: bash tests/test_sync_agents_codex.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Minimal TOML top-level scalar reader: prints the value (unquoted) of a bare `key = "..."`.
# Good enough for name/description/model/model_reasoning_effort (single-line basic strings).
toml_get(){ sed -n -E 's/^'"$2"'[[:space:]]*=[[:space:]]*"(.*)"[[:space:]]*$/\1/p' "$1" | head -n1; }
toml_has_key(){ grep -qE "^$2[[:space:]]*=" "$1"; }

# Opt a sandbox repo into [claude, codex] and generate.
mk_codex_repo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}

# --- codex per-repo pass writes TOML wrappers, not .md ---
mk_codex_repo
assert "codex: writes .codex/agents/docket-status.toml" '[ -f "$SBX/.codex/agents/docket-status.toml" ]'
assert "codex: does NOT write a codex .md wrapper"       '[ ! -f "$SBX/.codex/agents/docket-status.md" ]'
assert "codex: full built-in set as TOML (9 files)"      '[ "$(find "$SBX/.codex/agents" -name "docket-*.toml" | wc -l | tr -d " ")" = "9" ]'

T="$SBX/.codex/agents/docket-status.toml"
assert "codex TOML: name = docket-status"          '[ "$(toml_get "$T" name)" = "docket-status" ]'
assert "codex TOML: description matches source"    '[ "$(toml_get "$T" description)" = "$(sed -n "/^description:/{s/^description:[[:space:]]*//;p;q;}" "$REPO/agents/docket-status.md")" ]'
assert "codex TOML: model = built-in claude id"    '[ "$(toml_get "$T" model)" = "claude-haiku-4-5-20251001" ]'
assert "codex TOML: model_reasoning_effort = medium" '[ "$(toml_get "$T" model_reasoning_effort)" = "medium" ]'
assert "codex TOML: has developer_instructions"    'grep -qE "^developer_instructions[[:space:]]*=" "$T"'
assert "codex TOML: dev_instructions carry body"   'grep -qi "refresh docket state" "$T"'
assert "codex TOML: dev_instructions name the skills to load" 'grep -qi "docket-convention" "$T"'
# claude side is untouched and still markdown
assert "claude side still .md, byte-identical to source" 'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'
rm -rf "$SBX"

# --- regression: emit_codex_toml preserves a --- thematic break inside the body ---
DIVDIR="$(mktemp -d)"
cat > "$DIVDIR/docket-divfixture.md" <<'FIX'
---
name: docket-divfixture
description: Fixture with a divider in its body.
model: claude-x
effort: medium
skills: [docket-divfixture, docket-convention]
---
Above the rule.

---

Below the rule.
FIX
DIVOUT="$( . "$REPO/sync-agents.sh"; set +e +u; emit_codex_toml "$DIVDIR/docket-divfixture.md" "" "" )"
assert "codex TOML: --- divider line inside body is preserved" 'printf "%s\n" "$DIVOUT" | grep -qxF -- "---"'
assert "codex TOML: body text above the divider preserved"    'printf "%s\n" "$DIVOUT" | grep -qF "Above the rule."'
assert "codex TOML: body text below the divider preserved"    'printf "%s\n" "$DIVOUT" | grep -qF "Below the rule."'
rm -rf "$DIVDIR"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
