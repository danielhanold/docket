#!/usr/bin/env bash
# tests/test_sync_agents_opencode.sh — the opencode emitter contract (change 0192).
# opencode agent definitions are markdown with YAML frontmatter in .opencode/agents/.
# Verified against opencode 1.18.11 and re-verified at 1.18.14: `mode: subagent` is honored, an
# unrecognized frontmatter key is forwarded to the provider under `options`, and a double-prefixed
# OpenRouter model ID parses into providerID + modelID. The 1.18.14 probe also showed opencode
# honors a `reasoningEffort` with no `model:` — docket's effort-drop below is a docket design
# choice (no effort without a named model), not an opencode limitation. See docs/opencode/setup.md.
# run: bash tests/test_sync_agents_opencode.sh
set -u
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

REPO="$(cd "$(dirname "$0")/.." && pwd)"
HD="$REPO/agents/harness-defaults.yml"
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"

WORK="$(mktemp -d "${TMPDIR:-/tmp}/docket-oc-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

mk_opencode_repo(){  # $1=dest
  mkdir -p "$1"
  git -C "$1" init -q 2>/dev/null || true
  printf 'agent_harnesses: [opencode]\n' > "$1/.docket.yml"
}

R="$WORK/repo"
mk_opencode_repo "$R"
( cd "$R" && DOCKET_HARNESS_ROOT="$WORK/home" bash "$REPO/sync-agents.sh" >"$WORK/gen.log" 2>&1 ) || true

D="$R/.opencode/agents"
assert "opencode wrappers are generated as .md" '[ "$(ls "$D"/docket-*.md 2>/dev/null | wc -l | tr -d " ")" = "17" ]'

# Every generated definition carries the sidecar's exact resolved pair. Population is derived from
# the sidecar, with a floor so a broken read cannot pass with an empty loop.
n=0
while IFS= read -r a; do
  [ -n "$a" ] || continue
  f="$D/docket-$a.md"
  m="$(hd_field "$HD" opencode "$a" model)"
  e="$(hd_field "$HD" opencode "$a" effort)"
  assert "opencode/$a: definition exists"    '[ -f "$f" ]'
  assert "opencode/$a: mode is subagent"     'grep -qx "mode: subagent" "$f"'
  assert "opencode/$a: model is $m"          'grep -qx "model: '"$m"'" "$f"'
  assert "opencode/$a: effort is $e"         'grep -qx "reasoningEffort: '"$e"'" "$f"'
  assert "opencode/$a: has a description"    'grep -q "^description: ." "$f"'
  assert "opencode/$a: carries no claude-shaped effort key" '! grep -qx "effort: '"$e"'" "$f"'
  n=$((n+1))
done < <(hd_agents "$HD" opencode)
assert "opencode: the per-agent loop covered the whole block" '[ "$n" -ge 16 ]'

# --- the shared AGENTS.md dispatch block (change 0192) -----------------------
# opencode reads the same committed project-root AGENTS.md that Codex reads, so the single managed
# block serves both harnesses. It is COMMITTED into consumer repos, so a false claim here ships.
A="$R/AGENTS.md"
assert "opencode-only repo gets the AGENTS.md dispatch block" '[ -f "$A" ] && grep -q "docket:dispatch:start" "$A"'
assert "block lists every wrapper source" \
  '[ "$(grep -c "^- \*\*docket-" "$A")" = "17" ]'
# Machine-neutrality (ADR-0036): the committed block must carry no model IDs.
assert "block carries no model IDs" \
  '! grep -qE "openrouter/|gpt-5|claude-opus|kimi-k3|deepseek" "$A"'
# Harness-neutral prose: with the block shared, naming ONE harness's artifact path is a false claim
# in the other harness's repo. Anchor on shape — the block must not hardcode either harness's
# generated-file path in its head prose.
assert "block prose is harness-neutral about the generated path" \
  '! grep -qE "\.codex/agents/docket-\*\.toml|\.opencode/agents/docket-\*\.md" "$A"'
assert "block prose names the hosting harness generically" \
  'grep -qi "hosting harness" "$A"'

# De-list: dropping the only AGENTS.md-dispatch harness strips the block. The replacement harness
# must target NO parent-facing surface: since change 0242 `claude` is itself one, adopting AGENTS.md
# through a CLAUDE.md symlink, so de-listing TO claude correctly keeps the block (covered by
# tests/test_sync_agents_claude_surface.sh) and cannot express this assert's claim.
R2="$WORK/repo2"; mk_opencode_repo "$R2"
( cd "$R2" && DOCKET_HARNESS_ROOT="$WORK/home2" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "block present before de-list" 'grep -q "docket:dispatch:start" "$R2/AGENTS.md"'
printf 'agent_harnesses: [cursor]\n' > "$R2/.docket.yml"
( cd "$R2" && DOCKET_HARNESS_ROOT="$WORK/home2" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "block stripped when no AGENTS.md-dispatch harness is targeted" \
  '! grep -q "docket:dispatch:start" "$R2/AGENTS.md" 2>/dev/null'

# --- two AGENTS.md-dispatch harnesses share ONE block (change 0245) ----------
# The discriminating fixture: every existing fixture above configures exactly ONE dispatch harness,
# and a single-owner fixture produces identical output under "is opencode listed" and "is ANY
# dispatch harness listed" — so a suite of them is green against code that never learned to share
# (learnings: shared-resource-keeps-first-owner-assumptions).
R3="$WORK/repo3"; mkdir -p "$R3"; git -C "$R3" init -q 2>/dev/null || true
printf 'agent_harnesses: [codex, opencode]\n' > "$R3/.docket.yml"
( cd "$R3" && DOCKET_HARNESS_ROOT="$WORK/home3" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
A3="$R3/AGENTS.md"
assert "two dispatch harnesses get the block EXACTLY once" \
  '[ "$(grep -c "docket:dispatch:start" "$A3")" = "1" ]'
assert "two dispatch harnesses: exactly one closing marker" \
  '[ "$(grep -c "docket:dispatch:end" "$A3")" = "1" ]'
assert "two dispatch harnesses: the block still lists every wrapper source once" \
  '[ "$(grep -c "^- \*\*docket-" "$A3")" = "17" ]'

# De-list ONE of the two: the block must SURVIVE. This is the assert no single-owner fixture reaches.
printf 'agent_harnesses: [opencode]\n' > "$R3/.docket.yml"
( cd "$R3" && DOCKET_HARNESS_ROOT="$WORK/home3" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "de-listing codex leaves the block in place (opencode still targets it)" \
  'grep -q "docket:dispatch:start" "$A3"'

# De-list the LAST one: now it goes. `cursor`, not `claude` — see the R2 de-list note above.
printf 'agent_harnesses: [cursor]\n' > "$R3/.docket.yml"
( cd "$R3" && DOCKET_HARNESS_ROOT="$WORK/home3" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "de-listing the LAST dispatch harness strips the block" \
  '! grep -q "docket:dispatch:start" "$A3" 2>/dev/null'

# --- the opencode emitter's BODY and skills preamble (change 0245) -----------
# codex and cursor both assert their emitted body survives and carries the preamble; opencode had
# neither, so a regression to an empty prompt would have shipped green.
OC="$D/docket-status.md"
assert "opencode: emitted body is non-empty" \
  '[ -n "$(awk "/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}" "$OC" | tr -d "[:space:]")" ]'
assert "opencode: body preamble tells the child to LOAD its skills" \
  'grep -qiF "load these docket skills" "$OC"'
assert "opencode: preamble names the opencode skills directory" \
  'grep -qiF "opencode skills directory" "$OC"'
assert "opencode: preamble names the agent's own skill" \
  'grep -q "load these docket skills from your opencode skills directory: docket-status" "$OC"'
assert "opencode: preamble names docket-convention"     'grep -qF "docket-convention" "$OC"'
assert "opencode: wrapper body survives verbatim"       'grep -qi "refresh docket state" "$OC"'

# --- effort is DROPPED when no model resolves (change 0245) ------------------
# Docket's own half of the "a provider option with no provider selected has nothing to reach"
# rationale: pinned by test regardless of whether the opencode CLI is present to probe the other
# half. The fixture pins model to the `inherit` sentinel, which normalizes to "no model".
R4="$WORK/repo4"; mkdir -p "$R4"; git -C "$R4" init -q 2>/dev/null || true
cat > "$R4/.docket.yml" <<'YML'
agent_harnesses: [opencode]
agents:
  opencode:
    status: {model: inherit, effort: high}
YML
( cd "$R4" && DOCKET_HARNESS_ROOT="$WORK/home4" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/oc4.err" || true
F4="$R4/.opencode/agents/docket-status.md"
assert "opencode effort-drop: no model line is emitted"  '! grep -qE "^model:" "$F4"'
assert "opencode effort-drop: NO reasoningEffort key is emitted" '! grep -qE "^reasoningEffort:" "$F4"'
assert "opencode effort-drop: the drop is WARNed, not silent" \
  'grep -q "effort .high. dropped" "$WORK/oc4.err"'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
