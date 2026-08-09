#!/usr/bin/env bash
# tests/test_sync_agents.sh — run: bash tests/test_sync_agents.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

# ---- Task 1: built-in wrapper source files ---------------------------------

assert "agents/ source dir exists" '[ -d "$AGENTS" ]'
assert "exactly 16 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'

for w in $AUTONOMOUS; do
  f="$AGENTS/$w.md"
  assert "$w: file exists" '[ -f "$f" ]'
  assert "$w: name matches file" '[ "$(fm "$f" name)" = "$w" ]'
  assert "$w: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$w: description matches the skill (single source)" \
    '[ "$(fm "$f" description)" = "$(fm "$REPO/skills/$w/SKILL.md" description)" ]'
  assert "$w: skills: injects the skill itself" 'grep -Eq "^skills:.*\b'"$w"'\b" "$f"'
  assert "$w: skills: injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  assert "$w: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Shipped model/effort match the §4 default table. Change 0168 moved these OUT of the wrapper
# sources and into agents/harness-defaults.yml, so they are read with hd_field, not fm(). Reading
# them off the source with fm() would be worse than stale: fm() is a first-match-ANYWHERE read, so
# with no `model:` line left in the frontmatter it scans on into the body and can return prose.
for w in $AUTONOMOUS; do
  n="${w#docket-}"
  assert "$w: shipped model is a known alias or full id" \
    '[[ "$(hd_field "$HD" claude "'"$n"'" model)" =~ ^(opus|sonnet|haiku|fable|claude-[a-z0-9]+(-[a-z0-9]+)*)$ ]]'
  assert "$w: shipped effort in allowed set" \
    '[[ "$(hd_field "$HD" claude "'"$n"'" effort)" =~ ^(low|medium|high|xhigh|max)$ ]]'
done
assert "implement-next shipped = claude-opus-5/medium" \
  '[ "$(hd_field "$HD" claude implement-next model)/$(hd_field "$HD" claude implement-next effort)" = "claude-opus-5/medium" ]'
assert "auto-groom shipped = claude-opus-5/low" \
  '[ "$(hd_field "$HD" claude auto-groom model)/$(hd_field "$HD" claude auto-groom effort)" = "claude-opus-5/low" ]'
assert "finalize-change shipped = claude-opus-5/low" \
  '[ "$(hd_field "$HD" claude finalize-change model)/$(hd_field "$HD" claude finalize-change effort)" = "claude-opus-5/low" ]'
assert "status shipped = claude-haiku-4-5-20251001/medium" \
  '[ "$(hd_field "$HD" claude status model)/$(hd_field "$HD" claude status effort)" = "claude-haiku-4-5-20251001/medium" ]'
assert "adr shipped = claude-opus-5/low" \
  '[ "$(hd_field "$HD" claude adr model)/$(hd_field "$HD" claude adr effort)" = "claude-opus-5/low" ]'

# Advisory/interactive skills must NOT have a wrapper file.
assert "no wrapper for new-change (advisory)" '[ ! -f "$AGENTS/docket-new-change.md" ]'
assert "no wrapper for groom-next (advisory)" '[ ! -f "$AGENTS/docket-groom-next.md" ]'

# ---- Task 2: sync-agents.sh generator --------------------------------------
assert "sync-agents.sh exists and is executable-by-bash" '[ -f "$SYNC" ]'

# -- cached reader equivalence: flow-map and block-shaped field boundaries --
reader_out="$({
  . "$SYNC"
  set +e  # sync-agents.sh enables errexit for direct invocation; this test intentionally does not.
  printf '%s\t%s\n' inline "$(field_of 'x: {model: a.b_c-d, effort: high}' model)"
  printf '%s\t%s\n' block "$(field_of '  model: slash/vendor:id' model)"
  printf '%s\t%s\n' prefix "$(field_of 'x: {model_alias: wrong, model: right}' model)"
  printf '%s\t%s\n' repeated "$(field_of 'x: {model: first, model: last}' model)"
  printf '%s\t%s\n' missing "$(field_of 'x: {effort: high}' model)"
  printf '%s\t%s\n' raw "$(field_of_raw 'x: {model: two words   , effort: high}' model)"
} )"
assert "0175 readers: consumed/raw edge cases preserve fixed semantics" \
  '[ "$reader_out" = "$(printf "inline\\ta.b_c-d\\nblock\\tslash/vendor:id\\nprefix\\tright\\nrepeated\\tlast\\nmissing\\t\\nraw\\ttwo words")" ]'

# The optimization's standing performance oracle: retain real generator behavior while bounding
# only its historic dominant parser commands. The nonzero floor makes a broken shim setup red too.
parser_subprocess_count "$SYNC"
assert "0175 parser subprocess guard: real generation completes successfully" '[ "$FORK_RC" = "0" ]'
assert "0175 parser subprocess guard: shims observed real generator calls" '[ "$FORK_COUNT" -gt 0 ]'
assert "0175 parser subprocess guard: dominant parser commands stay below 400" '[ "$FORK_COUNT" -lt 400 ]'

# -- command-line contract: help/errors must return before any generation side effect --
make_sandbox
HROOT175A="$(mktemp -d)"; mkdir -p "$HROOT175A/.claude"
help_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175A" bash "$SYNC" --help 2>&1)"; help_rc=$?
assert "0175 args: --help succeeds" '[ "$help_rc" = "0" ]'
assert "0175 args: --help prints inventory-safe usage" '/usr/bin/grep -qF "Usage: sync-agents.sh [--check]" <<<"$help_out"'
assert "0175 args: --help writes no wrapper" '[ ! -e "$HROOT175A/.claude/agents/docket-status.md" ]'
bad_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175A" bash "$SYNC" --bogus 2>&1)"; bad_rc=$?
assert "0175 args: unknown flag fails with rc=2" '[ "$bad_rc" = "2" ]'
assert "0175 args: unknown flag names the argument" '/usr/bin/grep -qF "unknown argument: --bogus" <<<"$bad_out"'
assert "0175 args: unknown flag writes no wrapper" '[ ! -e "$HROOT175A/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT175A"

# -- optimized sidecar validation must preserve the raw top-header rule from hd_validate --
SCR175V="$(mktemp -d)"
cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCR175V/"
awk '{ if (!done && $0 == "agents:") { print "agents: # not a bare header"; done=1 } else print }' \
  "$SCR175V/agents/harness-defaults.yml" > "$SCR175V/agents/harness-defaults.yml.tmp"
mv "$SCR175V/agents/harness-defaults.yml.tmp" "$SCR175V/agents/harness-defaults.yml"
make_sandbox
HROOT175V="$(mktemp -d)"; mkdir -p "$HROOT175V/.claude"
v175_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175V" bash "$SCR175V/sync-agents.sh" 2>&1 >/dev/null)"; v175_rc=$?
assert "0175 validator parity: commented top header is rejected" '[ "$v175_rc" != "0" ]'
assert "0175 validator parity: commented top header names the missing bare block" \
  '/usr/bin/grep -qF "no top-level '\''agents:'\'' block" <<<"$v175_err"'
assert "0175 validator parity: rejection happens before wrapper writes" \
  '[ ! -e "$HROOT175V/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT175V" "$SCR175V"

# -- user-level install: built-in wrappers, verbatim, into present harnesses --
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "writes into present .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "writes into present .agents/agents" '[ -f "$SBX/.agents/agents/docket-status.md" ]'
assert "all 16 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "does NOT create an absent harness (.cursor)" '[ ! -d "$SBX/.cursor/agents" ]'
# Change 0168 replaced the byte-identity assert here: the generator now INJECTS the pin from
# agents/harness-defaults.yml instead of copying the source's frontmatter, so byte identity is
# structurally impossible. The mechanism this guarded — an unconfigured run reproduces the source
# faithfully and adds nothing of its own — is asserted directly instead.
assert "no override => body verbatim from the built-in source" \
  'diff -q <(body_of "$REPO/agents/docket-status.md") <(body_of "$SBX/.claude/agents/docket-status.md") >/dev/null'
assert "no override => name/description/skills come from the source" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" name)" = "docket-status" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" description)" = "$(fm "$REPO/agents/docket-status.md" description)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" skills)" = "$(fm "$REPO/agents/docket-status.md" skills)" ]'
assert "no override => the emitted pin is the SHIPPED sidecar value" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "$(hd_field "$HD" claude status model)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "$(hd_field "$HD" claude status effort)" ]'

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
assert "auto keeps the shipped model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-5" ]'
assert "unlisted skill keeps shipped model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-opus-5/low" ]'
rm -rf "$SBX"

# -- global: a per-harness block overrides default for THAT harness only (user-level) --
make_sandbox                                        # .claude and .cursor both present so both get user-level files
mkdir -p "$SBX/.cursor" "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: haiku }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
# 0135: the Cursor wrapper encodes effort INSIDE the model value. This config sets only a model,
# and since change 0168 there is nothing left for the effort to fall through TO on this harness:
# agents/harness-defaults.yml ships no cursor entry for `status`, and the Claude wrapper source is
# no longer a default store. A bare model is the correct output — the `[effort=medium]` this used to
# expect was docket-status's CLAUDE built-in leaking onto a harness that never saw a Claude pin.
# (Bracket encoding itself is still covered below by the agents.default effort fixture, and in
# tests/test_sync_agents_cursor.sh by the explicit model+effort override.)
assert "global cursor block wins for cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "global cursor block leaks no claude effort into the model value" \
  '! grep -q "\[effort=" "$SBX/.cursor/agents/docket-status.md"'
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
assert "0048: unlisted skill NOW generated at shipped default (implement-next)" '[ -f "$SBX/.claude/agents/docket-implement-next.md" ]'
assert "0048: unlisted implement-next carries shipped model (claude-opus-5)" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-5" ]'
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
assert "0048: full set — all 16 built-ins land in project-level .claude/agents" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "0048: listed agent carries its override (status=sonnet)" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0048: UNLISTED agent generated at shipped default (implement-next=claude-opus-5/medium)" \
  '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)/$(fm "$SBX/.claude/agents/docket-implement-next.md" effort)" = "claude-opus-5/medium" ]'
rm -rf "$SBX" "$HROOT48A"

# 0048 Piece 2 — the Cursor dispatch rule is generated per-repo when cursor is listed.
make_sandbox
HROOT48R="$(mktemp -d)"; mkdir -p "$HROOT48R/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48R" bash "$SYNC" >/dev/null )
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 rule: per-repo docket-dispatch.mdc written for cursor" '[ -f "$RULE" ]'
assert "0048 rule: carries alwaysApply: true frontmatter" 'grep -q "^alwaysApply: true" "$RULE"'
# The rule is a GENERATED artifact whose head is cursor-rules/dispatch.head.md catted verbatim.
# tests/test_cursor_dispatch_rule.sh guards the head's CONTENT; nothing guarded that the generated
# file still carries it, so a head edit could be asserted true at the source and shipped mangled
# (0168 whole-branch review, IMPORTANT 3). Byte-compare the generated prefix against the source.
assert "0048 rule: generated file opens with dispatch.head.md byte-for-byte" \
  'diff -q <(head -n "$(wc -l < "$REPO/cursor-rules/dispatch.head.md")" "$RULE") "$REPO/cursor-rules/dispatch.head.md" >/dev/null'
assert "0048 rule: has the required dispatch pattern heading" 'grep -q "## Required dispatch pattern" "$RULE"'
assert "0048 rule: has a subsection for every built-in agent (16)" \
  '[ "$(grep -cE "^## docket-.* — dispatch only" "$RULE")" = "16" ]'
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
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRATCH/"
rm -f "$SCRATCH/cursor-rules/dispatch/docket-status.md"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48F" bash "$SCRATCH/sync-agents.sh" 2>&1 >/dev/null)"
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 auto-block: warns about the missing fragment" 'grep <<<"$gen_err" -qi "no dispatch fragment for docket-status"'
assert "0048 auto-block: still emits a docket-status subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
# 0135: the auto-block instructs by CAPABILITY, not by a tool name (ADR-0059 §2) — it must still
# name the agent it dispatches to, so this pins the dispatch sentence, not the old `subagent_type:`.
assert "0048 auto-block: subsection dispatches to the named subagent by capability" \
  'grep -q "Dispatch to the subagent .docket-status. using this mode.s subagent-launch mechanism" "$RULE"'
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
  'grep <<<"$chk_out" -q "advisory" && grep <<<"$chk_out" -q "docket-dispatch.mdc"'
# Delete the committed rule -> advisory (missing local file).
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )   # regenerate clean
rm -f "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: advisory-flags a missing committed rule (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 rule-check: missing-rule advisory names it" \
  'grep <<<"$chk_out" -q "advisory" && grep <<<"$chk_out" -q "docket-dispatch.mdc"'
rm -rf "$SBX" "$HROOT48C"

# 0048 Piece 3 — removing a built-in agent prunes its generated files (both layers) + rule subsection.
make_sandbox
HROOT48P="$(mktemp -d)"; mkdir -p "$HROOT48P/.cursor"   # present user-level cursor root
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Scratch clone we can mutate (remove a built-in agent + its fragment).
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRATCH/"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: adr generated before removal (per-repo)" '[ -f "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: adr generated before removal (user-level)" '[ -f "$HROOT48P/.cursor/agents/docket-adr.md" ]'
# Remove the built-in agent + its fragment, regenerate: the orphan must be pruned.
rm -f "$SCRATCH/agents/docket-adr.md" "$SCRATCH/cursor-rules/dispatch/docket-adr.md"
# change 0168: the sidecar's claude block is set-EQUAL to agents/docket-*.md in both
# directions, so retiring a built-in also retires its shipped default entry. Leaving it
# behind is a genuine sidecar defect and hd_validate refuses the whole run before any
# wrapper is written — which would make this leg fail for the wrong reason.
sed -i.bak '/^    adr:/d' "$SCRATCH/agents/harness-defaults.yml"; rm -f "$SCRATCH/agents/harness-defaults.yml.bak"
assert "0048 prune fixture: sidecar adr entry removed with the wrapper" \
  '[ "$(grep -c "^    adr:" "$SCRATCH/agents/harness-defaults.yml")" = "0" ]'
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
assert "0048 orphan-check: names the orphaned file" 'grep <<<"$chk_out" -q "advisory" && grep <<<"$chk_out" -q "docket-bogus.md"'
assert "0048 orphan-check: --check does NOT delete the orphan" '[ -f "$SBX/.claude/agents/docket-bogus.md" ]'
rm -rf "$SBX" "$HROOT48O"

# (a)+(b) harness override wins; field-level merge — model from cursor, effort inherited from default.
make_sandbox
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" >/dev/null )
assert "0046 (a): cursor model from cursor block" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
# 0135 retired the standalone `effort:` key from Cursor wrappers, but the MECHANISM this guards —
# field-level merge, where effort falls through to default: while model comes from the cursor: block
# — is still live. Narrowed to read the surviving carrier of that value.
assert "0046 (b): cursor effort inherited from default (now inside the model value)" \
  '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
assert "0046 (a): claude model falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0046 (a): claude effort from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# (c) arbitrary non-Claude id passes through verbatim; the two harness files now DIFFER (was byte-identical pre-0046).
assert "0046 (c): non-Claude id verbatim in .cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
# NOTE (0135): this next assert is now trivially true — every Cursor wrapper differs from its Claude
# counterpart by shape alone. Its discriminating power moved to "0135 (d): default-only => harness
# files DIFFER". Kept because it is cheap.
assert "0046: harness files differ when overridden" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTM"

# (d) default-only (no harness block) reaches EVERY listed harness. 0135 inverted the byte-identity
# half of this: a Cursor wrapper is no longer Claude-shaped, so the two files must now DIFFER.
make_sandbox
HROOTD0="$(mktemp -d)"; mkdir -p "$HROOTD0/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD0" bash "$SYNC" >/dev/null )
# 0135 inverted this: a Cursor wrapper is NO LONGER Claude-shaped, so default-only config must
# produce DIFFERENT files. The surviving property is that the default: block reaches both.
assert "0135 (d): default-only => harness files DIFFER (cursor has its own shape)" \
  '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
assert "0046 (d): default-only applies model to claude" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0135 (d): default-only applies model+effort to cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "sonnet[effort=high]" ]'
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
assert "critic: shipped model is claude-opus-5" '[ "$(hd_field "$HD" claude auto-groom-critic model)" = "claude-opus-5" ]'
assert "critic: shipped effort is medium" '[ "$(hd_field "$HD" claude auto-groom-critic effort)" = "medium" ]'
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
  'grep <<<"$chk_out" -q "advisory" && grep <<<"$chk_out" -q "docket-auto-groom-critic.md"'
rm -rf "$SBX" "$HROOT2"

# ---- Task 1c: the two finalize-gate wrappers (wrap NO skill) ----------------
# docket-rebase-resolver (①) and docket-integration-repair (②): like the critic,
# they inject ONLY docket-convention, pin opus/medium, and carry abort-and-report.
for nw in docket-rebase-resolver docket-integration-repair; do
  f="$AGENTS/$nw.md"
  assert "$nw: wrapper exists" '[ -f "$f" ]'
  assert "$nw: name matches file" '[ "$(fm "$f" name)" = "$nw" ]'
  assert "$nw: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$nw: shipped model is claude-opus-5" '[ "$(hd_field "$HD" claude "'"${nw#docket-}"'" model)" = "claude-opus-5" ]'
  assert "$nw: shipped effort is medium" '[ "$(hd_field "$HD" claude "'"${nw#docket-}"'" effort)" = "medium" ]'
  assert "$nw: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  # Isolation: the skills: line wraps NO docket skill (only the convention).
  nw_skills_line="$(grep -E "^skills:" "$f" || true)"
  assert "$nw: skills EXCLUDES any wrapped docket skill" \
    '! grep -Eq "docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change)" <<<"$nw_skills_line"'
  assert "$nw: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# ---- the brainstorm consultant wrapper (wraps NO skill AND injects NO convention) ----
CONSULT="$AGENTS/docket-brainstorm-consultant.md"
assert "consultant: wrapper exists" '[ -f "$CONSULT" ]'
assert "consultant: name matches file" '[ "$(fm "$CONSULT" name)" = "docket-brainstorm-consultant" ]'
assert "consultant: has a description" '[ -n "$(fm "$CONSULT" description)" ]'
assert "consultant: shipped model is claude-opus-5" '[ "$(hd_field "$HD" claude brainstorm-consultant model)" = "claude-opus-5" ]'
assert "consultant: shipped effort is medium" '[ "$(hd_field "$HD" claude brainstorm-consultant effort)" = "medium" ]'
# Deliberate ADR-0009 deviation: injects NEITHER a wrapped skill NOR docket-convention.
assert "consultant: injects NO docket-convention" '! grep -Eq "^skills:.*docket-convention" "$CONSULT"'
assert "consultant: injects NO wrapped docket skill" '! grep -Eq "^skills:.*docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change|brainstorm)\b" "$CONSULT"'
assert "consultant: body names the spec deliverable + assumptions requirement" 'grep -qi "spec" "$CONSULT" && grep -qi "assumption" "$CONSULT"'

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
  'grep <<<"$chk_out" -q "advisory" && grep <<<"$chk_out" -q "docket-rebase-resolver.md"'
rm -rf "$SBX" "$HROOT3"

exit $fail
