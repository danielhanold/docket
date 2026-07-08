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
assert "exactly 8 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'

for w in $AUTONOMOUS; do
  f="$AGENTS/$w.md"
  assert "$w: file exists" '[ -f "$f" ]'
  assert "$w: name matches file" '[ "$(fm "$f" name)" = "$w" ]'
  assert "$w: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$w: description matches the skill (single source)" \
    '[ "$(fm "$f" description)" = "$(fm "$REPO/skills/$w/SKILL.md" description)" ]'
  assert "$w: model is a known alias or full id" '[[ "$(fm "$f" model)" =~ ^(opus|sonnet|haiku|fable|claude-[a-z0-9]+(-[a-z0-9]+)*)$ ]]'
  assert "$w: effort in allowed set" '[[ "$(fm "$f" effort)" =~ ^(low|medium|high|xhigh|max)$ ]]'
  assert "$w: skills: injects the skill itself" 'grep -Eq "^skills:.*\b'"$w"'\b" "$f"'
  assert "$w: skills: injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  assert "$w: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Built-in model/effort match the §4 default table.
assert "implement-next built-in = claude-opus-4-8/xhigh" \
  '[ "$(fm "$AGENTS/docket-implement-next.md" model)/$(fm "$AGENTS/docket-implement-next.md" effort)" = "claude-opus-4-8/xhigh" ]'
assert "auto-groom built-in = claude-opus-4-8/xhigh" \
  '[ "$(fm "$AGENTS/docket-auto-groom.md" model)/$(fm "$AGENTS/docket-auto-groom.md" effort)" = "claude-opus-4-8/xhigh" ]'
assert "finalize-change built-in = claude-sonnet-5/medium" \
  '[ "$(fm "$AGENTS/docket-finalize-change.md" model)/$(fm "$AGENTS/docket-finalize-change.md" effort)" = "claude-sonnet-5/medium" ]'
assert "status built-in = claude-haiku-4-5-20251001/medium" \
  '[ "$(fm "$AGENTS/docket-status.md" model)/$(fm "$AGENTS/docket-status.md" effort)" = "claude-haiku-4-5-20251001/medium" ]'
assert "adr built-in = claude-sonnet-5/medium" \
  '[ "$(fm "$AGENTS/docket-adr.md" model)/$(fm "$AGENTS/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'

# Advisory/interactive skills must NOT have a wrapper file.
assert "no wrapper for new-change (advisory)" '[ ! -f "$AGENTS/docket-new-change.md" ]'
assert "no wrapper for groom-next (advisory)" '[ ! -f "$AGENTS/docket-groom-next.md" ]'

# ---- Task 2: sync-agents.sh generator --------------------------------------
SYNC="$REPO/sync-agents.sh"
assert "sync-agents.sh exists and is executable-by-bash" '[ -f "$SYNC" ]'

# Helper: a fresh fake harness root + repo for an isolated generator run.
make_sandbox(){ SBX="$(mktemp -d)"; mkdir -p "$SBX/.claude" "$SBX/.agents"; }   # .cursor/.codex/.kiro/.windsurf absent on purpose

# -- user-level install: built-in wrappers, verbatim, into present harnesses --
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "writes into present .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "writes into present .agents/agents" '[ -f "$SBX/.agents/agents/docket-status.md" ]'
assert "all 8 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
assert "does NOT create an absent harness (.cursor)" '[ ! -d "$SBX/.cursor/agents" ]'
assert "no override => byte-identical to built-in source" 'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'

# -- idempotency: second run is byte-identical ----
before="$(cat "$SBX/.claude/agents/docket-implement-next.md")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
after="$(cat "$SBX/.claude/agents/docket-implement-next.md")"
assert "second run idempotent (byte-identical)" '[ "$before" = "$after" ]'
rm -rf "$SBX"

# -- global layer (harness-first): ~/.config/docket/agents.yaml default: block overrides model/effort --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku, effort: low }\n  implement-next: { effort: auto }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global default sets model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
assert "global default sets effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
assert "effort: auto drops the effort line" '! grep -q "^effort:" "$SBX/.claude/agents/docket-implement-next.md"'
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
rm -rf "$SBX"

# -- global: a per-harness block overrides default for THAT harness only (user-level) --
make_sandbox                                        # .claude and .cursor both present so both get user-level files
mkdir -p "$SBX/.cursor" "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku }\ncursor:\n  status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global cursor block wins for cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "global claude falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# -- per-repo layer (harness-first): .docket.yml agents.default: => committed project-level files --
make_sandbox                                       # SBX = the repo
HROOT="$(mktemp -d)"; mkdir -p "$HROOT/.claude"    # separate user-level harness root
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n    new-change: { model: opus }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT" bash "$SYNC" >/dev/null )
assert "per-repo default writes project-level file" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "per-repo default applies model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "per-repo default applies effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
assert "0048: unlisted skill NOW generated at built-in default (implement-next)" '[ -f "$SBX/.claude/agents/docket-implement-next.md" ]'
assert "0048: unlisted implement-next carries built-in model (claude-opus-4-8)" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
assert "advisory skill in agents: produces NO file (new-change)" '[ ! -f "$SBX/.claude/agents/docket-new-change.md" ]'
rm -rf "$SBX" "$HROOT"

# ============================================================================
# Change 0048 — always-full-set per-repo generation (Piece 1)
# ============================================================================

# Per-repo now generates the FULL built-in set for a listed harness even when the
# agents: block lists only a subset; unlisted agents carry the built-in default model.
make_sandbox                                       # SBX = the repo
HROOT48A="$(mktemp -d)"; mkdir -p "$HROOT48A/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48A" bash "$SYNC" >/dev/null )
assert "0048: full set — all 8 built-ins land in project-level .claude/agents" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
assert "0048: listed agent carries its override (status=sonnet)" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0048: UNLISTED agent generated at built-in default (implement-next=claude-opus-4-8/xhigh)" \
  '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)/$(fm "$SBX/.claude/agents/docket-implement-next.md" effort)" = "claude-opus-4-8/xhigh" ]'
rm -rf "$SBX" "$HROOT48A"

# (a)+(b) harness override wins; field-level merge — model from cursor, effort inherited from default.
make_sandbox
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" >/dev/null )
assert "0046 (a): cursor model from cursor block" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0046 (b): cursor effort inherited from default" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" effort)" = "high" ]'
assert "0046 (a): claude model falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0046 (a): claude effort from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# (c) arbitrary non-Claude id passes through verbatim; the two harness files now DIFFER (was byte-identical pre-0046).
assert "0046 (c): non-Claude id verbatim in .cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0046: harness files differ when overridden" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTM"

# (d) default-only (no harness block) reproduces today's .claude/agents output byte-for-byte across harnesses.
make_sandbox
HROOTD0="$(mktemp -d)"; mkdir -p "$HROOTD0/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD0" bash "$SYNC" >/dev/null )
assert "0046 (d): default-only => both harness files byte-identical" 'diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
assert "0046 (d): default-only applies model to claude" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTD0"

# 0046: tab-indented .docket.yml agents: block resolves (ind() must count tabs as indentation, not drop the block)
make_sandbox
HROOTT="$(mktemp -d)"; mkdir -p "$HROOTT/.claude"
printf 'agents:\n\tdefault:\n\t\tstatus: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTT" bash "$SYNC" >/dev/null )
assert "0046: tab-indented agents: block is not silently dropped" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0046: tab-indented default: resolves model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTT"

# ---- Task 1b: the docket-auto-groom-critic wrapper (wraps NO skill) ---------
CRITIC="$AGENTS/docket-auto-groom-critic.md"
assert "critic wrapper exists" '[ -f "$CRITIC" ]'
assert "critic: name matches file" '[ "$(fm "$CRITIC" name)" = "docket-auto-groom-critic" ]'
assert "critic: has a description" '[ -n "$(fm "$CRITIC" description)" ]'
assert "critic: model is claude-opus-4-8" '[ "$(fm "$CRITIC" model)" = "claude-opus-4-8" ]'
assert "critic: effort is xhigh" '[ "$(fm "$CRITIC" effort)" = "xhigh" ]'
assert "critic: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$CRITIC"'
# Isolation: the skills: line must NOT pull in the designer skill (would re-inject its bias).
# Scope the check to the skills: line — the name: line legitimately contains "docket-auto-groom".
crit_skills_line="$(grep -E "^skills:" "$CRITIC" || true)"
assert "critic: skills EXCLUDES the docket-auto-groom designer skill" '! grep -q "docket-auto-groom" <<<"$crit_skills_line"'
assert "critic: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$CRITIC"'

# Per-repo override of the critic key (auto-groom-critic) resolves to this wrapper source,
# proving the precedence path + --check drift gate cover the critic.
make_sandbox                                        # SBX = the repo
HROOT2="$(mktemp -d)"; mkdir -p "$HROOT2/.claude"   # separate user-level harness root
printf 'agents:\n  default:\n    auto-groom-critic: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" >/dev/null )
assert "per-repo critic override writes project-level file" '[ -f "$SBX/.claude/agents/docket-auto-groom-critic.md" ]'
assert "per-repo critic override applies model" '[ "$(fm "$SBX/.claude/agents/docket-auto-groom-critic.md" model)" = "sonnet" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes for in-sync critic (rc=0)" '[ "$chk_rc" = "0" ]'
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-auto-groom-critic.md"; rm -f "$SBX/.claude/agents/docket-auto-groom-critic.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check flags critic drift (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOT2"

# ---- Task 1c: the two finalize-gate wrappers (wrap NO skill) ----------------
# docket-rebase-resolver (①) and docket-integration-repair (②): like the critic,
# they inject ONLY docket-convention, pin opus/xhigh, and carry abort-and-report.
for nw in docket-rebase-resolver docket-integration-repair; do
  f="$AGENTS/$nw.md"
  assert "$nw: wrapper exists" '[ -f "$f" ]'
  assert "$nw: name matches file" '[ "$(fm "$f" name)" = "$nw" ]'
  assert "$nw: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$nw: model is claude-opus-4-8" '[ "$(fm "$f" model)" = "claude-opus-4-8" ]'
  assert "$nw: effort is xhigh" '[ "$(fm "$f" effort)" = "xhigh" ]'
  assert "$nw: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  # Isolation: the skills: line wraps NO docket skill (only the convention).
  nw_skills_line="$(grep -E "^skills:" "$f" || true)"
  assert "$nw: skills EXCLUDES any wrapped docket skill" \
    '! grep -Eq "docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change)" <<<"$nw_skills_line"'
  assert "$nw: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Per-repo override of a new key (rebase-resolver) resolves to its wrapper source,
# proving the precedence path + --check drift gate cover the new wrappers.
make_sandbox                                        # SBX = the repo
HROOT3="$(mktemp -d)"; mkdir -p "$HROOT3/.claude"   # separate user-level harness root
printf 'agents:\n  default:\n    rebase-resolver: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" >/dev/null )
assert "per-repo rebase-resolver override writes project-level file" '[ -f "$SBX/.claude/agents/docket-rebase-resolver.md" ]'
assert "per-repo rebase-resolver override applies model" '[ "$(fm "$SBX/.claude/agents/docket-rebase-resolver.md" model)" = "sonnet" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes for in-sync rebase-resolver (rc=0)" '[ "$chk_rc" = "0" ]'
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-rebase-resolver.md"; rm -f "$SBX/.claude/agents/docket-rebase-resolver.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check flags rebase-resolver drift (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOT3"

# ---- Task 3: --check drift gate --------------------------------------------
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )   # generate committed project file
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when committed agents match config (rc=0)" '[ "$chk_rc" = "0" ]'

# Out-of-band edit to a committed project-level file -> drift.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check fails on drift (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "--check reports a diff" 'printf "%s" "$chk_out" | grep -q "drift"'

# Committed file entirely absent (--check before sync-agents.sh ever ran) -> drift.
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
# intentionally do NOT generate; $SBX/.claude/agents/docket-status.md does not exist
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check fails when committed file is missing (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "--check reports missing-file drift" 'printf "%s" "$chk_out" | grep -q "drift"'

# 0048: an empty agents: block still expects the FULL committed set. Generate, then --check passes.
rm -f "$SBX/.docket.yml"; : > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes with empty agents: block once full set is committed (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"

# 0048: a repo with NO .docket.yml at all has nothing to check -> passes.
make_sandbox
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes when no .docket.yml (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"

# ---- Task 5: docket-convention documents the agent layer -------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention documents the agents: config block" 'grep -q "agents:" "$CONV"'
assert "convention names the generator sync-agents.sh" 'grep -q "sync-agents.sh" "$CONV"'
assert "convention states the precedence" 'grep -qi "per-repo > global > built-in" "$CONV"'
assert "convention states auto => omit effort" 'grep -qi "auto" "$CONV" && grep -qi "omit" "$CONV"'
assert "convention states abort-and-report for autonomous subagents" 'grep -qi "abort-and-report" "$CONV"'
assert "convention points at composition (0017)" 'grep -q "0017" "$CONV"'
# Non-vacuous guard: the agent section must be a distinct heading, not an incidental word.
assert "convention has an agent-layer section heading" 'grep -qiE "^#+ .*(agent layer|model/effort|subagent)" "$CONV"'

# 0046: convention documents the harness-first agents: shape (default: + harness keys, field-level fallback).
assert "0046 doc: convention names the reserved default: key" 'grep -qE "default:" "$CONV" && grep -Pzoq "agents:[\s\S]{0,400}default:" "$CONV"'
assert "0046 doc: convention shows a per-harness key example (cursor)" 'grep -Pzoq "agents:[\s\S]{0,600}cursor:" "$CONV"'
assert "0046 doc: convention states field-level fallback H -> default -> built-in" 'grep -qiE "harness.*default.*built-in|<harness>.*default.*built-in" "$CONV"'
assert "0046 doc: convention notes non-Claude fallback warning" 'grep -qi "default/built-in" "$CONV"'

# ---- Task 6: advisory recommendation in the interactive skills -------------
NEWC="$REPO/skills/docket-new-change/SKILL.md"
GROOM="$REPO/skills/docket-groom-next/SKILL.md"
assert "new-change carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$NEWC"'
assert "new-change recommends sonnet" 'grep -qi "sonnet" "$NEWC"'
assert "groom-next carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$GROOM"'
assert "groom-next recommends sonnet/high" 'grep -qiE "sonnet[^A-Za-z]+high|high[^A-Za-z]+sonnet" "$GROOM"'
# Non-vacuous: it must be advisory, not a hard requirement (we cannot force the session model).
assert "new-change frames it as advisory" 'grep -qi "advisory" "$NEWC"'
# Explicit pin (change 0042): the advisory must name the full model ID, not the bare alias.
assert "new-change advisory pins claude-sonnet-5" 'grep -q "claude-sonnet-5" "$NEWC"'
assert "groom-next advisory pins claude-sonnet-5" 'grep -q "claude-sonnet-5" "$GROOM"'

# ============================================================================
# Change 0045 — multi-harness project-level generation (agent_harnesses)
# ============================================================================

# (a) DEFAULT (no agent_harnesses key) => [claude]: project-level writes
#     .claude/agents ONLY (byte-identical to pre-0045 behavior). Separate HROOT
#     so <repo>/.claude/agents is purely project-level output.
make_sandbox                                          # SBX = the repo
HROOTA="$(mktemp -d)"; mkdir -p "$HROOTA/.claude"     # separate user-level root
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTA" bash "$SYNC" >/dev/null )
assert "0045 default: writes project-level .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 default: does NOT write .cursor/agents" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 default: per-repo model applied" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTA"

# (b) agent_harnesses: [claude, cursor] => BOTH dirs generated; cursor gets its own model
#     override so the files DIFFER (0046: no longer byte-identical when overridden).
make_sandbox
HROOTB="$(mktemp -d)"; mkdir -p "$HROOTB/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTB" bash "$SYNC" >/dev/null )
assert "0045 fanout: .claude/agents generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 fanout: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0046 fanout: claude carries default model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0046 fanout: cursor carries its override model" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0046 fanout: harness files differ when cursor overrides" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTB"

# (b') agent_harnesses: [cursor] ONLY => cursor generated, claude NOT (no forced-claude).
make_sandbox
HROOTC="$(mktemp -d)"; mkdir -p "$HROOTC/.claude"
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTC" bash "$SYNC" >/dev/null )
assert "0045 cursor-only: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 cursor-only: .claude/agents NOT generated" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTC"

# (d) unknown harness token => warned + dropped, NOT fatal; known harness still generated.
make_sandbox
HROOTD="$(mktemp -d)"; mkdir -p "$HROOTD/.claude"
printf 'agent_harnesses: [claude, bogus]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0045 unknown-token: generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0045 unknown-token: warns about the token" 'printf "%s" "$gen_err" | grep -qi "unknown agent_harnesses token"'
assert "0045 unknown-token: names the bad token" 'printf "%s" "$gen_err" | grep -q "bogus"'
assert "0045 unknown-token: known harness still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 unknown-token: bad-token dir NOT created" '[ ! -e "$SBX/.bogus/agents" ]'
rm -rf "$SBX" "$HROOTD"

# (e) explicit empty list agent_harnesses: [] => resolves to no targets: no project
#     files generated (mirrors board_surfaces: []). Locks the empty-set code path.
make_sandbox
HROOTE0="$(mktemp -d)"; mkdir -p "$HROOTE0/.claude"
printf 'agent_harnesses: []\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTE0" bash "$SYNC" >/dev/null )
assert "0045 empty-list: no .claude project file" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 empty-list: no .cursor project file" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTE0"

# --check must span every listed harness: drift in a .cursor/agents file fails CI.
make_sandbox
HROOTF="$(mktemp -d)"; mkdir -p "$HROOTF/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" >/dev/null )   # generate both harness files
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: passes when both harness files in sync (rc=0)" '[ "$chk_rc" = "0" ]'
# Drift the CURSOR file only.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.cursor/agents/docket-status.md"; rm -f "$SBX/.cursor/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: flags .cursor/agents drift (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0045 check: drift report names the cursor harness" 'printf "%s" "$chk_out" | grep -q "drift" && printf "%s" "$chk_out" | grep -q "cursor"'
# A listed-harness file never generated -> missing-file drift.
rm -f "$SBX/.cursor/agents/docket-status.md"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: flags missing cursor file (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOTF"

# Convention documents agent_harnesses + the direct-model-ID (harness-neutral) contract.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "0045 doc: convention names agent_harnesses" 'grep -q "agent_harnesses" "$CONV"'
assert "0045 doc: convention states default [claude]" 'grep -qE "agent_harnesses.*\[claude\]|default.*\[claude\]" "$CONV"'
assert "0045 doc: convention states harness-neutral direct model IDs" 'grep -qiE "harness-neutral|direct model id" "$CONV"'
assert "0045 doc: convention notes passthrough enables non-Claude harnesses" 'grep -qi "passthrough" "$CONV"'
assert "0045 doc: convention points at ADR-0015 near agent_harnesses" 'grep -Pzoq "agent_harnesses[\s\S]{0,500}ADR-0015|ADR-0015[\s\S]{0,500}agent_harnesses" "$CONV"'

# (f) a glob-metachar token must NOT expand against the cwd (set -f guard). A decoy
#     file present in the repo must never leak into the warnings.
make_sandbox
HROOTG="$(mktemp -d)"; mkdir -p "$HROOTG/.claude"
: > "$SBX/DECOYFILE"                                  # a filename the glob would match
printf 'agent_harnesses: [claude, *]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTG" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0045 glob-token: generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0045 glob-token: cwd decoy file did NOT leak into warnings" '! printf "%s" "$gen_err" | grep -q "DECOYFILE"'
assert "0045 glob-token: known harness still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTG"

# (g) agent_harnesses is a top-level (column-0) key: an indented decoy under another
#     block must NOT be read; the real top-level key wins.
make_sandbox
HROOTH="$(mktemp -d)"; mkdir -p "$HROOTH/.claude"
printf 'decoy:\n  agent_harnesses: [cursor]\nagent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTH" bash "$SYNC" >/dev/null )
assert "0045 anchor: top-level agent_harnesses honored (.claude generated)" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 anchor: indented decoy ignored (.cursor NOT generated)" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTH"

# ---- README discoverability of the agent model/effort refresh workflow (change 0047) ----
# The facts already exist buried in the Install prose, so a whole-README grep would pass
# vacuously. Extract the NEW dedicated section (heading -> next `## `) and assert within it,
# so each sentinel is RED before the section exists and non-vacuous after.
READMEF="$REPO/README.md"
sec="$(awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"

assert "0047: README has a discoverable agent model/effort section" '[ -n "$sec" ]'
assert "0047 §agent-cfg: names the global layer ~/.config/docket/agents.yaml" \
  'grep -qF "~/.config/docket/agents.yaml" <<<"$sec"'
assert "0047 §agent-cfg: names the per-repo .docket.yml agents: layer" \
  'grep -qF "\`agents:\` block in a repo" <<<"$sec"'
assert "0047 §agent-cfg: gives the refresh command (bash sync-agents.sh)" \
  'grep -qE "bash sync-agents\.sh" <<<"$sec"'
assert "0047 §agent-cfg: names the user-level target (every present harness)" \
  'grep -qiE "present.*harness" <<<"$sec"'
assert "0047 §agent-cfg: names the project-level target (agent_harnesses)" \
  'grep -qF "agent_harnesses" <<<"$sec"'
assert "0047 §agent-cfg: documents the --check drift gate" \
  'grep -qF "sync-agents.sh --check" <<<"$sec"'
assert "0047 §agent-cfg: references docket-convention Agent layer for the shape (not restated)" \
  'grep -qF "docket-convention" <<<"$sec" && grep -qi "agent layer" <<<"$sec"'
assert "0047 §agent-cfg: documents effort: auto drops the pinned effort line" \
  'grep -qF "effort: auto" <<<"$sec" && grep -qF "drops the effort line" <<<"$sec"'
# Non-restatement guard: the section must NOT hardcode a per-skill model/effort literal
# (those are config-overridable; built-in defaults live only in agents/docket-*.md). LEARNINGS #17.
assert "0047 §agent-cfg: does NOT hardcode a model/effort literal (references the source instead)" \
  '! grep -qiE "\b(opus|sonnet|haiku|fable)\b.*\b(xhigh|high|medium|low)\b|model:[[:space:]]*(opus|sonnet|haiku|claude-)" <<<"$sec"'

# ============================================================================
# Change 0046 — per-harness values: diagnostics
# ============================================================================

# (h) Non-Claude fallback warning: a cursor file whose model fell through to default/built-in warns;
#     suppressed for claude, and suppressed when cursor supplies its own model.
make_sandbox
HROOTW="$(mktemp -d)"; mkdir -p "$HROOTW/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h): generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (h): warns cursor model came from default/built-in" 'printf "%s" "$gen_err" | grep -qi "cursor" && printf "%s" "$gen_err" | grep -qi "default/built-in"'
assert "0046 (h): does NOT warn for the claude harness" '! printf "%s" "$gen_err" | grep -qiE "claude/docket-status|WARN claude"'
rm -rf "$SBX" "$HROOTW"

# (h') warning suppressed when the cursor block supplies the model.
make_sandbox
HROOTW2="$(mktemp -d)"; mkdir -p "$HROOTW2/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW2" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h'): no fallback warning when cursor supplies model" '! printf "%s" "$gen_err" | grep -qi "status.*default/built-in"'
rm -rf "$SBX" "$HROOTW2"

# (f) Legacy bare-agent-key block (pre-0046 flat shape) => warned + ignored; --check flags it as drift.
make_sandbox
HROOTL="$(mktemp -d)"; mkdir -p "$HROOTL/.claude"
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"   # bare agent key, no default:/harness
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (f): legacy shape not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (f): warns about the legacy bare agent key" 'printf "%s" "$gen_err" | grep -qi "legacy" && printf "%s" "$gen_err" | grep -q "status"'
assert "0046 (f): legacy status NOT applied (no project file / built-in only)" '[ ! -f "$SBX/.claude/agents/docket-status.md" ] || [ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0046 (g'): --check flags the legacy shape (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0046 (g'): --check names the legacy shape" 'printf "%s" "$chk_out" | grep -qi "legacy"'
rm -rf "$SBX" "$HROOTL"

# (e) Dead-config harness (a block in agents: not present in agent_harnesses) => warned + dropped.
make_sandbox
HROOTX="$(mktemp -d)"; mkdir -p "$HROOTX/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTX" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (e): dead-config not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (e): warns cursor block is not in agent_harnesses" 'printf "%s" "$gen_err" | grep -qi "cursor" && printf "%s" "$gen_err" | grep -qi "agent_harnesses"'
assert "0046 (e): cursor file NOT generated (dropped)" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0046 (e): claude still generated from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTX"

exit $fail
