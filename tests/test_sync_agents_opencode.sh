#!/usr/bin/env bash
# tests/test_sync_agents_opencode.sh — the opencode emitter contract (change 0192).
# opencode agent definitions are markdown with YAML frontmatter in .opencode/agents/.
# Verified against opencode 1.18.11: `mode: subagent` is honored, an unrecognized frontmatter key
# is forwarded to the provider under `options`, and a double-prefixed OpenRouter model ID parses
# into providerID + modelID. See docs/opencode/setup.md.
# run: bash tests/test_sync_agents_opencode.sh
set -u
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

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
assert "opencode wrappers are generated as .md" '[ "$(ls "$D"/docket-*.md 2>/dev/null | wc -l | tr -d " ")" = "16" ]'

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

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
