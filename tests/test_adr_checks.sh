#!/usr/bin/env bash
# tests/test_adr_checks.sh — verifies change 0030: scripts/adr-checks.sh, the ADR-ledger analog of
# board-checks.sh (numbering gaps, dangling links, status inconsistencies). Offline (no gh, no
# network); warn-only. Run: bash tests/test_adr_checks.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/adr-checks.sh"
SKILL="$REPO/skills/docket-adr/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
# has_finding OUTPUT CHECK-ID ADR-ID — literal-TAB ERE (portable; no grep -P).
has_finding(){ printf '%s' "$1" | grep -qE "$(printf '^%s\t%s\t' "$2" "$3")"; }

mkadr(){ # mkadr DIR ID STATUS SUPERSEDES REVERSES RELATES  (lists like "[]" or "[4]")
  cat > "$1/$(printf '%04d' "$2")-a$2.md" <<EOF
---
id: $2
slug: a$2
title: Decision $2
status: $3
date: 2026-06-01
supersedes: $4
reverses: $5
relates_to: $6
change:
---
## Decision
x.
EOF
}

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# ===== clean ledger: 1,2,3 all Accepted, 3 supersedes-free => no output, exit 0 =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[]" "[1]"
mkadr "$d" 3 Accepted "[]" "[]" "[]"
echo "# index" > "$d/README.md"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"; rc=$?
assert "clean ledger: no output" '[ -z "$out" ]'
assert "clean ledger: exit 0" '[ "$rc" -eq 0 ]'
bash "$SCRIPT" --adrs-dir "$d" --strict >/dev/null 2>&1
assert "clean ledger: --strict exits 0" '[ "$?" -eq 0 ]'
rm -rf "$d"

# ===== adr-numbering-gap: 1 and 3 present, 2 missing =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 3 Accepted "[]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "numbering-gap flagged on missing id 2" 'has_finding "$out" adr-numbering-gap 2'
assert "numbering-gap NOT flagged on present id 1" '! has_finding "$out" adr-numbering-gap 1'
rm -rf "$d"

# ===== adr-dangling-link: ADR-2 relates_to [9] which has no file =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[]" "[9]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "dangling-link flagged on ADR-2 (relates_to 9 absent)" 'has_finding "$out" adr-dangling-link 2'
rm -rf "$d"

# ===== adr-status-inconsistent arm (a): status 'Superseded by ADR-0099', no ADR-99 =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 "Superseded by ADR-0099" "[]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (a) flagged on ADR-2 (target 99 absent)" 'has_finding "$out" adr-status-inconsistent 2'
rm -rf "$d"

# ===== adr-status-inconsistent arm (b): ADR-2 supersedes [1] but ADR-1 status still Accepted =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[1]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) flagged on un-flipped target ADR-1" 'has_finding "$out" adr-status-inconsistent 1'
# control: when ADR-1 IS flipped, arm (b) is silent.
d2="$(mktemp -d)"
mkadr "$d2" 1 "Superseded by ADR-0002" "[]" "[]" "[]"
mkadr "$d2" 2 Accepted "[1]" "[]" "[]"
out2="$(bash "$SCRIPT" --adrs-dir "$d2" 2>/dev/null)"
assert "status-inconsistent (b) silent when target correctly flipped" '! has_finding "$out2" adr-status-inconsistent 1'
rm -rf "$d" "$d2"

# ===== adr-status-inconsistent arm (b) verb-mismatch: right id, WRONG verb =====
# ADR-2 supersedes [1] but ADR-1's status uses the REVERSED verb -> must flag ADR-1.
d="$(mktemp -d)"
mkadr "$d" 1 "Reversed by ADR-0002" "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[1]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) flagged on supersedes-edge w/ reversed-verb target" 'has_finding "$out" adr-status-inconsistent 1'
rm -rf "$d"
# ADR-2 reverses [1] but ADR-1's status uses the SUPERSEDED verb -> must flag ADR-1.
d="$(mktemp -d)"
mkadr "$d" 1 "Superseded by ADR-0002" "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[1]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) flagged on reverses-edge w/ superseded-verb target" 'has_finding "$out" adr-status-inconsistent 1'
rm -rf "$d"
# control: reverses edge with the CORRECT verb on target -> silent.
d="$(mktemp -d)"
mkadr "$d" 1 "Reversed by ADR-0002" "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[1]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) silent on reverses edge correctly flipped" '! has_finding "$out" adr-status-inconsistent 1'
rm -rf "$d"

# ===== --strict exits 1 on any finding =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 3 Accepted "[]" "[]" "[]"   # gap at 2
bash "$SCRIPT" --adrs-dir "$d" --strict >/dev/null 2>&1
assert "--strict exits 1 when findings exist" '[ "$?" -eq 1 ]'
rm -rf "$d"

# ===== usage =====
bash "$SCRIPT" >/dev/null 2>&1; assert "missing --adrs-dir exits 2" '[ "$?" -eq 2 ]'

# ===== malformed-id: a non-integer id is flagged, valid neighbours unaffected =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[]" "[]"
printf -- '---\nid: abc\nslug: bad\ntitle: Bad\nstatus: Accepted\ndate: 2026-06-01\nsupersedes: []\nreverses: []\nrelates_to: []\nchange:\n---\n## Decision\nx.\n' > "$d/9001-bad.md"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"; rc=$?
assert "adr malformed-id flagged on non-integer id 'abc'" 'has_finding "$out" malformed-id abc'
assert "adr malformed-id: no numbering-gap false positive (valid 1,2 only)" '! has_finding "$out" adr-numbering-gap 1'
assert "adr malformed-id: script still exits 0 (warn-only)" '[ "$rc" -eq 0 ]'
rm -rf "$d"
# control: clean ledger emits no malformed-id
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "adr malformed-id silent on clean ledger" '! has_finding "$out" malformed-id 1'
rm -rf "$d"

# ===== docket-adr wiring sentinel =====
assert "docket-adr Index/validate invokes adr-checks.sh" 'grep -qF "scripts/adr-checks.sh" "$SKILL"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
