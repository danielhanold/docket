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

# ---- shipped defaults: exact four pairs (frozen against the sidecar) --------
row(){ awk -v h="$1" '$0 ~ "^  " h ":$" {inh=1; next} inh && /^  [a-z]/ {inh=0} inh && /plan-writer:/ {print}' "$SIDECAR"; }
c="$(row claude)";  assert "claude ships claude-opus-5/high"  'grep -q "model: claude-opus-5, effort: high" <<<"$c"'
x="$(row codex)";   assert "codex ships gpt-5.6-terra/high"   'grep -q "model: gpt-5.6-terra, effort: high" <<<"$x"'
u="$(row cursor)";  assert "cursor ships cursor-grok-4.5-xhigh/auto" 'grep -q "model: cursor-grok-4.5-xhigh, effort: auto" <<<"$u"'
o="$(row opencode)"; assert "opencode ships deepseek-v4-pro-0813/medium" \
  'grep -q "model: openrouter/deepseek/deepseek-v4-pro-0813, effort: medium" <<<"$o"'

exit "$fail"
