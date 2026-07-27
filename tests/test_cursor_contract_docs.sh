#!/usr/bin/env bash
# tests/test_cursor_contract_docs.sh — the agent-layer reference states the per-harness wrapper
# shapes rather than implying one uniform shape (change 0135), and the Cursor validation runbook
# carries its load-bearing evidence rule.
# run: bash tests/test_cursor_contract_docs.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

AL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "agent-layer: has a per-harness wrapper-shape table" \
  'grep -qiE "^\| *harness *\|" "$AL"'
# One row per harness with a named emitter. Derived from the emitters that actually exist, so a
# new named emitter without a doc row reddens (correspondence-guard-runs-one-way: anchor on the
# consuming code, never an allowlist).
for h in claude cursor codex; do
  assert "agent-layer: table has a $h row" 'grep -qE "^\| *'"$h"' *\|" "$AL"'
done
# Row-scoped, not file-scoped: `[effort=` and "body preamble" are asserted against the CURSOR ROW
# itself, so the encoding claim cannot be satisfied by the same literal appearing in an unrelated
# paragraph of the same file (a whole-file grep for a string the file discusses elsewhere is a
# false green).
assert "agent-layer: cursor row shows bracket-encoded effort" \
  'grep -qE "^\| *cursor *\|.*\[effort=" "$AL"'
assert "agent-layer: cursor row says skills ride in the body" \
  'grep -qE "^\| *cursor *\|.*body preamble" "$AL"'
# NEGATIVE assert: the removed wording, not the added wording. A guard that only confirms the new
# sentence proves the replacement is present, never that the wrong claim is gone.
assert "agent-layer: no longer claims the Cursor rule forces a Task dispatch" \
  '! grep -qF "forces a Task dispatch" "$AL"'
# The generic fallback branch is named, and named as a best guess rather than a supported mapping —
# the assumption that let the Cursor defect ship.
assert "agent-layer: names the generic \`*)\` fallback branch" \
  'grep -qF -- "*)\` branch" "$AL"'
assert "agent-layer: calls the generic branch not a supported mapping" \
  'grep -qiF -- "not a supported mapping" "$AL"'

# Every emitter named in sync-agents.sh must have a documented row (the reverse direction).
# The extraction is keyed on the SYNTACTIC SHAPE of an `emit_for_harness` case branch — any
# lowercase token followed by `)` and an `emit…` call — never on a hand-listed set of harness names
# (AGENTS.md: key a guard on syntactic shape, never an enumerated list of spellings). Naming the
# three known harnesses here would make this loop a verbatim duplicate of the forward `for h in …`
# loop above and detect nothing: a NEW named emitter shipped without a doc row — the one failure
# this direction exists to catch — would never be extracted. Mutation-proven by adding a
# `windsurf) emit_windsurf_md …` branch to sync-agents.sh.
emitters=0
while IFS= read -r h; do
  [ -n "$h" ] || continue
  emitters=$((emitters+1))
  assert "agent-layer: emitter '$h' has a documented wrapper shape" 'grep -qE "^\| *'"$h"' *\|" "$AL"'
done < <(sed -n -E 's/^[[:space:]]*([a-z][a-z0-9_-]*)\)[[:space:]]*emit.*/\1/p' "$REPO/sync-agents.sh")
# Population floor: the derivation must actually have reached sync-agents.sh's dispatch table. With
# zero extracted emitters the loop above iterates zero times and every reverse assert reads PASS
# vacuously — a renamed branch or a reshaped `case` must redden here.
assert "agent-layer: emitter derivation reached sync-agents.sh (floor: >=3 named emitters)" \
  '[ "$emitters" -ge 3 ]'

RB="$REPO/docs/cursor/validation.md"
assert "runbook: exists" '[ -f "$RB" ]'
assert "runbook: states the Tier 2 evidence rule (a negative CLI result proves nothing)" \
  'grep -qiE "never evidence that the wrapper contract is wrong" "$RB"'
# HEADING-ANCHORED, not a whole-file OR-grep: `certifying tier` also appears in body prose, so a
# file-scoped grep stayed green when the Tier 3 heading itself was downgraded to
# "(optional, best-effort)". This assert guards the merge-gate obligation the change hangs on, so
# it must key on the heading that declares it.
assert "runbook: Tier 3 heading declares it required before merge" \
  'grep -qE "^## Tier 3 .*required before merge" "$RB"'
assert "runbook: names all six IDE phases" \
  '[ "$(grep -cE "^### Phase [1-6]" "$RB")" = "6" ]'
# The probe is copy-pasteable, and its non-gating posture is stated where a future implementer
# would look to promote it.
assert "runbook: carries the cursor-agent probe invocation" \
  'grep -qF -- "cursor-agent -p" "$RB"'
assert "runbook: forbids re-promoting the Tier 2 spike to a gate" \
  'grep -qiE "must not (be )?re-?promote" "$RB"'
assert "runbook: closes with the merge-gate obligation" \
  'grep -qE "^## The merge-gate obligation" "$RB"'

assert "README: registers cursor as a shipped runner" \
  'grep -qE "runners/cursor\.md|runner: cursor" "$REPO/README.md"'
# NEGATIVE assert: README's runner count claimed ONE shipped pair before cursor landed. Confirming
# only the new "Two pairs" wording would leave the stale sentence undetected if both survived.
assert "README: no longer claims only one runner pair ships" \
  '! grep -qF "One pair ships today" "$REPO/README.md"'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
