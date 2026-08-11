#!/usr/bin/env bash
# tests/test_cursor_contract_docs.sh — the agent-layer reference states the per-harness wrapper
# shapes rather than implying one uniform shape (change 0135), and the Cursor validation runbook
# carries its load-bearing evidence rule.
# run: bash tests/test_cursor_contract_docs.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

AL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "agent-layer: has a per-harness wrapper-shape table" \
  'grep -qiE "^\| *harness *\|" "$AL"'
# One row per harness with a named emitter. Population derived from HD_SHIPPED_HARNESSES rather
# than a literal list, so a newly shipped harness cannot land without a doc row
# (correspondence-guard-runs-one-way: anchor on the consuming code, never an allowlist).
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
n_rows=0
for h in $HD_SHIPPED_HARNESSES; do
  assert "agent-layer: table has a $h row" 'grep -qE "^\| *'"$h"' *\|" "$AL"'
  n_rows=$((n_rows+1))
done
assert "agent-layer: the row loop covered every shipped harness" '[ "$n_rows" -ge 4 ]'
# Row-scoped, not file-scoped: `[effort=` and "body preamble" are asserted against the CURSOR ROW
# itself, so the encoding claim cannot be satisfied by the same literal appearing in an unrelated
# paragraph of the same file (a whole-file grep for a string the file discusses elsewhere is a
# false green).
assert "agent-layer: cursor row shows bracket-encoded effort" \
  'grep -qE "^\| *cursor *\|.*\[effort=" "$AL"'
assert "agent-layer: cursor row says skills ride in the body" \
  'grep -qE "^\| *cursor *\|.*body preamble" "$AL"'
assert "agent-layer: opencode row shows the reasoningEffort passthrough" \
  'grep -qE "^\| *opencode *\|.*reasoningEffort" "$AL"'
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
assert "runbook: names all seven IDE phases" \
  '[ "$(grep -cE "^### Phase [1-7]" "$RB")" = "7" ]'
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
# The same stale claim also shipped in .docket.example.yml's `runners:` comment and went stale there
# for two whole runners (cursor, then opencode) because this guard hand-listed README as its only
# site. Derive the sites instead of naming one (AGENTS.md), so the next runner cannot re-stale a
# surface nobody is watching.
#
# Enumerate with `git grep`, NOT `grep -r "$REPO"`. The checkout root contains docket's own
# `.worktrees/<slug>/` siblings and the `.docket/` metadata worktree; a recursive filesystem walk
# audits those foreign checkouts too, and a sibling branched off older main legitimately still
# carries the stale sentence — which would redden this suite in the primary tree for reasons that
# have nothing to do with this repo's content. `git grep` is scoped to THIS checkout's tracked
# files, matching the `git ls-files` idiom test_comment_anchor_style.sh already uses for the same
# reason. Pathspecs also replace the --include filters, so the whole pipeline is one extractor.
stale_grep(){ git -C "$REPO" grep -lF "$1" -- '*.md' '*.yml' 2>/dev/null | grep -v "^docs/changes/archive/\|^docs/results/\|^docs/superpowers/"; }
stale_pair_sites="$(stale_grep "One pair ships today" | tr '\n' ' ')"
assert "no maintained file still claims only one runner pair ships (${stale_pair_sites:-none})" \
  '[ -z "${stale_pair_sites// /}" ]'
# NON-VACUITY COMPANION, through the SAME extractor (assert-detects-removal-not-replacement): the
# earlier version grepped two named files with no recursion, no pathspecs and no exclusion pipeline,
# so a broken traversal would leave the absence assert vacuously green while the companion still
# passed. Probe a sentinel that must exist in a maintained *.md/*.yml via the identical function.
probe_sites="$(stale_grep "runner:" | tr '\n' ' ')"
assert "the stale-claim extractor still finds live content through the same pipeline (got ${probe_sites:-none})" \
  '[ -n "${probe_sites// /}" ]'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
