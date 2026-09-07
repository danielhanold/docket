#!/usr/bin/env bash
# docket-suite: go
# tests/test_sync_integration_wiring.sh — the prose sentinel for change 0388's
# repository.sync-integration wiring across the maintained workflow skills.
#
# It is a SAMPLING guard, not a parser: each assert anchors one greppable claim
# to the site that must carry it (learnings: prose-guard-binds-phrase-to-claim,
# specified-but-unreachable). Whitespace is collapsed into a per-file variable
# first, then matched with a here-string — never a `producer | grep -q` pipe,
# which takes SIGPIPE under pipefail (AGENTS.md Shell). Every ERE carries at
# most ONE bounded gap (learnings: stacked-gap-regex-hangs-instead-of-failing)
# and every bound stays <= 200 so it survives /usr/bin/grep's 255 interval cap
# (PATH grep is ugrep and would mask the breach). Freshness of the embedded
# copies is NOT asserted here — internal/assets' TestEmbeddedMatchesAuthored
# owns the drift guard.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

CONV="skills/docket-convention/SKILL.md"
FIN="skills/docket-finalize-change/SKILL.md"
STAT="skills/docket-status/SKILL.md"

# Collapse newlines+runs of spaces so a claim split across wrapped lines stays
# matchable. `tr` never early-exits, so this pipe is SIGPIPE-safe under pipefail.
conv="$(tr '\n' ' ' < "$CONV" | tr -s ' ')"
fin="$(tr '\n' ' ' < "$FIN" | tr -s ' ')"
stat="$(tr '\n' ' ' < "$STAT" | tr -s ' ')"

# (1) assert-detects-removal: the old "manual, human step" posture is GONE from
#     the convention. Negated grep reads the file directly (no pipe), and `--`
#     is declared even though this pattern does not lead with it (AGENTS.md).
assert "convention no longer calls post-merge sync a human/manual step" \
  '! grep -qF -- "is a human step, not part of the automated flow" '"$CONV"

# (2) the finalize skill names the semantic operation (resolved from the catalog,
#     never a restated argv).
assert "finalize skill names repository.sync-integration" \
  'grep -qF -- "repository.sync-integration" <<<"$fin"'

# (3) producer-side reachability: finalize binds the op to the post-cleanup
#     suffix — some "cleanup" precedes "sync-integration" within one window.
assert "finalize places the sync after a cleanup step" \
  'grep -qE "cleanup.{0,200}sync-integration" <<<"$fin"'

# (4) the status skill carries the no-duplicate-sync boundary.
assert "status forbids a duplicate sync-integration invocation" \
  'grep -qE "duplicate.{0,60}sync-integration" <<<"$stat"'
assert "status keeps its read-only posture beside the no-duplicate boundary" \
  'grep -qE "read-only.{0,200}duplicate" <<<"$stat"'

# (5) both maintenance-sweep scopes are named adjacent to the sync claim, in the
#     convention and in the status skill.
assert "convention binds the sync op to both maintenance-sweep scopes" \
  'grep -qE "sync-integration.{0,140}both maintenance-sweep scopes" <<<"$conv"'
assert "convention names full and implementation scope beside the scopes claim" \
  'grep -qE "maintenance-sweep scopes.{0,40}full and implementation" <<<"$conv"'
assert "status names both scopes adjacent to the sync op" \
  'grep -qE "full and implementation.{0,80}sync-integration" <<<"$stat"'

if [ "$fail" -eq 0 ]; then printf 'ALL PASS\n'; exit 0; else exit 1; fi
