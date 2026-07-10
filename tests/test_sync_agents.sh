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

# -- global layer (harness-first, change 0050): config.yml agents: default: block overrides model/effort --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: haiku, effort: low }\n    implement-next: { effort: auto }\n' > "$SBX/.config/docket/config.yml"
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
printf 'agents:\n  default:\n    status: { model: haiku }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global cursor block wins for cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "global claude falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# -- per-repo layer (harness-first): .docket.yml agents.default: => project-level files (machine-local since 0051) --
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

# 0048 Piece 2 — the Cursor dispatch rule is generated per-repo when cursor is listed.
make_sandbox
HROOT48R="$(mktemp -d)"; mkdir -p "$HROOT48R/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48R" bash "$SYNC" >/dev/null )
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 rule: per-repo docket-dispatch.mdc written for cursor" '[ -f "$RULE" ]'
assert "0048 rule: carries alwaysApply: true frontmatter" 'grep -q "^alwaysApply: true" "$RULE"'
assert "0048 rule: has the required dispatch pattern heading" 'grep -q "## Required dispatch pattern" "$RULE"'
assert "0048 rule: has a subsection for every built-in agent (8)" \
  '[ "$(grep -cE "^## docket-.* — dispatch only" "$RULE")" = "8" ]'
assert "0048 rule: names docket-implement-next as a subsection" 'grep -q "^## docket-implement-next — dispatch only" "$RULE"'
assert "0048 rule: names docket-status as a subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
assert "0048 rule: no subsection for a non-existent agent" '! grep -q "docket-nonexistent" "$RULE"'
assert "0048 rule: deterministic order — adr before status" \
  '[ "$(grep -n "^## docket-adr — dispatch only" "$RULE" | cut -d: -f1)" -lt "$(grep -n "^## docket-status — dispatch only" "$RULE" | cut -d: -f1)" ]'
rm -rf "$SBX" "$HROOT48R"

# 0048 Piece 2 — cursor NOT listed => no per-repo rule (claude/other harness gets none).
make_sandbox
HROOT48N="$(mktemp -d)"; mkdir -p "$HROOT48N/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48N" bash "$SYNC" >/dev/null )
assert "0048 rule: no dispatch rule for a claude-only repo" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 rule: no rules dir under .claude" '[ ! -e "$SBX/.claude/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX" "$HROOT48N"

# 0048 Piece 2 — user-level: rule written to ~/.cursor/rules when ~/.cursor present, skipped when absent.
make_sandbox                                  # make_sandbox creates .claude + .agents; .cursor ABSENT
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0048 rule: user-level rule SKIPPED when ~/.cursor absent" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
mkdir -p "$SBX/.cursor"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0048 rule: user-level rule WRITTEN when ~/.cursor present" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX"

# 0048 Piece 2 — a built-in agent lacking a fragment gets a minimal auto-block + a warning.
# Simulate by pointing the generator at a scratch clone whose fragment we remove.
make_sandbox
HROOT48F="$(mktemp -d)"; mkdir -p "$HROOT48F/.claude"
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Remove one fragment in a throwaway copy of the repo scripts so the auto-block path fires.
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/sync-agents.sh" "$SCRATCH/"
rm -f "$SCRATCH/cursor-rules/dispatch/docket-status.md"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48F" bash "$SCRATCH/sync-agents.sh" 2>&1 >/dev/null)"
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 auto-block: warns about the missing fragment" 'printf "%s" "$gen_err" | grep -qi "no dispatch fragment for docket-status"'
assert "0048 auto-block: still emits a docket-status subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
assert "0048 auto-block: subsection includes a Task subagent_type" 'grep -q "subagent_type: \"docket-status\"" "$RULE"'
rm -rf "$SBX" "$HROOT48F" "$SCRATCH"

# 0048 Piece 2 --check — a committed dispatch rule that drifts fails --check.
make_sandbox
HROOT48C="$(mktemp -d)"; mkdir -p "$HROOT48C/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: passes for an in-sync committed rule (rc=0)" '[ "$chk_rc" = "0" ]'
# Hand-edit the committed rule -> advisory (leg c; content staleness never fails CI).
printf '\n<!-- tampered -->\n' >> "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: advisory-flags a hand-edited rule (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 rule-check: names the dispatch rule in the advisory report" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-dispatch.mdc"'
# Delete the committed rule -> advisory (missing local file).
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )   # regenerate clean
rm -f "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: advisory-flags a missing committed rule (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 rule-check: missing-rule advisory names it" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-dispatch.mdc"'
rm -rf "$SBX" "$HROOT48C"

# 0048 Piece 3 — removing a built-in agent prunes its generated files (both layers) + rule subsection.
make_sandbox
HROOT48P="$(mktemp -d)"; mkdir -p "$HROOT48P/.cursor"   # present user-level cursor root
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Scratch clone we can mutate (remove a built-in agent + its fragment).
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/sync-agents.sh" "$SCRATCH/"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: adr generated before removal (per-repo)" '[ -f "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: adr generated before removal (user-level)" '[ -f "$HROOT48P/.cursor/agents/docket-adr.md" ]'
# Remove the built-in agent + its fragment, regenerate: the orphan must be pruned.
rm -f "$SCRATCH/agents/docket-adr.md" "$SCRATCH/cursor-rules/dispatch/docket-adr.md"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: removed built-in pruned from per-repo .cursor/agents" '[ ! -e "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: removed built-in pruned from user-level .cursor/agents" '[ ! -e "$HROOT48P/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: rule subsection for removed agent dropped" '! grep -q "^## docket-adr — dispatch only" "$SBX/.cursor/rules/docket-dispatch.mdc"'
assert "0048 prune: a surviving agent remains" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT48P" "$SCRATCH"

# 0048 Piece 3 — de-listing cursor prunes its per-repo docket files + rule, keeps a co-located non-docket file.
make_sandbox
HROOT48D="$(mktemp -d)"; mkdir -p "$HROOT48D/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48D" bash "$SYNC" >/dev/null )
: > "$SBX/.cursor/agents/my-own-agent.md"          # operator's own co-located file
assert "0048 delist: cursor agents present before de-list" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048 delist: cursor rule present before de-list" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
# De-list cursor.
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48D" bash "$SYNC" >/dev/null )
assert "0048 delist: cursor docket agents pruned" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048 delist: cursor dispatch rule pruned" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 delist: operator's co-located non-docket file preserved" '[ -f "$SBX/.cursor/agents/my-own-agent.md" ]'
assert "0048 delist: claude still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT48D"

# 0048 Piece 3 --check — an orphaned local file is reported as advisory, NOT deleted
# (change 0051: orphaned per-repo files are untracked local artifacts now, not CI-fatal).
make_sandbox
HROOT48O="$(mktemp -d)"; mkdir -p "$HROOT48O/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48O" bash "$SYNC" >/dev/null )
: > "$SBX/.claude/agents/docket-bogus.md"           # an orphan: no built-in docket-bogus
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48O" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 orphan-check: advisory-flags the orphan (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 orphan-check: names the orphaned file" 'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-bogus.md"'
assert "0048 orphan-check: --check does NOT delete the orphan" '[ -f "$SBX/.claude/agents/docket-bogus.md" ]'
rm -rf "$SBX" "$HROOT48O"

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
assert "--check advisory-flags critic drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check advisory-flags critic drift (names file)" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-auto-groom-critic.md"'
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
assert "--check advisory-flags rebase-resolver drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check advisory-flags rebase-resolver drift (names file)" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-rebase-resolver.md"'
rm -rf "$SBX" "$HROOT3"

# ---- Task 3: --check drift gate --------------------------------------------
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )   # generate committed project file
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when committed agents match config (rc=0)" '[ "$chk_rc" = "0" ]'

# Out-of-band edit to a local project-level file -> advisory (leg c), never CI-fatal.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check advisory-flags drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check reports an advisory" 'printf "%s" "$chk_out" | grep -q "advisory"'

# Local file removed after having been generated once (block already written) ->
# advisory only (leg c; missing local file is never CI-fatal).
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )   # generate + write the gitignore block
rm -f "$SBX/.claude/agents/docket-status.md"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check advisory-flags a missing local file (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check reports the missing-local-file advisory" 'printf "%s" "$chk_out" | grep -q "advisory"'

# leg (a): opted-in repo whose .gitignore block was never written (sync never ran) -> rc!=0.
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check leg-a: missing gitignore block fails (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "--check leg-a: names the gitignore block" 'printf "%s" "$chk_out" | grep -qi "gitignore"'

# 0048 opt-in: a .docket.yml present for change-tracking only (no agents: / no agent_harnesses) does
# NOT opt into per-repo generation — nothing is written and --check stays a no-op (backward-compat).
make_sandbox                                          # SBX = the repo
HROOTTO="$(mktemp -d)"; mkdir -p "$HROOTTO/.claude"   # separate user-level root
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"      # tracking-only: no opt-in keys
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTTO" bash "$SYNC" >/dev/null )
assert "0048 opt-in: tracking-only repo writes NO project-level wrappers" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTTO" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 opt-in: tracking-only repo --check is a no-op (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTTO"

# 0048 opt-in: agent_harnesses alone (NO agents: block) opts in — the real Cursor-repo case:
# full built-in set + dispatch rule generated for the listed harnesses, at built-in defaults.
make_sandbox
HROOTAH="$(mktemp -d)"; mkdir -p "$HROOTAH/.claude"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.docket.yml"   # no agents: block at all
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTAH" bash "$SYNC" >/dev/null )
assert "0048 opt-in: agent_harnesses-only generates full set for cursor" '[ "$(find "$SBX/.cursor/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
assert "0048 opt-in: agent_harnesses-only generates full set for claude" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
assert "0048 opt-in: agent_harnesses-only generates the cursor dispatch rule" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 opt-in: agent_harnesses-only wrappers carry built-in default (no overrides)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX" "$HROOTAH"

# 0048: a repo with NO .docket.yml at all has nothing to check -> passes.
make_sandbox
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes when no .docket.yml (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"

# ---- Task 5: docket-convention documents the agent layer -------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention documents the agents: config block" 'grep -q "agents:" "$CONV"'
assert "convention names the generator sync-agents.sh" 'grep -q "sync-agents.sh" "$CONV"'
assert "convention states the precedence" 'grep -qi "repo-local > repo-committed > global > built-in" "$CONV"'
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

# 0048 doc: convention states per-repo generates the full built-in set (config override-only)
# and that cursor gets a generated docket-dispatch.mdc rule.
assert "0048 doc: convention says per-repo writes the full built-in set" 'grep -qiE "full (built-in )?(agent )?set" "$CONV"'
assert "0048 doc: convention says the agents: block is override-only" 'grep -qi "override-only" "$CONV"'
assert "0048 doc: convention names the cursor dispatch rule" 'grep -q "docket-dispatch.mdc" "$CONV"'

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
assert "0048: [cursor]-only leaves the pre-existing user .claude dir intact" '[ -d "$SBX/.claude" ]'
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
assert "0048: empty-list leaves the pre-existing user .claude dir intact" '[ -d "$SBX/.claude" ]'
rm -rf "$SBX" "$HROOTE0"

# --check must span every listed harness: drift in a .cursor/agents file fails CI.
make_sandbox
HROOTF="$(mktemp -d)"; mkdir -p "$HROOTF/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" >/dev/null )   # generate both harness files
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: passes when both harness files in sync (rc=0)" '[ "$chk_rc" = "0" ]'
# Drift the CURSOR file only -> advisory (leg c), never CI-fatal.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.cursor/agents/docket-status.md"; rm -f "$SBX/.cursor/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: advisory-flags .cursor/agents drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0045 check: advisory report names the cursor harness" 'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "cursor"'
# A listed-harness file never generated locally -> advisory (missing local file).
rm -f "$SBX/.cursor/agents/docket-status.md"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: advisory-flags missing cursor file (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0045 check: missing-file advisory names cursor" 'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "cursor"'
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
assert "0047 §agent-cfg: names the global layer ~/.config/docket/config.yml" \
  'grep -qF "~/.config/docket/config.yml" <<<"$sec"'
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
# Pre-run a normal sync so the .gitignore block exists (leg a green) and the legacy
# committed-config-shape leg is isolated (still rc!=0 — CI-meaningful, not advisory).
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" >/dev/null 2>&1 )
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

# ============================================================================
# Change 0050 — agents.yaml -> config.yml auto-migration (owned by sync-agents.sh)
# ============================================================================

# Happy path: agents.yaml (old top-level harness-first map) is rewritten under agents:
# in config.yml, the original renamed .migrated, the run logs loudly, values apply.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku, effort: low }\n' > "$SBX/.config/docket/agents.yaml"
mig_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"
assert "0050 mig: config.yml gains an agents: block" 'grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: old file renamed to .migrated" '[ -f "$SBX/.config/docket/agents.yaml.migrated" ] && [ ! -e "$SBX/.config/docket/agents.yaml" ]'
assert "0050 mig: logs the migration loudly" 'printf "%s" "$mig_err" | grep -qi "migrat"'
assert "0050 mig: migrated values applied to wrappers" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
# Idempotency: a second run leaves config.yml byte-identical (no duplicate agents: block).
cfg_before="$(cat "$SBX/.config/docket/config.yml")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
cfg_after="$(cat "$SBX/.config/docket/config.yml")"
assert "0050 mig: second run no-ops on config.yml" '[ "$cfg_before" = "$cfg_after" ]'
assert "0050 mig: exactly one agents: block" '[ "$(grep -cE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml")" = "1" ]'
rm -rf "$SBX"

# Migration preserves pre-existing non-agents config.yml content.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'auto_groom: true\n' > "$SBX/.config/docket/config.yml"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 mig: pre-existing config.yml keys preserved" 'grep -q "^auto_groom: true" "$SBX/.config/docket/config.yml"'
assert "0050 mig: agents: appended alongside" 'grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: values from the appended block apply" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# Migration into a config.yml whose last line lacks a trailing newline must not glue keys.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'auto_groom: true' > "$SBX/.config/docket/config.yml"     # NO trailing newline
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 mig: no-trailing-newline config.yml not glued" 'grep -q "^auto_groom: true$" "$SBX/.config/docket/config.yml" && grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: no-trailing-newline values still apply" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# Stale twin: config.yml already has agents: AND a live agents.yaml is present ->
# warn stale, do NOT read it, do NOT rename it (only the migration renames).
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.config/docket/config.yml"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
stale_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"
assert "0050 stale: warns agents.yaml is stale/unread" 'printf "%s" "$stale_err" | grep -qi "stale"'
assert "0050 stale: config.yml value wins" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0050 stale: agents.yaml left in place" '[ -f "$SBX/.config/docket/agents.yaml" ]'
rm -rf "$SBX"

# No dual-read: a lone agents.yaml.migrated (post-migration state) is never read.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml.migrated"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 no-dual-read: .migrated is not read (built-in model)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX"

# ============================================================================
# Change 0050 — global agent_harnesses scopes the USER-LEVEL pass only
# ============================================================================

# Extends + narrows: the global list overrides presence-on-disk detection.
make_sandbox                                   # creates .claude + .agents; .cursor ABSENT
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah: listed ABSENT harness extended (cursor created+written)" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0050 gah: listed present harness written (claude)" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah: present-but-UNLISTED harness narrowed (.agents untouched)" '[ ! -e "$SBX/.agents/agents/docket-status.md" ]'
assert "0050 gah: user-level cursor dispatch rule written when cursor listed" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX"

# Global [] => the user-level pass writes nothing (explicit empty list, not "unset"),
# and existing user-level docket wrappers are pruned (every known harness is de-listed).
make_sandbox
mkdir -p "$SBX/.config/docket" "$SBX/.claude/agents"
: > "$SBX/.claude/agents/docket-status.md"          # stale wrapper from an earlier run
printf 'agent_harnesses: []\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah []: no user-level files written despite present .claude" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah []: harness root preserved after prune" '[ -d "$SBX/.claude" ]'
rm -rf "$SBX"

# Unset global key => presence-on-disk detection unchanged (regression pin).
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah unset: presence detection still writes .claude" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah unset: absent harness still skipped" '[ ! -d "$SBX/.cursor/agents" ]'
rm -rf "$SBX"

# Unknown token in the GLOBAL list: warned + dropped, not fatal.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [claude, bogus]\n' > "$SBX/.config/docket/config.yml"
gah_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"; gah_rc=$?
assert "0050 gah unknown: not fatal (rc=0)" '[ "$gah_rc" = "0" ]'
assert "0050 gah unknown: warns and names the token" 'printf "%s" "$gah_err" | grep -qi "unknown agent_harnesses token" && printf "%s" "$gah_err" | grep -q "bogus"'
assert "0050 gah unknown: known harness still written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# Scope split: the global key never opts a repo into per-repo generation, and the
# per-repo committed pass is governed SOLELY by the repo's own agent_harnesses.
REPO50="$(mktemp -d)"; HROOT50="$(mktemp -d)"
mkdir -p "$HROOT50/.claude" "$HROOT50/.config/docket"
printf 'metadata_branch: docket\n' > "$REPO50/.docket.yml"          # tracking-only repo
printf 'agent_harnesses: [claude]\n' > "$HROOT50/.config/docket/config.yml"
( cd "$REPO50" && DOCKET_HARNESS_ROOT="$HROOT50" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah scope: global key does NOT opt repo into per-repo generation" '[ ! -e "$REPO50/.claude/agents/docket-status.md" ]'
assert "0050 gah scope: user-level still written" '[ -f "$HROOT50/.claude/agents/docket-status.md" ]'
rm -rf "$REPO50" "$HROOT50"

REPO51="$(mktemp -d)"; HROOT51="$(mktemp -d)"
mkdir -p "$HROOT51/.claude" "$HROOT51/.config/docket"
printf 'agent_harnesses: [claude]\n' > "$REPO51/.docket.yml"        # repo opts in: claude only
printf 'agent_harnesses: [cursor]\n' > "$HROOT51/.config/docket/config.yml"
( cd "$REPO51" && DOCKET_HARNESS_ROOT="$HROOT51" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah scope: per-repo pass follows the REPO list (claude written)" '[ -f "$REPO51/.claude/agents/docket-status.md" ]'
assert "0050 gah scope: per-repo pass ignores the global list (no repo .cursor)" '[ ! -e "$REPO51/.cursor/agents/docket-status.md" ]'
assert "0050 gah scope: global [cursor] scopes user-level (cursor written)" '[ -f "$HROOT51/.cursor/agents/docket-status.md" ]'
assert "0050 gah scope: user-level claude NOT written (narrowed by global list)" '[ ! -e "$HROOT51/.claude/agents/docket-status.md" ]'
rm -rf "$REPO51" "$HROOT51"

# Narrowing the global list on a later run prunes the de-listed harness's USER-LEVEL
# docket-owned files (mirrors the per-repo de-list rule); user content + the root survive.
make_sandbox
mkdir -p "$SBX/.config/docket" "$SBX/.cursor"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah prune: cursor user files present before narrowing" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
: > "$SBX/.cursor/agents/my-own-agent.md"
printf 'agent_harnesses: [claude]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah prune: de-listed cursor docket agents pruned" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0050 gah prune: de-listed cursor dispatch rule pruned" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0050 gah prune: user's own co-located file preserved" '[ -f "$SBX/.cursor/agents/my-own-agent.md" ]'
assert "0050 gah prune: harness root dir preserved" '[ -d "$SBX/.cursor" ]'
assert "0050 gah prune: listed claude still written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# ---- Change 0050 — README "Global config" section + convention three-layer story ----
# Extract the new dedicated README section (heading -> next `## `), assert within it.
gsec="$(awk '/^##[[:space:]].*[Gg]lobal config/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"
assert "0050 doc: README has a Global config section" '[ -n "$gsec" ]'
assert "0050 doc: §global names the canonical path" 'grep -qF "~/.config/docket/config.yml" <<<"$gsec"'
assert "0050 doc: §global states the same-schema rule" 'grep -qiE "same schema as .?\.docket\.yml" <<<"$gsec"'
assert "0050 doc: §global states per-key precedence" 'grep -qi "repo-local > repo-committed > global > built-in" <<<"$gsec"'
assert "0050 doc: §global states coordination keys are per-repo-only" 'grep -qi "per-repo-only" <<<"$gsec"'
assert "0050 doc: §global names the agents.yaml migration" 'grep -qF "agents.yaml.migrated" <<<"$gsec"'
assert "0050 doc: §global scopes agent_harnesses to the user-level pass" 'grep -qiE "user-level pass" <<<"$gsec"'
# Tuning section gains the both-passes clarification (LEARNINGS #49 — surface end-to-end).
sec="$(awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"
assert "0050 doc: tuning section states sync-agents writes BOTH layers" 'grep -qiE "both" <<<"$sec" && grep -qiE "project (level )?win|project-over-user|project wins" <<<"$sec"'
# Convention: Configuration documents the three-layer story + the fence.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "0050 doc: convention names config.yml" 'grep -qF "config.yml" "$CONV"'
assert "0050 doc: convention states the coordination-key fence" 'grep -qi "fence" "$CONV" && grep -qi "per-repo-only" "$CONV"'
assert "0050 doc: convention Agent layer global row points at config.yml agents: block" \
  'grep -qE "^\| Global \|.*config\.yml" "$CONV"'

# ---- Change 0051 doc sentinels ----
assert "0051 doc: README documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$READMEF"'
assert "0051 doc: README states generated agents are machine-local, never committed" \
  'grep -qiE "machine-local" "$READMEF" && grep -qiE "never committed" "$READMEF"'
assert "0051 doc: README documents the docket:generated gitignore block" 'grep -qF "docket:generated" "$READMEF"'
assert "0051 doc: README documents the migration (git rm --cached / one commit)" 'grep -qiE "migrat" "$READMEF" && grep -qF -e "--cached" "$READMEF"'
assert "0051 doc: convention documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$CONV"'
assert "0051 doc: convention states all-local generation (gitignored, never committed)" 'grep -qiE "gitignored, never committed|machine-local, never committed" "$CONV"'
assert "0051 doc: convention documents the three-leg --check" 'grep -qi "advisory" "$CONV" && grep -qF "docket:generated" "$CONV"'
assert "0051 doc: sample .docket.yml agents comment states machine-local generation" 'grep -qi "machine-local" "$REPO/.docket.yml"'
assert "0051 doc: sample .docket.yml drops the stale agents.yaml global reference" '! grep -q "agents.yaml" "$REPO/.docket.yml"'

# ============================================================================
# Change 0051 — four-layer per-field agents: resolution; all-local generation.
# Precedence: local.agents.H.X -> local.default.X -> committed.H.X -> committed.default.X
#             -> global.H.X -> global.default.X -> built-in. THE 0050 BUG FIX:
# a global agents: block now REACHES per-repo generated files (no committed shadow).
# ============================================================================

# (4L-a) THE FIX — opted-in repo + global agents: + no repo/local override
# => the generated project-level file carries the GLOBAL model (was: built-in + SHADOWED warning).
make_sandbox
HROOT51A="$(mktemp -d)"; mkdir -p "$HROOT51A/.claude" "$HROOT51A/.config/docket"
printf 'agents:\n  default:\n    status: { model: global-model-x }\n' > "$HROOT51A/.config/docket/config.yml"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
sw_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51A" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 4L: global agents value reaches the per-repo generated file" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "global-model-x" ]'
assert "0051 4L: the 0050 SHADOWED stopgap warning is gone" '! printf "%s" "$sw_err" | grep -q "SHADOWED"'
rm -rf "$SBX" "$HROOT51A"

# (4L-b) full chain: local beats committed beats global; per-FIELD independence
# (model from local, effort from committed) and harness-over-default within a layer.
make_sandbox
HROOT51B="$(mktemp -d)"; mkdir -p "$HROOT51B/.claude" "$HROOT51B/.config/docket"
printf 'agents:\n  default:\n    status: { model: global-m, effort: low }\n' > "$HROOT51B/.config/docket/config.yml"
printf 'agents:\n  default:\n    status: { model: committed-m, effort: high }\n' > "$SBX/.docket.yml"
printf 'agents:\n  default:\n    status: { model: local-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51B" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: local model beats committed+global"        '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
assert "0051 4L: effort unset locally falls to committed"   '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# harness key in a LOWER layer still loses to default in a HIGHER layer for that field:
printf 'agents:\n  claude:\n    status: { model: committed-claude-m }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51B" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: local default beats committed harness key" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
rm -rf "$SBX" "$HROOT51B"

# (4L-c) opt-in via the LOCAL file alone — a machine opts a tracking-only repo in
# without touching committed config; local agent_harnesses governs the target list.
make_sandbox
HROOT51C="$(mktemp -d)"; mkdir -p "$HROOT51C/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"           # tracking-only committed file
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: local-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51C" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 opt-in: local file alone opts in (claude generated)"  '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0051 opt-in: local agent_harnesses honored (cursor too)"   '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0051 opt-in: cursor dispatch rule generated"               '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0051 opt-in: local model applied"                          '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
rm -rf "$SBX" "$HROOT51C"

# (4L-d) local agent_harnesses BEATS committed (key-level precedence, not a merge).
make_sandbox
HROOT51D="$(mktemp -d)"; mkdir -p "$HROOT51D/.claude"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.docket.yml"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51D" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gah: local list wins (claude generated)"     '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0051 gah: committed cursor overridden away"       '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT51D"

# (4L-e) tracking-only repo with NEITHER file opted in: still zero files (regression pin).
make_sandbox
HROOT51E="$(mktemp -d)"; mkdir -p "$HROOT51E/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51E" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 opt-in: neither file => zero project files" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT51E"

# (4L-f) malformed .docket.local.yml (a directory): warn + skip, run still succeeds,
# committed layer still honored.
make_sandbox
HROOT51F="$(mktemp -d)"; mkdir -p "$HROOT51F/.claude"
printf 'agents:\n  default:\n    status: { model: committed-m }\n' > "$SBX/.docket.yml"
mkdir "$SBX/.docket.local.yml"
mf_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51F" bash "$SYNC" 2>&1 >/dev/null)"; mf_rc=$?
assert "0051 malformed local: not fatal (rc=0)"        '[ "$mf_rc" = "0" ]'
assert "0051 malformed local: warns and names the file" 'printf "%s" "$mf_err" | grep -qi "docket.local.yml"'
assert "0051 malformed local: committed layer still applies" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "committed-m" ]'
rm -rf "$SBX" "$HROOT51F"

# (4L-g) tab-indented local YAML resolves (LEARNINGS #46 — indent classes must be [^[:space:]]).
make_sandbox
HROOT51G="$(mktemp -d)"; mkdir -p "$HROOT51G/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf 'agents:\n\tdefault:\n\t\tstatus: { model: tab-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51G" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: tab-indented local YAML resolves" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "tab-m" ]'
rm -rf "$SBX" "$HROOT51G"

# (rider) prune_orphans empty-scan_dirs guard: bash 3.2 + set -u with NO harness roots
# on disk AND no opt-in must not crash ("${scan_dirs[@]}" on an empty array).
SBXR="$(mktemp -d)"                                   # deliberately NO .claude/.agents dirs
rid_rc=0
( cd "$SBXR" && DOCKET_HARNESS_ROOT="$SBXR" /bin/bash "$SYNC" >/dev/null 2>&1 ) || rid_rc=$?
assert "0051 rider: empty scan_dirs run succeeds under /bin/bash (rc=0)" '[ "$rid_rc" = "0" ]'
rm -rf "$SBXR"

# ============================================================================
# Change 0051 — managed .gitignore block (# docket:generated:start/end)
# ============================================================================

# (gi-a) opted-in repo: block created (file didn't exist), loud "commit" notice,
# patterns strictly docket-scoped, emitted from the harness table (all 6 tokens).
make_sandbox
HROOTGA="$(mktemp -d)"; mkdir -p "$HROOTGA/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
gi_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
GI="$SBX/.gitignore"
assert "0051 gi: .gitignore created with the managed block" 'grep -q "^# docket:generated:start" "$GI" && grep -q "^# docket:generated:end$" "$GI"'
assert "0051 gi: block ignores .docket.local.yml"            'grep -q "^\.docket\.local\.yml$" "$GI"'
assert "0051 gi: block ignores claude agents pattern"        'grep -q "^\.claude/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores cursor agents pattern"        'grep -q "^\.cursor/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores the cursor dispatch rule"     'grep -q "^\.cursor/rules/docket-dispatch\.mdc$" "$GI"'
assert "0051 gi: loud commit-this notice"                    'printf "%s" "$gi_err" | grep -qi "commit"'
assert "0051 gi: every block line is docket-scoped (starts with . or #)" \
  '! awk "/# docket:generated:start/,/# docket:generated:end/" "$GI" | grep -qvE "^(#|\.)"'

# (gi-b) idempotent: second run leaves .gitignore byte-identical and prints no notice.
gi_before="$(cat "$GI")"
gi_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 gi: second run byte-identical"    '[ "$gi_before" = "$(cat "$GI")" ]'
assert "0051 gi: second run no UPDATED notice" '! printf "%s" "$gi_err2" | grep -q "managed block"'

# (gi-c) hand-edit inside the block repaired; content OUTSIDE the markers preserved.
printf 'my-own-ignore/\n%s\n' "$(cat "$GI")" > "$GI"          # user content above the block
sed -i.bak '/docket-dispatch/d' "$GI"; rm -f "$GI.bak"        # vandalize the block
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: hand-edited block repaired"   'grep -q "docket-dispatch" "$GI"'
assert "0051 gi: user content preserved"       'grep -q "^my-own-ignore/$" "$GI"'
assert "0051 gi: exactly one block after repair" '[ "$(grep -c "^# docket:generated:start" "$GI")" = "1" ]'
rm -rf "$SBX" "$HROOTGA"

# (gi-d) tracking-only repo WITH a .docket.local.yml that has NO opt-in keys: the block
# is still written (the local file itself must never be committable); zero agent files.
make_sandbox
HROOTGD="$(mktemp -d)"; mkdir -p "$HROOTGD/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf 'finalize:\n  gate: off\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGD" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: local-file-present repo gets the block"  'grep -q "^# docket:generated:start" "$SBX/.gitignore"'
assert "0051 gi: but still generates zero agent files"    '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGD"

# (gi-e) repo with NEITHER signal: .gitignore never touched/created (LEARNINGS #48 posture).
make_sandbox
HROOTGE="$(mktemp -d)"; mkdir -p "$HROOTGE/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGE" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
rm -rf "$SBX" "$HROOTGE"

# (gi-f) UNTERMINATED block (start marker, no end): refuse to rewrite, warn, preserve
# every byte — user content after the dangling marker must survive.
make_sandbox
HROOTGF="$(mktemp -d)"; mkdir -p "$HROOTGF/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf '# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket.local.yml\nnode_modules/\n' > "$SBX/.gitignore"
gi_before="$(cat "$SBX/.gitignore")"
gf_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGF" bash "$SYNC" 2>&1 >/dev/null)"; gf_rc=$?
assert "0051 gi-f: unterminated block run still succeeds (rc=0)" '[ "$gf_rc" = "0" ]'
assert "0051 gi-f: warns the block is corrupt/unterminated" 'printf "%s" "$gf_err" | grep -qi "untermin\|corrupt"'
assert "0051 gi-f: file left byte-identical (user content preserved)" '[ "$gi_before" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX" "$HROOTGF"

# ============================================================================
# Change 0051 — migration (0048-era tracked wrappers) + --check three legs
# ============================================================================

# git-repo fixture: sandbox repo with identity + one commit (for ls-files-based legs).
mkgitrepo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
}

# (mig-a) 0048-era repo: tracked wrappers + rule -> deleted from the worktree, block
# written, local set regenerated, single migration commit printed. Idempotent.
mkgitrepo
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
mkdir -p "$SBX/.claude/agents" "$SBX/.cursor/agents" "$SBX/.cursor/rules"
printf 'stale 0048 wrapper\n' > "$SBX/.claude/agents/docket-status.md"
printf 'stale 0048 wrapper\n' > "$SBX/.cursor/agents/docket-status.md"
printf 'stale 0048 rule\n'    > "$SBX/.cursor/rules/docket-dispatch.mdc"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m "0048-era state"
mig_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"; mig_rc=$?
assert "0051 mig: run succeeds (rc=0)"                     '[ "$mig_rc" = "0" ]'
assert "0051 mig: announces the migration"                 'printf "%s" "$mig_err" | grep -qi "migrat"'
assert "0051 mig: prints git rm --cached instructions"     'printf "%s" "$mig_err" | grep -q -e "git rm" '
assert "0051 mig: gitignore block written"                 'grep -q "^# docket:generated:start" "$SBX/.gitignore"'
assert "0051 mig: local files regenerated (fresh content)" 'grep -q "^model: sonnet" "$SBX/.claude/agents/docket-status.md"'
assert "0051 mig: full local set regenerated"              '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
# perform the printed migration commit; second run must NOT re-announce
( cd "$SBX" && git rm -r -q --cached '.claude/agents/docket-*.md' '.cursor/agents/docket-*.md' '.cursor/rules/docket-dispatch.mdc' && git add .gitignore && git commit -q -m "docket: agent files go machine-local" )
mig_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 mig: idempotent — post-commit run is silent about migration" '! printf "%s" "$mig_err2" | grep -qi "migrat"'
# and --check is fully green now (all three legs)
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 mig: post-migration --check green (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTM"

# (mig-b) stale tracked wrappers in a repo with NO current opt-in and no .gitignore:
# the printed remedy must be runnable AS PRINTED (no git add .gitignore clause).
mkgitrepo
HROOTMB="$(mktemp -d)"; mkdir -p "$HROOTMB/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"        # tracking-only: NOT opted in
mkdir -p "$SBX/.claude/agents"
printf 'stale 0048 wrapper\n' > "$SBX/.claude/agents/docket-status.md"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m "0048-era stale state"
migb_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTMB" bash "$SYNC" 2>&1 >/dev/null)"; migb_rc=$?
assert "0051 mig-b: run succeeds (rc=0)"                      '[ "$migb_rc" = "0" ]'
assert "0051 mig-b: remedy omits git add .gitignore"          'printf "%s" "$migb_err" | grep -e "git rm" | grep -v -q "git add .gitignore"'
assert "0051 mig-b: no .gitignore was created (not wanted)"   '[ ! -e "$SBX/.gitignore" ]'
# the printed remedy must actually run: extract and eval it, then leg (b) goes green.
remedy="$(printf '%s\n' "$migb_err" | sed -n 's/^sync-agents:[[:space:]]*\(git rm .*\)$/\1/p' | head -n1)"
assert "0051 mig-b: a runnable remedy line was printed"       '[ -n "$remedy" ]'
( cd "$SBX" && eval "$remedy" ) >/dev/null 2>&1
migb_chk="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTMB" bash "$SYNC" --check 2>&1)"; migb_chk_rc=$?
assert "0051 mig-b: after running the printed remedy, --check leg (b) green (rc=0)" '[ "$migb_chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTMB"

# (chk-a) leg (a): opted-in repo, block missing (sync never ran) -> rc!=0 naming the block.
make_sandbox
HROOTCA="$(mktemp -d)"; mkdir -p "$HROOTCA/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-a: missing block fails --check (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0051 chk-a: names the gitignore block"           'printf "%s" "$chk_out" | grep -qi "gitignore"'
# stale block (hand-pruned pattern) also fails:
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak '/docket-dispatch/d' "$SBX/.gitignore"; rm -f "$SBX/.gitignore.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-a: stale block fails --check (rc!=0)"   '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOTCA"

# (chk-b) leg (b): tracked generated file -> rc!=0 with the migration remedy.
mkgitrepo
HROOTCB="$(mktemp -d)"; mkdir -p "$HROOTCB/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCB" bash "$SYNC" >/dev/null 2>&1 )   # block + local files
git -C "$SBX" add -A -f; git -C "$SBX" commit --quiet -m "wrongly track everything"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCB" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-b: tracked generated file fails --check (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0051 chk-b: names a tracked path"                          'printf "%s" "$chk_out" | grep -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCB"

# (chk-c) leg (c): content staleness is ADVISORY — rc stays 0, output says advisory.
make_sandbox
HROOTCC="$(mktemp -d)"; mkdir -p "$HROOTCC/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-c: content drift is advisory (rc=0)"  '[ "$chk_rc" = "0" ]'
assert "0051 chk-c: advisory names the drifted file"   'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCC"

# (chk-d) fresh clone of a MIGRATED repo: committed .docket.yml (opted-in) + committed
# block, NO generated files -> --check fully green (leg c vacuous on CI).
mkgitrepo
HROOTCD="$(mktemp -d)"; mkdir -p "$HROOTCD/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCD" bash "$SYNC" >/dev/null 2>&1 )     # writes block + files
find "$SBX" -name 'docket-*.md' -path '*/agents/*' -delete                       # simulate the fresh clone
git -C "$SBX" add .docket.yml .gitignore; git -C "$SBX" commit --quiet -m "migrated repo"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCD" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-d: fresh migrated clone --check green (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTCD"

exit $fail
