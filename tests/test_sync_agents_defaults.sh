#!/usr/bin/env bash
# tests/test_sync_agents_defaults.sh — shipped-sidecar layering (changes 0051, 0168) (shard of test_sync_agents.sh,
# change 0227). Run: bash tests/test_sync_agents_defaults.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

# Path constants the pre-split file set earlier in what is now
# tests/test_sync_agents_drift_docs.sh; copied verbatim so this shard stands alone.
READMEF="$REPO/README.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
AGL="$REPO/skills/docket-convention/references/agent-layer.md"

# ---- Change 0051 doc sentinels ----
assert "0051 doc: README documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$READMEF"'
assert "0051 doc: README states generated agents are machine-local, never committed" \
  'grep -qiE "machine-local" "$READMEF" && grep -qiE "never committed" "$READMEF"'
assert "0057 doc: README documents the managed docket .gitignore block" 'grep -qF "# docket:start" "$READMEF" || grep -qE "managed .docket. block" "$READMEF"'
assert "0057 doc: README no longer names the legacy docket:generated block" '! grep -qF "docket:generated" "$READMEF"'
assert "0051 doc: README documents the migration (git rm --cached / one commit)" 'grep -qiE "migrat" "$READMEF" && grep -qF -e "--cached" "$READMEF"'
assert "0051 doc: convention documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$CONV"'
assert "0051 doc: agent-layer ref states all-local generation (gitignored, never committed)" 'grep -qiE "gitignored, never committed|machine-local, never committed" "$AGL"'
assert "0057 doc: agent-layer ref documents the managed docket block (new marker)" 'grep -qF "# docket:start" "$AGL" || grep -qi "managed docket .gitignore block" "$AGL"'
assert "0057 doc: agent-layer ref documents the check via the managed block" 'grep -qi "advisory" "$AGL"'
assert "0057 doc: agent-layer ref no longer names docket:generated" '! grep -qF "docket:generated" "$AGL"'
# Change 0101: the agents: documentation moved from this repo's .docket.yml (now values-only)
# to .docket.example.yml, the canonical reference. Both asserts follow it.
assert "0051 doc: example agents comment states machine-local generation" 'grep -qi "machine-local" "$REPO/.docket.example.yml"'
assert "0051 doc: example drops the stale agents.yaml global reference" '! grep -q "agents.yaml" "$REPO/.docket.example.yml"'

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
assert "0051 4L: the 0050 SHADOWED stopgap warning is gone" '! grep <<<"$sw_err" -q "SHADOWED"'
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
assert "0051 malformed local: warns and names the file" 'grep <<<"$mf_err" -qi "docket.local.yml"'
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

# ---- change 0168: the shipped sidecar is the lowest layer -------------------
# Outcome asserts only: they pin WHAT resolves, not WHERE it came from. While the
# sources still carry model:/effort: these are green either way — the mechanism
# (the resolver actually reading agents/harness-defaults.yml) is proved separately
# by pointing the resolver at a sentinel sidecar, and permanently by Task 4's
# deletion of the source frontmatter.
make_sandbox
HROOT168="$(mktemp -d)"; mkdir -p "$HROOT168/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    adr: { effort: high }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168" bash "$SYNC" >/dev/null 2>&1 )
A="$SBX/.claude/agents/docket-adr.md"
assert "0168: unconfigured agent takes the shipped claude model" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
assert "0168: a user effort override beats the shipped effort" '[ "$(fm "$A" effort)" = "high" ]'
assert "0168: the un-overridden field still comes from the sidecar" \
  '[ "$(fm "$A" model)" = "claude-opus-5" ]'
rm -rf "$SBX" "$HROOT168"

# 0168 fail-before-write gate: an invalid sidecar aborts the run with a named diagnostic
# and leaves ZERO wrappers behind — never a half-regenerated agent directory.
make_sandbox
HROOT168B="$(mktemp -d)"; mkdir -p "$HROOT168B/.claude"
SCR168="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCR168/"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf '    phantom-not-a-wrapper: { model: x, effort: low }\n' >> "$SCR168/agents/harness-defaults.yml"
hd_rc=0
hd_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168B" bash "$SCR168/sync-agents.sh" 2>&1 >/dev/null)" || hd_rc=$?
assert "0168 gate: invalid sidecar fails the run (rc!=0)"  '[ "$hd_rc" != "0" ]'
assert "0168 gate: the diagnostic names harness-defaults"  'grep <<<"$hd_err" -q "harness-defaults"'
assert "0168 gate: no per-repo wrapper was written"        '[ "$(find "$SBX" -name "docket-*.md" -path "*/agents/*" | wc -l | tr -d " ")" = "0" ]'
assert "0168 gate: no user-level wrapper was written"      '[ "$(find "$HROOT168B" -name "docket-*.md" | wc -l | tr -d " ")" = "0" ]'
rm -rf "$SBX" "$HROOT168B" "$SCR168"

# ============================================================================
# Change 0051/0057 — managed .gitignore block (# docket:start/end; mechanics now
# live in scripts/lib/docket-gitignore-block.sh, sourced by sync-agents.sh)
# ============================================================================

# (gi-a) opted-in repo: block created (file didn't exist), loud "commit" notice,
# patterns strictly docket-scoped, emitted from the harness table (all 6 tokens).
make_sandbox
HROOTGA="$(mktemp -d)"; mkdir -p "$HROOTGA/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
gi_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
GI="$SBX/.gitignore"
assert "0051 gi: .gitignore created with the managed block" 'grep -q "^# docket:start" "$GI" && grep -q "^# docket:end$" "$GI"'
assert "0051 gi: block ignores .docket.local.yml"            'grep -q "^\.docket\.local\.yml$" "$GI"'
assert "0051 gi: block ignores claude agents pattern"        'grep -q "^\.claude/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores cursor agents pattern"        'grep -q "^\.cursor/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores the cursor dispatch rule"     'grep -q "^\.cursor/rules/docket-dispatch\.mdc$" "$GI"'
assert "0051 gi: loud commit-this notice"                    'grep <<<"$gi_err" -qi "commit"'
assert "0051 gi: every block line is docket-scoped (starts with . or #)" \
  '! awk "/# docket:start/,/# docket:end/" "$GI" | grep >/dev/null -vE "^(#|\.)"'

# (gi-b) idempotent: second run leaves .gitignore byte-identical and prints no notice.
gi_before="$(cat "$GI")"
gi_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 gi: second run byte-identical"    '[ "$gi_before" = "$(cat "$GI")" ]'
assert "0051 gi: second run no UPDATED notice" '! grep <<<"$gi_err2" -q "managed block"'

# (gi-c) hand-edit inside the block repaired; content OUTSIDE the markers preserved.
printf 'my-own-ignore/\n%s\n' "$(cat "$GI")" > "$GI"          # user content above the block
sed -i.bak '/docket-dispatch/d' "$GI"; rm -f "$GI.bak"        # vandalize the block
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: hand-edited block repaired"   'grep -q "docket-dispatch" "$GI"'
assert "0051 gi: user content preserved"       'grep -q "^my-own-ignore/$" "$GI"'
assert "0051 gi: exactly one block after repair" '[ "$(grep -c "^# docket:start" "$GI")" = "1" ]'
rm -rf "$SBX" "$HROOTGA"

# (gi-d) tracking-only repo WITH a .docket.local.yml that has NO opt-in keys: the block
# is still written (the local file itself must never be committable); zero agent files.
make_sandbox
HROOTGD="$(mktemp -d)"; mkdir -p "$HROOTGD/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf 'finalize:\n  gate: off\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGD" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: local-file-present repo gets the block"  'grep -q "^# docket:start" "$SBX/.gitignore"'
assert "0051 gi: but still generates zero agent files"    '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGD"

# (gi-e) repo with NEITHER signal: .gitignore never touched/created (LEARNINGS #48 posture).
make_sandbox
HROOTGE="$(mktemp -d)"; mkdir -p "$HROOTGE/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGE" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
rm -rf "$SBX" "$HROOTGE"

# (gi-core) the block now carries the three core docket-owned entries (change 0057).
make_sandbox
HROOTGC="$(mktemp -d)"; mkdir -p "$HROOTGC/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGC" bash "$SYNC" >/dev/null 2>&1 )
GI="$SBX/.gitignore"
assert "0057 gi: block carries .docket/"              'grep -qxF ".docket/" "$GI"'
assert "0057 gi: block carries .worktrees/"           'grep -qxF ".worktrees/" "$GI"'
assert "0057 gi: block carries settings.local.json"   'grep -qxF ".claude/settings.local.json" "$GI"'
assert "0057 gi: new start marker, no legacy marker"  'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$GI" && ! grep -qF "docket:generated" "$GI"'
rm -rf "$SBX" "$HROOTGC"

# (gi-widen+) widened trigger POSITIVE: a tracking-only repo (NOT opted in, no local file) that
# HAS a local docket branch heals the block (the bootstrap guard's DOCKET probe).
mkgitrepo
HROOTGW="$(mktemp -d)"; mkdir -p "$HROOTGW/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"        # tracking-only, not opted in
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
git -C "$SBX" branch docket                                    # DOCKET signal present
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGW" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: docket-branch repo heals the block"  'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore"'
assert "0057 gi: but still generates zero agent files" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGW"

# (gi-widen-) widened trigger NEGATIVE (the 0048 regression): a repo with NO docket signal
# (no opt-in, no .docket.local.yml, no docket branch, no existing block) is untouched.
mkgitrepo
HROOTGN="$(mktemp -d)"; mkdir -p "$HROOTGN/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGN" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGN" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0057 gi: no-signal repo --check stays a no-op (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTGN"

# (gi-heal-present) heal-if-present: a repo carrying only a legacy block (no other signal) is
# UPGRADED to the new block.
mkgitrepo
HROOTGH="$(mktemp -d)"; mkdir -p "$HROOTGH/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf '# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket.local.yml\n# docket:generated:end\n' > "$SBX/.gitignore"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGH" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: legacy-only repo upgraded to new block" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore" && ! grep -qF "docket:generated" "$SBX/.gitignore"'
rm -rf "$SBX" "$HROOTGH"

# (gi-f) UNTERMINATED block (start marker, no end): refuse to rewrite, warn, preserve
# every byte — user content after the dangling marker must survive.
make_sandbox
HROOTGF="$(mktemp -d)"; mkdir -p "$HROOTGF/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf '# docket:start (managed by docket — do not hand-edit)\n.docket.local.yml\nnode_modules/\n' > "$SBX/.gitignore"
gi_before="$(cat "$SBX/.gitignore")"
gf_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGF" bash "$SYNC" 2>&1 >/dev/null)"; gf_rc=$?
assert "0051 gi-f: unterminated block run still succeeds (rc=0)" '[ "$gf_rc" = "0" ]'
assert "0051 gi-f: warns the block is corrupt/unterminated" 'grep <<<"$gf_err" -qi "untermin\|corrupt"'
assert "0051 gi-f: file left byte-identical (user content preserved)" '[ "$gi_before" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX" "$HROOTGF"

# ============================================================================
# Change 0051 — migration (0048-era tracked wrappers) + --check three legs
# ============================================================================

# (mkgitrepo defined earlier, alongside make_sandbox, so the 0057 widened-trigger tests above
# can use it too.)

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
assert "0051 mig: announces the migration"                 'grep <<<"$mig_err" -qi "migrat"'
assert "0051 mig: prints git rm --cached instructions"     'grep <<<"$mig_err" -q -e "git rm" '
assert "0051 mig: gitignore block written"                 'grep -q "^# docket:start" "$SBX/.gitignore"'
assert "0051 mig: local files regenerated (fresh content)" 'grep -q "^model: sonnet" "$SBX/.claude/agents/docket-status.md"'
assert "0051 mig: full local set regenerated"              '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "17" ]'
# perform the printed migration commit; second run must NOT re-announce
( cd "$SBX" && git rm -r -q --cached '.claude/agents/docket-*.md' '.cursor/agents/docket-*.md' '.cursor/rules/docket-dispatch.mdc' && git add .gitignore && git commit -q -m "docket: agent files go machine-local" )
mig_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 mig: idempotent — post-commit run is silent about migration" '! grep <<<"$mig_err2" -qi "migrat"'
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
assert "0051 mig-b: remedy omits git add .gitignore"          'grep <<<"$migb_err" -e "git rm" | grep >/dev/null -v "git add .gitignore"'
assert "0051 mig-b: no .gitignore was created (not wanted)"   '[ ! -e "$SBX/.gitignore" ]'
# the printed remedy must actually run: extract and eval it, then leg (b) goes green.
remedy="$(printf '%s\n' "$migb_err" | sed -n 's/^sync-agents:[[:space:]]*\(git rm .*\)$/\1/p' | sed -n 1p)"
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
assert "0051 chk-a: names the gitignore block"           'grep <<<"$chk_out" -qi "gitignore"'
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
assert "0051 chk-b: names a tracked path"                          'grep <<<"$chk_out" -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCB"

# (chk-c) leg (c): content staleness is ADVISORY — rc stays 0, output says advisory.
make_sandbox
HROOTCC="$(mktemp -d)"; mkdir -p "$HROOTCC/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-c: content drift is advisory (rc=0)"  '[ "$chk_rc" = "0" ]'
assert "0051 chk-c: advisory names the drifted file"   'grep <<<"$chk_out" -q "advisory" && grep <<<"$chk_out" -q "docket-status.md"'
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
