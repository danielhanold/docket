#!/usr/bin/env bash
# tests/test_convention_extraction.sh — run: bash tests/test_convention_extraction.sh
#
# Guards change 0005's extraction invariant in BOTH directions:
#   - the docket-convention reference skill exists and carries the full contract
#   - no operating skill contains a copy of convention content (sentinel scan)
#   - every operating skill carries the blocking Step-0 load line
#   - the retired sync machinery stays retired
#   - link-skills.sh's glob picks up the sixth skill
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

REF="$REPO/skills/docket-convention/SKILL.md"
OPERATING=(docket-new-change docket-implement-next docket-status docket-finalize-change docket-adr docket-groom-next)

# (a) reference skill exists and carries the convention's section headers
assert "docket-convention/SKILL.md exists" '[ -f "$REF" ]'
for h in "### Configuration" "### Directory layout" "### Change manifest" "### ADR file" "### Lifecycle" "### Build-readiness" "### Bootstrap guard" "### Branch model"; do
  assert "reference has header: $h" '[ -f "$REF" ] && grep -qF "$h" "$REF"'
done

# (b) anti-copy sentinels — one per convention section (spec §5); each must be IN the
# reference and ABSENT from every operating skill. The old sync markers count as copies.
SENTINELS=(
  "never gitignored"
  "proposed ──claim──▶"
  "satisfied when it reaches"
  "immutable once Accepted"
  "live planning surface"
  "half-migrated"
  "only flow of metadata onto the code line"
  "zero-padded to 4 digits"
  "PM-altitude proposal"
  "must never trail the change files"
  "<!-- docket:convention:begin -->"
  "<!-- docket:convention:end -->"
)
# slice excludes the two markers — they must be absent everywhere, including the reference
for s in "${SENTINELS[@]:0:10}"; do
  assert "reference contains sentinel: $s" '[ -f "$REF" ] && grep -qF "$s" "$REF"'
done
for sk in "${OPERATING[@]}"; do
  f="$REPO/skills/$sk/SKILL.md"
  for s in "${SENTINELS[@]}"; do
    assert "$sk has no convention copy: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
  # (c) the blocking Step-0 load line
  assert "$sk has the Step-0 load heading" 'grep -qF "## Convention (load first — blocking)" "$f"'
  assert "$sk names docket-convention" 'grep -qF "docket-convention" "$f"'
done

# (d) retired machinery stays retired
assert "sync-convention.sh retired" '[ ! -e "$REPO/sync-convention.sh" ]'
assert "test_sync_convention.sh retired" '[ ! -e "$REPO/tests/test_sync_convention.sh" ]'
assert "no other test calls sync-convention" \
  '! grep -rl "sync-convention" "$REPO/tests" --include="*.sh" | grep -v test_convention_extraction >/dev/null'

# (e) link-skills.sh globs the sixth skill (uses the script's DOCKET_HARNESS_ROOT test seam)
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/.claude/skills"
DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null
assert "link-skills.sh links docket-convention" '[ -L "$tmp/.claude/skills/docket-convention" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
