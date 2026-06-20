#!/usr/bin/env bash
# tests/test_render_adr_index.sh — verifies change 0030: deterministic ADR index rendering.
# A fixture adrs/ tree spanning Active (with change:/relates_to:/→supersedes/→reverses), a
# Superseded, a Reversed, and a Deprecated entry is rendered and byte-compared to a hand-authored
# golden; a second render must be byte-identical (idempotence); an empty-ledger case renders all
# three groups as _None._. Also asserts docket-adr wiring. Run: bash tests/test_render_adr_index.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/render-adr-index.sh"
SKILL="$REPO/skills/docket-adr/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- Active: two Accepted ADRs (plurality), one with change: + relates_to (plural), one with
#     → supersedes (plural) AND → reverses to exercise every annotation + the ", " joiner. ---
cat > "$tmp/0001-alpha.md" <<'EOF'
---
id: 1
slug: alpha
title: Alpha decision
status: Accepted
date: 2026-06-01
supersedes: []
reverses: []
relates_to: []
change: 2
---
## Decision
A.
EOF
cat > "$tmp/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo decision
status: Accepted
date: 2026-06-02
supersedes: []
reverses: []
relates_to: [1, 5]
change: 3
---
## Decision
B.
EOF
cat > "$tmp/0006-foxtrot.md" <<'EOF'
---
id: 6
slug: foxtrot
title: Foxtrot decision
status: Accepted
date: 2026-06-06
supersedes: [4, 5]
reverses: [3]
relates_to: []
change:
---
## Decision
F.
EOF
# --- Superseded / Reversed: one of each (plurality in the group). ---
cat > "$tmp/0004-delta.md" <<'EOF'
---
id: 4
slug: delta
title: Delta decision
status: Superseded by ADR-0006
date: 2026-06-04
supersedes: []
reverses: []
relates_to: []
change: 4
---
## Decision
D.
EOF
cat > "$tmp/0005-echo.md" <<'EOF'
---
id: 5
slug: echo
title: Echo decision
status: Superseded by ADR-0006
date: 2026-06-05
supersedes: []
reverses: []
relates_to: []
change:
---
## Decision
E.
EOF
cat > "$tmp/0003-charlie.md" <<'EOF'
---
id: 3
slug: charlie
title: Charlie decision
status: Reversed by ADR-0006
date: 2026-06-03
supersedes: []
reverses: []
relates_to: []
change:
---
## Decision
C.
EOF
# --- Deprecated. ---
cat > "$tmp/0007-golf.md" <<'EOF'
---
id: 7
slug: golf
title: Golf decision
status: Deprecated
date: 2026-06-07
supersedes: []
reverses: []
relates_to: []
change:
---
## Decision
G.
EOF
# A README.md must be ignored (never indexed, never a row).
echo "# old index" > "$tmp/README.md"

# Hand-authored golden — the byte-for-byte contract for the renderer.
golden="$tmp/golden.md"
cat > "$golden" <<'EOF'
# Architecture Decision Records

Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.

## Active

- [ADR-0001](0001-alpha.md) — Alpha decision (Accepted) ← change #2
- [ADR-0002](0002-bravo.md) — Bravo decision (Accepted) ← change #3 · relates to ADR-0001, ADR-0005
- [ADR-0006](0006-foxtrot.md) — Foxtrot decision (Accepted) → supersedes ADR-0004, ADR-0005 → reverses ADR-0003

## Superseded / Reversed

- [ADR-0003](0003-charlie.md) — Charlie decision (Reversed by ADR-0006)
- [ADR-0004](0004-delta.md) — Delta decision (Superseded by ADR-0006)
- [ADR-0005](0005-echo.md) — Echo decision (Superseded by ADR-0006)

## Deprecated

- [ADR-0007](0007-golf.md) — Golf decision (Deprecated)
EOF

rendered="$tmp/out.md"
bash "$SCRIPT" --adrs-dir "$tmp" > "$rendered" 2>/dev/null
assert "rendered output matches the golden byte-for-byte" 'diff -u "$golden" "$rendered"'

rendered2="$tmp/out2.md"
bash "$SCRIPT" --adrs-dir "$tmp" > "$rendered2" 2>/dev/null
assert "render is idempotent (re-run is byte-identical)" 'diff -u "$rendered" "$rendered2"'

assert "README.md is never emitted as a row" '! grep -q "old index" "$rendered"'

# --- empty-ledger: all three groups render _None._ ---
empty="$(mktemp -d)"
emptyout="$(bash "$SCRIPT" --adrs-dir "$empty" 2>/dev/null)"
assert "empty ledger: Active group renders _None._" \
  'printf "%s\n" "$emptyout" | awk "/## Active/{f=1;next} /## /{f=0} f&&/_None._/{ok=1} END{exit !ok}"'
assert "empty ledger: three _None._ lines (one per group)" \
  '[ "$(printf "%s\n" "$emptyout" | grep -c "^_None\._$")" -eq 3 ]'
rm -rf "$empty"

# --- usage errors ---
bash "$SCRIPT" >/dev/null 2>&1; assert "missing --adrs-dir exits 2" '[ "$?" -eq 2 ]'
bash "$SCRIPT" --adrs-dir "$tmp/nope" >/dev/null 2>&1; assert "absent dir exits 2" '[ "$?" -eq 2 ]'

# --- docket-adr wiring sentinels (the SKILL is code on the integration branch) ---
assert "docket-adr Index/validate invokes render-adr-index.sh" 'grep -qF "scripts/render-adr-index.sh" "$SKILL"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
