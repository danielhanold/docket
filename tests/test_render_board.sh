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
# Since change 0058 the docket-status Board pass lives in scripts/docket-status.sh, not this
# SKILL — the SKILL only *describes* the inline surface (naming render-board.sh) and delegates to
# the orchestrator. Change 0059 therefore does NOT edit this SKILL; the gated-write wiring
# (board_pass_inline -> board-refresh.sh) is asserted in tests/test_docket_status.sh instead. These
# two sentinels are unchanged from main.
assert "docket-status inline surface names render-board.sh" \
  'grep -qF "/render-board.sh" "$SKILL"'
assert "docket-status keeps the regenerate-don't-3-way-merge rule" \
  'grep -qiF "never 3-way merge" "$SKILL"'

# --- negative sentinel: no skill body may redirect render-board.sh stdout straight into
# BOARD.md (the pre-0059 anti-pattern this task removes). Whitespace-normalize per file first
# since the old redirect could span physical lines. The guard regex:
#   render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md
# Design (each element defends a specific real shape in this codebase's prose):
#   - `.{0,200}` bounded any-char gaps (NOT `[^>]*`): the historical redirect's destination is
#     a bracket placeholder `<metadata working tree>/<changes_dir>/BOARD.md` whose `>` characters
#     and internal spaces a `[^>]*` class could never cross — so `[^>]*` was BLIND to the exact
#     reintroduction shape this sentinel exists to catch. `.` crosses placeholder `>`s freely.
#   - `[[:space:]]>[[:space:]]` a whitespace-bounded redirect operator: a real ` > ` redirect has
#     a space on both sides, whereas a placeholder's closing bracket is `tree>` / `dir>/` (letter
#     before `>`, or no space after) — so the porcelain guard line and every `<...>` placeholder
#     are structurally excluded.
#   - `/BOARD\.md` (slash required, not bare `BOARD.md`): a real redirect target is a PATH ending
#     in `/BOARD.md`; this rejects a flattened markdown blockquote (`\n> ` -> ` > `) that lands a
#     bare "BOARD.md" prose word within the window — blockquotes genuinely appear in
#     docket-status and docket-implement-next, so this is a live false-positive class, not
#     hypothetical.
# All five requirements + the blockquote case are verified empirically below and by the
# positive-control assertion. THE SAME REGEX is used for both the positive control and the
# across-skills scan, so weakening it (e.g. back to `[^>]*`) trips the positive control loudly.
REDIRECT_RE='render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md'

# Positive control ("test the test"): the historical bracket-placeholder redirect that WAS in
# this codebase pre-0059 MUST still be flagged by the guard. If a future edit weakens REDIRECT_RE
# so it can no longer cross placeholder brackets, this assertion fails — not the silent scan.
HISTORICAL_REDIRECT='"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir <metadata working tree>/<changes_dir> > <metadata working tree>/<changes_dir>/BOARD.md'
assert "guard regex flags the historical bracket-placeholder redirect (positive control)" \
  'printf "%s" "$HISTORICAL_REDIRECT" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'

# Negative scan: no CURRENT skill body redirects render-board.sh stdout into BOARD.md.
redirect_found=0
for f in "$REPO"/skills/*/SKILL.md; do
  if tr '\n' ' ' < "$f" | grep -Eq "$REDIRECT_RE"; then
    echo "  (direct render-board.sh -> BOARD.md redirect found in: $f)"
    redirect_found=1
  fi
done
assert "no skills/*/SKILL.md redirects render-board.sh stdout directly into BOARD.md" \
  '[ "$redirect_found" -eq 0 ]'

# Same regex, now also over scripts/docket-status.sh (change 0069). That script gained a LEGITIMATE
# render-board.sh call (`--format digest`, read-only, piped into its report), and the sentinel in
# tests/test_docket_status.sh that polices it only tokenizes the FLAG — so
# `render-board.sh --changes-dir "$1" --format digest > "$1/BOARD.md"` satisfies it while writing
# the very file board-refresh.sh is supposed to own. This scan guards the WRITE, not the flag; the
# two catch different holes, so both stay.
status_redirect=0
if tr '\n' ' ' < "$REPO/scripts/docket-status.sh" | grep -Eq "$REDIRECT_RE"; then
  echo "  (direct render-board.sh -> BOARD.md redirect found in: scripts/docket-status.sh)"
  status_redirect=1
fi
assert "scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md" \
  '[ "$status_redirect" -eq 0 ]'

# --- malformed id is skipped (active + archive), renderer still succeeds ---
printf -- '---\nid: abc\nslug: bad\ntitle: Bad Active\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$tmp/active/0099-bad.md"
printf -- '---\nid: nope\nslug: badarc\ntitle: Bad Archive\nstatus: done\npriority: low\ndepends_on: []\n---\n' > "$tmp/archive/2026-06-01-0098-badarc.md"
mout="$("$SCRIPT" --changes-dir "$tmp" 2>/tmp/render-board-stderr.$$)"; mrc=$?
assert "render-board exits 0 with a malformed-id file present" '[ "$mrc" -eq 0 ]'
assert "render-board skips malformed active row (title absent)"  '! printf "%s" "$mout" | grep -q "Bad Active"'
assert "render-board skips malformed archive row (title absent)" '! printf "%s" "$mout" | grep -q "Bad Archive"'
rm -f "$tmp/active/0099-bad.md" "$tmp/archive/2026-06-01-0098-badarc.md" /tmp/render-board-stderr.$$

# --- change 0069: --format digest (the line-oriented backlog projection) ---
# The digest is a SECOND projection of the dependency/readiness pass render-board.sh already
# runs — same source of truth as the board's Readiness cell, machine-parseable instead of prose.
# It is report output, never a board surface: docket-status.sh pipes it through without writing.

# (a) regression guard: the DEFAULT (no --format) output is byte-identical to the golden.
#     This is the load-bearing guarantee of the whole change — the digest must not perturb the
#     markdown path by a single byte. ("$rendered" was produced from the golden compare above.)
defaulted="$tmp/out-default.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$defaulted" 2>/dev/null
assert "default output is byte-identical to the golden after --format lands" \
  'diff -u "$golden" "$defaulted"'

# (b) an explicit --format markdown is byte-identical to the default.
explicit="$tmp/out-markdown.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format markdown > "$explicit" 2>/dev/null
assert "--format markdown is byte-identical to the default" 'diff -u "$defaulted" "$explicit"'

# (c) the digest's exact shape, byte-compared to a hand-authored golden. Rollups first (fixed
#     status order, non-zero only), then one `change` line per ACTIVE change, ascending id.
digest_golden="$tmp/digest-golden.txt"
cat > "$digest_golden" <<'EOF'
backlog in-progress 1
backlog proposed 5
backlog blocked 1
backlog deferred 1
backlog implemented 1
backlog done 2
backlog killed 1
change 1 in-progress - alpha
change 2 proposed build-ready bravo
change 3 proposed needs-brainstorm charlie
change 4 proposed auto-groom-blocked delta
change 5 proposed waiting-on-3-unbuilt echo
change 6 proposed waiting-on-8-needs-merge foxtrot
change 7 blocked - golf
change 8 implemented - hotel
change 9 deferred - india
EOF
digest_out="$tmp/digest-out.txt"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format digest > "$digest_out" 2>/dev/null
drc=$?
assert "--format digest exits 0" '[ "$drc" -eq 0 ]'
assert "--format digest matches the digest golden byte-for-byte" 'diff -u "$digest_golden" "$digest_out"'

# (d) each readiness band individually (named asserts so a break names the band it broke).
assert "digest: build-ready token"            'grep -qxF "change 2 proposed build-ready bravo" "$digest_out"'
assert "digest: needs-brainstorm token"       'grep -qxF "change 3 proposed needs-brainstorm charlie" "$digest_out"'
assert "digest: auto-groom-blocked token"     'grep -qxF "change 4 proposed auto-groom-blocked delta" "$digest_out"'
assert "digest: waiting-on-N-unbuilt token"   'grep -qxF "change 5 proposed waiting-on-3-unbuilt echo" "$digest_out"'
assert "digest: waiting-on-N-needs-merge token" 'grep -qxF "change 6 proposed waiting-on-8-needs-merge foxtrot" "$digest_out"'
assert "digest: readiness is - for a non-proposed change" 'grep -qxF "change 1 in-progress - alpha" "$digest_out"'

# (e) the digest carries NO markdown board (it is a projection, not the board).
assert "digest emits no board markdown" '! grep -qF "# Backlog" "$digest_out"'
assert "digest emits no mermaid graph"  '! grep -qF "mermaid" "$digest_out"'

# (f) archive rollups only — archived changes get no `change` line (the digest is the ACTIVE backlog).
assert "digest: archived changes get no change line" '! grep -qE "^change (10|11|12) " "$digest_out"'

# (g) an unknown --format value is an argument error (exit 2), like any other bad flag.
bash "$SCRIPT" --changes-dir "$tmp" --format bogus >/dev/null 2>"$tmp/fmt-err.txt"
frc=$?
assert "unknown --format exits 2" '[ "$frc" -eq 2 ]'
assert "unknown --format names the flag on stderr" 'grep -qi "format" "$tmp/fmt-err.txt"'

# (h) the digest is a READ-ONLY projection: it writes no BOARD.md (that file is board-refresh.sh's
#     alone) and creates no git state. The BOARD.md check is the one that bites — nothing ever
#     `git init`s $tmp, so the git-free half is true before render-board.sh is even invoked and
#     would stay green even if the digest branch wrote the board.
assert "digest run writes no BOARD.md into the changes dir" '[ ! -e "$tmp/BOARD.md" ]'
assert "digest run leaves the fixture dir git-free" '[ ! -d "$tmp/.git" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
