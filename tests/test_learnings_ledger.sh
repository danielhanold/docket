#!/usr/bin/env bash
# tests/test_learnings_ledger.sh — guards change 0006 (the learnings ledger):
#   - the convention carries the Learnings ledger contract (single source)
#   - the harvest procedure lives in docket-finalize-change; docket-status references it
#   - the readers (implement-next, groom-next) carry their read lines
#   - no operating skill restates the contract (sentinel scan)
# The ledger FILE lives on the docket branch only and is not testable here (see plan/results).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"
OPERATING=(docket-new-change docket-groom-next docket-implement-next docket-status docket-finalize-change docket-adr)

# (a) the convention contract — single source
assert "convention has the Learnings ledger section" 'grep -qF "### Learnings ledger" "$CONV"'
assert "convention names the findings directory" 'grep -qF "<changes_dir>/learnings/" "$CONV"'
assert "convention names the generated index as derived" \
  'grep -qF "is a **derived view**" "$CONV"'
assert "convention states the tiering criterion" \
  'grep -qF "will the agent know to search for this?" "$CONV"'
assert "convention states the cap counts active findings" \
  'grep -qF "counts **active findings**" "$CONV"'
assert "convention states the off switch is a gate, not a purge" \
  'grep -qF "a no-op **read/write gate, never a" "$CONV"'
assert "convention pins the promotion_state enum" \
  'grep -qF "retained | candidate | promoted" "$CONV"'
assert "directory layout lists the learnings dir" \
  'grep -qE "^  learnings/ +# curated build-loop findings" "$CONV"'
assert "convention keeps the LEARNINGS.md stub pointer" 'grep -qF "remains as a pointer stub" "$CONV"'

# (b) the harvest procedure — RETIRED for the finalize Go sequencer (0316, Out of scope).
# RETIRED (0316, category (a)): the finalize skill used to single-source an automatic "Harvest
# learnings" step (a changes:-list idempotency probe, a learnings.enabled gate, and an index
# re-render through `docket.sh render-learnings-index`). 0316's *Out of scope* defers "automatic
# learning harvest" — by design `docket learning` is MANUAL `record`/`update` only, so the Go
# sequencer carries no harvest step. Authority #1 (Out of scope: automatic learning harvest). Do NOT
# restore the harvest step to make these green. Inverted guards proving the automatic harvest is
# absent from the finalize sequencer, with a non-vacuity anchor. (docket-status's own harvest leg,
# index self-heal, and capacity/promotion advisories are RETIRED by change 0372 — the 0372 block
# below asserts their absence; keep it and the finalize inversions here together.)
FIN_LL="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize SKILL is the Go sequencer (non-vacuity anchor)" 'grep -qF "docket finalize" "$FIN_LL"'
assert "finalize carries no deferred automatic learning-harvest step" \
  '! grep -qF "Harvest learnings" "$FIN_LL"'
assert "finalize carries no deferred harvest idempotency probe" \
  '! grep -qF "already contains this change" "$FIN_LL"'
assert "finalize carries no deferred learnings.enabled harvest gate" \
  '! grep -qF "learnings disabled — harvest skipped" "$FIN_LL"'
assert "finalize does not re-render the learnings index by hand (deferred harvest)" \
  '! grep -qF "docket.sh render-learnings-index" "$FIN_LL"'
# --- change 0372: learnings automation is retired; the read path and ledger format stay --
st372="$(cat "$REPO/skills/docket-status/SKILL.md")"
assert "0372: docket-status carries no harvest leg" '! grep -Fq "Harvest learnings" <<<"$st372"'
assert "0372: docket-status never invokes the learnings-index renderer" \
  '! grep -Eq "render-learnings-index" <<<"$st372"'
assert "0372: docket-status computes no capacity/promotion advisories" \
  '! grep -Fq "over-cap" <<<"$st372" && ! grep -Fq "promotion-pending" <<<"$st372"'
ll372="$(cat "$REPO/skills/docket-convention/references/learnings.md")"
assert "0372: learnings ref states the harvest deferral diagnostic (floor)" \
  'grep -Fq "automated learnings harvest is deferred from Go v1" <<<"$ll372"'
assert "0372: learnings ref states the index deferral diagnostic (floor)" \
  'grep -Fq "automated learnings-index rendering is deferred from Go v1" <<<"$ll372"'
assert "0372: learnings ref states the capacity/promotion deferral diagnostic (floor)" \
  'grep -Fq "automated learnings capacity and promotion are deferred from Go v1" <<<"$ll372"'
# read path survives (non-vacuity companions through the same files the absence asserts read)
assert "0372: implement-next still reads the learnings index" \
  '[ "$(grep -cF "learnings/README.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "0372: implement-next still gates reads on learnings.enabled" \
  '[ "$(grep -cF "learnings.enabled" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 1 ]'

# (c) the readers — the two-step index-first read contract, at all three hot moments
assert "implement-next reads the index at plan time AND review" \
  '[ "$(grep -cF "learnings/README.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
# Both hot-moment reads stay enablement-gated, but change 0324 split WHERE the plan-time read runs:
# Step 6's review read is still performed and gated in this skill (`learnings.enabled`), while Step
# 4's plan-time read moved into the dispatched docket-plan-writer child — the parent now RESOLVES
# enablement and passes "whether learnings are enabled" (and, only when enabled, the index path) in
# the dispatch payload, and the child reads. Keyed on each moment's own gating phrase so a regression
# that drops either gate still reddens; the child's own read-gating is guarded by the "the child
# gates its learnings-index read on enablement" assert in test_plan_writer_agent.sh.
assert "implement-next review read gates on learnings.enabled" \
  '[ "$(grep -cF "learnings.enabled" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 1 ]'
assert "implement-next plan-time read gates enablement into the plan-writer child payload" \
  'grep -qF "whether learnings are enabled" "$REPO/skills/docket-implement-next/SKILL.md"'
assert "groom-next reads the index before the brainstorm" \
  'grep -qF "learnings/README.md" "$REPO/skills/docket-groom-next/SKILL.md"'
assert "groom-next gates its read on learnings.enabled" \
  'grep -qF "learnings.enabled" "$REPO/skills/docket-groom-next/SKILL.md"'
# No reader may still point at the retired single-file ledger as a READ target.
for sk in docket-implement-next docket-groom-next; do
  assert "$sk no longer reads the retired LEARNINGS.md" \
    '! grep -qEi "read .*LEARNINGS\.md" "$REPO/skills/$sk/SKILL.md"'
done

# (c') change 0067 plan-gap fix: the two sites Task 7's enumeration missed —
# docket-auto-groom's self-brainstorm scan and docket-brainstorm's consultant payload.
assert "auto-groom reads the learnings index before its self-brainstorm" \
  'grep -qF "learnings/README.md" "$REPO/skills/docket-auto-groom/SKILL.md"'
assert "auto-groom gates its learnings read on learnings.enabled" \
  'grep -qF "learnings.enabled" "$REPO/skills/docket-auto-groom/SKILL.md"'
assert "brainstorm's consultant payload references learnings findings/index, not the retired ledger" \
  'grep -qEi "learnings (findings|index)" "$REPO/skills/docket-brainstorm/SKILL.md"'
assert "convention's Readers line names docket-auto-groom" \
  'grep -E "^\*\*Readers:\*\*.*docket-auto-groom" "$CONV" >/dev/null'

# Completeness guard — SHAPE, not a hand-listed corpus. A hand-listed file set (like the
# `for sk in ...` loop above) is exactly the floor-not-the-set defect that let auto-groom
# and docket-brainstorm slip past Task 7's enumeration. Glob every live skill instead: any
# SKILL.md mentioning LEARNINGS.md WITHOUT qualifying it as the retired pointer stub is
# treated as a live read target and fails. docket-convention's directory-layout line and its
# "remains as a pointer stub" sentence are the only legitimate mentions, and both carry the
# phrase "pointer stub" on the same line — that phrase is the exemption, not a filename.
assert "no live skill still names LEARNINGS.md as a read target (glob corpus; convention's pointer-stub mentions exempt)" \
  '[ -z "$(grep -F "LEARNINGS.md" "$REPO"/skills/*/SKILL.md 2>/dev/null | grep -Fvi "pointer stub")" ]'

# Completeness guard #2 (ADDED alongside, never narrowing the guard above — a widened/collapsed
# guard in place is the retired-vocabulary equivalent of ADR-0031's "never weaken in place" rule).
# skills/*/SKILL.md alone is a floor, not the set: docket-brainstorm-consultant.md's built-in
# agent wrapper (agents/) and its Cursor dispatch-fragment twin (cursor-rules/dispatch/) both carry
# the same "handed a settled design" payload prose as skills/docket-brainstorm/SKILL.md, and both
# went stale on the retired "LEARNINGS excerpts" wording when that SKILL.md was updated to
# "relevant learnings findings, drawn from the learnings index" — exactly the class of miss a
# hand-scoped corpus lets through. This corpus is wide from the start.
LEARN_VOCAB_CORPUS=( "$REPO"/skills/*/SKILL.md "$REPO"/agents/docket-*.md "$REPO"/cursor-rules/dispatch/*.md )
assert "the retired-vocabulary corpus is non-empty (the guard actually scanned files)" \
  '[ "${#LEARN_VOCAB_CORPUS[@]}" -ge 20 ]'
assert "no live surface still says LEARNINGS excerpts (glob corpus: skills/, agents/, cursor-rules/dispatch/)" \
  '[ -z "$(grep -lF "LEARNINGS excerpts" "${LEARN_VOCAB_CORPUS[@]}" 2>/dev/null)" ]'

# (d) anti-restatement sentinels — contract phrases live ONLY in the convention
for s in "build-loop memory" "will the agent know to search for this?"; do
  assert "convention contains sentinel: $s" 'grep -qF "$s" "$CONV"'
  for sk in "${OPERATING[@]}"; do
    f="$REPO/skills/$sk/SKILL.md"
    assert "$sk does not restate: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
done

# (e) end-to-end surfacing — LEARNINGS #49: a knob is not done when it merely works
# Change 0101 relocated the user-facing config surface to .docket.example.yml, where every key
# ships ACTIVE at its shipped default (this repo's .docket.yml is now values-only). The knob must
# still be discoverable to a user reading the canonical reference — that is what (e) is for.
assert "the canonical example carries the learnings block" \
  'grep -qE "^learnings:$" "$REPO/.docket.example.yml"'
# Block-scoped: an unanchored grep for `enabled: true` / `cap: 300` would match those values
# under ANY block in the example, so it would still pass if the learnings block were removed.
learnings_block(){  # echoes the active lines nested under `learnings:`
  awk '/^learnings:[[:space:]]*$/{f=1;next} f&&/^[^[:space:]#]/{f=0} f' "$REPO/.docket.example.yml"
}
assert "the example documents both keys at their defaults" \
  'learnings_block | grep >/dev/null -E "^[[:space:]]+enabled: true$" && learnings_block | grep >/dev/null -E "^[[:space:]]+cap: 300$"'
assert "README presents learnings as a feature" 'grep -qF "## Learnings — the loop" "$REPO/README.md"'
assert "README points at the convention rather than restating mechanics" \
  'grep -qF "Learnings ledger" "$REPO/README.md"'
assert "AGENTS.md exists as the promotion destination" '[ -f "$REPO/AGENTS.md" ]'
assert "AGENTS.md states the tiering criterion" \
  'grep -qF "will the agent know to search for this?" "$REPO/AGENTS.md"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
