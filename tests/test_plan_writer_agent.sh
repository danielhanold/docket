#!/usr/bin/env bash
# tests/test_plan_writer_agent.sh — the internal docket-plan-writer agent source, its shipped
# per-harness defaults, and its generated wrappers (change 0324).
# run: bash tests/test_plan_writer_agent.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT="$REPO/agents/docket-plan-writer.md"
SIDECAR="$REPO/agents/harness-defaults.yml"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# ---- source shape -----------------------------------------------------------
assert "agent source exists" '[ -f "$AGENT" ]'
assert "agent is feature-worktree-scoped" 'grep -q "^worktree-scope: feature$" "$AGENT"'
assert "agent preloads no skill (internal leaf, like the consultant)" '! grep -q "^skills:" "$AGENT"'
assert "agent name is docket-plan-writer" 'grep -q "^name: docket-plan-writer$" "$AGENT"'
# The contract's load-bearing clauses (bind phrase to claim — prose-guard learning):
body="$(cat "$AGENT")"
assert "contract: success line is PLAN_PATH=" 'grep -q "PLAN_PATH=<repo-relative-path>" <<<"$body"'
assert "contract: commit carries the Docket-Plan-Path trailer" 'grep -q "Docket-Plan-Path: <repo-relative-path>" <<<"$body"'
assert "contract: no docket metadata mutation" 'grep -qi "no Docket metadata mutation" <<<"$body"'
assert "contract: never success-shaped output for an uncommitted plan" \
  'grep -qi "never.*success-shaped output for an uncommitted" <<<"$body"'
assert "contract: missing plan skill degrades inside the child, warns" \
  'grep -qi "missing-skill" <<<"$body" && grep -qi "warn" <<<"$body"'
assert "contract: stages only the plan path" 'grep -qi "stage only" <<<"$body"'
# The child conditions its learnings-index READ on enablement — the compensating guard for
# test_learnings_ledger.sh relaxing its parent >=2 assert to >=1 once Step 4's plan-time read
# moved into this child. Keyed on the read step's own gating clause ("when learnings are enabled
# — the learnings index, then the finding files"), not a line number: a regression that drops the
# gate and reads the index unconditionally reddens here.
assert "contract: the child gates its learnings-index read on enablement" \
  'grep -qE "when learnings are enabled.*the learnings index, then the finding files" <<<"$body"'

# ---- shipped defaults: exact four pairs (frozen against the sidecar) --------
row(){ awk -v h="$1" '$0 ~ "^  " h ":$" {inh=1; next} inh && /^  [a-z]/ {inh=0} inh && /plan-writer:/ {print}' "$SIDECAR"; }
c="$(row claude)";  assert "claude ships claude-opus-5/high"  'grep -q "model: claude-opus-5, effort: high" <<<"$c"'
x="$(row codex)";   assert "codex ships gpt-5.6-terra/high"   'grep -q "model: gpt-5.6-terra, effort: high" <<<"$x"'
u="$(row cursor)";  assert "cursor ships cursor-grok-4.5-xhigh/auto" 'grep -q "model: cursor-grok-4.5-xhigh, effort: auto" <<<"$u"'
o="$(row opencode)"; assert "opencode ships deepseek-v4-pro-0813/medium" \
  'grep -q "model: openrouter/deepseek/deepseek-v4-pro-0813, effort: medium" <<<"$o"'

# ---- generated wrappers (all four harnesses) --------------------------------
# Auto-discovery: sync-agents.sh globs agents/docket-*.md, so this source needs no generator
# change to be wrapped on every enabled harness. Each assert reads REAL emitted output from one
# sandboxed run — one existence + one shipped-model-content check per harness, on the actual
# emitted path each sibling sync test proves for its harness (tests/test_sync_agents_cursor.sh,
# tests/test_sync_agents_codex.sh, tests/test_sync_agents_opencode.sh).
SBX="$(mktemp -d "${TMPDIR:-/tmp}/planwriter.XXXXXX")"
git -C "$SBX" init --quiet
git -C "$SBX" config user.email t@t.test; git -C "$SBX" config user.name Test
printf 'agent_harnesses: [claude, cursor, codex, opencode]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 )

# claude: markdown wrapper carrying the full frontmatter (scope + injected pin), no skills preload
W="$SBX/.claude/agents/docket-plan-writer.md"
assert "claude wrapper generated" '[ -f "$W" ]'
assert "claude wrapper pins the shipped model" 'grep -q "claude-opus-5" "$W"'
assert "claude wrapper is feature-scoped" 'grep -q "^worktree-scope: feature$" "$W"'
assert "claude wrapper preloads no skill" '! grep -q "^skills:" "$W"'

# cursor: markdown wrapper at .cursor/agents/, model injected into frontmatter
CU="$SBX/.cursor/agents/docket-plan-writer.md"
assert "cursor wrapper generated" '[ -f "$CU" ]'
assert "cursor wrapper pins the shipped model" 'grep -q "cursor-grok-4.5-xhigh" "$CU"'

# codex: TOML wrapper at .codex/agents/, model = "..."
CX="$SBX/.codex/agents/docket-plan-writer.toml"
assert "codex wrapper generated" '[ -f "$CX" ]'
assert "codex wrapper pins the shipped model" 'grep -q "gpt-5.6-terra" "$CX"'

# opencode: markdown wrapper at .opencode/agents/, model injected into frontmatter
OC="$SBX/.opencode/agents/docket-plan-writer.md"
assert "opencode wrapper generated" '[ -f "$OC" ]'
assert "opencode wrapper pins the shipped model" 'grep -q "openrouter/deepseek/deepseek-v4-pro-0813" "$OC"'

rm -rf "$SBX"

exit "$fail"
