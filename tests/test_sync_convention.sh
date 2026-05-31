#!/usr/bin/env bash
# tests/test_sync_convention.sh — run: bash tests/test_sync_convention.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/skills/docket-new-change" "$tmp/skills/docket-status"
cp "$REPO/sync-convention.sh" "$tmp/sync-convention.sh"

B='<!-- docket:convention:begin -->'
E='<!-- docket:convention:end -->'
# Canonical (docket-new-change) holds the real block; status holds a STALE one.
printf '# head\n%s\n## Convention\nCANON v2\n%s\nrest-new\n' "$B" "$E" > "$tmp/skills/docket-new-change/SKILL.md"
printf '# head\n%s\n## Convention\nOLD v1\n%s\nrest-status\n'  "$B" "$E" > "$tmp/skills/docket-status/SKILL.md"

# --check detects drift (exit 1)
( cd "$tmp" && bash sync-convention.sh --check >/dev/null 2>&1 ); rc=$?
assert "--check flags drift (exit 1)" '[ "$rc" = "1" ]'

# sync propagates canonical into status, preserving non-block content
( cd "$tmp" && bash sync-convention.sh >/dev/null ); rc=$?
assert "sync exits 0" '[ "$rc" = "0" ]'
assert "status block now matches canonical" 'grep -q "CANON v2" "$tmp/skills/docket-status/SKILL.md"'
assert "stale block removed"               '! grep -q "OLD v1" "$tmp/skills/docket-status/SKILL.md"'
assert "non-block content preserved"        'grep -q "rest-status" "$tmp/skills/docket-status/SKILL.md"'
assert "canonical left untouched"           'grep -q "rest-new" "$tmp/skills/docket-new-change/SKILL.md"'

# --check now passes (exit 0)
( cd "$tmp" && bash sync-convention.sh --check >/dev/null 2>&1 ); rc=$?
assert "--check passes after sync (exit 0)" '[ "$rc" = "0" ]'

# Missing markers in a non-canonical skill is an error in sync mode (exit != 0,1)
mkdir -p "$tmp/skills/docket-adr"; printf 'no markers here\n' > "$tmp/skills/docket-adr/SKILL.md"
( cd "$tmp" && bash sync-convention.sh >/dev/null 2>&1 ); rc=$?
assert "sync errors when markers missing (exit 2)" '[ "$rc" = "2" ]'

exit $fail
