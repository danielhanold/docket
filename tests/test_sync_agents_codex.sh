#!/usr/bin/env bash
# tests/test_sync_agents_codex.sh — Codex harness TOML wrapper generation (change 0077).
#
# The AGENTS.md dispatch-block half lives in tests/test_sync_agents_codex_dispatch.sh, sharded out
# at change 0242's review: this file measured 56-57s serially against a 55s row and the table's
# hard 60s ceiling, and the remedy for a file that outgrows its ceiling is a shard, never a bigger
# number. Add a wrapper/TOML assertion here; add a dispatch-block assertion to the sibling.
#
# run: bash tests/test_sync_agents_codex.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Minimal TOML top-level scalar reader: prints the value (unquoted) of a bare `key = "..."`.
# Good enough for name/description/model/model_reasoning_effort (single-line basic strings).
toml_get(){ sed -n -E 's/^'"$2"'[[:space:]]*=[[:space:]]*"(.*)"[[:space:]]*$/\1/p' "$1" | sed -n 1p; }
toml_has_key(){ grep -qE "^$2[[:space:]]*=" "$1"; }

# Markdown frontmatter readers — needed since change 0168 to state what the CLAUDE side of a codex
# run must still look like (byte identity with the source is gone; see the assert block below).
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | sed -n 1p | sed 's/[[:space:]]*$//'; }
body_of(){ awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$1"; }

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
assert "codex: full built-in set as TOML (16 files)"      '[ "$(find "$SBX/.codex/agents" -name "docket-*.toml" | wc -l | tr -d " ")" = "16" ]'

T="$SBX/.codex/agents/docket-status.toml"
assert "codex TOML: name = docket-status"          '[ "$(toml_get "$T" name)" = "docket-status" ]'
assert "codex TOML: description matches source"    '[ "$(toml_get "$T" description)" = "$(sed -n "/^description:/{s/^description:[[:space:]]*//;p;q;}" "$REPO/agents/docket-status.md")" ]'
# Change 0169 ships a complete codex block, so the honest output is a PINNED wrapper. These read
# the expected values from the sidecar rather than restating them: a literal here would be a second
# copy of the shipped table, free to drift from the one the generator actually reads.
#
# These are the two asserts change 0168 inverted to absence; they are inverted BACK rather than
# deleted, so their original job — catching a cross-harness leak, a Claude ID landing in a Codex
# wrapper — stays live. The leak now shows up as a value mismatch instead of as an unexpected key.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
assert "codex TOML: model is the shipped codex pin" \
  '[ -n "$(hd_field "$HD" codex status model)" ] &&
   [ "$(toml_get "$T" model)" = "$(hd_field "$HD" codex status model)" ]'
assert "codex TOML: model_reasoning_effort is the shipped codex effort" \
  '[ -n "$(hd_field "$HD" codex status effort)" ] &&
   [ "$(toml_get "$T" model_reasoning_effort)" = "$(hd_field "$HD" codex status effort)" ]'
# And no Codex wrapper carries a Claude-namespace model ID — the cross-harness leak, stated as the
# property rather than as one agent's value.
assert "codex TOML: model is not a claude-namespace ID" \
  '! grep -qE "^model[[:space:]]*=[[:space:]]*\"claude-" "$T"'
# Whole-set coverage: every one of the sixteen generated wrappers matches its sidecar row. Population
# derived from the sidecar, so a seventeenth agent arms this loop automatically.
n_codex_checked=0
while IFS= read -r a; do
  [ -n "$a" ] || continue
  n_codex_checked=$((n_codex_checked+1))
  assert "codex TOML docket-$a: model + effort match the sidecar" \
    '[ "$(toml_get "$SBX/.codex/agents/docket-'"$a"'.toml" model)" = "$(hd_field "$HD" codex "'"$a"'" model)" ] &&
     [ "$(toml_get "$SBX/.codex/agents/docket-'"$a"'.toml" model_reasoning_effort)" = "$(hd_field "$HD" codex "'"$a"'" effort)" ]'
done < <(hd_agents "$HD" codex)
assert "codex TOML: every shipped codex entry was checked (floor 16; got $n_codex_checked)" \
  '[ "$n_codex_checked" -ge 16 ]'
assert "codex TOML: has developer_instructions"    'grep -qE "^developer_instructions[[:space:]]*=" "$T"'
assert "codex TOML: dev_instructions carry body"   'grep -qi "refresh docket state" "$T"'
assert "codex TOML: dev_instructions name the skills to load" 'grep -qi "docket-convention" "$T"'
# claude side is untouched and still markdown. Byte identity with the source is structurally
# impossible since change 0168 — the source is a behavior-only template and the generator injects
# the pin from agents/harness-defaults.yml — so the property is asserted directly: the codex
# emitter changed NOTHING on the Claude side except that injection.
assert "claude side still .md, not .toml" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "claude side: body is verbatim from its source" \
  'diff -q <(body_of "$REPO/agents/docket-status.md") <(body_of "$SBX/.claude/agents/docket-status.md") >/dev/null'
assert "claude side: name/description/skills come from the source" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" name)" = "docket-status" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" description)" = "$(fm "$REPO/agents/docket-status.md" description)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" skills)" = "$(fm "$REPO/agents/docket-status.md" skills)" ]'
# Pins both halves: the generated file HAS the shipped pin and the source does NOT, so restoring a
# default to the source frontmatter reddens it just as dropping the injection does.
assert "claude side: carries the SHIPPED pin, injected not copied" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "medium" ] &&
   ! grep -qE "^(model|effort):" "$REPO/agents/docket-status.md"'
rm -rf "$SBX"

# --- user override outranks the shipped codex block, FIELD BY FIELD (spec Tier-1 property 6) -----
# Nothing else writes an `agents.codex.<agent>` override and checks the wrapper, so before this the
# property was carried only by the .docket.example.yml round-trip — which cannot see it (the example
# mirrors the sidecar, so both sides of that comparison move together).
#
# The fixture is deliberately PARTIAL — a user model with no user effort — because that is the
# backward-compat hazard change 0169 introduces: before the codex block shipped, such an agent got
# NO effort line; now it silently inherits docket's shipped effort next to a model the user chose.
# Cursor had no equivalent hazard at change 0168 (every cursor effort is `auto`, which the emitter
# drops); Codex's efforts are real tokens that reach the harness. This assert pins that resolution
# as INTENDED behavior and is the executable half of the upgrade warning in docs/codex/setup.md.
SBX="$(mktemp -d)"
git -C "$SBX" init --quiet
git -C "$SBX" config user.email t@t.test
git -C "$SBX" config user.name Test
printf 'agent_harnesses: [claude, codex]\nagents:\n  codex:\n    status: { model: gpt-5.1-codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
OT="$SBX/.codex/agents/docket-status.toml"
assert "override: the wrapper carries the USER model, not the shipped one" \
  '[ -f "$OT" ] && [ "$(toml_get "$OT" model)" = "gpt-5.1-codex" ] &&
   [ "$(hd_field "$HD" codex status model)" != "gpt-5.1-codex" ]'
assert "override: with no user effort, the SHIPPED codex effort is still applied (fields resolve independently)" \
  '[ -n "$(hd_field "$HD" codex status effort)" ] &&
   [ "$(toml_get "$OT" model_reasoning_effort)" = "$(hd_field "$HD" codex status effort)" ]'
assert "override: an unoverridden codex agent is untouched by the partial override" \
  '[ "$(toml_get "$SBX/.codex/agents/docket-adr.toml" model)" = "$(hd_field "$HD" codex adr model)" ]'
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
assert "codex TOML: --- divider line inside body is preserved" 'grep <<<"$DIVOUT" -qxF -- "---"'
assert "codex TOML: body text above the divider preserved"    'grep <<<"$DIVOUT" -qF "Above the rule."'
assert "codex TOML: body text below the divider preserved"    'grep <<<"$DIVOUT" -qF "Below the rule."'
rm -rf "$DIVDIR"

# --- orphan prune: a removed built-in drops its codex .toml wrapper ---
mk_codex_repo
cp "$REPO/agents/docket-status.md" "$SBX/.codex/agents/docket-ghost.toml"   # simulate a stale wrapper
touch "$SBX/.codex/agents/docket-ghost.toml"
# regenerate: docket-ghost has no built-in source -> must be pruned
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "prune: orphan .toml wrapper removed" '[ ! -e "$SBX/.codex/agents/docket-ghost.toml" ]'
assert "prune: real .toml wrapper kept"      '[ -f "$SBX/.codex/agents/docket-status.toml" ]'
rm -rf "$SBX"

# --- de-list codex: its .toml wrappers are pruned ---
mk_codex_repo
assert "pre: codex wrappers exist" '[ -f "$SBX/.codex/agents/docket-status.toml" ]'
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "delist: codex .toml wrappers pruned" '[ ! -e "$SBX/.codex/agents/docket-status.toml" ]'
assert "delist: claude wrappers remain"      '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# --- --check leg (b): a TRACKED codex .toml is CI-meaningful (exit non-zero) ---
mk_codex_repo
git -C "$SBX" add -f .codex/agents/docket-status.toml
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: tracked .toml wrapper fails --check"; fail=1
else
  echo "ok - check: tracked .toml wrapper fails --check"
fi
rm -rf "$SBX"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
