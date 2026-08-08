#!/usr/bin/env bash
# tests/test_sync_agents_codex.sh — Codex harness TOML generation + AGENTS.md dispatch (change 0077).
# run: bash tests/test_sync_agents_codex.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Minimal TOML top-level scalar reader: prints the value (unquoted) of a bare `key = "..."`.
# Good enough for name/description/model/model_reasoning_effort (single-line basic strings).
toml_get(){ sed -n -E 's/^'"$2"'[[:space:]]*=[[:space:]]*"(.*)"[[:space:]]*$/\1/p' "$1" | head -n1; }
toml_has_key(){ grep -qE "^$2[[:space:]]*=" "$1"; }

# Markdown frontmatter readers — needed since change 0168 to state what the CLAUDE side of a codex
# run must still look like (byte identity with the source is gone; see the assert block below).
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
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
assert "codex TOML: --- divider line inside body is preserved" 'printf "%s\n" "$DIVOUT" | grep -qxF -- "---"'
assert "codex TOML: body text above the divider preserved"    'printf "%s\n" "$DIVOUT" | grep -qF "Above the rule."'
assert "codex TOML: body text below the divider preserved"    'printf "%s\n" "$DIVOUT" | grep -qF "Below the rule."'
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

# --- AGENTS.md dispatch block: created, machine-neutral, committed-style ---
mk_codex_repo
A="$SBX/AGENTS.md"
assert "agentsmd: block created" '[ -f "$A" ] && grep -qF "docket:dispatch:start" "$A"'
assert "agentsmd: has closing marker" 'grep -qF "docket:dispatch:end" "$A"'
assert "agentsmd: names an agent to delegate to" 'grep -qi "docket-implement-next" "$A"'
assert "agentsmd: carries NO model id (machine-neutral)" '! grep -qE "claude-|gpt-|model_reasoning_effort|model[[:space:]]*=" "$A"'

# 0168 whole-branch review, IMPORTANT 4. This block is COMMITTED into consumer repos and checked by
# `sync-agents.sh --check`, so a false claim in it is shipped, not just displayed. It used to open
# "Docket ships model/effort-pinned agent definitions … its pinned model and reasoning effort are
# the whole point" — true when the wrapper sources carried pins, false since change 0168 moved the
# default store to a harness-indexed sidecar that ships NO codex entries. The premise is derived
# from that sidecar rather than hard-coded. Change 0169 landed a complete codex block, so the
# premise is now false and the `else` arm below asserts the post-0169 claim just as hard.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
n_codex_shipped="$(hd_agents "$HD" codex | grep -c . || true)"
if [ "$n_codex_shipped" = "0" ]; then
  assert "agentsmd: makes no blanket 'ships pinned agent definitions' claim while the sidecar ships no codex pins" \
    '! grep -qiE "ships model/effort-pinned agent definitions" "$A"'
  assert "agentsmd: says an unconfigured codex agent runs UNPINNED" 'grep -qi "unpinned" "$A"'
  assert "agentsmd: still requires the dispatch regardless of the pin" \
    'grep -qi "either way" "$A"'
else
  # Change 0169 shipped the codex block, so the premise above is false and that arm no longer runs.
  # A guard that merely switches off leaves its NEW truth unguarded — which is exactly how the
  # cursor dispatch head kept a stale "ships IDs for the three build profiles only" claim after
  # change 0168 completed the cursor block. So the else arm asserts the post-0169 claim just as
  # hard: the block must no longer call an unconfigured Codex agent unpinned, and must still
  # require the dispatch for a reason that survives the pin.
  assert "agentsmd: no longer claims an unconfigured codex agent runs unpinned" \
    '! grep -qi "unpinned" "$A"'
  assert "agentsmd: no longer promises validated IDs are still to come" \
    '! grep -qiE "ships no validated|no validated codex|change 0169" "$A"'
  assert "agentsmd: states the dispatch is required for reasons beyond the pin" \
    'grep -qi "either way" "$A"'
  assert "agentsmd: still carries NO model id (machine-neutral even now that pins exist)" \
    '! grep -qE "claude-|gpt-|model_reasoning_effort|model[[:space:]]*=" "$A"'
fi
# 0245 whole-branch review, finding 1. The head's shipped-harness roster must be DERIVED from
# HD_SHIPPED_HARNESSES, not hand-listed: this block is committed into consumer repos and
# --check-enforced, so a hand-list goes silently false the day a fifth harness ships defaults.
# Scoped to the head (start marker .. first agent bullet) so the per-agent descriptions below
# cannot satisfy it. Case-sensitive on purpose: the derived list is lowercase, so a capitalized
# literal reintroduced alongside it reddens here.
head_txt="$(awk '/docket:dispatch:start/{f=1} f && /^- \*\*docket-/{exit} f' "$A")"
for tok in $HD_SHIPPED_HARNESSES; do
  assert "agentsmd: head names shipped harness '$tok' (derived roster)" \
    'printf "%s\n" "$head_txt" | grep -qw -- "$tok"'
done
for tok in $HD_KNOWN_HARNESSES windsurf aider zed gemini copilot; do
  case " $HD_SHIPPED_HARNESSES " in *" $tok "*) continue;; esac
  assert "agentsmd: head does not name unshipped harness '$tok'" \
    '! printf "%s\n" "$head_txt" | grep -qwi -- "$tok"'
done

# Population floor: without this, an emptied codex block would take the else arm out of service and
# BOTH arms would be satisfied by whichever branch happened to run. Anchored on the source glob so a
# seventeenth wrapper does not redden it.
n_src_codex=0
for f in "$REPO"/agents/docket-*.md; do [ -e "$f" ] || continue; n_src_codex=$((n_src_codex+1)); done
assert "agentsmd: the pinned-premise branch is the live one (codex ships $n_codex_shipped of $n_src_codex)" \
  '[ "$n_codex_shipped" = "$n_src_codex" ] && [ "$n_src_codex" -ge 16 ]'

# idempotent second run: byte-identical
before="$(cat "$A")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd: idempotent second run byte-identical" '[ "$before" = "$(cat "$A")" ]'
rm -rf "$SBX"

# --- outside bytes preserved; hand-written AGENTS.md content survives ---
mk_codex_repo   # remove then recreate with pre-existing content
rm -f "$SBX/AGENTS.md"
printf '# Our project agents\n\nHand-written guidance here.\n' > "$SBX/AGENTS.md"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd: pre-existing heading preserved" 'grep -qxF "# Our project agents" "$SBX/AGENTS.md"'
assert "agentsmd: pre-existing prose preserved"   'grep -qxF "Hand-written guidance here." "$SBX/AGENTS.md"'
assert "agentsmd: block appended below user content" 'grep -qF "docket:dispatch:start" "$SBX/AGENTS.md"'
rm -rf "$SBX"

# --- malformed markers: refuse, touch nothing ---
mk_codex_repo
rm -f "$SBX/AGENTS.md"
printf 'keepme\n<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->\ndangling\n' > "$SBX/AGENTS.md"
before="$(cat "$SBX/AGENTS.md")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd: malformed markers left untouched" '[ "$before" = "$(cat "$SBX/AGENTS.md")" ]'
rm -rf "$SBX"

# --- de-list codex: dispatch block stripped (but user content kept) ---
# The replacement harness must target NO parent-facing surface at all. Since change 0242 `claude`
# is itself a surface harness that adopts AGENTS.md through a CLAUDE.md symlink, so de-listing
# codex TO claude keeps the block alive — a correct outcome, covered by
# tests/test_sync_agents_claude_surface.sh, and the wrong fixture for this assert's claim.
mk_codex_repo
printf '# keep me\n' >> "$SBX/AGENTS.md"   # note: block is above; add trailing user line to prove strip keeps it
printf 'agent_harnesses: [cursor]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd delist: block removed" '! grep -qF "docket:dispatch:start" "$SBX/AGENTS.md"'
assert "agentsmd delist: trailing user line preserved" 'grep -qxF "# keep me" "$SBX/AGENTS.md"'
rm -rf "$SBX"

# --- --check: codex enabled + block present & current => pass ---
mk_codex_repo
git -C "$SBX" add -A >/dev/null 2>&1
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "ok - check: codex enabled, block current => passes"
else
  echo "NOT OK - check: codex enabled, block current => passes"; fail=1
fi

# --- --check: codex enabled + block STALE => CI-meaningful failure ---
# mutate the committed block so it no longer matches the emitter
perl -0pi -e 's/(docket:dispatch:start[^\n]*\n)/$1STALE-EXTRA-LINE\n/' "$SBX/AGENTS.md"
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: stale AGENTS.md block fails --check"; fail=1
else
  echo "ok - check: stale AGENTS.md block fails --check"
fi
rm -rf "$SBX"

# --- --check: codex enabled + block MISSING => CI-meaningful failure ---
mk_codex_repo
rm -f "$SBX/AGENTS.md"
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: missing AGENTS.md block fails --check"; fail=1
else
  echo "ok - check: missing AGENTS.md block fails --check"
fi
rm -rf "$SBX"

# --- --check: codex de-listed but AGENTS.md block still present => CI-meaningful failure ---
mk_codex_repo
git -C "$SBX" add -A >/dev/null 2>&1
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"   # de-list codex, do NOT re-run sync (block left behind)
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: leftover AGENTS.md block with codex de-listed fails --check"; fail=1
else
  echo "ok - check: leftover AGENTS.md block with codex de-listed fails --check"
fi
rm -rf "$SBX"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
