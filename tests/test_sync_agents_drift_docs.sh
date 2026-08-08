#!/usr/bin/env bash
# tests/test_sync_agents_drift_docs.sh — --check drift gate + doc/README sentinels (shard of test_sync_agents.sh,
# change 0227). Run: bash tests/test_sync_agents_drift_docs.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

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
assert "0048 opt-in: agent_harnesses-only generates full set for cursor" '[ "$(find "$SBX/.cursor/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "0048 opt-in: agent_harnesses-only generates full set for claude" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "0048 opt-in: agent_harnesses-only generates the cursor dispatch rule" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 opt-in: agent_harnesses-only wrappers carry shipped default (no overrides)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX" "$HROOTAH"

# 0048: a repo with NO .docket.yml at all has nothing to check -> passes.
make_sandbox
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes when no .docket.yml (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"

# ---- Task 5: docket-convention documents the agent layer -------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "agent-layer reference exists" '[ -f "$AGL" ]'
assert "convention points at the agent-layer reference (blocking)" 'grep -qF "references/agent-layer.md" "$CONV"'
assert "convention documents the agents: config block" 'grep -q "agents:" "$CONV"'
assert "convention names the generator sync-agents.sh" 'grep -q "sync-agents.sh" "$CONV"'
assert "convention states the precedence" 'grep -qi "repo-local > repo-committed > global > built-in" "$CONV"'
assert "agent-layer ref states auto => omit effort" 'grep -qi "auto" "$AGL" && grep -qi "omit" "$AGL"'
assert "convention states abort-and-report for autonomous subagents" 'grep -qi "abort-and-report" "$CONV"'
assert "convention points at composition (0017)" 'grep -q "0017" "$CONV"'
# Non-vacuous guard: the agent section must be a distinct heading, not an incidental word.
assert "convention has an agent-layer section heading" 'grep -qiE "^#+ .*(agent layer|model/effort|subagent)" "$CONV"'

# 0046: convention documents the harness-first agents: shape (default: + harness keys, field-level fallback).
assert "0046 doc: agent-layer ref names the reserved default: key" 'within "$AGL" "agents:" "default:" 400'
assert "0046 doc: agent-layer ref shows a per-harness key example (cursor)" 'within "$AGL" "agents:" "cursor:" 600'
assert "0046 doc: agent-layer ref states field-level fallback H -> default -> built-in" 'grep -qiE "harness.*default.*built-in|<harness>.*default.*built-in" "$AGL"'
# Change 0168 reworded this line: the shipped layer is harness-indexed, so a non-claude harness no
# longer falls back to a claude ID — it warns and ships unpinned. Anchored on the verbatim clause.
assert "0046 doc: agent-layer ref notes non-Claude fallback warning" \
  'grep -qiE "non-.?claude.? harness with no harness-specific model gets a non-fatal warning" "$AGL"'

# 0048 doc: convention states per-repo generates the full built-in set (config override-only)
# and that cursor gets a generated docket-dispatch.mdc rule.
assert "0048 doc: agent-layer ref says per-repo writes the full built-in set" 'grep -qiE "full (built-in )?(agent )?set" "$AGL"'
assert "0048 doc: agent-layer ref says the agents: block is override-only" 'grep -qi "override-only" "$AGL"'
assert "0048 doc: agent-layer ref names the cursor dispatch rule" 'grep -q "docket-dispatch.mdc" "$AGL"'

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
# 0135 + 0168: bracket-encoded when an effort resolves. This fixture pins no effort, and the cursor
# harness has no shipped `status` entry in agents/harness-defaults.yml, so the model is emitted bare
# rather than picking up docket-status's Claude built-in effort (see "global cursor block wins").
assert "0046 fanout: cursor carries its override model" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
# NOTE (0135): trivially true now (see the note on "0046: harness files differ when overridden").
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
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "0045 doc: convention names agent_harnesses" 'grep -q "agent_harnesses" "$CONV"'
assert "0045 doc: convention states default [claude]" 'grep -qE "agent_harnesses.*\[claude\]|default.*\[claude\]" "$CONV"'
assert "0045 doc: agent-layer ref states harness-neutral direct model IDs" 'grep -qiE "harness-neutral|direct model id" "$AGL"'
assert "0045 doc: agent-layer ref notes passthrough enables non-Claude harnesses" 'grep -qi "passthrough" "$AGL"'
assert "0045 doc: agent-layer ref points at ADR-0015 near agent_harnesses" 'within "$AGL" "agent_harnesses" "ADR-0015" 500 || within "$AGL" "ADR-0015" "agent_harnesses" 500'

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
# (those are config-overridable; the shipped defaults live only in agents/harness-defaults.yml
# since change 0168 — the wrapper sources are behavior-only templates). LEARNINGS #17.
assert "0047 §agent-cfg: does NOT hardcode a model/effort literal (references the source instead)" \
  '! grep -qiE "\b(opus|sonnet|haiku|fable)\b.*\b(xhigh|high|medium|low)\b|model:[[:space:]]*(opus|sonnet|haiku|claude-)" <<<"$sec"'

# ============================================================================
# Change 0046 — per-harness values: diagnostics
# ============================================================================

# (h) Non-Claude fallback warning: a cursor file whose model fell through to agents.default warns;
#     suppressed for claude, and suppressed when cursor supplies its own model.
# Change 0168 re-worded the diagnostic: the source frontmatter is no longer a default store, so
# the fallthrough can only come from agents.default (or from nothing at all, which is a distinct
# "generated unpinned" warning). The property guarded is unchanged — this fixture pins a Claude
# ID under agents.default, which outranks the sidecar, so the cursor wrapper really is emitted with
# a foreign ID and cursor must be told. (Cursor now ships a complete sidecar block; what makes the
# warning correct here is that agents.default WON the field, not that the pair is uncovered.)
make_sandbox
HROOTW="$(mktemp -d)"; mkdir -p "$HROOTW/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h): generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (h): warns cursor model came from agents.default" 'grep -qi "cursor/docket-status" <<<"$gen_err" && grep -qF "came from agents.default" <<<"$gen_err"'
assert "0046 (h): does NOT warn for the claude harness" '! printf "%s" "$gen_err" | grep -qiE "claude/docket-status|WARN claude"'
rm -rf "$SBX" "$HROOTW"

# (h') warning suppressed when the cursor block supplies the model.
make_sandbox
HROOTW2="$(mktemp -d)"; mkdir -p "$HROOTW2/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW2" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h'): no fallback warning when cursor supplies model" '! grep -qiE "cursor/docket-status: (no harness-specific model|model .* came from agents\.default)" <<<"$gen_err"'
rm -rf "$SBX" "$HROOTW2"

# (f) Legacy bare-agent-key block (pre-0046 flat shape) => warned + ignored; --check flags it as drift.
make_sandbox
HROOTL="$(mktemp -d)"; mkdir -p "$HROOTL/.claude"
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"   # bare agent key, no default:/harness
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (f): legacy shape not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (f): warns about the legacy bare agent key" 'printf "%s" "$gen_err" | grep -qi "legacy" && printf "%s" "$gen_err" | grep -q "status"'
assert "0046 (f): legacy status NOT applied (no project file / shipped only)" '[ ! -f "$SBX/.claude/agents/docket-status.md" ] || [ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
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
assert "0050 no-dual-read: .migrated is not read (shipped model)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
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
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "0050 doc: convention names config.yml" 'grep -qF "config.yml" "$CONV"'
assert "0050 doc: convention states the coordination-key fence" 'grep -qi "fence" "$CONV" && grep -qi "per-repo-only" "$CONV"'
assert "0050 doc: agent-layer ref Agent layer global row points at config.yml agents: block" \
  'grep -qE "^\| Global \|.*config\.yml" "$AGL"'

# ---- 0255: the unquoted / no-`#` rule is stated at the five points of use --------------------
# The finding this guards: before change 0255 the rule appeared NOWHERE a user reads before
# tripping the gate — the gate self-described only once tripped. Each assert is scoped to the
# `agents:` example block in its file, so the sentence drifting into unrelated prose reddens.
rule_re='unquoted and space-free'
# `unquoted and space-free` predates 0255 (it is the gate's own remedy text), so it alone would
# assert only the half of the rule that already existed. The NEW clause is the `#` prohibition, so
# each slice is checked for a second literal too — otherwise deleting the `#` clause from all five
# sites leaves this guard green (detects removal of the sentence, not replacement of it).
flow_re='inside the `{…}` flow map'
# Hazard being defended against: the clause wraps mid-sentence in README.md and .docket.example.yml,
# so a multi-word grep over the raw slice would double as a line-wrap guard and redden on a pure
# re-flow. Every slice is therefore normalized (newlines -> spaces, whitespace runs collapsed)
# before matching, so wrapping is invisible to both literals. Matching is /usr/bin/grep -F: the
# `…` is multi-byte UTF-8 and must be compared literally, never as a regex.
norm() { tr '\n' ' ' | tr -s '[:space:]' ' '; }

# Anchored on the "— optional; applies to every repo" variant: the bare path also heads an earlier
# change_types example, and a range starting there would span both blocks.
readme_global="$(sed -n '/^# ~\/\.config\/docket\/config\.yml — optional/,/^```$/p' "$READMEF" | norm)"
assert "0255 docs: README global config.yml example states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$readme_global"'
assert "0255 docs: README global config.yml example states the no-\`#\` clause" \
  '/usr/bin/grep -qF "$flow_re" <<<"$readme_global"'

readme_local="$(sed -n '/^# <repo>\/\.docket\.local\.yml/,/^```$/p' "$READMEF" | norm)"
assert "0255 docs: README .docket.local.yml example states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$readme_local"'
assert "0255 docs: README .docket.local.yml example states the no-\`#\` clause" \
  '/usr/bin/grep -qF "$flow_re" <<<"$readme_local"'

skill_agents_line="$(/usr/bin/grep -n '^agents:' "$CONV" | norm || true)"
assert "0255 docs: convention SKILL.md agents: schema line states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$skill_agents_line"'
assert "0255 docs: convention SKILL.md agents: schema line states the no-\`#\` clause" \
  '/usr/bin/grep -qF "$flow_re" <<<"$skill_agents_line"'

layer_example="$(sed -n '/^agents:  */,/^```$/p' "$AGL" | norm)"
assert "0255 docs: agent-layer.md example block states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$layer_example"'
assert "0255 docs: agent-layer.md example block states the no-\`#\` clause" \
  '/usr/bin/grep -qF "$flow_re" <<<"$layer_example"'

example_intro="$(sed -n '/^# agents — per-skill subagent model\/effort/,/^# agents:$/p' "$REPO/.docket.example.yml" | norm)"
assert "0255 docs: .docket.example.yml agents: intro states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$example_intro"'
assert "0255 docs: .docket.example.yml agents: intro states the no-\`#\` clause" \
  '/usr/bin/grep -qF "$flow_re" <<<"$example_intro"'

# Non-vacuity: every slice above must be non-empty, or a renamed heading turns all five asserts
# into vacuous greens against nothing.
assert "0255 docs: every doc slice is non-empty" \
  '[ -n "$readme_global" ] && [ -n "$readme_local" ] && [ -n "$skill_agents_line" ] &&
   [ -n "$layer_example" ] && [ -n "$example_intro" ]'

exit $fail
