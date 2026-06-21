#!/usr/bin/env bash
# tests/test_render_board.sh — verifies change 0022: deterministic BOARD.md rendering.
# A fixture changes/ tree spanning every status is rendered and byte-compared to a hand-authored
# golden; a second render must be byte-identical (idempotence). Also asserts the docket-status
# inline-surface wiring. Run: bash tests/test_render_board.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/render-board.sh"
SKILL="$REPO/skills/docket-status/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

cat > "$tmp/active/0001-alpha.md" <<'EOF'
---
id: 1
slug: alpha
title: Alpha feature
status: in-progress
priority: high
depends_on: []
spec: docs/superpowers/specs/2026-06-10-alpha.md
branch: feat/alpha
EOF
cat > "$tmp/active/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo feature
status: proposed
priority: medium
depends_on: [10]
spec: docs/superpowers/specs/2026-06-10-bravo.md
EOF
cat > "$tmp/active/0003-charlie.md" <<'EOF'
---
id: 3
slug: charlie
title: Charlie feature
status: proposed
priority: medium
depends_on: []
spec:
EOF
cat > "$tmp/active/0004-delta.md" <<'EOF'
---
id: 4
slug: delta
title: Delta feature
status: proposed
priority: low
depends_on: []
spec:
---

## Auto-groom blocked

2026-06-12 — abstained.
EOF
cat > "$tmp/active/0005-echo.md" <<'EOF'
---
id: 5
slug: echo
title: Echo feature
status: proposed
priority: medium
depends_on: [3]
spec: docs/superpowers/specs/2026-06-10-echo.md
EOF
cat > "$tmp/active/0006-foxtrot.md" <<'EOF'
---
id: 6
slug: foxtrot
title: Foxtrot feature
status: proposed
priority: medium
depends_on: [8]
spec: docs/superpowers/specs/2026-06-10-foxtrot.md
EOF
cat > "$tmp/active/0007-golf.md" <<'EOF'
---
id: 7
slug: golf
title: Golf feature
status: blocked
priority: medium
depends_on: []
blocked_by: upstream API frozen until Q3
EOF
cat > "$tmp/active/0008-hotel.md" <<'EOF'
---
id: 8
slug: hotel
title: Hotel feature
status: implemented
priority: high
depends_on: []
pr: https://github.com/o/r/pull/142
EOF
cat > "$tmp/active/0009-india.md" <<'EOF'
---
id: 9
slug: india
title: India feature
status: deferred
priority: low
depends_on: []
EOF
cat > "$tmp/archive/2026-06-15-0010-juliet.md" <<'EOF'
---
id: 10
slug: juliet
title: Juliet feature
status: done
priority: medium
depends_on: []
EOF
cat > "$tmp/archive/2026-06-16-0012-lima.md" <<'EOF'
---
id: 12
slug: lima
title: Lima feature
status: done
priority: medium
depends_on: []
EOF
cat > "$tmp/archive/2026-06-14-0011-kilo.md" <<'EOF'
---
id: 11
slug: kilo
title: Kilo feature
status: killed
priority: low
depends_on: []
EOF

# Hand-authored golden — the executable form of docket-status Board -> Structure.
golden="$tmp/golden.md"
cat > "$golden" <<'EOF'
# Backlog

**12 changes** — 🟢 1 in progress · 🟡 5 proposed · 🔴 1 blocked · ⚪ 1 deferred · 🔵 1 implemented · ✅ 2 done · 🗑️ 1 killed

## 🟢 In progress (1)

| # | Title | Priority | Spec | Branch |
|---|-------|----------|------|--------|
| [0001](active/0001-alpha.md) | Alpha feature | `high` | [spec](../superpowers/specs/2026-06-10-alpha.md) | `feat/alpha` |

## 🟡 Proposed (5)

| # | Title | Priority | Readiness |
|---|-------|----------|-----------|
| [0002](active/0002-bravo.md) | Bravo feature | `medium` | build-ready |
| [0003](active/0003-charlie.md) | Charlie feature | `medium` | needs-brainstorm |
| [0004](active/0004-delta.md) | Delta feature | `low` | auto-groom blocked — needs you |
| [0005](active/0005-echo.md) | Echo feature | `medium` | ⏳ waiting on #3 — not yet built |
| [0006](active/0006-foxtrot.md) | Foxtrot feature | `medium` | ⏳ waiting on #8 — needs your merge |

## 🔴 Blocked (1)

| # | Title | Priority | Blocked by |
|---|-------|----------|------------|
| [0007](active/0007-golf.md) | Golf feature | `medium` | upstream API frozen until Q3 |

## ⚪ Deferred (1)

| # | Title | Priority |
|---|-------|----------|
| [0009](active/0009-india.md) | India feature | `low` |

## 🔵 Implemented — awaiting merge (1)

| # | Title | Priority | PR |
|---|-------|----------|----|
| [0008](active/0008-hotel.md) | Hotel feature | `high` | [#142](https://github.com/o/r/pull/142) |

```mermaid
graph TD
  0001
  0010 --> 0002
  0003
  0004
  0003 --> 0005
  0008 --> 0006
  0007
  0008
  0009
  0010:::done
  0012:::done
  classDef done fill:#d3f9d8;
```

<details><summary>✅🗑️ Archive — done + killed (3)</summary>

| # | Title | Merged |
|---|-------|--------|
| [0012](archive/2026-06-16-0012-lima.md) | Lima feature | 2026-06-16 |
| [0010](archive/2026-06-15-0010-juliet.md) | Juliet feature | 2026-06-15 |
| [0011](archive/2026-06-14-0011-kilo.md) | Kilo feature | 2026-06-14 |

</details>
EOF

rendered="$tmp/out.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$rendered" 2>/dev/null
assert "rendered output matches the golden byte-for-byte" 'diff -u "$golden" "$rendered"'

# idempotence: a second render is byte-identical to the first
rendered2="$tmp/out2.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$rendered2" 2>/dev/null
assert "render is idempotent (re-run is byte-identical)" 'diff -u "$rendered" "$rendered2"'

# PR cell: the docket convention is that pr: holds the FULL URL (#8 Hotel above, exercised by the
# golden). Also cover the bare-number fallback in a focused fixture (renders the same #N link via --repo).
bare="$(mktemp -d)"; mkdir -p "$bare/active" "$bare/archive"
cat > "$bare/active/0001-bare.md" <<'EOF'
---
id: 1
slug: bare
title: Bare PR
status: implemented
priority: medium
depends_on: []
pr: 77
EOF
bareout="$(bash "$SCRIPT" --changes-dir "$bare" --repo o/r 2>/dev/null)"
assert "pr: full URL renders [#N](url) without double-wrapping (Hotel #8 in the golden)" \
  'grep -qF "[#142](https://github.com/o/r/pull/142)" "$rendered"'
assert "pr: bare number falls back to [#N](built-url) via --repo" \
  'printf "%s" "$bareout" | grep -qF "[#77](https://github.com/o/r/pull/77)"'
rm -rf "$bare"

# --- docket-status inline-surface wiring sentinels (the SKILL is code on main) ---
assert "docket-status inline surface invokes render-board.sh" \
  'grep -qF "/render-board.sh" "$SKILL"'
assert "docket-status keeps the regenerate-don't-3-way-merge rule" \
  'grep -qiF "never 3-way merge" "$SKILL"'

# --- malformed id is skipped (active + archive), renderer still succeeds ---
printf -- '---\nid: abc\nslug: bad\ntitle: Bad Active\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$tmp/active/0099-bad.md"
printf -- '---\nid: nope\nslug: badarc\ntitle: Bad Archive\nstatus: done\npriority: low\ndepends_on: []\n---\n' > "$tmp/archive/2026-06-01-0098-badarc.md"
mout="$("$SCRIPT" --changes-dir "$tmp" 2>/tmp/render-board-stderr.$$)"; mrc=$?
assert "render-board exits 0 with a malformed-id file present" '[ "$mrc" -eq 0 ]'
assert "render-board skips malformed active row (title absent)"  '! printf "%s" "$mout" | grep -q "Bad Active"'
assert "render-board skips malformed archive row (title absent)" '! printf "%s" "$mout" | grep -q "Bad Archive"'
rm -f "$tmp/active/0099-bad.md" "$tmp/archive/2026-06-01-0098-badarc.md" /tmp/render-board-stderr.$$

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
