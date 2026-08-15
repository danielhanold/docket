#!/usr/bin/env bash
# tests/test_sync_agents_runners_pins.sh — the resolved model/effort PIN INJECTION half of the
# runner suite (sharded out of tests/test_sync_agents_runners.sh at change 0324, when the 17-agent
# matrix pushed the single file to ~96s against the hard 60s ceiling and the table's remedy is a
# shard, never a bigger number). Covers emit()'s single model:/effort: insertion, `model: inherit`
# as a Claude value, the shipped-default vs user-configured provenance boundary, the two headline
# bare-opt-in properties (change 0168), and provider-prefixed model IDs rounding through whole
# (change 0173). Shim rendering + shim-pin config stayed in tests/test_sync_agents_runners.sh; the
# generation gates and atomicity are in tests/test_sync_agents_runners_gates.sh.
# Run: bash tests/test_sync_agents_runners_pins.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

# ---- change 0168: emit() inserts exactly one model:/effort: line -------------
# emit() is strip-then-insert now: it drops any model:/effort: the SOURCE still carries and injects
# the resolved pair before the closing fence. The failure mode that rewrite introduces is a
# DUPLICATED key (insert without strip), which no earlier assert could see because substitution
# cannot duplicate. Counted inside the FIRST frontmatter block — a whole-file count would also see
# body prose (AGENTS.md: anchor a frontmatter read to the first ---…--- block) — plus a whole-file
# count, so an insertion that lands outside the block is caught too.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
for w in docket-status docket-implement-next docket-build-max; do
  G="$SBX/.claude/agents/$w.md"
  assert "0168: $w emits exactly one model: line in the frontmatter" \
    '[ "$(fm_key_count "$G" model)" = "1" ]'
  assert "0168: $w emits exactly one effort: line in the frontmatter" \
    '[ "$(fm_key_count "$G" effort)" = "1" ]'
  assert "0168: $w emits exactly one model: line in the whole file" \
    '[ "$(grep -c "^model:" "$G")" = "1" ]'
  assert "0168: $w emits exactly one effort: line in the whole file" \
    '[ "$(grep -c "^effort:" "$G")" = "1" ]'
done
rm -rf "$SBX"

# ---- `model: inherit` is a CLAUDE VALUE, not a cross-harness sentinel -------
# 0168 whole-branch review, IMPORTANT 2. `inherit` is a documented Claude Code frontmatter value
# meaning "run this subagent on the parent conversation's model"; Claude Code reads it and acts on
# it. It is NOT a docket sentinel there. Cursor and Codex have no such value, so their emitters
# (both pre-0168) normalize it to "emit no pin" — the harness then applies its own default.
# Change 0168's rewritten emit() briefly folded the Cursor sentinel into the SHARED emitter, which
# silently turned `model: inherit` into NO model: line on Claude — a different runtime meaning
# (parent's model vs. Claude Code's own subagent default) on the one harness the change promised
# to leave byte-for-byte alone. These asserts pin the split: verbatim on claude, dropped elsewhere.
make_sandbox
mkdir -p "$SBX/.cursor" "$SBX/.codex"
HROOTINH="$(mktemp -d)"; mkdir -p "$HROOTINH/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude, cursor, codex]
agents:
  default:
    status: { model: inherit, effort: medium }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTINH" bash "$SYNC" >/dev/null 2>&1 )
assert "inherit: claude emits it VERBATIM (Claude Code's 'use the parent's model')" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "inherit" ]'
assert "inherit: claude still emits the configured effort alongside it" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "medium" ]'
assert "inherit: cursor emits NO model: line (no such Cursor value)" \
  '! grep -q "^model:" "$SBX/.cursor/agents/docket-status.md"'
assert "inherit: codex emits NO model = line (no such Codex value)" \
  '! grep -q "^model = " "$SBX/.codex/agents/docket-status.toml"'
rm -rf "$SBX" "$HROOTINH"

# ---- change 0168: a shipped default never becomes a child-runner flag -------
# The provenance boundary. `runner:` delegates this agent to a DIFFERENT harness's CLI, so the
# baked --model/--effort flags are read by that child, not by Claude. A shipped
# agents/harness-defaults.yml value is a CLAUDE default; baking it into a Codex dispatch sends a
# Claude model ID to a Codex child. Only a USER-configured value may cross that boundary. What the
# wrapper's own frontmatter carries is a separate question, settled by the change-0269 note below.
#
# CHANGE 0205 NARROWED THIS TEST'S REACH, and the narrowing is a strengthening, not lost coverage.
# The fixture used to configure a bare `runner: codex` with no model; that is now a generation-time
# ERROR (the required-model rule), which makes the MODEL half of the provenance leak structurally
# impossible rather than merely guarded — a shipped model can no longer be the resolved-and-baked
# value under a runner, because a user model is mandatory. What remains reachable, and is therefore
# what this block now pins, is the EFFORT half: effort stays optional, so a user who configures only
# a model must still not have the shipped effort forwarded to the child.
mkgitrepo
mkdir -p "$SBX/.claude"
HROOT168F="$(mktemp -d)"; mkdir -p "$HROOT168F/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: user-picked-id }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
S="$SBX/.claude/agents/docket-status.md"
# Fixture sanity FIRST: without a real shim the negative asserts below are vacuous.
assert "0168: a user-model runner config emits a shim" 'grep -qF "docket.sh runner-dispatch" "$S"'
assert "0168: the shim names the runner" 'grep -qF -- "--runner codex" "$S"'
assert "0168: the USER model is baked (the provenance rule's positive half)" \
  'grep -qF -- "--model user-picked-id" "$S"'
assert "0168: no user effort configured => NO --effort flag baked" '! grep -qF -- "--effort" "$S"'
# 0169 (re-pointed by 0205): the sidecar supplies an EFFORT for this very agent on both the parent
# and the child harness, so the negative assert above is non-vacuous in the direction that matters
# — a shipped effort could be baked into the flags if provenance were ignored. Pin the fixture's
# premise so a future emptying of either block cannot quietly re-vacuum it.
assert "0169: the claude sidecar really does supply an effort for this agent (the guard above is not vacuous)" \
  '[ -n "$(hd_field "$HD" claude status effort)" ]'
assert "0169: the codex sidecar really does supply an effort for this agent" \
  '[ -n "$(hd_field "$HD" codex status effort)" ]'
assert "0169: and neither shipped EFFORT leaked into the runner flags" \
  '! grep -qF -- "--effort $(hd_field "$HD" claude status effort)" "$S" && ! grep -qF -- "--effort $(hd_field "$HD" codex status effort)" "$S"'
assert "0169: nor did the shipped CODEX model" \
  '! grep -qF -- "$(hd_field "$HD" codex status model)" "$S"'
# change 0269 replaces 0168's two "the shim frontmatter carries the resolved native pin
# (bookkeeping)" asserts — the effort half and the model half of one claim whose premise was false.
# The shim runs in the PARENT harness, so its frontmatter is the parent-side relay's own pin: the
# runners.<name> knobs, here unset and therefore at their defaults. The resolved native values
# belong to the CHILD and reach it only through the baked flags asserted above.
assert "0269: an unconfigured runner shim's frontmatter effort is the shim default, not the resolved native effort" \
  '[ "$(fm "$S" effort)" = "low" ] && [ "$(fm "$S" effort)" != "$(hd_field "$HD" claude status effort)" ]'
assert "0269: an unconfigured runner shim's frontmatter model is the shim default, not the child's resolved model" \
  '[ "$(fm "$S" model)" = "inherit" ] && [ "$(fm "$S" model)" != "user-picked-id" ]'

# A user-configured pair is policy, not a shipped guess: it still passes through to the child.
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: gpt-5.5, effort: high }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
assert "0168: an explicit override still passes through to the child" \
  'grep -qF -- "--model gpt-5.5" "$S" && grep -qF -- "--effort high" "$S"'

# The two fields split independently: a user model with no user effort bakes --model only,
# even though the sidecar supplies an effort for this agent.
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: gpt-5.5 }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
assert "0168: user model alone bakes --model but not the shipped --effort" \
  'grep -qF -- "--model gpt-5.5" "$S" && ! grep -qF -- "--effort" "$S"'
rm -rf "$SBX" "$HROOT168F"

# The fallback warning's premise moved with the default store. A non-claude harness/agent pair the
# sidecar SUPPLIES is a deliberate shipped default, not a leak, so it must stay silent; a pair it
# does not cover generates unpinned rather than inheriting a foreign ID, and says so.
#
# The unpinned leg can no longer be driven by any SHIPPED harness — claude, cursor, and codex all
# carry complete blocks since change 0169. What the rule guards is still reachable (it is what a
# newly-added, not-yet-mapped harness hits), so the fixture reconstructs that state in a throwaway
# copy of the repo rather than asserting a condition the shipped tree can no longer reach: drop
# codex from the copy's shipped list AND delete its block, which is exactly "known but unshipped".
make_sandbox
HROOT168W="$(mktemp -d)"; mkdir -p "$HROOT168W/.claude"
SCRW="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRW/"
# Anchored on the token being removed, not on its position in the list: `codex` stopped being the
# final token when change 0192 appended `opencode`, and a position-anchored pattern would silently
# stop matching (the fixture sanity asserts just below are what catch that).
sed -i.bak 's/^HD_SHIPPED_HARNESSES="\(.*\)codex\(.*\)"$/HD_SHIPPED_HARNESSES="\1\2"/' "$SCRW/scripts/lib/harness-defaults.sh"
# Normalize the doubled/trailing space the substitution above can leave. Address-scoped to the
# assignment line: an unscoped `s/  */ /g` would also collapse the two literal spaces inside
# _hd_block's `"^  "h` indent regexes and silently break the whole reader in the copy.
sed -i.bak2 '/^HD_SHIPPED_HARNESSES=/{ s/  */ /g; s/= /=/; s/" /"/; s/ "$/"/; }' "$SCRW/scripts/lib/harness-defaults.sh"
awk '/^  codex:[[:space:]]*$/{skip=1; next}
     skip && /^  [A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/{skip=0}
     !skip' "$SCRW/agents/harness-defaults.yml" > "$SCRW/hd.tmp" && mv "$SCRW/hd.tmp" "$SCRW/agents/harness-defaults.yml"
# Fixture sanity FIRST: if either strip silently missed, every assert below is vacuous — the copy
# would still ship codex and simply never warn.
assert "0169 fixture: the copy no longer lists codex as shipped" \
  '! grep -qE "^HD_SHIPPED_HARNESSES=.*codex" "$SCRW/scripts/lib/harness-defaults.sh"'
assert "0169 fixture: the copy has no codex block" \
  '[ -z "$(hd_agents "$SCRW/agents/harness-defaults.yml" codex)" ]'
assert "0169 fixture: the copy still ships a complete cursor block (only codex was stripped)" \
  '[ "$(hd_agents "$SCRW/agents/harness-defaults.yml" cursor | grep -c .)" = "17" ]'
printf 'agent_harnesses: [claude, cursor, codex]\n' > "$SBX/.docket.yml"
w168="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168W" bash "$SCRW/sync-agents.sh" 2>&1 >/dev/null)"
assert "0168: a cursor agent the sidecar supplies draws no warning" \
  '! grep -qF "cursor/docket-build-standard" <<<"$w168"'
assert "0168: a complete cursor block silences the whole harness" \
  '! grep -qF "WARN cursor/" <<<"$w168"'
assert "0168: an agent with no sidecar entry warns that it is generated unpinned" \
  'grep -qF "codex/docket-status: no harness-specific model" <<<"$w168"'
assert "0168: the unpinned warning names the key that would fix it" \
  'grep -qF "agents.codex.status.model" <<<"$w168"'
rm -rf "$SCRW" "$SBX" "$HROOT168W"
# Complement, on the REAL tree: because codex now ships complete, a shipped harness draws no
# unpinned warning at all.
#
# The negative assert CANNOT carry this on its own, and the pair is what makes the property real.
# A dropped or partial codex block makes `hd_validate` abort generation before any wrapper is
# written, so no `WARN codex/` line is ever emitted and the pure-negative assert stays green; it
# would also stay green on any unrelated `sync-agents.sh` failure. The positive companion supplies
# the missing half — the run really succeeded, a codex wrapper really exists, and it really carries
# the value the sidecar ships — so between them: generation reached completion AND it produced a
# pinned wrapper AND it did so silently. Mutation-proved by deleting the codex `status` row: the
# companion reddens (the abort writes no wrapper) while the negative alone does not.
make_sandbox
HROOT169S="$(mktemp -d)"; mkdir -p "$HROOT169S/.claude"
printf 'agent_harnesses: [claude, cursor, codex]\n' > "$SBX/.docket.yml"
w169="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT169S" bash "$SYNC" 2>&1 >/dev/null)"; rc169=$?
assert "0169: a complete codex block silences the whole harness" \
  '! grep -qF "WARN codex/" <<<"$w169"'
assert "0169: and generation actually SUCCEEDED and pinned the codex wrapper (the silence is not an abort)" \
  '[ "$rc169" = "0" ] &&
   [ -f "$SBX/.codex/agents/docket-status.toml" ] &&
   [ -n "$(hd_field "$HD" codex status model)" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SBX/.codex/agents/docket-status.toml")" = "$(hd_field "$HD" codex status model)" ]'
rm -rf "$SBX" "$HROOT169S"
# Amendment guard: a user `agents.default` outranks the sidecar, so the wrapper carries the FOREIGN
# id — the warning must fire even though the pair IS covered. Testing entry-existence instead of
# value-provenance silenced this exact case.
make_sandbox
HROOT168D="$(mktemp -d)"; mkdir -p "$HROOT168D/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n' > "$SBX/.docket.yml"
w168d="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168D" bash "$SYNC" 2>&1 >/dev/null)"
assert "0168: agents.default overriding a COVERED cursor pair still warns" \
  'grep -qF "cursor/docket-status: model '"'"'claude-opus-4-8'"'"' came from agents.default" <<<"$w168d"'
assert "0168: and the wrapper really does carry the foreign id (the warning is not a false alarm)" \
  '[ "$(sed -n "s/^model:[[:space:]]*//p" "$SBX/.cursor/agents/docket-status.md" | sed -n 1p)" = "claude-opus-4-8" ]'
rm -rf "$SBX" "$HROOT168D"

# ---- change 0168's two headline properties, asserted on a BARE opt-in --------
# 0168 whole-branch review, Recommendation 2. Everything above proves a mechanism; this proves the
# OUTCOME a repo actually gets from `agent_harnesses:` and nothing else — no agents: block, no
# overrides in any layer. Two properties, stated the way the change states them:
#   (a) Claude keeps a complete pin — every generated claude wrapper carries BOTH model and effort;
#   (b) no Claude-only model ID leaks into ANY other shipped harness's wrapper.
# (b) is the defect class change 0135 shipped and 0168 was written to make structurally impossible,
# and it is the one a future harness token added without its own sidecar entry would re-open — so
# the population below is derived from $HD_SHIPPED_HARNESSES, never hand-listed: a newly shipped
# harness is opted in, generated, and leak-scanned here for free (repo AGENTS.md).
make_sandbox
hlist=""
for h in $HD_SHIPPED_HARNESSES; do
  hlist="${hlist:+$hlist, }$h"
  [ "$h" = "claude" ] || mkdir -p "$SBX/.$h"
done
HROOT168R="$(mktemp -d)"; mkdir -p "$HROOT168R/.claude"
printf 'agent_harnesses: [%s]\n' "$hlist" > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168R" bash "$SYNC" >/dev/null 2>&1 )
n_r2=0
for f in "$SBX"/.claude/agents/docket-*.md; do
  [ -e "$f" ] || continue
  n_r2=$((n_r2+1)); b="$(basename "$f")"
  assert "0168 R2: claude/$b carries a non-empty model"  '[ -n "$(fm_anchored "'"$f"'" model)" ]'
  assert "0168 R2: claude/$b carries a non-empty effort" '[ -n "$(fm_anchored "'"$f"'" effort)" ]'
done
assert "0168 R2: the full claude set generated (floor 16; got $n_r2) — the loop above is not vacuous" \
  '[ "$n_r2" -ge 16 ]'

# Claude-ONLY model IDs: every model the sidecar's claude block names, minus every model any other
# harness block names. Derived from the sidecar, so a cursor entry that legitimately reuses a
# Claude ID (claude-opus-5-high today) is excluded rather than hand-waived.
claude_models="$(for a in $(hd_agents "$HD" claude); do hd_field "$HD" claude "$a" model; printf '\n'; done | sort -u | grep -v '^$')"
other_models="$(for h in $HD_SHIPPED_HARNESSES; do
  [ "$h" = "claude" ] && continue
  for a in $(hd_agents "$HD" "$h"); do hd_field "$HD" "$h" "$a" model; printf '\n'; done
done | sort -u | grep -v '^$')"
claude_only="$(comm -23 <(printf '%s\n' "$claude_models") <(printf '%s\n' "$other_models"))"
assert "0168 R2: the claude-only model set is non-empty (floor — otherwise the leak asserts are vacuous)" \
  '[ -n "$claude_only" ]'
# Cursor encodes effort INSIDE the model value, so compare on the bare ID with any [effort=…]
# suffix stripped; a substring match would false-positive on cursor's own claude-opus-5-high.
# The scan is keyed on the FILE's shape (TOML `model = "…"` vs frontmatter `model: …`), not on a
# list of harness names, so every non-claude harness docket ships is covered by construction.
leaks=""
n_scan=0
for h in $HD_SHIPPED_HARNESSES; do
  [ "$h" = "claude" ] && continue
  for f in "$SBX/.$h"/agents/docket-*; do
    [ -e "$f" ] || continue
    case "$f" in
      # `{p;q;}` rather than `| head -n1`: an early-exiting consumer would SIGPIPE sed under pipefail.
      *.toml) v="$(sed -n -E '/^model[[:space:]]*=/{s/^model[[:space:]]*=[[:space:]]*"(.*)"[[:space:]]*$/\1/p;q;}' "$f")" ;;
      *)      v="$(fm_anchored "$f" model)"; v="${v%%\[*}" ;;
    esac
    [ -n "$v" ] || continue
    n_scan=$((n_scan+1))
    grep -qxF "$v" <<<"$claude_only" && leaks="$leaks $h:$(basename "$f")=$v"
  done
done
# Floor: 16 wrappers on each non-claude shipped harness. Without it, a harness whose directory was
# never generated would make the leak assert below pass vacuously.
assert "0168 R2: the leak scan read a full wrapper set per non-claude harness (got $n_scan)" \
  '[ "$n_scan" -ge 48 ]'
assert "0168 R2: no non-claude wrapper carries a model that lives ONLY in the sidecar's claude block (leaks:${leaks:- none})" \
  '[ -z "$leaks" ]'
rm -rf "$SBX" "$HROOT168R"

# ============================================================================
# Change 0173 — field_of() value class: provider-prefixed model IDs round-trip
# ============================================================================
# The truncation is SILENT: a wrapper is still written and still parses, it just
# carries `anthropic` where the user wrote `anthropic/claude-opus-5`. Every assert
# here is therefore value-level — "generation succeeded" and "the wrapper exists"
# both pass against the bug.

# -- layer 1 of 3: global config.yml --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: anthropic/claude-opus-5, effort: low }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0173: global layer — slash-bearing model survives whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "anthropic/claude-opus-5" ]'
assert "0173: global layer — effort alongside it is unaffected" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
rm -rf "$SBX"

# -- layer 2 of 3: repo-committed .docket.yml --
make_sandbox
HROOT173B="$(mktemp -d)"; mkdir -p "$HROOT173B/.claude"
printf 'agents:\n  default:\n    status: { model: openai:gpt-5.6-sol, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173B" bash "$SYNC" >/dev/null )
assert "0173: committed layer — colon-bearing model survives whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "openai:gpt-5.6-sol" ]'
rm -rf "$SBX" "$HROOT173B"

# -- layer 3 of 3: machine-local .docket.local.yml --
make_sandbox
HROOT173C="$(mktemp -d)"; mkdir -p "$HROOT173C/.claude"
printf 'agents:\n  default:\n    status: { model: openrouter:vendor/model, effort: high }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173C" bash "$SYNC" >/dev/null )
assert "0173: local layer — colon AND slash together survive whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "openrouter:vendor/model" ]'
rm -rf "$SBX" "$HROOT173C"

# -- non-regression: a plain unprefixed id is untouched by the widening --
make_sandbox
HROOT173D="$(mktemp -d)"; mkdir -p "$HROOT173D/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173D" bash "$SYNC" >/dev/null )
assert "0173: plain unprefixed model still resolves exactly (non-regression)" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0173: closing brace is not swallowed into the value" \
  '! /usr/bin/grep -q "model:.*}" "$SBX/.claude/agents/docket-status.md"'
rm -rf "$SBX" "$HROOT173D"

# -- the agents.default vs agents.<harness> merge, with provenance --
# A harness-specific line and a default line, both provider-prefixed. The harness line must win
# for its own harness, the default must reach the other, and RES_MODEL_FROM_HARNESS (which drives
# warn_fallback_model) must be unaffected by the widening.
make_sandbox
mkdir -p "$SBX/.cursor"
HROOT173E="$(mktemp -d)"; mkdir -p "$HROOT173E/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: anthropic/claude-opus-5 }\n  cursor:\n    status: { model: openrouter:vendor/model }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173E" bash "$SYNC" >/dev/null )
# Cursor encodes effort INSIDE the model value as `<id>[effort=<e>]`, but only when an effort
# actually resolves. Probed against the real generated file for THIS fixture (no cursor effort is
# configured and none resolves): the emitted value is bare, with no `[effort=…]` suffix. So compare
# on the whole value — an unstripped comparison also catches a suffix appearing where none belongs.
cur_m="$(fm_anchored "$SBX/.cursor/agents/docket-status.md" model)"
assert "0173: merge — harness block wins for cursor, whole" '[ "$cur_m" = "openrouter:vendor/model" ]'
assert "0173: merge — claude falls to agents.default, whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "anthropic/claude-opus-5" ]'
rm -rf "$SBX" "$HROOT173E"

exit $fail
