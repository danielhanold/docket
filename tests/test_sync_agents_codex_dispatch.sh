#!/usr/bin/env bash
# tests/test_sync_agents_codex_dispatch.sh — the AGENTS.md dispatch-block half of the codex harness
# suite (sharded out of tests/test_sync_agents_codex.sh at change 0242's review).
#
# WHY THIS FILE EXISTS. Its parent covered two independent surfaces — the per-repo `.codex/agents/*.toml`
# wrappers and the committed `AGENTS.md` dispatch block — and change 0242 added a second `--check`
# leg to the dispatch half. Measured serially at 56-57s against a 55s row and a hard 60s ceiling,
# the file had no room left, and the table's own remedy is a shard, never a bigger number
# (tests/runtime-budgets.tsv, "NO row may exceed 60 seconds"). The `# ---` section banner
# "AGENTS.md dispatch block: created, machine-neutral, committed-style" is the boundary: nothing
# above it reads $A, and nothing below it reads a .toml wrapper.
#
# run: bash tests/test_sync_agents_codex_dispatch.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Opt a sandbox repo into [claude, codex] and generate.
mk_codex_repo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}

# --- AGENTS.md dispatch block: created, machine-neutral, committed-style ---
mk_codex_repo
A="$SBX/AGENTS.md"
assert "agentsmd: block created" '[ -f "$A" ] && grep -qF "docket:dispatch:start" "$A"'
assert "agentsmd: has closing marker" 'grep -qF "docket:dispatch:end" "$A"'
assert "agentsmd: names an agent to delegate to" 'grep -qi "docket-implement-next" "$A"'
assert "agentsmd: carries NO model id (machine-neutral)" '! grep -qE "claude-|gpt-|model_reasoning_effort|model[[:space:]]*=" "$A"'

# This block is COMMITTED into consumer repos and checked by `sync-agents.sh --check`, so a false
# claim in it ships rather than merely displaying. Since change 0334 it no longer enumerates the
# agent roster OR the shipped-harness list: it carries a compact registered-agent routing rule that
# defers to the harness's own registry. The guards below assert that compact block, and — as
# importantly — assert the roster STAYS removed (learnings: assert-detects-removal-not-replacement);
# an absence guard keyed on new wording would go green the day the roster crept back under a rename.
# These are the shell twins of the Go interior guard in internal/harness/dispatch_test.go — the two
# generators emit a byte-identical block, so both surfaces move together.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"

# Whitespace-collapse the managed block so a CLAIM survives a pure re-flow (learnings:
# phrase-grep-over-wrapped-prose). block_flat is the marker-to-marker interior on one line;
# block_lines keeps the line structure so the roster-bullet SHAPE can be matched per line.
block_flat="$(awk '/docket:dispatch:start/{f=1;next} /docket:dispatch:end/{f=0} f' "$A" | tr '\n' ' ' | tr -s '[:space:]' ' ')"
block_lines="$(awk '/docket:dispatch:start/{f=1;next} /docket:dispatch:end/{f=0} f' "$A")"

# Positive — the routing rule's load-bearing claims, each bound to what it asserts.
assert "agentsmd: routes only when a same-name agent is registered" \
  '[[ "$block_flat" == *"registered same-name"* ]]'
assert "agentsmd: defers to the harness registry for names/descriptions/availability" \
  '[[ "$block_flat" == *"authoritative for agent names, descriptions, and availability"* ]]'
assert "agentsmd: forbids inventing an unregistered agent" \
  '[[ "$block_flat" == *"do not invent one"* ]]'
# The dispatch is still required for a reason that survives the pin: the agent carries the
# workflow's dispatch contract and skill preload, not just a model/effort pin.
assert "agentsmd: states the dispatch carries the contract + preload, beyond the pin" \
  '[[ "$block_flat" == *"dispatch contract"*"skill preload"* ]]'

# Negative — the roster is GONE. Detect its REMOVAL by SHAPE, never a spelling list (learnings:
# key-a-guard-on-shape): no interior line is a `- **docket-...` roster bullet, and the delegation
# clause is absent. Captured into a variable first, then grepped from a here-string — never
# `producer | grep -q` under pipefail (repo AGENTS.md shell rule).
assert 'agentsmd: no roster bullet survives (shape `- **docket-`)' \
  '! grep <<<"$block_lines" -qE "^- \*\*docket-"'
assert "agentsmd: no roster delegation clause survives" \
  '! grep -qF -- "Delegate to the" "$A"'
# Machine-neutral: the block names NO harness token at all (roster + shipped-harness list removed).
# A capitalized or lowercase reintroduction of any known/adjacent harness name reddens here.
for tok in $HD_KNOWN_HARNESSES windsurf aider zed gemini copilot; do
  assert "agentsmd: block names no harness token '$tok' (machine-neutral, roster removed)" \
    '! grep <<<"$block_flat" -qwi -- "$tok"'
done

# Codex-sidecar completeness (independent of the dispatch block, which no longer restates the
# roster): the codex default store must ship a pin for every source agent, so a de-pinned sidecar
# cannot slip through unnoticed. Anchored on the source glob so a seventeenth wrapper does not
# redden it.
HD="$REPO/agents/harness-defaults.yml"
n_codex_shipped="$(hd_agents "$HD" codex | grep -c . || true)"
n_src_codex=0
for f in "$REPO"/agents/docket-*.md; do [ -e "$f" ] || continue; n_src_codex=$((n_src_codex+1)); done
assert "agentsmd: codex sidecar ships a pin for every source agent ($n_codex_shipped of $n_src_codex)" \
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

# --- --check: no dispatch-surface harness left but the AGENTS.md block is still present =>
# CI-meaningful failure. De-listed to [cursor], NOT to [claude]: since change 0242 claude is itself
# a dispatch-surface harness, and mk_codex_repo's own [claude, codex] run left a CLAUDE.md symlink
# pointing AT this AGENTS.md — so under [claude] the block is still live through that link, not
# leftover, and a real sync would keep it. The claim being guarded is unchanged: a block no
# targeted harness owns must fail the check.
#
# Both de-list directions run against ONE generated fixture — a second mk_codex_repo would put this
# file over its runtime budget, and neither direction mutates anything the other reads.
mk_codex_repo
git -C "$SBX" add -A >/dev/null 2>&1
# Direction 1 — de-listed to [claude] alone: the block stays LIVE, because mk_codex_repo's CLAUDE.md
# link resolves to this very AGENTS.md. The check must PASS; reporting it would tell CI to run a
# sync that would change nothing.
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "ok - check: codex de-listed to claude keeps the block live through the CLAUDE.md link"
else
  echo "NOT OK - check: codex de-listed to claude keeps the block live through the CLAUDE.md link"; fail=1
fi
# Direction 2 — de-listed to [cursor]: now NO harness targets a dispatch surface, so the block is
# genuinely leftover and the check must FAIL.
printf 'agent_harnesses: [cursor]\n' > "$SBX/.docket.yml"   # de-list every surface harness, do NOT re-run sync
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: leftover AGENTS.md block with every surface harness de-listed fails --check"; fail=1
else
  echo "ok - check: leftover AGENTS.md block with every surface harness de-listed fails --check"
fi
rm -rf "$SBX"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
