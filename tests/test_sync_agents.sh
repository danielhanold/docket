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

# -- global layer: ~/.config/docket/agents.yaml overrides model/effort (user-level) --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'status: { model: haiku, effort: low }\nimplement-next: { effort: auto }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global override sets model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
assert "global override sets effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
assert "effort: auto drops the effort line" '! grep -q "^effort:" "$SBX/.claude/agents/docket-implement-next.md"'
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
rm -rf "$SBX"

# -- global keys are top-level only: an indented decoy must not shadow the real top-level key --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'decoy:\n  status: { model: haiku }\nstatus: { model: fable }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global match anchors at top level (indented decoy ignored)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "fable" ]'
rm -rf "$SBX"

# -- per-repo layer: .docket.yml agents: => committed project-level files --
# Decouple the harness root from the repo so <repo>/.claude/agents holds ONLY project-level output
# (else the user-level pass writes verbatim copies there and masks a too-eager per-repo pass).
make_sandbox                                       # SBX = the repo
HROOT="$(mktemp -d)"; mkdir -p "$HROOT/.claude"    # separate user-level harness root
printf 'agents:\n  status: { model: sonnet, effort: high }\n  new-change: { model: opus }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT" bash "$SYNC" >/dev/null )
assert "per-repo override writes project-level file" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "per-repo override applies model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "per-repo override applies effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
assert "no project-level file for unlisted skill (implement-next)" '[ ! -f "$SBX/.claude/agents/docket-implement-next.md" ]'
assert "advisory skill in agents: produces NO file (new-change)" '[ ! -f "$SBX/.claude/agents/docket-new-change.md" ]'
rm -rf "$SBX" "$HROOT"

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
printf 'agents:\n  auto-groom-critic: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
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
printf 'agents:\n  rebase-resolver: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
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
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
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
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
# intentionally do NOT generate; $SBX/.claude/agents/docket-status.md does not exist
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check fails when committed file is missing (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "--check reports missing-file drift" 'printf "%s" "$chk_out" | grep -q "drift"'

# A repo with no agents: block has nothing to check -> passes.
rm -f "$SBX/.docket.yml"; : > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when no agents: block (rc=0)" '[ "$chk_rc" = "0" ]'
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

# ---- Task 6: advisory recommendation in the interactive skills -------------
NEWC="$REPO/skills/docket-new-change/SKILL.md"
GROOM="$REPO/skills/docket-groom-next/SKILL.md"
assert "new-change carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$NEWC"'
assert "new-change recommends sonnet" 'grep -qi "sonnet" "$NEWC"'
assert "groom-next carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$GROOM"'
assert "groom-next recommends sonnet/high" 'grep -qiE "sonnet[^A-Za-z]+high|high[^A-Za-z]+sonnet" "$GROOM"'
# Non-vacuous: it must be advisory, not a hard requirement (we cannot force the session model).
assert "new-change frames it as advisory" 'grep -qi "advisory" "$NEWC"'

exit $fail
