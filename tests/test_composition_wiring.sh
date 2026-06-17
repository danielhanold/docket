#!/usr/bin/env bash
# tests/test_composition_wiring.sh — guards change 0017 (subagent composition wiring):
#   - implement-next step 0 dispatches the docket-status subagent
#   - implement-next step 6 dispatches the docket-adr subagent
#   - docket-convention's Composition section is the present-tense contract (no forward-pointer),
#     still references 0017, names the docket-auto-groom-critic wrapper, and states the isolation
#   - the dispatch prose pins NO literal model/effort tier (config-overridable; wrapper is the source)
# Sentinels are sampling, not parsing (LEARNINGS #5/#13) — pair with the whole-branch review.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

IMPL="$REPO/skills/docket-implement-next/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"

# --- implement-next: the two dispatch sites ---
assert "implement-next step 0 dispatches the docket-status subagent" \
  'grep -Eqi "dispatch the .?docket-status.? subagent" "$IMPL"'
assert "implement-next step 6 dispatches the docket-adr subagent" \
  'grep -Eqi "dispatch the .?docket-adr.? subagent" "$IMPL"'

# --- convention: present-tense composition contract ---
# Non-vacuous: the forward-pointer wording must be GONE (deleting the conversion flips this red).
assert "convention: composition is present-tense (no 'will spawn')" '! grep -qi "will spawn" "$CONV"'
assert "convention: composition has no 'Until 0017 lands' forward-pointer" '! grep -qi "Until 0017 lands" "$CONV"'
assert "convention: composition still references change 0017" 'grep -q "0017" "$CONV"'
assert "convention: composition names the docket-auto-groom-critic wrapper" 'grep -qF "docket-auto-groom-critic" "$CONV"'
assert "convention: critic wraps no skill" 'grep -qi "no skill" "$CONV"'
assert "convention: critic loads only docket-convention" 'grep -Eqi "only .?docket-convention" "$CONV"'

# --- no hardcoded, config-overridable model/effort tiers in the dispatch prose ---
# model/effort lives in the wrapper frontmatter + layered .docket.yml/global config (the single
# source of truth, ADR-0008); restating a literal tier here would silently drift the moment a repo
# overrides it. The prose names the subagent and says it runs at its OWN wrapper-resolved tier —
# never a literal value. Regex matches alias/effort pairs like `opus/xhigh`, `sonnet/medium`.
assert "implement-next dispatch prose pins no literal model/effort tier" \
  '! grep -qE "(opus|sonnet|haiku|fable)/(low|medium|high|xhigh|max)" "$IMPL"'
assert "convention composition pins no literal model/effort tier" \
  '! grep -qE "(opus|sonnet|haiku|fable)/(low|medium|high|xhigh|max)" "$CONV"'

exit $fail
